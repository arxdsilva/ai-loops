package backlog

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/arxdsilva/ai-loops/internal/domain/backlog"
)

// NextShown is how many queued issues Status lists.
const NextShown = 5

// ErrNoState means `loops backlog init` has not run.
var ErrNoState = errors.New("no backlog here: run `loops backlog init --human <login>` first")

// Service runs the backlog use cases.
type Service struct {
	store Store
	forge Forge
	prs   PullRequests
	wt    Worktrees
	agent Agent
	lanes Lanes
	clock Clock
	// Warn receives best-effort failures (a label the forge refused); nil
	// drops them.
	Warn func(format string, args ...any)
}

// New wires the service.
func New(store Store, forge Forge, prs PullRequests, wt Worktrees, agent Agent, lanes Lanes, clock Clock) *Service {
	return &Service{store: store, forge: forge, prs: prs, wt: wt, agent: agent, lanes: lanes, clock: clock}
}

func (s *Service) warn(format string, args ...any) {
	if s.Warn != nil {
		s.Warn(format, args...)
	}
}

// InitInput is `loops backlog init`: zero fields keep the current setting
// (or take the default on a first init).
type InitInput struct {
	Settings   backlog.Settings
	AnyAuthor  bool   // --authors any: lift the author filter
	MachineCap int    // `loops hw`'s parallel agent cap, split between lanes
	Worktrees  string // default worktrees dir
}

// Init writes the settings, keeping the queue, and makes sure the forge has
// the backlog's labels.
func (s *Service) Init(ctx context.Context, in InitInput) (backlog.Settings, error) {
	var cur backlog.Settings
	if s.store.Exists() {
		st, err := s.store.Load(ctx)
		if err != nil {
			return cur, err
		}
		cur = st.Settings
	}
	set := merge(cur, in.Settings)
	if in.AnyAuthor {
		set.Authors = nil
	}
	var err error
	if set.Repo == "" {
		if set.Repo, err = s.forge.Repo(ctx); err != nil {
			return set, fmt.Errorf("repo: %w (pass --repo owner/name)", err)
		}
	}
	if set.Base == "" {
		if set.Base, err = s.forge.DefaultBranch(ctx, set.Repo); err != nil {
			return set, fmt.Errorf("base branch: %w (pass --base)", err)
		}
	}
	if len(set.Authors) == 0 && !in.AnyAuthor && cur.Repo == "" {
		u, err := s.forge.User(ctx)
		if err != nil {
			return set, fmt.Errorf("forge user: %w (pass --authors a,b or --authors any)", err)
		}
		set.Authors = []string{u}
	}
	if set.Exclude == nil {
		set.Exclude = slices.Clone(backlog.DefaultExclude)
	}
	if set.Worktrees == "" {
		set.Worktrees = in.Worktrees
	}
	if in.Settings.MaxAgents == 0 && (cur.MaxAgents == 0 || in.Settings.Lanes != 0) {
		lanes := set.Lanes
		if lanes == 0 {
			lanes = backlog.DefaultLanes
		}
		set.MaxAgents = backlog.LaneAgents(in.MachineCap, lanes)
	}
	if set, err = set.Normalize(); err != nil {
		return set, err
	}
	if set, err = s.fitMachine(set, in, cur.Repo == ""); err != nil {
		return set, err
	}
	for _, l := range [][2]string{
		{set.FindingLabel, "Found by a loops backlog lane; for a human to triage"},
		{set.HumanLabel, "A loops backlog lane could not finish it; a human decides"},
		{set.ClaimLabel, "A loops backlog lane is working on it"},
	} {
		if err := s.forge.EnsureLabel(ctx, set.Repo, l[0], l[1]); err != nil {
			s.warn("label %s: %v", l[0], err)
		}
	}
	err = s.store.Update(ctx, func(st *backlog.State) error {
		st.Version, st.Settings = backlog.Version, set
		return nil
	})
	return set, err
}

// fitMachine keeps lanes × agents per lane within the machine's cap
// (BACKLOG §7): it lowers a lane count the user did not give, and refuses
// one they did. One lane always fits. A later init that changes neither
// keeps them: the cap is probed under the load of running lanes.
func (s *Service) fitMachine(set backlog.Settings, in InitInput, first bool) (backlog.Settings, error) {
	given := in.Settings.Lanes != 0 || in.Settings.MaxAgents != 0
	if (!first && !given) || in.MachineCap <= 0 || set.Lanes <= 1 || set.Lanes*set.MaxAgents <= in.MachineCap {
		return set, nil
	}
	if given {
		return set, fmt.Errorf("%d lanes × %d agents = %d agents, over this machine's cap of %d (loops hw): lower --lanes or --max-agents",
			set.Lanes, set.MaxAgents, set.Lanes*set.MaxAgents, in.MachineCap)
	}
	lanes := max(1, in.MachineCap/set.MaxAgents)
	s.warn("lanes lowered from %d to %d: %d agents each fit this machine's cap of %d (loops hw)", set.Lanes, lanes, set.MaxAgents, in.MachineCap)
	set.Lanes = lanes
	return set, nil
}

// merge returns cur with every non-zero field of next applied.
func merge(cur, next backlog.Settings) backlog.Settings {
	str := func(dst *string, v string) {
		if v != "" {
			*dst = v
		}
	}
	num := func(dst *int, v int) {
		if v != 0 {
			*dst = v
		}
	}
	list := func(dst *[]string, v []string) {
		if v != nil {
			*dst = v
		}
	}
	str(&cur.Repo, next.Repo)
	str(&cur.Human, next.Human)
	str(&cur.Base, next.Base)
	str(&cur.Timeout, next.Timeout)
	str(&cur.Agent, next.Agent)
	str(&cur.Budget, next.Budget)
	str(&cur.Worktrees, next.Worktrees)
	str(&cur.FindingLabel, next.FindingLabel)
	str(&cur.HumanLabel, next.HumanLabel)
	str(&cur.ClaimLabel, next.ClaimLabel)
	num(&cur.Lanes, next.Lanes)
	num(&cur.MaxAgents, next.MaxAgents)
	num(&cur.Attempts, next.Attempts)
	num(&cur.MaxPRs, next.MaxPRs)
	list(&cur.Labels, next.Labels)
	list(&cur.Exclude, next.Exclude)
	list(&cur.Authors, next.Authors)
	return cur
}

func (s *Service) load(ctx context.Context) (backlog.State, error) {
	if !s.store.Exists() {
		return backlog.State{}, ErrNoState
	}
	return s.store.Load(ctx)
}

// Sync re-queues lanes whose process is gone, retries unconfirmed
// hand-offs, rebuilds the queue from the forge's open issues (BACKLOG §2),
// and files the findings still pending (BACKLOG §5).
func (s *Service) Sync(ctx context.Context) (backlog.SyncResult, error) {
	st, err := s.load(ctx)
	if err != nil {
		return backlog.SyncResult{}, err
	}
	set := st.Settings
	// Recover first: a dead lane's hand-off changes the issue's labels.
	handed, err := s.recover(ctx, set)
	if err != nil {
		return backlog.SyncResult{}, err
	}
	open, err := s.forge.OpenIssues(ctx, set.Repo)
	if err != nil {
		return backlog.SyncResult{}, err
	}
	since := s.clock.Now()
	var prs []string
	counted := false
	if set.PRCap() > 0 {
		if prs, err = s.prs.OpenPRs(ctx, set.Repo); err != nil {
			s.warn("open PRs not counted: %v", err)
		} else {
			counted = true
		}
	}
	var res backlog.SyncResult
	err = s.store.Update(ctx, func(st *backlog.State) error {
		res = st.Sync(open, s.clock.Now())
		if counted {
			res.Gate = st.Gate(prs, since)
		}
		return nil
	})
	if err != nil {
		return res, err
	}
	res.Handed = handed
	res.Filed = len(s.flush(ctx, set, 0))
	return res, nil
}

// recover finishes running lanes whose process died (no lane lock held),
// keeping their findings, then retries every hand-off the forge has not
// confirmed. It returns the hand-offs confirmed.
func (s *Service) recover(ctx context.Context, set backlog.Settings) (int, error) {
	st, err := s.store.Load(ctx)
	if err != nil {
		return 0, err
	}
	for _, n := range st.Stale(func(it backlog.Item) bool { return s.store.LaneHeld(it.Number) }) {
		it, _ := st.Item(n)
		if err := s.recoverLane(ctx, set, *it); err != nil {
			return 0, err
		}
	}
	if st, err = s.store.Load(ctx); err != nil {
		return 0, err
	}
	handed := 0
	for _, it := range st.Unconfirmed() {
		if s.retryHandoff(ctx, set, it.Number) {
			handed++
		}
	}
	return handed, nil
}

// recoverLane records the outcome of seen, a running lane whose process is
// gone. It takes the issue's lane lock first, so one sync owns it, and
// acts only while the item is still that lane (same lane count): another
// sync may have finished it and a new lane started meanwhile. The claim
// label goes while the issue is still running, so a lane that claims it
// once it is re-queued never loses its own label; the outcome and the
// lane's findings are saved together (BACKLOG §5). A run that cannot be
// read leaves the lane running, for the next sync to read again.
func (s *Service) recoverLane(ctx context.Context, set backlog.Settings, seen backlog.Item) error {
	lease, err := s.store.HoldLane(seen.Number)
	if err != nil {
		return nil // alive again, or another sync is recovering it
	}
	defer lease.Release()
	same := func(st *backlog.State) (*backlog.Item, bool) {
		it, ok := st.Item(seen.Number)
		return it, ok && it.Status == backlog.Running && it.Lanes == seen.Lanes && it.Worktree == seen.Worktree
	}
	st, err := s.store.Load(ctx)
	if err != nil {
		return err
	}
	if _, ok := same(&st); !ok {
		return nil
	}
	lane, err := s.lanes.Latest(seen.Worktree)
	if err != nil {
		s.warn("issue %d: lane run not readable, left running for the next sync: %v", seen.Number, err)
		return nil
	}
	s.edit(ctx, set, seen.Number, Edit{RemoveLabels: []string{set.ClaimLabel}})
	out := lane.Outcome("the lane process is gone")
	pending := s.pending(seen.Number, cmp.Or(out.PR, out.Opened), lane)
	return s.store.Update(ctx, func(st *backlog.State) error {
		if _, ok := same(st); !ok {
			return nil
		}
		if _, err := st.Finish(seen.Number, out, s.clock.Now()); err == nil {
			st.AddPending(pending...)
		}
		return nil
	})
}

// retryHandoff retries issue n's unconfirmed hand-off under its lane lock,
// so it has one owner: a live lane, a `loops backlog finish` or another
// sync handing it off holds the lock, and this sync leaves it to them. The
// item is read again under the lock, so a hand-off confirmed meanwhile is
// not repeated. It reports whether the hand-off was confirmed.
func (s *Service) retryHandoff(ctx context.Context, set backlog.Settings, n int) bool {
	lease, err := s.store.HoldLane(n)
	if err != nil {
		return false
	}
	defer lease.Release()
	st, err := s.store.Load(ctx)
	if err != nil {
		return false
	}
	it, ok := st.Item(n)
	if !ok || it.Status != backlog.Human || it.HandedOff {
		return false
	}
	return s.handoff(ctx, set, *it)
}

// Report is `loops backlog status`.
type Report struct {
	Settings backlog.Settings
	Synced   string
	Counts   map[backlog.Status]int
	Running  []RunningLane
	Next     []backlog.Item // the first queued issues, in lane order
	Findings int
}

// RunningLane is one lane in flight.
type RunningLane struct {
	Item  backlog.Item
	Phase string // its live run's phase ("" = no run yet)
	Log   string
	Alive bool // its process still holds the lane lock
}

// Status reads the state and the live lanes. It writes nothing.
func (s *Service) Status(ctx context.Context) (Report, error) {
	st, err := s.load(ctx)
	if err != nil {
		return Report{}, err
	}
	r := Report{Settings: st.Settings, Counts: map[backlog.Status]int{}, Findings: len(st.Findings)}
	if !st.Synced.IsZero() {
		r.Synced = st.Synced.Format("2006-01-02 15:04 MST")
	}
	for _, it := range st.Items {
		r.Counts[it.Status]++
		if it.Status == backlog.Running {
			r.Running = append(r.Running, RunningLane{Item: it, Phase: s.lanes.Phase(it.Worktree),
				Log: s.store.LogPath(it.Number), Alive: s.store.LaneHeld(it.Number)})
		}
	}
	q := st.Queue()
	r.Next = q[:min(len(q), NextShown)]
	return r, nil
}

// worktree is the directory of issue n's lane-th lane.
func worktree(set backlog.Settings, n, lane int) string {
	return filepath.Join(set.Worktrees, fmt.Sprintf("issue-%d-%d", n, lane))
}

// Prompt is the lane agent's prompt (BACKLOG §4).
func Prompt(set backlog.Settings, n int) string {
	return fmt.Sprintf(`/loops:work %s#%d --max-agents %d

This is an unattended backlog lane (specs/BACKLOG.md §4). Nobody will answer: never ask the user, never wait for one.
- Skip step 0 (the update): the supervisor owns it.
- At step 6 record loops run ask none, whatever the config says. Every checkpoint is auto; decide every open point and record it.
- If the ticket cannot be worked without a human (missing information, a product decision, a contradiction, an unauthenticated connector), record loops run decide --topic needs-human --note "<what is missing>", then end with loops run end --outcome abandoned.
- Record every finding outside the charter the moment you find it: loops run event --kind note --name finding --note "<one-line title>" (details in a run-dir file passed as --ref), and list it under pr.md Open follow-ups. Never create or comment on issues yourself.
- Put the line "Backlog lane: yes" in every spawn prompt, the reflector's included: it records harness issues as findings instead of filing them.
- Open the PR; never merge it.
- Write repository files with the Edit and Write tools, not through Bash, so the run log records them; tell every agent you spawn the same.`,
		set.Repo, n, set.MaxAgents)
}
