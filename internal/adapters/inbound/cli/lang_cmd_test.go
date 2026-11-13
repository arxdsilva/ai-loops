package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/packfs"
	"github.com/arxdsilva/ai-loops/internal/application/langpack"
	"github.com/arxdsilva/ai-loops/internal/domain/pack"
)

func langSetup(t *testing.T) (*rcEnv, string) {
	t.Helper()
	e := rcSetup(t, "")
	e.app.Langs = langpack.New(packfs.Embedded{FS: os.DirFS("../../../..")}, packfs.OS{})
	proj := t.TempDir()
	if err := os.WriteFile(filepath.Join(proj, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.app.Root = proj
	return e, proj
}

func TestLangListShowValidate(t *testing.T) {
	e, _ := langSetup(t)
	if code := e.do("lang", "list"); code != 0 || !strings.Contains(e.out.String(), "go ") || !strings.Contains(e.out.String(), "gocover") {
		t.Fatal(code, e.out.String(), e.err.String())
	}
	if code := e.do("lang", "list", "--json"); code != 0 || !strings.Contains(e.out.String(), `"id": "go"`) {
		t.Fatal(code, e.out.String())
	}
	if code := e.do("lang", "show", "go"); code != 0 || !strings.Contains(e.out.String(), `"race"`) {
		t.Fatal(code, e.out.String())
	}
	if code := e.do("lang", "show", "cobol"); code == 0 {
		t.Fatal("unknown pack shown")
	}
	if code := e.do("lang", "show", "go", "--variant", "nope"); code == 0 {
		t.Fatal("unknown variant shown")
	}
	if code := e.do("lang", "show"); code != 2 {
		t.Fatal(code)
	}
	if code := e.do("lang", "validate"); code != 0 || !strings.Contains(e.out.String(), "ok:") {
		t.Fatal(code, e.err.String())
	}
	bad := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(bad, []byte(`{"schema":9}`), 0o644)
	if code := e.do("lang", "validate", "../../../../specs/languages/go/pack.json", bad, "/missing.json"); code != 1 ||
		!strings.Contains(e.out.String(), "ok: ../../../../specs/languages/go/pack.json") || strings.Count(e.err.String(), "\n") < 2 {
		t.Fatal(code, e.out.String(), e.err.String())
	}
	for _, args := range [][]string{{"lang"}, {"lang", "frob"}, {"lang", "list", "--bogus"}, {"lang", "show", "--bogus"}, {"lang", "detect", "--bogus"}} {
		if code := e.do(args...); code != 2 {
			t.Errorf("%v: code %d", args, code)
		}
	}
	e.app.Langs = nil
	if code := e.do("lang", "list"); code == 0 {
		t.Fatal("nil Langs")
	}
}

func TestLangDetect(t *testing.T) {
	e, proj := langSetup(t)
	if code := e.do("lang", "detect"); code != 0 || !strings.Contains(e.out.String(), "go at .: race flag, coverage gocover") {
		t.Fatal(code, e.out.String(), e.err.String())
	}
	if code := e.do("run", "start", "--title", "t"); code != 0 {
		t.Fatal(e.err.String())
	}
	if code := e.do("lang", "detect", "--json", "--write"); code != 0 {
		t.Fatal(code, e.err.String())
	}
	var ds []langpack.Detected
	if err := json.Unmarshal(e.out.Bytes(), &ds); err != nil || len(ds) != 1 || ds[0].ID != "go" {
		t.Fatalf("%s %v", e.out.String(), err)
	}
	run, _ := e.app.Tracker.Dir(t.Context(), "")
	if b, err := os.ReadFile(filepath.Join(run, "research", "language.json")); err != nil || !strings.Contains(string(b), `"root": "."`) {
		t.Fatalf("%s %v", b, err)
	}
	empty := t.TempDir()
	if code := e.do("lang", "detect", "--root", empty); code != 0 || !strings.Contains(e.out.String(), "no language pack detected") {
		t.Fatal(e.out.String())
	}
	if code := e.do("lang", "detect", "--root", empty, "--json"); code != 0 || strings.TrimSpace(e.out.String()) != "[]" {
		t.Fatal(e.out.String())
	}
	e.app.Languages = []langpack.Override{{ID: "cobol"}}
	if code := e.do("lang", "detect", "--root", proj); code == 0 {
		t.Fatal("bad override accepted")
	}
}

func TestLangValidateRequiresSecurity(t *testing.T) {
	e, _ := langSetup(t)
	if code := e.do("lang", "validate"); code != 0 || e.out.String() != "ok: 15 packs\n" {
		t.Fatal(code, e.out.String(), e.err.String())
	}
	good, err := os.ReadFile("../../../../specs/languages/go/pack.json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(good, &m); err != nil {
		t.Fatal(err)
	}
	delete(m, "security")
	b, _ := json.Marshal(m)
	noSec := filepath.Join(t.TempDir(), "nosec.json")
	if err := os.WriteFile(noSec, b, 0o644); err != nil {
		t.Fatal(err)
	}
	if code := e.do("lang", "validate", noSec); code != 1 || !strings.Contains(e.err.String(), "security: not declared") {
		t.Fatal(code, e.out.String(), e.err.String())
	}
}

type noSecLangs struct{ gateLangs }

func (noSecLangs) All() ([]pack.Pack, error) {
	return []pack.Pack{{ID: "a", Security: &pack.Security{None: "x"}}, {ID: "b"}}, nil
}

func TestLangValidateAllReportsUndeclared(t *testing.T) {
	e, _ := langSetup(t)
	e.app.Langs = noSecLangs{}
	if code := e.do("lang", "validate"); code != 1 || !strings.Contains(e.err.String(), "pack b: security: not declared") || strings.Contains(e.out.String(), "ok:") {
		t.Fatal(code, e.out.String(), e.err.String())
	}
}
