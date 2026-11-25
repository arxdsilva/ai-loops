package backlog

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/arxdsilva/ai-loops/internal/domain/backlog"
	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

// fakeStore keeps the state in memory.
type fakeStore struct {
	mu     sync.Mutex
	exists bool
	st     backlog.State
	held   map[int]bool
}

func (f *fakeStore) Exists() bool { f.mu.Lock(); defer f.mu.Unlock(); return f.exists }
func (f *fakeStore) Load(context.Context) (backlog.State, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return clone(f.st), nil
}
func (f *fakeStore) Update(_ context.Context, fn func(*backlog.State) error) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	st := clone(f.st)
	if err := fn(&st); err != nil {
		return err
	}
	f.st, f.exists = st, true
	return nil
}
func (f *fakeStore) LogPath(n int) string { return fmt.Sprintf("/logs/issue-%d.log", n) }
func (f *fakeStore) HoldLane(n int) (func(), error) {
	// Called under Update's lock.
	if f.held == nil {
		f.held = map[int]bool{}
	}
	f.held[n] = true
	return func() { f.mu.Lock(); delete(f.held, n); f.mu.Unlock() }, nil
}
func (f *fakeStore) LaneHeld(n int) bool { return f.held[n] }

func clone(st backlog.State) backlog.State {
	st.Items = slices.Clone(st.Items)
	st.Findings = slices.Clone(st.Findings)
	return st
}

// fakeForge records every call.
type fakeForge struct {
	mu       sync.Mutex
	open     []backlog.Issue
	created  []NewIssue
	comments map[int][]string
	edits    map[int][]Edit
	labels   []string
	failList error
	editErr  error
}

func (f *fakeForge) Repo(context.Context) (string, error) { return "o/r", nil }
func (f *fakeForge) DefaultBranch(context.Context, string) (string, error) {
	return "main", nil
}
func (f *fakeForge) User(context.Context) (string, error) { return "me", nil }
func (f *fakeForge) OpenIssues(context.Context, string) ([]backlog.Issue, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.open), f.failList
}
func (f *fakeForge) Create(_ context.Context, _ string, in NewIssue) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.created = append(f.created, in)
	return fmt.Sprintf("https://f/%d", 100+len(f.created)), nil
}
func (f *fakeForge) Comment(_ context.Context, _ string, n int, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.comments == nil {
		f.comments = map[int][]string{}
	}
	f.comments[n] = append(f.comments[n], body)
	return nil
}

// Edit records the change and applies it to the open issues, as the forge would.
func (f *fakeForge) Edit(_ context.Context, _ string, n int, e Edit) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.edits == nil {
		f.edits = map[int][]Edit{}
	}
	f.edits[n] = append(f.edits[n], e)
	for i := range f.open {
		is := &f.open[i]
		if is.Number != n {
			continue
		}
		is.Labels = append(slices.DeleteFunc(slices.Clone(is.Labels), func(l string) bool { return slices.Contains(e.RemoveLabels, l) }), e.AddLabels...)
		is.Assignees = append(slices.DeleteFunc(slices.Clone(is.Assignees), func(a string) bool { return slices.Contains(e.RemoveAssignees, a) }), e.AddAssignees...)
	}
	return f.editErr
}
func (f *fakeForge) EnsureLabel(_ context.Context, _, name, _ string) error {
	f.labels = append(f.labels, name)
	return nil
}

type fakeWT struct {
	mu      sync.Mutex
	added   []string
	removed []string
	failAdd error
	present map[string]bool
}

func (w *fakeWT) Add(_ context.Context, dir, _ string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.added = append(w.added, dir)
	return w.failAdd
}
func (w *fakeWT) Remove(_ context.Context, dir string) error {
	w.removed = append(w.removed, dir)
	return nil
}
func (w *fakeWT) Exists(dir string) bool { return w.present[dir] }

// fakeLanes returns the run each worktree "recorded"; fakeAgent decides it.
type fakeLanes struct {
	mu   sync.Mutex
	runs map[string]backlog.LaneRun
	refs map[string]string
}

func (l *fakeLanes) Latest(wt string) (backlog.LaneRun, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.runs[wt], nil
}
func (l *fakeLanes) Phase(wt string) string { return "implement" }
func (l *fakeLanes) ReadRef(_, rel string) (string, bool) {
	s, ok := l.refs[rel]
	return s, ok
}

type fakeAgent struct {
	lanes   *fakeLanes
	outcome func(dir string) (backlog.LaneRun, error)
	prompts []string
	mu      sync.Mutex
}

func (a *fakeAgent) Run(_ context.Context, dir, prompt, _ string, _ backlog.Settings) error {
	a.mu.Lock()
	a.prompts = append(a.prompts, prompt)
	a.mu.Unlock()
	lane, err := a.outcome(dir)
	a.lanes.mu.Lock()
	if a.lanes.runs == nil {
		a.lanes.runs = map[string]backlog.LaneRun{}
	}
	a.lanes.runs[dir] = lane
	a.lanes.mu.Unlock()
	return err
}

// at names every lane's worktree dir.
func at(dir string) func(int, int) string { return func(int, int) string { return dir } }

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, 5, 6, 7, 8, 9, 0, time.UTC) }

func events(outcome, pr string, extra ...run.Event) []run.Event {
	ev := []run.Event{{Kind: run.KindRun, Run: "r", Name: "t"}}
	ev = append(ev, extra...)
	if pr != "" {
		ev = append(ev, run.Event{Kind: run.KindPR, Ref: pr})
	}
	return append(ev, run.Event{Kind: run.KindEnd, Status: outcome})
}

type rig struct {
	svc   *Service
	store *fakeStore
	forge *fakeForge
	wt    *fakeWT
	lanes *fakeLanes
	agent *fakeAgent
	warns []string
}

func newRig(t *testing.T, open ...backlog.Issue) *rig {
	t.Helper()
	r := &rig{store: &fakeStore{}, forge: &fakeForge{open: open}, wt: &fakeWT{present: map[string]bool{}}, lanes: &fakeLanes{refs: map[string]string{}}}
	r.agent = &fakeAgent{lanes: r.lanes, outcome: func(string) (backlog.LaneRun, error) {
		return backlog.LaneRun{Dir: "/run", Events: events("done", "https://pr/1")}, nil
	}}
	r.svc = New(r.store, r.forge, r.wt, r.agent, r.lanes, fixedClock{})
	var mu sync.Mutex
	r.svc.Warn = func(f string, a ...any) { mu.Lock(); r.warns = append(r.warns, fmt.Sprintf(f, a...)); mu.Unlock() }
	return r
}

func (r *rig) init(t *testing.T, s backlog.Settings) backlog.Settings {
	t.Helper()
	if s.Human == "" {
		s.Human = "hum"
	}
	set, err := r.svc.Init(context.Background(), InitInput{Settings: s, MachineCap: 12, Worktrees: "/wt"})
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func is(n int, labels ...string) backlog.Issue {
	return backlog.Issue{Number: n, Title: fmt.Sprintf("issue %d", n), URL: fmt.Sprintf("https://i/%d", n), Author: "me", Labels: labels}
}

func TestInitDefaultsAndMerge(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	if _, err := r.svc.Sync(ctx); !errors.Is(err, ErrNoState) {
		t.Fatalf("sync before init: %v", err)
	}
	if _, err := r.svc.Init(ctx, InitInput{}); err == nil || !strings.Contains(err.Error(), "human") {
		t.Fatalf("init without a human: %v", err)
	}
	set := r.init(t, backlog.Settings{})
	if set.Repo != "o/r" || set.Base != "main" || !slices.Equal(set.Authors, []string{"me"}) ||
		set.Worktrees != "/wt" || set.MaxAgents != 6 || set.Lanes != 2 || !slices.Contains(set.Exclude, "wontfix") {
		t.Fatalf("defaults = %+v", set)
	}
	if !slices.Equal(r.forge.labels, []string{"loops-finding", "loops-needs-human", "loops-wip"}) {
		t.Fatalf("labels = %v", r.forge.labels)
	}
	// A second init changes only what it is given; more lanes re-split agents.
	set, err := r.svc.Init(ctx, InitInput{Settings: backlog.Settings{Lanes: 3, Budget: "5"}, MachineCap: 12})
	if err != nil {
		t.Fatal(err)
	}
	if set.Human != "hum" || set.Lanes != 3 || set.MaxAgents != 4 || set.Budget != "5" || !slices.Equal(set.Authors, []string{"me"}) {
		t.Fatalf("merged = %+v", set)
	}
	set, err = r.svc.Init(ctx, InitInput{AnyAuthor: true, MachineCap: 12})
	if err != nil || set.Authors != nil || set.MaxAgents != 4 {
		t.Fatalf("any author = %+v, %v", set, err)
	}
}

func TestWorkDone(t *testing.T) {
	r := newRig(t, is(4, "P2"), is(2, "P1"))
	r.init(t, backlog.Settings{})
	ctx := context.Background()
	if res, err := r.svc.Sync(ctx); err != nil || res.Queued != 2 {
		t.Fatalf("sync = %+v, %v", res, err)
	}
	res, err := r.svc.Work(ctx, 0)
	if err != nil || res.Status != backlog.Done || res.Item.Number != 2 || res.Item.PR != "https://pr/1" {
		t.Fatalf("work = %+v, %v", res, err)
	}
	if !slices.Equal(r.wt.added, []string{"/wt/issue-2-1"}) {
		t.Fatalf("worktrees = %v", r.wt.added)
	}
	p := r.agent.prompts[0]
	for _, w := range []string{"/loops:work o/r#2 --max-agents 6", "loops run ask none", "Skip step 0", "--name finding", "never merge", "--topic needs-human"} {
		if !strings.Contains(p, w) {
			t.Fatalf("prompt lacks %q:\n%s", w, p)
		}
	}
	e := r.forge.edits[2]
	if len(e) != 2 || !slices.Equal(e[0].AddLabels, []string{"loops-wip"}) || !slices.Equal(e[1].RemoveLabels, []string{"loops-wip"}) {
		t.Fatalf("claim label edits = %+v", e)
	}
	if r.store.LaneHeld(2) {
		t.Fatal("lane lock not released")
	}
	st, _ := r.store.Load(ctx)
	if it, _ := st.Item(2); it.Status != backlog.Done || it.RunDir != "/run" {
		t.Fatalf("state = %+v", it)
	}
}

func TestWorkEmptyAndUnknown(t *testing.T) {
	r := newRig(t)
	r.init(t, backlog.Settings{})
	ctx := context.Background()
	if res, err := r.svc.Work(ctx, 0); err != nil || !res.Empty {
		t.Fatalf("empty queue = %+v, %v", res, err)
	}
	if _, err := r.svc.Work(ctx, 9); err == nil {
		t.Fatal("unknown issue claimed")
	}
}

func TestWorkFailureRetriesThenHandsOff(t *testing.T) {
	r := newRig(t, is(7))
	r.init(t, backlog.Settings{})
	ctx := context.Background()
	r.svc.Sync(ctx)
	r.agent.outcome = func(string) (backlog.LaneRun, error) {
		return backlog.LaneRun{Dir: "/run", Events: events("failed", "",
			run.Event{Kind: run.KindDecision, Name: "qa-limit", Note: "which unit?"})}, nil
	}
	res, err := r.svc.Work(ctx, 0)
	if err != nil || res.Status != backlog.Queued || !strings.Contains(res.Reason, "which unit?") {
		t.Fatalf("first failure = %+v, %v", res, err)
	}
	if len(r.forge.comments[7]) != 0 {
		t.Fatal("a retry must not hand off")
	}
	res, _ = r.svc.Work(ctx, 0)
	if res.Status != backlog.Human || res.Item.Worktree != "/wt/issue-7-2" || !slices.Equal(res.Item.Past, []string{"/wt/issue-7-1"}) {
		t.Fatalf("second failure = %+v", res)
	}
	c := r.forge.comments[7]
	if len(c) != 1 || !strings.Contains(c[0], "@hum") || !strings.Contains(c[0], "which unit?") || !strings.Contains(c[0], "Attempts: 2 of 2") {
		t.Fatalf("hand-off comment = %q", c)
	}
	last := r.forge.edits[7][len(r.forge.edits[7])-1]
	if !slices.Equal(last.AddLabels, []string{"loops-needs-human"}) || !slices.Equal(last.AddAssignees, []string{"hum"}) {
		t.Fatalf("hand-off edit = %+v", last)
	}
}

func TestWorkNeedsHumanHandsOffAtOnce(t *testing.T) {
	r := newRig(t, is(8))
	r.init(t, backlog.Settings{Attempts: 3})
	ctx := context.Background()
	r.svc.Sync(ctx)
	r.agent.outcome = func(string) (backlog.LaneRun, error) {
		return backlog.LaneRun{Dir: "/run", Events: events("abandoned", "",
			run.Event{Kind: run.KindDecision, Name: backlog.NeedsHumanTopic, Note: "no spec"})}, nil
	}
	res, err := r.svc.Work(ctx, 0)
	if err != nil || res.Status != backlog.Human || res.Item.Attempts != 1 || len(r.forge.comments[8]) != 1 {
		t.Fatalf("needs-human = %+v, %v", res, err)
	}
	st, _ := r.store.Load(ctx)
	if it, _ := st.Item(8); !it.HandedOff {
		t.Fatal("confirmed hand-off not recorded")
	}
}

func TestHandoffNotConfirmedWhenEditFails(t *testing.T) {
	r := newRig(t, is(9))
	r.init(t, backlog.Settings{Attempts: 1})
	ctx := context.Background()
	r.svc.Sync(ctx)
	r.forge.editErr = errors.New("label refused")
	r.agent.outcome = func(string) (backlog.LaneRun, error) { return backlog.LaneRun{}, nil }
	res, _ := r.svc.Work(ctx, 0)
	st, _ := r.store.Load(ctx)
	if it, _ := st.Item(9); res.Status != backlog.Human || it.HandedOff {
		t.Fatalf("unconfirmed hand-off = %+v", it)
	}
	if len(r.warns) == 0 {
		t.Fatal("refused edit not warned")
	}
}

func TestWorkWorktreeAndAgentErrors(t *testing.T) {
	r := newRig(t, is(1), is(2))
	r.init(t, backlog.Settings{Attempts: 5})
	ctx := context.Background()
	r.svc.Sync(ctx)
	r.wt.failAdd = errors.New("disk full")
	res, err := r.svc.Work(ctx, 1)
	if err != nil || res.Status != backlog.Queued || !strings.Contains(res.Reason, "worktree: disk full") {
		t.Fatalf("worktree failure = %+v, %v", res, err)
	}
	if len(r.agent.prompts) != 0 {
		t.Fatal("agent ran without a worktree")
	}
	r.wt.failAdd = nil
	r.agent.outcome = func(string) (backlog.LaneRun, error) { return backlog.LaneRun{}, errors.New("exit status 2") }
	res, _ = r.svc.Work(ctx, 2)
	if !strings.Contains(res.Reason, "agent: exit status 2") {
		t.Fatalf("agent failure reason = %q", res.Reason)
	}
}

func TestWorkTimeout(t *testing.T) {
	r := newRig(t, is(1))
	r.init(t, backlog.Settings{Timeout: "1ms", Attempts: 3})
	ctx := context.Background()
	r.svc.Sync(ctx)
	r.svc.agent = agentFunc(func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() })
	res, err := r.svc.Work(ctx, 1)
	if err != nil || !strings.Contains(res.Reason, "agent stopped after the 1ms timeout") {
		t.Fatalf("timeout = %+v, %v", res, err)
	}
}

func TestWorkStoppedStillTidies(t *testing.T) {
	r := newRig(t, is(1))
	r.init(t, backlog.Settings{Attempts: 3})
	ctx, cancel := context.WithCancel(context.Background())
	r.svc.Sync(ctx)
	r.svc.agent = agentFunc(func(actx context.Context) error { cancel(); <-actx.Done(); return actx.Err() })
	res, err := r.svc.Work(ctx, 1)
	if err != nil || res.Status != backlog.Queued || !strings.Contains(res.Reason, "lane stopped") {
		t.Fatalf("stopped lane = %+v, %v", res, err)
	}
	e := r.forge.edits[1]
	if last := e[len(e)-1]; !slices.Equal(last.RemoveLabels, []string{"loops-wip"}) {
		t.Fatalf("claim label left on a stopped lane: %+v", e)
	}
}

type agentFunc func(ctx context.Context) error

func (f agentFunc) Run(ctx context.Context, _, _, _ string, _ backlog.Settings) error { return f(ctx) }

func TestFindingsFiledOnceAndDeduped(t *testing.T) {
	r := newRig(t, is(1), is(2), backlog.Issue{Number: 50, Title: "Known bug in parser", URL: "https://i/50", Assignees: []string{"x"}})
	r.init(t, backlog.Settings{})
	ctx := context.Background()
	r.svc.Sync(ctx)
	r.lanes.refs["f1.md"] = "details of the leak"
	r.agent.outcome = func(string) (backlog.LaneRun, error) {
		return backlog.LaneRun{Dir: "/run", PRBody: "## Open follow-ups\n- known bug in parser\n",
			Events: events("done", "https://pr/9", run.Event{Kind: run.KindNote, Name: "finding", Note: "Pool leaks", Ref: "f1.md"})}, nil
	}
	res, err := r.svc.Work(ctx, 1)
	if err != nil || len(res.Findings) != 2 {
		t.Fatalf("findings = %+v, %v", res.Findings, err)
	}
	if res.Findings[0].Duplicate || !res.Findings[1].Duplicate || res.Findings[1].URL != "https://i/50" {
		t.Fatalf("dedup = %+v", res.Findings)
	}
	c := r.forge.created
	if len(c) != 1 || c[0].Title != "Pool leaks" || !strings.Contains(c[0].Body, "details of the leak") ||
		!strings.Contains(c[0].Body, "#1 (https://pr/9)") || !slices.Equal(c[0].Assignees, []string{"hum"}) ||
		!slices.Equal(c[0].Labels, []string{"loops-finding"}) {
		t.Fatalf("created = %+v", c)
	}
	if len(r.forge.comments[1]) != 1 || !strings.Contains(r.forge.comments[1][0], "https://i/50") {
		t.Fatalf("duplicate comment = %v", r.forge.comments[1])
	}
	// The second lane finds the same leak: no second issue.
	res, _ = r.svc.Work(ctx, 2)
	if len(r.forge.created) != 1 || !res.Findings[0].Duplicate {
		t.Fatalf("second lane refiled: %+v", r.forge.created)
	}
}

func TestFileByHand(t *testing.T) {
	r := newRig(t)
	r.init(t, backlog.Settings{})
	ctx := context.Background()
	if _, err := r.svc.File(ctx, "  ", "", 0); err == nil {
		t.Fatal("empty title accepted")
	}
	f, err := r.svc.File(ctx, "Slow query", "body", 0)
	if err != nil || f.Duplicate || f.URL == "" {
		t.Fatalf("file = %+v, %v", f, err)
	}
	f, _ = r.svc.File(ctx, "slow  QUERY", "", 0)
	if !f.Duplicate {
		t.Fatal("hand-filed duplicate not caught")
	}
	r.forge.failList = errors.New("gh down")
	if _, err := r.svc.File(ctx, "x", "", 0); err == nil {
		t.Fatal("forge failure swallowed")
	}
}

func TestFinish(t *testing.T) {
	r := newRig(t, is(3))
	r.init(t, backlog.Settings{Attempts: 1})
	ctx := context.Background()
	r.svc.Sync(ctx)
	if _, err := r.svc.Finish(ctx, 3, backlog.Running, "", false); err == nil {
		t.Fatal("running set by hand")
	}
	it, err := r.svc.Finish(ctx, 3, backlog.Human, "needs a product call", false)
	if err != nil || it.Status != backlog.Human || len(r.forge.comments[3]) != 1 {
		t.Fatalf("hand-off = %+v, %v", it, err)
	}
	it, err = r.svc.Finish(ctx, 3, backlog.Queued, "", false)
	if err != nil || it.Status != backlog.Queued {
		t.Fatalf("re-queue (no attempts used) = %+v, %v", it, err)
	}
	last := r.forge.edits[3][len(r.forge.edits[3])-1]
	if !slices.Equal(last.RemoveLabels, []string{"loops-needs-human"}) || !slices.Equal(last.RemoveAssignees, []string{"hum"}) {
		t.Fatalf("re-queue edit = %+v", last)
	}
	// A live lane cannot be overruled.
	r.store.held = map[int]bool{3: true}
	r.store.Update(ctx, func(st *backlog.State) error { it, _ := st.Item(3); it.Status = backlog.Running; return nil })
	if _, err := r.svc.Finish(ctx, 3, backlog.Skipped, "", false); err == nil || !strings.Contains(err.Error(), "live lane") {
		t.Fatalf("live lane overruled: %v", err)
	}
}

func TestSyncRecoversDeadLanes(t *testing.T) {
	r := newRig(t, is(5))
	r.init(t, backlog.Settings{Attempts: 1})
	ctx := context.Background()
	r.svc.Sync(ctx)
	r.store.Update(ctx, func(st *backlog.State) error {
		_, err := st.Claim(5, at("/wt/issue-5"), time.Time{})
		return err
	})
	res, err := r.svc.Sync(ctx)
	if err != nil || res.Queued != 0 {
		t.Fatalf("sync = %+v, %v", res, err)
	}
	st, _ := r.store.Load(ctx)
	it, _ := st.Item(5)
	if it.Status != backlog.Human || !strings.Contains(it.Note, "lane process is gone") || len(r.forge.comments[5]) != 1 {
		t.Fatalf("dead lane = %+v", it)
	}
	if e := r.forge.edits[5]; len(e) == 0 || !slices.Equal(e[0].RemoveLabels, []string{"loops-wip"}) {
		t.Fatalf("dead lane's claim label not removed: %+v", e)
	}
}

func TestStatus(t *testing.T) {
	r := newRig(t, is(1), is(2), is(3, "P0"))
	r.init(t, backlog.Settings{})
	ctx := context.Background()
	if _, err := (newRig(t)).svc.Status(ctx); !errors.Is(err, ErrNoState) {
		t.Fatal("status without state")
	}
	r.svc.Sync(ctx)
	r.store.Update(ctx, func(st *backlog.State) error { _, err := st.Claim(2, at("/wt/issue-2"), time.Time{}); return err })
	rep, err := r.svc.Status(ctx)
	if err != nil || rep.Counts[backlog.Queued] != 2 || rep.Counts[backlog.Running] != 1 || rep.Synced == "" {
		t.Fatalf("status = %+v, %v", rep, err)
	}
	if len(rep.Running) != 1 || rep.Running[0].Phase != "implement" || rep.Running[0].Alive || rep.Running[0].Log != "/logs/issue-2.log" {
		t.Fatalf("running = %+v", rep.Running)
	}
	if rep.Next[0].Number != 3 {
		t.Fatalf("next = %+v", rep.Next)
	}
}

func TestClean(t *testing.T) {
	r := newRig(t)
	r.init(t, backlog.Settings{})
	ctx := context.Background()
	r.store.Update(ctx, func(st *backlog.State) error {
		st.Items = []backlog.Item{
			{Number: 1, Status: backlog.Done, Worktree: "/wt/issue-1"},
			{Number: 2, Status: backlog.Human, Worktree: "/wt/issue-2"},
			{Number: 3, Status: backlog.Running, Worktree: "/wt/issue-3"},
			{Number: 4, Status: backlog.Skipped, Worktree: "/wt/gone"},
			{Number: 5, Status: backlog.Queued, Worktree: "/wt/issue-5-2", Past: []string{"/wt/issue-5-1"}},
		}
		return nil
	})
	r.wt.present = map[string]bool{"/wt/issue-1": true, "/wt/issue-2": true, "/wt/issue-3": true, "/wt/issue-5-1": true, "/wt/issue-5-2": true}
	got, err := r.svc.Clean(ctx, false, true)
	if err != nil || !slices.Equal(got, []string{"/wt/issue-1", "/wt/issue-5-1"}) || len(r.wt.removed) != 0 {
		t.Fatalf("dry run = %v, %v (removed %v)", got, err, r.wt.removed)
	}
	got, _ = r.svc.Clean(ctx, true, false)
	if !slices.Equal(got, []string{"/wt/issue-1", "/wt/issue-2", "/wt/issue-5-1"}) || !slices.Equal(r.wt.removed, got) {
		t.Fatalf("clean --all = %v", got)
	}
	st, _ := r.store.Load(ctx)
	if it, _ := st.Item(1); it.Worktree != "" {
		t.Fatal("removed worktree still recorded")
	}
	if it, _ := st.Item(3); it.Worktree == "" {
		t.Fatal("a running lane's worktree was touched")
	}
	if it, _ := st.Item(5); it.Worktree != "/wt/issue-5-2" || len(it.Past) != 0 {
		t.Fatalf("queued issue's history = %+v", it)
	}
}

func TestRunDrainsTheQueue(t *testing.T) {
	r := newRig(t, is(1), is(2), is(3), is(4))
	r.init(t, backlog.Settings{Lanes: 2})
	ctx := context.Background()
	r.agent.outcome = func(dir string) (backlog.LaneRun, error) {
		if strings.Contains(dir, "issue-3-") {
			return backlog.LaneRun{Dir: "/run", Events: events("failed", "")}, nil
		}
		return backlog.LaneRun{Dir: "/run", Events: events("done", "https://pr/"+dir)}, nil
	}
	var out bytes.Buffer
	sum, err := r.svc.Run(ctx, RunInput{Out: &out})
	if err != nil {
		t.Fatalf("run: %v\n%s", err, out.String())
	}
	// 1, 2, 4 done; 3 fails twice (retry, then hand-off).
	if sum.Started != 5 || sum.Done != 3 || sum.Retried != 1 || sum.Human != 1 {
		t.Fatalf("summary = %+v\n%s", sum, out.String())
	}
	if !strings.Contains(out.String(), "#3 handed to the human") {
		t.Fatalf("output:\n%s", out.String())
	}
}

func TestRunMaxAndErrors(t *testing.T) {
	r := newRig(t, is(1), is(2), is(3))
	r.init(t, backlog.Settings{Lanes: 1})
	ctx := context.Background()
	sum, err := r.svc.Run(ctx, RunInput{Max: 2})
	if err != nil || sum.Started != 2 || sum.Done != 2 {
		t.Fatalf("max run = %+v, %v", sum, err)
	}
	r.forge.failList = errors.New("gh down")
	if _, err := r.svc.Run(ctx, RunInput{}); err == nil {
		t.Fatal("sync failure swallowed")
	}
}
