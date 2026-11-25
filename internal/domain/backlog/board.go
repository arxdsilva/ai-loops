package backlog

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

// RecentShown is how many finished issues a Board lists.
const RecentShown = 6

// Board is the live view of a backlog (BACKLOG §8 `loops backlog watch`):
// the settings that shape it, the counts, every running lane with what its
// run is doing, the next queued issues and the latest finished ones. It is
// read-only: building it writes nothing and calls no forge.
type Board struct {
	Settings Settings
	Synced   time.Time
	Counts   map[Status]int
	Running  []LaneView
	Next     []Item // the first queued issues, in lane order
	Recent   []Item // the latest finished issues, newest first
	// PRs counts the PRs lanes recorded (open or not: the board never asks
	// the forge, so it cannot tell which are still open).
	PRs      int
	Findings int // filed
	Pending  int // waiting to be filed
}

// LaneView is one running lane: its issue, whether its process still holds
// the lane lock, its log, and its run's progress (nil = no run yet).
type LaneView struct {
	Item  Item
	Alive bool
	Log   string
	Run   *Progress
}

// Progress summarises a lane's loops run (its worktree's .loops/status.json).
type Progress struct {
	Phase   run.Phase
	Ended   bool
	Outcome string
	PR      string
	Updated time.Time
	Steps   int
	Errors  int
	Agents  int // agents spawned so far
	Busy    int // agents running now
	GatesOK int
	Gates   int
	ACsOK   int
	ACs     int
	// Rounds counts plan rounds past the first plus QA and product
	// loop-backs: how often the run went back.
	Rounds int
	Last   string // the newest step: "<role> <tool> <target>"
}

// NewProgress summarises a run's state.
func NewProgress(s run.State) Progress {
	p := Progress{Phase: s.Phase, Ended: s.Ended, Outcome: s.Outcome, PR: s.PR, Updated: s.Updated,
		Steps: s.Steps, Errors: s.Errors, Agents: len(s.Lanes), Gates: len(s.Gates),
		Rounds: max(s.PlanRounds-1, 0) + len(s.LoopBacks)}
	for _, l := range s.Lanes {
		if l.Running {
			p.Busy++
		}
	}
	for _, g := range s.Gates {
		if g.OK {
			p.GatesOK++
		}
	}
	acs := map[string]string{}
	for _, v := range s.Verdicts {
		if v.Phase == run.PhaseProduct || strings.HasPrefix(v.Name, "AC") {
			acs[v.Name] = v.Status
		}
	}
	p.ACs = len(acs)
	for _, st := range acs {
		if st == run.StatusVerified {
			p.ACsOK++
		}
	}
	if n := len(s.Recent); n > 0 {
		st := s.Recent[n-1]
		who := cmp.Or(st.Role, "lead")
		p.Last = strings.TrimSpace(who + " " + st.Tool + " " + st.Target)
	}
	return p
}

// NewBoard builds the board from the state and the running lanes (in the
// state's order; the caller reads each lane's liveness and run).
func NewBoard(st State, running []LaneView) Board {
	b := Board{Settings: st.Settings, Synced: st.Synced, Counts: map[Status]int{},
		Running: running, Findings: len(st.Findings), Pending: len(st.Pending)}
	var recent []Item
	for _, it := range st.Items {
		b.Counts[it.Status]++
		b.PRs += len(it.lanePRs())
		switch it.Status {
		case Done, Failed, Human, Skipped:
			recent = append(recent, it)
		}
	}
	slices.SortStableFunc(recent, func(a, c Item) int {
		if d := c.Updated.Compare(a.Updated); d != 0 {
			return d
		}
		return c.Number - a.Number
	})
	b.Recent = recent[:min(len(recent), RecentShown)]
	q := st.Queue()
	b.Next = q[:min(len(q), RecentShown)]
	return b
}
