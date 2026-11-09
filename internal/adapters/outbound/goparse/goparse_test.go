package goparse

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/insurgencygames/ai-loops/internal/domain/archrule"
)

func parseFixture(t *testing.T, name string) archrule.Module {
	t.Helper()
	m, err := New().Parse(context.Background(), filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("Parse(%s): %v", name, err)
	}
	return m
}

func findPkg(t *testing.T, m archrule.Module, dir string) archrule.Package {
	t.Helper()
	for _, p := range m.Packages {
		if p.Dir == dir {
			return p
		}
	}
	t.Fatalf("package %s not parsed; have %v", dir, pkgDirs(m))
	return archrule.Package{}
}

func pkgDirs(m archrule.Module) []string {
	var out []string
	for _, p := range m.Packages {
		out = append(out, p.Dir)
	}
	return out
}

func findDir(t *testing.T, m archrule.Module, path string) archrule.Dir {
	t.Helper()
	for _, d := range m.Dirs {
		if d.Path == path {
			return d
		}
	}
	t.Fatalf("dir %s not walked", path)
	return archrule.Dir{}
}

func pos(file string, line int) archrule.Pos { return archrule.Pos{File: file, Line: line} }

func TestParsePackagesAndPaths(t *testing.T) {
	m := parseFixture(t, "clean")
	if m.Path != "example.com/clean" {
		t.Fatalf("module path = %q", m.Path)
	}
	want := []string{"cmd/clean", "internal/adapters/outbound/db", "internal/application/orders", "internal/compose/app", "internal/domain/order"}
	if got := pkgDirs(m); !reflect.DeepEqual(got, want) {
		t.Fatalf("packages = %v, want %v", got, want)
	}
	p := findPkg(t, m, "internal/application/orders")
	if p.ImportPath != "example.com/clean/internal/application/orders" {
		t.Errorf("import path = %q", p.ImportPath)
	}
	if !reflect.DeepEqual(p.Files, []string{"internal/application/orders/orders.go"}) || p.Lines != 15 {
		t.Errorf("files = %v, lines = %d, want orders.go and 15", p.Files, p.Lines)
	}
	if d := findDir(t, m, "cmd/clean"); !reflect.DeepEqual(d.Entries, []string{"main.go"}) {
		t.Errorf("cmd/clean entries = %v", d.Entries)
	}
	if d := findDir(t, m, "."); !reflect.DeepEqual(d.Entries, []string{"cmd/", "go.mod", "internal/"}) {
		t.Errorf("root entries = %v", d.Entries)
	}
}

func TestParseImports(t *testing.T) {
	m := parseFixture(t, "imports")
	p := findPkg(t, m, "internal/domain/order")
	f := "internal/domain/order/order.go"
	want := []archrule.Import{
		{Path: "net/http", Pos: pos(f, 4)},
		{Path: "os", Pos: pos(f, 5)},
		{Path: "github.com/x/y", Pos: pos(f, 7)},
		{Path: "example.com/imports/internal/adapters/outbound/db", Pos: pos(f, 9)},
		{Path: "example.com/imports/internal/compose/app", Pos: pos(f, 10)},
	}
	if !reflect.DeepEqual(p.Imports, want) {
		t.Fatalf("imports = %+v\nwant %+v", p.Imports, want)
	}
}

func TestParseInits(t *testing.T) {
	p := findPkg(t, parseFixture(t, "init"), "internal/platform/reg")
	want := []archrule.Pos{pos("internal/platform/reg/reg.go", 8)}
	if !reflect.DeepEqual(p.Inits, want) {
		t.Fatalf("inits = %+v, want %+v (no methods named init, no test files)", p.Inits, want)
	}
	if !reflect.DeepEqual(p.TestFiles, []string{"internal/platform/reg/reg_test.go"}) {
		t.Errorf("test files = %v", p.TestFiles)
	}
}

func TestParseGlobals(t *testing.T) {
	p := findPkg(t, parseFixture(t, "global"), "internal/platform/state")
	f := "internal/platform/state/state.go"
	want := []archrule.Global{
		{Name: "counter", Decl: pos(f, 8), Assigned: pos("internal/platform/state/state_test.go", 6)},
		{Name: "assigned", Decl: pos(f, 10), Assigned: pos(f, 36)},
		{Name: "ptr", Decl: pos(f, 25), Assigned: pos(f, 62)},
	}
	if !reflect.DeepEqual(p.Globals, want) {
		t.Fatalf("globals = %+v\nwant %+v", p.Globals, want)
	}
}

func TestParseCallsAndGoStmts(t *testing.T) {
	p := findPkg(t, parseFixture(t, "purity"), "internal/domain/sim")
	var got []string
	for _, c := range p.Calls {
		got = append(got, fmt.Sprintf("%s.%s@%s:%d", c.Pkg, c.Name, c.Pos.File, c.Pos.Line))
	}
	sort.Strings(got)
	f := "internal/domain/sim/sim.go"
	want := []string{
		"math/rand.Intn@" + f + ":19",
		"math/rand.Intn@" + f + ":22",
		"math/rand.New@" + f + ":16",
		"math/rand.NewSource@" + f + ":16",
		"math/rand.Rand@" + f + ":27",
		"math/rand/v2.Float64@" + f + ":26",
		"math/rand/v2.N@" + f + ":19",
		"math/rand/v2.New@" + f + ":18",
		"math/rand/v2.NewPCG@" + f + ":18",
		"time.Now@" + f + ":11",
		"time.Now@" + f + ":12",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %v\nwant %v", got, want)
	}
	if !reflect.DeepEqual(p.GoStmts, []archrule.Pos{pos(f, 15)}) {
		t.Fatalf("go statements = %+v (test files are not scanned)", p.GoStmts)
	}
}

func TestParseTypes(t *testing.T) {
	p := findPkg(t, parseFixture(t, "limits"), "internal/adapters/outbound/big")
	byName := map[string]archrule.Type{}
	for _, ty := range p.Types {
		byName[ty.Name] = ty
	}
	f := "internal/adapters/outbound/big/big.go"
	want := map[string]archrule.Type{
		"Big":   {Name: "Big", Pos: pos(f, 4), Kind: archrule.KindStruct, Methods: 41, Fields: 26},
		"Small": {Name: "Small", Pos: pos(f, 116), Kind: archrule.KindInterface, IfaceMethods: 1},
		"Fat":   {Name: "Fat", Pos: pos(f, 121), Kind: archrule.KindInterface, IfaceMethods: 9},
	}
	if !reflect.DeepEqual(byName, want) {
		t.Fatalf("types = %+v\nwant %+v", byName, want)
	}
}

func TestParseTypeDeclaredAfterItsMethods(t *testing.T) {
	root := writeModule(t, map[string]string{
		"a/a.go": "package a\n\nfunc (t *T[K]) M() {}\n\nfunc (T[K]) N() {}\n",
		"a/b.go": "package a\n\ntype T[K any] struct{ a, b int; K }\n\ntype Alias = int\n\nfunc (Weird) X() {}\n",
	})
	m, err := New().Parse(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]archrule.Type{}
	for _, ty := range findPkg(t, m, "a").Types {
		byName[ty.Name] = ty
	}
	if got := byName["T"]; got.Methods != 2 || got.Fields != 3 || got.Kind != archrule.KindStruct || got.Pos != pos("a/b.go", 3) {
		t.Errorf("T = %+v: generic receivers count, the declaration wins the position", got)
	}
	if got := byName["Alias"]; got.Kind != archrule.KindOther {
		t.Errorf("Alias = %+v", got)
	}
	if got := byName["Weird"]; got.Kind != "" || got.Methods != 1 || got.Pos != pos("a/b.go", 7) {
		t.Errorf("an undeclared receiver type is placed at its first method: %+v", got)
	}
}

func TestParseMainFile(t *testing.T) {
	m := parseFixture(t, "cmd")
	mf := findPkg(t, m, "cmd/bad").Main
	if mf == nil {
		t.Fatal("cmd/bad/main.go facts missing")
	}
	f := "cmd/bad/main.go"
	want := archrule.MainFile{
		File:  f,
		Lines: 20,
		Decls: []archrule.Fact{{What: "var declaration", Pos: pos(f, 5)}, {What: "func main", Pos: pos(f, 7)}, {What: "func help", Pos: pos(f, 18)}},
		Logic: []archrule.Fact{
			{What: "for/range statement", Pos: pos(f, 8)},
			{What: "switch statement", Pos: pos(f, 11)},
			{What: "go statement", Pos: pos(f, 14)},
			{What: "function literal", Pos: pos(f, 14)},
		},
		Strings: []archrule.StringLit{{Runes: 52, Pos: pos(f, 19)}},
	}
	if !reflect.DeepEqual(*mf, want) {
		t.Fatalf("main facts = %+v\nwant %+v", *mf, want)
	}
	if d := findDir(t, m, "cmd/bad"); !reflect.DeepEqual(d.Entries, []string{"main.go", "notes.txt"}) {
		t.Errorf("cmd/bad entries = %v", d.Entries)
	}
	if d := findDir(t, m, "cmd/sub"); !reflect.DeepEqual(d.Entries, []string{"main.go", "render/"}) {
		t.Errorf("cmd/sub entries = %v", d.Entries)
	}
	if findPkg(t, m, "cmd/nomain").Main != nil {
		t.Error("a directory without main.go has no main facts")
	}
	ok := findPkg(t, m, "cmd/ok").Main
	if ok == nil || len(ok.Strings) != 0 || len(ok.Logic) != 0 {
		t.Errorf("import paths are not string literals: %+v", ok)
	}
	var mainMethod = writeModule(t, map[string]string{"cmd/x/main.go": "package main\n\ntype T struct{}\n\nfunc (T) main() {}\n\nfunc main() {}\n"})
	mm, err := New().Parse(context.Background(), mainMethod)
	if err != nil {
		t.Fatal(err)
	}
	decls := findPkg(t, mm, "cmd/x").Main.Decls
	if len(decls) != 3 || decls[0].What != "type declaration" || decls[1].What != "method main" {
		t.Errorf("a method named main is not func main: %+v", decls)
	}
}

func TestParseSkips(t *testing.T) {
	m := parseFixture(t, "skips")
	if got := pkgDirs(m); !reflect.DeepEqual(got, []string{".", "sub"}) {
		t.Fatalf("packages = %v: testdata, vendor, hidden, _ and nested-module dirs are skipped", got)
	}
	root := findPkg(t, m, ".")
	if root.ImportPath != "example.com/skips" || !reflect.DeepEqual(root.Files, []string{"skips.go"}) {
		t.Errorf("root package = %+v: files starting with . or _ are ignored", root)
	}
	for _, p := range m.Packages {
		if len(p.Inits) != 0 {
			t.Errorf("%s: a skipped file was parsed: %+v", p.Dir, p.Inits)
		}
	}
	var dirs []string
	for _, d := range m.Dirs {
		dirs = append(dirs, d.Path)
	}
	if !reflect.DeepEqual(dirs, []string{".", "sub"}) {
		t.Errorf("dirs = %v", dirs)
	}
	want := []string{"_ignored.go", "_under/", "go.mod", "nested/", "skips.go", "sub/", "testdata/", "vendor/"}
	if got := findDir(t, m, ".").Entries; !reflect.DeepEqual(got, want) {
		t.Errorf("root entries = %v, want %v (hidden entries dropped)", got, want)
	}
}

// writeModule writes files into a fresh module "example.com/tmp".
func writeModule(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if _, ok := files["go.mod"]; !ok {
		files["go.mod"] = "module example.com/tmp\n\ngo 1.24\n"
	}
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestParseCountsLinesAndFiles(t *testing.T) {
	files := map[string]string{}
	body := "package big\n" + strings.Repeat("\n", 199)
	for i := range 61 {
		files[fmt.Sprintf("big/f%02d.go", i)] = body
	}
	files["big/f_test.go"] = "package big\n" + strings.Repeat("\n", 5000)
	m, err := New().Parse(context.Background(), writeModule(t, files))
	if err != nil {
		t.Fatal(err)
	}
	p := findPkg(t, m, "big")
	if len(p.Files) != 61 || p.Lines != 61*200 || len(p.TestFiles) != 1 {
		t.Fatalf("files = %d, lines = %d, tests = %d; want 61, %d, 1", len(p.Files), p.Lines, len(p.TestFiles), 61*200)
	}
}

func TestParseModulePathForms(t *testing.T) {
	for _, gomod := range []string{
		"module example.com/q\n",
		"// comment\nmodule \"example.com/q\" // trailing\n\ngo 1.24\n",
		"module\texample.com/q\n",
	} {
		m, err := New().Parse(context.Background(), writeModule(t, map[string]string{"go.mod": gomod}))
		if err != nil {
			t.Fatalf("%q: %v", gomod, err)
		}
		if m.Path != "example.com/q" {
			t.Errorf("%q: module path = %q", gomod, m.Path)
		}
	}
}

func TestParseErrors(t *testing.T) {
	ctx := context.Background()
	if _, err := New().Parse(ctx, t.TempDir()); err == nil || !strings.Contains(err.Error(), "go.mod") {
		t.Errorf("no go.mod: err = %v", err)
	}
	if _, err := New().Parse(ctx, writeModule(t, map[string]string{"go.mod": "go 1.24\n"})); err == nil || !strings.Contains(err.Error(), "module") {
		t.Errorf("go.mod without module line: err = %v", err)
	}
	bad := writeModule(t, map[string]string{"a/a.go": "package a\n\nfunc {\n"})
	if _, err := New().Parse(ctx, bad); err == nil || !strings.Contains(err.Error(), "a/a.go") {
		t.Errorf("syntax error: err = %v, want it to name the file", err)
	}
	badTest := writeModule(t, map[string]string{"a/a.go": "package a\n", "a/a_test.go": "package a\n\nfunc {\n"})
	if _, err := New().Parse(ctx, badTest); err == nil || !strings.Contains(err.Error(), "a/a_test.go") {
		t.Errorf("syntax error in a test file: err = %v", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := New().Parse(cancelled, filepath.Join("testdata", "clean")); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled context: err = %v", err)
	}
}
