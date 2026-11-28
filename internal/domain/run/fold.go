package run

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/pack"
)

// Fold replays events in order into a State. Events for other runs are
// ignored once the first KindRun event has set the ID. Fold is total: it
// never fails on unknown kinds or missing fields.
func Fold(events []Event) State {
	var s State
	var ck checkTrack
	for _, e := range events {
		if s.ID != "" && e.Run != "" && e.Run != s.ID {
			continue
		}
		if e.Time.After(s.Updated) {
			s.Updated = e.Time
		}
		switch e.Kind {
		case KindRun:
			if s.ID == "" {
				s.ID = e.Run
				s.Title = e.Name
				s.Ticket = e.Target
				s.Source = e.Note
				s.Started = e.Time
			}
		case KindPhase:
			s.Phase = e.Phase
			s.PhaseLog = append(s.PhaseLog, PhaseEntry{Phase: e.Phase, At: e.Time, Note: e.Note})
		case KindAgentStart:
			l := lane(&s, e.Agent)
			l.Role, l.Model, l.Wave = e.Role, e.Model, e.Wave
			l.Phase = s.Phase
			l.Running = true
			l.OK = nil
			l.Started = e.Time
		case KindAgentStop:
			l := lane(&s, e.Agent)
			if e.Role != "" && l.Role == "" {
				l.Role = e.Role
			}
			l.Running = false
			l.OK = e.OK
			l.Stopped = e.Time
			foldTokens(&s, l, e.Tokens)
		case KindLabel:
			l := lane(&s, e.Agent)
			l.Label = e.Name
			if e.Role != "" {
				l.Role = e.Role
			}
			if e.Wave != 0 {
				l.Wave = e.Wave
			}
			if e.Model != "" {
				l.Model = e.Model // the model actually used, e.g. after a fallback
			}
		case KindStep:
			foldStep(&s, e)
			if isEdit(e.Name) && (pack.LooksLikeSource(e.Target) || strings.HasSuffix(e.Target, ".go")) {
				ck.edit(e.Target, e.Time)
			}
		case KindGate:
			foldGate(&s, e)
			ck.gate(e)
		case KindVerdict:
			foldVerdict(&s, e)
		case KindLoopBack:
			s.LoopBacks = append(s.LoopBacks, LoopBack{From: e.From, To: e.To, Phase: e.Phase, Round: e.Round, Reason: e.Note, At: e.Time})
		case KindProof:
			s.Proofs = append(s.Proofs, ProofInfo{Name: e.Name, Ref: e.Ref, Note: e.Note, At: e.Time})
		case KindGuardDeny:
			s.Denials++
		case KindMemory:
			switch e.Name {
			case "write":
				s.MemoryW++
			case "query", "briefing":
				s.MemoryQ++
			}
		case KindOverride:
			s.Overrides = append(s.Overrides, OverrideInfo{LoopsRule: e.Name, ProjectRule: e.Target, Note: e.Note, Agent: e.Agent, Role: e.Role, Phase: vphase(&s, e), At: e.Time})
		case KindLimits:
			var l Limits
			if json.Unmarshal([]byte(e.Note), &l) == nil {
				s.Limits = l
				s.LimitsAt = e.Time
			}
		case KindBudget:
			s.Budgets = append(s.Budgets, BudgetInfo{Phase: vphase(&s, e), Role: e.Role, Limit: e.Name, Reason: e.Note, At: e.Time})
		case KindAsk:
			s.Ask, _ = ParseAsk(e.Note) // recorded by `loops run ask`, which validates; unknown names are dropped
		case KindDecision:
			s.Decisions = append(s.Decisions, DecisionInfo{Phase: vphase(&s, e), By: e.From, Topic: e.Name, Note: e.Note, Ref: e.Ref, At: e.Time})
		case KindTriage:
			var t Triage
			if json.Unmarshal([]byte(e.Note), &t) == nil {
				s.Triage = t
			}
		case KindNote:
			if e.Name == NoteGateStart {
				s.GateRunning, s.GateStarted = true, e.Time
			}
			if e.Name == NoteLeadTokens && e.Tokens > s.LeadTokens {
				s.LeadTokens = e.Tokens
			}
		case KindPR:
			s.PR = e.Ref
		case KindEnd:
			s.Ended = true
			s.Outcome = e.Status
		}
	}
	s.PlanRounds = planRounds(s.Verdicts)
	laneQA(&s)
	ck.apply(&s)
	return s
}

// foldTokens applies an agent_stop token count. A transcript only grows (a
// resumed agent keeps its id and transcript), so the count is the lane's
// cumulative total: the lane keeps the highest one seen and only the growth
// is added to PhaseTokens, under the lane's phase (the current phase when
// the lane was never opened).
func foldTokens(s *State, l *Lane, tokens int64) {
	if tokens <= l.Tokens {
		return
	}
	delta := tokens - l.Tokens
	l.Tokens = tokens
	p := l.Phase
	if p == "" {
		p = s.Phase
	}
	if p == "" {
		return
	}
	if s.PhaseTokens == nil {
		s.PhaseTokens = map[Phase]int64{}
	}
	s.PhaseTokens[p] += delta
}

func planRounds(vs []VerdictInfo) int {
	n := 0
	for _, v := range vs {
		if v.Name == "plan" && (v.Status == StatusChanges || v.Status == StatusApprove) {
			n++
		}
	}
	return n
}

// lane returns the lane for agent id, creating it (not running) if absent.
func lane(s *State, id string) *Lane {
	for i := range s.Lanes {
		if s.Lanes[i].Agent == id {
			return &s.Lanes[i]
		}
	}
	s.Lanes = append(s.Lanes, Lane{Agent: id})
	return &s.Lanes[len(s.Lanes)-1]
}

func foldStep(s *State, e Event) {
	ok := e.OK == nil || *e.OK
	s.Steps++
	if !ok {
		s.Errors++
	}
	role := ""
	if e.Agent == "" {
		s.LeadStep++
	} else {
		l := lane(s, e.Agent)
		l.Steps++
		if !ok {
			l.Errors++
		}
		role = l.Role
		if role == "" {
			role = e.Role
		}
	}
	s.Recent = append(s.Recent, StepInfo{Agent: e.Agent, Role: role, Tool: e.Name, Target: e.Target, OK: ok, DurationMS: e.DurationMS, At: e.Time})
	if len(s.Recent) > RecentSteps {
		s.Recent = append([]StepInfo(nil), s.Recent[len(s.Recent)-RecentSteps:]...)
	}
	if isEdit(e.Name) {
		if strings.HasSuffix(e.Target, ".go") {
			s.GoDirty = true
			s.LastGoChange = e.Time
		}
		if pack.LooksLikeSource(e.Target) {
			s.SourceDirty = true
			s.LastSourceChange = e.Time
		}
	}
}

// isEdit reports whether tool is a file-editing tool.
func isEdit(tool string) bool {
	switch tool {
	case "Edit", "Write", "MultiEdit", "NotebookEdit":
		return true
	}
	return false
}

func foldGate(s *State, e Event) {
	if e.Name == GateAll {
		s.GateRunning = false
	}
	g := GateResult{Name: e.Name, OK: e.OK != nil && *e.OK, At: e.Time, DurationMS: e.DurationMS, Note: e.Note}
	found := false
	for i := range s.Gates {
		if s.Gates[i].Name == e.Name {
			s.Gates[i] = g
			found = true
			break
		}
	}
	if !found {
		s.Gates = append(s.Gates, g)
	}
	// A green "all" is stamped with the pipeline start: it only clears edits
	// made before it began.
	if e.Name == GateAll && g.OK {
		if !s.LastGoChange.After(e.Time) {
			s.GoDirty = false
		}
		if !s.LastSourceChange.After(e.Time) {
			s.SourceDirty = false
		}
		s.LastGreen = e.Time
	}
}

func foldVerdict(s *State, e Event) {
	s.Verdicts = append(s.Verdicts, VerdictInfo{Name: e.Name, Status: e.Status, Agent: e.Agent, Role: e.Role, Phase: vphase(s, e), Round: e.Round, Ref: e.Ref, At: e.Time})
}

// laneQA recomputes each lane's QA status from all verdicts: FAILED if any
// claim's latest status is FAILED, else VERIFIED if the lane has a verdict.
func laneQA(s *State) {
	for i := range s.Lanes {
		l := &s.Lanes[i]
		l.QA, l.QARounds = "", 0
		latest := map[string]string{}
		for _, v := range s.Verdicts {
			if !(l.Label != "" && strings.HasPrefix(v.Name, l.Label+":")) && !strings.HasPrefix(v.Name, l.Agent+":") {
				continue
			}
			latest[v.Name] = v.Status
			if v.Round > l.QARounds {
				l.QARounds = v.Round
			}
		}
		for _, st := range latest {
			if st == StatusFailed {
				l.QA = StatusFailed
				break
			}
			l.QA = StatusVerified
		}
	}
}

// StopReason returns a non-empty reason when the lead must not stop yet:
// the run is active (not Ended), SourceDirty (or GoDirty, for states built
// before SourceDirty existed) is set, and no lane is Running. Running lanes
// mean the lead is legitimately waiting for background agents, so it may end
// its turn. The full gate runs once per run (WORKFLOW §6), so the reason
// asks for it only when none has checked the current tree: after a green
// one it names the scoped gate that covers the changed files, and after a
// red one on an unchanged tree it says to fix the failures instead.
func StopReason(s State) string {
	if s.Ended || !s.GoDirty && !s.SourceDirty {
		return ""
	}
	for _, l := range s.Lanes {
		if l.Running {
			return ""
		}
	}
	if s.GateRunning && s.Updated.Sub(s.GateStarted) < GateRunningMax {
		return "" // the lead is waiting for its full gate, as for a running lane
	}
	if s.FileChecksOnly && !s.Rechecked {
		return "loops: the full gate failed only per-file checks (coverage, arch), which a scoped gate judges the same way on the run's files. Send the failures in the run's files to their owners (`loops run loopback --from gate --to <id>`); then re-check " + listFiles(s.Unchecked) + " with `loops gate --pkgs " + ScopeArg(s.Unchecked) + "` (every check, scoped), never the full gate again. Or `loops run end --outcome failed`."
	}
	if len(s.Unchecked) > 0 {
		return "loops: source files changed after the run's green full gate (" + listFiles(s.Unchecked) + "). The full gate runs once per run: re-check them with `loops gate --pkgs " + ScopeArg(s.Unchecked) + "` (every check, scoped; or each owner re-runs its scope), never the full gate again. Or `loops run end --outcome abandoned` to stop the run."
	}
	if beforeQATail(s) {
		return "loops: source files changed and the run is not finished: follow `loops run next`. Agents check only their own packages (`loops gate --pkgs <owned>`); the full gate (`loops gate`) runs once per run, at the QA tail, not now. Or `loops run end --outcome abandoned` to stop the run."
	}
	if redUnchanged(s) {
		return "loops: source files changed since the last green `loops gate`, and the full gate is red on this tree: running it again changes nothing. Send each failure to its owning lane (`loops run loopback --from gate --to <id>`), re-check the fixes with `loops gate --pkgs <their packages>`, then run the full gate once more; or `loops run end --outcome failed`."
	}
	return "loops: source files changed since the last green `loops gate` (build, lint, tests with the race policy, coverage). Run `loops gate` and fix failures before finishing, or `loops run end --outcome abandoned` to stop the run."
}

// listFiles renders at most three files, then a count of the rest.
func listFiles(fs []string) string {
	if len(fs) <= 3 {
		return strings.Join(fs, ", ")
	}
	return strings.Join(fs[:3], ", ") + fmt.Sprintf(" and %d more", len(fs)-3)
}

// EndReason returns a non-empty reason when the run must not end "done":
// a source file changed since the last green full gate (the Stop gate's
// condition, without its running-lane exemption: ending the run is final),
// or the latest full-gate ("all") or coverage result is red. A run with no
// gate result and no source change (no detected pack, a docs-only run) may
// end done.
func EndReason(s State) string {
	if len(s.Unchecked) > 0 && (s.GoDirty || s.SourceDirty) {
		return "source files changed after the green full gate and no green scoped gate covers them (" + listFiles(s.Unchecked) + "); run `loops gate --pkgs " + ScopeArg(s.Unchecked) + "` first"
	}
	if s.GoDirty || s.SourceDirty {
		return "source files changed since the last green `loops gate`; run `loops gate` and fix failures first"
	}
	for _, g := range s.Gates {
		if g.Name == GateAll && !g.OK && s.Rechecked {
			continue // its only failure, coverage, was re-checked on every edited file
		}
		if (g.Name == GateAll || g.Name == GateCoverage) && !g.OK {
			return "the last `loops gate` " + g.Name + " result is red; fix it and run `loops gate` again"
		}
	}
	return ""
}

func vphase(s *State, e Event) Phase {
	if e.Phase != "" {
		return e.Phase
	}
	return s.Phase
}
