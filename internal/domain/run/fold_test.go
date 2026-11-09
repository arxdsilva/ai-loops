package run

import (
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 9, 15, 0, 0, 0, time.UTC)

func at(i int, e Event) Event { e.Time = t0.Add(time.Duration(i) * time.Second); return e }

func TestFoldRunAndPhases(t *testing.T) {
	s := Fold([]Event{
		at(0, Event{Kind: KindRun, Run: "r1", Name: "T", Target: "#1", Note: "github"}),
		at(1, Event{Kind: KindPhase, Phase: PhaseIntake}),
		at(2, Event{Kind: KindPhase, Phase: PhasePlan, Note: "n"}),
		at(3, Event{Kind: KindVerdict, Name: "plan", Status: StatusChanges}),
		at(4, Event{Kind: KindVerdict, Name: "plan", Status: StatusApprove}),
		at(5, Event{Kind: KindVerdict, Name: "AC-1", Status: StatusVerified}),
		at(6, Event{Kind: "weird"}),
		at(7, Event{Kind: KindRun, Run: "other", Name: "X"}),
		at(8, Event{Kind: KindPR, Ref: "http://pr"}),
		at(9, Event{Kind: KindEnd, Status: OutcomeDone}),
	})
	if s.ID != "r1" || s.Title != "T" || s.Ticket != "#1" || s.Source != "github" || !s.Started.Equal(t0) {
		t.Fatalf("header: %+v", s)
	}
	if s.Phase != PhasePlan || len(s.PhaseLog) != 2 || s.PhaseLog[1].Note != "n" {
		t.Fatalf("phases: %+v", s.PhaseLog)
	}
	if s.PlanRounds != 2 || len(s.Verdicts) != 3 {
		t.Fatalf("rounds=%d verdicts=%d", s.PlanRounds, len(s.Verdicts))
	}
	if s.PR != "http://pr" || !s.Ended || s.Outcome != OutcomeDone || !s.Updated.Equal(t0.Add(9*time.Second)) {
		t.Fatalf("end: %+v", s)
	}
	if Fold(nil).ID != "" {
		t.Fatal("empty")
	}
}

func TestFoldLanesAndSteps(t *testing.T) {
	events := []Event{
		at(0, Event{Kind: KindRun, Run: "r", Name: "T"}),
		at(1, Event{Kind: KindPhase, Phase: PhaseImplement}),
		at(2, Event{Kind: KindAgentStart, Agent: "a1", Role: "domain-engineer", Model: "sonnet", Wave: 1}),
		at(3, Event{Kind: KindLabel, Agent: "a1", Name: "W1-domain", Role: "domain-engineer", Wave: 1}),
		at(4, Event{Kind: KindStep, Agent: "a1", Name: "Edit", Target: "x/y.go", OK: Bool(true)}),
		at(5, Event{Kind: KindStep, Agent: "a1", Name: "Bash", OK: Bool(false)}),
		at(6, Event{Kind: KindStep, Name: "Read", Target: "a.md"}),
		at(7, Event{Kind: KindLabel, Agent: "a2", Name: "W1-x", Role: "r", Wave: 2}),
		at(8, Event{Kind: KindAgentStop, Agent: "a1", OK: Bool(true)}),
		at(9, Event{Kind: KindVerdict, Name: "W1-domain:C1", Status: StatusFailed, Round: 1}),
		at(10, Event{Kind: KindVerdict, Name: "a1:C2", Status: StatusVerified, Round: 2}),
		at(11, Event{Kind: KindGuardDeny}),
		at(12, Event{Kind: KindMemory, Name: "query"}),
		at(12, Event{Kind: KindMemory, Name: "briefing"}),
		at(12, Event{Kind: KindMemory, Name: "write"}),
		at(13, Event{Kind: KindLoopBack, From: "qa", To: "a1", Round: 1, Note: "why"}),
		at(14, Event{Kind: KindProof, Name: "AC-1", Ref: "proofs/AC-1.md"}),
	}
	s := Fold(events)
	if len(s.Lanes) != 2 {
		t.Fatalf("lanes %+v", s.Lanes)
	}
	l := s.Lanes[0]
	if l.Role != "domain-engineer" || l.Model != "sonnet" || l.Label != "W1-domain" || l.Phase != PhaseImplement ||
		l.Running || l.OK == nil || !*l.OK || l.Steps != 2 || l.Errors != 1 || l.QA != StatusFailed || l.QARounds != 2 || !l.Stopped.Equal(t0.Add(8*time.Second)) {
		t.Fatalf("lane: %+v", l)
	}
	if s.Lanes[1].Label != "W1-x" || s.Lanes[1].Wave != 2 {
		t.Fatalf("lane2: %+v", s.Lanes[1])
	}
	if s.Steps != 3 || s.Errors != 1 || s.LeadStep != 1 || s.Denials != 1 || s.MemoryQ != 2 || s.MemoryW != 1 {
		t.Fatalf("counters: %+v", s)
	}
	if s.Recent[0].Role != "domain-engineer" || len(s.Recent) != 3 {
		t.Fatalf("recent: %+v", s.Recent)
	}
	if len(s.LoopBacks) != 1 || s.LoopBacks[0].Reason != "why" || len(s.Proofs) != 1 {
		t.Fatalf("lb/proofs")
	}
	if !s.GoDirty || !s.LastGoChange.Equal(t0.Add(4*time.Second)) {
		t.Fatalf("dirty: %+v", s)
	}
}

func TestFoldRecentRing(t *testing.T) {
	var ev []Event
	for i := 0; i < RecentSteps+5; i++ {
		ev = append(ev, at(i, Event{Kind: KindStep, Name: "Read"}))
	}
	s := Fold(ev)
	if len(s.Recent) != RecentSteps || !s.Recent[0].At.Equal(t0.Add(5*time.Second)) {
		t.Fatalf("ring: %d %v", len(s.Recent), s.Recent[0].At)
	}
}

func TestFoldGates(t *testing.T) {
	s := Fold([]Event{
		at(1, Event{Kind: KindStep, Name: "Write", Target: "a.go"}),
		at(2, Event{Kind: KindGate, Name: "build", OK: Bool(false), Note: "x"}),
		at(3, Event{Kind: KindGate, Name: "build", OK: Bool(true)}),
		at(4, Event{Kind: KindGate, Name: GateAll, OK: Bool(false)}),
	})
	if len(s.Gates) != 2 || !s.Gates[0].OK || !s.GoDirty {
		t.Fatalf("%+v", s)
	}
	s = Fold([]Event{
		at(1, Event{Kind: KindStep, Name: "MultiEdit", Target: "a.go"}),
		at(2, Event{Kind: KindGate, Name: GateAll, OK: Bool(true)}),
		at(3, Event{Kind: KindStep, Name: "Edit", Target: "a.txt"}),
	})
	if s.GoDirty || !s.LastGreen.Equal(t0.Add(2*time.Second)) {
		t.Fatalf("%+v", s)
	}
}

func TestStopReason(t *testing.T) {
	cases := []struct {
		name string
		s    State
		want bool
	}{
		{"clean", State{}, false},
		{"dirty", State{GoDirty: true}, true},
		{"dirty ended", State{GoDirty: true, Ended: true}, false},
		{"dirty running lane", State{GoDirty: true, Lanes: []Lane{{Running: true}}}, false},
		{"dirty stopped lane", State{GoDirty: true, Lanes: []Lane{{}}}, true},
	}
	for _, c := range cases {
		got := StopReason(c.s)
		if (got != "") != c.want {
			t.Errorf("%s: %q", c.name, got)
		}
		if got != "" && !strings.Contains(got, "loops gate") {
			t.Errorf("%s: text", c.name)
		}
	}
}

func TestGreenAllDoesNotClearEditsMadeDuringGate(t *testing.T) {
	s := Fold([]Event{
		at(1, Event{Kind: KindStep, Name: "Edit", Target: "a.go"}),
		at(5, Event{Kind: KindStep, Name: "Edit", Target: "b.go"}),  // during the gate
		at(3, Event{Kind: KindGate, Name: GateAll, OK: Bool(true)}), // stamped with pipeline start
	})
	if !s.GoDirty || !s.LastGreen.Equal(t0.Add(3*time.Second)) {
		t.Fatalf("%+v", s)
	}
	s = Fold([]Event{
		at(1, Event{Kind: KindStep, Name: "Edit", Target: "a.go"}),
		at(3, Event{Kind: KindGate, Name: GateAll, OK: Bool(true)}),
	})
	if s.GoDirty {
		t.Fatal("should clear")
	}
}

func TestLaneQAAggregation(t *testing.T) {
	base := []Event{
		at(0, Event{Kind: KindLabel, Agent: "a1", Name: "W1"}),
		at(1, Event{Kind: KindVerdict, Name: "W1:C1", Status: StatusFailed, Round: 1}),
		at(2, Event{Kind: KindVerdict, Name: "W1:C2", Status: StatusVerified, Round: 1}),
	}
	s := Fold(base)
	if s.Lanes[0].QA != StatusFailed || s.Lanes[0].QARounds != 1 {
		t.Fatalf("%+v", s.Lanes[0])
	}
	s = Fold(append(base, at(3, Event{Kind: KindVerdict, Name: "W1:C1", Status: StatusVerified, Round: 2})))
	if s.Lanes[0].QA != StatusVerified || s.Lanes[0].QARounds != 2 {
		t.Fatalf("%+v", s.Lanes[0])
	}
	// verdict before the label still attaches
	s = Fold([]Event{
		at(0, Event{Kind: KindVerdict, Name: "W1:C1", Status: StatusFailed, Round: 1}),
		at(1, Event{Kind: KindLabel, Agent: "a1", Name: "W1"}),
	})
	if s.Lanes[0].QA != StatusFailed {
		t.Fatalf("%+v", s.Lanes[0])
	}
}
