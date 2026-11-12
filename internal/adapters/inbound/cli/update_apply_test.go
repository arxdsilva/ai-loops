package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arxdsilva/ai-loops/internal/application/updatecheck"
)

type fakeApplier struct {
	calls    int
	current  string
	platform string
	deadline time.Time
	hasDL    bool
	out      updatecheck.Outcome
}

func (f *fakeApplier) Apply(ctx context.Context, current, platform string) updatecheck.Outcome {
	f.calls++
	f.current, f.platform = current, platform
	f.deadline, f.hasDL = ctx.Deadline()
	return f.out
}

type fakePlugins map[string]string

func (f fakePlugins) PluginVersion(root string) (string, error) {
	if v, ok := f[root]; ok {
		return v, nil
	}
	return "", errors.New("no plugin.json")
}

// applyEnv isolates the environment and the pointer files; it returns the
// project dir and a state home.
func applyEnv(t *testing.T) (proj, state string) {
	t.Helper()
	clearEnv(t)
	t.Setenv("CI", "")
	proj, state = t.TempDir(), t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	return
}

func pointer(t *testing.T, path, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(root+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestApplyOptOuts(t *testing.T) {
	cases := []struct {
		name, env, val string
		off            bool
		want           string
	}{
		{"env", "LOOPS_NO_UPDATE_CHECK", "1", false, "LOOPS_NO_UPDATE_CHECK"},
		{"ci true", "CI", "true", false, "CI"},
		{"ci 1", "CI", "1", false, "CI"},
		{"config", "", "", true, "update_check: false"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			applyEnv(t)
			if c.env != "" {
				t.Setenv(c.env, c.val)
			}
			f := &fakeApplier{}
			code, out, errs := runApp(&App{Version: "0.9.5", Applier: f, UpdateOff: c.off}, "update", "--apply")
			want := "loops update: skipped (" + c.want + ")\n"
			if code != 0 || out != want || errs != "" || f.calls != 0 {
				t.Fatalf("code=%d out=%q err=%q calls=%d", code, out, errs, f.calls)
			}
		})
	}
}

func TestApplyNilApplier(t *testing.T) {
	applyEnv(t)
	_, out, _ := runApp(&App{Version: "0.9.5"}, "update", "--apply")
	if out != "loops update: skipped (update check unavailable)\n" {
		t.Fatal(out)
	}
}

func TestApplyDevGuards(t *testing.T) {
	for _, kind := range []string{"dir", "file"} {
		t.Run(kind, func(t *testing.T) {
			applyEnv(t)
			root := t.TempDir()
			git := filepath.Join(root, ".git")
			if kind == "dir" {
				_ = os.Mkdir(git, 0o755)
			} else {
				_ = os.WriteFile(git, []byte("gitdir: x"), 0o644)
			}
			f := &fakeApplier{}
			a := &App{Version: "0.9.5", PluginRoot: root, Plugins: fakePlugins{root: "0.9.5"}, Applier: f}
			_, out, _ := runApp(a, "update", "--apply")
			if out != "loops update: skipped (dev checkout at "+root+")\n" || f.calls != 0 {
				t.Fatalf("%q calls=%d", out, f.calls)
			}
		})
	}
	applyEnv(t)
	root := t.TempDir()
	f := &fakeApplier{}
	a := &App{Version: "0.9.5", PluginRoot: root, Plugins: fakePlugins{root: "0.0.0-dev"}, Applier: f}
	_, out, _ := runApp(a, "update", "--apply")
	if out != "loops update: skipped (dev build v0.0.0-dev)\n" || f.calls != 0 {
		t.Fatalf("%q calls=%d", out, f.calls)
	}
}

func TestApplyLines(t *testing.T) {
	at := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	retry := at.Add(24 * time.Hour)
	cases := []struct {
		name string
		o    updatecheck.Outcome
		want string
	}{
		{"up-to-date", updatecheck.Outcome{Kind: updatecheck.KindUpToDate}, "up to date (v0.7.1)"},
		{"updated", updatecheck.Outcome{Kind: updatecheck.KindUpdated, Latest: "0.8.0", Platform: "claude", Reload: "/reload-plugins or a new session"},
			"updated v0.7.1 -> v0.8.0 on claude; this session keeps the v0.7.1 plugin until /reload-plugins or a new session; the new plugin's bin/loops fetches the v0.8.0 binary on first use"},
		{"pending", updatecheck.Outcome{Kind: updatecheck.KindPending, Latest: "v0.8.0", At: at, Reload: "a new session"},
			"v0.8.0 was installed at 2026-11-12T10:00:00Z; this session keeps the v0.7.1 plugin until a new session"},
		{"failed", updatecheck.Outcome{Kind: updatecheck.KindFailed, Latest: "v0.8.0", Detail: "boom", Manual: "do it"},
			"update to v0.8.0 failed (boom); continuing on v0.7.1; to update by hand: do it"},
		{"backoff", updatecheck.Outcome{Kind: updatecheck.KindBackoff, Latest: "v0.8.0", At: at, RetryAt: retry, Detail: "boom", Manual: "do it"},
			"update to v0.8.0 failed at 2026-11-12T10:00:00Z (boom); next try after 2026-11-13T10:00:00Z; to update by hand: do it"},
		{"manual", updatecheck.Outcome{Kind: updatecheck.KindManual, Latest: "v0.8.0", Platform: "codex", Manual: "m"},
			"v0.8.0 available (running v0.7.1); no automatic update on codex; to update: m"},
		{"manual blank", updatecheck.Outcome{Kind: updatecheck.KindManual, Latest: "v0.8.0", Manual: "m"},
			"v0.8.0 available (running v0.7.1); no automatic update on this platform; to update: m"},
		{"check-failed", updatecheck.Outcome{Kind: updatecheck.KindCheckFailed, Detail: "offline"}, "check failed (offline); continuing on v0.7.1"},
		{"unknown", updatecheck.Outcome{Kind: "weird"}, "weird"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			applyEnv(t)
			root := t.TempDir()
			f := &fakeApplier{out: c.o}
			a := &App{Version: "0.7.1", PluginRoot: root, Plugins: fakePlugins{root: "0.7.1"}, Applier: f}
			code, out, errs := runApp(a, "update", "--apply")
			want := "loops update: " + c.want + "\n"
			if code != 0 || out != want || errs != "" {
				t.Fatalf("code=%d\n got %q\nwant %q err=%q", code, out, want, errs)
			}
		})
	}
}

func TestApplyPlatformAndBudget(t *testing.T) {
	applyEnv(t)
	f := &fakeApplier{}
	runApp(&App{Version: "0.7.1", Applier: f}, "update", "--apply", "--platform", "COPILOT")
	if f.platform != "copilot" {
		t.Fatal(f.platform)
	}
	t.Setenv("CLAUDE_PROJECT_DIR", "/x")
	before := time.Now()
	runApp(&App{Version: "0.7.1", Applier: f}, "update", "--apply")
	if f.platform != "claude" {
		t.Fatal(f.platform)
	}
	if !f.hasDL || f.deadline.After(before.Add(applyBudget+time.Second)) || f.deadline.Before(before) {
		t.Fatalf("deadline %v", f.deadline)
	}
}

func TestApplyUsageStillExits2(t *testing.T) {
	applyEnv(t)
	if c, _, e := runApp(&App{Applier: &fakeApplier{}}, "update", "--apply", "extra"); c != 2 || !strings.Contains(e, "[--apply]") {
		t.Fatalf("%d %q", c, e)
	}
}

func TestApplyPluginVersionWins(t *testing.T) {
	applyEnv(t)
	root := t.TempDir()
	f := &fakeApplier{out: updatecheck.Outcome{Kind: updatecheck.KindUpToDate}}
	a := &App{Version: "0.9.5", PluginRoot: root, Plugins: fakePlugins{root: "0.7.1"}, Applier: f}
	_, out, _ := runApp(a, "update", "--apply")
	want := "loops update: up to date (v0.7.1) [this loops binary is v0.9.5; call " + root + "/bin/loops]\n"
	if f.current != "0.7.1" || out != want {
		t.Fatalf("current=%q out=%q", f.current, out)
	}
	a.Version = "v0.7.1"
	_, out, _ = runApp(a, "update", "--apply")
	if out != "loops update: up to date (v0.7.1)\n" {
		t.Fatal(out)
	}
}

func TestApplyRootOrder(t *testing.T) {
	proj, state := applyEnv(t)
	flag, projRoot, stateRoot := t.TempDir(), t.TempDir(), t.TempDir()
	pointer(t, filepath.Join(proj, ".loops", "plugin-root"), projRoot)
	pointer(t, filepath.Join(state, "loops", "plugin-root"), stateRoot)
	plugins := fakePlugins{flag: "1.0.0", projRoot: "2.0.0", stateRoot: "3.0.0"}
	mk := func(pr string) string {
		f := &fakeApplier{}
		runApp(&App{Version: "0.1.0", Root: proj, PluginRoot: pr, Plugins: plugins, Applier: f}, "update", "--apply")
		return f.current
	}
	if got := mk(flag); got != "1.0.0" {
		t.Fatal("PluginRoot should win:", got)
	}
	if got := mk(""); got != "2.0.0" {
		t.Fatal("project pointer should win:", got)
	}
	pointer(t, filepath.Join(proj, ".loops", "plugin-root"), filepath.Join(proj, "gone"))
	if got := mk(""); got != "3.0.0" {
		t.Fatal("missing dir should fall through:", got)
	}
	if got := mk(filepath.Join(proj, "nope")); got != "3.0.0" {
		t.Fatal("stale PluginRoot should fall through:", got)
	}
}

func TestApplyEmptyVersionSkips(t *testing.T) {
	applyEnv(t)
	t.Setenv("HOME", t.TempDir())
	f := &fakeApplier{}
	_, out, _ := runApp(&App{Applier: f}, "update", "--apply")
	if out != "loops update: skipped (plugin version unknown)\n" || f.calls != 0 {
		t.Fatalf("%q calls=%d", out, f.calls)
	}
}

func TestApplyBinaryFallback(t *testing.T) {
	proj, _ := applyEnv(t)
	t.Setenv("HOME", t.TempDir())
	f := &fakeApplier{out: updatecheck.Outcome{Kind: updatecheck.KindUpToDate}}
	_, out, _ := runApp(&App{Version: "0.9.5", Root: proj, Applier: f}, "update", "--apply")
	if f.current != "0.9.5" || out != "loops update: up to date (v0.9.5) [version of this loops binary: plugin root unknown]\n" {
		t.Fatalf("%q %q", f.current, out)
	}
	root := t.TempDir()
	f = &fakeApplier{out: updatecheck.Outcome{Kind: updatecheck.KindUpToDate}}
	_, out, _ = runApp(&App{Version: "0.9.5", PluginRoot: root, Plugins: fakePlugins{}, Applier: f}, "update", "--apply")
	if f.current != "0.9.5" || out != "loops update: up to date (v0.9.5) [version of this loops binary: no plugin.json at "+root+"]\n" {
		t.Fatalf("%q %q", f.current, out)
	}
	f = &fakeApplier{out: updatecheck.Outcome{Kind: updatecheck.KindUpToDate}}
	_, out, _ = runApp(&App{Version: "0.9.5", PluginRoot: root, Applier: f}, "update", "--apply")
	if !strings.Contains(out, "no plugin.json at "+root) {
		t.Fatal(out)
	}
}
