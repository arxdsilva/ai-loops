package run

import (
	"encoding/json"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/pack"
)

// Fold replays events in order into a State. Events for other runs are
// ignored once the first KindRun event has set the ID. Fold is total: it
// never fails on unknown kinds or missing fields.
func Fold(events []Event) State {
	var s State
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
		case KindGate:
			foldGate(&s, e)
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
		case KindPR:
			s.PR = e.Ref
		case KindEnd:
			s.Ended = true
			s.Outcome = e.Status
		}
	}
	s.PlanRounds = planRounds(s.Verdicts)
	laneQA(&s)
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
	switch e.Name {
	case "Edit", "Write", "MultiEdit", "NotebookEdit":
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

func foldGate(s *State, e Event) {
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
// its turn. The reason tells the lead to run `loops gate`.
func StopReason(s State) string {
	if s.Ended || !s.GoDirty && !s.SourceDirty {
		return ""
	}
	for _, l := range s.Lanes {
		if l.Running {
			return ""
		}
	}
	return "loops: source files changed since the last green `loops gate` (build, lint, tests with the race policy, coverage). Run `loops gate` and fix failures before finishing, or `loops run end --outcome abandoned` to stop the run."
}

func vphase(s *State, e Event) Phase {
	if e.Phase != "" {
		return e.Phase
	}
	return s.Phase
}
