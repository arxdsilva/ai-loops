package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/application/updatecheck"
	"github.com/arxdsilva/ai-loops/internal/domain/run"
	"github.com/arxdsilva/ai-loops/internal/domain/update"
)

type fakeUpd struct {
	st  update.State
	err error
}

func (f fakeUpd) Fetch(context.Context) (update.State, error) { return f.st, f.err }
func (f fakeUpd) Load() (update.State, error)                 { return f.st, f.err }

func runApp(a *App, args ...string) (int, string, string) {
	var o, e bytes.Buffer
	a.Out, a.Err = &o, &e
	c := a.Run(context.Background(), args)
	return c, o.String(), e.String()
}

func clearEnv(t *testing.T) {
	for _, k := range []string{"CURSOR_PLUGIN_ROOT", "CURSOR_PROJECT_DIR", "GEMINI_PROJECT_DIR", "GEMINI_CWD", "DEVIN_PROJECT_DIR", "CLAUDE_PROJECT_DIR", "CLAUDE_PLUGIN_ROOT", "PLUGIN_ROOT", "LOOPS_NO_UPDATE_CHECK"} {
		t.Setenv(k, "")
	}
}

const testVersion = "0.3.0"

func TestVersionPlain(t *testing.T) {
	c, o, _ := runApp(&App{Version: testVersion}, "version")
	if c != 0 || o != "loops "+testVersion+"\n" {
		t.Fatalf("%d %q", c, o)
	}
	if c, _, _ := runApp(&App{}, "version", "--bogus"); c != 2 {
		t.Fatal(c)
	}
}

func TestVersionCheck(t *testing.T) {
	clearEnv(t)
	a := &App{Version: testVersion, Update: fakeUpd{st: update.State{Latest: "v9.0.0"}}}
	c, o, _ := runApp(a, "version", "--check", "--platform", "claude")
	if c != 0 || !strings.Contains(o, "loops "+testVersion) || !strings.Contains(o, "latest: v9.0.0") ||
		!strings.Contains(o, "update available: "+updatecheck.Command("claude")) {
		t.Fatalf("%d %q", c, o)
	}
	t.Setenv("CLAUDE_PROJECT_DIR", "/x")
	_, o, _ = runApp(a, "version", "--check")
	if !strings.Contains(o, updatecheck.Command("claude")) {
		t.Fatalf("autodetect: %q", o)
	}
	a = &App{Version: testVersion, Update: fakeUpd{st: update.State{Latest: "v" + testVersion}}}
	if _, o, _ = runApp(a, "version", "--check"); !strings.Contains(o, "up to date") {
		t.Fatalf("%q", o)
	}
	a = &App{Update: fakeUpd{err: errors.New("offline")}}
	if c, o, _ = runApp(a, "version", "--check"); c != 0 || !strings.Contains(o, "latest: unknown (offline)") {
		t.Fatalf("%d %q", c, o)
	}
	if c, o, _ = runApp(&App{}, "version", "--check"); c != 0 || !strings.Contains(o, "unavailable") {
		t.Fatalf("%d %q", c, o)
	}
}

func TestUpdateCmd(t *testing.T) {
	clearEnv(t)
	c, o, _ := runApp(&App{}, "update", "--platform", "codex")
	if c != 0 || strings.TrimSpace(o) != updatecheck.Command("codex") {
		t.Fatalf("%d %q", c, o)
	}
	_, o, _ = runApp(&App{}, "update", "--platform", "weird")
	if strings.TrimSpace(o) != updatecheck.Command("weird") {
		t.Fatalf("%q", o)
	}
	_, o, _ = runApp(&App{}, "update")
	for n := range updatecheck.Commands() {
		if !strings.Contains(o, n+":") {
			t.Errorf("missing %s in %q", n, o)
		}
	}
	t.Setenv("GEMINI_CWD", "/x")
	_, o, _ = runApp(&App{}, "update")
	if strings.TrimSpace(o) != updatecheck.Command("gemini") {
		t.Fatalf("%q", o)
	}
	if c, _, _ := runApp(&App{}, "update", "extra"); c != 2 {
		t.Fatal(c)
	}
}

func TestDetectPlatform(t *testing.T) {
	for env, want := range map[string]string{"CURSOR_PROJECT_DIR": "cursor", "DEVIN_PROJECT_DIR": "windsurf", "CLAUDE_PLUGIN_ROOT": "claude", "PLUGIN_ROOT": "codex", "NOPE": ""} {
		got := detectPlatform(func(k string) string {
			if k == env {
				return "1"
			}
			return ""
		})
		if got != want {
			t.Errorf("%s: %q want %q", env, got, want)
		}
	}
}

func TestStatusLineUpdateMarker(t *testing.T) {
	clearEnv(t)
	newer := fakeUpd{st: update.State{Latest: "v9.0.0"}}
	a := &App{Version: testVersion, Tracker: fakeNoRun{}, UpdateState: newer}
	_, o, _ := runApp(a, "status", "--statusline")
	if !strings.Contains(o, " · ⬆ v9.0.0") {
		t.Fatalf("%q", o)
	}
	for name, mod := range map[string]func(*App){
		"off":     func(a *App) { a.UpdateOff = true },
		"nostate": func(a *App) { a.UpdateState = nil },
		"same":    func(a *App) { a.UpdateState = fakeUpd{st: update.State{Latest: "v" + testVersion}} },
		"err":     func(a *App) { a.UpdateState = fakeUpd{err: errors.New("x")} },
	} {
		a := &App{Version: testVersion, Tracker: fakeNoRun{}, UpdateState: newer}
		mod(a)
		if _, o, _ := runApp(a, "status", "--statusline"); strings.Contains(o, "⬆") {
			t.Errorf("%s: %q", name, o)
		}
	}
	t.Setenv("LOOPS_NO_UPDATE_CHECK", "1")
	if _, o, _ := runApp(&App{Version: testVersion, Tracker: fakeNoRun{}, UpdateState: newer}, "status", "--statusline"); strings.Contains(o, "⬆") {
		t.Errorf("env: %q", o)
	}
}

type fakeNoRun struct{ Tracker }

func (fakeNoRun) State(context.Context, string) (run.State, error) {
	return run.State{}, errors.New("none")
}
