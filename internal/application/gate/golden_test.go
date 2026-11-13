package gate

import (
	"context"
	"strings"
	"testing"
)

// legacyArgv is the pre-pack gate's runCheck switch, verbatim: the oracle
// the Go pack must reproduce.
func legacyArgv(name string, optPkgs []string, coverProfile string) []string {
	pkgs := optPkgs
	if len(pkgs) == 0 {
		pkgs = []string{"./..."}
	}
	switch name {
	case CheckBuild:
		return append([]string{"go", "build"}, pkgs...)
	case CheckGofmt:
		return append([]string{"gofmt", "-l"}, fmtDirs(optPkgs)...)
	case CheckVet:
		return append([]string{"go", "vet"}, pkgs...)
	case CheckStaticcheck:
		return append([]string{"staticcheck"}, pkgs...)
	case CheckTest:
		args := []string{"go", "test", "-race", "-count=1"}
		if coverProfile != "" {
			args = append(args, "-covermode=atomic", "-coverprofile="+coverProfile)
		}
		return append(args, pkgs...)
	case CheckVuln:
		return append([]string{"govulncheck"}, pkgs...)
	}
	return nil
}

func TestGoldenGoArgv(t *testing.T) {
	p := goPack()
	var names []string
	for _, c := range p.Checks {
		names = append(names, c.Name)
	}
	if strings.Join(names, ",") != strings.Join(CheckNames, ",") {
		t.Fatalf("pack order %v, compat %v", names, CheckNames)
	}
	var req []string
	for _, c := range p.Checks {
		if c.Required {
			req = append(req, c.Name)
		}
	}
	if strings.Join(req, ",") != strings.Join(RequiredChecks, ",") {
		t.Fatalf("required %v, compat %v", req, RequiredChecks)
	}
	for _, tc := range []struct {
		pkgs []string
		prof string
	}{
		{nil, "/tmp/cover.out"},
		{nil, ""},
		{[]string{"./internal/domain/..."}, "/tmp/c.out"},
		{[]string{"./a/...", "b", "./..."}, ""},
		{[]string{"example.com/m/x"}, "/tmp/p q.out"},
	} {
		s := New(&fakeCmd{}, fakeArch{ok: true}, &fakeRec{}, &tick{})
		pr := s.newPackRun(goOpts(Options{Root: "/p", Pkgs: tc.pkgs, CoverProfile: tc.prof}), goOpts(Options{}).Packs[0], 0, (&tick{}).Now())
		for _, c := range p.Checks {
			if c.Internal != "" || c.Name == CheckGosec {
				continue // gosec is new: no legacy oracle (asserted in security_test.go)
			}
			got, err := s.argvFor(context.Background(), pr, c, c.Argv, "")
			if err != nil {
				t.Fatal(c.Name, err)
			}
			want := legacyArgv(c.Name, tc.pkgs, tc.prof)
			if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
				t.Errorf("%s %v %q:\n got %q\nwant %q", c.Name, tc.pkgs, tc.prof, got, want)
			}
		}
	}
}

// TestGoldenGoPipeline runs the whole Go pipeline with every tool present
// and checks the exact command sequence of the pre-pack gate.
func TestGoldenGoPipeline(t *testing.T) {
	f := &fakeCmd{has: map[string]bool{"staticcheck": true, "govulncheck": true}}
	s, rec := setup(f, fakeArch{ok: true})
	res, err := s.Run(context.Background(), opts(Options{Root: "/p", Vuln: true}))
	if err != nil || !res.OK {
		t.Fatal(names(res), err)
	}
	var want []string
	for _, n := range []string{CheckBuild, CheckGofmt, CheckVet, CheckStaticcheck, CheckTest, CheckVuln} {
		want = append(want, strings.Join(legacyArgv(n, nil, "/tmp/cover.out"), " "))
	}
	if strings.Join(f.calls, "\n") != strings.Join(want, "\n") {
		t.Fatalf("calls\n%s\nwant\n%s", strings.Join(f.calls, "\n"), strings.Join(want, "\n"))
	}
	for _, e := range rec.evs {
		if e.Pack != "" {
			t.Fatalf("single pack event carries a pack: %+v", e)
		}
	}
}

func TestGoMissingToolsAndVuln(t *testing.T) {
	// staticcheck missing: skipped with its install hint, nothing recorded.
	f := &fakeCmd{}
	s, rec := setup(f, fakeArch{ok: true})
	res, _ := s.Run(context.Background(), opts(Options{}))
	if c := res.Checks[3]; !c.Skipped || !strings.Contains(c.Output, "go install honnef.co/go/tools/cmd/staticcheck@v0.8.2") {
		t.Fatalf("%+v", c)
	}
	if !res.OK || names(res) != "build:ok gofmt:ok vet:ok staticcheck:skip arch:ok test:ok coverage:ok govulncheck:skip gosec:skip" {
		t.Fatal(names(res))
	}
	for _, e := range rec.evs {
		if e.Name == "staticcheck" || e.Name == "govulncheck" {
			t.Fatalf("recorded %+v", e)
		}
	}
	// --vuln without govulncheck: an enabled check whose tool is missing fails (D16).
	s, _ = setup(&fakeCmd{}, fakeArch{ok: true})
	res, _ = s.Run(context.Background(), opts(Options{Vuln: true}))
	if c := res.Checks[7]; c.OK || c.Skipped || !strings.Contains(c.Output, "govulncheck not installed; install: go install golang.org/x/vuln/cmd/govulncheck@v1.9.0") {
		t.Fatalf("%+v", c)
	}
	// --enable turns a default-off check on by name.
	f = &fakeCmd{has: map[string]bool{"govulncheck": true}}
	s, _ = setup(f, fakeArch{ok: true})
	_, _ = s.Run(context.Background(), opts(Options{Enable: []string{"govulncheck"}}))
	if last := f.calls[len(f.calls)-1]; last != "govulncheck ./..." {
		t.Fatal(f.calls)
	}
	// A missing required tool fails with a clear message, not an exec error.
	s, _ = setup(&fakeCmd{has: map[string]bool{"go": false}}, fakeArch{ok: true})
	res, _ = s.Run(context.Background(), opts(Options{}))
	if c := res.Checks[0]; c.OK || c.Output != "go not installed" {
		t.Fatalf("%+v", c)
	}
	// The race check (test) can't run: it fails as unavailable, never skips (D24).
	if c := res.Checks[5]; c.OK || c.Skipped || !strings.HasPrefix(c.Output, "race policy unavailable: go not installed") {
		t.Fatalf("%+v", c)
	}
}

func TestGoRaceHostFailure(t *testing.T) {
	f := &fakeCmd{fail: map[string]string{"go test": "go: -race requires cgo; enable cgo by setting CGO_ENABLED=1\n"}}
	s, rec := setup(f, fakeArch{ok: true})
	res, _ := s.Run(context.Background(), opts(Options{}))
	c := res.Checks[5]
	if c.OK || !strings.HasPrefix(c.Output, "race policy unavailable: ") || !strings.Contains(c.Output, "requires cgo") {
		t.Fatalf("%+v", c)
	}
	for _, e := range rec.evs {
		if e.Name == "test" && !strings.HasPrefix(e.Note, "race policy unavailable") {
			t.Fatalf("%+v", e)
		}
	}
}

func TestNoPack(t *testing.T) {
	s, rec := setup(&fakeCmd{}, fakeArch{ok: true})
	if _, err := s.Run(context.Background(), Options{}); err != ErrNoPack || !strings.Contains(err.Error(), "languages in .loops/config.json") {
		t.Fatal(err)
	}
	if len(rec.evs) != 0 {
		t.Fatal(rec.evs)
	}
}
