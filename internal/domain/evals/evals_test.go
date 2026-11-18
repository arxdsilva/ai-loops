package evals

import (
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

func ok(b bool) *bool { return run.Bool(b) }

func step(agent, role, tool, target string, success bool) run.Event {
	return run.Event{Kind: run.KindStep, Agent: agent, Role: role, Name: tool, Target: target, OK: ok(success)}
}

func verdict(name, status string, round int, ref string) run.Event {
	return run.Event{Kind: run.KindVerdict, Name: name, Status: status, Round: round, Ref: ref}
}

func gate(name string, success bool) run.Event {
	return run.Event{Kind: run.KindGate, Name: name, OK: ok(success)}
}

func end(status string) run.Event { return run.Event{Kind: run.KindEnd, Status: status} }

func check(t *testing.T, a Assertion, ev Evidence) Check {
	t.Helper()
	r := Evaluate(Scenario{Name: "s", Assertions: []Assertion{a}}, ev)
	if len(r.Checks) != 1 || r.OK != r.Checks[0].OK {
		t.Fatalf("result = %+v", r)
	}
	return r.Checks[0]
}

func expect(t *testing.T, c Check, want bool, detail string) {
	t.Helper()
	if c.OK != want || !strings.Contains(c.Detail, detail) {
		t.Fatalf("check = %+v, want ok=%v with %q", c, want, detail)
	}
}

func TestValidate(t *testing.T) {
	for name, tc := range map[string]struct {
		s    Scenario
		want string
	}{
		"ok":            {Scenario{Name: "x", Assertions: []Assertion{{Kind: KindScope}}}, ""},
		"no name":       {Scenario{Assertions: []Assertion{{Kind: KindScope}}}, "no name"},
		"no assertions": {Scenario{Name: "x"}, "no assertions"},
		"unknown kind":  {Scenario{Name: "x", Assertions: []Assertion{{Kind: "scop"}}}, `unknown kind "scop"`},
		"test_first":    {Scenario{Name: "x", Assertions: []Assertion{{Kind: KindTestFirst, Test: "a_test.go"}}}, "needs test and impl"},
	} {
		t.Run(name, func(t *testing.T) {
			err := tc.s.Validate()
			if (tc.want == "") != (err == nil) || (err != nil && !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestEvaluate_EmptyLogFailsEveryAssertion(t *testing.T) {
	s := Scenario{Name: "s", Assertions: []Assertion{{Kind: KindGateHolds}, {Kind: KindScope}}}
	r := Evaluate(s, Evidence{})
	if r.OK || len(r.Checks) != 2 || r.Checks[0].Detail != "no events recorded" || r.Checks[1].OK {
		t.Fatalf("result = %+v", r)
	}
}

func TestEvaluate_UnknownKindFails(t *testing.T) {
	c := check(t, Assertion{Kind: "bogus"}, Evidence{Events: []run.Event{end("done")}})
	expect(t, c, false, "unknown")
}

func TestIsClaim(t *testing.T) {
	for name, want := range map[string]bool{
		"W1-domain:C1": true, "qa-all:C12": true,
		"quality:Q1": false, "security:S1": false, "quality:C1": false,
		"AC-1": false, "plan": false, ":C1": false, "W1:C": false, "W1:Cx": false,
	} {
		if got := isClaim(name); got != want {
			t.Errorf("isClaim(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestQAFailsClaim(t *testing.T) {
	refs := map[string]string{"qa/a.r1.md": "Repro: `go test`", "qa/b.r1.md": "no steps"}
	a := Assertion{Kind: KindQAFailsClaim, Marker: "Repro:"}
	for name, tc := range map[string]struct {
		a      Assertion
		events []run.Event
		want   bool
		detail string
	}{
		"failed with repro":      {a, []run.Event{verdict("W1-a:C1", run.StatusFailed, 1, "qa/a.r1.md")}, true, "W1-a:C1 FAILED in round 1"},
		"round 0 is round 1":     {a, []run.Event{verdict("W1-a:C1", run.StatusFailed, 0, "qa/a.r1.md")}, true, "round 1"},
		"only verified":          {a, []run.Event{verdict("W1-a:C1", run.StatusVerified, 1, "qa/a.r1.md")}, false, "no claim verdict FAILED"},
		"quality is not a claim": {a, []run.Event{verdict("quality:Q1", run.StatusFailed, 1, "qa/a.r1.md")}, false, "no claim verdict FAILED"},
		"too late":               {a, []run.Event{verdict("W1-a:C1", run.StatusFailed, 2, "qa/a.r1.md")}, false, "only after round 1"},
		"later round allowed":    {Assertion{Kind: KindQAFailsClaim, MaxRound: 2}, []run.Event{verdict("W1-a:C1", run.StatusFailed, 2, "qa/x.md")}, true, "round 2"},
		"no repro in file":       {a, []run.Event{verdict("W1-a:C1", run.StatusFailed, 1, "qa/b.r1.md")}, false, "without a repro"},
		"unreadable ref":         {a, []run.Event{verdict("W1-a:C1", run.StatusFailed, 1, "qa/missing.md")}, false, "without a repro"},
		"no ref":                 {Assertion{Kind: KindQAFailsClaim}, []run.Event{verdict("W1-a:C1", run.StatusFailed, 1, "")}, false, "without a repro"},
		"other claim":            {Assertion{Kind: KindQAFailsClaim, Claim: "*:C2"}, []run.Event{verdict("W1-a:C1", run.StatusFailed, 1, "qa/a.r1.md")}, false, "no claim verdict FAILED"},
		"matching claim":         {Assertion{Kind: KindQAFailsClaim, Claim: "*:C1"}, []run.Event{verdict("W1-a:C1", run.StatusFailed, 1, "qa/a.r1.md")}, true, "FAILED"},
	} {
		t.Run(name, func(t *testing.T) {
			expect(t, check(t, tc.a, Evidence{Events: tc.events, Refs: refs}), tc.want, tc.detail)
		})
	}
}

func lane(agent, role, id string) []run.Event {
	return []run.Event{
		{Kind: run.KindAgentStart, Agent: agent, Role: role},
		{Kind: run.KindLabel, Agent: agent, Role: role, Name: id},
	}
}

func stop(agent string) run.Event { return run.Event{Kind: run.KindAgentStop, Agent: agent} }

func TestScope(t *testing.T) {
	lanes := []Lane{{ID: "W1-d", Role: "domain-engineer", Owns: []string{"internal/domain/**"}}, {ID: "W1-h", Role: "api-engineer", Owns: []string{"internal/adapters/inbound/**"}}}
	start := append(lane("d", "domain-engineer", "W1-d"), lane("h", "api-engineer", "W1-h")...)
	both := []run.Event{
		step("d", "domain-engineer", "Write", "internal/domain/x.go", true),
		step("h", "api-engineer", "Edit", "internal/adapters/inbound/h.go", true),
	}
	join := func(parts ...[]run.Event) []run.Event {
		var out []run.Event
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}
	a := Assertion{Kind: KindScope, Parallel: true}
	for name, tc := range map[string]struct {
		a      Assertion
		events []run.Event
		want   bool
		detail string
	}{
		"inside scope, parallel": {a, join(start, both, []run.Event{stop("d"), stop("h")}), true, "2 lanes edited"},
		"outside scope":          {a, join(start, both, []run.Event{step("h", "api-engineer", "Edit", "internal/domain/x.go", true)}), false, "W1-h (api-engineer) wrote internal/domain/x.go outside internal/adapters/inbound/**"},
		"run files, scratch and lead are free": {a, join(start, both, []run.Event{
			step("h", "api-engineer", "Write", ".loops/runs/r/reports/W1-h.md", true),
			step("h", "api-engineer", "Write", "/tmp/scratch/x.go", true),
			step("", "", "Edit", "README.md", true),
			step("h", "api-engineer", "Read", "internal/domain/x.go", true),
			step("h", "api-engineer", "Edit", "internal/domain/x.go", false),
			step("q", "qa-verifier", "Write", "internal/domain/x.go", true),
		}), true, "2 lanes"},
		"allow list":          {Assertion{Kind: KindScope, Allow: []string{"docs/**"}}, join(start, both, []run.Event{step("h", "api-engineer", "Write", "docs/api.md", true)}), true, "2 lanes"},
		"one lane":            {a, join(start, both[:1]), false, "1 engineer lane(s) edited files, want 2"},
		"min lanes 1":         {Assertion{Kind: KindScope, MinLanes: 1}, join(start, both[:1]), true, "1 lanes"},
		"not parallel":        {a, join(lane("d", "domain-engineer", "W1-d"), both[:1], []run.Event{stop("d")}, lane("h", "api-engineer", "W1-h"), both[1:], []run.Event{stop("h")}), false, "never ran at the same time"},
		"no stop overlaps":    {a, join(lane("d", "domain-engineer", "W1-d"), both[:1], lane("h", "api-engineer", "W1-h"), both[1:]), true, "2 lanes"},
		"unlabelled engineer": {a, join(start, both, []run.Event{step("z", "persistence-engineer", "Write", "migrations/1.sql", true)}), false, "z (persistence-engineer) has no assigned scope"},
		"role fallback":       {Assertion{Kind: KindScope, Roles: map[string][]string{"persistence-engineer": {"migrations/**"}}}, join(start, both, []run.Event{step("z", "persistence-engineer", "Write", "migrations/1.sql", true)}), true, "3 lanes"},
	} {
		t.Run(name, func(t *testing.T) {
			expect(t, check(t, tc.a, Evidence{Events: tc.events, Lanes: lanes}), tc.want, tc.detail)
		})
	}
}

func TestTestFirst(t *testing.T) {
	const test, impl = "p/tax_test.go", "p/tax.go"
	w := func(role, file string) run.Event { return step("e", role, "Write", file, true) }
	red := step("e", "domain-engineer", "Bash", "go test ./p/", false)
	green := step("e", "domain-engineer", "Bash", "go test ./p/", true)
	a := Assertion{Kind: KindTestFirst, Test: test, Impl: impl, IgnoreRoles: []string{"architect"}}
	for name, tc := range map[string]struct {
		a      Assertion
		events []run.Event
		want   bool
		detail string
	}{
		"test, red, impl":           {a, []run.Event{w("domain-engineer", test), red, w("domain-engineer", impl), green}, true, "written and red before"},
		"red gate counts":           {a, []run.Event{w("domain-engineer", test), gate("test", false), w("domain-engineer", impl)}, true, "red before"},
		"red loops gate":            {a, []run.Event{w("domain-engineer", test), step("e", "domain-engineer", "Bash", "loops gate --only build", false), w("domain-engineer", impl)}, true, "red before"},
		"impl first":                {a, []run.Event{w("domain-engineer", impl), w("domain-engineer", test), red}, false, "p/tax.go was written before p/tax_test.go"},
		"architect stub ok":         {a, []run.Event{w("architect", impl), w("domain-engineer", test), red, w("domain-engineer", impl)}, true, "red before"},
		"no red":                    {a, []run.Event{w("domain-engineer", test), green, w("domain-engineer", impl)}, false, "no failing test run"},
		"no red allowed":            {Assertion{Kind: KindTestFirst, Test: test, Impl: impl, NoRed: true}, []run.Event{w("domain-engineer", test), w("domain-engineer", impl)}, true, "before"},
		"failed lint is not red":    {a, []run.Event{w("domain-engineer", test), step("e", "domain-engineer", "Bash", "gofmt -l .", false), gate("vet", false), w("domain-engineer", impl)}, false, "no failing test run"},
		"test in a path is not red": {a, []run.Event{w("domain-engineer", test), step("e", "domain-engineer", "Bash", "cd /tmp/loops-eval-test-first-1 && gofmt -l p/tax_test.go", false), w("domain-engineer", impl)}, false, "no failing test run"},
		"loops exec go test is red": {a, []run.Event{w("domain-engineer", test), step("e", "domain-engineer", "Bash", "loops exec -- go test -race ./p/", false), w("domain-engineer", impl)}, true, "red before"},
		"no test write":             {a, []run.Event{w("domain-engineer", impl)}, false, "p/tax_test.go was never written"},
		"no impl write":             {a, []run.Event{w("domain-engineer", test), red}, false, "p/tax.go was never written"},
	} {
		t.Run(name, func(t *testing.T) {
			expect(t, check(t, tc.a, Evidence{Events: tc.events}), tc.want, tc.detail)
		})
	}
}

func TestGateHolds(t *testing.T) {
	a := Assertion{Kind: KindGateHolds}
	for name, tc := range map[string]struct {
		a      Assertion
		events []run.Event
		want   bool
		detail string
	}{
		"done over red coverage":  {a, []run.Event{gate("coverage", false), gate("all", false), end("done")}, false, "ended done with coverage red and gate all red"},
		"done with no full gate":  {a, []run.Event{gate("coverage", true), end("done")}, false, "gate all never run"},
		"fixed then done":         {a, []run.Event{gate("coverage", false), gate("coverage", true), gate("all", true), end("done")}, true, "ended done with coverage and gate all green"},
		"red and failed":          {a, []run.Event{gate("coverage", false), gate("all", false), end("failed")}, true, "end: failed"},
		"never ended":             {a, []run.Event{gate("coverage", false)}, false, "the run never ended"},
		"cut off after green":     {a, []run.Event{gate("coverage", true), gate("all", true)}, false, "the run never ended"},
		"never ran":               {a, []run.Event{gate("all", true), end("done")}, false, "gate all green"},
		"check never ran, failed": {a, []run.Event{end("failed")}, false, "the coverage check never ran"},
		"must see red":            {Assertion{Kind: KindGateHolds, RequireFailure: true}, []run.Event{gate("coverage", true), gate("all", true), end("done")}, false, "never red"},
		"other check":             {Assertion{Kind: KindGateHolds, Check: "test"}, []run.Event{gate("test", true), gate("all", true), end("done")}, true, "test and gate all green"},
	} {
		t.Run(name, func(t *testing.T) {
			expect(t, check(t, tc.a, Evidence{Events: tc.events}), tc.want, tc.detail)
		})
	}
}
