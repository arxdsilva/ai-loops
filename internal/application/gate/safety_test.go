package gate

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/application/langpack"
	"github.com/arxdsilva/ai-loops/internal/domain/pack"
)

func covPack(path, format string) pack.Pack {
	return pack.Pack{ID: "java", SourceGlobs: []string{"**/*.java"}, TestGlobs: []string{"**/*Test.java"},
		Checks: []pack.Check{
			{Name: "test", Kind: "test", Argv: []string{"mvn", "verify"}, Required: true},
			{Name: "coverage", Kind: "coverage", Internal: "coverage", Required: true},
		},
		Coverage: pack.Coverage{Format: format, Path: path, ProducedBy: []string{"test"}, SourceRoots: []string{"src/main/java"}}}
}

// Stale-report cleanup never deletes the project: empty placeholders, the
// pack root, the run dir and globs rooted at / are refused, and only files
// under the coverage dir, the run dir or below the pack root are removed.
func TestStaleCleanupContainment(t *testing.T) {
	project := map[string]string{"/p/main.py": "src", "/p/pkg/mod.py": "src", "/p/README.md": "doc", "/elsewhere/x.cobertura.xml": "x", "/r/events.jsonl": "{}"}
	for _, tc := range []struct {
		path string
		o    Options
		want string // the error reportPaths returns
	}{
		{"{coverage}", Options{}, `no coverprofile path`},
		{"{coverage_dir}/**/*cobertura.xml", Options{}, `no coverprofile path`},
		{"{coverage_dir}", Options{}, `no coverprofile path`},
		{"{tmp}/*.xml", Options{}, `{tmp} is empty`},
		{"{run}", Options{RunDir: "/r"}, `is the pack root or run dir`},
		{".", Options{}, `is the pack root or run dir`},
		{"./", Options{}, `is the pack root or run dir`},
		{"/", Options{}, `is rooted at /`},
		{"/**/*.xml", Options{}, `is rooted at /`},
		{"/*cobertura.xml", Options{}, `is rooted at /`},
	} {
		fs := newMemFS(clone(project))
		p := covPack(tc.path, "cobertura")
		s := New(&scriptCmd{}, fakeArch{ok: true}, &fakeRec{}, &tick{}).WithFiles(fs).WithCoverage(&packCov{files: fs.files}, CoverageConfig{})
		o := tc.o
		o.Root, o.Packs = "/p", []langpack.Detected{detected(p, ".")}
		res, _ := s.Run(context.Background(), o)
		if len(fs.removed) > 0 {
			t.Errorf("%s: removed %v", tc.path, fs.removed)
		}
		if c := byName(res, "coverage"); c.OK || !strings.Contains(c.Output, tc.want) {
			t.Errorf("%s: %q", tc.path, c.Output)
		}
	}
	// A report outside every allowed dir is read but never removed.
	fs := newMemFS(clone(project))
	pr := &packRun{p: covPack("/elsewhere/*.cobertura.xml", "cobertura"), root: "/p", covDir: "/cov", runDir: "/r", ran: map[string]bool{}, phs: map[string][]string{}}
	s := New(&scriptCmd{}, fakeArch{ok: true}, &fakeRec{}, &tick{}).WithFiles(fs)
	got, err := s.reportPaths(context.Background(), pr, fs)
	if err != nil || !slices.Equal(got, []string{"/elsewhere/x.cobertura.xml"}) {
		t.Fatalf("%v %v", got, err)
	}
	s.removeStale(context.Background(), pr)
	if len(fs.removed) > 0 {
		t.Fatalf("removed %v", fs.removed)
	}
	for p, want := range map[string]bool{"/cov/a.xml": true, "/r/x/a.xml": true, "/p/target/a.xml": true, "/p": false, "/cov": false, "/elsewhere/a.xml": false, "/pp/a.xml": false} {
		if pr.removable(p) != want {
			t.Errorf("removable(%s) = %v", p, !want)
		}
	}
	if (&packRun{covDir: "/"}).removable("/etc/passwd") {
		t.Error("a coverage dir at / makes everything removable")
	}
	if !(&packRun{covDir: "."}).removable("cover.out") {
		t.Error("relative coverage dir")
	}
	for in, want := range map[string]string{"/a/*.xml": "/a", "/**": "/", "/a/b/c.xml": "/a/b/c.xml", "C:/x/*.xml": "C:/x"} {
		if got := globBase(in); got != want {
			t.Errorf("globBase(%s) = %s, want %s", in, got, want)
		}
	}
	for in, want := range map[string]bool{"": true, ".": true, "/": true, "/a": false} {
		if isFSRoot(in) != want {
			t.Errorf("isFSRoot(%q)", in)
		}
	}
}

func clone(m map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range m {
		out[k] = v
	}
	return out
}

const jacocoModule = `<?xml version="1.0"?><report name="m"><package name="com/acme">
<sourcefile name="%s"><line nr="3" mi="2" ci="0" mb="0" cb="0"/><line nr="4" mi="1" ci="0" mb="0" cb="0"/></sourcefile>
</package></report>`

// realCov is a packCov whose RealPath maps /tmp to /private/tmp.
type realCov struct{ packCov }

func (r *realCov) RealPath(p string) (string, error) {
	if rest, ok := strings.CutPrefix(p, "/tmp/"); ok {
		return "/private/tmp/" + rest, nil
	}
	return p, nil
}

// Multi-module JaCoCo: each module's report resolves against its own
// source roots, so an untested module file fails instead of passing as
// "no new statements".
func TestCoverageMultiModuleJacoco(t *testing.T) {
	p := covPack("**/target/site/jacoco/jacoco.xml", "jacoco")
	fs := newMemFS(map[string]string{
		"/p/module-a/target/site/jacoco/jacoco.xml":   "stale",
		"/p/module-a/src/main/java/com/acme/Foo.java": "class Foo{}",
		"/p/module-b/src/main/java/com/acme/Bar.java": "class Bar{}",
	})
	cmd := &scriptCmd{fn: func(string, []string) (string, error) {
		fs.files["/p/module-a/target/site/jacoco/jacoco.xml"] = strings.Replace(jacocoModule, "%s", "Foo.java", 1)
		fs.files["/p/module-b/target/site/jacoco/jacoco.xml"] = strings.Replace(jacocoModule, "%s", "Bar.java", 1)
		return "", nil
	}}
	diff := "diff --git a/module-a/src/main/java/com/acme/Foo.java b/module-a/src/main/java/com/acme/Foo.java\n--- a/module-a/src/main/java/com/acme/Foo.java\n+++ b/module-a/src/main/java/com/acme/Foo.java\n@@ -2,0 +3,2 @@\n+a\n+b\n"
	cov := &packCov{fakeCov: fakeCov{diff: Diff{Base: "main", Text: diff}}, files: fs.files}
	res, err := New(cmd, fakeArch{ok: true}, &fakeRec{}, &tick{}).WithCoverage(cov, CoverageConfig{}).WithFiles(fs).
		Run(context.Background(), Options{Root: "/p", CoverDir: "/cov", Packs: []langpack.Detected{detected(p, ".")}})
	if err != nil {
		t.Fatal(err)
	}
	c := byName(res, "coverage")
	if c.OK || !strings.Contains(c.Output, "module-a/src/main/java/com/acme/Foo.java:3: new code in Foo.java is untested (0/2 statements)") {
		t.Fatalf("%+v", c)
	}
	// The stale report went before the test check ran.
	if !slices.Contains(fs.removed, "/p/module-a/target/site/jacoco/jacoco.xml") {
		t.Fatalf("removed %v", fs.removed)
	}
}

// A pack root reached through a symlink still matches the physical paths
// the tool wrote; a modified file missing from the report fails.
func TestCoverageSymlinkRootAndUnreported(t *testing.T) {
	p := covPack("coverage/lcov.info", "lcov")
	p.ID, p.SourceGlobs, p.TestGlobs = "python", []string{"**/*.py"}, []string{"**/test_*.py"}
	lcov := "SF:/private/tmp/repo/app/svc.py\nDA:1,0\nDA:2,0\nend_of_record\n"
	fs := newMemFS(nil)
	cmd := &scriptCmd{fn: func(string, []string) (string, error) {
		fs.files["/tmp/repo/coverage/lcov.info"] = lcov
		return "", nil
	}}
	diff := "diff --git a/app/svc.py b/app/svc.py\n--- a/app/svc.py\n+++ b/app/svc.py\n@@ -0,0 +1,2 @@\n+a\n+b\n" +
		"diff --git a/app/other.py b/app/other.py\n--- a/app/other.py\n+++ b/app/other.py\n@@ -4,0 +5 @@\n+c\n"
	cov := &realCov{packCov{fakeCov: fakeCov{diff: Diff{Base: "main", Text: diff}}, files: fs.files}}
	res, _ := New(cmd, fakeArch{ok: true}, &fakeRec{}, &tick{}).WithCoverage(cov, CoverageConfig{}).WithFiles(fs).
		Run(context.Background(), Options{Root: "/tmp/repo", CoverDir: "/cov", Packs: []langpack.Detected{detected(p, ".")}})
	c := byName(res, "coverage")
	if c.OK || !strings.Contains(c.Output, "app/svc.py:1: new code in svc.py is untested (0/2 statements)") ||
		!strings.Contains(c.Output, "app/other.py:5: changed file other.py is not in the coverage report") {
		t.Fatalf("%+v", c)
	}
	s := New(cmd, fakeArch{ok: true}, &fakeRec{}, &tick{})
	if s.realPath("/x") != "" || s.WithCoverage(cov, CoverageConfig{}).realPath("/x") != "" || s.realPath("") != "" {
		t.Fatal("realPath without a change")
	}
}
