package gate

import (
	"context"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/application/langpack"
	"github.com/arxdsilva/ai-loops/internal/domain/pack"
)

func twoPacks() []langpack.Detected {
	mk := func(id string) langpack.Detected {
		return langpack.Detected{ID: id, Name: id, Root: ".", Pack: pack.Pack{
			ID:     id,
			Checks: []pack.Check{{Name: "x", Kind: "lint", Argv: []string{id + "x"}, Tool: id + "x"}},
		}}
	}
	return []langpack.Detected{mk("go"), mk("other")}
}

func TestPackChecksAlone(t *testing.T) {
	f := &fakeCmd{has: map[string]bool{"gox": true, "otherx": true}}
	s, rec := setup(f, fakeArch{ok: true})
	res, err := s.Run(context.Background(), Options{Packs: twoPacks(), PackChecks: map[string][]string{"go": {"x"}}})
	if err != nil || !res.OK {
		t.Fatal(res, err)
	}
	if strings.Join(f.calls, ",") != "gox " {
		t.Fatalf("calls %q", f.calls)
	}
	var sawX bool
	for _, e := range rec.evs {
		if e.Name == "all" {
			t.Fatalf("scoped run recorded all: %+v", e)
		}
		sawX = sawX || e.Name == "x"
	}
	if !sawX {
		t.Fatalf("no per-check event: %+v", rec.evs)
	}
	// nil runs everything, with an all event.
	f = &fakeCmd{has: map[string]bool{"gox": true, "otherx": true}}
	s, rec = setup(f, fakeArch{ok: true})
	if _, err := s.Run(context.Background(), Options{Packs: twoPacks()}); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 2 || rec.evs[len(rec.evs)-1].Name != "all" {
		t.Fatalf("calls %q evs %+v", f.calls, rec.evs)
	}
}

func TestMissingAlone(t *testing.T) {
	// Default: enabled default-off check with a missing tool fails.
	s, _ := setup(&fakeCmd{}, fakeArch{ok: true})
	res, _ := s.Run(context.Background(), opts(Options{Only: []string{"govulncheck", "test"}, Enable: []string{"govulncheck"}}))
	if c := res.Checks[7]; c.OK || c.Skipped {
		t.Fatalf("default: %+v", c)
	}
	// MissingSkip: skipped with the hint; the race check still fails.
	f := &fakeCmd{has: map[string]bool{"go": false, "govulncheck": false}}
	s, _ = setup(f, fakeArch{ok: true})
	res, _ = s.Run(context.Background(), opts(Options{Only: []string{"build", "govulncheck", "test"}, Enable: []string{"govulncheck"}, Missing: MissingSkip}))
	if c := res.Checks[0]; !c.Skipped || !strings.Contains(c.Output, "go not installed") {
		t.Fatalf("build: %+v", c)
	}
	if c := res.Checks[7]; !c.Skipped || !strings.Contains(c.Output, "govulncheck@v1.9.0") {
		t.Fatalf("vuln: %+v", c)
	}
	if c := res.Checks[5]; c.OK || c.Skipped || !strings.HasPrefix(c.Output, raceUnavailable) {
		t.Fatalf("race: %+v", c)
	}
}

func TestNoRecordAlone(t *testing.T) {
	s, rec := setup(&fakeCmd{}, fakeArch{ok: true})
	a, _ := s.Run(context.Background(), opts(Options{}))
	if len(rec.evs) == 0 {
		t.Fatal("baseline records")
	}
	s, rec = setup(&fakeCmd{}, fakeArch{ok: true})
	b, _ := s.Run(context.Background(), opts(Options{NoRecord: true}))
	if len(rec.evs) != 0 || names(a) != names(b) || a.OK != b.OK {
		t.Fatalf("evs %+v\n%s\n%s", rec.evs, names(a), names(b))
	}
}

func secOpts(t *testing.T, o Options) Options {
	t.Helper()
	p := goPack()
	if len(p.SecurityChecks()) != 2 {
		t.Skip("pack.SecurityChecks not implemented yet (W1-pack)")
	}
	return opts(o)
}

func TestSecurityPassGo(t *testing.T) {
	f := &fakeCmd{has: map[string]bool{"govulncheck": true, "gosec": true}}
	s, rec := setup(f, fakeArch{ok: true})
	so, notes := SecurityPass(secOpts(t, Options{Root: "/p"}))
	if len(notes) != 0 {
		t.Fatal(notes)
	}
	res, err := s.Run(context.Background(), so)
	if err != nil || !res.OK {
		t.Fatal(names(res), err)
	}
	if got := strings.Join(f.calls, "\n"); got != "govulncheck ./...\ngosec -quiet -exclude-generated ./..." {
		t.Fatalf("calls %q", got)
	}
	if len(rec.evs) != 0 {
		t.Fatal(rec.evs)
	}

	// No tools: OK, both skipped with pinned hints.
	s, _ = setup(&fakeCmd{}, fakeArch{ok: true})
	res, _ = s.Run(context.Background(), so)
	if !res.OK {
		t.Fatal(names(res))
	}
	var n int
	for _, c := range res.Checks {
		switch c.Name {
		case "govulncheck":
			n++
			if !c.Skipped || !strings.Contains(c.Output, "@v1.9.0") {
				t.Fatalf("%+v", c)
			}
		case "gosec":
			n++
			if !c.Skipped || !strings.Contains(c.Output, "@v2.30.0") {
				t.Fatalf("%+v", c)
			}
		default:
			if !c.Skipped {
				t.Fatalf("ran %+v", c)
			}
		}
	}
	if n != 2 {
		t.Fatal(names(res))
	}

	// gosec exiting 1 fails; skip_on output skips.
	f = &fakeCmd{has: map[string]bool{"gosec": true}, fail: map[string]string{"gosec": "[G101] hardcoded credential\n"}}
	s, _ = setup(f, fakeArch{ok: true})
	if res, _ = s.Run(context.Background(), so); res.OK {
		t.Fatal(names(res))
	}
	f = &fakeCmd{has: map[string]bool{"gosec": true}, fail: map[string]string{"gosec": "internal error: package without types\n"}}
	s, _ = setup(f, fakeArch{ok: true})
	if res, _ = s.Run(context.Background(), so); !res.OK {
		t.Fatal(names(res))
	}
}

func TestSecurityPassNotesAndPurity(t *testing.T) {
	ps := twoPacks()
	ps[0].Pack.Security = &pack.Security{None: "static site"}
	ps = append(ps, ps[1], ps[1])
	in := Options{Packs: ps, Enable: []string{"keep"}}
	so, notes := SecurityPass(in)
	if len(notes) != 2 || notes[0] != "gate: go: no security tools (static site)" || notes[1] != "gate: other: security not declared" {
		t.Fatalf("%q", notes)
	}
	if in.PackChecks != nil || in.Missing != MissingDefault || in.NoRecord || len(in.Enable) != 1 {
		t.Fatalf("input mutated: %+v", in)
	}
	if !so.NoRecord || so.Missing != MissingSkip || len(so.PackChecks["go"]) != 0 || len(so.PackChecks) != 2 {
		t.Fatalf("%+v", so)
	}
	f := &fakeCmd{has: map[string]bool{"gox": true, "otherx": true}}
	s, _ := setup(f, fakeArch{ok: true})
	if _, err := s.Run(context.Background(), so); err != nil || len(f.calls) != 0 {
		t.Fatal(f.calls, err)
	}
}

func TestPlainRunNeverGosec(t *testing.T) {
	f := &fakeCmd{has: map[string]bool{"staticcheck": true, "govulncheck": true, "gosec": true}}
	s, _ := setup(f, fakeArch{ok: true})
	res, _ := s.Run(context.Background(), opts(Options{Vuln: true}))
	if !res.OK {
		t.Fatal(names(res))
	}
	for _, c := range f.calls {
		if strings.HasPrefix(c, "gosec") {
			t.Fatal(f.calls)
		}
	}
	if !strings.HasSuffix(names(res), "govulncheck:ok gosec:skip") {
		t.Fatal(names(res))
	}
}

func TestMissingEnvUnderMissingSkip(t *testing.T) {
	ps := twoPacks()[:1]
	ps[0].Pack.Checks[0].Required = true
	ps[0].Pack.Checks[0].EnvRequired = []string{"LOOPS_TEST_TOKEN_UNSET"}
	for _, tc := range []struct {
		m    MissingPolicy
		skip bool
	}{{MissingDefault, false}, {MissingSkip, true}} {
		f := &fakeCmd{has: map[string]bool{"gox": true}}
		s, _ := setup(f, fakeArch{ok: true})
		res, _ := s.Run(context.Background(), Options{Packs: ps, Missing: tc.m})
		c := res.Checks[0]
		if c.Skipped != tc.skip || c.OK != tc.skip && !c.Skipped && c.OK || !strings.Contains(c.Output, "missing env LOOPS_TEST_TOKEN_UNSET") || len(f.calls) != 0 {
			t.Fatalf("%v: %+v calls %q", tc.m, c, f.calls)
		}
	}
}

func TestCheckWithoutArgvFails(t *testing.T) {
	ps := twoPacks()[:1]
	ps[0].Pack.Checks[0].Argv = nil
	s, _ := setup(&fakeCmd{}, fakeArch{ok: true})
	res, _ := s.Run(context.Background(), Options{Packs: ps})
	if c := res.Checks[0]; c.OK || c.Skipped || c.Output != "no argv" {
		t.Fatalf("%+v", c)
	}
}
