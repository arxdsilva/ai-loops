package run

import (
	"fmt"
	"slices"
	"strings"
)

// Triage is the skipper (WORKFLOW §2.8). The lead reads the request and
// fills Signals; Decide maps them to a Tier and the Steps the run skips, so a
// simple task does not pay for checks that would only spend tokens. The
// floor is not a Step and so can never be skipped: `loops gate`, the branch
// and PR rules, the race policy and the project's own rules.

// Step is one skippable part of the workflow. The values are part of the
// events.jsonl contract (the triage event's Note) and of `loops run step`.
type Step string

const (
	StepClarify           Step = "clarify"            // intake: loops:clarify; skipped = narrowest reading, charter assumptions
	StepMemory            Step = "memory"             // intake: the memory briefing and query
	StepResearchWeb       Step = "research-web"       // research: web-researcher
	StepResearchPractices Step = "research-practices" // research: practices-researcher
	StepResearchCode      Step = "research-code"      // research: code-researcher; skipped = the lead runs loops lang detect --write
	StepResearchHardware  Step = "research-hardware"  // research: hardware-researcher; skipped = plan check probes the machine
	StepPlanner           Step = "planner"            // plan: skipped = the lead writes a short plan.v1.md itself
	StepPlanReview        Step = "plan-review"        // plan: plan-reviewer
	StepArchitect         Step = "architect"          // implement: wave 0 (architect and <id>-engineer toolchain lanes)
	StepQAPerLane         Step = "qa-per-lane"        // qa: skipped = one qa-verifier for every lane
	StepQAVerify          Step = "qa-verify"          // qa: skipped = no verifier; the gate is the check
	StepQuality           Step = "qa-quality"         // qa: the quality pass
	StepSecurity          Step = "qa-security"        // qa: the security pass
	StepProductProof      Step = "product-proof"      // product: skipped = the lead opens the PR with test and gate evidence, no manual run
	StepReflect           Step = "reflect"            // reflect: skipped = the lead writes a short reflection.md, no memory pages
)

// Steps lists every skippable step in workflow order.
var Steps = [...]Step{
	StepClarify, StepMemory,
	StepResearchWeb, StepResearchPractices, StepResearchCode, StepResearchHardware,
	StepPlanner, StepPlanReview,
	StepArchitect,
	StepQAPerLane, StepQAVerify, StepQuality, StepSecurity,
	StepProductProof,
	StepReflect,
}

// researchSteps are the four researchers; when all are skipped the run
// goes from intake straight to plan.
var researchSteps = [...]Step{StepResearchWeb, StepResearchPractices, StepResearchCode, StepResearchHardware}

// Tier is how much of the workflow a run needs.
type Tier string

const (
	TierTrivial  Tier = "trivial"  // no behaviour change: the gate is the only check
	TierSmall    Tier = "small"    // a contained change: planner, code research and one verifier
	TierStandard Tier = "standard" // the full workflow, nothing skipped
)

// TaskKind is what the request asks for.
type TaskKind string

const (
	TaskDocs     TaskKind = "docs"     // documentation or comments only
	TaskTypo     TaskKind = "typo"     // a typo, rename of a message, or a one-token fix with no logic
	TaskConfig   TaskKind = "config"   // a configuration value
	TaskTest     TaskKind = "test"     // tests only
	TaskBugfix   TaskKind = "bugfix"   // a behaviour fix
	TaskFeature  TaskKind = "feature"  // new behaviour
	TaskRefactor TaskKind = "refactor" // structure only, behaviour unchanged
	TaskUpgrade  TaskKind = "upgrade"  // a language, toolchain or dependency upgrade (implies deps)
)

// TaskKinds lists the kinds `loops run triage --kind` accepts.
var TaskKinds = [...]TaskKind{TaskDocs, TaskTypo, TaskConfig, TaskTest, TaskBugfix, TaskFeature, TaskRefactor, TaskUpgrade}

// Touch is a risk area the change reaches. Each one keeps the steps that
// guard it.
type Touch string

const (
	TouchAPI         Touch = "api"         // a public interface, contract, CLI or wire format
	TouchSchema      Touch = "schema"      // persisted data, migrations, the events schema
	TouchSecurity    Touch = "security"    // auth, crypto, secrets, untrusted input
	TouchConcurrency Touch = "concurrency" // goroutines, locks, ordering, retries
	TouchDeps        Touch = "deps"        // adds, removes or bumps a dependency
	TouchExternal    Touch = "external"    // an outside API or library whose behaviour needs looking up
)

// Touches lists the risk areas `loops run triage --touches` accepts.
var Touches = [...]Touch{TouchAPI, TouchSchema, TouchSecurity, TouchConcurrency, TouchDeps, TouchExternal}

// Size limits per tier, in files the change touches.
const (
	TrivialMaxFiles = 2
	SmallMaxFiles   = 5
)

// Phase is the phase the step belongs to ("" for an unknown step).
func (st Step) Phase() Phase {
	switch st {
	case StepClarify, StepMemory:
		return PhaseIntake
	case StepResearchWeb, StepResearchPractices, StepResearchCode, StepResearchHardware:
		return PhaseResearch
	case StepPlanner, StepPlanReview:
		return PhasePlan
	case StepArchitect:
		return PhaseImplement
	case StepQAPerLane, StepQAVerify, StepQuality, StepSecurity:
		return PhaseQA
	case StepProductProof:
		return PhaseProduct
	case StepReflect:
		return PhaseReflect
	}
	return ""
}

// rank orders tiers: trivial < small < standard.
func (t Tier) rank() int {
	switch t {
	case TierTrivial:
		return 0
	case TierSmall:
		return 1
	}
	return 2
}

// changesBehaviour reports whether the kind changes what the software does,
// so that an AC needs proof against the running system.
func (k TaskKind) changesBehaviour() bool {
	switch k {
	case TaskDocs, TaskTypo, TaskConfig, TaskTest, TaskRefactor:
		return false
	}
	return true
}

// Signals are the facts the lead reads from the request (and, at the
// recheck, from the real diff).
type Signals struct {
	Kind    TaskKind `json:"kind"`
	Files   int      `json:"files"`
	Langs   int      `json:"langs,omitempty"` // language packs the change touches; 0 counts as 1
	Touches []Touch  `json:"touches,omitempty"`
	Unclear bool     `json:"unclear,omitempty"` // the ticket has open points: keep clarify
}

func (s Signals) has(t Touch) bool {
	return slices.Contains(s.Touches, t) || (t == TouchDeps && s.Kind == TaskUpgrade)
}

// String renders the signals on one line, e.g.
// "kind=bugfix files=3 langs=1 touches=none".
func (s Signals) String() string {
	ts := "none"
	if len(s.Touches) > 0 {
		parts := make([]string, len(s.Touches))
		for i, t := range s.Touches {
			parts[i] = string(t)
		}
		ts = strings.Join(parts, ",")
	}
	out := fmt.Sprintf("kind=%s files=%d langs=%d touches=%s", s.Kind, s.Files, max(s.Langs, 1), ts)
	if s.Unclear {
		out += " unclear"
	}
	return out
}

// Triage is the recorded decision (the last triage event wins).
type Triage struct {
	Tier    Tier    `json:"tier"`
	Skip    []Step  `json:"skip,omitempty"` // in Steps order
	Signals Signals `json:"signals"`
	Full    bool    `json:"full,omitempty"`    // --full or config "triage": "off": nothing is skipped
	Recheck bool    `json:"recheck,omitempty"` // checked against the real diff after implement
}

// Decide maps the signals to a tier and its skipped steps. It is total:
// an unknown kind is treated as the full workflow.
func Decide(sig Signals) Triage {
	t := tierOf(sig)
	return Triage{Tier: t, Skip: skipsFor(t, sig), Signals: sig}
}

// Full is the triage of a run that skips nothing.
func Full(sig Signals) Triage {
	return Triage{Tier: TierStandard, Signals: sig, Full: true}
}

func tierOf(sig Signals) Tier {
	switch {
	case !slices.Contains(TaskKinds[:], sig.Kind),
		max(sig.Langs, 1) > 1,
		sig.Files > SmallMaxFiles,
		sig.has(TouchAPI), sig.has(TouchSchema), sig.has(TouchConcurrency):
		return TierStandard
	case !sig.Kind.changesBehaviour() && sig.Files <= TrivialMaxFiles && len(sig.Touches) == 0:
		return TierTrivial
	}
	return TierSmall
}

// skipsFor is every step minus the ones the tier and the touches keep.
func skipsFor(t Tier, sig Signals) []Step {
	if t == TierStandard {
		return nil
	}
	keep := map[Step]bool{}
	if t == TierSmall {
		keep[StepMemory] = true
		keep[StepResearchCode] = true
		keep[StepPlanner] = true
		keep[StepQAVerify] = true
		keep[StepProductProof] = sig.Kind.changesBehaviour()
	}
	keep[StepClarify] = sig.Unclear
	if sig.has(TouchExternal) || sig.has(TouchDeps) {
		keep[StepResearchWeb] = true
	}
	if sig.has(TouchSecurity) || sig.has(TouchDeps) {
		keep[StepResearchPractices] = true
		keep[StepSecurity] = true
	}
	if sig.has(TouchDeps) {
		keep[StepArchitect] = true // wave 0 owns the manifests and lockfiles
	}
	var out []Step
	for _, st := range Steps {
		if !keep[st] {
			out = append(out, st)
		}
	}
	return out
}

// Recheck re-triages with the facts known after implement (the real file
// count, any new touch). It only adds work back: a step stays skipped when
// the fresh decision also skips it, or when its phase is at or before cur,
// since it can no longer run. The tier never goes down.
func Recheck(prev Triage, sig Signals, cur Phase) Triage {
	if prev.Full {
		out := Full(sig)
		out.Recheck = true
		return out
	}
	fresh := Decide(sig)
	out := Triage{Tier: prev.Tier, Signals: sig, Recheck: true}
	if fresh.Tier.rank() > prev.Tier.rank() {
		out.Tier = fresh.Tier
	}
	for _, st := range prev.Skip {
		if slices.Contains(fresh.Skip, st) || st.Phase().Index() <= cur.Index() {
			out.Skip = append(out.Skip, st)
		}
	}
	return out
}

// Adjust applies the lead's overrides: keep runs a step the tier skips, skip
// skips one it runs. The result stays in Steps order.
func Adjust(t Triage, keep, skip []Step) Triage {
	set := map[Step]bool{}
	for _, st := range t.Skip {
		set[st] = true
	}
	for _, st := range skip {
		set[st] = true
	}
	for _, st := range keep {
		delete(set, st)
	}
	t.Skip = nil
	for _, st := range Steps {
		if set[st] {
			t.Skip = append(t.Skip, st)
		}
	}
	if len(skip) > 0 {
		t.Full = false
	}
	return t
}

// Skips reports whether the run skips step st. Reflect comes back once the
// run has a loop-back: a loop-back is a lesson worth a reflector.
func Skips(s State, st Step) bool {
	if !slices.Contains(s.Triage.Skip, st) {
		return false
	}
	return st != StepReflect || len(s.LoopBacks) == 0
}

// Skipped returns the steps the run skips now, in Steps order.
func Skipped(s State) []Step {
	var out []Step
	for _, st := range Steps {
		if Skips(s, st) {
			out = append(out, st)
		}
	}
	return out
}

// researchSkipped reports whether triage skipped every researcher.
func researchSkipped(s State) bool {
	for _, st := range researchSteps {
		if !Skips(s, st) {
			return false
		}
	}
	return true
}

// researchRuns counts the researchers the run keeps.
func researchRuns(s State) int {
	n := 0
	for _, st := range researchSteps {
		if !Skips(s, st) {
			n++
		}
	}
	return n
}

// ParseSteps parses a comma- or space-separated step list.
func ParseSteps(s string) ([]Step, error) {
	var out []Step
	var unknown []string
	for _, f := range fields(s) {
		st := Step(f)
		if st.Phase() == "" {
			unknown = append(unknown, f)
			continue
		}
		if !slices.Contains(out, st) {
			out = append(out, st)
		}
	}
	if len(unknown) > 0 {
		return out, fmt.Errorf("unknown step %s (want %s; the gate is never a step)", strings.Join(unknown, ", "), JoinSteps(Steps[:]))
	}
	return out, nil
}

// ParseTouches parses a comma- or space-separated touch list ("none" = empty).
func ParseTouches(s string) ([]Touch, error) {
	var out []Touch
	var unknown []string
	for _, f := range fields(s) {
		t := Touch(f)
		switch {
		case f == "none":
		case !slices.Contains(Touches[:], t):
			unknown = append(unknown, f)
		case !slices.Contains(out, t):
			out = append(out, t)
		}
	}
	if len(unknown) > 0 {
		return out, fmt.Errorf("unknown touch %s (want %v or none)", strings.Join(unknown, ", "), Touches)
	}
	return out, nil
}

// ParseTaskKind validates a --kind value.
func ParseTaskKind(s string) (TaskKind, error) {
	k := TaskKind(strings.ToLower(strings.TrimSpace(s)))
	if !slices.Contains(TaskKinds[:], k) {
		return "", fmt.Errorf("unknown kind %q (want %v)", s, TaskKinds)
	}
	return k, nil
}

// JoinSteps renders steps joined by ", ", or "none".
func JoinSteps(ss []Step) string {
	if len(ss) == 0 {
		return "none"
	}
	parts := make([]string, len(ss))
	for i, st := range ss {
		parts[i] = string(st)
	}
	return strings.Join(parts, ", ")
}

func fields(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
}
