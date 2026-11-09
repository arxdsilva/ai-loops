package cli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
	"github.com/arxdsilva/ai-loops/internal/domain/workspace"
)

type fakeWS struct {
	prepared []string
	prepErr  error
	cleaned  []string
	dry      []bool
	report   workspace.Report
	cleanErr error
	goEnv    []string
	goArgs   []string
	goDir    string
	goCode   int
	envErr   error
}

func (w *fakeWS) Prepare(_ context.Context, dir string) error {
	w.prepared = append(w.prepared, dir)
	return w.prepErr
}
func (w *fakeWS) Env(dir string) ([]string, error) {
	if w.envErr != nil {
		return nil, w.envErr
	}
	return workspace.Env(dir), nil
}
func (w *fakeWS) Clean(_ context.Context, dir string, dry bool) (workspace.Report, error) {
	w.cleaned = append(w.cleaned, dir)
	w.dry = append(w.dry, dry)
	r := w.report
	r.DryRun = dry
	return r, w.cleanErr
}
func (w *fakeWS) Go(_ context.Context, dir string, env, args []string, _ io.Reader, out, _ io.Writer) int {
	w.goDir, w.goEnv, w.goArgs = dir, env, args
	io.WriteString(out, "go ran\n")
	return w.goCode
}

func wsSetup(t *testing.T) (*rcEnv, *fakeWS) {
	t.Helper()
	e := rcSetup(t, "")
	ws := &fakeWS{report: workspace.Report{CacheFiles: 3, CacheBytes: 1 << 20, Removed: []workspace.Item{{Path: "x.test", Bytes: 1 << 20}}, Kept: []string{"notes.md"}}}
	e.app.Workspace = ws
	return e, ws
}

func (e *rcEnv) cleanupNotes() []run.Event {
	var out []run.Event
	for _, ev := range e.events() {
		if ev.Kind == run.KindNote && ev.Name == "cleanup" {
			out = append(out, ev)
		}
	}
	return out
}

func TestRunStartPreparesWorkspace(t *testing.T) {
	e, ws := wsSetup(t)
	if code := e.do("run", "start", "--title", "x"); code != 0 {
		t.Fatal(code, e.err)
	}
	if !slices.Equal(ws.prepared, []string{e.log.Dir(e.ptr.id)}) {
		t.Fatal(ws.prepared)
	}
	ws.prepErr = errors.New("disk full")
	e.ptr.id = ""
	if code := e.do("run", "start", "--title", "y"); code != 0 || !strings.Contains(e.err.String(), "disk full") {
		t.Fatal(code, e.err)
	}
}

func TestRunEnv(t *testing.T) {
	e, _ := wsSetup(t)
	if code := e.do("run", "env"); code != 0 || e.out.String() != "" || e.err.String() != "" {
		t.Fatalf("no run: %d %q %q", code, e.out, e.err)
	}
	e.do("run", "start", "--title", "x")
	dir := e.log.Dir(e.ptr.id)
	for shell, want := range map[string]string{
		"":           "export GOCACHE='" + workspace.GoCache(dir) + "' GOTMPDIR='" + workspace.GoTmp(dir) + "'\n",
		"sh":         "export GOCACHE='" + workspace.GoCache(dir) + "' GOTMPDIR='" + workspace.GoTmp(dir) + "'\n",
		"fish":       "set -gx GOCACHE '" + workspace.GoCache(dir) + "'; set -gx GOTMPDIR '" + workspace.GoTmp(dir) + "'\n",
		"powershell": "$env:GOCACHE='" + workspace.GoCache(dir) + "'; $env:GOTMPDIR='" + workspace.GoTmp(dir) + "'\n",
	} {
		args := []string{"run", "env"}
		if shell != "" {
			args = append(args, "--shell", shell)
		}
		if code := e.do(args...); code != 0 || e.out.String() != want {
			t.Errorf("%s: %d %q", shell, code, e.out)
		}
	}
	if code := e.do("run", "env", "--shell", "cmd"); code != 2 {
		t.Fatal(code)
	}
	if code := e.do("run", "env", "extra"); code != 2 {
		t.Fatal(code)
	}
	if code := e.do("run", "env", "--bogus"); code != 2 {
		t.Fatal(code)
	}
}

func TestRunEnvErrors(t *testing.T) {
	e, ws := wsSetup(t)
	e.do("run", "start", "--title", "x")
	ws.envErr = errors.New("read-only fs")
	if code := e.do("run", "env"); code != 1 || !strings.Contains(e.err.String(), "read-only fs") {
		t.Fatal(code, e.err)
	}
	e.app.Workspace = nil
	if code := e.do("run", "env"); code != 0 || e.out.String() != "" {
		t.Fatal(code, e.out)
	}
}

func TestGoCmd(t *testing.T) {
	e, ws := wsSetup(t)
	ws.goCode = 3
	if code := e.do("go", "test", "-race", "./..."); code != 3 || ws.goEnv != nil || !slices.Equal(ws.goArgs, []string{"test", "-race", "./..."}) || ws.goDir != "" {
		t.Fatalf("no run: %d %v %v", code, ws.goEnv, ws.goArgs)
	}
	if e.out.String() != "go ran\n" {
		t.Fatal(e.out)
	}
	e.do("run", "start", "--title", "x")
	ws.goCode = 0
	if code := e.do("go", "build", "./..."); code != 0 || !slices.Equal(ws.goEnv, workspace.Env(e.log.Dir(e.ptr.id))) {
		t.Fatalf("%d %v", code, ws.goEnv)
	}
	ws.envErr = errors.New("nope")
	if code := e.do("go", "build"); code != 1 || !strings.Contains(e.err.String(), "nope") {
		t.Fatal(code, e.err)
	}
	e.app.Workspace = nil
	if code := e.do("go", "build"); code != 1 {
		t.Fatal(code)
	}
}

func TestGateCmdUsesRunEnv(t *testing.T) {
	e, _ := wsSetup(t)
	e.gate.res.OK = true
	if e.do("gate") != 0 || e.gate.opts.Env != nil {
		t.Fatalf("no run: %v", e.gate.opts.Env)
	}
	e.do("run", "start", "--title", "x")
	if code := e.do("gate"); code != 0 || !slices.Equal(e.gate.opts.Env, workspace.Env(e.log.Dir(e.ptr.id))) {
		t.Fatalf("%d %v", code, e.gate.opts.Env)
	}
}

func TestRunClean(t *testing.T) {
	e, ws := wsSetup(t)
	if code := e.do("run", "clean"); code != 1 || !strings.Contains(e.err.String(), "no active run") {
		t.Fatal(code, e.err)
	}
	e.do("run", "start", "--title", "x")
	if code := e.do("run", "clean", "--dry-run"); code != 0 || !strings.Contains(e.out.String(), "dry run") || len(e.cleanupNotes()) != 0 {
		t.Fatalf("%d %q", code, e.out)
	}
	if code := e.do("run", "clean", "--json"); code != 0 {
		t.Fatal(code, e.err)
	}
	var r workspace.Report
	if err := json.Unmarshal(e.out.Bytes(), &r); err != nil || len(r.Removed) != 1 || r.CacheFiles != 3 {
		t.Fatalf("%q %v", e.out, err)
	}
	notes := e.cleanupNotes()
	if len(notes) != 1 || notes[0].Note != "4 files, 2.0 MB freed; 1 kept for review" {
		t.Fatalf("%+v", notes)
	}
	if !slices.Equal(ws.dry, []bool{true, false}) {
		t.Fatal(ws.dry)
	}
	ws.cleanErr = errors.New("boom")
	if code := e.do("run", "clean"); code != 1 || !strings.Contains(e.err.String(), "boom") {
		t.Fatal(code, e.err)
	}
	if code := e.do("run", "clean", "x"); code != 2 {
		t.Fatal(code)
	}
	if code := e.do("run", "clean", "--nope"); code != 2 {
		t.Fatal(code)
	}
	e.app.Workspace = nil
	if code := e.do("run", "clean"); code != 1 {
		t.Fatal(code)
	}
}

func TestRunEndCleans(t *testing.T) {
	for _, outcome := range []string{"done", "abandoned", "failed"} {
		e, ws := wsSetup(t)
		e.do("run", "start", "--title", "x")
		id := e.ptr.id
		if code := e.do("run", "end", "--outcome", outcome); code != 0 {
			t.Fatal(code, e.err)
		}
		evs := e.log.m[id]
		n := len(evs)
		if evs[n-1].Kind != run.KindEnd || evs[n-2].Kind != run.KindNote || evs[n-2].Name != "cleanup" {
			t.Fatalf("%s: %+v", outcome, evs[n-2:])
		}
		if len(ws.cleaned) != 1 || ws.dry[0] || !strings.Contains(e.out.String(), "cleanup: 4 files") {
			t.Fatalf("%s: %v %q", outcome, ws.cleaned, e.out)
		}
	}
}

func TestRunEndNoClean(t *testing.T) {
	e, ws := wsSetup(t)
	e.do("run", "start", "--title", "x")
	id := e.ptr.id
	if code := e.do("run", "end", "--no-clean"); code != 0 || len(ws.cleaned) != 0 {
		t.Fatal(code, ws.cleaned)
	}
	for _, ev := range e.log.m[id] {
		if ev.Name == "cleanup" {
			t.Fatal("cleanup recorded with --no-clean")
		}
	}
}

func TestRunEndCleanupErrorStillEnds(t *testing.T) {
	e, ws := wsSetup(t)
	e.do("run", "start", "--title", "x")
	ws.cleanErr = errors.New("git exploded")
	if code := e.do("run", "end"); code != 0 || e.ptr.id != "" || !strings.Contains(e.err.String(), "git exploded") {
		t.Fatal(code, e.ptr.id, e.err)
	}
	// without an active run, end fails as before and cleans nothing.
	if code := e.do("run", "end"); code != 1 || len(ws.cleaned) != 1 {
		t.Fatal(code, ws.cleaned)
	}
}
