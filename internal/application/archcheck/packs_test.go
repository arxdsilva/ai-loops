package archcheck_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/goparse"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/importscan"
	"github.com/arxdsilva/ai-loops/internal/application/archcheck"
	"github.com/arxdsilva/ai-loops/internal/application/langpack"
	"github.com/arxdsilva/ai-loops/internal/domain/archrule"
	"github.com/arxdsilva/ai-loops/internal/domain/pack"
)

var goArch = pack.Arch{
	Parser: pack.ParserGo, Resolver: "module-path",
	ForbiddenInDomain:     []string{"os", "net/http"},
	ThirdPartyForbiddenIn: []string{"domain", "application", "pkg"},
	ImpureCalls:           []string{"time.Now", "math/rand", "math/rand/v2", "go"},
}

func detected(id, root string, a pack.Arch) langpack.Detected {
	p := pack.Pack{ID: id, Arch: a, SourceGlobs: []string{"**/*.java"}}
	return langpack.Detected{Pack: p, ID: id, Root: root}
}

// The Go pack at the project root gives exactly today's report.
func TestCheckPacksGoAtRootIsToday(t *testing.T) {
	ctx := context.Background()
	svc := archcheck.New(goparse.New())
	for _, name := range []string{"clean", "imports", "purity", "cmd", "limits"} {
		root := filepath.Join(fixtures, name)
		want, err := svc.Check(ctx, root, archrule.Layers{})
		if err != nil {
			t.Fatal(err)
		}
		res, err := svc.CheckPacks(ctx, root, archrule.Layers{}, []langpack.Detected{detected("go", ".", goArch)})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(res.Report, want) || len(res.Notes) != 0 {
			t.Errorf("%s: CheckPacks differs from Check:\n%+v\n%+v", name, res.Report, want)
		}
		// an empty arch block keeps the Go defaults too
		res, _ = svc.CheckPacks(ctx, root, archrule.Layers{}, []langpack.Detected{detected("go", ".", pack.Arch{})})
		if !reflect.DeepEqual(res.Report, want) {
			t.Errorf("%s: the Go defaults must apply to an empty arch block", name)
		}
	}
}

type fakeScanner struct {
	m    archrule.Module
	err  error
	root string
	id   string
}

func (f *fakeScanner) Scan(_ context.Context, root string, p pack.Pack) (archrule.Module, error) {
	f.root, f.id = root, p.ID
	return f.m, f.err
}

var javaArch = pack.Arch{Parser: pack.ParserRegex, ImportRegex: "import", Resolver: "package-dir",
	ForbiddenInDomain: []string{"org.springframework"}}

func javaModule() archrule.Module {
	return archrule.Module{Path: "java", Parser: "regex", Packages: []archrule.Package{{
		Dir: "src/main/java/x/domain", Files: []string{"src/main/java/x/domain/A.java"},
		Imports: []archrule.Import{{Path: "x.adapters.Db", Kind: archrule.ImportInternal, Dir: "src/main/java/x/adapters",
			Pos: archrule.Pos{File: "src/main/java/x/domain/A.java", Line: 3}}},
	}}}
}

func TestCheckPacksMergesAndPrefixes(t *testing.T) {
	sc := &fakeScanner{m: javaModule()}
	p := &fakeParser{m: archrule.Module{Path: "example.com/m", Packages: []archrule.Package{{
		Dir: "internal/domain/x", Files: []string{"internal/domain/x/x.go"},
		Imports: []archrule.Import{{Path: "os", Pos: archrule.Pos{File: "internal/domain/x/x.go", Line: 4}}},
	}}}}
	svc := archcheck.New(p).WithScanner(sc)
	ds := []langpack.Detected{
		detected("go", ".", goArch),
		detected("java", "services/api", javaArch),
		detected("python", "tools", pack.Arch{Parser: pack.ParserNone, NativeCheck: "import-linter"}),
		detected("ruby", "web", pack.Arch{Parser: pack.ParserNone}),
	}
	res, err := svc.CheckPacks(context.Background(), "/proj", archrule.Layers{}, ds)
	if err != nil {
		t.Fatal(err)
	}
	if p.root != "/proj" || sc.root != filepath.Join("/proj", "services", "api") || sc.id != "java" {
		t.Errorf("parsed %q, scanned %q (%s)", p.root, sc.root, sc.id)
	}
	var got []string
	for _, f := range res.Report.Findings {
		got = append(got, f.File+" "+f.Rule+" "+strings.SplitN(f.Msg, ":", 2)[0])
	}
	want := []string{
		"internal/domain/x/x.go imports go@.",
		"services/api/src/main/java/x/domain/A.java imports java@services/api",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %q, want %q", got, want)
	}
	if res.Report.Module != "go@., java@services/api" || res.Report.OK() {
		t.Errorf("module = %q ok=%v", res.Report.Module, res.Report.OK())
	}
	wantNotes := []string{
		"arch: python delegates to its native check import-linter (run by loops gate)",
		"arch: ruby has no built-in parser and no native check",
	}
	if !reflect.DeepEqual(res.Notes, wantNotes) {
		t.Errorf("notes = %q", res.Notes)
	}
}

func TestCheckPacksSinglePackKeepsMessages(t *testing.T) {
	svc := archcheck.New(&fakeParser{}).WithScanner(&fakeScanner{m: javaModule()})
	res, err := svc.CheckPacks(context.Background(), "/p", archrule.Layers{}, []langpack.Detected{detected("java", "api", javaArch)})
	if err != nil {
		t.Fatal(err)
	}
	f := res.Report.Findings[0]
	if f.File != "api/src/main/java/x/domain/A.java" || !strings.HasPrefix(f.Msg, "imports:") || res.Report.Module != "java" {
		t.Fatalf("finding = %+v module %q", f, res.Report.Module)
	}
}

func TestCheckPacksErrors(t *testing.T) {
	boom := errors.New("boom")
	ctx := context.Background()
	cases := map[string]struct {
		svc *archcheck.Service
		d   langpack.Detected
	}{
		"parse":      {archcheck.New(&fakeParser{err: boom}), detected("go", ".", goArch)},
		"scan":       {archcheck.New(nil).WithScanner(&fakeScanner{err: boom}), detected("java", ".", javaArch)},
		"no scanner": {archcheck.New(nil), detected("java", ".", javaArch)},
		"parser":     {archcheck.New(nil), detected("x", ".", pack.Arch{Parser: "ast"})},
	}
	for name, c := range cases {
		_, err := c.svc.CheckPacks(ctx, ".", archrule.Layers{}, []langpack.Detected{c.d})
		if err == nil || !strings.Contains(err.Error(), c.d.ID+"@.") {
			t.Errorf("%s: err = %v, want one naming the pack", name, err)
		}
	}
}

func TestPackLayers(t *testing.T) {
	cfg := archrule.Layers{
		Domain:   []string{"svc/api/core", "./internal/domain", "**/model"},
		Adapters: []string{`svc\api\infra`},
	}
	if l, ok := archcheck.PackLayers(cfg, "."); !ok || !reflect.DeepEqual(l, cfg) {
		t.Errorf("at the root the config applies unchanged: %+v %v", l, ok)
	}
	l, ok := archcheck.PackLayers(cfg, "svc/api")
	want := archrule.Layers{Domain: []string{"core", "**/model"}, Adapters: []string{"infra"}}
	if !ok || !reflect.DeepEqual(l, want) {
		t.Errorf("PackLayers = %+v %v, want %+v", l, ok, want)
	}
	if _, ok := archcheck.PackLayers(archrule.Layers{Domain: []string{"internal/domain"}}, "web"); ok {
		t.Error("entries outside the pack root do not apply")
	}
	if _, ok := archcheck.PackLayers(archrule.Layers{}, "."); ok {
		t.Error("empty config layers do not apply")
	}
}

func TestPackRules(t *testing.T) {
	a := pack.Arch{ForbiddenInDomain: []string{"x"}, SizeWarnings: map[string]int{"lines": 9}}
	if r := archcheck.PackRules(a); len(r.ThirdPartyForbiddenIn) != 0 || r.Limits["lines"] != 9 || r.ForbiddenInDomain[0] != "x" {
		t.Errorf("PackRules = %+v", r)
	}
	r := archcheck.GoPackRules(a)
	if !reflect.DeepEqual(r.ForbiddenInDomain, []string{"x"}) || !reflect.DeepEqual(r.ThirdPartyForbiddenIn, archrule.GoRules().ThirdPartyForbiddenIn) {
		t.Errorf("GoPackRules = %+v", r)
	}
}

func TestGateAdapter(t *testing.T) {
	ctx := context.Background()
	svc := archcheck.New(goparse.New()).WithScanner(&fakeScanner{m: javaModule()})
	out, ok, err := archcheck.Gate{Service: svc}.Check(ctx, filepath.Join(fixtures, "clean"))
	if err != nil || !ok || out != "" {
		t.Fatalf("clean: %q %v %v", out, ok, err)
	}
	out, ok, err = archcheck.Gate{Service: svc}.Check(ctx, filepath.Join(fixtures, "init"))
	if err != nil || ok || !strings.HasPrefix(out, "internal/platform/reg/reg.go:8 init init: ") {
		t.Fatalf("init: %q %v %v", out, ok, err)
	}
	g := archcheck.Gate{Service: svc, Packs: []langpack.Detected{detected("java", ".", javaArch)}}
	out, ok, err = g.Check(ctx, "/x")
	if err != nil || ok || !strings.HasPrefix(out, "src/main/java/x/domain/A.java:3 imports imports: ") {
		t.Fatalf("packs: %q %v %v", out, ok, err)
	}
	if _, _, err := (archcheck.Gate{Service: archcheck.New(&fakeParser{err: errors.New("x")})}).Check(ctx, "."); err == nil {
		t.Error("errors propagate")
	}
}

func TestWithPacksRoutesCheck(t *testing.T) {
	svc := archcheck.New(&fakeParser{}).WithScanner(&fakeScanner{m: javaModule()}).
		WithPacks([]langpack.Detected{detected("java", ".", javaArch)})
	rep, err := svc.Check(context.Background(), "/x", archrule.Layers{})
	if err != nil || rep.OK() || rep.Module != "java" {
		t.Fatalf("Check with packs = %+v, %v", rep, err)
	}
}

// A polyglot tree end to end: the Go module at the root and a Java service
// below it, with the real parser and scanner.
func TestPolyglotEndToEnd(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/poly\n\ngo 1.24\n")
	write("internal/domain/a/a.go", "package a\n\nimport \"os\"\n\nvar _ = os.Args\n")
	write("svc/src/main/java/x/domain/Order.java", "package x.domain;\n\nimport x.adapters.Db;\n\nclass Order { long t = Instant.now().toEpochMilli(); }\n")
	write("svc/src/main/java/x/adapters/Db.java", "package x.adapters;\n")
	java := pack.Pack{ID: "java", SourceGlobs: []string{"**/src/main/java/**/*.java"}, Arch: pack.Arch{
		Parser: pack.ParserRegex, Resolver: "package-dir", ImportRegex: `^\s*import\s+(?:static\s+)?([A-Za-z_][\w.]*)(?:\.\*)?\s*;`,
		ImpureCalls: []string{"Instant.now"},
	}}
	ds := []langpack.Detected{detected("go", ".", goArch), {Pack: java, ID: "java", Root: "svc"}}
	svc := archcheck.New(goparse.New()).WithScanner(importscan.New())
	res, err := svc.CheckPacks(context.Background(), root, archrule.Layers{}, ds)
	if err != nil {
		t.Fatal(err)
	}
	got := summary(res.Report)
	want := []string{
		"internal/domain/a/a.go:3 imports error",
		"svc/src/main/java/x/domain/Order.java:3 imports error",
		"svc/src/main/java/x/domain/Order.java:5 impure error",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findings:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
