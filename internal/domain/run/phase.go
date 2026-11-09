package run

// Phase is one step of the loops workflow (specs/WORKFLOW.md §2). The JSON
// values are part of the events.jsonl contract.
type Phase string

const (
	PhaseIntake    Phase = "intake"    // 0: ticket fetched and normalised
	PhaseResearch  Phase = "research"  // 1: web, practices and code researchers
	PhasePlan      Phase = "plan"      // 2: planner ↔ plan-reviewer loop (+ optional user approval)
	PhaseImplement Phase = "implement" // 3: architect wave 0, role engineers in waves
	PhaseQA        Phase = "qa"        // 4: one qa-verifier per implementer, then full gates
	PhaseProduct   Phase = "product"   // 5: product-reviewer proves every AC, opens the PR/MR
	PhaseReflect   Phase = "reflect"   // 6: reflector writes reflection.md and ai-memory pages
)

// Phases lists the phases in workflow order.
var Phases = [...]Phase{PhaseIntake, PhaseResearch, PhasePlan, PhaseImplement, PhaseQA, PhaseProduct, PhaseReflect}

// Index returns the phase's 0-based position in Phases, or -1 if unknown.
func (p Phase) Index() int {
	for i, q := range Phases {
		if q == p {
			return i
		}
	}
	return -1
}

// Title is the short human label used by the status line, pane and report.
func (p Phase) Title() string {
	switch p {
	case PhaseIntake:
		return "Intake"
	case PhaseResearch:
		return "Research"
	case PhasePlan:
		return "Plan"
	case PhaseImplement:
		return "Implement"
	case PhaseQA:
		return "QA"
	case PhaseProduct:
		return "Product"
	case PhaseReflect:
		return "Reflect"
	}
	return string(p)
}

// Loop limits (specs/WORKFLOW.md §4).
const (
	MaxPlanRounds    = 3 // planner ↔ plan-reviewer rounds before escalating to the user
	MaxQARounds      = 3 // implementer ↔ qa-verifier rounds before escalating to the user
	EscalateAfter    = 2 // failed QA rounds before the implementer is relaunched one model tier up
	MaxProductRounds = 2 // product → implementer returns before escalating to the user
)
