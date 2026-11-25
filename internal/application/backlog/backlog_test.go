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
	hm     sync.Mutex // guards held (LaneHeld runs inside Update too)
	// failSave fails the save after fn ran (the state is not written).
	failSave error
	// onUpdate runs under the lock before fn (another process's change),
	// onSave sees each state saved, and onHold runs when a lane lock is
	// taken (outside the state lock only).
	onUpdate func(*backlog.State)
	failNext error // fails the next save only
	onSave   func(backlog.State)
	onHold   func(n int)
}

func (f *fakeStore) Exists() bool { f.mu.Lock(); defer f.mu.Unlock(); return f.exists }
func (f *fakeStore) Load(context.Context) (backlog.State, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return clone(f.st), nil
}

// Update fails on a cancelled ctx, as a real lock wait would.
func (f *fakeStore) Update(ctx context.Context, fn func(*backlog.State) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.onUpdate != nil {
		f.onUpdate(&f.st)
	}
	st := clone(f.st)
	if err := fn(&st); err != nil {
		return err
	}
	if f.failSave != nil {
		return f.failSave
	}
	if err := f.failNext; err != nil {
		f.failNext = nil
		return err
	}
	f.st, f.exists = st, true
	if f.onSave != nil {
		f.onSave(clone(st))
	}
	return nil
}
func (f *fakeStore) LogPath(n int) string { return fmt.Sprintf("/logs/issue-%d.log", n) }

// fakeLease releases a fake lane lock once.
type fakeLease struct {
	once sync.Once
	fn   func()
}

func (l *fakeLease) Release() { l.once.Do(l.fn) }

func (f *fakeStore) HoldLane(n int) (Lease, error) {
	if f.onHold != nil {
		f.onHold(n)
	}
	f.hm.Lock()
	defer f.hm.Unlock()
	if f.held == nil {
		f.held = map[int]bool{}
	}
	if f.held[n] {
		return nil, fmt.Errorf("issue %d lane is held", n)
	}
	f.held[n] = true
	return &fakeLease{fn: func() { f.hm.Lock(); delete(f.held, n); f.hm.Unlock() }}, nil
}
func (f *fakeStore) LaneHeld(n int) bool { f.hm.Lock(); defer f.hm.Unlock(); return f.held[n] }

func clone(st backlog.State) backlog.State {
	st.Items = slices.Clone(st.Items)
	st.Findings = slices.Clone(st.Findings)
	st.Pending = slices.Clone(st.Pending)
	return st
}

// fakeForge records every call.
type fakeForge struct {
	mu                      sync.Mutex
	open                    []backlog.Issue
	created                 []NewIssue
	comments                map[int][]string
	edits                   map[int][]Edit
	labels                  []string
	failList                error
	editErr                 error
	prs                     []string // open PR URLs
	failPRs                 error
	failPRsAt, prCalls      int // failPRsAt fails that listing only (1 = the first)
	prRepos                 []string
	onPRs                   func(call int)
	failCreate, failComment error
	onComment               func(n int)
	// editFail fails an edit before it changes anything; editErr after.
	editFail error
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
func (f *fakeForge) OpenPRs(_ context.Context, repo string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.prRepos = append(f.prRepos, repo)
	if f.prCalls++; f.onPRs != nil {
		f.onPRs(f.prCalls)
	}
	if f.prCalls == f.failPRsAt {
		return nil, errors.New("gh down")
	}
	return slices.Clone(f.prs), f.failPRs
}

// Create records the issue and lists it as open, as the forge would.
func (f *fakeForge) Create(_ context.Context, _ string, in NewIssue) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failCreate != nil {
		return "", f.failCreate
	}
	f.created = append(f.created, in)
	url := fmt.Sprintf("https://f/%d", 100+len(f.created))
	f.open = append(f.open, backlog.Issue{Number: 100 + len(f.created), Title: in.Title, URL: url, Labels: in.Labels, Assignees: in.Assignees})
	return url, nil
}
func (f *fakeForge) Comment(_ context.Context, _ string, n int, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.onComment != nil {
		f.onComment(n)
	}
	if f.failComment != nil {
		return f.failComment
	}
	if f.comments == nil {
		f.comments = map[int][]string{}
	}
	f.comments[n] = append(f.comments[n], body)
	return nil
}

// Edit records the change and applies it to the open issues, as the forge
// would; a cancelled ctx fails it, as it would fail gh.
func (f *fakeForge) Edit(ctx context.Context, _ string, n int, e Edit) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.editFail != nil {
		return f.editFail
	}
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
	mu         sync.Mutex
	runs       map[string]backlog.LaneRun
	refs       map[string]string
	failLatest error
}

func (l *fakeLanes) Latest(wt string) (backlog.LaneRun, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.runs[wt], l.failLatest
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

func (a *fakeAgent) Run(_ context.Context, dir, prompt, _ string, _ backlog.Settings, _ Lease) error {
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

// fixedClock never moves; Sleep runs sleep (when set) instead of waiting.
type fixedClock struct{ sleep func(time.Duration) }

func (fixedClock) Now() time.Time { return time.Date(2026, 5, 6, 7, 8, 9, 0, time.UTC) }
func (c fixedClock) Sleep(ctx context.Context, d time.Duration) error {
	if c.sleep != nil {
		c.sleep(d)
	}
	return ctx.Err()
}

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
	r.svc = New(r.store, r.forge, r.forge, r.wt, r.agent, r.lanes, fixedClock{})
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
	for _, w := range []string{"/loops:work o/r#2 --max-agents 6", "loops run ask none", "Skip step 0", "--name finding", "never merge", "--topic needs-human", "Backlog lane: yes"} {
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
	// The fakes refuse a cancelled ctx, so this outcome was saved on a live one.
	st, _ := r.store.Load(context.Background())
	if it, _ := st.Item(1); it.Status != backlog.Queued || !strings.Contains(it.Note, "lane stopped") || r.store.LaneHeld(1) {
		t.Fatalf("stopped lane's saved state = %+v", it)
	}
}

func TestWorkClaimSaveFailureReleasesTheLane(t *testing.T) {
	r := newRig(t, is(1))
	r.init(t, backlog.Settings{})
	ctx := context.Background()
	r.svc.Sync(ctx)
	r.store.failSave = errors.New("disk full")
	if _, err := r.svc.Work(ctx, 0); err == nil {
		t.Fatal("save failure swallowed")
	}
	if r.store.LaneHeld(1) {
		t.Fatal("lane lock kept after the claim was not saved")
	}
}

type agentFunc func(ctx context.Context) error

func (f agentFunc) Run(ctx context.Context, _, _, _ string, _ backlog.Settings, _ Lease) error {
	return f(ctx)
}

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
	r.store.hm.Lock()
	r.store.held = map[int]bool{3: true}
	r.store.hm.Unlock()
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

// prOutcome makes every lane open the next PR and lists it as open, as the
// forge would.
func prOutcome(r *rig) func(string) (backlog.LaneRun, error) {
	n := 10
	return func(string) (backlog.LaneRun, error) {
		r.forge.mu.Lock()
		defer r.forge.mu.Unlock()
		n++
		url := fmt.Sprintf("https://github.com/o/r/pull/%d", n)
		r.forge.prs = append(r.forge.prs, url)
		return backlog.LaneRun{Dir: "/run", Events: events("done", url)}, nil
	}
}

func TestWorkGatedAtOpenPRCap(t *testing.T) {
	r := newRig(t, is(1), is(2), is(3))
	if set := r.init(t, backlog.Settings{MaxPRs: 2}); set.MaxPRs != 2 {
		t.Fatalf("max prs = %d", set.MaxPRs)
	}
	ctx := context.Background()
	r.svc.Sync(ctx)
	r.agent.outcome = prOutcome(r)
	for _, n := range []int{1, 2} {
		if res, err := r.svc.Work(ctx, 0); err != nil || res.Status != backlog.Done || res.Item.Number != n {
			t.Fatalf("lane %d = %+v, %v", n, res, err)
		}
	}
	res, err := r.svc.Work(ctx, 0)
	if err != nil || !res.Gated || res.Gate != (backlog.Gate{Open: 2, Cap: 2}) || len(r.wt.added) != 2 {
		t.Fatalf("at the cap = %+v, %v (worktrees %v)", res, err, r.wt.added)
	}
	st, _ := r.store.Load(ctx)
	if it, _ := st.Item(3); it.Status != backlog.Queued || it.Attempts != 0 {
		t.Fatalf("gated issue claimed: %+v", it)
	}
	if sync, _ := r.svc.Sync(ctx); sync.Gate != (backlog.Gate{Open: 2, Cap: 2}) {
		t.Fatalf("sync gate = %+v", sync.Gate)
	}
	// The forge cannot list PRs: no lane starts (the cap fails closed).
	r.forge.failPRs = errors.New("gh down")
	if _, err := r.svc.Work(ctx, 0); err == nil || !strings.Contains(err.Error(), "--max-prs") {
		t.Fatalf("PR listing failure = %v", err)
	}
	if sync, err := r.svc.Sync(ctx); err != nil || sync.Gate.Cap != 0 {
		t.Fatalf("sync with no PR list = %+v, %v", sync, err)
	}
	// A PR merged: the lane starts.
	r.forge.failPRs = nil
	r.forge.prs = r.forge.prs[1:]
	if res, err := r.svc.Work(ctx, 0); err != nil || res.Status != backlog.Done || res.Item.Number != 3 {
		t.Fatalf("after a merge = %+v, %v", res, err)
	}
}

func TestWorkNoPRCap(t *testing.T) {
	r := newRig(t, is(1))
	r.init(t, backlog.Settings{MaxPRs: -1})
	ctx := context.Background()
	r.svc.Sync(ctx)
	r.forge.failPRs = errors.New("never listed")
	if res, err := r.svc.Work(ctx, 0); err != nil || res.Status != backlog.Done {
		t.Fatalf("no cap = %+v, %v", res, err)
	}
}

// Parallel claims count each other's running lanes, so they never pass the cap.
func TestWorkGateHoldsUnderParallelClaims(t *testing.T) {
	r := newRig(t, is(1), is(2))
	r.init(t, backlog.Settings{MaxPRs: 1})
	ctx := context.Background()
	r.svc.Sync(ctx)
	// A regression starts both lanes: it must fail, not hang on release.
	release, entered := make(chan struct{}), make(chan struct{}, 2)
	var once sync.Once
	free := func() { once.Do(func() { close(release) }) }
	t.Cleanup(free)
	r.svc.agent = agentFunc(func(context.Context) error { entered <- struct{}{}; <-release; return nil })
	results := make(chan LaneResult, 2)
	for range 2 {
		go func() {
			res, err := r.svc.Work(ctx, 0)
			if err != nil {
				t.Error(err)
			}
			results <- res
		}()
	}
	var first LaneResult
	for lanes, got := 0, false; !got; {
		select {
		case <-entered:
			if lanes++; lanes == 2 {
				t.Fatal("two lanes started under a cap of 1")
			}
		case first = <-results:
			got = true
		}
	}
	if !first.Gated || first.Gate != (backlog.Gate{Open: 1, Cap: 1}) {
		t.Fatalf("first to end = %+v, want the gated one", first)
	}
	free()
	if second := <-results; second.Gated || second.Item.Number != 1 {
		t.Fatalf("lane = %+v", second)
	}
}

func TestRunPausesAtCap(t *testing.T) {
	r := newRig(t, is(1), is(2))
	r.init(t, backlog.Settings{Lanes: 1, MaxPRs: 1})
	ctx := context.Background()
	r.agent.outcome = prOutcome(r)
	var out bytes.Buffer
	sum, err := r.svc.Run(ctx, RunInput{Out: &out})
	if err != nil || !sum.Gated || sum.Done != 1 || sum.Started != 1 || !strings.Contains(out.String(), "paused: 1 of 1 lane PRs open") {
		t.Fatalf("run at the cap = %+v, %v\n%s", sum, err, out.String())
	}
	// With --wait it sleeps until a PR is merged, works the rest, then
	// keeps watching the empty queue until stopped.
	var waits []time.Duration
	wctx, stop := context.WithCancel(ctx)
	defer stop()
	r.svc.clock = fixedClock{sleep: func(d time.Duration) {
		waits = append(waits, d)
		r.forge.mu.Lock()
		r.forge.prs = nil
		r.forge.mu.Unlock()
		if len(waits) == 2 {
			stop()
		}
	}}
	out.Reset()
	sum, err = r.svc.Run(wctx, RunInput{Out: &out, Wait: time.Minute})
	if !errors.Is(err, context.Canceled) || sum.Gated || sum.Done != 1 || !slices.Equal(waits, []time.Duration{time.Minute, time.Minute}) ||
		!strings.Contains(out.String(), "idle: nothing queued; checking again in 1m0s") {
		t.Fatalf("run --wait = %+v, %v, waits %v\n%s", sum, err, waits, out.String())
	}
	// Stopped while paused.
	r2 := newRig(t, is(1))
	r2.init(t, backlog.Settings{MaxPRs: 1})
	r2.forge.prs = []string{"https://github.com/o/r/pull/5"}
	r2.store.Update(ctx, func(st *backlog.State) error {
		st.Items = append(st.Items, backlog.Item{Number: 9, Status: backlog.Done, PR: "https://github.com/o/r/pull/5/"})
		return nil
	})
	cctx, cancel := context.WithCancel(ctx)
	r2.svc.clock = fixedClock{sleep: func(time.Duration) { cancel() }}
	if _, err := r2.svc.Run(cctx, RunInput{Wait: time.Hour}); !errors.Is(err, context.Canceled) {
		t.Fatalf("stopped while paused: %v", err)
	}
}

func TestInitFitsTheMachine(t *testing.T) {
	ctx := context.Background()
	r := newRig(t)
	// Default lanes on a small machine: lowered to fit, with a warning.
	set, err := r.svc.Init(ctx, InitInput{Settings: backlog.Settings{Human: "hum"}, MachineCap: 3, Worktrees: "/wt"})
	if err != nil || set.Lanes != 1 || set.MaxAgents != 2 || len(r.warns) != 1 || !strings.Contains(r.warns[0], "lanes lowered from 2 to 1") {
		t.Fatalf("small machine = %+v, %v, %v", set, err, r.warns)
	}
	// Lanes the user asked for that do not fit: refused, nothing saved.
	if _, err := r.svc.Init(ctx, InitInput{Settings: backlog.Settings{Lanes: 3}, MachineCap: 4}); err == nil || !strings.Contains(err.Error(), "over this machine's cap of 4") {
		t.Fatalf("explicit lanes over the cap: %v", err)
	}
	if _, err := r.svc.Init(ctx, InitInput{Settings: backlog.Settings{Lanes: 2, MaxAgents: 5}, MachineCap: 8}); err == nil {
		t.Fatal("explicit agents over the cap accepted")
	}
	if st, _ := r.store.Load(ctx); st.Settings.Lanes != 1 {
		t.Fatalf("refused init saved: %+v", st.Settings)
	}
	// One lane always fits, even over the cap.
	if set, err := r.svc.Init(ctx, InitInput{Settings: backlog.Settings{Lanes: 1, MaxAgents: 9}, MachineCap: 4}); err != nil || set.MaxAgents != 9 {
		t.Fatalf("one lane = %+v, %v", set, err)
	}
}

func TestFindingsStayPendingUntilFiled(t *testing.T) {
	r := newRig(t, is(1))
	r.init(t, backlog.Settings{})
	ctx := context.Background()
	r.svc.Sync(ctx)
	r.agent.outcome = func(string) (backlog.LaneRun, error) {
		return backlog.LaneRun{Dir: "/run", Events: events("done", "https://pr/1", run.Event{Kind: run.KindNote, Name: "finding", Note: "Pool leaks"})}, nil
	}
	r.forge.failCreate = errors.New("gh down")
	res, err := r.svc.Work(ctx, 0)
	if err != nil || res.Status != backlog.Done || len(res.Findings) != 0 {
		t.Fatalf("lane = %+v, %v", res, err)
	}
	st, _ := r.store.Load(ctx)
	if len(st.Pending) != 1 || st.Pending[0].From != 1 || !strings.Contains(st.Pending[0].Body, "#1 (https://pr/1)") {
		t.Fatalf("pending = %+v", st.Pending)
	}
	// The forge is back: the next sync files it, once.
	r.forge.failCreate = nil
	if sync, err := r.svc.Sync(ctx); err != nil || sync.Filed != 1 {
		t.Fatalf("sync = %+v, %v", sync, err)
	}
	if sync, _ := r.svc.Sync(ctx); sync.Filed != 0 || len(r.forge.created) != 1 {
		t.Fatalf("refiled: %+v, %+v", sync, r.forge.created)
	}
	st, _ = r.store.Load(ctx)
	if len(st.Pending) != 0 || len(st.Findings) != 1 {
		t.Fatalf("state = pending %+v, findings %+v", st.Pending, st.Findings)
	}
	// Another process already filed it: not filed again.
	if _, err := r.svc.file(ctx, st.Settings, "Pool leaks", "", 1, true); !errors.Is(err, errNotPending) {
		t.Fatalf("filed a finding no longer pending: %v", err)
	}
}

// A finding whose issue exists on the forge but not in the state (an
// earlier save failed) is matched by the listing taken under the lock.
func TestFindingMatchesAnIssueCreatedElsewhere(t *testing.T) {
	r := newRig(t)
	r.init(t, backlog.Settings{})
	ctx := context.Background()
	r.forge.open = append(r.forge.open, backlog.Issue{Number: 77, Title: "Pool leaks", URL: "https://i/77", Assignees: []string{"hum"}})
	f, err := r.svc.File(ctx, "pool LEAKS", "", 0)
	if err != nil || !f.Duplicate || f.URL != "https://i/77" || len(r.forge.created) != 0 {
		t.Fatalf("file = %+v, %v", f, err)
	}
}

func TestSyncFilesADeadLanesFindings(t *testing.T) {
	r := newRig(t, is(5))
	r.init(t, backlog.Settings{})
	ctx := context.Background()
	r.svc.Sync(ctx)
	r.store.Update(ctx, func(st *backlog.State) error {
		_, err := st.Claim(5, at("/wt/issue-5-1"), time.Time{})
		return err
	})
	r.lanes.runs = map[string]backlog.LaneRun{"/wt/issue-5-1": {Dir: "/run", Events: []run.Event{
		{Kind: run.KindRun, Run: "r", Name: "t"}, {Kind: run.KindNote, Name: "finding", Note: "Flaky clock test"}}}}
	res, err := r.svc.Sync(ctx)
	if err != nil || res.Filed != 1 || len(r.forge.created) != 1 || r.forge.created[0].Title != "Flaky clock test" {
		t.Fatalf("sync = %+v, %v, created %+v", res, err, r.forge.created)
	}
}

func TestSyncRetriesUnconfirmedHandoffs(t *testing.T) {
	r := newRig(t, is(3))
	r.init(t, backlog.Settings{})
	ctx := context.Background()
	r.svc.Sync(ctx)
	r.forge.editErr = errors.New("gh down")
	if _, err := r.svc.Finish(ctx, 3, backlog.Human, "needs a call", false); err != nil {
		t.Fatal(err)
	}
	st, _ := r.store.Load(ctx)
	if it, _ := st.Item(3); it.HandedOff || !it.Commented {
		t.Fatalf("after a failed edit = %+v", it)
	}
	// Still failing: retried, the comment not posted again.
	if res, _ := r.svc.Sync(ctx); res.Handed != 0 || len(r.forge.comments[3]) != 1 {
		t.Fatalf("retry = %+v, comments %d", res, len(r.forge.comments[3]))
	}
	r.forge.editErr = nil
	if res, _ := r.svc.Sync(ctx); res.Handed != 1 || len(r.forge.comments[3]) != 1 {
		t.Fatalf("retry = %+v, comments %d", res, len(r.forge.comments[3]))
	}
	st, _ = r.store.Load(ctx)
	if it, _ := st.Item(3); !it.HandedOff {
		t.Fatalf("hand-off not confirmed: %+v", it)
	}
	if res, _ := r.svc.Sync(ctx); res.Handed != 0 {
		t.Fatal("a confirmed hand-off was retried")
	}
	// An edit the forge refused before applying it: the retry applies the
	// label and the assignee, and posts no second comment.
	r3 := newRig(t, is(6))
	r3.init(t, backlog.Settings{})
	r3.svc.Sync(ctx)
	r3.forge.editFail = errors.New("gh down")
	r3.svc.Finish(ctx, 6, backlog.Human, "why", false)
	if is := r3.forge.open[0]; slices.Contains(is.Labels, "loops-needs-human") || slices.Contains(is.Assignees, "hum") {
		t.Fatalf("a refused edit changed the issue: %+v", is)
	}
	r3.forge.editFail = nil
	if res, _ := r3.svc.Sync(ctx); res.Handed != 1 || len(r3.forge.comments[6]) != 1 ||
		!slices.Contains(r3.forge.open[0].Labels, "loops-needs-human") || !slices.Contains(r3.forge.open[0].Assignees, "hum") {
		t.Fatalf("edit retry = %+v, issue %+v, comments %d", res, r3.forge.open[0], len(r3.forge.comments[6]))
	}
	// A comment that failed is posted by the retry.
	r2 := newRig(t, is(4))
	r2.init(t, backlog.Settings{})
	r2.svc.Sync(ctx)
	r2.forge.failComment = errors.New("gh down")
	r2.svc.Finish(ctx, 4, backlog.Human, "why", false)
	r2.forge.failComment = nil
	if res, _ := r2.svc.Sync(ctx); res.Handed != 1 || len(r2.forge.comments[4]) != 1 {
		t.Fatalf("comment retry = %+v, %v", res, r2.forge.comments[4])
	}
}

// The lane keeps its lock through the hand-off, so a sync meanwhile does
// not hand the issue off (and comment) a second time.
func TestHandoffHoldsTheLane(t *testing.T) {
	r := newRig(t, is(1))
	r.init(t, backlog.Settings{Attempts: 1})
	ctx := context.Background()
	r.svc.Sync(ctx)
	r.agent.outcome = func(string) (backlog.LaneRun, error) {
		return backlog.LaneRun{Dir: "/run", Events: events("failed", "")}, nil
	}
	var heldAtComment bool
	r.forge.onComment = func(n int) { heldAtComment = r.store.LaneHeld(n) }
	res, err := r.svc.Work(ctx, 1)
	if err != nil || res.Status != backlog.Human || !heldAtComment || r.store.LaneHeld(1) {
		t.Fatalf("hand-off = %+v, %v, held at comment %v", res, err, heldAtComment)
	}
	// An unconfirmed hand-off whose lane is live is left to that lane.
	r.store.Update(ctx, func(st *backlog.State) error {
		it, _ := st.Item(1)
		it.HandedOff, it.Commented = false, false
		return nil
	})
	r.store.hm.Lock()
	r.store.held = map[int]bool{1: true}
	r.store.hm.Unlock()
	if res, _ := r.svc.Sync(ctx); res.Handed != 0 || len(r.forge.comments[1]) != 1 {
		t.Fatalf("sync handed off a live lane's issue: %+v, %d comments", res, len(r.forge.comments[1]))
	}
}

func TestReinitKeepsLanes(t *testing.T) {
	r := newRig(t)
	r.init(t, backlog.Settings{Lanes: 3})
	// The cap probed later, under load, is lower: an init that changes
	// neither lanes nor agents keeps them.
	set, err := r.svc.Init(context.Background(), InitInput{Settings: backlog.Settings{MaxPRs: 8}, MachineCap: 4})
	if err != nil || set.Lanes != 3 || set.MaxAgents != 4 || set.MaxPRs != 8 || len(r.warns) != 0 {
		t.Fatalf("re-init = %+v, %v, %v", set, err, r.warns)
	}
}

// A cap turned on by an init between Work's load and its claim is checked
// against a fresh listing, not against no listing at all.
func TestWorkRechecksACapTurnedOnMeanwhile(t *testing.T) {
	r := newRig(t, is(1))
	r.init(t, backlog.Settings{MaxPRs: -1})
	ctx := context.Background()
	r.svc.Sync(ctx)
	r.store.Update(ctx, func(st *backlog.State) error {
		st.Items = append(st.Items, backlog.Item{Number: 9, Status: backlog.Done, PR: "https://github.com/o/r/pull/5"})
		return nil
	})
	r.forge.prs = []string{"https://github.com/o/r/pull/5"}
	r.store.onUpdate = func(st *backlog.State) { st.Settings.MaxPRs = 1; r.store.onUpdate = nil }
	res, err := r.svc.Work(ctx, 0)
	if err != nil || !res.Gated || res.Gate != (backlog.Gate{Open: 1, Cap: 1}) || len(r.wt.added) != 0 {
		t.Fatalf("work after the cap went on = %+v, %v (worktrees %v)", res, err, r.wt.added)
	}
}

// A lane that fails before its claim does not use up a --max slot.
func TestRunMaxCountsOnlyClaimedLanes(t *testing.T) {
	r := newRig(t, is(1), is(2))
	r.init(t, backlog.Settings{Lanes: 2})
	ctx := context.Background()
	r.agent.outcome = prOutcome(r)
	r.forge.failPRsAt = 2 // the first lane's, after the run's sync
	var out bytes.Buffer
	sum, err := r.svc.Run(ctx, RunInput{Max: 2, Out: &out})
	if err != nil || sum.Done != 2 || sum.Errors != 1 || sum.Started != 2 {
		t.Fatalf("run --max 2 = %+v, %v\n%s", sum, err, out.String())
	}
}

// The claim label is gone before the issue can be claimed again, and the
// lane's findings are saved with its outcome, both by the lane itself and
// by a sync that recovers a dead lane.
func TestOutcomeSavedAfterLabelAndWithFindings(t *testing.T) {
	finding := run.Event{Kind: run.KindNote, Name: "finding", Note: "Flaky clock test"}
	check := func(t *testing.T, r *rig, n int) *[]string {
		var bad []string
		r.store.onSave = func(st backlog.State) {
			it, ok := st.Item(n)
			if !ok || it.Status == backlog.Running {
				return
			}
			r.forge.mu.Lock()
			defer r.forge.mu.Unlock()
			for _, is := range r.forge.open {
				if is.Number == n && slices.Contains(is.Labels, "loops-wip") {
					bad = append(bad, "claim label still on at "+string(it.Status))
				}
			}
			if !slices.ContainsFunc(st.Pending, func(p backlog.Pending) bool { return p.Title == "Flaky clock test" }) &&
				!slices.ContainsFunc(st.Findings, func(f backlog.Finding) bool { return f.Title == "Flaky clock test" }) {
				bad = append(bad, "outcome saved without the finding at "+string(it.Status))
			}
		}
		return &bad
	}
	ctx := context.Background()
	t.Run("lane", func(t *testing.T) {
		r := newRig(t, is(1))
		r.init(t, backlog.Settings{})
		r.svc.Sync(ctx)
		r.agent.outcome = func(string) (backlog.LaneRun, error) {
			return backlog.LaneRun{Dir: "/run", Events: events("failed", "", finding)}, nil
		}
		bad := check(t, r, 1)
		if res, err := r.svc.Work(ctx, 1); err != nil || res.Status != backlog.Queued || len(*bad) != 0 {
			t.Fatalf("lane = %+v, %v, %v", res, err, *bad)
		}
	})
	t.Run("recovered", func(t *testing.T) {
		r := newRig(t, is(5))
		r.init(t, backlog.Settings{})
		r.svc.Sync(ctx)
		r.store.Update(ctx, func(st *backlog.State) error {
			_, err := st.Claim(5, at("/wt/issue-5-1"), time.Time{})
			return err
		})
		r.forge.Edit(ctx, "o/r", 5, Edit{AddLabels: []string{"loops-wip"}})
		r.lanes.runs = map[string]backlog.LaneRun{"/wt/issue-5-1": {Dir: "/run", Events: events("failed", "", finding)}}
		bad := check(t, r, 5)
		if res, err := r.svc.Sync(ctx); err != nil || res.Filed != 1 || len(*bad) != 0 {
			t.Fatalf("sync = %+v, %v, %v", res, err, *bad)
		}
	})
}

// A hand-off retried by sync has one owner: one confirmed by another
// process while this sync waited for the lane lock is not repeated, and
// finish cannot hand off an issue a sync is handing off.
func TestHandoffRetryHasOneOwner(t *testing.T) {
	r := newRig(t, is(3))
	r.init(t, backlog.Settings{})
	ctx := context.Background()
	r.svc.Sync(ctx)
	r.store.Update(ctx, func(st *backlog.State) error {
		_, err := st.Set(3, backlog.Human, "why", false, time.Time{})
		return err
	})
	r.store.onHold = func(n int) {
		r.store.onHold = nil
		r.store.mu.Lock()
		it, _ := r.store.st.Item(n)
		it.Commented, it.HandedOff = true, true
		r.store.mu.Unlock()
	}
	if res, err := r.svc.Sync(ctx); err != nil || res.Handed != 0 || len(r.forge.comments[3]) != 0 {
		t.Fatalf("sync repeated a confirmed hand-off: %+v, %v, comments %v", res, err, r.forge.comments[3])
	}
	lease, _ := r.store.HoldLane(3)
	defer lease.Release()
	if _, err := r.svc.Finish(ctx, 3, backlog.Human, "again", false); err == nil || !strings.Contains(err.Error(), "being handed off") {
		t.Fatalf("finish during another hand-off: %v", err)
	}
}

// A lane run that cannot be read is not recorded without its PR and
// findings: the lane and a recovering sync both leave it running, and the
// sync after the read works again records it.
func TestUnreadableRunStaysRunning(t *testing.T) {
	r := newRig(t, is(1))
	r.init(t, backlog.Settings{})
	ctx := context.Background()
	r.svc.Sync(ctx)
	r.lanes.failLatest = errors.New("permission denied")
	res, err := r.svc.Work(ctx, 1)
	st, _ := r.store.Load(ctx)
	if it, _ := st.Item(1); err == nil || !res.Claimed || it.Status != backlog.Running || r.store.LaneHeld(1) {
		t.Fatalf("work with an unreadable run = %+v, %v, item %+v", res, err, it)
	}
	r.svc.Sync(ctx)
	if st, _ = r.store.Load(ctx); st.Items[0].Status != backlog.Running {
		t.Fatalf("sync recorded an unreadable run: %+v", st.Items[0])
	}
	r.lanes.failLatest = nil
	r.svc.Sync(ctx)
	if st, _ = r.store.Load(ctx); st.Items[0].Status != backlog.Done || st.Items[0].PR != "https://pr/1" {
		t.Fatalf("sync after the read works = %+v", st.Items[0])
	}
}

// A sync recovers only the dead lane it saw: if another sync recovered it
// and a new lane started on the issue meanwhile, that lane is left alone.
func TestRecoveryChecksTheLane(t *testing.T) {
	r := newRig(t, is(5))
	r.init(t, backlog.Settings{})
	ctx := context.Background()
	r.svc.Sync(ctx)
	r.store.Update(ctx, func(st *backlog.State) error {
		_, err := st.Claim(5, func(n, lane int) string { return fmt.Sprintf("/wt/issue-%d-%d", n, lane) }, time.Time{})
		return err
	})
	r.store.onHold = func(n int) {
		r.store.onHold = nil
		r.store.mu.Lock()
		defer r.store.mu.Unlock()
		it, _ := r.store.st.Item(n)
		it.Lanes, it.Worktree, it.Past = 2, "/wt/issue-5-2", []string{"/wt/issue-5-1"}
	}
	r.svc.Sync(ctx)
	st, _ := r.store.Load(ctx)
	if it, _ := st.Item(5); it.Status != backlog.Running || it.Lanes != 2 || len(r.forge.edits[5]) != 0 {
		t.Fatalf("sync finished a lane it did not see: %+v, edits %+v", it, r.forge.edits[5])
	}
}

// Every ruling has one owner: finish refuses while a hand-off (or a lane)
// holds the issue.
func TestFinishWaitsForTheHandoff(t *testing.T) {
	r := newRig(t, is(3))
	r.init(t, backlog.Settings{})
	ctx := context.Background()
	r.svc.Sync(ctx)
	r.svc.Finish(ctx, 3, backlog.Human, "why", false)
	lease, _ := r.store.HoldLane(3)
	if _, err := r.svc.Finish(ctx, 3, backlog.Queued, "", true); err == nil || !strings.Contains(err.Error(), "being handed off") {
		t.Fatalf("queued during a hand-off: %v", err)
	}
	if edits := r.forge.edits[3]; len(edits) != 1 {
		t.Fatalf("the refused ruling edited the issue: %+v", edits)
	}
	lease.Release()
	if _, err := r.svc.Finish(ctx, 3, backlog.Queued, "", true); err != nil {
		t.Fatal(err)
	}
}

// A repo changed by an init between the load and the claim is listed again.
func TestClaimRelistsAChangedRepo(t *testing.T) {
	r := newRig(t, is(1))
	r.init(t, backlog.Settings{})
	ctx := context.Background()
	r.svc.Sync(ctx)
	r.forge.prRepos = nil
	r.store.onUpdate = func(st *backlog.State) { st.Settings.Repo = "o/other"; r.store.onUpdate = nil }
	if _, err := r.svc.Work(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(r.forge.prRepos, []string{"o/r", "o/other"}) {
		t.Fatalf("PRs listed for %v", r.forge.prRepos)
	}
}

// Run tells a queue another process emptied from lanes that cannot start,
// and a lane whose outcome could not be saved from one that never ran.
func TestRunDrainOutcomes(t *testing.T) {
	ctx := context.Background()
	takeQueue := func(r *rig) func(int) {
		return func(call int) {
			if call == 2 { // the first lane's listing, after the run's sync
				r.store.mu.Lock()
				r.store.st.Items[0].Status = backlog.Running
				r.store.mu.Unlock()
				r.store.hm.Lock()
				r.store.held = map[int]bool{1: true}
				r.store.hm.Unlock()
			}
		}
	}
	r := newRig(t, is(1))
	r.init(t, backlog.Settings{Lanes: 1})
	r.forge.onPRs = takeQueue(r)
	if sum, err := r.svc.Run(ctx, RunInput{}); err != nil || sum.Started != 0 {
		t.Fatalf("queue taken by another process = %+v, %v", sum, err)
	}
	r = newRig(t, is(1))
	r.init(t, backlog.Settings{Lanes: 1})
	r.forge.onPRs = takeQueue(r)
	wctx, stop := context.WithCancel(ctx)
	defer stop()
	var out bytes.Buffer
	r.svc.clock = fixedClock{sleep: func(time.Duration) { stop() }}
	if _, err := r.svc.Run(wctx, RunInput{Out: &out, Wait: time.Minute}); !errors.Is(err, context.Canceled) || !strings.Contains(out.String(), "idle: nothing left to claim") {
		t.Fatalf("--wait with the queue taken = %v\n%s", err, out.String())
	}
	r = newRig(t, is(1))
	r.init(t, backlog.Settings{Lanes: 1})
	r.agent.outcome = func(string) (backlog.LaneRun, error) {
		r.store.failNext = errors.New("disk full")
		return backlog.LaneRun{Dir: "/run", Events: events("done", "https://pr/1")}, nil
	}
	if sum, err := r.svc.Run(ctx, RunInput{Max: 1}); err != nil || sum.Started != 1 || sum.Errors != 1 {
		t.Fatalf("a lane whose outcome was not saved = %+v, %v", sum, err)
	}
}
