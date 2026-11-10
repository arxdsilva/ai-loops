package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/goparse"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/importscan"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/packfs"
	"github.com/arxdsilva/ai-loops/internal/application/archcheck"
	"github.com/arxdsilva/ai-loops/internal/application/langpack"
	"github.com/arxdsilva/ai-loops/internal/domain/archrule"
	"github.com/arxdsilva/ai-loops/internal/domain/pack"
)

type archFake struct {
	rep    archrule.Report
	err    error
	root   string
	layers archrule.Layers
}

func (f *archFake) Check(_ context.Context, root string, l archrule.Layers) (archrule.Report, error) {
	f.root, f.layers = root, l
	return f.rep, f.err
}

func archApp(a Archer) (*App, *bytes.Buffer, *bytes.Buffer) {
	var out, errw bytes.Buffer
	return &App{Arch: a, Root: "/repo", Layers: archrule.Layers{Domain: []string{"core"}}, Out: &out, Err: &errw}, &out, &errw
}

var archMixed = archrule.Report{Module: "example.com/m", Findings: []archrule.Finding{
	{File: "a/a.go", Line: 3, Rule: "methods", Severity: archrule.SeverityWarn, Msg: "methods: too many"},
	{File: "b/b.go", Line: 7, Rule: "init", Severity: archrule.SeverityError, Msg: "init: no init"},
	{File: "c", Line: 0, Rule: "lines", Severity: archrule.SeverityWarn, Msg: "lines: too long"},
	{File: "d/d.go", Line: 1, Rule: "imports", Severity: archrule.SeverityError, Msg: "imports: bad edge"},
}}

func TestArchTextErrorsFirst(t *testing.T) {
	f := &archFake{rep: archMixed}
	a, out, _ := archApp(f)
	if code := a.Run(context.Background(), []string{"arch"}); code != 1 {
		t.Fatalf("exit = %d, want 1 on errors", code)
	}
	want := strings.Join([]string{
		"b/b.go:7 init error init: no init",
		"d/d.go:1 imports error imports: bad edge",
		"a/a.go:3 methods warn methods: too many",
		"c:0 lines warn lines: too long",
		"arch: 2 errors, 2 warnings",
	}, "\n") + "\n"
	if out.String() != want {
		t.Fatalf("output:\n%s\nwant:\n%s", out.String(), want)
	}
	if f.root != "/repo" || !reflect.DeepEqual(f.layers, archrule.Layers{Domain: []string{"core"}}) {
		t.Errorf("checked %q with %+v, want the app root and layers", f.root, f.layers)
	}
}

func TestArchWarningsOnlyPass(t *testing.T) {
	a, out, _ := archApp(&archFake{rep: archrule.Report{Findings: archMixed.Findings[:1]}})
	if code := a.Run(context.Background(), []string{"arch"}); code != 0 {
		t.Fatalf("exit = %d, want 0: warnings never fail", code)
	}
	if !strings.HasSuffix(out.String(), "arch: 0 errors, 1 warnings\n") {
		t.Errorf("output = %q", out.String())
	}
}

func TestArchJSON(t *testing.T) {
	a, out, _ := archApp(&archFake{rep: archMixed})
	if code := a.Run(context.Background(), []string{"arch", "--json"}); code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	var got archrule.Report
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out.String())
	}
	if !reflect.DeepEqual(got, archMixed) {
		t.Fatalf("json report = %+v, want %+v", got, archMixed)
	}
	a, out, _ = archApp(&archFake{rep: archrule.Report{Module: "m", Findings: []archrule.Finding{}}})
	if code := a.Run(context.Background(), []string{"arch", "-json"}); code != 0 || !strings.Contains(out.String(), `"findings": []`) {
		t.Fatalf("exit = %d, output = %s", code, out.String())
	}
}

func TestArchRootFlag(t *testing.T) {
	f := &archFake{}
	a, _, _ := archApp(f)
	if code := a.Run(context.Background(), []string{"arch", "--root", "/elsewhere"}); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if f.root != "/elsewhere" {
		t.Errorf("root = %q, want /elsewhere", f.root)
	}
	a.Root = ""
	if a.Run(context.Background(), []string{"arch"}); f.root != "." {
		t.Errorf("root = %q, want . when the app has none", f.root)
	}
}

func TestArchUsageErrors(t *testing.T) {
	for _, args := range [][]string{{"arch", "--nope"}, {"arch", "extra"}} {
		f := &archFake{}
		a, out, errw := archApp(f)
		if code := a.Run(context.Background(), args); code != 2 {
			t.Errorf("%v: exit = %d, want 2", args, code)
		}
		if out.Len() != 0 || !strings.Contains(errw.String(), "usage: loops arch") || !strings.Contains(errw.String(), ".loops/config.json") || f.root != "" {
			t.Errorf("%v: out = %q, err = %q, checked = %q", args, out.String(), errw.String(), f.root)
		}
	}
}

func TestArchCheckFailure(t *testing.T) {
	a, out, errw := archApp(&archFake{err: errors.New("boom")})
	if code := a.Run(context.Background(), []string{"arch"}); code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if out.Len() != 0 || !strings.Contains(errw.String(), "loops arch: boom") {
		t.Errorf("out = %q, err = %q", out.String(), errw.String())
	}
}

// archRepoRoot walks up from the test directory to the go.mod.
func archRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		up := filepath.Dir(dir)
		if up == dir {
			t.Fatal("go.mod not found above the test directory")
		}
		dir = up
	}
}

// archPackFake is an Archer that also checks packs.
type archPackFake struct {
	archFake
	res   archcheck.Result
	ds    []langpack.Detected
	calls int
}

func (f *archPackFake) CheckPacks(_ context.Context, root string, l archrule.Layers, ds []langpack.Detected) (archcheck.Result, error) {
	f.root, f.layers, f.ds = root, l, ds
	f.calls++
	return f.res, f.err
}

// archLangs detects a fixed list.
type archLangs struct {
	ds  []langpack.Detected
	err error
	got []langpack.Override
}

func (f *archLangs) All() ([]pack.Pack, error)     { return nil, nil }
func (f *archLangs) Get(string) (pack.Pack, error) { return pack.Pack{}, nil }
func (f *archLangs) Detect(_ string, o []langpack.Override) ([]langpack.Detected, error) {
	f.got = o
	return f.ds, f.err
}

var twoPacks = []langpack.Detected{{ID: "go", Root: "."}, {ID: "python", Root: "tools"}}

func TestArchChecksEveryDetectedPack(t *testing.T) {
	f := &archPackFake{res: archcheck.Result{
		Report: archrule.Report{Findings: archMixed.Findings[:1]},
		Notes:  []string{"arch: python delegates to its native check import-linter (run by loops gate)"},
	}}
	a, out, _ := archApp(f)
	langs := &archLangs{ds: twoPacks}
	a.Langs, a.Languages = langs, []langpack.Override{{ID: "go"}}
	if code := a.Run(context.Background(), []string{"arch"}); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	want := "arch: python delegates to its native check import-linter (run by loops gate)\n" +
		"a/a.go:3 methods warn methods: too many\narch: 0 errors, 1 warnings\n"
	if out.String() != want {
		t.Fatalf("output:\n%s\nwant:\n%s", out.String(), want)
	}
	if f.calls != 1 || f.root != "/repo" || !reflect.DeepEqual(f.ds, twoPacks) || len(langs.got) != 1 {
		t.Errorf("CheckPacks(%q, %+v) x%d, overrides %+v", f.root, f.ds, f.calls, langs.got)
	}
	out.Reset()
	if code := a.Run(context.Background(), []string{"arch", "--json"}); code != 0 || !strings.Contains(out.String(), `"notes": [`) || !strings.Contains(out.String(), `"findings": [`) {
		t.Fatalf("json: exit %d\n%s", code, out.String())
	}
	f.err = errors.New("boom")
	if code := a.Run(context.Background(), []string{"arch"}); code != 1 {
		t.Errorf("a CheckPacks error exits 1, got %d", code)
	}
}

func TestArchFallsBackToGo(t *testing.T) {
	for name, langs := range map[string]*archLangs{
		"none detected": {},
		"detect error":  {err: errors.New("bad pack"), ds: twoPacks},
	} {
		f := &archPackFake{archFake: archFake{rep: archrule.Report{Findings: []archrule.Finding{}}}}
		a, out, errw := archApp(f)
		a.Langs = langs
		if code := a.Run(context.Background(), []string{"arch"}); code != 0 || f.calls != 0 || f.root != "/repo" {
			t.Errorf("%s: exit %d, CheckPacks x%d, Check root %q", name, code, f.calls, f.root)
		}
		if langs.err != nil && !strings.Contains(errw.String(), "bad pack") {
			t.Errorf("%s: the detection error is reported: %q", name, errw.String())
		}
		if !strings.HasSuffix(out.String(), "arch: 0 errors, 0 warnings\n") {
			t.Errorf("%s: output %q", name, out.String())
		}
	}
}

// `loops arch` on this repository, with the embedded packs detected, has
// no errors.
func TestArchPacksOnThisRepo(t *testing.T) {
	root := archRepoRoot(t)
	a, out, errw := archApp(archcheck.New(goparse.New()).WithScanner(importscan.New()))
	a.Root, a.Layers = root, archrule.Layers{}
	a.Langs = langpack.New(packfs.Embedded{FS: os.DirFS(root)}, packfs.OS{})
	if code := a.Run(context.Background(), []string{"arch"}); code != 0 {
		t.Fatalf("loops arch on this repo: exit %d\n%s%s", code, out.String(), errw.String())
	}
	if !strings.Contains(out.String(), "arch: 0 errors,") {
		t.Fatalf("output = %q", out.String())
	}
}

// `loops arch` on this repository has no errors.
func TestArchOnThisRepo(t *testing.T) {
	a, out, errw := archApp(archcheck.New(goparse.New()))
	a.Root, a.Layers = archRepoRoot(t), archrule.Layers{}
	if code := a.Run(context.Background(), []string{"arch"}); code != 0 {
		t.Fatalf("loops arch on this repo: exit %d\n%s%s", code, out.String(), errw.String())
	}
	if !strings.Contains(out.String(), "arch: 0 errors,") {
		t.Fatalf("output = %q", out.String())
	}
}
