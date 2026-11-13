package run

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// briefDecisions is how many recent decisions Brief shows.
const briefDecisions = 5

// Who made a decision (Event.From for KindDecision).
const (
	ByLead = "lead"
	ByUser = "user"
)

// DecisionInfo is one recorded decision (KindDecision).
type DecisionInfo struct {
	Phase Phase     `json:"phase,omitempty"`
	By    string    `json:"by,omitempty"` // ByLead or ByUser
	Topic string    `json:"topic,omitempty"`
	Note  string    `json:"note"`
	Ref   string    `json:"ref,omitempty"`
	At    time.Time `json:"at"`
}

// askAliases maps the extra checkpoint names to phases. "approve" keeps the
// old --approve meaning (the plan); "pr" is the push and PR in product.
var askAliases = map[string]Phase{"approve": PhasePlan, "approval": PhasePlan, "pr": PhaseProduct}

// ParseAsk parses a checkpoint list (WORKFLOW §2.7): phase names, the
// aliases approve/approval (plan) and pr (product), "all", or "none". Items
// are separated by commas or spaces and case is ignored. The result is in
// workflow order without duplicates; an empty list or "none" yields nil.
// Unknown items are reported in the error and left out of the result.
func ParseAsk(s string) ([]Phase, error) {
	set := map[Phase]bool{}
	var unknown []string
	for _, f := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return r == ',' || r == ' ' || r == '\t' }) {
		switch {
		case f == "none":
		case f == "all":
			for _, p := range Phases {
				set[p] = true
			}
		case Phase(f).Index() >= 0:
			set[Phase(f)] = true
		case askAliases[f] != "":
			set[askAliases[f]] = true
		default:
			unknown = append(unknown, f)
		}
	}
	var out []Phase
	for _, p := range Phases {
		if set[p] {
			out = append(out, p)
		}
	}
	if len(unknown) > 0 {
		return out, fmt.Errorf("unknown checkpoint %s (want phase names %v, approve, pr, all or none)", strings.Join(unknown, ", "), Phases)
	}
	return out, nil
}

// FormatAsk renders a checkpoint list for an ask event and for people: the
// phases joined by commas, or "none".
func FormatAsk(ps []Phase) string {
	if len(ps) == 0 {
		return "none"
	}
	ss := make([]string, len(ps))
	for i, p := range ps {
		ss[i] = string(p)
	}
	return strings.Join(ss, ",")
}

// Asks reports whether the lead stops for the user at phase p.
func Asks(s State, p Phase) bool { return slices.Contains(s.Ask, p) }

// OpenFailures returns, per verdict name, the latest verdict when it is
// FAILED or CHANGES: the work the run still owes. Order is first-seen.
func OpenFailures(s State) []VerdictInfo {
	latest := map[string]VerdictInfo{}
	var order []string
	for _, v := range s.Verdicts {
		if _, ok := latest[v.Name]; !ok {
			order = append(order, v.Name)
		}
		latest[v.Name] = v
	}
	var out []VerdictInfo
	for _, n := range order {
		if v := latest[n]; v.Status == StatusFailed || v.Status == StatusChanges {
			out = append(out, v)
		}
	}
	return out
}

// Running returns the lanes still running.
func Running(s State) []Lane {
	var out []Lane
	for _, l := range s.Lanes {
		if l.Running {
			out = append(out, l)
		}
	}
	return out
}

// Next is the lead's next step for the state, in one line (WORKFLOW §2.7).
// It is a reminder derived from the events, not a decision: the lead still
// reads the phase's outputs before acting.
func Next(s State) string {
	if s.Ended {
		return "the run has ended (" + s.Outcome + "); start a new one for new work"
	}
	if n := len(Running(s)); n > 0 {
		return fmt.Sprintf("wait for the %d running agent(s), then act on their reports", n)
	}
	open := OpenFailures(s)
	switch s.Phase {
	case "", PhaseIntake:
		return "write charter.md, record the checkpoints (loops run ask), then enter research"
	case PhaseResearch:
		return "when the four research outputs exist, enter plan" + gateSuffix(s, PhaseResearch)
	case PhasePlan:
		return nextPlan(s)
	case PhaseImplement:
		return "every lane has reported: enter qa and verify each report" + gateSuffix(s, PhaseImplement)
	case PhaseQA:
		return nextQA(s, open)
	case PhaseProduct:
		if len(open) > 0 {
			return fmt.Sprintf("%s is %s: send its proof to the owning lane (product round %d of %d)", open[0].Name, open[0].Status, productRounds(s)+1, MaxProductRounds)
		}
		if s.PR != "" {
			return "the PR is open: enter reflect" + gateSuffix(s, PhaseProduct)
		}
		return "have the product-reviewer prove every AC, then open the PR" + gateSuffix(s, PhaseProduct)
	case PhaseReflect:
		return "when reflection.md is written: loops run end --outcome done" + gateSuffix(s, PhaseReflect)
	}
	return "continue the phase"
}

func nextPlan(s State) string {
	last := ""
	for _, v := range s.Verdicts {
		if v.Name == "plan" {
			last = v.Status
		}
	}
	switch {
	case last == StatusApprove:
		return "the plan is approved: extract the lanes and enter implement" + gateSuffix(s, PhasePlan)
	case last == StatusChanges && s.PlanRounds >= MaxPlanRounds:
		if Asks(s, PhasePlan) {
			return "plan rounds exhausted: ask the user about the open blocking items"
		}
		return "plan rounds exhausted: rule on each open blocking item yourself (loops run decide), then have the planner apply the rulings"
	case last == StatusChanges:
		return fmt.Sprintf("send plan-review.r%d.md back to the same planner (round %d of %d)", s.PlanRounds, s.PlanRounds+1, MaxPlanRounds)
	}
	return "spawn the planner, then the plan-reviewer"
}

func nextQA(s State, open []VerdictInfo) string {
	if len(open) > 0 {
		v := open[0]
		switch {
		case v.Round >= MaxQARounds && Asks(s, PhaseQA):
			return v.Name + " failed " + fmt.Sprint(v.Round) + " rounds: ask the user"
		case v.Round >= MaxQARounds:
			return v.Name + " failed " + fmt.Sprint(v.Round) + " rounds: decide yourself (re-plan the lane, move the claim to Open items, or end the run failed) and record it with loops run decide"
		case v.Round >= EscalateAfter:
			return v.Name + " failed twice: the author states the premise (ENGINEERING §11); relaunch it one model tier up"
		}
		return "send " + v.Name + " back to its author with the QA file (round " + fmt.Sprint(v.Round+1) + ")"
	}
	for _, g := range s.Gates {
		if g.Name == GateAll && g.OK && !s.SourceDirty {
			return "the gate is green: enter product" + gateSuffix(s, PhaseQA)
		}
	}
	return "every claim is verified: run the quality pass, then loops gate"
}

func productRounds(s State) int {
	n := 0
	for _, lb := range s.LoopBacks {
		if lb.Phase == PhaseProduct {
			n++
		}
	}
	return n
}

// gateSuffix is the checkpoint suffix for leaving phase p.
func gateSuffix(s State, p Phase) string {
	if Asks(s, p) {
		return " (checkpoint: ask the user first)"
	}
	return ""
}

// Brief renders the lead's re-anchor (`loops run brief`, the session-start
// hook): who it is, the scope files, where the run stands, what is owed and
// the next step. dir is the run directory.
func Brief(s State, dir string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are the lead (master agent) of loops run %s: %q", s.ID, s.Title)
	if s.Ticket != "" {
		fmt.Fprintf(&b, ", ticket %s", s.Ticket)
	}
	b.WriteString(".\n")
	if dir != "" {
		fmt.Fprintf(&b, "Scope: %s/charter.md (goal, ACs, scope, decisions); plan: the latest %s/plan.v<N>.md.\n", dir, dir)
	}
	fmt.Fprintf(&b, "Phase: %s%s.\n", s.Phase, phasePath(s.PhaseLog))
	if len(s.Ask) == 0 {
		b.WriteString("Checkpoints: none. You decide every phase yourself and record each decision (loops run decide).\n")
	} else {
		fmt.Fprintf(&b, "Checkpoints: %s. Ask the user there; decide everything else yourself.\n", FormatAsk(s.Ask))
	}
	if r := Running(s); len(r) > 0 {
		names := make([]string, len(r))
		for i, l := range r {
			names[i] = laneName(l)
		}
		fmt.Fprintf(&b, "Running: %s.\n", strings.Join(names, ", "))
	}
	ver, failed := verdictCounts(s.Verdicts)
	fmt.Fprintf(&b, "Verdicts: %d verified, %d open failures; loop-backs %d.\n", ver, failed, len(s.LoopBacks))
	for i, v := range OpenFailures(s) {
		if i == 3 {
			b.WriteString("  …\n")
			break
		}
		fmt.Fprintf(&b, "  open: %s %s (round %d, %s)\n", v.Name, v.Status, v.Round, v.Ref)
	}
	for _, g := range s.Gates {
		if g.Name == GateAll {
			state := "red"
			if g.OK && !s.SourceDirty {
				state = "green"
			} else if g.OK {
				state = "green, but sources changed since"
			}
			fmt.Fprintf(&b, "Gate: %s.\n", state)
		}
	}
	if s.PR != "" {
		fmt.Fprintf(&b, "PR: %s.\n", s.PR)
	}
	if len(s.Decisions) > 0 {
		b.WriteString("Decisions (latest last):\n")
		ds := s.Decisions
		if len(ds) > briefDecisions {
			ds = ds[len(ds)-briefDecisions:]
		}
		for _, d := range ds {
			fmt.Fprintf(&b, "  [%s] %s: %s: %s\n", d.Phase, d.By, d.Topic, d.Note)
		}
	}
	fmt.Fprintf(&b, "Next: %s.\n", Next(s))
	return b.String()
}

func phasePath(log []PhaseEntry) string {
	if len(log) < 2 {
		return ""
	}
	ps := make([]string, len(log))
	for i, e := range log {
		ps[i] = string(e.Phase)
	}
	return " (path " + strings.Join(ps, " → ") + ")"
}

func laneName(l Lane) string {
	if l.Label != "" {
		return l.Label
	}
	if l.Role != "" {
		return l.Role
	}
	return l.Agent
}

// verdictCounts counts latest verdicts per name: verified, and open failures.
func verdictCounts(vs []VerdictInfo) (verified, failed int) {
	latest := map[string]string{}
	for _, v := range vs {
		latest[v.Name] = v.Status
	}
	for _, st := range latest {
		switch st {
		case StatusVerified, StatusApprove:
			verified++
		case StatusFailed, StatusChanges:
			failed++
		}
	}
	return verified, failed
}
