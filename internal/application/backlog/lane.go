package backlog

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/arxdsilva/ai-loops/internal/domain/backlog"
)

// tidyTimeout bounds the bookkeeping after a lane ends (state, labels,
// hand-off, findings), which runs even when the lane was stopped.
const tidyTimeout = 2 * time.Minute

// maxBody caps a finding's detail file in an issue body.
const maxBody = 20000

// LaneResult is how `loops backlog work` ended.
type LaneResult struct {
	Item     backlog.Item   // the issue after the lane
	Status   backlog.Status // done, queued (retry) or human
	Reason   string         // why it did not open a PR
	Findings []Filed
	Empty    bool         // nothing was queued: no lane ran
	Gated    bool         // the open-PR cap was reached: no lane ran
	Gate     backlog.Gate // the cap at the claim
}

var (
	// errGated stops a claim at the open-PR cap.
	errGated = errors.New("open-PR cap reached")
	// errNotPending means another process already filed a pending finding.
	errNotPending = errors.New("no longer pending")
)

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
	// The open-PR cap (BACKLOG §7) fails closed: without the forge's list
	// of open PRs no lane starts.
	since := s.clock.Now()
	var prs []string
	if set.PRCap() > 0 {
		if prs, err = s.prs.OpenPRs(ctx, set.Repo); err != nil {
			return LaneResult{}, fmt.Errorf("list the open PRs for the --max-prs cap: %w", err)
		}
	}
	var it backlog.Item
	var gate backlog.Gate
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
		// Running lanes count, so parallel claims never pass the cap.
		if gate = st.Gate(prs, since); gate.Closed() {
			return errGated
		}
		if it, err = st.Claim(pick, func(n, lane int) string { return worktree(set, n, lane) }, s.clock.Now()); err != nil {
			return err
		}
		release, err = s.store.HoldLane(pick)
		return err
	})
	switch {
	case errors.Is(err, backlog.ErrEmpty):
		return LaneResult{Empty: true}, nil
	case errors.Is(err, errGated):
		return LaneResult{Gated: true, Gate: gate}, nil
	case err != nil:
		// The claim was not saved: give back a lane lock taken before the save failed.
		if release != nil {
			release()
		}
		return LaneResult{}, err
	}
	held := true
	defer func() {
		if held {
			release()
		}
	}()
	s.edit(ctx, set, it.Number, Edit{AddLabels: []string{set.ClaimLabel}})

	agentErr, started := "", false
	if err := s.wt.Add(ctx, it.Worktree, set.Base); err != nil {
		agentErr = "worktree: " + err.Error()
	} else {
		started = true
		runCtx, cancel := context.WithTimeout(ctx, set.TimeoutDuration())
		if err := s.agent.Run(runCtx, it.Worktree, Prompt(set, it.Number), s.store.LogPath(it.Number), set); err != nil {
			switch {
			case ctx.Err() != nil:
				agentErr = "lane stopped: " + ctx.Err().Error()
			case runCtx.Err() != nil:
				agentErr = "agent stopped after the " + set.Timeout + " timeout"
			default:
				agentErr = "agent: " + err.Error()
			}
		}
		cancel()
	}
	// Record the outcome and tidy the forge even when the lane was stopped.
	ctx, cancelTidy := context.WithTimeout(context.WithoutCancel(ctx), tidyTimeout)
	defer cancelTidy()
	// The lane's own worktree is fresh, so its newest run is this lane's.
	var lane backlog.LaneRun
	if started {
		if lane, err = s.lanes.Latest(it.Worktree); err != nil {
			s.warn("read lane %d: %v", it.Number, err)
		}
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
	// The outcome is recorded: free the lane now, so a refill can claim the
	// issue again while the forge calls below run. A hand-off keeps it
	// until done, so a sync meanwhile does not hand it off a second time.
	if err != nil || res.Status != backlog.Human {
		held = false
		release()
	}
	if err != nil {
		return res, err
	}
	s.edit(ctx, set, it.Number, Edit{RemoveLabels: []string{set.ClaimLabel}})
	if res.Status == backlog.Human {
		s.handoff(ctx, set, res.Item)
		held = false
		release()
	}
	s.queueFindings(ctx, res.Item, lane)
	res.Findings = s.flush(ctx, set, it.Number)
	return res, nil
}

// edit applies a best-effort label or assignee change.
func (s *Service) edit(ctx context.Context, set backlog.Settings, n int, e Edit) {
	if err := s.forge.Edit(ctx, set.Repo, n, e); err != nil {
		s.warn("issue %d: %v", n, err)
	}
}

// queueFindings saves the lane's findings as pending (BACKLOG §5), so a
// forge failure or a crash before they are filed never loses one.
func (s *Service) queueFindings(ctx context.Context, it backlog.Item, lane backlog.LaneRun) {
	found := lane.Findings()
	if len(found) == 0 {
		return
	}
	pr := it.PR
	if pr == "" {
		pr = "no PR"
	}
	ps := make([]backlog.Pending, 0, len(found))
	for _, f := range found {
		body := fmt.Sprintf("Found by a loops backlog lane while working #%d (%s).\n", it.Number, pr)
		if f.Ref != "" {
			if text, ok := s.lanes.ReadRef(lane.Dir, f.Ref); ok {
				body += "\n" + truncate(text, maxBody) + "\n"
			}
		}
		ps = append(ps, backlog.Pending{Title: f.Title, Body: body, From: it.Number})
	}
	if err := s.store.Update(ctx, func(st *backlog.State) error { st.AddPending(ps...); return nil }); err != nil {
		s.warn("findings of issue %d not recorded: %v", it.Number, err)
	}
}

// flush files the pending findings of issue from (0 = every issue). A
// finding that fails stays pending for the next sync.
func (s *Service) flush(ctx context.Context, set backlog.Settings, from int) []Filed {
	st, err := s.store.Load(ctx)
	if err != nil {
		s.warn("pending findings: %v", err)
		return nil
	}
	var out []Filed
	for _, p := range st.Pending {
		if from != 0 && p.From != from {
			continue
		}
		filed, err := s.file(ctx, set, p.Title, p.Body, p.From, true)
		switch {
		case errors.Is(err, errNotPending):
		case err != nil:
			s.warn("finding %q stays pending: %v", p.Title, err)
		default:
			out = append(out, filed)
		}
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
	return s.file(ctx, st.Settings, title, body, from, false)
}

// file creates the finding's issue unless it duplicates one (BACKLOG §5).
// Everything runs under the state lock, the open issues are listed there,
// and the issue is recorded (and, when pending, leaves the pending list)
// in the same save, so two lanes ending together never file the same
// finding twice. A failure saves nothing: a pending finding stays pending.
func (s *Service) file(ctx context.Context, set backlog.Settings, title, body string, from int, pending bool) (Filed, error) {
	res := Filed{Title: title}
	err := s.store.Update(ctx, func(st *backlog.State) error {
		if pending && !st.IsPending(title, from) {
			return errNotPending
		}
		if pending {
			st.DropPending(title, from)
		}
		dup := st.Duplicate(title, nil)
		if dup == "" {
			open, err := s.forge.OpenIssues(ctx, set.Repo)
			if err != nil {
				return err
			}
			dup = st.Duplicate(title, open)
		}
		if dup != "" {
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

// handoff comments on the issue, labels it and assigns the human (BACKLOG
// §6). It is confirmed (HandedOff) only once the comment is posted and the
// forge took the label and assignee; otherwise the next sync retries it,
// never posting the comment twice. It reports whether it was confirmed.
func (s *Service) handoff(ctx context.Context, set backlog.Settings, it backlog.Item) bool {
	commented := it.Commented
	if !commented {
		pr := it.PR
		if pr == "" {
			pr = "none"
		}
		body := fmt.Sprintf("loops backlog handed this issue to @%s.\n\n- Reason: %s\n- Attempts: %d of %d\n- PR: %s\n- Lane log (supervisor's machine): %s\n- Last run: %s\n\nTo give it back to the lanes, unassign it and remove the `%s` label.\n",
			set.Human, orNone(it.Note), it.Attempts, set.Attempts, pr, s.store.LogPath(it.Number), orNone(it.RunDir), set.HumanLabel)
		if err := s.forge.Comment(ctx, set.Repo, it.Number, body); err != nil {
			s.warn("issue %d: hand-off comment: %v (the next sync retries)", it.Number, err)
		} else {
			commented = true
		}
	}
	edited := true
	if err := s.forge.Edit(ctx, set.Repo, it.Number, Edit{AddLabels: []string{set.HumanLabel}, AddAssignees: []string{set.Human}}); err != nil {
		s.warn("issue %d: hand-off: %v (the next sync retries)", it.Number, err)
		edited = false
	}
	confirmed := commented && edited
	err := s.store.Update(ctx, func(st *backlog.State) error {
		if cur, ok := st.Item(it.Number); ok && cur.Status == backlog.Human {
			cur.Commented = cur.Commented || commented
			cur.HandedOff = confirmed
		}
		return nil
	})
	if err != nil {
		s.warn("issue %d: %v", it.Number, err)
		return false
	}
	return confirmed
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

// Clean removes the worktrees of finished lanes: every earlier lane of an
// issue, and the last one of done and skipped issues (plus failed and human
// with all). It returns the worktrees removed (or, with
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
		dirs := slices.Clone(it.Past)
		if it.Status != backlog.Running && it.Status != backlog.Queued && slices.Contains(finished, it.Status) {
			dirs = append(dirs, it.Worktree)
		}
		for _, d := range dirs {
			if d == "" || !s.wt.Exists(d) {
				continue
			}
			if !dryRun {
				if err := s.wt.Remove(ctx, d); err != nil {
					s.warn("keep %s: %v", d, err)
					continue
				}
			}
			removed = append(removed, d)
		}
	}
	if dryRun || len(removed) == 0 {
		return removed, nil
	}
	return removed, s.store.Update(ctx, func(st *backlog.State) error {
		for i := range st.Items {
			it := &st.Items[i]
			if slices.Contains(removed, it.Worktree) {
				it.Worktree = ""
			}
			it.Past = slices.DeleteFunc(it.Past, func(d string) bool { return slices.Contains(removed, d) })
		}
		return nil
	})
}
