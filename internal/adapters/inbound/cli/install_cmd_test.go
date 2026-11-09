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
	if code := a.Run(context.Background(), []string{"install", "--target", "vim"}); code != 2 || !strings.Contains(errb.String(), "unknown platform") {
		t.Errorf("%d %q", code, errb)
	}
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
