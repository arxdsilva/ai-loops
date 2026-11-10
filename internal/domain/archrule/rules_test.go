package archrule

import (
	"reflect"
	"strconv"
	"testing"
)

func TestEvaluateUsesGoRules(t *testing.T) {
	p := withImports(pkg("internal/domain/order"), "os/exec", "net/http", "example.org/x", mod+"/internal/adapters/outbound/db")
	p.Calls = []Call{{Pkg: "time", Name: "Now", Pos: at("internal/domain/order/x.go", 9)}}
	p.GoStmts = []Pos{at("internal/domain/order/x.go", 10)}
	m := Module{Path: mod, Packages: []Package{p}}
	if a, b := Evaluate(m, Layers{}), EvaluateWith(m, Layers{}, GoRules()); !reflect.DeepEqual(a, b) {
		t.Fatalf("Evaluate != EvaluateWith(GoRules):\n%+v\n%+v", a, b)
	}
}

func TestMatchImport(t *testing.T) {
	cases := []struct {
		entry, imp string
		want       bool
	}{
		{"os", "os", true}, {"os", "os/exec", true}, {"os", "osext", false},
		{"java.io", "java.io.File", true}, {"java.io", "java.iox", false},
		{"std::fs", "std::fs::File", true}, {`Illuminate\`, `Illuminate\Support\Str`, true},
		{"ring.", "ring.middleware", true}, {"@nestjs/*", "@nestjs/common", true},
		{"boost/asio", "boost/asio.hpp", true}, {"PDO", "PDO", true}, {"PDO", "PDOStatement", false},
		{"*", "anything", false}, {"", "x", false},
	}
	for _, c := range cases {
		if got := matchImport(c.entry, c.imp); got != c.want {
			t.Errorf("matchImport(%q, %q) = %v, want %v", c.entry, c.imp, got, c.want)
		}
	}
}

// regexPkg is a non-Go package whose imports the resolver classified.
func regexPkg(dir string, imps ...Import) Package {
	f := dir + "/A.x"
	for i := range imps {
		imps[i].Pos = at(f, i+1)
	}
	return Package{Dir: dir, Files: []string{f}, Lines: 10, Imports: imps}
}

func in(dir string) Import           { return Import{Path: dir, Kind: ImportInternal, Dir: dir} }
func ext(path, kind string) Import   { return Import{Path: path, Kind: kind} }
func regexMod(ps ...Package) Module  { return Module{Path: "java", Parser: "regex", Packages: ps} }
func lines(r Report) map[string]bool { return findingSet(r) }

func findingSet(r Report) map[string]bool {
	out := map[string]bool{}
	for _, f := range r.Findings {
		out[f.File+":"+strconv.Itoa(f.Line)+" "+f.Rule+" "+string(f.Severity)] = true
	}
	return out
}

func TestConventionalLayersOnRegexModules(t *testing.T) {
	const root = "src/main/java/com/acme"
	dom := root + "/domain/order"
	m := regexMod(
		regexPkg(dom,
			in(root+"/adapters/db"),                   // 1 error: domain -> adapters
			in(root+"/application/orders"),            // 2 error: domain -> application
			in(root+"/domain/money"),                  // 3 fine
			in(root+"/domain"),                        // 4 fine: parent imports are a Go rule
			ext("org.springframework.x", ImportThird), // 5 error: forbidden
			ext("java.util.List", ImportStd),          // 6 fine
			ext("com.google.common.X", ImportThird),   // 7 fine: no third-party rule by default
		),
		regexPkg(root+"/application/orders", in(root+"/infrastructure/jpa")), // 1 error: application -> adapters
		regexPkg(root+"/adapters/out/db", in(root+"/adapters/in/web")),       // 1 error: outbound -> inbound
		regexPkg(root+"/compose", in(root+"/domain/order")),                  // fine
		regexPkg("src/Shop.Domain", in("src/Shop.Infrastructure")),           // 1 error (C# names)
	)
	r := EvaluateWith(m, ConventionalLayers("project-reference"), Rules{ForbiddenInDomain: []string{"org.springframework"}})
	want := map[string]bool{
		dom + "/A.x:1 imports error":                     true,
		dom + "/A.x:2 imports error":                     true,
		dom + "/A.x:5 imports error":                     true,
		root + "/application/orders/A.x:1 imports error": true,
		root + "/adapters/out/db/A.x:1 imports error":    true,
		"src/Shop.Domain/A.x:1 imports error":            true,
	}
	if got := lines(r); !reflect.DeepEqual(got, want) {
		t.Fatalf("findings:\n%v\nwant:\n%v", got, want)
	}
	// without the C# resolver the dotted project names are no layers
	if r := EvaluateWith(regexMod(regexPkg("src/Shop.Domain", in("src/Shop.Infrastructure"))), ConventionalLayers("package-dir"), Rules{}); len(r.Findings) != 0 {
		t.Errorf("want no findings, got %+v", r.Findings)
	}
}

func TestDeepestLayerGlobWins(t *testing.T) {
	c := newClassifier(ConventionalLayers(""))
	cases := map[string]layer{
		"src/domain":                      layerDomain,
		"src/Domain/Model":                layerDomain,
		"src/adapters/inbound/http":       layerInbound,
		"src/Adapters/Outbound":           layerOutbound,
		"src/adapters/persistence":        layerAdapters,
		"src/adapters/persistence/domain": layerDomain,
		"src/infra/db":                    layerAdapters,
		"src/usecases/place":              layerApplication,
		"lib":                             layerNone,
	}
	for dir, want := range cases {
		if got := c.layerOf(dir); got != want {
			t.Errorf("layerOf(%q) = %s, want %s", dir, got, want)
		}
	}
}

func TestGoOnlyRulesSkipNonGoModules(t *testing.T) {
	p := regexPkg("src/domain", in("src/compose"))
	p.Inits = []Pos{at("src/domain/A.x", 2)}
	p.Globals = []Global{{Name: "g", Decl: at("src/domain/A.x", 3), Assigned: at("src/domain/A.x", 4)}}
	m := regexMod(p, regexPkg("src/domain/sub", in("src/domain")))
	m.Dirs = []Dir{{Path: "cmd/x", Entries: []string{"util.go"}}}
	r := EvaluateWith(m, ConventionalLayers(""), Rules{})
	if len(r.Findings) != 0 {
		t.Fatalf("Go-only rules ran on a regex module: %+v", r.Findings)
	}
	m.Parser = "go"
	if r := EvaluateWith(m, Layers{Domain: []string{"src/domain"}, Compose: []string{"src/compose"}}, Rules{}); len(r.Findings) == 0 {
		t.Fatal("the same module parsed by Go must trip the Go-only rules")
	}
}

func TestImpureHitsOnlyInTheDomain(t *testing.T) {
	d := regexPkg("src/domain")
	d.Impure = []Hit{{Call: "Instant.now", Pos: at("src/domain/A.x", 7)}}
	a := regexPkg("src/adapters")
	a.Impure = []Hit{{Call: "Instant.now", Pos: at("src/adapters/A.x", 7)}}
	r := EvaluateWith(regexMod(d, a), ConventionalLayers(""), Rules{})
	if len(r.Findings) != 1 || r.Findings[0].File != "src/domain/A.x" || r.Findings[0].Rule != "impure" || r.OK() {
		t.Fatalf("want one impure error in the domain, got %+v", r.Findings)
	}
}

func TestRulesThirdPartyAndLimits(t *testing.T) {
	p := regexPkg("src/application", ext("com.google.X", ImportThird), ext("java.util.X", ImportStd))
	p.Lines = 600
	r := EvaluateWith(regexMod(p), ConventionalLayers(""), Rules{
		ThirdPartyForbiddenIn: []string{"application"}, Limits: map[string]int{"lines": 500},
	})
	got := lines(r)
	want := map[string]bool{"src/application/A.x:1 imports error": true, "src/application:0 lines warn": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %v, want %v", got, want)
	}
}

func TestImpureCallsGateGoPurity(t *testing.T) {
	p := pkg("internal/domain/x")
	p.Calls = []Call{
		{Pkg: "time", Name: "Now", Pos: at("internal/domain/x/x.go", 3)},
		{Pkg: "math/rand", Name: "Intn", Pos: at("internal/domain/x/x.go", 4)},
	}
	p.GoStmts = []Pos{at("internal/domain/x/x.go", 5)}
	r := EvaluateWith(Module{Path: mod, Packages: []Package{p}}, Layers{}, Rules{ImpureCalls: []string{"math/rand"}})
	if len(r.Findings) != 1 || r.Findings[0].Rule != "rand" {
		t.Fatalf("only the listed impure calls apply, got %+v", r.Findings)
	}
}

func TestCmdLayerGlob(t *testing.T) {
	c := newClassifier(Layers{Cmd: []string{"**/cmd"}})
	if !c.isCmdDir("tools/cmd/x") || c.isCmdDir("tools/cmd") {
		t.Error("a cmd glob matches the parent of cmd/<x>")
	}
}
