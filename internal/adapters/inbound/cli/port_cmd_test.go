package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/arxdsilva/ai-loops/internal/application/port"
	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
)

// memPackages is an in-memory PackageWriter.
type memPackages struct {
	written  map[string][]plugin.File
	placed   []plugin.Placement
	force    bool
	diff     map[string][]string
	writeErr error
	diffErr  error
	placeErr error
}

func (m *memPackages) Write(dir string, files []plugin.File) error {
	if m.writeErr != nil {
		return m.writeErr
	}
	if m.written == nil {
		m.written = map[string][]plugin.File{}
	}
	m.written[dir] = files
	return nil
}

func (m *memPackages) Diff(dir string, files []plugin.File) ([]string, error) {
	return m.diff[dir], m.diffErr
}

func (m *memPackages) Place(p []plugin.Placement, force bool) ([]string, error) {
	if m.placeErr != nil {
		return nil, m.placeErr
	}
	m.placed, m.force = append(m.placed, p...), force
	var out []string
	for _, x := range p {
		out = append(out, x.Dest)
	}
	return out, nil
}

func portSource() fstest.MapFS {
	f := func(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s)} }
	return fstest.MapFS{
		".claude-plugin/plugin.json":  f(`{"name":"loops","version":"0.3.0","description":"d","author":{"name":"A"}}`),
		"skills/work/SKILL.md":        f("---\nname: work\ndescription: Work.\n---\nbody\n"),
		"agents/planner.md":           f("---\nname: planner\ndescription: Plan.\nmodel: opus\ndisallowedTools: Edit\n---\nPlan.\n"),
		"hooks/hooks.json":            f(`{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"loops hook stop-gate","timeout":10}]}]}}`),
		"templates/PLUGIN_CONTEXT.md": f("ctx\n"),
	}
}

func portApp(t *testing.T, src fstest.MapFS) (*App, *memPackages, *bytes.Buffer, *bytes.Buffer, string) {
	t.Helper()
	var out, errw bytes.Buffer
	pk := &memPackages{}
	root := t.TempDir()
	a := &App{Source: src, Packages: pk, Root: root, Out: &out, Err: &errw}
	if src == nil {
		a.Source = nil
	}
	return a, pk, &out, &errw, root
}

func TestPortBuild(t *testing.T) {
	a, pk, out, errw, root := portApp(t, portSource())
	code := a.Run(context.Background(), []string{"port", "build", "--platform", "codex,cursor,claude"})
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errw)
	}
	for _, pl := range []string{"codex", "cursor", "claude"} {
		if _, ok := pk.written[filepath.Join(root, "platforms", pl)]; !ok {
			t.Errorf("%s not written: %v", pl, pk.written)
		}
	}
	if len(pk.written) != 3 {
		t.Errorf("wrote %d packages", len(pk.written))
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 4 || !strings.HasPrefix(lines[0], "codex: ") || !strings.HasSuffix(lines[0], " warnings") || lines[3] != "root: 2 files" {
		t.Errorf("stdout = %q", out)
	}
	if !strings.Contains(errw.String(), "warning: codex:") {
		t.Errorf("warnings not printed: %q", errw)
	}
	if !pk.force || len(pk.placed) != 2 {
		t.Fatalf("placements = %+v force=%v", pk.placed, pk.force)
	}
	dests := map[string]bool{}
	for _, p := range pk.placed {
		dests[p.Dest] = true
	}
	if !dests[filepath.Join(root, ".agents/plugins/marketplace.json")] || !dests[filepath.Join(root, ".cursor-plugin/marketplace.json")] {
		t.Errorf("dests = %v", dests)
	}
}

func TestPortBuildOutAndModels(t *testing.T) {
	a, pk, out, errw, _ := portApp(t, portSource())
	dir := t.TempDir()
	models := filepath.Join(dir, "models.json")
	if err := os.WriteFile(models, []byte(`{"codex":{"opus":"gpt-9"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "gen", "platforms")
	if code := a.Run(context.Background(), []string{"port", "build", "--out", outDir, "--platform", "codex", "--models", models}); code != 0 {
		t.Fatalf("exit %d: %s", code, errw)
	}
	files := pk.written[filepath.Join(outDir, "codex")]
	found := false
	for _, f := range files {
		if f.Path == "agents/planner.toml" {
			found = strings.Contains(string(f.Data), `model = "gpt-9"`)
		}
	}
	if !found {
		t.Error("model mapping not applied to agents/planner.toml")
	}
	if len(pk.placed) != 0 {
		t.Errorf("custom --out placed root files: %+v", pk.placed)
	}
	if !strings.Contains(out.String(), "root marketplaces skipped (custom --out)") {
		t.Errorf("stdout lacks the skip note: %q", out)
	}
}

func TestPortCheck(t *testing.T) {
	src := portSource()
	a, pk, out, errw, root := portApp(t, src)
	p, err := port.Load(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range port.RootFiles(p) {
		dest := filepath.Join(root, f.Path)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dest, f.Data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if code := a.Run(context.Background(), []string{"port", "check", "--platform", "codex,cursor"}); code != 0 {
		t.Fatalf("clean check exit %d: %s", code, errw)
	}
	if !strings.Contains(out.String(), "up to date") || len(pk.written) != 0 || len(pk.placed) != 0 {
		t.Errorf("check wrote files or said %q", out)
	}

	pk.diff = map[string][]string{filepath.Join(root, "platforms", "codex"): {"plugin.json"}}
	if err := os.WriteFile(filepath.Join(root, ".cursor-plugin/marketplace.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	errw.Reset()
	if code := a.Run(context.Background(), []string{"port", "check", "--platform", "codex,cursor"}); code != 1 {
		t.Fatalf("drift exit %d", code)
	}
	for _, want := range []string{"run `loops port build`", "platforms/codex/plugin.json", ".cursor-plugin/marketplace.json"} {
		if !strings.Contains(errw.String(), want) {
			t.Errorf("stderr lacks %q: %s", want, errw)
		}
	}
}

func TestPortCheckCustomOutSkipsRoot(t *testing.T) {
	a, _, out, errw, root := portApp(t, portSource())
	// No root marketplaces exist under root: a default check would drift.
	if code := a.Run(context.Background(), []string{"port", "check", "--platform", "codex"}); code != 1 {
		t.Fatalf("default check exit %d, want 1 (missing root files)", code)
	}
	out.Reset()
	errw.Reset()
	if code := a.Run(context.Background(), []string{"port", "check", "--platform", "codex", "--out", filepath.Join(root, "elsewhere")}); code != 0 {
		t.Fatalf("custom --out check exit %d: %s", code, errw.String())
	}
	if !strings.Contains(out.String(), "root marketplaces skipped (custom --out)") || !strings.Contains(out.String(), "up to date") {
		t.Errorf("stdout = %q", out)
	}
}

// TestPortShimFollowsSymlinks runs the generated shim through a symlink
// and checks LOOPS_PLUGIN_ROOT is the package root, not the link's dir.
func TestPortShimFollowsSymlinks(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	p, err := port.Load(portSource())
	if err != nil {
		t.Fatal(err)
	}
	files, _, err := port.Build(p, portSource(), plugin.Codex, port.Options{})
	if err != nil {
		t.Fatal(err)
	}
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(tmp, "pkg")
	for _, f := range files {
		if f.Path != "bin/loops" {
			continue
		}
		if err := os.MkdirAll(filepath.Join(pkg, "bin"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pkg, "bin", "loops"), f.Data, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := exec.Command("sh", "-n", filepath.Join(pkg, "bin", "loops")).CombinedOutput(); err != nil {
		t.Fatalf("sh -n: %v %s", err, out)
	}
	cache := filepath.Join(tmp, "cache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	fake := "#!/bin/sh\necho \"$LOOPS_PLUGIN_ROOT\"\n"
	if err := os.WriteFile(filepath.Join(cache, "loops-"+p.Version), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	// A relative link to an absolute-dir link: links/a -> ../links2/b, links2/b -> <pkg>/bin/loops.
	for _, d := range []string{"links", "links2"} {
		if err := os.MkdirAll(filepath.Join(tmp, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(pkg, "bin", "loops"), filepath.Join(tmp, "links2", "b")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "links2", "b"), filepath.Join(tmp, "links", "a")); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", filepath.Join(tmp, "links", "a"))
	cmd.Env = []string{"PATH=/usr/bin:/bin", "LOOPS_BIN_CACHE=" + cache, "HOME=" + tmp}
	got, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("shim: %v %s", err, got)
	}
	if strings.TrimSpace(string(got)) != pkg {
		t.Fatalf("LOOPS_PLUGIN_ROOT = %q, want %q", strings.TrimSpace(string(got)), pkg)
	}
}

func TestPortErrors(t *testing.T) {
	dir := t.TempDir()
	write := func(name, s string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	badJSON := write("bad.json", "{")
	badTier := write("tier.json", `{"codex":{"gpt":"x"}}`)
	badPlat := write("plat.json", `{"vim":{"opus":"x"}}`)
	cases := []struct {
		name string
		args []string
		code int
		want string
	}{
		{"no sub", []string{"port"}, 2, "usage"},
		{"bad sub", []string{"port", "make"}, 2, "usage"},
		{"bad flag", []string{"port", "build", "--nope"}, 2, "usage"},
		{"positional", []string{"port", "build", "extra"}, 2, "usage"},
		{"unknown platform", []string{"port", "build", "--platform", "codex,vim"}, 2, `unknown platform "vim"`},
		{"models missing", []string{"port", "build", "--models", filepath.Join(dir, "none.json")}, 2, "none.json"},
		{"models bad json", []string{"port", "build", "--models", badJSON}, 2, "bad.json"},
		{"models bad tier", []string{"port", "build", "--models", badTier}, 2, `unknown tier "gpt"`},
		{"models bad platform", []string{"port", "build", "--models", badPlat}, 2, `unknown platform "vim"`},
		{"bad src", []string{"port", "build", "--src", dir}, 1, "plugin.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, _, _, errw, _ := portApp(t, portSource())
			if code := a.Run(context.Background(), tc.args); code != tc.code || !strings.Contains(errw.String(), tc.want) {
				t.Fatalf("exit %d (want %d), stderr %q (want %q)", code, tc.code, errw, tc.want)
			}
		})
	}
}

func TestPortNoSource(t *testing.T) {
	a, _, _, errw, _ := portApp(t, nil)
	if code := a.Run(context.Background(), []string{"port", "build"}); code != 1 || !strings.Contains(errw.String(), "--src") {
		t.Fatalf("exit %d, %q", code, errw)
	}
}

func TestPortWriterErrors(t *testing.T) {
	boom := errors.New("boom")
	for name, set := range map[string]func(*memPackages){
		"write": func(m *memPackages) { m.writeErr = boom },
		"place": func(m *memPackages) { m.placeErr = boom },
	} {
		a, pk, _, errw, _ := portApp(t, portSource())
		set(pk)
		if code := a.Run(context.Background(), []string{"port", "build", "--platform", "codex"}); code != 1 || !strings.Contains(errw.String(), "boom") {
			t.Errorf("%s: exit %d, %q", name, code, errw)
		}
	}
	a, pk, _, errw, _ := portApp(t, portSource())
	pk.diffErr = boom
	if code := a.Run(context.Background(), []string{"port", "check", "--platform", "codex"}); code != 1 || !strings.Contains(errw.String(), "boom") {
		t.Errorf("diff: exit %d, %q", code, errw)
	}
}

func TestPortBuildError(t *testing.T) {
	src := portSource()
	delete(src, "skills/work/SKILL.md")
	src["skills/work/notes.md"] = &fstest.MapFile{Data: []byte("x")}
	a, _, _, errw, _ := portApp(t, src)
	if code := a.Run(context.Background(), []string{"port", "build", "--platform", "codex"}); code != 1 {
		t.Fatalf("exit %d: %s", code, errw)
	}
}

func TestParsePlatformsDefaultIsAll(t *testing.T) {
	got, err := parsePlatforms(" ")
	if err != nil || len(got) != len(plugin.Platforms) {
		t.Fatalf("default platforms = %v, %v", got, err)
	}
	if relTo("/a/b", "/c/d") != "/c/d" || relTo("/a", "/a/b/c") != filepath.Join("b", "c") {
		t.Error("rel")
	}
}

// repoRoot walks up from the test's directory to the go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

// TestPortRealTree builds the P1 platforms from the real canonical tree.
func TestPortRealTree(t *testing.T) {
	root := repoRoot(t)
	src := os.DirFS(root)
	p, err := port.Load(src)
	if err != nil {
		t.Fatalf("Load(repo): %v", err)
	}
	if p.Name != "loops" || p.Version == "" || len(p.Agents) < 16 || len(p.Hooks) < 9 {
		t.Fatalf("loaded %s %s: %d agents, %d hooks", p.Name, p.Version, len(p.Agents), len(p.Hooks))
	}
	commands := 0
	for _, s := range p.Skills {
		if s.Command {
			commands++
		}
	}
	if commands != len(port.Commands) {
		t.Errorf("command skills = %d, want %d", commands, len(port.Commands))
	}
	readOnly := map[string]bool{}
	for _, ag := range p.Agents {
		if ag.ReadOnly {
			readOnly[ag.Name] = true
		}
	}
	for _, r := range []string{"planner", "plan-reviewer", "qa-verifier", "code-researcher", "reflector"} {
		if !readOnly[r] {
			t.Errorf("%s not read-only", r)
		}
	}

	var out, errw bytes.Buffer
	pk := &memPackages{}
	a := &App{Source: src, Packages: pk, Root: t.TempDir(), Out: &out, Err: &errw}
	if code := a.Run(context.Background(), []string{"port", "build", "--platform", "claude,copilot,codex,cursor"}); code != 0 {
		t.Fatalf("exit %d: %s", code, errw.String())
	}
	byPath := func(pl string) map[string]plugin.File {
		m := map[string]plugin.File{}
		for _, f := range pk.written[filepath.Join(a.Root, "platforms", pl)] {
			m[f.Path] = f
		}
		return m
	}
	codex, cursor := byPath("codex"), byPath("cursor")
	for _, ag := range p.Agents {
		if _, ok := codex["agents/"+ag.Name+".toml"]; !ok {
			t.Errorf("codex lacks agent %s", ag.Name)
		}
		f, ok := cursor["agents/"+ag.Name+".md"]
		if !ok {
			t.Errorf("cursor lacks agent %s", ag.Name)
			continue
		}
		// Every role writes its run-dir file, so none is Cursor-readonly;
		// read-only roles get an instruction line instead.
		if !strings.Contains(string(f.Data), "readonly: false\n") {
			t.Errorf("cursor %s: want readonly: false", ag.Name)
		}
		if note := strings.Contains(string(f.Data), "never edit repo code"); note != ag.ReadOnly {
			t.Errorf("cursor %s: read-only note = %v, want %v", ag.Name, note, ag.ReadOnly)
		}
	}
	for _, s := range p.Skills {
		for _, f := range s.Files {
			if strings.HasPrefix(f, "skills/work/platforms/") && !strings.HasSuffix(f, "/codex.md") {
				if _, ok := codex[f]; ok {
					t.Errorf("codex carries %s", f)
				}
				continue
			}
			if _, ok := codex[f]; !ok {
				t.Errorf("codex lacks %s", f)
			}
		}
	}
	var hooks map[string]any
	if err := json.Unmarshal(codex["hooks/hooks.json"].Data, &hooks); err != nil {
		t.Fatalf("codex hooks.json: %v", err)
	}
	if err := json.Unmarshal(cursor["hooks/hooks.json"].Data, &hooks); err != nil {
		t.Fatalf("cursor hooks.json: %v", err)
	}
	if !codex["bin/loops"].Exec || !cursor["bin/loops"].Exec {
		t.Error("shim not executable")
	}

	// Determinism: a second build is byte-identical.
	first := pk.written
	pk2 := &memPackages{}
	a.Packages = pk2
	if code := a.Run(context.Background(), []string{"port", "build", "--platform", "claude,copilot,codex,cursor"}); code != 0 {
		t.Fatalf("second build exit %d", code)
	}
	for dir, files := range first {
		b, _ := json.Marshal(files)
		c, _ := json.Marshal(pk2.written[dir])
		if !bytes.Equal(b, c) {
			t.Errorf("%s differs between builds", dir)
		}
	}
}
