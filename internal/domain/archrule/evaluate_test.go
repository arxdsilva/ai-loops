package archrule

import (
	"sort"
	"strings"
	"testing"
)

const mod = "example.com/app"

// pkg builds a package at dir with one file, dir/x.go.
func pkg(dir string) Package {
	return Package{Dir: dir, ImportPath: mod + "/" + dir, Files: []string{dir + "/x.go"}, Lines: 10}
}

func at(file string, line int) Pos { return Pos{File: file, Line: line} }

func withImports(p Package, paths ...string) Package {
	for i, ip := range paths {
		p.Imports = append(p.Imports, Import{Path: ip, Pos: at(p.Files[0], 3+i)})
	}
	return p
}

func eval(pkgs ...Package) Report {
	return Evaluate(Module{Path: mod, Packages: pkgs}, Layers{})
}

func only(r Report, rule string) []Finding {
	var out []Finding
	for _, f := range r.Findings {
		if f.Rule == rule {
			out = append(out, f)
		}
	}
	return out
}

func TestEvaluateCleanModule(t *testing.T) {
	r := eval(withImports(pkg("internal/domain/order"), "strings", "fmt", mod+"/pkg/money", mod+"/internal/domain/money"))
	if r.Module != mod {
		t.Errorf("Module = %q, want %q", r.Module, mod)
	}
	if r.Findings == nil || len(r.Findings) != 0 {
		t.Fatalf("want an empty, non-nil findings slice, got %#v", r.Findings)
	}
	if !r.OK() {
		t.Fatal("a clean module must be OK")
	}
}

func TestImportRules(t *testing.T) {
	cases := []struct {
		from, imp string
		want      Severity // "" means no finding
	}{
		// domain depends on nothing outward
		{"internal/domain/order", mod + "/internal/application/orders", SeverityError},
		{"internal/domain/order", mod + "/internal/ports/store", SeverityError},
		{"internal/domain/order", mod + "/internal/adapters/inbound/cli", SeverityError},
		{"internal/domain/order", mod + "/internal/adapters/outbound/db", SeverityError},
		{"internal/domain/order", mod + "/internal/platform/clock", SeverityError},
		{"internal/domain/order", mod + "/internal/compose/app", SeverityError},
		{"internal/domain/order", mod + "/cmd/app", SeverityError},
		{"internal/domain/order", "os", SeverityError},
		{"internal/domain/order", "os/exec", SeverityError},
		{"internal/domain/order", "net/http", SeverityError},
		{"internal/domain/order", "net/url", ""},
		{"internal/domain/order", "osx", ""},
		{"internal/domain/order", mod + "/internal/domain/money", ""},
		{"internal/domain/order", mod + "/pkg/money", ""},
		// application and ports never see adapters
		{"internal/application/orders", mod + "/internal/adapters/inbound/cli", SeverityError},
		{"internal/application/orders", mod + "/internal/adapters/outbound/db", SeverityError},
		{"internal/application/orders", mod + "/internal/domain/order", ""},
		{"internal/application/orders", mod + "/internal/ports/store", ""},
		{"internal/application/orders", "os", ""},
		{"internal/ports/store", mod + "/internal/adapters/outbound/db", SeverityError},
		{"internal/ports/store", mod + "/internal/adapters/inbound/cli", SeverityError},
		// outbound never sees inbound; inbound may see outbound
		{"internal/adapters/outbound/db", mod + "/internal/adapters/inbound/cli", SeverityError},
		{"internal/adapters/outbound/db", mod + "/internal/application/orders", ""},
		{"internal/adapters/inbound/cli", mod + "/internal/adapters/outbound/db", ""},
		// the standard-library core
		{"internal/domain/order", "github.com/x/y", SeverityError},
		{"internal/application/orders", "golang.org/x/sync/errgroup", SeverityError},
		{"pkg/money", "github.com/x/y", SeverityError},
		{"internal/adapters/outbound/db", "github.com/x/y", ""},
		{"internal/platform/clock", "github.com/x/y", ""},
		// only cmd builds the composition root
		{"internal/adapters/inbound/cli", mod + "/internal/compose/app", SeverityError},
		{"internal/application/orders", mod + "/internal/compose/app", SeverityError},
		{"tools/gen", mod + "/internal/compose/app", SeverityError},
		{"cmd/app", mod + "/internal/compose/app", ""},
		{"internal/compose/app", mod + "/internal/compose/app/wire", ""},
		// a sub-package is a consumer of its parent, never its importer
		{"internal/application/orders/refund", mod + "/internal/application/orders", SeverityError},
		{"internal/domain/order/line", mod + "/internal/domain/order", SeverityError},
		{"internal/domain/order/line/tax", mod + "/internal/domain/order", SeverityError},
		{"internal/adapters/outbound/db/sqlite", mod + "/internal/adapters/outbound/db", SeverityWarn},
		{"tools/gen/sub", mod + "/tools/gen", SeverityWarn},
		{"internal/application/orders", mod + "/internal/application/orders/refund", ""},
		{"internal/application/ordersx", mod + "/internal/application/orders", ""},
		// the module root is no package's "parent" (library layouts)
		{"cmd/app", mod, ""},
		{"internal/application/orders", mod, ""},
		{"internal/domain/order", mod, ""},
		// any adapter tree, not only inbound/outbound (FN2)
		{"internal/domain/order", mod + "/internal/adapters/shared", SeverityError},
		{"internal/domain/order", mod + "/internal/adapters", SeverityError},
		{"internal/application/orders", mod + "/internal/adapters/shared/x", SeverityError},
		{"internal/ports/store", mod + "/internal/adapters/db", SeverityError},
		{"internal/adapters/inbound/cli", mod + "/internal/adapters/shared", ""},
		{"internal/adapters/shared", mod + "/internal/adapters/inbound/cli", ""},
	}
	for _, c := range cases {
		r := eval(withImports(pkg(c.from), c.imp))
		got := only(r, "imports")
		switch {
		case c.want == "" && len(got) != 0:
			t.Errorf("%s -> %s: want no finding, got %+v", c.from, c.imp, got)
		case c.want != "" && len(got) != 1:
			t.Errorf("%s -> %s: want one %s finding, got %+v", c.from, c.imp, c.want, got)
		case c.want != "" && got[0].Severity != c.want:
			t.Errorf("%s -> %s: severity %s, want %s", c.from, c.imp, got[0].Severity, c.want)
		case c.want != "" && (got[0].File != c.from+"/x.go" || got[0].Line != 3):
			t.Errorf("%s -> %s: at %s:%d, want %s/x.go:3", c.from, c.imp, got[0].File, got[0].Line, c.from)
		case c.want != "" && !strings.Contains(got[0].Msg, c.imp):
			t.Errorf("%s -> %s: message %q does not name the import", c.from, c.imp, got[0].Msg)
		}
	}
}

func TestImportFindingPerSpec(t *testing.T) {
	p := pkg("internal/domain/order")
	p.Files = append(p.Files, "internal/domain/order/y.go")
	p.Imports = []Import{{Path: "os", Pos: at("internal/domain/order/x.go", 3)}, {Path: "os", Pos: at("internal/domain/order/y.go", 4)}}
	if got := only(eval(p), "imports"); len(got) != 2 {
		t.Fatalf("want a finding per import spec, got %+v", got)
	}
}

func TestLayersFallBackFieldByField(t *testing.T) {
	m := Module{Path: mod, Packages: []Package{
		withImports(pkg("core/order"), "os", mod+"/internal/application/orders"),
		withImports(pkg("internal/domain/order"), "os"),
	}}
	r := Evaluate(m, Layers{Domain: []string{"./core/"}})
	got := only(r, "imports")
	if len(got) != 2 {
		t.Fatalf("want core (custom domain) -> os and -> default application, got %+v", got)
	}
	for _, f := range got {
		if !strings.HasPrefix(f.File, "core/") {
			t.Errorf("internal/domain is not the domain any more, got %+v", f)
		}
	}
}

func TestAdaptersLayerIsConfigurable(t *testing.T) {
	m := Module{Path: mod, Packages: []Package{withImports(pkg("internal/application/orders"), mod+"/plugins/db", mod+"/internal/adapters/shared")}}
	got := only(Evaluate(m, Layers{Adapters: []string{"plugins"}}), "imports")
	if len(got) != 1 || !strings.Contains(got[0].Msg, "plugins/db") {
		t.Fatalf("a custom adapters root replaces internal/adapters, got %+v", got)
	}
}

func TestLongestLayerPrefixWins(t *testing.T) {
	m := Module{Path: mod, Packages: []Package{
		withImports(pkg("core/plat"), "os"),
		withImports(pkg("core/order"), "os"),
	}}
	r := Evaluate(m, Layers{Domain: []string{"core"}, Platform: []string{"core/plat"}})
	got := only(r, "imports")
	if len(got) != 1 || got[0].File != "core/order/x.go" {
		t.Fatalf("core/plat is platform, core/order is domain; got %+v", got)
	}
}

func TestInitRule(t *testing.T) {
	p := pkg("internal/adapters/outbound/db")
	p.Inits = []Pos{at("internal/adapters/outbound/db/x.go", 7)}
	got := only(eval(p), "init")
	if len(got) != 1 || got[0].Severity != SeverityError || got[0].Line != 7 {
		t.Fatalf("want one init error at line 7, got %+v", got)
	}
	if got := only(eval(pkg("internal/adapters/outbound/db")), "init"); len(got) != 0 {
		t.Fatalf("no init, no finding; got %+v", got)
	}
}

func TestGlobalRule(t *testing.T) {
	p := pkg("internal/platform/cache")
	p.Globals = []Global{{Name: "hits", Decl: at("internal/platform/cache/x.go", 5), Assigned: at("internal/platform/cache/x_test.go", 20)}}
	got := only(eval(p), "global")
	if len(got) != 1 || got[0].Severity != SeverityError || got[0].Line != 5 || got[0].File != "internal/platform/cache/x.go" {
		t.Fatalf("want one global error at the declaration, got %+v", got)
	}
	if !strings.Contains(got[0].Msg, "hits") || !strings.Contains(got[0].Msg, "x_test.go:20") {
		t.Errorf("message must name the var and where it is assigned: %q", got[0].Msg)
	}
	if got := only(eval(pkg("internal/platform/cache")), "global"); len(got) != 0 {
		t.Fatalf("no globals, no finding; got %+v", got)
	}
}

func TestDomainPurity(t *testing.T) {
	impure := func(dir string) Package {
		p := pkg(dir)
		f := p.Files[0]
		p.GoStmts = []Pos{at(f, 9)}
		p.Calls = []Call{
			{Pkg: "time", Name: "Now", Pos: at(f, 10)},
			{Pkg: "math/rand", Name: "Intn", Pos: at(f, 11)},
			{Pkg: "math/rand/v2", Name: "N", Pos: at(f, 12)},
			{Pkg: "math/rand", Name: "New", Pos: at(f, 13)},
			{Pkg: "math/rand", Name: "NewSource", Pos: at(f, 14)},
			{Pkg: "math/rand/v2", Name: "NewPCG", Pos: at(f, 15)},
			{Pkg: "math/rand/v2", Name: "NewChaCha8", Pos: at(f, 16)},
			{Pkg: "math/rand/v2", Name: "NewZipf", Pos: at(f, 17)},
			{Pkg: "time", Name: "Since", Pos: at(f, 18)},
		}
		return p
	}
	r := eval(impure("internal/domain/order"))
	want := map[string][]int{"go": {9}, "clock": {10}, "rand": {11, 12}}
	for rule, lines := range want {
		got := only(r, rule)
		if len(got) != len(lines) {
			t.Errorf("%s: want lines %v, got %+v", rule, lines, got)
			continue
		}
		for i, f := range got {
			if f.Line != lines[i] || f.Severity != SeverityError {
				t.Errorf("%s: want error at %d, got %+v", rule, lines[i], f)
			}
		}
	}
	if r := eval(impure("internal/application/orders")); len(r.Findings) != 0 {
		t.Fatalf("clock, rand and go are domain-only rules, got %+v", r.Findings)
	}
}

func cmdModule(dirs []Dir, pkgs ...Package) Module {
	return Module{Path: mod, Dirs: dirs, Packages: pkgs}
}

func mainPkg(dir string, mf MainFile) Package {
	p := Package{Dir: dir, ImportPath: mod + "/" + dir, Files: []string{dir + "/main.go"}}
	mf.File = dir + "/main.go"
	p.Main = &mf
	return p
}

func TestCmdRule(t *testing.T) {
	good := MainFile{Lines: 12, Decls: []Fact{{What: "func main", Pos: at("cmd/ok/main.go", 9)}}, Strings: []StringLit{{Runes: 40, Pos: at("cmd/ok/main.go", 10)}}}
	m := cmdModule(
		[]Dir{
			{Path: "cmd", Entries: []string{"ok/", "extra/", "sub/", "nomain/", "helper/", "logic/", "long/", "README.md"}},
			{Path: "cmd/ok", Entries: []string{"main.go"}},
			{Path: "cmd/extra", Entries: []string{"main.go", "flags.go"}},
			{Path: "cmd/sub", Entries: []string{"main.go", "render/"}},
			{Path: "cmd/sub/render", Entries: []string{"r.go"}},
			{Path: "cmd/nomain", Entries: []string{"tool.go"}},
			{Path: "cmd/helper", Entries: []string{"main.go"}},
			{Path: "cmd/logic", Entries: []string{"main.go"}},
			{Path: "cmd/long", Entries: []string{"main.go"}},
			{Path: "internal/x", Entries: []string{"a.go", "b.go"}},
		},
		mainPkg("cmd/ok", good),
		mainPkg("cmd/extra", good),
		mainPkg("cmd/sub", good),
		mainPkg("cmd/helper", MainFile{Lines: 10, Decls: []Fact{
			{What: "func main", Pos: at("cmd/helper/main.go", 5)},
			{What: "func help", Pos: at("cmd/helper/main.go", 7)},
			{What: "var declaration", Pos: at("cmd/helper/main.go", 3)},
		}}),
		mainPkg("cmd/logic", MainFile{Lines: 10, Decls: []Fact{{What: "func main", Pos: at("cmd/logic/main.go", 3)}},
			Logic: []Fact{
				{What: "for/range statement", Pos: at("cmd/logic/main.go", 4)},
				{What: "switch statement", Pos: at("cmd/logic/main.go", 5)},
				{What: "select statement", Pos: at("cmd/logic/main.go", 6)},
				{What: "go statement", Pos: at("cmd/logic/main.go", 7)},
				{What: "function literal", Pos: at("cmd/logic/main.go", 7)},
			},
			Strings: []StringLit{{Runes: 41, Pos: at("cmd/logic/main.go", 8)}},
		}),
		mainPkg("cmd/long", MainFile{Lines: 81, Decls: []Fact{{What: "func main", Pos: at("cmd/long/main.go", 3)}}}),
		Package{Dir: "cmd/sub/render", ImportPath: mod + "/cmd/sub/render", Files: []string{"cmd/sub/render/r.go"}},
	)
	r := Evaluate(m, Layers{})
	byFile := map[string][]string{}
	for _, f := range only(r, "cmd") {
		if f.Severity != SeverityError {
			t.Errorf("cmd findings are errors: %+v", f)
		}
		byFile[f.File] = append(byFile[f.File], f.Msg)
	}
	if len(byFile["cmd/ok/main.go"]) != 0 || len(byFile["cmd/ok"]) != 0 {
		t.Errorf("a wiring-only main must pass, got %v", byFile)
	}
	want := map[string][]string{
		"cmd/extra":          {"flags.go"},
		"cmd/sub":            {"render/"},
		"cmd/nomain":         {"tool.go", "no main.go"},
		"cmd/helper/main.go": {"func help", "var declaration"},
		"cmd/logic/main.go":  {"for/range", "switch", "select", "go statement", "function literal", "41 runes"},
		"cmd/long/main.go":   {"81 lines"},
	}
	for file, needles := range want {
		got := strings.Join(byFile[file], "; ")
		for _, n := range needles {
			if !strings.Contains(got, n) {
				t.Errorf("%s: want %q in %q", file, n, got)
			}
		}
	}
	if n := len(byFile["cmd/helper/main.go"]); n != 2 {
		t.Errorf("func main is allowed: want 2 helper findings, got %v", byFile["cmd/helper/main.go"])
	}
	if len(byFile["cmd"]) != 0 || len(byFile["cmd/sub/render"]) != 0 {
		t.Errorf("only direct cmd/<x> dirs are checked, got %v", byFile)
	}
	if got := only(r, "cmd"); len(got) != 13 {
		t.Errorf("want 13 cmd findings, got %d: %+v", len(got), got)
	}
}

func TestCmdRuleUsesCmdLayer(t *testing.T) {
	m := cmdModule([]Dir{{Path: "tools/x", Entries: []string{"main.go", "extra.go"}}, {Path: "cmd/y", Entries: []string{"main.go", "extra.go"}}})
	got := only(Evaluate(m, Layers{Cmd: []string{"tools"}}), "cmd")
	if len(got) != 1 || got[0].File != "tools/x" || !strings.Contains(got[0].Msg, "extra.go") {
		t.Fatalf("the cmd layer is configurable: want tools/x findings only, got %+v", got)
	}
}

func TestLimits(t *testing.T) {
	p := pkg("internal/adapters/outbound/db")
	f := p.Files[0]
	p.Types = []Type{
		{Name: "Big", Kind: KindStruct, Methods: 41, Pos: at(f, 1)},
		{Name: "AtLimit", Kind: KindStruct, Methods: 40, Fields: 25, Pos: at(f, 2)},
		{Name: "Wide", Kind: KindStruct, Methods: 5, Fields: 26, Pos: at(f, 3)},
		{Name: "WideData", Kind: KindStruct, Methods: 4, Fields: 26, Pos: at(f, 4)},
		{Name: "Fat", Kind: KindInterface, IfaceMethods: 9, Pos: at(f, 5)},
		{Name: "Slim", Kind: KindInterface, IfaceMethods: 8, Pos: at(f, 6)},
	}
	p.Lines = 12001
	p.Files = make([]string, 61)
	for i := range p.Files {
		p.Files[i] = f
	}
	r := eval(p)
	if !r.OK() {
		t.Fatal("size limits warn, they never fail")
	}
	want := map[string]int{"methods": 1, "fields": 1, "iface": 1, "lines": 1, "files": 1}
	for rule, n := range want {
		got := only(r, rule)
		if len(got) != n {
			t.Errorf("%s: want %d finding, got %+v", rule, n, got)
			continue
		}
		if got[0].Severity != SeverityWarn {
			t.Errorf("%s: want warn, got %s", rule, got[0].Severity)
		}
	}
	for _, f := range only(r, "lines") {
		if f.File != "internal/adapters/outbound/db" {
			t.Errorf("package-wide findings point at the package dir, got %+v", f)
		}
	}
	if got := only(r, "methods"); len(got) == 1 && (!strings.Contains(got[0].Msg, "Big") || got[0].Line != 1) {
		t.Errorf("methods finding must name the type at its declaration: %+v", got[0])
	}
	small := pkg("internal/adapters/outbound/db")
	small.Lines, small.Types = 12000, []Type{{Name: "T", Kind: KindStruct, Methods: 40, Fields: 25}}
	small.Files = make([]string, 60)
	if r := eval(small); len(r.Findings) != 0 {
		t.Fatalf("at the limit is fine, got %+v", r.Findings)
	}
}

// everything builds a module that breaks every rule once.
func everything() Module {
	d := withImports(pkg("internal/domain/order"), "os", "github.com/x/y", mod+"/internal/compose/app", mod+"/internal/domain")
	d.Inits = []Pos{at("internal/domain/order/x.go", 20)}
	d.Globals = []Global{{Name: "g", Decl: at("internal/domain/order/x.go", 2), Assigned: at("internal/domain/order/x.go", 30)}}
	d.GoStmts = []Pos{at("internal/domain/order/x.go", 31)}
	d.Calls = []Call{{Pkg: "time", Name: "Now", Pos: at("internal/domain/order/x.go", 32)}, {Pkg: "math/rand", Name: "Int", Pos: at("internal/domain/order/x.go", 33)}}
	d.Types = []Type{{Name: "A", Kind: KindStruct, Methods: 50, Fields: 30}, {Name: "I", Kind: KindInterface, IfaceMethods: 10}}
	d.Lines = 20000
	o := withImports(pkg("internal/adapters/outbound/db/sql"), mod+"/internal/adapters/outbound/db")
	return Module{
		Path:     mod,
		Packages: []Package{d, o, mainPkg("cmd/x", MainFile{Lines: 100, Decls: []Fact{{What: "func h"}}, Logic: []Fact{{What: "go statement"}}, Strings: []StringLit{{Runes: 99}}})},
		Dirs:     []Dir{{Path: "cmd/x", Entries: []string{"main.go", "b.go"}}},
	}
}

func TestFindingsSortedAndCiteSpecs(t *testing.T) {
	r := Evaluate(everything(), Layers{})
	rules := map[string]bool{}
	for _, f := range r.Findings {
		rules[f.Rule] = true
		spec := "specs/SOLID.md"
		if f.Rule == "imports" || f.Rule == "clock" || f.Rule == "rand" || f.Rule == "go" {
			spec = "specs/ARCHITECTURE.md"
		}
		if !strings.Contains(f.Msg, spec) {
			t.Errorf("%s finding must cite %s: %q", f.Rule, spec, f.Msg)
		}
		if !strings.Contains(f.Msg, f.Rule+":") {
			t.Errorf("%s finding must name its rule: %q", f.Rule, f.Msg)
		}
	}
	for _, rule := range []string{"imports", "init", "global", "cmd", "clock", "rand", "go", "methods", "fields", "iface", "lines"} {
		if !rules[rule] {
			t.Errorf("rule %s never fired", rule)
		}
	}
	if !sort.SliceIsSorted(r.Findings, func(i, j int) bool { return less(r.Findings[i], r.Findings[j]) }) {
		t.Errorf("findings not sorted by file, line, rule: %+v", r.Findings)
	}
}

func less(a, b Finding) bool {
	if a.File != b.File {
		return a.File < b.File
	}
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	return a.Rule < b.Rule
}

func TestEvaluateDoesNotMutateInput(t *testing.T) {
	m := everything()
	before := len(m.Packages[0].Imports)
	l := Layers{}
	Evaluate(m, l)
	if len(m.Packages[0].Imports) != before || l.Domain != nil {
		t.Fatal("Evaluate must not mutate its arguments")
	}
}
