package run

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseAsk(t *testing.T) {
	cases := []struct {
		in      string
		want    []Phase
		wantErr string
	}{
		{"", nil, ""},
		{"none", nil, ""},
		{"plan", []Phase{PhasePlan}, ""},
		{"PR, approve", []Phase{PhasePlan, PhaseProduct}, ""},
		{"approval pr", []Phase{PhasePlan, PhaseProduct}, ""},
		{"reflect,intake,reflect", []Phase{PhaseIntake, PhaseReflect}, ""},
		{"all", Phases[:], ""},
		{"none,qa", []Phase{PhaseQA}, ""},
		{"plan,bogus,merge", []Phase{PhasePlan}, "bogus, merge"},
	}
	for _, c := range cases {
		got, err := ParseAsk(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("ParseAsk(%q) = %v, want %v", c.in, got, c.want)
		}
		switch {
		case c.wantErr == "" && err != nil:
			t.Errorf("ParseAsk(%q) error %v", c.in, err)
		case c.wantErr != "" && (err == nil || !strings.Contains(err.Error(), c.wantErr)):
			t.Errorf("ParseAsk(%q) error %v, want it to name %q", c.in, err, c.wantErr)
		}
	}
}

func TestFormatAsk(t *testing.T) {
	if got := FormatAsk(nil); got != "none" {
		t.Fatalf("empty: %q", got)
	}
	if got := FormatAsk([]Phase{PhasePlan, PhaseProduct}); got != "plan,product" {
		t.Fatalf("two: %q", got)
	}
	ps, err := ParseAsk(FormatAsk([]Phase{PhaseQA, PhaseReflect}))
	if err != nil || !reflect.DeepEqual(ps, []Phase{PhaseQA, PhaseReflect}) {
		t.Fatalf("round trip: %v %v", ps, err)
	}
}

func TestFoldAskAndDecisions(t *testing.T) {
	s := Fold([]Event{
		at(0, Event{Kind: KindRun, Run: "r", Name: "T"}),
		at(1, Event{Kind: KindAsk, Note: "plan,product"}),
		at(2, Event{Kind: KindPhase, Phase: PhasePlan}),
		at(3, Event{Kind: KindDecision, From: ByLead, Name: "plan-limit", Note: "item 2 rejected: out of scope", Ref: "plan-review.r3.md"}),
		at(4, Event{Kind: KindDecision, Phase: PhaseQA, From: ByUser, Name: "W1:C2", Note: "move to Open"}),
	})
	if !Asks(s, PhasePlan) || !Asks(s, PhaseProduct) || Asks(s, PhaseQA) {
		t.Fatalf("ask: %v", s.Ask)
	}
	want := []DecisionInfo{
		{Phase: PhasePlan, By: ByLead, Topic: "plan-limit", Note: "item 2 rejected: out of scope", Ref: "plan-review.r3.md", At: t0.Add(3e9)},
		{Phase: PhaseQA, By: ByUser, Topic: "W1:C2", Note: "move to Open", At: t0.Add(4e9)},
	}
	if !reflect.DeepEqual(s.Decisions, want) {
		t.Fatalf("decisions:\n got %+v\nwant %+v", s.Decisions, want)
	}
	// The last ask event wins, and "none" clears it.
	s = Fold([]Event{at(0, Event{Kind: KindRun, Run: "r"}), at(1, Event{Kind: KindAsk, Note: "all"}), at(2, Event{Kind: KindAsk, Note: "none"})})
	if len(s.Ask) != 0 {
		t.Fatalf("cleared: %v", s.Ask)
	}
}

func TestOpenFailures(t *testing.T) {
	s := Fold([]Event{
		at(0, Event{Kind: KindRun, Run: "r"}),
		at(1, Event{Kind: KindVerdict, Name: "W1:C1", Status: StatusFailed, Round: 1}),
		at(2, Event{Kind: KindVerdict, Name: "W1:C2", Status: StatusFailed, Round: 1}),
		at(3, Event{Kind: KindVerdict, Name: "W1:C1", Status: StatusVerified, Round: 2}),
		at(4, Event{Kind: KindVerdict, Name: "plan", Status: StatusChanges, Round: 1}),
		at(5, Event{Kind: KindVerdict, Name: "AC-1", Status: StatusVerified}),
	})
	got := OpenFailures(s)
	if len(got) != 2 || got[0].Name != "W1:C2" || got[1].Name != "plan" {
		t.Fatalf("open: %+v", got)
	}
	v, f := verdictCounts(s.Verdicts)
	if v != 2 || f != 2 {
		t.Fatalf("counts: verified %d failed %d", v, f)
	}
}

// state builds a folded state from events after a run start.
func state(evs ...Event) State {
	all := []Event{at(0, Event{Kind: KindRun, Run: "r", Name: "T"})}
	for i, e := range evs {
		all = append(all, at(i+1, e))
	}
	return Fold(all)
}

func phase(p Phase) Event { return Event{Kind: KindPhase, Phase: p} }

func verdict(name, status string, round int) Event {
	return Event{Kind: KindVerdict, Name: name, Status: status, Round: round}
}

func TestNext(t *testing.T) {
	ask := func(list string) Event { return Event{Kind: KindAsk, Note: list} }
	gateAll := func(ok bool) Event { return Event{Kind: KindGate, Name: GateAll, OK: Bool(ok)} }
	cases := []struct {
		name string
		s    State
		want string
	}{
		{"ended", state(Event{Kind: KindEnd, Status: OutcomeFailed}), "has ended (failed)"},
		{"running", state(phase(PhaseResearch), Event{Kind: KindAgentStart, Agent: "a"}), "wait for the 1 running"},
		{"intake", state(phase(PhaseIntake)), "write charter.md"},
		{"no phase", state(), "write charter.md"},
		{"research", state(phase(PhaseResearch)), "enter plan"},
		{"research asks", state(ask("research"), phase(PhaseResearch)), "enter plan (checkpoint: ask the user first)"},
		{"plan start", state(phase(PhasePlan)), "spawn the planner"},
		{"plan changes", state(phase(PhasePlan), verdict("plan", StatusChanges, 1)), "plan-review.r1.md back to the same planner (round 2 of 3)"},
		{"plan exhausted", state(phase(PhasePlan), verdict("plan", StatusChanges, 1), verdict("plan", StatusChanges, 2), verdict("plan", StatusChanges, 3)), "rule on each open blocking item yourself"},
		{"plan exhausted asks", state(ask("plan"), phase(PhasePlan), verdict("plan", StatusChanges, 1), verdict("plan", StatusChanges, 2), verdict("plan", StatusChanges, 3)), "ask the user about the open blocking items"},
		{"plan approved", state(phase(PhasePlan), verdict("plan", StatusApprove, 1)), "enter implement"},
		{"plan approved asks", state(ask("approve"), phase(PhasePlan), verdict("plan", StatusApprove, 1)), "enter implement (checkpoint"},
		{"implement", state(phase(PhaseImplement)), "enter qa"},
		{"qa round 1", state(phase(PhaseQA), verdict("W1:C1", StatusFailed, 1)), "send W1:C1 back to its author with the QA file (round 2)"},
		{"qa round 2", state(phase(PhaseQA), verdict("W1:C1", StatusFailed, 2)), "states the premise"},
		{"qa round 3", state(phase(PhaseQA), verdict("W1:C1", StatusFailed, 3)), "decide yourself"},
		{"qa round 3 asks", state(ask("qa"), phase(PhaseQA), verdict("W1:C1", StatusFailed, 3)), "failed 3 rounds: ask the user"},
		{"qa verified", state(phase(PhaseQA), verdict("W1:C1", StatusVerified, 1)), "run the quality pass"},
		{"qa red gate", state(phase(PhaseQA), gateAll(false)), "run the quality pass"},
		{"qa green", state(phase(PhaseQA), gateAll(true)), "the gate is green: enter product"},
		{"qa green then edit", state(phase(PhaseQA), gateAll(true), Event{Kind: KindStep, Name: "Edit", Target: "a.go"}), "run the quality pass"},
		{"product", state(phase(PhaseProduct)), "prove every AC"},
		{"product gap", state(phase(PhaseProduct), verdict("AC-2", StatusFailed, 0), Event{Kind: KindLoopBack, Phase: PhaseProduct, From: "product-reviewer", To: "W1"}), "AC-2 is FAILED: send its proof to the owning lane (product round 2 of 2)"},
		{"product pr", state(phase(PhaseProduct), Event{Kind: KindPR, Ref: "u"}), "the PR is open: enter reflect"},
		{"product pr asks", state(ask("pr"), phase(PhaseProduct), Event{Kind: KindPR, Ref: "u"}), "enter reflect (checkpoint"},
		{"reflect", state(phase(PhaseReflect)), "loops run end --outcome done"},
		{"unknown phase", state(Event{Kind: KindPhase, Phase: "weird"}), "continue the phase"},
	}
	for _, c := range cases {
		if got := Next(c.s); !strings.Contains(got, c.want) {
			t.Errorf("%s: Next = %q, want it to contain %q", c.name, got, c.want)
		}
	}
	if got := Next(state(phase(PhaseResearch))); strings.Contains(got, "checkpoint") {
		t.Errorf("no checkpoint expected: %q", got)
	}
}

func TestBrief(t *testing.T) {
	s := Fold([]Event{
		at(0, Event{Kind: KindRun, Run: "r1", Name: "Add export", Target: "#42"}),
		at(1, Event{Kind: KindAsk, Note: "plan"}),
		at(2, phase(PhaseIntake)),
		at(3, phase(PhaseQA)),
		at(4, Event{Kind: KindAgentStart, Agent: "a1", Role: "qa-verifier"}),
		at(5, Event{Kind: KindLabel, Agent: "a1", Name: "qa-W1-domain"}),
		at(6, verdict("W1:C1", StatusFailed, 2)),
		at(7, Event{Kind: KindVerdict, Name: "W1:C2", Status: StatusVerified, Round: 1, Ref: "qa/W1.r1.md"}),
		at(8, Event{Kind: KindGate, Name: GateAll, OK: Bool(false)}),
		at(9, Event{Kind: KindPR, Ref: "https://pr/1"}),
		at(10, Event{Kind: KindDecision, From: ByLead, Name: "scope", Note: "CSV only"}),
	})
	got := Brief(s, "/runs/r1")
	for _, want := range []string{
		`lead (master agent) of loops run r1: "Add export", ticket #42.`,
		"/runs/r1/charter.md",
		"Phase: qa (path intake → qa).",
		"Checkpoints: plan.",
		"Running: qa-W1-domain.",
		"1 verified, 1 open failures",
		"open: W1:C1 FAILED (round 2",
		"Gate: red.",
		"PR: https://pr/1.",
		"[qa] lead: scope: CSV only",
		"Next: wait for the 1 running agent(s)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("brief lacks %q:\n%s", want, got)
		}
	}
	bare := Brief(state(phase(PhasePlan)), "")
	for _, absent := range []string{"Scope:", "Running:", "Gate:", "PR:", "Decisions", "(path"} {
		if strings.Contains(bare, absent) {
			t.Errorf("bare brief has %q:\n%s", absent, bare)
		}
	}
	if !strings.Contains(bare, "Checkpoints: none. You decide every phase") {
		t.Errorf("bare brief checkpoints:\n%s", bare)
	}
}

func TestBriefTrimsLists(t *testing.T) {
	var evs []Event
	for i := range 7 {
		evs = append(evs, Event{Kind: KindDecision, From: ByLead, Name: "d" + string(rune('0'+i)), Note: "n"})
	}
	for i := range 5 {
		evs = append(evs, verdict("W:C"+string(rune('0'+i)), StatusFailed, 1))
	}
	evs = append(evs, Event{Kind: KindGate, Name: GateAll, OK: Bool(true)}, Event{Kind: KindStep, Name: "Edit", Target: "a.go"})
	got := Brief(state(evs...), "/d")
	if strings.Contains(got, "d1:") || !strings.Contains(got, "d2:") || !strings.Contains(got, "d6:") {
		t.Errorf("decisions not trimmed to the last 5:\n%s", got)
	}
	if strings.Count(got, "  open:") != 3 || !strings.Contains(got, "  …") {
		t.Errorf("open failures not trimmed to 3:\n%s", got)
	}
	if !strings.Contains(got, "Gate: green, but sources changed since.") {
		t.Errorf("dirty gate:\n%s", got)
	}
	clean := Brief(state(Event{Kind: KindGate, Name: GateAll, OK: Bool(true)}), "")
	if !strings.Contains(clean, "Gate: green.") {
		t.Errorf("green gate:\n%s", clean)
	}
}

func TestLaneName(t *testing.T) {
	if laneName(Lane{Agent: "a", Role: "r", Label: "L"}) != "L" || laneName(Lane{Agent: "a", Role: "r"}) != "r" || laneName(Lane{Agent: "a"}) != "a" {
		t.Fatal("laneName fallbacks")
	}
}
