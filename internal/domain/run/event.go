// Package run is the domain model of a loops run: the six-phase workflow a
// ticket goes through, the agents (role lanes) working in it, and the events
// that record every step. It is pure: no I/O, no wall clock (times arrive on
// events), no goroutines.
//
// The canonical description of phases, event kinds and loop limits is
// specs/WORKFLOW.md; this package is its executable form.
package run

import "time"

// Kind names an event type. The JSON value is part of the events.jsonl
// contract read by the status line, the mod pane and the HTML report: values
// are append-only, never renamed.
type Kind string

const (
	KindRun        Kind = "run"         // run started: Name=title, Target=ticket ref, Note=source
	KindPhase      Kind = "phase"       // phase entered: Phase, Note
	KindAgentStart Kind = "agent_start" // lane opened: Agent, Role, Model, Wave
	KindAgentStop  Kind = "agent_stop"  // lane closed: Agent, Role, OK
	KindStep       Kind = "step"        // tool call: Agent, Name=tool, Target, OK, DurationMS
	KindLoopBack   Kind = "loop_back"   // work sent back: From, To, Round, Note=reason, Phase
	KindVerdict    Kind = "verdict"     // claim/AC/plan verdict: Name=claim or AC id, Status, Agent, Round, Ref
	KindGate       Kind = "gate"        // quality gate result: Name=check ("build","vet",…,"all"), OK, Note, DurationMS
	KindProof      Kind = "proof"       // proof captured: Name=AC id, Ref=file in run dir, Note
	KindGuardDeny  Kind = "guard_deny"  // a hook denied a tool call: Name=guard, Agent, Note=reason
	KindMemory     Kind = "memory"      // ai-memory use: Name="query"|"write"|"briefing", Target, Note
	KindNote       Kind = "note"        // free annotation: Note
	KindPR         Kind = "pr"          // PR/MR opened: Ref=URL
	KindLabel      Kind = "label"       // names a lane: Agent=Claude agent id (from the Agent tool result), Name=plan agent id ("W1-domain"), Role, Wave
	KindOverride   Kind = "override"    // a project rule superseded a loops rule: Name=loops rule ("SOLID §9"), Target=project rule + file:line, Note=what was done, Agent
	KindLimits     Kind = "limits"      // limits in force for the run (opt-in): Note=JSON of run.Limits
	KindBudget     Kind = "budget"      // a spawn was blocked by a limit: Phase, Role, Name=limit ("tokens","agents","roles"), Note=reason
	KindEnd        Kind = "end"         // run finished: Status=outcome ("done","abandoned","failed")
)

// Verdict statuses (Event.Status for KindVerdict).
const (
	StatusApprove  = "APPROVE"  // plan-reviewer approved the plan
	StatusChanges  = "CHANGES"  // plan-reviewer requested changes
	StatusVerified = "VERIFIED" // QA verified a claim / product verified an AC
	StatusFailed   = "FAILED"   // QA or product found a claim / AC false
)

// Run outcomes (Event.Status for KindEnd).
const (
	OutcomeDone      = "done"
	OutcomeAbandoned = "abandoned"
	OutcomeFailed    = "failed"
)

// GateAll is the gate name recorded once the whole gate pipeline passes or fails.
const GateAll = "all"

// Event is one line of events.jsonl. Fields are optional per Kind (see the
// Kind constants). Unknown fields are ignored by readers so the schema can
// grow additively.
type Event struct {
	Time       time.Time `json:"t"`
	Kind       Kind      `json:"kind"`
	Run        string    `json:"run,omitempty"`
	Phase      Phase     `json:"phase,omitempty"`
	Agent      string    `json:"agent,omitempty"` // Claude Code agent_id; empty = lead (main session)
	Role       string    `json:"role,omitempty"`  // role name, e.g. "domain-engineer"
	Model      string    `json:"model,omitempty"`
	Wave       int       `json:"wave,omitempty"`
	Round      int       `json:"round,omitempty"`
	Name       string    `json:"name,omitempty"`
	Target     string    `json:"target,omitempty"`
	From       string    `json:"from,omitempty"`
	To         string    `json:"to,omitempty"`
	Status     string    `json:"status,omitempty"`
	OK         *bool     `json:"ok,omitempty"`
	DurationMS int64     `json:"ms,omitempty"`
	Ref        string    `json:"ref,omitempty"`
	Note       string    `json:"note,omitempty"`
	Pack       string    `json:"pack,omitempty"`   // gate: the language pack (and root) the check belongs to, when the project has several
	Tokens     int64     `json:"tokens,omitempty"` // agent_stop: tokens the agent used (from its transcript, when the platform exposes one)
}

// Bool returns a pointer to b, for Event.OK.
func Bool(b bool) *bool { return &b }
