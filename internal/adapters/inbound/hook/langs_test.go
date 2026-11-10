package hook

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/application/langpack"
	"github.com/arxdsilva/ai-loops/internal/domain/pack"
	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

type fakeLangs struct {
	ds    []langpack.Detected
	err   error
	calls int
}

func (f *fakeLangs) Detect(string) ([]langpack.Detected, error) { f.calls++; return f.ds, f.err }

// whichCmd is a Commander with a per-tool PATH.
type whichCmd struct {
	fakeCmd
	path map[string]bool
}

func (c *whichCmd) Has(name string) bool { return c.path[name] }

var (
	goDet = langpack.Detected{ID: "go", Root: ".", Pack: pack.Pack{ID: "go", SourceGlobs: []string{"**/*.go"}, TestGlobs: []string{"**/*_test.go"},
		PostEdit:     &pack.PostEdit{FormatFile: []string{"gofmt", "-w", "{file}"}, LintDir: []string{"go", "vet", "./{dir}"}},
		TestCommands: []pack.TestCommand{{ArgvPrefix: []string{"go", "test"}}}, Race: pack.Race{Mode: pack.RaceFlag, Flags: []string{"-race"}}}}
	rustDet = langpack.Detected{ID: "rust", Root: "crates/core", Pack: pack.Pack{ID: "rust", Name: "Rust", Manifest: "Cargo.toml",
		SourceGlobs:  []string{"**/*.rs"},
		Placeholders: map[string]pack.Placeholder{"edition": {File: "{manifest_dir}/Cargo.toml", Regex: `edition\s*=\s*"(\d+)"`}},
		PostEdit: &pack.PostEdit{FormatFile: []string{"rustfmt", "--edition", "{edition}", "{file}"},
			LintDir: []string{"cargo", "clippy", "--manifest-path", "{manifest_dir}/Cargo.toml", "--", "-D", "warnings"}},
		Checks:       []pack.Check{{Name: "test", Argv: []string{"cargo", "test"}, Tool: "cargo"}},
		TestCommands: []pack.TestCommand{{ArgvPrefix: []string{"cargo", "test"}}}, Race: pack.Race{Mode: pack.RaceNone},
		CacheEnv: map[string]string{"CARGO_TARGET_DIR": "cache"}}}
	swiftDet = langpack.Detected{ID: "swift", Root: "app", Pack: pack.Pack{ID: "swift", Name: "Swift",
		SourceGlobs:  []string{"**/*.swift"},
		TestCommands: []pack.TestCommand{{ArgvPrefix: []string{"swift", "test"}}},
		Race:         pack.Race{Mode: pack.RaceFlag, Flags: []string{"--sanitize=thread"}},
		CacheArgv:    map[string][]string{"swift build": {"--scratch-path", "{run}/swift/build"}, "swift test": {"--scratch-path", "{run}/swift/build"}}}}
	pyDet = langpack.Detected{ID: "python", Root: ".", Pack: pack.Pack{ID: "python", SourceGlobs: []string{"**/*.py"},
		TestCommands: []pack.TestCommand{{ArgvPrefix: []string{"pytest"}}}, Race: pack.Race{Mode: pack.RaceNone}}}
)

func langSetup(cur string, ds ...langpack.Detected) (*env, *whichCmd, *fakeLangs) {
	e := setup(cur)
	c := &whichCmd{path: map[string]bool{"rustfmt": true, "cargo": true}}
	l := &fakeLangs{ds: ds}
	e.h.d.Cmd, e.h.d.Langs = c, l
	e.h.d.Files = fakeFiles{lines: map[string]int{
		"/repo/pkg/a.go": 1, "/repo/crates/core/src/lib.rs": 1, "/repo/crates/core/src/x/m.rs": 1,
		"/repo/crates/core/Cargo.toml": 1, "/repo/crates/core/src/x/Cargo.toml": 1, "/repo/README.md": 1, "/repo/tool.py": 1, "/repo/a.rs": 1,
	}}
	e.h.d.ReadFile = func(p string) ([]byte, error) {
		switch p {
		case "/repo/crates/core/Cargo.toml":
			return []byte("[package]\nedition = \"2024\"\n"), nil
		}
		return nil, errors.New("no file")
	}
	return e, c, l
}

func TestPostEditPacks(t *testing.T) {
	edit := func(f string) string { return `{"tool_name":"Edit","tool_input":{"file_path":"` + f + `"}}` }
	e, c, l := langSetup("r", goDet, rustDet, pyDet)
	if code, o, _ := e.run("post-edit", edit("/repo/crates/core/src/lib.rs")); code != 0 || o != "" {
		t.Fatalf("%d %q", code, o)
	}
	want := "[rustfmt --edition 2024 /repo/crates/core/src/lib.rs cargo clippy --manifest-path /repo/crates/core/Cargo.toml -- -D warnings]"
	if fmt.Sprint(c.calls) != want || l.calls != 1 {
		t.Fatalf("%v (detect %d)", c.calls, l.calls)
	}
	if ev := e.rec.evs[0]; ev.Name != "post-edit" || !*ev.OK || ev.Target != "/repo/crates/core/src/lib.rs" {
		t.Fatalf("%+v", ev)
	}

	// Nearest manifest wins; its edition is unreadable, so rustfmt is skipped.
	e, c, _ = langSetup("r", rustDet)
	e.run("post-edit", edit("/repo/crates/core/src/x/m.rs"))
	if fmt.Sprint(c.calls) != "[cargo clippy --manifest-path /repo/crates/core/src/x/Cargo.toml -- -D warnings]" {
		t.Fatalf("%v", c.calls)
	}

	// Go through a detected pack: the Go-only hook's commands and names.
	e, c, _ = langSetup("r", goDet, rustDet)
	e.run("go-post-edit", edit("/repo/pkg/a.go"))
	if fmt.Sprint(c.calls) != "[gofmt -w /repo/pkg/a.go go vet ./pkg]" || e.rec.evs[0].Name != "go-post-edit" {
		t.Fatalf("%v %+v", c.calls, e.rec.evs)
	}
	e, c, _ = langSetup("r", goDet)
	c.path["goimports"] = true
	e.run("post-edit", edit("/repo/pkg/a.go"))
	if c.calls[0] != "goimports -w /repo/pkg/a.go" {
		t.Fatal(c.calls)
	}

	// Failures: format failure is reported and skips the lint.
	e, c, _ = langSetup("r", rustDet)
	c.fail = map[string]string{"rustfmt": "error: expected item"}
	_, o, _ := e.run("post-edit", edit("/repo/crates/core/src/lib.rs"))
	if !strings.Contains(o, `"decision":"block"`) || !strings.Contains(o, "post-edit: rustfmt failed for /repo/crates/core/src/lib.rs") || len(c.calls) != 1 || *e.rec.evs[0].OK {
		t.Fatalf("%q %v", o, c.calls)
	}
	e, c, _ = langSetup("r", rustDet)
	c.fail = map[string]string{"cargo clippy": "warning: x"}
	_, o, _ = e.run("post-edit", edit("/repo/crates/core/src/lib.rs"))
	if !strings.Contains(o, "cargo clippy failed") {
		t.Fatalf("%q", o)
	}

	// No-ops: no owning pack, no post_edit, tool not on PATH, outside a root.
	for _, f := range []string{"/repo/README.md", "/repo/tool.py", "/repo/a.rs", "/repo/pkg/a.go"} {
		e, c, _ = langSetup("r", rustDet, pyDet)
		if f == "/repo/pkg/a.go" {
			e, c, _ = langSetup("r", rustDet) // a .go file with no Go pack detected
		}
		if _, o, _ := e.run("post-edit", edit(f)); o != "" || len(c.calls) != 0 || len(e.rec.evs) != 0 {
			t.Errorf("%s: %q %v", f, o, c.calls)
		}
	}
	e, c, _ = langSetup("r", rustDet)
	c.path = map[string]bool{}
	if e.run("post-edit", edit("/repo/crates/core/src/lib.rs")); len(c.calls) != 0 {
		t.Errorf("missing tools: %v", c.calls)
	}

	// Detection failure falls back to the Go-only hook.
	e, c, l = langSetup("r", rustDet)
	l.err = errors.New("bad pack")
	e.run("post-edit", edit("/repo/pkg/a.go"))
	if fmt.Sprint(c.calls) != "[gofmt -w /repo/pkg/a.go go vet ./pkg]" {
		t.Fatalf("fallback: %v", c.calls)
	}
}

func TestOwnerDeepestRoot(t *testing.T) {
	outer := langpack.Detected{ID: "rust", Root: ".", Pack: pack.Pack{SourceGlobs: []string{"**/*.rs"}}}
	inner := langpack.Detected{ID: "rust2", Root: "a/b", Pack: pack.Pack{SourceGlobs: []string{"**/*.rs"}}}
	d, root, rel, ok := owner([]langpack.Detected{outer, inner}, "/repo", "/repo/a/b/src/x.rs")
	if !ok || d.ID != "rust2" || root != "/repo/a/b" || rel != "src/x.rs" {
		t.Fatalf("%v %s %s %v", d.ID, root, rel, ok)
	}
	if _, _, _, ok := owner([]langpack.Detected{inner}, "/repo", "/repo/a/bc/x.rs"); ok {
		t.Fatal("sibling prefix matched")
	}
}

func TestToolLabel(t *testing.T) {
	for argv, want := range map[string]string{
		"go vet ./pkg": "go vet", "gofmt -w f": "gofmt", "cargo clippy --x": "cargo clippy", "a b c d": "a b c", "x": "x",
	} {
		if got := toolLabel(strings.Fields(argv)); got != want {
			t.Errorf("%q: %q", argv, got)
		}
	}
}

func TestTestGuardPacks(t *testing.T) {
	bash := func(c string) string { return `{"tool_name":"Bash","tool_input":{"command":"` + c + `"}}` }
	e, _, _ := langSetup("r", goDet, swiftDet, pyDet)
	c, _, er := e.run("test-guard", bash("cd app && swift test"))
	if c != 2 || !strings.Contains(er, "test_guard:") || e.rec.evs[0].Name != "test-guard" || e.rec.evs[0].Kind != run.KindGuardDeny {
		t.Fatalf("%d %q %+v", c, er, e.rec.evs)
	}
	for _, ok := range []string{"swift test --sanitize=thread", "pytest -q", "swift build"} {
		if c, _, er := e.run("test-guard", bash(ok)); c != 0 || er != "" {
			t.Errorf("%s: %d %q", ok, c, er)
		}
	}
	e, _, _ = langSetup("r", swiftDet)
	if c, _, _ := e.run("race-guard", bash("go test ./...")); c != 2 || e.rec.evs[0].Name != "race-guard" {
		t.Fatalf("go always checked, named race-guard: %d %+v", c, e.rec.evs)
	}
	e, _, l := langSetup("r", swiftDet)
	l.err = errors.New("x")
	if c, _, _ := e.run("test-guard", bash("swift test")); c != 0 {
		t.Fatal("failed detection: Go only")
	}
	if c, _, _ := e.run("test-guard", bash("")); c != 0 {
		t.Fatal("empty")
	}
}

func TestToolchainEnvPacks(t *testing.T) {
	e, rd := goEnvSetup(t, "r1")
	l := &fakeLangs{ds: []langpack.Detected{goDet, rustDet, swiftDet}}
	e.h.d.Langs = l
	for _, p := range []string{"claude", "gemini", "windsurf"} {
		code, out, er := e.runP(p, "toolchain-env", bashPayload(p, "cargo test"))
		cmd, allow, _ := rewritten(t, p, out)
		if code != 0 || er != "" || allow || cmd != "CARGO_TARGET_DIR='"+filepath.Join(rd, "rust", "cache")+"' cargo test" {
			t.Fatalf("%s: %d %q %q", p, code, out, er)
		}
	}
	if _, err := os.Stat(filepath.Join(rd, "rust", "tmp")); err != nil {
		t.Fatal("pack dirs not created")
	}
	_, out, _ := e.runP("claude", "toolchain-env", bashPayload("claude", "swift build -c release"))
	if cmd, _, _ := rewritten(t, "claude", out); cmd != "swift build '--scratch-path' '"+rd+"/swift/build' -c release" {
		t.Fatalf("%q", cmd)
	}
	for _, p := range []string{"codex", "cursor", "copilot"} { // never denied, no rewrite
		if c, o, er := e.runP(p, "toolchain-env", bashPayload(p, "cargo test")); c != 0 || o != "" || er != "" {
			t.Errorf("%s: %d %q %q", p, c, o, er)
		}
	}
	for _, cmd := range []string{"CARGO_TARGET_DIR=/x cargo test", "loops exec cargo test", "ls", "swift run", "swift test"} {
		if c, o, er := e.runP("claude", "toolchain-env", bashPayload("claude", cmd)); c != 0 || o != "" || er != "" {
			t.Errorf("%s: %d %q %q", cmd, c, o, er)
		}
	}
	// Go is unchanged and wins.
	_, out, _ = e.runP("claude", "go-env", bashPayload("claude", "go build ./..."))
	if cmd, allow, _ := rewritten(t, "claude", out); !strings.HasPrefix(cmd, "GOCACHE=") || !allow {
		t.Fatalf("%q", cmd)
	}
	// Detection failure: non-Go commands are left alone.
	l.err = errors.New("x")
	if _, o, _ := e.runP("claude", "toolchain-env", bashPayload("claude", "cargo test")); o != "" {
		t.Fatalf("%q", o)
	}
	// No active run: silent.
	e2, _ := goEnvSetup(t, "")
	e2.h.d.Langs = &fakeLangs{ds: []langpack.Detected{rustDet}}
	if _, o, _ := e2.runP("claude", "toolchain-env", bashPayload("claude", "cargo test")); o != "" {
		t.Fatalf("%q", o)
	}
}

func TestToolchainEnvMkdirFailsOpen(t *testing.T) {
	e, rd := goEnvSetup(t, "r1")
	e.h.d.Langs = &fakeLangs{ds: []langpack.Detected{rustDet}}
	if err := os.MkdirAll(rd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rd, "rust"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if c, o, er := e.runP("claude", "toolchain-env", bashPayload("claude", "cargo test")); c != 0 || o != "" || er != "" {
		t.Fatalf("%d %q %q", c, o, er)
	}
}

func TestNewNamesFailOpen(t *testing.T) {
	e, _, _ := langSetup("", rustDet)
	for _, n := range []string{"post-edit", "test-guard", "toolchain-env"} {
		if c, o, er := e.run(n, "not json"); c != 0 || o != "" || er != "" {
			t.Errorf("%s: %d %q %q", n, c, o, er)
		}
	}
}
