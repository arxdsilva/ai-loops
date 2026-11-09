package run

import "time"

// State is the folded view of a run's events: what the status line, the mod
// pane, the HTML report and the Stop gate read. It is derived, never stored
// as the source of truth (events.jsonl is).
type State struct {
	ID      string    `json:"id"`
	Title   string    `json:"title"`
	Ticket  string    `json:"ticket,omitempty"`
	Source  string    `json:"source,omitempty"` // github | gitlab | jira | linear | text
	Started time.Time `json:"started"`
	Updated time.Time `json:"updated"`

	Phase      Phase         `json:"phase"`
	PhaseLog   []PhaseEntry  `json:"phase_log"`  // every phase entry in order (a phase can be re-entered)
	Lanes      []Lane        `json:"lanes"`      // agents in first-seen order
	Gates      []GateResult  `json:"gates"`      // latest result per gate name, first-seen order
	Verdicts   []VerdictInfo `json:"verdicts"`   // every verdict in order
	LoopBacks  []LoopBack    `json:"loop_backs"` // every loop-back in order
	Proofs     []ProofInfo   `json:"proofs"`
	Recent     []StepInfo    `json:"recent"` // last RecentSteps steps, oldest first
	PlanRounds int           `json:"plan_rounds"`

	Steps    int `json:"steps"`
	Errors   int `json:"errors"` // steps with OK=false
	Denials  int `json:"denials"`
	MemoryQ  int `json:"memory_queries"`
	MemoryW  int `json:"memory_writes"`
	LeadStep int `json:"lead_steps"` // steps by the main session (Agent == "")

	// GoDirty is true when a .go file was edited (Edit/Write/MultiEdit step on
	// a *.go target) after the last passing GateAll.
	GoDirty      bool      `json:"go_dirty"`
	LastGreen    time.Time `json:"last_green,omitzero"`
	LastGoChange time.Time `json:"last_go_change,omitzero"`

	// Overrides are project rules that superseded loops rules (WORKFLOW §1).
	Overrides []OverrideInfo `json:"overrides"`
	// Limits are the opt-in limits in force (zero = none), from the last KindLimits event.
	Limits Limits `json:"limits"`
	// LimitsAt is when the last KindLimits event was recorded (zero = never);
	// a newer limits event lifts a budget pause.
	LimitsAt time.Time `json:"limits_at,omitzero"`
	// PhaseTokens sums agent_stop Tokens per phase (lane phase at open).
	PhaseTokens map[Phase]int64 `json:"phase_tokens,omitempty"`
	// Budgets lists blocked spawns (KindBudget), in order.
	Budgets []BudgetInfo `json:"budgets,omitempty"`

	PR      string `json:"pr,omitempty"`
	Ended   bool   `json:"ended"`
	Outcome string `json:"outcome,omitempty"`
}

// RecentSteps is how many steps State.Recent keeps.
const RecentSteps = 12

// PhaseEntry records when a phase was entered.
type PhaseEntry struct {
	Phase Phase     `json:"phase"`
	At    time.Time `json:"at"`
	Note  string    `json:"note,omitempty"`
}

// Lane is one agent working in the run.
type Lane struct {
	Agent   string    `json:"agent"`           // Claude agent id
	Label   string    `json:"label,omitempty"` // plan agent id from a KindLabel event ("W1-domain")
	Role    string    `json:"role"`
	Model   string    `json:"model,omitempty"`
	Wave    int       `json:"wave,omitempty"`
	Phase   Phase     `json:"phase,omitempty"` // phase current when the lane opened
	Running bool      `json:"running"`
	OK      *bool     `json:"ok,omitempty"`
	Steps   int       `json:"steps"`
	Errors  int       `json:"errors"`
	Started time.Time `json:"started"`
	Stopped time.Time `json:"stopped,omitzero"`
	// QA is the lane's latest QA verdict status ("", VERIFIED, FAILED) and
	// QARounds the highest Round among those verdicts. A verdict targets a
	// lane when its Name starts with "<Label>:" or "<Agent>:" (claim ids are
	// "<plan agent id>:C<n>").
	QA       string `json:"qa,omitempty"`
	QARounds int    `json:"qa_rounds,omitempty"`
	Tokens   int64  `json:"tokens,omitempty"`
}

// GateResult is the latest result of one gate check.
type GateResult struct {
	Name       string    `json:"name"`
	OK         bool      `json:"ok"`
	At         time.Time `json:"at"`
	DurationMS int64     `json:"ms,omitempty"`
	Note       string    `json:"note,omitempty"`
}

// VerdictInfo is one verdict on a plan, claim or acceptance criterion.
type VerdictInfo struct {
	Name   string    `json:"name"`
	Status string    `json:"status"`
	Agent  string    `json:"agent,omitempty"`
	Role   string    `json:"role,omitempty"`
	Phase  Phase     `json:"phase,omitempty"`
	Round  int       `json:"round,omitempty"`
	Ref    string    `json:"ref,omitempty"`
	At     time.Time `json:"at"`
}

// LoopBack is work sent back from a reviewer to its author.
type LoopBack struct {
	From   string    `json:"from"`
	To     string    `json:"to"`
	Phase  Phase     `json:"phase,omitempty"`
	Round  int       `json:"round,omitempty"`
	Reason string    `json:"reason,omitempty"`
	At     time.Time `json:"at"`
}

// ProofInfo is one captured proof.
type ProofInfo struct {
	Name string    `json:"name"`
	Ref  string    `json:"ref"`
	Note string    `json:"note,omitempty"`
	At   time.Time `json:"at"`
}

// StepInfo is one recorded tool call.
type StepInfo struct {
	Agent      string    `json:"agent,omitempty"`
	Role       string    `json:"role,omitempty"`
	Tool       string    `json:"tool"`
	Target     string    `json:"target,omitempty"`
	OK         bool      `json:"ok"`
	DurationMS int64     `json:"ms,omitempty"`
	At         time.Time `json:"at"`
}

// OverrideInfo is one project rule that superseded a loops rule.
type OverrideInfo struct {
	LoopsRule   string    `json:"loops_rule"`
	ProjectRule string    `json:"project_rule"`
	Note        string    `json:"note,omitempty"`
	Agent       string    `json:"agent,omitempty"`
	Role        string    `json:"role,omitempty"`
	Phase       Phase     `json:"phase,omitempty"`
	At          time.Time `json:"at"`
}

// BudgetInfo is one spawn blocked by a limit.
type BudgetInfo struct {
	Phase  Phase     `json:"phase"`
	Role   string    `json:"role,omitempty"`
	Limit  string    `json:"limit"`
	Reason string    `json:"reason"`
	At     time.Time `json:"at"`
}
