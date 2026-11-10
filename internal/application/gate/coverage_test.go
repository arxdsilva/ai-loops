package gate

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

type fakeCov struct {
	profile   string
	profErr   error
	module    string
	modErr    error
	diff      Diff
	diffErr   error
	hasTests  map[string]bool
	generated map[string]bool

	gotBase, gotProfile string
	askedGen, askedPkg  []string
}

func (f *fakeCov) Profile(p string) (string, error) { f.gotProfile = p; return f.profile, f.profErr }
func (f *fakeCov) Module(string) (string, error)    { return f.module, f.modErr }
func (f *fakeCov) Diff(_ context.Context, _, base string) (Diff, error) {
	f.gotBase = base
	return f.diff, f.diffErr
}
func (f *fakeCov) HasTests(_, dir string) bool {
	f.askedPkg = append(f.askedPkg, dir)
	return f.hasTests[dir]
}
func (f *fakeCov) Generated(_, file string) bool {
	f.askedGen = append(f.askedGen, file)
	return f.generated[file]
}

const covProfile = `mode: atomic
example.com/m/internal/x/a.go:3.1,5.2 4 1
example.com/m/internal/x/a.go:7.1,9.2 1 0
example.com/m/internal/y/b.go:1.1,4.2 3 0
example.com/m/internal/x/gen.go:1.1,4.2 9 0
example.com/m/internal/z/new.go:1.1,4.2 2 1
`

const covDiff = `diff --git a/internal/x/a.go b/internal/x/a.go
--- a/internal/x/a.go
+++ b/internal/x/a.go
@@ -2,0 +3,7 @@
+a
+b
+c
+d
+e
+f
+g
`

func runCov(t *testing.T, cov *fakeCov, cfg CoverageConfig, o Options) (Result, CheckResult, *fakeRec) {
	t.Helper()
	r := &fakeRec{}
	s := New(&fakeCmd{}, fakeArch{ok: true}, r, &tick{}).WithCoverage(cov, cfg)
	res, err := s.Run(context.Background(), opts(o))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range res.Checks {
		if c.Name == CheckCoverage {
			return res, c, r
		}
	}
	t.Fatal("no coverage check")
	return res, CheckResult{}, r
}

func covEvent(r *fakeRec) run.Event {
	for _, e := range r.evs {
		if e.Name == CheckCoverage {
			return e
		}
	}
	return run.Event{}
}

func TestCoveragePasses(t *testing.T) {
	cov := &fakeCov{profile: covProfile, module: "example.com/m",
		diff:     Diff{Base: "abc123", Text: covDiff, Untracked: []string{"internal/z/new.go"}},
		hasTests: map[string]bool{"internal/x": true, "internal/z": true}}
	res, c, rec := runCov(t, cov, CoverageConfig{Base: "origin/dev"}, Options{})
	if !res.OK || !c.OK || c.Skipped {
		t.Fatalf("%+v", c)
	}
	// 4 covered + 1 uncovered in a.go, 2 covered in the untracked file: 6/7.
	if ev := covEvent(rec); ev.Note != "new code 85.7% (min 70%)" || !*ev.OK {
		t.Fatalf("%+v", ev)
	}
	for _, s := range []string{"base abc123", "internal/x/a.go", "internal/z/new.go", "total"} {
		if !strings.Contains(c.Output, s) {
			t.Fatalf("output lacks %q:\n%s", s, c.Output)
		}
	}
	if cov.gotBase != "origin/dev" || cov.gotProfile != "/tmp/cover.out" {
		t.Fatalf("base %q profile %q", cov.gotBase, cov.gotProfile)
	}
	if strings.Join(cov.askedPkg, ",") != "internal/x,internal/z" {
		t.Fatal(cov.askedPkg)
	}
}

func TestCoverageFails(t *testing.T) {
	diff := covDiff + "diff --git a/internal/y/b.go b/internal/y/b.go\n--- a/internal/y/b.go\n+++ b/internal/y/b.go\n@@ -1 +1,2 @@\n-a\n+b\n+c\n" +
		"diff --git a/internal/x/gen.go b/internal/x/gen.go\n--- a/internal/x/gen.go\n+++ b/internal/x/gen.go\n@@ -1 +1,2 @@\n-a\n+b\n+c\n"
	cov := &fakeCov{profile: covProfile, module: "example.com/m", diff: Diff{Base: "main", Text: diff},
		hasTests: map[string]bool{"internal/x": true}, generated: map[string]bool{"internal/x/gen.go": true}}
	res, c, rec := runCov(t, cov, CoverageConfig{}, Options{Base: "v1.0.0"})
	if res.OK || c.OK {
		t.Fatalf("%+v", c)
	}
	for _, s := range []string{
		"internal/y/b.go:1: package internal/y has no tests",
		"internal/y/b.go:1: new code in b.go is untested (0/3 statements)",
		"new code coverage 50.0% < 70%",
	} {
		if !strings.Contains(c.Output, s) {
			t.Fatalf("output lacks %q:\n%s", s, c.Output)
		}
	}
	if strings.Contains(c.Output, "gen.go") || cov.gotBase != "v1.0.0" {
		t.Fatalf("generated file judged or base %q:\n%s", cov.gotBase, c.Output)
	}
	if ev := covEvent(rec); ev.Note != "new code 50.0% (min 70%)" || *ev.OK {
		t.Fatalf("%+v", ev)
	}
}

func TestCoverageConfigMinAndExclude(t *testing.T) {
	cov := &fakeCov{profile: covProfile, module: "example.com/m", diff: Diff{Text: covDiff},
		hasTests: map[string]bool{"internal/x": true}}
	_, c, _ := runCov(t, cov, CoverageConfig{Min: 90}, Options{})
	if c.OK || !strings.Contains(c.Output, "new code coverage 80.0% < 90%") {
		t.Fatalf("%+v", c)
	}
	_, c, _ = runCov(t, cov, CoverageConfig{Min: 90, Exclude: []string{"internal/x/**"}}, Options{})
	if !c.OK || c.note != "no new statements" {
		t.Fatalf("%+v", c)
	}
}

func TestCoverageScopedAndNoGit(t *testing.T) {
	cov := &fakeCov{profile: covProfile, module: "example.com/m", diff: Diff{NoGit: true}}
	res, c, rec := runCov(t, cov, CoverageConfig{}, Options{Only: []string{"test", "coverage"}, Pkgs: []string{"./internal/z/..."}})
	if !res.OK || !c.OK || c.note != "no git base: whole-package coverage 100.0% (min 70%)" {
		t.Fatalf("%+v", c)
	}
	if !strings.Contains(c.Output, "no git base") {
		t.Fatal(c.Output)
	}
	for _, e := range rec.evs {
		if e.Name == run.GateAll {
			t.Fatal("scoped run recorded all")
		}
	}
	_, c, _ = runCov(t, cov, CoverageConfig{}, Options{Only: []string{"test", "coverage"}, Pkgs: []string{"./internal/y"}})
	if c.OK || !strings.Contains(c.Output, "package coverage 0.0% < 70%") {
		t.Fatalf("%+v", c)
	}
}

func TestCoverageErrors(t *testing.T) {
	tests := []struct {
		name string
		cov  *fakeCov
		o    Options
		want string
	}{
		{"test not run", &fakeCov{}, Options{Only: []string{"coverage"}}, "needs the test check"},
		{"profile error", &fakeCov{profErr: errors.New("gone")}, Options{}, "gone"},
		{"bad profile", &fakeCov{profile: "mode: set\nbad\n"}, Options{}, "coverprofile line 2"},
		{"module error", &fakeCov{modErr: errors.New("no go.mod")}, Options{}, "no go.mod"},
		{"diff error", &fakeCov{diffErr: errors.New("git broke")}, Options{}, "git broke"},
		{"bad diff", &fakeCov{diff: Diff{Text: "@@ nope\n"}}, Options{}, "diff line 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, c, rec := runCov(t, tt.cov, CoverageConfig{}, tt.o)
			if res.OK || c.OK || c.Skipped || !strings.Contains(c.Output, tt.want) {
				t.Fatalf("%+v", c)
			}
			if ev := covEvent(rec); ev.Name != CheckCoverage || *ev.OK || !strings.Contains(ev.Note, tt.want) {
				t.Fatalf("%+v", ev)
			}
		})
	}
	// No coverprofile path from the caller.
	s0 := New(&fakeCmd{}, fakeArch{ok: true}, &fakeRec{}, &tick{}).WithCoverage(&fakeCov{}, CoverageConfig{})
	if res, _ := s0.Run(context.Background(), goOpts(Options{})); res.OK || !strings.Contains(res.Checks[6].Output, "no coverprofile path") {
		t.Fatalf("%+v", res.Checks[6])
	}
	// No coverage port at all.
	s := New(&fakeCmd{}, fakeArch{ok: true}, &fakeRec{}, &tick{})
	res, _ := s.Run(context.Background(), opts(Options{}))
	if res.OK || !strings.Contains(res.Checks[6].Output, "not configured") {
		t.Fatalf("%+v", res.Checks[6])
	}
}

func TestTestCommandWithoutProfile(t *testing.T) {
	f := &fakeCmd{}
	s := New(f, fakeArch{ok: true}, &fakeRec{}, &tick{}).WithCoverage(&fakeCov{}, CoverageConfig{})
	_, _ = s.Run(context.Background(), goOpts(Options{Only: []string{"test"}}))
	if len(f.calls) != 1 || f.calls[0] != "go test -race -count=1 ./..." {
		t.Fatal(f.calls)
	}
}
