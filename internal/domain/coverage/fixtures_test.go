package coverage

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// The testdata reports were written by the real tools in Docker spot runs
// (one small "hello" project per language, a greet function half-covered
// and an unused function uncovered):
//
//	lcov/excoveralls.info   Elixir, excoveralls 0.18.5 `mix coveralls.lcov` (absolute SF under /tmp/hello)
//	lcov/simplecov.lcov     Ruby, simplecov-lcov single file (SF:./lib/…)
//	lcov/cloverage.info     Clojure, cloverage 1.2.4 --lcov (relative SF:src/…)
//	lcov/coveragepy.info    Python, coverage.py 7.17 `--cov-report=lcov` (FN/BRDA records)
//	cobertura/coveragepy.xml       Python, coverage.py XML (<source>/w/hello/src)
//	cobertura/coverlet-vstest.xml  C#, coverlet.collector via VSTest (<source>/</source>)
//	cobertura/mtp.xml              C#, Microsoft.Testing.Platform code coverage (BOM, no <sources>, absolute filename)
//	cobertura/covertool-a1.xml, -a2.xml  Erlang, rebar3 covertool 2.0.7, one file per app of an umbrella
//	cobertura/scoverage.xml        Scala 3, sbt-scoverage 2.3.1 (a bogus "--source" source, unsorted lines)
//	cobertura/phpunit.xml          PHP, PHPUnit 12.5 --coverage-cobertura with pcov
//	jacoco/maven.xml               Java, jacoco-maven-plugin 0.8.14 (Report 1.1 DOCTYPE)
//	jacoco/kover.xml               Kotlin, Kover 0.9.3 koverXmlReport (JaCoCo schema)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// lines renders each file's blocks as "line:count ..." for compact checks.
func lines(p Profile) map[string]string {
	out := map[string]string{}
	for f, bs := range p.Files {
		var parts []string
		for _, b := range bs {
			if b.StartLine != b.EndLine || b.NumStmt != 1 {
				parts = append(parts, fmt.Sprintf("bad block %+v", b))
				continue
			}
			parts = append(parts, fmt.Sprintf("%d:%d", b.StartLine, b.Count))
		}
		out[f] = strings.Join(parts, " ")
	}
	return out
}

func exists(files ...string) func(string) bool {
	return func(rel string) bool {
		for _, f := range files {
			if f == rel {
				return true
			}
		}
		return false
	}
}

func TestParseAndNormalizeRealReports(t *testing.T) {
	const (
		ex     = "2:0 4:1 8:0 9:0"
		cs     = "6:1 7:1 8:1 9:1 11:0 12:1 15:0 16:0 17:0"
		py     = "1:1 2:1 3:1 4:0 6:1 7:0 8:0"
		erl    = "5:0 7:1 10:0 11:0"
		java   = "3:0 5:2 6:3 8:0 12:0"
		kotlin = "5:2 6:2 8:0 12:0"
	)
	jvmRoots := []string{"src/main/java", "src/main/kotlin"}
	tests := []struct {
		name   string
		format string
		files  []string
		n      Normalizer
		want   map[string]string
	}{
		{"excoveralls absolute SF", "lcov", []string{"lcov/excoveralls.info"},
			Normalizer{PackRoot: "/tmp/hello"}, map[string]string{"lib/hello.ex": ex}},
		{"excoveralls outside the pack root is dropped", "lcov", []string{"lcov/excoveralls.info"},
			Normalizer{PackRoot: "/tmp/other"}, map[string]string{}},
		{"simplecov-lcov ./ prefix", "lcov", []string{"lcov/simplecov.lcov"},
			Normalizer{PackRoot: "/w/hello"}, map[string]string{"lib/hello.rb": "1:1 2:1 3:1 4:1 6:0 10:1 11:0"}},
		{"cloverage relative SF", "lcov", []string{"lcov/cloverage.info"},
			Normalizer{PackRoot: "/w/hello"}, map[string]string{"src/hello/core.clj": "1:1 3:1 4:2 5:4 8:1 9:0"}},
		{"coverage.py lcov with FN and BRDA", "lcov", []string{"lcov/coveragepy.info"},
			Normalizer{PackRoot: "/w/hello"}, map[string]string{"src/hello/__init__.py": py}},
		{"coverage.py XML resolves <sources>", "cobertura", []string{"cobertura/coveragepy.xml"},
			Normalizer{PackRoot: "/w/hello"}, map[string]string{"src/hello/__init__.py": py}},
		{"coverlet VSTest with an absolute / source", "cobertura", []string{"cobertura/coverlet-vstest.xml"},
			Normalizer{PackRoot: "/src/hello"}, map[string]string{"src/Hello/Greeter.cs": cs}},
		{"MTP with BOM and absolute filename", "cobertura", []string{"cobertura/mtp.xml"},
			Normalizer{PackRoot: "/repo"}, map[string]string{"src/Hello/Greeter.cs": cs}},
		{"a report read twice merges", "cobertura", []string{"cobertura/mtp.xml", "cobertura/mtp.xml"},
			Normalizer{PackRoot: "/repo/"}, map[string]string{"src/Hello/Greeter.cs": cs}},
		{"covertool one file per umbrella app", "cobertura", []string{"cobertura/covertool-a1.xml", "cobertura/covertool-a2.xml"},
			Normalizer{PackRoot: "/w/um"}, map[string]string{"apps/a1/src/a1_lib.erl": erl, "apps/a2/src/a2_lib.erl": erl}},
		{"scoverage skips the bogus relative source", "cobertura", []string{"cobertura/scoverage.xml"},
			Normalizer{PackRoot: "/w/hello"}, map[string]string{"src/main/scala/hello/Hello.scala": "4:1 6:1 8:0 10:0"}},
		{"scoverage with Exists", "cobertura", []string{"cobertura/scoverage.xml"},
			Normalizer{PackRoot: "/w/hello", Exists: exists("src/main/scala/hello/Hello.scala")},
			map[string]string{"src/main/scala/hello/Hello.scala": "4:1 6:1 8:0 10:0"}},
		{"PHPUnit cobertura reads class lines only", "cobertura", []string{"cobertura/phpunit.xml"},
			Normalizer{PackRoot: "/app"}, map[string]string{"src/Hello.php": "8:1 9:1 11:0 16:0"}},
		{"JaCoCo Maven picks the existing source root", "jacoco", []string{"jacoco/maven.xml"},
			Normalizer{PackRoot: "/w/hello", SourceRoots: []string{"src/main/kotlin", "src/main/java"}, Exists: exists("src/main/java/com/example/hello/Hello.java")},
			map[string]string{"src/main/java/com/example/hello/Hello.java": java}},
		{"Kover XML", "jacoco", []string{"jacoco/kover.xml"},
			Normalizer{PackRoot: "/w/hello", SourceRoots: jvmRoots, Exists: exists("src/main/kotlin/com/example/hello/Hello.kt")},
			map[string]string{"src/main/kotlin/com/example/hello/Hello.kt": kotlin}},
		{"Kover XML without a match keeps the first source root", "jacoco", []string{"jacoco/kover.xml"},
			Normalizer{PackRoot: "/w/hello", SourceRoots: jvmRoots, Exists: exists()},
			map[string]string{"src/main/java/com/example/hello/Hello.kt": kotlin}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ps []Profile
			for _, f := range tt.files {
				p, err := Parse(tt.format, readFixture(t, f))
				if err != nil {
					t.Fatalf("%s: %v", f, err)
				}
				if p.Format != tt.format {
					t.Fatalf("format %q", p.Format)
				}
				ps = append(ps, Normalize(p, tt.n))
			}
			got := MergeProfiles(ps...)
			if !got.Normalized {
				t.Fatal("merged normalized profiles must stay normalized")
			}
			if g := lines(got); !reflect.DeepEqual(g, tt.want) {
				t.Fatalf("got %q\nwant %q", g, tt.want)
			}
		})
	}
}

func TestParseMergedBeforeNormalize(t *testing.T) {
	// Merging raw reports first keeps every <sources> root, so Normalize
	// can still resolve each file.
	a, err := Parse("cobertura", readFixture(t, "cobertura/coveragepy.xml"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Parse("cobertura", readFixture(t, "cobertura/scoverage.xml"))
	if err != nil {
		t.Fatal(err)
	}
	m := MergeProfiles(a, b)
	if m.Format != "cobertura" || len(m.Sources) != 3 || m.Normalized {
		t.Fatalf("%+v", m)
	}
	got := Normalize(m, Normalizer{PackRoot: "/w/hello", Exists: exists("src/hello/__init__.py", "src/main/scala/hello/Hello.scala")})
	keys := make([]string, 0, len(got.Files))
	for k := range got.Files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if want := []string{"src/hello/__init__.py", "src/main/scala/hello/Hello.scala"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("%v", keys)
	}
}

func TestEvaluateLineBasedReport(t *testing.T) {
	p, err := Parse("cobertura", readFixture(t, "cobertura/mtp.xml"))
	if err != nil {
		t.Fatal(err)
	}
	p = Normalize(p, Normalizer{PackRoot: "/repo"})
	const f = "src/Hello/Greeter.cs"
	o := Options{SourceGlobs: []string{"**/*.cs"}, TestGlobs: []string{"tests/**"}, BaseExclude: []string{}, Module: "ignored/after/normalize"}
	// Lines 6-12 hold 6 instrumented lines (11 uncovered); line 8 sits in
	// two overlapping ranges and still counts once.
	r := Evaluate(p, Changes{f: {Ranges: []LineRange{{5, 8}, {8, 12}}}}, nil, o)
	if r.Stmts != 6 || r.Covered != 5 || !r.OK || len(r.Rows) != 1 || r.Rows[0].Line != 6 {
		t.Fatalf("%+v", r)
	}
	if !reflect.DeepEqual(r.Rows[0].Uncovered, []LineRange{{11, 11}}) {
		t.Fatalf("uncovered %v", r.Rows[0].Uncovered)
	}
	// The unused function alone: untested file, and no package finding
	// unless PackageTests is set.
	r = Evaluate(p, Changes{f: {Ranges: []LineRange{{14, 18}}}}, nil, o)
	if got := findingMsgs(r); !reflect.DeepEqual(got, []string{
		"src/Hello/Greeter.cs:15: new code in Greeter.cs is untested (0/3 statements)",
		"new code coverage 0.0% < 70%",
	}) {
		t.Fatalf("%q", got)
	}
	o.PackageTests = true
	r = Evaluate(p, Changes{f: {Ranges: []LineRange{{14, 18}}}}, map[string]bool{}, o)
	if len(r.Findings) != 3 || r.Findings[0].Msg != "package src/Hello has no tests" {
		t.Fatalf("%q", findingMsgs(r))
	}
	if wc := WholeChanges(p, "ignored"); len(wc) != 1 || wc[f].Ranges == nil {
		t.Fatalf("%+v", wc)
	}
}
