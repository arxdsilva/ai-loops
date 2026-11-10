package packfs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/arxdsilva/ai-loops/internal/domain/pack"
)

func TestEmbedded(t *testing.T) {
	fsys := fstest.MapFS{
		"specs/languages/go/pack.json": {Data: []byte("{}")},
		"specs/languages/go/RULES.md":  {Data: []byte("#")},
		"specs/WORKFLOW.md":            {Data: []byte("#")},
	}
	files, err := Embedded{FS: fsys}.PackFiles()
	if err != nil || len(files) != 1 || string(files["specs/languages/go/pack.json"]) != "{}" {
		t.Fatalf("%v %v", files, err)
	}
}

// TestRealPacks parses every pack in the repository and checks that each
// pack's source extensions are known to the stop gate (pack.SourceExts).
func TestRealPacks(t *testing.T) {
	files, err := Embedded{FS: os.DirFS("../../../..")}.PackFiles()
	if err != nil || len(files) == 0 {
		t.Fatalf("%d packs, %v", len(files), err)
	}
	for name, b := range files {
		p, err := pack.Parse(b)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		for _, g := range p.SourceGlobs {
			base := g[strings.LastIndex(g, "/")+1:]
			if !strings.HasPrefix(base, "*.") || strings.ContainsAny(base[1:], "*?[{") {
				continue
			}
			if !pack.LooksLikeSource("x" + base[1:]) {
				t.Errorf("%s: source glob %q has an extension missing from pack.SourceExts", name, g)
			}
		}
	}
}

func TestOSTree(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"go.mod", "svc/a/pom.xml", "svc/a/b/c/deep/pom.xml", "node_modules/x/package.json", ".git/HEAD"} {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("module x // hello"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dirs, err := OS{}.Dirs(root, 3, func(n string) bool { return n == "node_modules" || strings.HasPrefix(n, ".") })
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(dirs, ","); got != ".,svc,svc/a,svc/a/b" {
		t.Fatalf("dirs %s", got)
	}
	if _, err := (OS{}).Dirs(filepath.Join(root, "missing"), 3, func(string) bool { return false }); err == nil {
		t.Fatal("missing root accepted")
	}
	v := OS{}.View(root, ".")
	if !v.Exists("go.mod") || !v.Exists("svc") || v.Exists("nope") {
		t.Fatal("Exists")
	}
	if s, ok := v.Read("go.mod"); !ok || !strings.Contains(s, "hello") {
		t.Fatal("Read")
	}
	if _, ok := v.Read("nope"); ok {
		t.Fatal("Read missing")
	}
	if got := (OS{}).View(root, "svc").List("a"); len(got) != 2 {
		t.Fatalf("List %v", got)
	}
	if (OS{}).View(root, ".").List("nope") != nil {
		t.Fatal("List missing")
	}
	if !pack.Matches([]pack.Marker{{File: "go.mod", Contains: "module"}}, v) {
		t.Fatal("marker via OS view")
	}
}
