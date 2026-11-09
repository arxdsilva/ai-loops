package goparse

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"
)

// globalNames runs assignedGlobals over one package file and optional
// test files, all sources without the package clause unless they carry one.
func globalNames(t *testing.T, src string, tests ...string) []string {
	t.Helper()
	fset := token.NewFileSet()
	parse := func(name, body string) *ast.File {
		af, err := parser.ParseFile(fset, name, body, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("%s: %v\n%s", name, err, body)
		}
		return af
	}
	files := []*ast.File{parse("ext/ext.go", "package ext\n\n"+src)}
	var tfs []*ast.File
	for _, ts := range tests {
		tfs = append(tfs, parse("ext/ext_test.go", ts))
	}
	var names []string
	for _, g := range assignedGlobals(fset, "example.com/m/ext", files, tfs) {
		names = append(names, g.Name)
	}
	return names
}

func TestAssignedGlobalsScopes(t *testing.T) {
	cases := []struct {
		name, src string
		want      []string
	}{
		{"a block-local := only shadows inside its block",
			"var h int\nfunc F() {\n\tif true {\n\t\th := 1\n\t\t_ = h\n\t}\n\th = 5\n}\n", []string{"h"}},
		{"a func literal's var only shadows inside the literal",
			"var fnScope int\nfunc F() {\n\tfunc() {\n\t\tvar fnScope int\n\t\tfnScope = 1\n\t\t_ = fnScope\n\t}()\n\tfnScope = 9\n}\n", []string{"fnScope"}},
		{"a parameter shadows the whole body",
			"var p int\nfunc F(p int) {\n\t{\n\t\tp = 1\n\t}\n\tp = 2\n}\n", nil},
		{"a result and a receiver shadow the whole body",
			"var r, s int\ntype T struct{}\nfunc (s T) F() (r int) {\n\tr = 1\n\t_ = s\n\treturn\n}\n", nil},
		{"inside the shadowing block the global is safe",
			"var x int\nfunc F() {\n\t{\n\t\tx := 0\n\t\tx = 1\n\t\t_ = x\n\t}\n}\n", nil},
		{"a write before the local := is a global write",
			"var late int\nfunc F() {\n\tlate = 1\n\tlate := 2\n\t_ = late\n}\n", []string{"late"}},
		{"a write before a local var is a global write",
			"var d int\nfunc F() {\n\td = 1\n\tvar d int\n\t_ = d\n}\n", []string{"d"}},
		{"range := shadows inside the loop",
			"var k int\nfunc F() {\n\tfor k := range 3 {\n\t\tk = 1\n\t\t_ = k\n\t}\n}\n", nil},
		{"range = assigns the global",
			"var k2 int\nfunc F() {\n\tfor k2 = range 3 {\n\t}\n}\n", []string{"k2"}},
		{"an if-init := shadows in the if and else only",
			"var e error\nfunc g() error { return nil }\nfunc F() {\n\tif e := g(); e != nil {\n\t\te = nil\n\t} else {\n\t\te = nil\n\t}\n\te = nil\n}\n", []string{"e"}},
		{"a for-init := shadows in the loop only",
			"var i int\nfunc F() {\n\tfor i := 0; i < 3; i++ {\n\t}\n}\n", nil},
		{"a type switch binding shadows in its clauses only",
			"var v int\nfunc F(x any) {\n\tswitch v := x.(type) {\n\tcase int:\n\t\tv = 1\n\t\t_ = v\n\t}\n\tv = 2\n}\n", []string{"v"}},
		{"a switch-init := and a select comm := shadow locally",
			"var sw, msg int\nfunc F(c chan int) {\n\tswitch sw := 1; sw {\n\tcase 1:\n\t\tsw = 2\n\t}\n\tselect {\n\tcase msg := <-c:\n\t\tmsg = 1\n\t\t_ = msg\n\t}\n}\n", nil},
		{"func literal parameters shadow only the literal",
			"var a int\nfunc F() {\n\tf := func(a int) { a = 1 }\n\t_ = f\n\ta = 3\n}\n", []string{"a"}},
		{"writes inside defer and go literals count",
			"var g1, g2 int\nfunc F() {\n\tdefer func() { g1 = 1 }()\n\tgo func() { g2++ }()\n}\n", []string{"g1", "g2"}},
		{"labeled loops and compound assignment",
			"var c int\nfunc F() {\nL:\n\tfor {\n\t\tc += 1\n\t\tbreak L\n\t}\n}\n", []string{"c"}},
		{"a local type name shadows",
			"var tt int\nfunc F() {\n\ttype tt struct{ n int }\n\t_ = tt{}\n}\n", nil},
		{"rimworld exemptions still hold",
			"import \"sync\"\nvar (\n\tmu sync.Mutex\n\tcache map[string]int\n)\nvar _ = 1\nfunc F() {\n\tcache = nil\n\t_ = 2\n}\n", nil},
	}
	for _, c := range cases {
		if got := globalNames(t, c.src); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestAssignedGlobalsExternalTestPackage(t *testing.T) {
	src := "var priv int\nvar Exported, Other int\n"
	if got := globalNames(t, src, "package ext_test\n\nvar priv int\n\nfunc T() { priv = 1 }\n"); got != nil {
		t.Errorf("an external test package's own var is not the package's global: got %v", got)
	}
	ext := "package ext_test\n\nimport e \"example.com/m/ext\"\n\nfunc T() {\n\te.Exported = 1\n\tvar e struct{ Other int }\n\te.Other = 2\n}\n"
	if got := globalNames(t, src, ext); !reflect.DeepEqual(got, []string{"Exported"}) {
		t.Errorf("a qualified write from an external test counts (unless the import is shadowed): got %v", got)
	}
	if got := globalNames(t, src, "package ext\n\nfunc T() { priv++ }\n"); !reflect.DeepEqual(got, []string{"priv"}) {
		t.Errorf("an in-package test write counts: got %v", got)
	}
}
