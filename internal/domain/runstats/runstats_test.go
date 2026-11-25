package runstats

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

var t0 = time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)

func at(min float64, e run.Event) run.Event {
	e.Time = t0.Add(time.Duration(min * float64(time.Minute)))
	return e
}

func near(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func syntheticRun() []run.Event {
	return []run.Event{
		at(0, run.Event{Kind: run.KindRun, Run: "r1", Name: "t"}),
		at(0, run.Event{Kind: run.KindPhase, Phase: run.PhaseResearch}),
		at(1, run.Event{Kind: run.KindTriage, Name: "small"}),
		at(2, run.Event{Kind: run.KindTriage, Name: "medium"}), // the last one wins
		at(10, run.Event{Kind: run.KindPhase, Phase: run.PhaseImplement}),
		at(10, run.Event{Kind: run.KindAgentStart, Agent: "a1", Role: "domain-engineer"}),
		at(12, run.Event{Kind: run.KindAgentStart, Agent: "a2"}),
		at(12, run.Event{Kind: run.KindLabel, Agent: "a2", Name: "W1", Role: "api-engineer"}),
		at(14, run.Event{Kind: run.KindAgentStop, Agent: "a1", Tokens: 100}),
		at(15, run.Event{Kind: run.KindAgentStop, Agent: "a2", Role: "ignored", Tokens: 50}),
		at(16, run.Event{Kind: run.KindAgentStart, Agent: "a1", Role: "domain-engineer"}), // resumed
		at(17, run.Event{Kind: run.KindAgentStop, Agent: "a1", Tokens: 180}),              // cumulative
		at(18, run.Event{Kind: run.KindPhase, Phase: run.PhaseQA}),
		at(18, run.Event{Kind: run.KindAgentStart, Agent: "q1", Role: "qa-verifier"}), // never stops
		at(19, run.Event{Kind: run.KindLoopBack, From: "qa-verifier", To: "W1"}),
		at(19, run.Event{Kind: run.KindLoopBack, From: "qa-verifier", To: "W2"}),
		at(19, run.Event{Kind: run.KindLoopBack, From: "qa-quality", To: "W2"}),
		at(20, run.Event{Kind: run.KindGate, Name: "test", DurationMS: 1200}),
		at(21, run.Event{Kind: run.KindGate, Name: "test", DurationMS: 3400}),
		at(21, run.Event{Kind: run.KindGate, Name: "build", DurationMS: 9000}),
		at(21, run.Event{Kind: run.KindGuardDeny, Name: "read-guard"}),
		at(21, run.Event{Kind: run.KindGuardDeny, Name: "read-guard"}),
		at(21, run.Event{Kind: run.KindGuardDeny, Name: "test-guard"}),
		at(22, run.Event{Kind: run.KindNote, Name: "lead-tokens", Tokens: 500}),
		at(24, run.Event{Kind: run.KindNote, Name: "lead-tokens", Tokens: 900}),
		at(24, run.Event{Kind: run.KindNote, Name: "other", Tokens: 99999}),
		at(30, run.Event{Kind: run.KindEnd, Status: "done"}),
	}
}

func TestComputeSynthetic(t *testing.T) {
	r := Compute(syntheticRun())
	if r.ID != "r1" || r.Tier != "medium" {
		t.Errorf("id/tier = %q %q", r.ID, r.Tier)
	}
	near(t, "wall", r.WallMin, 30)
	near(t, "research", r.PhaseMin["research"], 10)
	near(t, "implement", r.PhaseMin["implement"], 8)
	near(t, "qa", r.PhaseMin["qa"], 12)
	// implement: agents cover 10-15 and 16-17 = 6 of 8 minutes
	near(t, "research lead", r.LeadMin["research"], 10)
	near(t, "implement lead", r.LeadMin["implement"], 2)
	// qa: q1 runs from 18 to the last event (30)
	near(t, "qa lead", r.LeadMin["qa"], 0)
	if r.Agents != 3 || r.Peak != 2 {
		t.Errorf("agents %d peak %d", r.Agents, r.Peak)
	}
	want := map[string][]int64{"domain-engineer": {180}, "api-engineer": {50}}
	if !reflect.DeepEqual(r.RoleTokens, want) {
		t.Errorf("role tokens = %v", r.RoleTokens)
	}
	if r.TotalTokens() != 230 || r.LeadTokens != 900 {
		t.Errorf("total %d lead %d", r.TotalTokens(), r.LeadTokens)
	}
	if !reflect.DeepEqual(r.LoopBacks, map[string]int{"qa-verifier": 2, "qa-quality": 1}) {
		t.Errorf("loop-backs %v", r.LoopBacks)
	}
	if r.TestMS != 3400 {
		t.Errorf("test ms %d", r.TestMS)
	}
	if !reflect.DeepEqual(r.Denies, map[string]int{"read-guard": 2, "test-guard": 1}) {
		t.Errorf("denies %v", r.Denies)
	}
}

func TestComputeEdges(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		r := Compute(nil)
		if r.Tier != "-" || r.WallMin != 0 || r.Agents != 0 {
			t.Errorf("%+v", r)
		}
	})
	t.Run("no tier no phases", func(t *testing.T) {
		r := Compute([]run.Event{at(0, run.Event{Kind: run.KindRun, Run: "x"}), at(3, run.Event{Kind: run.KindNote})})
		near(t, "wall", r.WallMin, 3)
		if r.Tier != "-" || len(r.PhaseMin) != 0 {
			t.Errorf("%+v", r)
		}
	})
	t.Run("overlap and touching spans", func(t *testing.T) {
		r := Compute([]run.Event{
			at(0, run.Event{Kind: run.KindPhase, Phase: run.PhaseImplement}),
			at(0, run.Event{Kind: run.KindAgentStart, Agent: "a"}),
			at(0, run.Event{Kind: run.KindAgentStart, Agent: "b"}),
			at(4, run.Event{Kind: run.KindAgentStop, Agent: "a"}),
			at(4, run.Event{Kind: run.KindAgentStart, Agent: "c"}), // starts as a ends: not concurrent
			at(6, run.Event{Kind: run.KindAgentStop, Agent: "b"}),
			at(8, run.Event{Kind: run.KindAgentStop, Agent: "c"}),
			at(10, run.Event{Kind: run.KindPhase, Phase: run.PhaseQA}),
		})
		if r.Peak != 2 || r.Agents != 3 {
			t.Errorf("peak %d agents %d", r.Peak, r.Agents)
		}
		near(t, "idle", r.LeadMin["implement"], 2)
	})
	t.Run("tokens without a role", func(t *testing.T) {
		r := Compute([]run.Event{at(0, run.Event{Kind: run.KindAgentStop, Agent: "z", Tokens: 7})})
		if !reflect.DeepEqual(r.RoleTokens, map[string][]int64{"unknown": {7}}) {
			t.Errorf("%v", r.RoleTokens)
		}
	})
}

func TestSummarize(t *testing.T) {
	mk := func(wall, impl, implLead float64, lead int64, roles map[string][]int64) Run {
		return Run{WallMin: wall, PhaseMin: map[string]float64{"implement": impl}, LeadMin: map[string]float64{"implement": implLead}, RoleTokens: roles, LeadTokens: lead}
	}
	s := Summarize([]Run{
		mk(10, 4, 1, 100, map[string][]int64{"a": {100, 300}}),
		mk(30, 8, 3, 0, map[string][]int64{"a": {200}, "b": {400}}),
		mk(20, 6, 2, 100, nil),
	})
	if s.Runs != 3 {
		t.Fatal(s.Runs)
	}
	near(t, "median wall", s.MedianWallMin, 20)
	if len(s.Phases) != 1 || s.Phases[0].Phase != "implement" || s.Phases[0].Runs != 3 {
		t.Fatalf("%+v", s.Phases)
	}
	near(t, "median impl", s.Phases[0].Min, 6)
	near(t, "median impl lead", s.Phases[0].LeadMin, 2)
	// a: 600 (3 agents, median 200), b: 400, lead: 200; total 1200
	want := []RoleRow{
		{Role: "a", Agents: 3, Tokens: 600, Median: 200, Share: 0.5},
		{Role: "b", Agents: 1, Tokens: 400, Median: 400, Share: 400.0 / 1200},
		{Role: "lead", Tokens: 200, Share: 200.0 / 1200},
	}
	if !reflect.DeepEqual(s.Roles, want) {
		t.Errorf("roles:\n got %+v\nwant %+v", s.Roles, want)
	}
	if e := Summarize(nil); e.Runs != 0 || len(e.Roles) != 0 || e.MedianWallMin != 0 {
		t.Errorf("%+v", e)
	}
}

func TestMedians(t *testing.T) {
	near(t, "even", medianF([]float64{4, 1, 3, 2}), 2.5)
	near(t, "odd", medianF([]float64{5, 1, 3}), 3)
	if medianI([]int64{1, 2, 3, 10}) != 2 || medianI(nil) != 0 {
		t.Error("medianI")
	}
}
