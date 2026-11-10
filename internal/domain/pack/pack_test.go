package pack

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func goPack(t *testing.T) Pack {
	t.Helper()
	b, err := os.ReadFile("../../../specs/languages/go/pack.json")
	if err != nil {
		t.Fatal(err)
	}
	p, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestGoPackParses(t *testing.T) {
	p := goPack(t)
	if p.ID != "go" || p.Race.Mode != RaceFlag || p.Coverage.Format != FormatGoCover {
		t.Fatalf("%+v", p)
	}
	names := []string{}
	for _, c := range p.Checks {
		names = append(names, c.Name)
	}
	want := []string{"build", "gofmt", "vet", "staticcheck", "arch", "test", "coverage", "govulncheck"}
	if !slices.Equal(names, want) {
		t.Fatalf("checks %v, want %v", names, want)
	}
	cov, _ := p.CheckByName("coverage")
	if cov.IsSkippable() {
		t.Fatal("coverage must not be skippable")
	}
	v, _ := p.CheckByName("govulncheck")
	if !v.OffByDefault() {
		t.Fatal("govulncheck should be off by default")
	}
	if !p.RaceCheck("test") || p.RaceCheck("vet") {
		t.Fatal("race checks")
	}
}

func TestParseRejectsUnknownFieldsAndBadData(t *testing.T) {
	if _, err := Parse([]byte(`{"schema":1,"id":"x","typo":1}`)); err == nil {
		t.Fatal("unknown field accepted")
	}
	if _, err := Parse([]byte(`not json`)); err == nil {
		t.Fatal("bad json accepted")
	}
	bad := Pack{Schema: 2, ID: "Bad ID", Race: Race{Mode: "maybe", Checks: []string{"nope"}},
		Coverage: Coverage{Format: "clover"}, Arch: Arch{Parser: "ast"},
		Checks: []Check{
			{Name: "a", Kind: "weird", Internal: "x", Default: "sometimes", FailsOn: "never", ExitCodes: map[string]string{"3": "maybe"}, Repeat: -1, FailRegex: "(", FallbackArgv: []string{"x"}},
			{Name: "a", Kind: "test", Argv: []string{"t", "{nope}"}},
			{Name: "i", Kind: "arch", Internal: "arch", Argv: []string{"x"}},
			{Name: "", Kind: "test"},
		},
		Detect:       []Marker{{File: "a", Glob: "b"}, {Glob: "["}, {Glob: "*.x", Contains: "y"}},
		SourceGlobs:  []string{"["},
		CacheEnv:     map[string]string{"X": "/abs"},
		Placeholders: map[string]Placeholder{"p": {}, "q": {File: "f"}},
		Variants:     []Variant{{Name: ""}, {Name: "v", Coverage: &Coverage{Format: "bad"}}},
		TestCommands: []TestCommand{{}},
	}
	err := Validate(bad)
	if err == nil {
		t.Fatal("bad pack accepted")
	}
	for _, want := range []string{"schema 2", "id", "name is empty", "as_of", "argv_prefix", "exactly one of file, glob",
		"contains needs file", "malformed", "test_globs is empty", "kind \"weird\"", "internal \"x\"", "default \"sometimes\"",
		"fails_on", "exit_codes", "fallback_argv needs when_file", "repeat < 0", "duplicate check", "has argv",
		"no name", "unknown placeholder {nope}", "race.mode", "race.checks: unknown", "coverage.format", "coverage.path",
		"produced_by is empty", "cache_env.X", "arch.parser", "placeholders.p", "file needs regex", "variants: empty",
		"variants.v.coverage.format", "variants.v.detect is empty"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in:\n%v", want, err)
		}
	}
}

func TestValidateRaceAndArchRules(t *testing.T) {
	p := Pack{Schema: 1, ID: "x", Name: "X", Versions: Versions{AsOf: "2026-11"},
		Detect: []Marker{{File: "x"}}, SourceGlobs: []string{"**/*.x"}, TestGlobs: []string{"**/*_test.x"},
		TestCommands: []TestCommand{{ArgvPrefix: []string{"x", "test"}}},
		Checks: []Check{{Name: "test", Kind: "test", Argv: []string{"x", "test"}, Required: true},
			{Name: "lint", Kind: "lint", Argv: []string{"x", "lint"}}},
		Race:     Race{Mode: RaceFlag, Checks: []string{"lint"}},
		Coverage: Coverage{Format: FormatLCOV, Path: "{coverage}", ProducedBy: []string{"ghost"}},
		Arch:     Arch{Parser: ParserRegex, NativeCheck: "lint", ImportRegex: "("},
	}
	err := Validate(p)
	for _, want := range []string{"mode flag needs flags", "must be required", "produced_by: unknown check", "needs import_regex", "arch.import_regex", "not an arch-kind"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in %v", want, err)
		}
	}
	p.Race = Race{Mode: RaceNone, Checks: []string{"test"}}
	p.Coverage.ProducedBy = []string{"test"}
	p.Arch = Arch{Parser: ParserNone}
	if err := Validate(p); err == nil || !strings.Contains(err.Error(), "needs a substitute") {
		t.Fatalf("want substitute error, got %v", err)
	}
	p.Race.Substitute = "random order, 3 runs"
	if err := Validate(p); err != nil {
		t.Fatal(err)
	}
	if err := Validate(Pack{}); err == nil || !strings.Contains(err.Error(), "checks is empty") {
		t.Fatalf("want empty-checks error, got %v", err)
	}
}

type view struct {
	files map[string]string
}

func (v view) Exists(rel string) bool {
	if _, ok := v.files[rel]; ok {
		return true
	}
	for f := range v.files {
		if strings.HasPrefix(f, rel+"/") {
			return true
		}
	}
	return false
}
func (v view) Read(rel string) (string, bool) { s, ok := v.files[rel]; return s, ok }
func (v view) List(dir string) []string {
	var out []string
	for f := range v.files {
		if dir == "." && !strings.Contains(f, "/") {
			out = append(out, f)
		}
		if rest, ok := strings.CutPrefix(f, dir+"/"); ok && !strings.Contains(rest, "/") {
			out = append(out, rest)
		}
	}
	return out
}

func TestDetectAndVariants(t *testing.T) {
	p := Pack{ID: "java", Detect: []Marker{{File: "pom.xml"}, {File: "build.gradle.kts"}},
		DetectNot: []Marker{{File: "src/main/kotlin"}},
		Checks: []Check{{Name: "test", Kind: "test", Argv: []string{"./mvnw", "verify"}, Required: true},
			{Name: "lint", Kind: "lint", Argv: []string{"x"}}},
		Coverage: Coverage{Format: FormatJaCoCo, Path: "target/site/jacoco/jacoco.xml", ProducedBy: []string{"test"}},
		Variants: []Variant{{Name: "gradle", Detect: []Marker{{Glob: "build.gradle*"}},
			Checks:    []Check{{Name: "test", Argv: []string{"./gradlew", "test", "--rerun"}}, {Name: "extra", Kind: "sbom", Argv: []string{"y"}}},
			Coverage:  &Coverage{Path: "build/reports/jacoco/test/jacocoTestReport.xml"},
			CacheArgv: map[string][]string{"./gradlew": {"--project-cache-dir", "{run}/.gradle"}},
			Race:      &Race{Mode: RaceNone, Checks: []string{"test"}, Substitute: "s"},
			PostEdit:  &PostEdit{}, CacheEnv: map[string]string{"A": "tmp"}, Env: map[string]string{"E": "1"},
			Artifacts: []string{"build/"}, TestCommands: []TestCommand{{ArgvPrefix: []string{"./gradlew", "test"}}}}},
	}
	maven := view{files: map[string]string{"pom.xml": "<project/>", "src/main/java/A.java": ""}}
	gradle := view{files: map[string]string{"build.gradle.kts": "plugins { java }"}}
	kotlin := view{files: map[string]string{"build.gradle.kts": "", "src/main/kotlin/A.kt": ""}}
	if !p.Detected(maven) || !p.Detected(gradle) || p.Detected(kotlin) {
		t.Fatal("detect")
	}
	if r := p.Resolve(maven); r.Variant != "" || r.Checks[0].Argv[0] != "./mvnw" {
		t.Fatalf("maven resolve: %+v", r)
	}
	r := p.Resolve(gradle)
	if r.Variant != "gradle" || r.Checks[0].Argv[0] != "./gradlew" || r.Checks[0].Kind != "test" || !r.Checks[0].Required {
		t.Fatalf("gradle resolve: %+v", r.Checks)
	}
	if len(r.Checks) != 3 || r.Coverage.Format != FormatJaCoCo || r.Coverage.Path != "build/reports/jacoco/test/jacocoTestReport.xml" {
		t.Fatalf("merge: %+v %+v", r.Checks, r.Coverage)
	}
	if r.Race.Mode != RaceNone || r.CacheArgv["./gradlew"][0] != "--project-cache-dir" || !r.IsTestCommand([]string{"./gradlew", "test", "x"}) {
		t.Fatal("overlay fields")
	}
	if p.Checks[0].Argv[0] != "./mvnw" {
		t.Fatal("Resolve mutated the base pack")
	}
	if _, err := p.WithVariant("gradle"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.WithVariant("sbt"); err == nil {
		t.Fatal("unknown variant accepted")
	}
	contains := Marker{File: "global.json", Contains: "Microsoft.Testing.Platform"}
	if Matches([]Marker{contains}, view{files: map[string]string{"global.json": "{}"}}) ||
		!Matches([]Marker{contains}, view{files: map[string]string{"global.json": `{"test":{"runner":"Microsoft.Testing.Platform"}}`}}) {
		t.Fatal("contains")
	}
	if Matches(nil, maven) || !Matches([]Marker{{Glob: "src/main/java/*.java"}}, maven) || Matches([]Marker{{Glob: "*.cs"}}, maven) {
		t.Fatal("globs")
	}
	if !Matches([]Marker{{Glob: "**/*.java"}}, view{files: map[string]string{"A.java": ""}}) {
		t.Fatal("wildcard dir glob")
	}
}

func TestPredicates(t *testing.T) {
	p := goPack(t)
	if !p.IsSource("internal/a.go") || p.IsSource("internal/a_test.go") || !p.IsTest("a_test.go") || p.IsSource("a.py") {
		t.Fatal("source/test")
	}
	if !p.IsGenerated("x/y.pb.go", "") || !p.IsGenerated("x/y.go", "// header\n// Code generated by stringer; DO NOT EDIT.\npackage x") || p.IsGenerated("x/y.go", "package x") {
		t.Fatal("generated")
	}
	if !p.IsTestCommand([]string{"go", "test", "./..."}) || p.IsTestCommand([]string{"go", "vet"}) || p.IsTestCommand(nil) {
		t.Fatal("test command")
	}
	bad := Pack{Generated: Generated{HeaderRegex: "("}}
	if bad.IsGenerated("a", "x") {
		t.Fatal("bad regex matched")
	}
	long := strings.Repeat("x\n", 30) + "// Code generated x DO NOT EDIT."
	if p.IsGenerated("a.go", long) {
		t.Fatal("header beyond line 20 counted")
	}
}

func TestExpand(t *testing.T) {
	vals := map[string][]string{"pkgs": {"./a/...", "./b"}, "coverage": {"/tmp/c.out"}, "coverage_args": nil, "seed": {"7"}}
	got, err := Expand([]string{"go", "test", "{coverage_args}", "-x={seed}", "{pkgs}"}, vals)
	if err != nil || strings.Join(got, " ") != "go test -x=7 ./a/... ./b" {
		t.Fatalf("%v %v", got, err)
	}
	if _, err := Expand([]string{"{nope}"}, vals); err == nil {
		t.Fatal("unknown whole placeholder")
	}
	if _, err := Expand([]string{"-p={nope}"}, vals); err == nil {
		t.Fatal("unknown inline placeholder")
	}
	if _, err := Expand([]string{"-p={pkgs}"}, vals); err == nil {
		t.Fatal("list in inline placeholder")
	}
	if ph := Placeholders([]string{"{a}", "x{b}y{a}"}); strings.Join(ph, ",") != "a,b" {
		t.Fatalf("%v", ph)
	}
	if !LooksLikeSource("A.Java") || !LooksLikeSource("x/y.ex") || LooksLikeSource("README.md") {
		t.Fatal("LooksLikeSource")
	}
}
