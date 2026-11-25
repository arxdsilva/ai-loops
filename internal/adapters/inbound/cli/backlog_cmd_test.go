package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	appbacklog "github.com/arxdsilva/ai-loops/internal/application/backlog"
	"github.com/arxdsilva/ai-loops/internal/domain/backlog"
	"github.com/arxdsilva/ai-loops/internal/domain/hw"
)

type fakeBacklog struct {
	init     appbacklog.InitInput
	work     appbacklog.LaneResult
	workN    int
	run      appbacklog.RunInput
	finish   []any
	file     []any
	clean    []bool
	err      error
	removed  []string
	filedDup bool
	sync     backlog.SyncResult
}

var testSettings = backlog.Settings{Repo: "o/r", Base: "main", Human: "hum", Lanes: 2, MaxAgents: 4, Attempts: 2, Timeout: "4h0m0s",
	Exclude: []string{"wontfix"}, Authors: []string{"me"}, Worktrees: "/wt"}

func (f *fakeBacklog) Init(_ context.Context, in appbacklog.InitInput) (backlog.Settings, error) {
	f.init = in
	return testSettings, f.err
}
func (f *fakeBacklog) Sync(context.Context) (backlog.SyncResult, error) {
	if f.sync.Open == 0 {
		f.sync = backlog.SyncResult{Open: 9, Added: 3, Dropped: 1, Queued: 5}
	}
	return f.sync, f.err
}
func (f *fakeBacklog) Status(context.Context) (appbacklog.Report, error) {
	return appbacklog.Report{Settings: testSettings, Synced: "2026-01-01 10:00 UTC", Findings: 2,
		Counts:  map[backlog.Status]int{backlog.Queued: 5, backlog.Running: 1},
		Running: []appbacklog.RunningLane{{Item: backlog.Item{Number: 7, Title: "Fix it", Attempts: 1}, Phase: "qa", Log: "/l/7.log"}},
		Next:    []backlog.Item{{Number: 3, Title: "Next one", Priority: 1}, {Number: 4, Title: "Unranked", Priority: backlog.NoPriority}},
	}, f.err
}
func (f *fakeBacklog) Work(_ context.Context, n int) (appbacklog.LaneResult, error) {
	f.workN = n
	return f.work, f.err
}
func (f *fakeBacklog) Run(_ context.Context, in appbacklog.RunInput) (appbacklog.RunSummary, error) {
	f.run = in
	return appbacklog.RunSummary{Started: 3, Done: 2, Human: 1}, f.err
}
func (f *fakeBacklog) Finish(_ context.Context, n int, st backlog.Status, note string, force bool) (backlog.Item, error) {
	f.finish = []any{n, st, note, force}
	return backlog.Item{Number: n, Status: st}, f.err
}
func (f *fakeBacklog) File(_ context.Context, title, body string, from int) (appbacklog.Filed, error) {
	f.file = []any{title, body, from}
	return appbacklog.Filed{URL: "https://f/1", Duplicate: f.filedDup}, f.err
}
func (f *fakeBacklog) Clean(_ context.Context, all, dry bool) ([]string, error) {
	f.clean = []bool{all, dry}
	return f.removed, f.err
}

func backlogApp(f *fakeBacklog) (*App, *bytes.Buffer, *bytes.Buffer) {
	var out, errw bytes.Buffer
	a := &App{Out: &out, Err: &errw, In: strings.NewReader("stdin body")}
	a.Backlog = func(_ context.Context, warn func(string, ...any)) BacklogEnv {
		warn("probe %d", 1)
		return BacklogEnv{Svc: f, Worktrees: "/default-wt"}
	}
	return a, &out, &errw
}

func TestBacklogUsage(t *testing.T) {
	a, _, _ := backlogApp(&fakeBacklog{})
	for _, args := range [][]string{
		{"backlog"}, {"backlog", "nope"}, {"backlog", "sync", "x"}, {"backlog", "status", "x"},
		{"backlog", "work", "a"}, {"backlog", "work", "1", "2"}, {"backlog", "run", "--max", "-1"},
		{"backlog", "finish"}, {"backlog", "finish", "x", "--status", "done"}, {"backlog", "finish", "3", "--status", "merged"},
		{"backlog", "file"}, {"backlog", "clean", "x"}, {"backlog", "init", "extra"},
		{"backlog", "init", "--max-prs", "0"}, {"backlog", "init", "--max-prs", "lots"}, {"backlog", "run", "--wait", "-1m"},
	} {
		if code := a.Run(context.Background(), args); code != 2 {
			t.Errorf("%v exit %d, want 2", args, code)
		}
	}
	if code := (&App{Out: io.Discard, Err: io.Discard}).Run(context.Background(), []string{"backlog", "sync"}); code != 2 {
		t.Fatal("no backlog service should be usage")
	}
}

func TestBacklogInit(t *testing.T) {
	f := &fakeBacklog{}
	a, out, errw := backlogApp(f)
	a.HW = fakeHW{f: hw.Facts{LogicalCPUs: 4, TotalRAMBytes: 64 << 30, AvailRAMBytes: 64 << 30}}
	code := a.Run(context.Background(), []string{"backlog", "init", "--human", "hum", "--lanes", "3", "--label", "bug, P1",
		"--exclude-label", "x", "--authors", "a,b", "--budget", "5", "--max-prs", "3"})
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errw.String())
	}
	in := f.init
	if in.Settings.Human != "hum" || in.Settings.Lanes != 3 || !slices.Equal(in.Settings.Labels, []string{"bug", "P1"}) ||
		!slices.Equal(in.Settings.Exclude, []string{"x"}) || !slices.Equal(in.Settings.Authors, []string{"a", "b"}) ||
		in.Settings.Budget != "5" || in.Worktrees != "/default-wt" || in.MachineCap != 16 || in.AnyAuthor || in.Settings.MaxPRs != 3 {
		t.Fatalf("init input = %+v", in)
	}
	for _, w := range []string{"repo o/r, base main, human @hum", "lanes 2 × 4 agents", "open PRs cap 5", "authors me", "worktrees: /wt", "loops backlog sync"} {
		if !strings.Contains(out.String(), w) {
			t.Fatalf("output lacks %q:\n%s", w, out.String())
		}
	}
	if !strings.Contains(errw.String(), "warning: probe 1") {
		t.Fatalf("warnings not relayed: %q", errw.String())
	}
	a, _, _ = backlogApp(f)
	a.Run(context.Background(), []string{"backlog", "init", "--authors", "any", "--max-prs", "none"})
	if !f.init.AnyAuthor || f.init.MachineCap != hw.DefaultMaxAgents || f.init.Settings.MaxPRs != -1 {
		t.Fatalf("any author, no PR cap = %+v", f.init)
	}
	var b bytes.Buffer
	printSettings(&b, backlog.Settings{MaxPRs: -1})
	if !strings.Contains(b.String(), "open PRs cap none") {
		t.Fatalf("no cap shown as %q", b.String())
	}
	f.err = errors.New("no human")
	if code := a.Run(context.Background(), []string{"backlog", "init"}); code != 1 {
		t.Fatalf("init error exit %d", code)
	}
}

func TestBacklogSyncStatus(t *testing.T) {
	f := &fakeBacklog{}
	a, out, _ := backlogApp(f)
	if a.Run(context.Background(), []string{"backlog", "sync"}) != 0 || !strings.Contains(out.String(), "9 open issues, 3 added, 1 dropped, 5 queued") {
		t.Fatalf("sync output: %s", out.String())
	}
	out.Reset()
	f.sync.Gate, f.sync.Filed, f.sync.Handed = backlog.Gate{Open: 5, Cap: 5}, 2, 1
	if a.Run(context.Background(), []string{"backlog", "sync"}) != 0 || !strings.Contains(out.String(), "open PRs: 5 of 5 (lane PRs and running lanes): lanes paused") ||
		!strings.Contains(out.String(), "retried: 2 pending findings filed, 1 hand-offs confirmed") {
		t.Fatalf("sync gate output: %s", out.String())
	}
	out.Reset()
	f.sync.Gate = backlog.Gate{Open: 1, Cap: 5}
	if a.Run(context.Background(), []string{"backlog", "sync"}); !strings.Contains(out.String(), "open PRs: 1 of 5 (lane PRs and running lanes): lanes may start") {
		t.Fatalf("sync gate output: %s", out.String())
	}
	out.Reset()
	a.Run(context.Background(), []string{"backlog", "sync", "--json"})
	if !strings.Contains(out.String(), `"Queued": 5`) {
		t.Fatalf("sync json: %s", out.String())
	}
	out.Reset()
	a.Run(context.Background(), []string{"backlog", "status"})
	for _, w := range []string{"queued 5 · running 1 · done 0", "running #7 (attempt 1, process gone", "phase qa", "log /l/7.log", "next #3 (P1): Next one", "next #4 (P-): Unranked", "findings filed: 2"} {
		if !strings.Contains(out.String(), w) {
			t.Fatalf("status lacks %q:\n%s", w, out.String())
		}
	}
	out.Reset()
	a.Run(context.Background(), []string{"backlog", "status", "--json"})
	if !strings.Contains(out.String(), `"Running"`) {
		t.Fatalf("status json: %s", out.String())
	}
	f.err = errors.New("no state")
	for _, sub := range []string{"sync", "status", "run", "clean"} {
		if code := a.Run(context.Background(), []string{"backlog", sub}); code != 1 {
			t.Fatalf("%s error exit %d", sub, code)
		}
	}
}

func TestBacklogWorkExitCodes(t *testing.T) {
	cases := []struct {
		res  appbacklog.LaneResult
		code int
		want string
	}{
		{appbacklog.LaneResult{Empty: true}, 3, "nothing queued"},
		{appbacklog.LaneResult{Gated: true, Gate: backlog.Gate{Open: 5, Cap: 5}}, 4, "paused: 5 of 5 lane PRs open"},
		{appbacklog.LaneResult{Status: backlog.Done, Item: backlog.Item{Number: 5, PR: "https://pr/5"},
			Findings: []appbacklog.Filed{{Title: "Leak", URL: "https://f/9"}, {Title: "Old", URL: "https://f/2", Duplicate: true}}},
			0, "#5 done: https://pr/5\nfinding filed: Leak https://f/9\nfinding already tracked: Old https://f/2"},
		{appbacklog.LaneResult{Status: backlog.Queued, Reason: "ended failed", Item: backlog.Item{Number: 5, Attempts: 1}}, 1, "queued again: ended failed"},
		{appbacklog.LaneResult{Status: backlog.Human, Reason: "needs a call", Item: backlog.Item{Number: 5}}, 1, "handed to the human: needs a call"},
	}
	for _, tc := range cases {
		f := &fakeBacklog{work: tc.res}
		a, out, _ := backlogApp(f)
		if code := a.Run(context.Background(), []string{"backlog", "work", "#5"}); code != tc.code || !strings.Contains(out.String(), tc.want) {
			t.Fatalf("work = %d %q, want %d %q", code, out.String(), tc.code, tc.want)
		}
		if f.workN != 5 {
			t.Fatalf("issue = %d", f.workN)
		}
	}
	f := &fakeBacklog{work: appbacklog.LaneResult{Status: backlog.Done}}
	a, out, _ := backlogApp(f)
	a.Run(context.Background(), []string{"backlog", "work", "--json"})
	if f.workN != 0 || !strings.Contains(out.String(), `"Status": "done"`) {
		t.Fatalf("json work: %d %s", f.workN, out.String())
	}
	f.err = errors.New("boom")
	if code := a.Run(context.Background(), []string{"backlog", "work"}); code != 1 {
		t.Fatalf("error exit %d", code)
	}
}

func TestBacklogRunFinishFileClean(t *testing.T) {
	f := &fakeBacklog{}
	a, out, _ := backlogApp(f)
	ctx := context.Background()
	if a.Run(ctx, []string{"backlog", "run", "--max", "4", "--wait", "10m"}) != 0 || f.run.Max != 4 || f.run.Wait != 10*time.Minute || !strings.Contains(out.String(), "run: 3 lanes, 2 PRs, 0 retried, 1 handed off") {
		t.Fatalf("run: %+v %s", f.run, out.String())
	}
	out.Reset()
	if a.Run(ctx, []string{"backlog", "finish", "#8", "--status", "human", "--note", "why", "--force"}) != 0 ||
		!slices.Equal(f.finish, []any{8, backlog.Human, "why", true}) || out.String() != "#8 human\n" {
		t.Fatalf("finish: %v %q", f.finish, out.String())
	}
	out.Reset()
	if a.Run(ctx, []string{"backlog", "file", "--title", "Leak", "--body-file", "-", "--from", "3"}) != 0 ||
		!slices.Equal(f.file, []any{"Leak", "stdin body", 3}) || out.String() != "https://f/1\n" {
		t.Fatalf("file: %v %q", f.file, out.String())
	}
	body := filepath.Join(t.TempDir(), "b.md")
	os.WriteFile(body, []byte("from file"), 0o644)
	f.filedDup = true
	out.Reset()
	a.Run(ctx, []string{"backlog", "file", "--title", "Leak", "--body-file", body})
	if f.file[1] != "from file" || out.String() != "duplicate https://f/1\n" {
		t.Fatalf("file dup: %v %q", f.file, out.String())
	}
	if a.Run(ctx, []string{"backlog", "file", "--title", "x", "--body-file", "/no/such"}) != 1 {
		t.Fatal("missing body file")
	}
	out.Reset()
	a.Run(ctx, []string{"backlog", "clean", "--dry-run"})
	if !slices.Equal(f.clean, []bool{false, true}) || !strings.Contains(out.String(), "no finished worktrees") {
		t.Fatalf("clean: %v %q", f.clean, out.String())
	}
	f.removed = []string{"/wt/issue-1"}
	out.Reset()
	a.Run(ctx, []string{"backlog", "clean", "--all"})
	if out.String() != "removed /wt/issue-1\n" {
		t.Fatalf("clean --all: %q", out.String())
	}
	f.err = errors.New("x")
	for _, args := range [][]string{{"backlog", "finish", "1", "--status", "done"}, {"backlog", "file", "--title", "t"}} {
		if a.Run(ctx, args) != 1 {
			t.Fatalf("%v should fail", args)
		}
	}
}
