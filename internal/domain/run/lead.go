package run

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// briefDecisions is how many recent decisions Brief shows.
const briefDecisions = 5

// The memory-store decision (WORKFLOW §7): the lead records it with
// `loops run decide --topic memory-store --note "files: <why>"`.
const (
	MemoryStoreTopic = "memory-store"
	MemoryStoreFiles = "files"
)

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
		return nextIntake(s)
	case PhaseResearch:
		if n := researchRuns(s); n < len(researchSteps) {
			return fmt.Sprintf("when the %d research output(s) triage kept exist, enter plan", n) + gateSuffix(s, PhaseResearch)
		}
		return "when the four research outputs exist, enter plan" + gateSuffix(s, PhaseResearch)
	case PhasePlan:
		return nextPlan(s)
	case PhaseImplement:
		if len(s.Triage.Skip) > 0 && !s.Triage.Recheck {
			return "every lane has reported: recheck the triage against the real diff (loops run triage --recheck --files N), then enter qa" + gateSuffix(s, PhaseImplement)
		}
		return "every lane has reported: enter qa and verify each report" + gateSuffix(s, PhaseImplement)
	case PhaseQA:
		return nextQA(s, open)
	case PhaseProduct:
		return nextProduct(s, open)
	case PhaseReflect:
		if reflectFirst(s) && !visited(s, PhaseProduct) {
			return "when reflection.md and the memory pages are written: enter product (the PR commit carries the pages)" + gateSuffix(s, PhaseReflect)
		}
		if Skips(s, StepReflect) && s.Triage.Tier == TierTrivial {
			return "trivial run: no reflection.md; put the outcome, decisions and overrides in your summary (post overrides.md if an override was recorded), then loops run end --outcome done" + gateSuffix(s, PhaseReflect)
		}
		if Skips(s, StepReflect) {
			return "triage skipped the reflector: write a short reflection.md yourself (outcome, triage, decisions, overrides), then loops run end --outcome done" + gateSuffix(s, PhaseReflect)
		}
		return "when reflection.md is written: loops run end --outcome done" + gateSuffix(s, PhaseReflect)
	}
	return "continue the phase"
}

func nextIntake(s State) string {
	charter := "write charter.md"
	if s.Triage.Tier == TierTrivial {
		charter = "write a short charter.md (the goal and the ACs only)"
	}
	switch {
	case s.Triage.Tier == "":
		return "triage the request (loops run triage), write charter.md, record the checkpoints (loops run ask), then enter research"
	case researchSkipped(s):
		return charter + ", record the checkpoints (loops run ask), run loops lang detect --write, then enter plan: triage skipped research"
	}
	return charter + ", record the checkpoints (loops run ask), then enter research"
}

// PhaseRefusal is why `loops run phase to` must not be recorded, or "". A run
// leaves intake only once it is triaged (WORKFLOW §2.8), so no run silently
// pays for the full workflow; runs already past intake are never refused.
func PhaseRefusal(s State, to Phase) string {
	if s.Triage.Tier != "" || to.Index() <= PhaseIntake.Index() || s.Phase.Index() > PhaseIntake.Index() {
		return ""
	}
	return "the run is not triaged: run loops run triage --kind <docs|typo|config|test|bugfix|feature|refactor|upgrade> --files <estimate> [--touches …] [--unclear] (or --full) first (WORKFLOW §2.8)"
}

func nextProduct(s State, open []VerdictInfo) string {
	if len(open) > 0 {
		return fmt.Sprintf("%s is %s: send its proof to the owning lane (product round %d of %d)", open[0].Name, open[0].Status, productRounds(s)+1, MaxProductRounds)
	}
	if s.PR != "" && reflectFirst(s) && visited(s, PhaseReflect) {
		return "the PR is open and reflect ran: loops run end --outcome done" + gateSuffix(s, PhaseProduct)
	}
	if s.PR != "" {
		return "the PR is open: enter reflect" + gateSuffix(s, PhaseProduct)
	}
	if Skips(s, StepProductProof) {
		return "triage skipped the manual proof: write pr.md mapping each AC to its test or gate evidence, then branch, commit, push and open the PR yourself" + gateSuffix(s, PhaseProduct)
	}
	return "have the product-reviewer prove every AC, then open the PR" + gateSuffix(s, PhaseProduct)
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
	case Skips(s, StepPlanner):
		return "triage skipped the planner: write a short plan.v1.md yourself (one lane per AC), run loops plan check on it, then enter implement" + gateSuffix(s, PhasePlan)
	case Skips(s, StepPlanReview):
		return "spawn the planner (triage skipped the plan-reviewer); when plan.v1.md passes loops plan check, enter implement" + gateSuffix(s, PhasePlan)
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
	if g, ok := latestAll(s); ok {
		switch {
		case GateGreen(s):
			if reflectFirst(s) && !visited(s, PhaseReflect) {
				return "the gate is green: enter reflect (file store: the memory pages go in the PR commit)" + gateSuffix(s, PhaseQA)
			}
			if !Skips(s, StepProductProof) {
				return "the gate is green: enter product and resume the product-reviewer with Gate: green and the lanes changed since it started" + gateSuffix(s, PhaseQA)
			}
			return "the gate is green: enter product" + gateSuffix(s, PhaseQA)
		case s.FileChecksOnly:
			return "the full gate failed only per-file checks (coverage, arch), which a scoped gate judges the same way: send the failures in the run's files to their owners (loops run loopback --from gate --to <id>); when they report, re-check with loops gate --pkgs " + ScopeArg(s.Unchecked) + ", never the full gate again"
		case g.OK && len(s.Unchecked) > 0:
			return "the full gate ran (green) and runs once per run: when the fixing lanes report, re-check what changed after it with loops gate --pkgs " + ScopeArg(s.Unchecked) + " (each owner's scope run counts), never the full gate again"
		case !g.OK && redUnchanged(s):
			return "the full gate is red: send each failure to its owning lane (loops run loopback --from gate --to <id>); when the fixes are re-verified, run loops gate once more (never on an unchanged tree)"
		}
	}
	return qaTail(s)
}

// qaTail is the QA tail (WORKFLOW §2.4): the full gate, the quality pass and
// the security pass start together when the last lane reports, and product
// proofs start once every lane is VERIFIED.
func qaTail(s State) string {
	prefix := "when the last lane reports, "
	if Skips(s, StepQAVerify) {
		prefix = "triage skipped the verifiers: "
	}
	todo := []string{"loops gate in the background"}
	if !Skips(s, StepQuality) {
		todo = append(todo, "the quality pass")
	}
	if !Skips(s, StepSecurity) {
		todo = append(todo, "the security pass")
	}
	text := prefix + "run loops gate"
	if len(todo) > 1 {
		text = prefix + "start together " + strings.Join(todo, ", ")
	}
	if !Skips(s, StepProductProof) {
		text += "; once every lane is VERIFIED, spawn the product-reviewer with Gate: pending (stay in qa)"
	}
	return text
}

// reflectFirst reports whether the latest memory-store decision picked the
// file store, which runs reflect before product. A skipped reflector writes
// no pages, so the run keeps the default order.
func reflectFirst(s State) bool {
	if Skips(s, StepReflect) {
		return false
	}
	for i := len(s.Decisions) - 1; i >= 0; i-- {
		d := s.Decisions[i]
		if d.Topic != MemoryStoreTopic {
			continue
		}
		f := strings.Fields(d.Note)
		return len(f) > 0 && strings.TrimRight(f[0], ":,") == MemoryStoreFiles
	}
	return false
}

// visited reports whether the run has entered phase p.
func visited(s State, p Phase) bool {
	for _, e := range s.PhaseLog {
		if e.Phase == p {
			return true
		}
	}
	return false
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
	if t := s.Triage; t.Tier != "" {
		fmt.Fprintf(&b, "Triage: %s (%s); skipped: %s; loops gate always runs.\n", t.Tier, t.Signals, JoinSteps(Skipped(s)))
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
			switch {
			case GateGreen(s) && !g.OK:
				state = "green (the full gate's per-file failures re-checked on every edited file)"
			case GateGreen(s):
				state = "green"
			case s.FileChecksOnly:
				state = "red on per-file checks only: re-check with loops gate --pkgs " + ScopeArg(s.Unchecked) + " after the fixes, not the full gate"
			case g.OK && len(s.Unchecked) > 0:
				state = "green, but sources changed since: re-check with loops gate --pkgs " + ScopeArg(s.Unchecked) + ", not the full gate"
			case g.OK:
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
