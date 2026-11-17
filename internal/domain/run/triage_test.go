package run

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestDecideTiers(t *testing.T) {
	cases := []struct {
		name string
		sig  Signals
		want Tier
	}{
		{"typo one file", Signals{Kind: TaskTypo, Files: 1}, TierTrivial},
		{"docs two files", Signals{Kind: TaskDocs, Files: 2}, TierTrivial},
		{"docs three files", Signals{Kind: TaskDocs, Files: 3}, TierSmall},
		{"config with deps", Signals{Kind: TaskConfig, Files: 1, Touches: []Touch{TouchDeps}}, TierSmall},
		{"bugfix one file", Signals{Kind: TaskBugfix, Files: 1}, TierSmall},
		{"feature five files", Signals{Kind: TaskFeature, Files: 5}, TierSmall},
		{"feature six files", Signals{Kind: TaskFeature, Files: 6}, TierStandard},
		{"two languages", Signals{Kind: TaskDocs, Files: 1, Langs: 2}, TierStandard},
		{"api", Signals{Kind: TaskBugfix, Files: 1, Touches: []Touch{TouchAPI}}, TierStandard},
		{"schema", Signals{Kind: TaskTypo, Files: 1, Touches: []Touch{TouchSchema}}, TierStandard},
		{"concurrency", Signals{Kind: TaskRefactor, Files: 2, Touches: []Touch{TouchConcurrency}}, TierStandard},
		{"unknown kind", Signals{Kind: "rewrite", Files: 1}, TierStandard},
		{"upgrade", Signals{Kind: TaskUpgrade, Files: 2}, TierSmall},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Decide(c.sig).Tier; got != c.want {
				t.Errorf("Decide(%v).Tier = %s, want %s", c.sig, got, c.want)
			}
		})
	}
}

func TestDecideSkips(t *testing.T) {
	t.Run("trivial skips every step", func(t *testing.T) {
		got := Decide(Signals{Kind: TaskTypo, Files: 1}).Skip
		if !slices.Equal(got, Steps[:]) {
			t.Errorf("skip = %v, want every step", got)
		}
	})
	t.Run("trivial unclear keeps clarify", func(t *testing.T) {
		got := Decide(Signals{Kind: TaskDocs, Files: 1, Unclear: true}).Skip
		if slices.Contains(got, StepClarify) || len(got) != len(Steps)-1 {
			t.Errorf("skip = %v, want every step but clarify", got)
		}
	})
	t.Run("standard skips nothing", func(t *testing.T) {
		if got := Decide(Signals{Kind: TaskFeature, Files: 9}).Skip; got != nil {
			t.Errorf("skip = %v, want none", got)
		}
	})
	cases := []struct {
		name       string
		sig        Signals
		keep, skip []Step
	}{
		{
			"small bugfix keeps the core and the proof",
			Signals{Kind: TaskBugfix, Files: 3},
			[]Step{StepMemory, StepResearchCode, StepPlanner, StepQAVerify, StepProductProof},
			[]Step{StepClarify, StepResearchWeb, StepResearchPractices, StepResearchHardware, StepPlanReview, StepArchitect, StepQAPerLane, StepQuality, StepSecurity, StepReflect},
		},
		{
			"small refactor skips the manual proof",
			Signals{Kind: TaskRefactor, Files: 4},
			[]Step{StepPlanner, StepQAVerify},
			[]Step{StepProductProof},
		},
		{
			"security keeps practices and the security pass",
			Signals{Kind: TaskBugfix, Files: 2, Touches: []Touch{TouchSecurity}},
			[]Step{StepResearchPractices, StepSecurity},
			[]Step{StepResearchWeb, StepArchitect},
		},
		{
			"deps keep web, practices, security and wave 0",
			Signals{Kind: TaskConfig, Files: 1, Touches: []Touch{TouchDeps}},
			[]Step{StepResearchWeb, StepResearchPractices, StepSecurity, StepArchitect},
			[]Step{StepPlanReview, StepQuality},
		},
		{
			"upgrade implies deps",
			Signals{Kind: TaskUpgrade, Files: 2},
			[]Step{StepResearchWeb, StepSecurity, StepArchitect, StepProductProof},
			[]Step{StepPlanReview},
		},
		{
			"external keeps web research",
			Signals{Kind: TaskFeature, Files: 2, Touches: []Touch{TouchExternal}},
			[]Step{StepResearchWeb},
			[]Step{StepResearchPractices, StepSecurity},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Decide(c.sig).Skip
			for _, st := range c.keep {
				if slices.Contains(got, st) {
					t.Errorf("%s skipped, want it kept (skip = %v)", st, got)
				}
			}
			for _, st := range c.skip {
				if !slices.Contains(got, st) {
					t.Errorf("%s kept, want it skipped (skip = %v)", st, got)
				}
			}
		})
	}
}

func TestStepPhases(t *testing.T) {
	prev := 0
	for _, st := range Steps {
		i := st.Phase().Index()
		if i < 0 {
			t.Fatalf("%s has no phase", st)
		}
		if i < prev {
			t.Errorf("%s (%s) is out of workflow order", st, st.Phase())
		}
		prev = i
	}
	if Step("gate").Phase() != "" {
		t.Error("the gate must never be a step")
	}
}

func TestRecheck(t *testing.T) {
	prev := Decide(Signals{Kind: TaskDocs, Files: 1})
	t.Run("same size keeps the skips", func(t *testing.T) {
		got := Recheck(prev, Signals{Kind: TaskDocs, Files: 2}, PhaseImplement)
		if got.Tier != TierTrivial || !slices.Equal(got.Skip, prev.Skip) || !got.Recheck {
			t.Errorf("got %+v, want trivial with the same skips", got)
		}
	})
	t.Run("a bigger diff adds later steps back only", func(t *testing.T) {
		got := Recheck(prev, Signals{Kind: TaskDocs, Files: 8}, PhaseImplement)
		if got.Tier != TierStandard {
			t.Errorf("tier = %s, want standard", got.Tier)
		}
		for _, st := range Steps {
			passed := st.Phase().Index() <= PhaseImplement.Index()
			if slices.Contains(got.Skip, st) != passed {
				t.Errorf("%s skipped = %v, want %v (phase passed = %v)", st, !passed, passed, passed)
			}
		}
	})
	t.Run("the tier never goes down", func(t *testing.T) {
		small := Decide(Signals{Kind: TaskBugfix, Files: 4})
		got := Recheck(small, Signals{Kind: TaskTypo, Files: 1}, PhaseImplement)
		if got.Tier != TierSmall || !slices.Equal(got.Skip, small.Skip) {
			t.Errorf("got %+v, want the small skips kept", got)
		}
	})
	t.Run("full stays full", func(t *testing.T) {
		got := Recheck(Full(Signals{}), Signals{Kind: TaskTypo, Files: 1}, PhaseImplement)
		if !got.Full || got.Skip != nil {
			t.Errorf("got %+v, want full", got)
		}
	})
}

func TestAdjust(t *testing.T) {
	base := Decide(Signals{Kind: TaskBugfix, Files: 2})
	got := Adjust(base, []Step{StepQuality}, []Step{StepMemory, StepMemory})
	if slices.Contains(got.Skip, StepQuality) {
		t.Error("kept step still skipped")
	}
	if !slices.Contains(got.Skip, StepMemory) {
		t.Error("forced step not skipped")
	}
	if !slices.IsSortedFunc(got.Skip, func(a, b Step) int { return slices.Index(Steps[:], a) - slices.Index(Steps[:], b) }) {
		t.Errorf("skip %v is not in workflow order", got.Skip)
	}
	if Adjust(Full(Signals{}), nil, []Step{StepReflect}).Full {
		t.Error("a forced skip must clear Full")
	}
}

func TestSkipsReflectComesBackOnLoopBack(t *testing.T) {
	tr := Decide(Signals{Kind: TaskTypo, Files: 1})
	note, _ := json.Marshal(tr)
	ev := Event{Kind: KindTriage, Name: string(tr.Tier), Note: string(note)}
	s := state(ev)
	if !Skips(s, StepReflect) || !Skips(s, StepQAVerify) {
		t.Fatalf("folded triage lost its skips: %+v", s.Triage)
	}
	s = state(ev, Event{Kind: KindLoopBack, From: "gate", To: "lead"})
	if Skips(s, StepReflect) {
		t.Error("reflect stays skipped after a loop-back")
	}
	if slices.Contains(Skipped(s), StepReflect) {
		t.Error("Skipped lists reflect after a loop-back")
	}
	if Skips(state(), StepReflect) {
		t.Error("an untriaged run skips nothing")
	}
}

func TestParseTriageInputs(t *testing.T) {
	if got, err := ParseSteps("reflect, qa-quality reflect"); err != nil || !slices.Equal(got, []Step{StepReflect, StepQuality}) {
		t.Errorf("ParseSteps = %v, %v", got, err)
	}
	if _, err := ParseSteps("gate"); err == nil || !strings.Contains(err.Error(), "never a step") {
		t.Errorf("ParseSteps(gate) err = %v, want the floor named", err)
	}
	if got, err := ParseTouches("api,deps none"); err != nil || !slices.Equal(got, []Touch{TouchAPI, TouchDeps}) {
		t.Errorf("ParseTouches = %v, %v", got, err)
	}
	if _, err := ParseTouches("ui"); err == nil {
		t.Error("ParseTouches(ui) accepted an unknown touch")
	}
	if k, err := ParseTaskKind(" Bugfix "); err != nil || k != TaskBugfix {
		t.Errorf("ParseTaskKind = %q, %v", k, err)
	}
	if _, err := ParseTaskKind("chore"); err == nil {
		t.Error("ParseTaskKind(chore) accepted an unknown kind")
	}
	if got := (Signals{Kind: TaskBugfix, Files: 3, Touches: []Touch{TouchAPI}, Unclear: true}).String(); got != "kind=bugfix files=3 langs=1 touches=api unclear" {
		t.Errorf("Signals.String = %q", got)
	}
	if JoinSteps(nil) != "none" {
		t.Error("JoinSteps(nil) != none")
	}
}

func TestNextFollowsTriage(t *testing.T) {
	tri := func(sig Signals) Event {
		tr := Decide(sig)
		note, _ := json.Marshal(tr)
		return Event{Kind: KindTriage, Name: string(tr.Tier), Note: string(note)}
	}
	rechecked := func(sig Signals) Event {
		tr := Decide(sig)
		tr.Recheck = true
		note, _ := json.Marshal(tr)
		return Event{Kind: KindTriage, Name: string(tr.Tier), Note: string(note)}
	}
	trivial := Signals{Kind: TaskTypo, Files: 1}
	small := Signals{Kind: TaskBugfix, Files: 2}
	gateAll := Event{Kind: KindGate, Name: GateAll, OK: Bool(true)}
	files := Event{Kind: KindDecision, From: ByLead, Name: MemoryStoreTopic, Note: "files: x"}
	cases := []struct {
		name string
		s    State
		want string
	}{
		{"untriaged intake", state(phase(PhaseIntake)), "triage the request (loops run triage)"},
		{"trivial intake skips research", state(tri(trivial), phase(PhaseIntake)), "then enter plan: triage skipped research"},
		{"small research count", state(tri(small), phase(PhaseResearch)), "when the 1 research output(s) triage kept exist"},
		{"trivial plan", state(tri(trivial), phase(PhasePlan)), "write a short plan.v1.md yourself"},
		{"small plan", state(tri(small), phase(PhasePlan)), "triage skipped the plan-reviewer"},
		{"implement asks for a recheck", state(tri(small), phase(PhaseImplement)), "loops run triage --recheck"},
		{"implement after recheck", state(rechecked(small), phase(PhaseImplement)), "enter qa and verify each report"},
		{"small qa", state(tri(small), phase(PhaseQA)), "every claim is verified: run loops gate"},
		{"trivial qa", state(tri(trivial), phase(PhaseQA)), "triage skipped the verifiers: run loops gate"},
		{"security kept", state(tri(Signals{Kind: TaskBugfix, Files: 1, Touches: []Touch{TouchSecurity}}), phase(PhaseQA)), "run the security pass, then loops gate"},
		{"file store, reflect skipped", state(files, tri(trivial), phase(PhaseQA), gateAll), "the gate is green: enter product"},
		{"trivial product", state(tri(trivial), phase(PhaseProduct)), "open the PR yourself"},
		{"small product keeps the proof", state(tri(small), phase(PhaseProduct)), "have the product-reviewer prove every AC"},
		{"trivial reflect", state(tri(trivial), phase(PhaseReflect)), "write a short reflection.md yourself"},
		{"loop-back brings the reflector back", state(tri(trivial), Event{Kind: KindLoopBack, From: "gate", To: "lead"}, phase(PhaseReflect)), "when reflection.md is written"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Next(c.s); !strings.Contains(got, c.want) {
				t.Errorf("Next = %q, want it to contain %q", got, c.want)
			}
		})
	}
}

func TestBriefShowsTriage(t *testing.T) {
	tr := Decide(Signals{Kind: TaskBugfix, Files: 2})
	note, _ := json.Marshal(tr)
	got := Brief(state(Event{Kind: KindTriage, Name: string(tr.Tier), Note: string(note)}), "")
	for _, want := range []string{"Triage: small (kind=bugfix files=2 langs=1 touches=none)", "research-web", "loops gate always runs"} {
		if !strings.Contains(got, want) {
			t.Errorf("Brief misses %q:\n%s", want, got)
		}
	}
	if strings.Contains(Brief(state(), ""), "Triage:") {
		t.Error("an untriaged brief shows a triage line")
	}
}
