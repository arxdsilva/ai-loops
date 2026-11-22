package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/portfs"
	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
)

func stubBuild(files []plugin.File, err error) packageBuilder {
	return func(fs.FS, plugin.Platform) ([]plugin.File, []string, error) {
		return files, []string{"lossy thing"}, err
	}
}

func runInstall(a *App, build packageBuilder, args ...string) int { return a.install(args, build) }

func installApp(root string) (*App, *bytes.Buffer, *bytes.Buffer) {
	out, errb := &bytes.Buffer{}, &bytes.Buffer{}
	return &App{Source: fstest.MapFS{}, Packages: portfs.New(), Root: root, Out: out, Err: errb}, out, errb
}

func TestInstallPlacesAndSkips(t *testing.T) {
	root := t.TempDir()
	build := stubBuild([]plugin.File{{Path: "agents/a.toml", Data: []byte("A")}}, nil)
	a, out, errb := installApp(root)
	if code := runInstall(a, build, "--target", "codex"); code != 0 {
		t.Fatalf("exit %d: %s", code, errb)
	}
	dest := filepath.Join(root, ".codex/agents/a.toml")
	if b, _ := os.ReadFile(dest); string(b) != "A" {
		t.Fatal("not placed")
	}
	for _, w := range []string{"1 written", "/hooks", "marketplace add"} {
		if !strings.Contains(out.String(), w) {
			t.Errorf("missing %q in %q", w, out)
		}
	}
	if !strings.Contains(errb.String(), "lossy thing") {
		t.Errorf("warnings not shown: %q", errb)
	}
	os.WriteFile(dest, []byte("mine"), 0o644)
	out.Reset()
	runInstall(a, build, "--target", "codex")
	if b, _ := os.ReadFile(dest); string(b) != "mine" || !strings.Contains(out.String(), "1 skipped") {
		t.Errorf("overwrote without --force: %q", out)
	}
	runInstall(a, build, "--target", "codex", "--force")
	if b, _ := os.ReadFile(dest); string(b) != "A" {
		t.Error("--force did not overwrite")
	}
}

func TestInstallDryRunUserScopeAndProjectFlag(t *testing.T) {
	home, proj := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	build := stubBuild([]plugin.File{{Path: "agents/a.toml", Data: []byte("A")}}, nil)
	a, out, _ := installApp("/ignored")
	if code := runInstall(a, build, "--target", "codex", "--scope", "user", "--dry-run"); code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out.String(), filepath.Join(home, ".codex/agents/a.toml")) || !strings.Contains(out.String(), "dry run") {
		t.Errorf("out: %q", out)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex")); !os.IsNotExist(err) {
		t.Error("dry run wrote files")
	}
	out.Reset()
	runInstall(a, build, "--target", "codex", "--project", proj, "--dry-run")
	if !strings.Contains(out.String(), filepath.Join(proj, ".codex/agents/a.toml")) {
		t.Errorf("--project ignored: %q", out)
	}
}

func TestInstallNativePlatformPrintsCommand(t *testing.T) {
	build := stubBuild(nil, nil)
	a, out, _ := installApp(t.TempDir())
	if code := runInstall(a, build, "--target", "claude"); code != 0 || !strings.Contains(out.String(), "/plugin marketplace add") {
		t.Errorf("%d %q", code, out)
	}
}

func TestInstallErrors(t *testing.T) {
	build := stubBuild(nil, nil)
	for _, args := range [][]string{{}, {"--target", "vim"}, {"--target", "codex", "--scope", "x"}, {"--bogus"}, {"--target", "codex", "extra"}} {
		a, _, errb := installApp(t.TempDir())
		if code := a.install(args, build); code != 2 || errb.Len() == 0 {
			t.Errorf("%v: exit %d", args, code)
		}
	}
	build = stubBuild(nil, errors.New("boom"))
	a, _, errb := installApp(t.TempDir())
	if code := runInstall(a, build, "--target", "codex"); code != 1 || !strings.Contains(errb.String(), "boom") {
		t.Errorf("build error: %d %q", code, errb)
	}
	a.Source = nil
	if code := runInstall(a, build, "--target", "codex"); code != 1 {
		t.Error("nil source")
	}
}

func TestPathFallsBackToInstalledPluginHome(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	a, out, errb := installApp(root)
	if code := a.Run(context.Background(), []string{"path"}); code != 1 || errb.Len() == 0 {
		t.Fatalf("nothing installed: %d", code)
	}
	userHome := filepath.Join(home, ".local/share/loops/plugin")
	os.MkdirAll(filepath.Join(userHome, "specs"), 0o755)
	if a.Run(context.Background(), []string{"path", "specs"}); strings.TrimSpace(out.String()) != userHome+"/specs" {
		t.Errorf("user home: %q", out)
	}
	out.Reset()
	projHome := filepath.Join(root, ".loops/plugin")
	os.MkdirAll(filepath.Join(projHome, "specs"), 0o755)
	a.Run(context.Background(), []string{"path"})
	if strings.TrimSpace(out.String()) != projHome {
		t.Errorf("project home wins: %q", out)
	}
	out.Reset()
	a.PluginRoot = "/explicit"
	a.Run(context.Background(), []string{"path"})
	if strings.TrimSpace(out.String()) != "/explicit" {
		t.Errorf("explicit root must win: %q", out)
	}
}

func TestInstallDispatch(t *testing.T) {
	a, _, errb := installApp(t.TempDir())
	if code := a.Run(context.Background(), []string{"install", "--target", "vim"}); code != 2 || !strings.Contains(errb.String(), "unknown platform") || !strings.Contains(errb.String(), "antigravity") {
		t.Errorf("%d %q", code, errb)
	}
}

func TestInstallAntigravityHintsAndStaleDirs(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	mk := func(names ...string) packageBuilder {
		var fs []plugin.File
		for _, n := range names {
			fs = append(fs, plugin.File{Path: n, Data: []byte("loops hook " + n)})
		}
		return stubBuild(fs, nil)
	}
	assertExists := func(t *testing.T, paths ...string) {
		t.Helper()
		for _, p := range paths {
			if _, err := os.Stat(p); err != nil {
				t.Fatalf("missing expected file %s: %v", p, err)
			}
		}
	}
	assertRemoved := func(t *testing.T, out string, paths ...string) {
		t.Helper()
		if !strings.Contains(out, "removed stale") {
			t.Errorf("missing removed stale output: %q", out)
		}
		for _, p := range paths {
			if _, err := os.Stat(p); !os.IsNotExist(err) {
				t.Errorf("stale file kept %s: err=%v, out=%q", p, err, out)
			}
		}
	}

	// Project scope: places under .agents/, prints Antigravity hint, removes stale .agents/ files.
	a, out, errb := installApp(root)
	if code := runInstall(a, mk("hooks.json", "skills/a/SKILL.md", "skills/b/SKILL.md", "agents/planner.md", "agents/old.md", "rules/AGENTS.md", "rules/OLD.md", "ROLES.md"), "--target", "antigravity"); code != 0 {
		t.Fatalf("project install %d: %s", code, errb)
	}
	if !strings.Contains(out.String(), "next: Antigravity:") || !strings.Contains(out.String(), "/skills reload") {
		t.Errorf("missing antigravity hint in %q", out)
	}
	projKeep := []string{
		filepath.Join(root, ".agents/hooks.json"),
		filepath.Join(root, ".agents/skills/a/SKILL.md"),
		filepath.Join(root, ".agents/agents/planner.md"),
		filepath.Join(root, ".agents/rules/AGENTS.md"),
	}
	projStale := []string{
		filepath.Join(root, ".agents/skills/b/SKILL.md"),
		filepath.Join(root, ".agents/agents/old.md"),
		filepath.Join(root, ".agents/rules/OLD.md"),
	}
	assertExists(t, append(projKeep, projStale...)...)
	out.Reset()
	if code := runInstall(a, mk("hooks.json", "skills/a/SKILL.md", "agents/planner.md", "rules/AGENTS.md", "ROLES.md"), "--target", "antigravity"); code != 0 {
		t.Fatalf("project reinstall %d: %s", code, errb)
	}
	assertRemoved(t, out.String(), projStale...)
	assertExists(t, projKeep...)

	// User scope: places under ~/.gemini/config/, removes stale ~/.gemini/config/ files.
	a, out, errb = installApp(root)
	if code := runInstall(a, mk("hooks.json", "skills/a/SKILL.md", "skills/b/SKILL.md", "agents/planner.md", "agents/old.md", "rules/AGENTS.md", "rules/OLD.md", "ROLES.md"), "--target", "antigravity", "--scope", "user"); code != 0 {
		t.Fatalf("user install %d: %s", code, errb)
	}
	userKeep := []string{
		filepath.Join(home, ".gemini/config/hooks.json"),
		filepath.Join(home, ".gemini/config/skills/a/SKILL.md"),
		filepath.Join(home, ".gemini/config/agents/planner.md"),
		filepath.Join(home, ".gemini/config/rules/AGENTS.md"),
	}
	userStale := []string{
		filepath.Join(home, ".gemini/config/skills/b/SKILL.md"),
		filepath.Join(home, ".gemini/config/agents/old.md"),
		filepath.Join(home, ".gemini/config/rules/OLD.md"),
	}
	assertExists(t, append(userKeep, userStale...)...)
	out.Reset()
	if code := runInstall(a, mk("hooks.json", "skills/a/SKILL.md", "agents/planner.md", "rules/AGENTS.md", "ROLES.md"), "--target", "antigravity", "--scope", "user"); code != 0 {
		t.Fatalf("user reinstall %d: %s", code, errb)
	}
	assertRemoved(t, out.String(), userStale...)
	assertExists(t, userKeep...)
}

func TestPathPrefersPluginRootFile(t *testing.T) {
	root, home, other := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	os.MkdirAll(filepath.Join(other, "specs"), 0o755)
	os.MkdirAll(filepath.Join(root, ".loops/plugin/specs"), 0o755)
	os.WriteFile(filepath.Join(root, ".loops/plugin-root"), []byte(other+"\n"), 0o644)
	a, out, _ := installApp(root)
	a.Run(context.Background(), []string{"path"})
	if strings.TrimSpace(out.String()) != other {
		t.Errorf("plugin-root file must win: %q", out)
	}
	os.RemoveAll(filepath.Join(other, "specs"))
	out.Reset()
	a.Run(context.Background(), []string{"path"})
	if strings.TrimSpace(out.String()) != filepath.Join(root, ".loops/plugin") {
		t.Errorf("fallback when specs/ missing: %q", out)
	}
}

func TestInstallRemovesStaleAndProtectsForeignConfig(t *testing.T) {
	root := t.TempDir()
	mk := func(names ...string) packageBuilder {
		var fs []plugin.File
		for _, n := range names {
			fs = append(fs, plugin.File{Path: n, Data: []byte("loops hook x " + n)})
		}
		return stubBuild(fs, nil)
	}
	a, out, _ := installApp(root)
	if code := runInstall(a, mk(".agents/skills/a/SKILL.md", ".agents/skills/b/SKILL.md"), "--target", "amp"); code != 0 {
		t.Fatal(code)
	}
	b := filepath.Join(root, ".agents/skills/b/SKILL.md")
	if _, err := os.Stat(b); err != nil {
		t.Fatal("not placed")
	}
	if _, err := os.Stat(filepath.Join(root, ".loops/plugin/installed-amp.json")); err != nil {
		t.Fatal("manifest missing")
	}
	runInstall(a, mk(".agents/skills/a/SKILL.md"), "--target", "amp")
	if _, err := os.Stat(b); !os.IsNotExist(err) || !strings.Contains(out.String(), "removed stale") {
		t.Errorf("stale file kept: %q", out)
	}
	// foreign config is never overwritten, even with --force
	cfg := filepath.Join(root, ".devin/hooks.v1.json")
	os.MkdirAll(filepath.Dir(cfg), 0o755)
	os.WriteFile(cfg, []byte(`{"mine":true}`), 0o644)
	out.Reset()
	runInstall(a, mk(".devin/hooks.v1.json"), "--target", "windsurf", "--force")
	if got, _ := os.ReadFile(cfg); string(got) != `{"mine":true}` || !strings.Contains(out.String(), "not created by loops") {
		t.Errorf("foreign config clobbered: %s / %q", got, out)
	}
	// a loops-created one is updatable with --force
	os.WriteFile(cfg, []byte(`loops hook old`), 0o644)
	runInstall(a, mk(".devin/hooks.v1.json"), "--target", "windsurf", "--force")
	if got, _ := os.ReadFile(cfg); !strings.Contains(string(got), "hooks.v1.json") {
		t.Errorf("not updated: %s", got)
	}
}

func writeManifest(t *testing.T, root, name string, entries []manifestEntry) {
	t.Helper()
	b, _ := json.Marshal(installManifest{Files: entries})
	os.MkdirAll(filepath.Join(root, ".loops/plugin"), 0o755)
	os.WriteFile(filepath.Join(root, ".loops/plugin", name), b, 0o644)
}

func TestInstallIgnoresCraftedManifests(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	victim := filepath.Join(outside, "victim")
	os.WriteFile(victim, []byte("v"), 0o644)
	inRoot := filepath.Join(root, "precious.txt")
	os.WriteFile(inRoot, []byte("p"), 0o644)
	// symlinked platform dir pointing outside
	os.MkdirAll(filepath.Join(root, ".agents"), 0o755)
	os.Symlink(outside, filepath.Join(root, ".agents/skills"))
	// right place, wrong hash
	os.MkdirAll(filepath.Join(root, ".devin"), 0o755)
	changed := filepath.Join(root, ".agents-note")
	_ = changed
	os.WriteFile(filepath.Join(root, ".devin/user.json"), []byte("mine"), 0o644)
	writeManifest(t, root, "installed-amp.json", []manifestEntry{
		{victim, sum([]byte("v"))},
		{"../" + filepath.Base(outside) + "/victim", sum([]byte("v"))},
		{"precious.txt", sum([]byte("p"))},
		{".agents/skills/victim", sum([]byte("v"))},
		{".devin/user.json", sum([]byte("other"))},
		{".agents/../precious.txt", sum([]byte("p"))},
	})
	a, out, _ := installApp(root)
	build := stubBuild([]plugin.File{{Path: ".agents/skills/x/SKILL.md", Data: []byte("x")}}, nil)
	_ = out
	if code := runInstall(a, build, "--target", "amp"); code != 0 {
		t.Fatal(code)
	}
	for _, f := range []string{victim, inRoot, filepath.Join(root, ".devin/user.json")} {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("deleted %s", f)
		}
	}
	// malformed manifest
	os.WriteFile(filepath.Join(root, ".loops/plugin/installed-amp.json"), []byte("{nope"), 0o644)
	out.Reset()
	runInstall(a, build, "--target", "amp")
	if !strings.Contains(out.String(), "malformed") {
		t.Errorf("no note: %q", out)
	}
}

func TestInstallManifestsArePerPlatform(t *testing.T) {
	root := t.TempDir()
	a, _, _ := installApp(root)
	oc := stubBuild([]plugin.File{{Path: ".opencode/agents/x.md", Data: []byte("o")}, {Path: ".agents/skills/s/SKILL.md", Data: []byte("s")}}, nil)
	amp := stubBuild([]plugin.File{{Path: ".agents/skills/t/SKILL.md", Data: []byte("t")}}, nil)
	runInstall(a, oc, "--target", "opencode")
	runInstall(a, amp, "--target", "amp")
	for _, f := range []string{".opencode/agents/x.md", ".agents/skills/s/SKILL.md", ".agents/skills/t/SKILL.md"} {
		if _, err := os.Stat(filepath.Join(root, f)); err != nil {
			t.Errorf("%s lost after installing another platform", f)
		}
	}
	// same-platform reinstall removes its own stale file
	runInstall(a, stubBuild([]plugin.File{{Path: ".opencode/agents/y.md", Data: []byte("y")}}, nil), "--target", "opencode")
	if _, err := os.Stat(filepath.Join(root, ".opencode/agents/x.md")); !os.IsNotExist(err) {
		t.Error("own stale file kept")
	}
	if _, err := os.Stat(filepath.Join(root, ".agents/skills/t/SKILL.md")); err != nil {
		t.Error("amp file removed by opencode reinstall")
	}
}

func TestInstallNativeSkipsBuild(t *testing.T) {
	a, out, errb := installApp(t.TempDir())
	boom := func(fs.FS, plugin.Platform) ([]plugin.File, []string, error) {
		t.Error("build called for a native platform")
		return nil, []string{"warn"}, nil
	}
	if code := a.install([]string{"--target", "gemini"}, boom); code != 0 || errb.Len() != 0 || !strings.Contains(out.String(), "platforms/gemini") {
		t.Errorf("%d %q %q", code, out, errb)
	}
}

func TestInstallRefusesSymlinkedProjectPath(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	os.Symlink(outside, filepath.Join(root, ".agents"))
	a, out, _ := installApp(root)
	build := stubBuild([]plugin.File{{Path: ".agents/skills/x/SKILL.md", Data: []byte("x")}}, nil)
	for _, args := range [][]string{{"--target", "amp"}, {"--target", "amp", "--force"}} {
		runInstall(a, build, args...)
		if ents, _ := os.ReadDir(outside); len(ents) != 0 {
			t.Fatalf("wrote through symlink with %v: %v", args, ents)
		}
	}
	if !strings.Contains(out.String(), "crosses a symlink") {
		t.Errorf("no note: %q", out)
	}
}

func TestInstallUserScopeSymlinks(t *testing.T) {
	home, outside, inside := t.TempDir(), t.TempDir(), "dotfiles"
	t.Setenv("HOME", home)
	os.MkdirAll(filepath.Join(home, inside), 0o755)
	os.Symlink(filepath.Join(home, inside), filepath.Join(home, ".agents"))
	a, _, _ := installApp(t.TempDir())
	build := stubBuild([]plugin.File{{Path: ".agents/skills/x/SKILL.md", Data: []byte("x")}}, nil)
	runInstall(a, build, "--target", "amp", "--scope", "user", "--force")
	if _, err := os.Stat(filepath.Join(home, inside, "skills/x/SKILL.md")); err != nil {
		t.Error("in-home symlink should be followed")
	}
	os.Remove(filepath.Join(home, ".agents"))
	os.Symlink(outside, filepath.Join(home, ".agents"))
	runInstall(a, build, "--target", "amp", "--scope", "user", "--force")
	if ents, _ := os.ReadDir(outside); len(ents) != 0 {
		t.Errorf("--force wrote outside $HOME: %v", ents)
	}
}

func TestPathFallsBackToStatePointer(t *testing.T) {
	root, home, state, plug := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", state)
	a, out, _ := installApp(root)
	os.MkdirAll(filepath.Join(state, "loops"), 0o755)
	os.WriteFile(filepath.Join(state, "loops/plugin-root"), []byte(plug), 0o644)
	if code := a.Run(context.Background(), []string{"path"}); code != 1 {
		t.Errorf("pointer without specs/ must be ignored: %d", code)
	}
	os.MkdirAll(filepath.Join(plug, "specs"), 0o755)
	a.Run(context.Background(), []string{"path"})
	if strings.TrimSpace(out.String()) != plug {
		t.Errorf("%q", out)
	}
}

func TestInstallAntigravityIdempotent(t *testing.T) {
	repoRoot, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, ".claude-plugin", "plugin.json")); err != nil {
		t.Skip("repository root not found")
	}

	cases := []struct {
		name      string
		scope     string
		wantFiles func(proj, home string) []string
	}{
		{
			name:  "project scope",
			scope: "project",
			wantFiles: func(proj, _ string) []string {
				return []string{
					filepath.Join(proj, ".agents", "hooks.json"),
					filepath.Join(proj, ".agents", "skills", "work", "SKILL.md"),
					filepath.Join(proj, ".agents", "agents", "planner.md"),
					filepath.Join(proj, ".agents", "rules", "AGENTS.md"),
					filepath.Join(proj, ".loops", "plugin", "specs", "languages", "go", "pack.json"),
					filepath.Join(proj, ".loops", "plugin", "bin", "loops"),
					filepath.Join(proj, ".loops", "plugin", "installed-antigravity.json"),
				}
			},
		},
		{
			name:  "user scope",
			scope: "user",
			wantFiles: func(_, home string) []string {
				return []string{
					filepath.Join(home, ".gemini", "config", "hooks.json"),
					filepath.Join(home, ".gemini", "config", "skills", "work", "SKILL.md"),
					filepath.Join(home, ".gemini", "config", "agents", "planner.md"),
					filepath.Join(home, ".gemini", "config", "rules", "AGENTS.md"),
					filepath.Join(home, ".local", "share", "loops", "plugin", "specs", "languages", "go", "pack.json"),
					filepath.Join(home, ".local", "share", "loops", "plugin", "bin", "loops"),
					filepath.Join(home, ".local", "share", "loops", "plugin", "installed-antigravity.json"),
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proj, home := t.TempDir(), t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
			t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))

			out, errb := &bytes.Buffer{}, &bytes.Buffer{}
			a := &App{Source: os.DirFS(repoRoot), Packages: portfs.New(), Root: proj, Out: out, Err: errb}
			args := []string{"install", "--target", "antigravity", "--scope", tc.scope, "--project", proj}
			if code := a.Run(context.Background(), args); code != 0 {
				t.Fatalf("run 1 exit %d: %s", code, errb)
			}
			for _, f := range tc.wantFiles(proj, home) {
				if _, err := os.Stat(f); err != nil {
					t.Fatalf("run 1 missing %s: %v", f, err)
				}
			}

			out.Reset()
			errb.Reset()
			if code := a.Run(context.Background(), args); code != 0 {
				t.Fatalf("run 2 exit %d: %s", code, errb)
			}
			got := out.String()
			for _, bad := range []string{"removed stale", "not created by loops"} {
				if strings.Contains(got, bad) {
					t.Errorf("run 2 output unexpectedly contains %q:\n%s", bad, got)
				}
			}
			if !strings.Contains(got, "0 written,") || strings.Contains(got, "0 skipped") {
				t.Errorf("run 2 expected 0 written and all skipped:\n%s", got)
			}
			for _, f := range tc.wantFiles(proj, home) {
				if _, err := os.Stat(f); err != nil {
					t.Errorf("run 2 deleted %s: %v", f, err)
				}
			}
		})
	}
}

func TestProtectForeignConfigs(t *testing.T) {
	base := t.TempDir()
	pluginHome := filepath.Join(base, ".loops", "plugin")
	manifest := filepath.Join(pluginHome, "installed-antigravity.json")

	// 1. Inside pluginHome (.json without "loops hook", even with different content) -> kept.
	packJSON := filepath.Join(pluginHome, "specs", "languages", "go", "pack.json")
	os.MkdirAll(filepath.Dir(packJSON), 0o755)
	os.WriteFile(packJSON, []byte(`{"id":"go","old":true}`), 0o644)

	// 2. Outside pluginHome, matching p.File.Data exactly -> kept.
	exactJSON := filepath.Join(base, ".agents", "exact.json")
	os.MkdirAll(filepath.Dir(exactJSON), 0o755)
	os.WriteFile(exactJSON, []byte(`{"exact":1}`), 0o644)

	// 3. Outside pluginHome, pinned shim with JSON-escaped quote (bin/loops\" hook) -> kept.
	escapedShim := filepath.Join(base, ".agents", "hooks.json")
	os.WriteFile(escapedShim, []byte(`{"cmd":"\"/tmp/bin/loops\" hook stop-gate --platform antigravity"}`), 0o644)

	// 4. Outside pluginHome, unescaped quote (bin/loops" hook) -> kept.
	unescapedShim := filepath.Join(base, ".agents", "hooks-raw.json")
	os.WriteFile(unescapedShim, []byte(`{"cmd":"/tmp/bin/loops" hook stop-gate"}`), 0o644)

	// 5. Outside pluginHome, hash recorded in manifest (even if p.File.Data changed) -> kept.
	recordedJSON := filepath.Join(base, ".agents", "recorded.json")
	recordedData := []byte(`{"v":1}`)
	os.WriteFile(recordedJSON, recordedData, 0o644)
	writeManifest(t, base, "installed-antigravity.json", []manifestEntry{
		{Path: ".agents/recorded.json", SHA256: sum(recordedData)},
	})

	// 6. Outside pluginHome, foreign content -> protected.
	foreignJSON := filepath.Join(base, ".agents", "foreign.json")
	os.WriteFile(foreignJSON, []byte(`{"userConfig":true}`), 0o644)

	// 7. Non-json and missing json -> kept.
	nonJSON := filepath.Join(base, ".agents", "note.md")
	os.WriteFile(nonJSON, []byte(`user md`), 0o644)
	missingJSON := filepath.Join(base, ".agents", "missing.json")

	ps := []plugin.Placement{
		{Dest: packJSON, File: plugin.File{Path: "specs/languages/go/pack.json", Data: []byte(`{"id":"go"}`)}},
		{Dest: exactJSON, File: plugin.File{Path: ".agents/exact.json", Data: []byte(`{"exact":1}`)}},
		{Dest: escapedShim, File: plugin.File{Path: ".agents/hooks.json", Data: []byte(`{"new":true}`)}},
		{Dest: unescapedShim, File: plugin.File{Path: ".agents/hooks-raw.json", Data: []byte(`{"new":true}`)}},
		{Dest: recordedJSON, File: plugin.File{Path: ".agents/recorded.json", Data: []byte(`{"v":2}`)}},
		{Dest: foreignJSON, File: plugin.File{Path: ".agents/foreign.json", Data: []byte(`{"loops":true}`)}},
		{Dest: nonJSON, File: plugin.File{Path: ".agents/note.md", Data: []byte(`md`)}},
		{Dest: missingJSON, File: plugin.File{Path: ".agents/missing.json", Data: []byte(`{}`)}},
	}

	keep, protected := protectForeignConfigs(ps, pluginHome, manifest, base)
	if len(protected) != 1 || protected[0] != foreignJSON {
		t.Fatalf("protected = %v, want [%s]", protected, foreignJSON)
	}
	if len(keep) != 7 {
		t.Fatalf("keep len = %d, want 7", len(keep))
	}

	// Empty pluginHome and malformed manifest still protect foreign JSON.
	os.WriteFile(manifest, []byte("{bad"), 0o644)
	keep2, prot2 := protectForeignConfigs([]plugin.Placement{{Dest: foreignJSON, File: plugin.File{Data: []byte(`{}`)}}}, "", manifest, base)
	if len(keep2) != 0 || len(prot2) != 1 {
		t.Fatalf("empty pluginHome/malformed manifest: keep=%v prot=%v", keep2, prot2)
	}
}

func TestInstallEdgeBranches(t *testing.T) {
	// Missing HOME in user scope -> resolveInstallDirs error.
	t.Setenv("HOME", "")
	a, out, errb := installApp(t.TempDir())
	if code := runInstall(a, stubBuild(nil, nil), "--target", "codex", "--scope", "user"); code != 1 || errb.Len() == 0 {
		t.Errorf("missing HOME: %d %q", code, errb)
	}

	// Empty Root and no --project -> port.InstallPlan project dir error.
	a, _, errb = installApp("")
	if code := runInstall(a, stubBuild(nil, nil), "--target", "codex"); code != 1 || !strings.Contains(errb.String(), "project directory") {
		t.Errorf("empty root: %d %q", code, errb)
	}

	// Place error in writePlacements.
	root := t.TempDir()
	out.Reset()
	errb.Reset()
	a = &App{Source: fstest.MapFS{}, Packages: &memPackages{placeErr: errors.New("disk full")}, Root: root, Out: out, Err: errb}
	if code := runInstall(a, stubBuild([]plugin.File{{Path: "agents/a.toml", Data: []byte("A")}}, nil), "--target", "codex"); code != 1 || !strings.Contains(errb.String(), "disk full") {
		t.Errorf("place err: %d %q", code, errb)
	}

	// writeInstallManifest logs Place error.
	out.Reset()
	errb.Reset()
	writeInstallManifest(errb, &memPackages{placeErr: errors.New("manifest fail")}, installOpts{base: root, scope: "project"}, filepath.Join(root, ".loops/plugin/installed-codex.json"), nil)
	if !strings.Contains(errb.String(), "writing manifest: manifest fail") {
		t.Errorf("manifest err: %q", errb)
	}

	// removeStale skips missing / directory entries.
	out.Reset()
	dir := filepath.Join(root, "adir")
	os.MkdirAll(dir, 0o755)
	removeStale(out, []string{dir, filepath.Join(root, "nope")})
	if out.Len() != 0 {
		t.Errorf("removeStale unexpected output: %q", out)
	}

	// safeEntry missing file, guardSymlinks outside base, non-existent base, root file, direct file symlink.
	missingRoot := filepath.Join(root, "does-not-exist")
	outside := t.TempDir()
	directLink := filepath.Join(root, "top.txt")
	os.Symlink(filepath.Join(outside, "target"), directLink)
	kept1, notes1 := guardSymlinks(missingRoot, "project", false, []plugin.Placement{
		{Dest: filepath.Join(outside, "outside.txt")},
		{Dest: filepath.Join(missingRoot, "root.txt")},
	})
	if len(kept1) != 2 || len(notes1) != 0 {
		t.Errorf("guardSymlinks missingBase/outside: kept=%d notes=%v", len(kept1), notes1)
	}
	kept2, notes2 := guardSymlinks(root, "project", false, []plugin.Placement{{Dest: directLink}})
	if len(kept2) != 0 || len(notes2) != 1 {
		t.Errorf("guardSymlinks direct symlink: kept=%d notes=%v", len(kept2), notes2)
	}
	if _, why := safeEntry(root, manifestEntry{Path: ".agents/skills/gone", SHA256: "abc"}, []string{".agents/skills"}); why != "already gone" {
		t.Errorf("safeEntry gone = %q", why)
	}
}
