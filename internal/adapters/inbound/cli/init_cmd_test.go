package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/application/langpack"
	"github.com/arxdsilva/ai-loops/internal/platform/loopsdir"
)

func TestInitCmd(t *testing.T) {
	e := rcSetup(t, "")
	e.app.Root = t.TempDir()
	e.app.Langs = gateLangs{ds: []langpack.Detected{{ID: "go", Root: "."}}}
	if e.do("init", "extra") != 2 {
		t.Fatal("usage")
	}
	if code := e.do("init"); code != 0 {
		t.Fatal(code, e.err)
	}
	out := e.out.String()
	for _, want := range []string{"created .loops/config.json\n", "languages: go at . (detected", "commit .loops/config.json"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %q", want, out)
		}
	}
	if b, err := os.ReadFile(filepath.Join(e.app.Root, ".loops", "config.json")); err != nil || string(b) != loopsdir.InitConfig {
		t.Fatalf("%q %v", b, err)
	}
	if e.do("init") != 0 || !strings.HasPrefix(e.out.String(), "kept .loops/config.json") {
		t.Fatalf("second init: %q", e.out)
	}

	// A project from before `loops init`: its config and old ignore file.
	e.app.Root = t.TempDir()
	dir := filepath.Join(e.app.Root, ".loops")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"triage":"off"}`), 0o644)
	_ = os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*\n"), 0o644)
	e.app.Langs = gateLangs{}
	if e.do("init") != 0 || !strings.Contains(e.out.String(), "added to .loops/.gitignore: !config.json") || !strings.Contains(e.out.String(), "languages: none detected") {
		t.Fatalf("%q", e.out)
	}
	e.app.Langs = gateLangs{err: errors.New("bad pack")}
	if e.do("init") != 0 || !strings.Contains(e.out.String(), "languages: unknown (bad pack)") {
		t.Fatalf("%q", e.out)
	}
	e.app.Langs = nil
	if e.do("init") != 0 || !strings.Contains(e.out.String(), "languages: unknown") {
		t.Fatalf("%q", e.out)
	}

	// .loops is a file: it cannot write.
	e.app.Root = t.TempDir()
	_ = os.WriteFile(filepath.Join(e.app.Root, ".loops"), nil, 0o644)
	if e.do("init") != 1 {
		t.Fatal("unwritable")
	}
}

func TestRunStartRequiresInit(t *testing.T) {
	e := rcSetup(t, "")
	e.app.Root = t.TempDir()
	e.app.RequireInit = true
	if e.do("run", "start", "--title", "x") != 1 || e.ptr.id != "" || !strings.Contains(e.err.String(), "run `loops init` first") {
		t.Fatalf("started uninitialized: %q", e.err)
	}
	if e.do("init") != 0 {
		t.Fatal(e.err)
	}
	if e.do("run", "start", "--title", "x") != 0 || e.ptr.id == "" {
		t.Fatalf("initialized project refused: %q", e.err)
	}
}
