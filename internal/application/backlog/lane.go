package backlog

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/backlog"
)

// maxBody caps a finding's detail file in an issue body.
const maxBody = 20000

// LaneResult is how `loops backlog work` ended.
type LaneResult struct {
	Item     backlog.Item   // the issue after the lane
	Status   backlog.Status // done, queued (retry) or human
	Reason   string         // why it did not open a PR
	Findings []Filed
	Empty    bool // nothing was queued: no lane ran
}

// Filed is one finding filed (or matched to an earlier one).
type Filed struct {
	Title     string
	URL       string
	Duplicate bool
}

// Work runs one lane end to end (BACKLOG §3): claim issue n (0 = the next
// one), add its worktree, run the agent, record the outcome, hand off or
// re-queue a failure, and file the lane's findings.
func (s *Service) Work(ctx context.Context, n int) (LaneResult, error) {
	st, err := s.load(ctx)
	if err != nil {
		return LaneResult{}, err
	}
	set := st.Settings
	var it backlog.Item
	var release func()
	err = s.store.Update(ctx, func(st *backlog.State) error {
		var err error
		pick := n
		if pick == 0 {
			q := st.Queue()
			if len(q) == 0 {
				return backlog.ErrEmpty
			}
			pick = q[0].Number
		}
		if it, err = st.Claim(pick, worktree(set, pick), s.clock.Now()); err != nil {
			return err
		}
		release, err = s.store.HoldLane(pick)
		return err
	})
	if errors.Is(err, backlog.ErrEmpty) {
		return LaneResult{Empty: true}, nil
	}
	if err != nil {
		return LaneResult{}, err
	}
	defer release()
	s.edit(ctx, set, it.Number, Edit{AddLabels: []string{set.ClaimLabel}})

	agentErr := ""
	if err := s.wt.Add(ctx, it.Worktree, set.Base); err != nil {
		agentErr = "worktree: " + err.Error()
	} else {
		runCtx, cancel := context.WithTimeout(ctx, set.TimeoutDuration())
		if err := s.agent.Run(runCtx, it.Worktree, Prompt(set, it.Number), s.store.LogPath(it.Number), set); err != nil {
			agentErr = "agent: " + err.Error()
			if runCtx.Err() != nil {
				agentErr = fmt.Sprintf("agent stopped after %s: %v", set.Timeout, runCtx.Err())
			}
		}
		cancel()
	}
	lane, err := s.lanes.Latest(it.Worktree)
	if err != nil {
		s.warn("read lane %d: %v", it.Number, err)
	}
	out := lane.Outcome(agentErr)
	if agentErr != "" && lane.Dir == "" {
		out = backlog.Outcome{Reason: agentErr}
	}
	var res LaneResult
	err = s.store.Update(ctx, func(st *backlog.State) error {
		status, err := st.Finish(it.Number, out, s.clock.Now())
		if cur, ok := st.Item(it.Number); ok {
			res.Item = *cur
		}
		res.Status, res.Reason = status, out.Reason
		return err
	})
	if err != nil {
		return res, err
	}
	s.edit(ctx, set, it.Number, Edit{RemoveLabels: []string{set.ClaimLabel}})
	if res.Status == backlog.Human {
		s.handoff(ctx, set, res.Item)
	}
	res.Findings = s.fileLane(ctx, set, res.Item, lane)
	return res, nil
}

// edit applies a best-effort label or assignee change.
func (s *Service) edit(ctx context.Context, set backlog.Settings, n int, e Edit) {
	if err := s.forge.Edit(ctx, set.Repo, n, e); err != nil {
		s.warn("issue %d: %v", n, err)
	}
}

// fileLane files every finding the lane recorded (BACKLOG §5).
func (s *Service) fileLane(ctx context.Context, set backlog.Settings, it backlog.Item, lane backlog.LaneRun) []Filed {
	found := lane.Findings()
	if len(found) == 0 {
		return nil
	}
	open, err := s.forge.OpenIssues(ctx, set.Repo)
	if err != nil {
		s.warn("findings of issue %d not filed: %v", it.Number, err)
		return nil
	}
	pr := it.PR
	if pr == "" {
		pr = "no PR"
	}
	var out []Filed
	for _, f := range found {
		body := fmt.Sprintf("Found by a loops backlog lane while working #%d (%s).\n", it.Number, pr)
		if f.Ref != "" {
			if text, ok := s.lanes.ReadRef(lane.Dir, f.Ref); ok {
				body += "\n" + truncate(text, maxBody) + "\n"
			}
		}
		filed, err := s.file(ctx, set, open, f.Title, body, it.Number)
		if err != nil {
			s.warn("finding %q: %v", f.Title, err)
			continue
		}
		out = append(out, filed)
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n\n…(truncated)"
}

// File files one finding by hand (`loops backlog file`).
func (s *Service) File(ctx context.Context, title, body string, from int) (Filed, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Filed{}, errors.New("a finding needs a title")
	}
	st, err := s.load(ctx)
	if err != nil {
		return Filed{}, err
	}
	open, err := s.forge.OpenIssues(ctx, st.Settings.Repo)
	if err != nil {
		return Filed{}, err
	}
	return s.file(ctx, st.Settings, open, title, body, from)
}

// file creates the finding's issue unless it duplicates one (BACKLOG §5).
// The duplicate check and the record run under the state lock, so two
// lanes ending together never file the same finding twice.
func (s *Service) file(ctx context.Context, set backlog.Settings, open []backlog.Issue, title, body string, from int) (Filed, error) {
	res := Filed{Title: title}
	err := s.store.Update(ctx, func(st *backlog.State) error {
		if dup := st.Duplicate(title, open); dup != "" {
			res.URL, res.Duplicate = dup, true
			return nil
		}
		body += fmt.Sprintf("\n---\nFiled by `loops backlog` and assigned to @%s to triage. A lane takes it only after the `%s` label is removed and it is unassigned.\n",
			set.Human, set.FindingLabel)
		url, err := s.forge.Create(ctx, set.Repo, NewIssue{Title: title, Body: body,
			Labels: []string{set.FindingLabel}, Assignees: []string{set.Human}})
		if err != nil {
			return err
		}
		res.URL = url
		st.Findings = append(st.Findings, backlog.Finding{Key: backlog.FindingKey(title), Title: title, URL: url, From: from})
		return nil
	})
	if err == nil && res.Duplicate && from > 0 {
		if cerr := s.forge.Comment(ctx, set.Repo, from, fmt.Sprintf("A loops backlog lane found: %s. Already tracked in %s.", title, res.URL)); cerr != nil {
			s.warn("issue %d: %v", from, cerr)
		}
	}
	return res, err
}

// Finish records the supervisor's ruling on issue n (`loops backlog
// finish`): human hands it off (BACKLOG §6); queued also takes back an
// earlier hand-off's label and assignee so sync keeps it.
func (s *Service) Finish(ctx context.Context, n int, status backlog.Status, note string, force bool) (backlog.Item, error) {
	st, err := s.load(ctx)
	if err != nil {
		return backlog.Item{}, err
	}
	if status == backlog.Running {
		return backlog.Item{}, errors.New("only a lane sets running (loops backlog work)")
	}
	var it backlog.Item
	err = s.store.Update(ctx, func(st *backlog.State) error {
		if cur, ok := st.Item(n); ok && cur.Status == backlog.Running && s.store.LaneHeld(n) {
			return fmt.Errorf("issue %d has a live lane; wait for it to end", n)
		}
		var err error
		it, err = st.Set(n, status, note, force, s.clock.Now())
		return err
	})
	if err != nil {
		return it, err
	}
	switch status {
	case backlog.Human:
		s.handoff(ctx, st.Settings, it)
	case backlog.Queued:
		s.edit(ctx, st.Settings, n, Edit{RemoveLabels: []string{st.Settings.HumanLabel}, RemoveAssignees: []string{st.Settings.Human}})
	}
	return it, nil
}

// handoff comments on the issue, labels it and assigns the human (BACKLOG §6).
func (s *Service) handoff(ctx context.Context, set backlog.Settings, it backlog.Item) {
	pr := it.PR
	if pr == "" {
		pr = "none"
	}
	body := fmt.Sprintf("loops backlog handed this issue to @%s.\n\n- Reason: %s\n- Attempts: %d of %d\n- PR: %s\n- Lane log (supervisor's machine): %s\n- Last run: %s\n\nTo give it back to the lanes, unassign it and remove the `%s` label.\n",
		set.Human, orNone(it.Note), it.Attempts, set.Attempts, pr, s.store.LogPath(it.Number), orNone(it.RunDir), set.HumanLabel)
	if err := s.forge.Comment(ctx, set.Repo, it.Number, body); err != nil {
		s.warn("issue %d: %v", it.Number, err)
	}
	if err := s.forge.Edit(ctx, set.Repo, it.Number, Edit{AddLabels: []string{set.HumanLabel}, AddAssignees: []string{set.Human}}); err != nil {
		s.warn("issue %d: %v", it.Number, err)
		return
	}
	err := s.store.Update(ctx, func(st *backlog.State) error {
		if cur, ok := st.Item(it.Number); ok && cur.Status == backlog.Human {
			cur.HandedOff = true
		}
		return nil
	})
	if err != nil {
		s.warn("issue %d: %v", it.Number, err)
	}
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

// Clean removes the worktrees of finished lanes: done and skipped, plus
// failed and human with all. It returns the worktrees removed (or, with
// dryRun, that would be).
func (s *Service) Clean(ctx context.Context, all, dryRun bool) ([]string, error) {
	st, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	finished := []backlog.Status{backlog.Done, backlog.Skipped}
	if all {
		finished = append(finished, backlog.Failed, backlog.Human)
	}
	var removed []string
	for _, it := range st.Items {
		if it.Worktree == "" || !slices.Contains(finished, it.Status) || !s.wt.Exists(it.Worktree) {
			continue
		}
		if !dryRun {
			if err := s.wt.Remove(ctx, it.Worktree); err != nil {
				s.warn("keep %s: %v", it.Worktree, err)
				continue
			}
		}
		removed = append(removed, it.Worktree)
	}
	if dryRun || len(removed) == 0 {
		return removed, nil
	}
	return removed, s.store.Update(ctx, func(st *backlog.State) error {
		for i := range st.Items {
			if slices.Contains(removed, st.Items[i].Worktree) {
				st.Items[i].Worktree = ""
			}
		}
		return nil
	})
}
