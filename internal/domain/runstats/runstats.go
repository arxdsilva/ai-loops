// Package runstats measures finished or running runs from their events: wall
// time, time per phase and per phase with no agent running, agents, tokens,
// loop-backs and guard denies. It is pure: no I/O, no wall clock (times come
// from the events).
package runstats

import (
	"slices"
	"sort"
	"time"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

// LeadTokensNote is the name of the note events that carry the lead's
// cumulative token count in Event.Tokens.
const LeadTokensNote = "lead-tokens"

// Run is the measurement of one run.
type Run struct {
	ID         string             `json:"id"`
	Tier       string             `json:"tier"`
	WallMin    float64            `json:"wall_min"`
	PhaseMin   map[string]float64 `json:"phase_min"`
	LeadMin    map[string]float64 `json:"lead_only_min"` // per phase, minutes with no agent running
	Agents     int                `json:"agents"`
	Peak       int                `json:"peak_agents"`
	RoleTokens map[string][]int64 `json:"role_tokens"` // per role, each agent's tokens (max of its cumulative counts)
	LeadTokens int64              `json:"lead_tokens"`
	LoopBacks  map[string]int     `json:"loop_backs"`   // by From
	TestMS     int64              `json:"test_gate_ms"` // longest gate named "test"
	Denies     map[string]int     `json:"guard_denies"` // by guard name
}

// TotalTokens is the sum of every agent's tokens in the run.
func (r Run) TotalTokens() int64 {
	var n int64
	for _, ts := range r.RoleTokens {
		for _, t := range ts {
			n += t
		}
	}
	return n
}

type span struct{ from, to time.Time }

func minutes(d time.Duration) float64 { return d.Minutes() }

// Compute measures one run's events (in the order they were recorded).
func Compute(events []run.Event) Run {
	r := Run{
		Tier: "-", PhaseMin: map[string]float64{}, LeadMin: map[string]float64{},
		RoleTokens: map[string][]int64{}, LoopBacks: map[string]int{}, Denies: map[string]int{},
	}
	if len(events) == 0 {
		return r
	}
	first, last := events[0].Time, events[0].Time
	for _, e := range events {
		if e.Time.Before(first) {
			first = e.Time
		}
		if e.Time.After(last) {
			last = e.Time
		}
	}
	r.WallMin = minutes(last.Sub(first))

	type phaseAt struct {
		p run.Phase
		t time.Time
	}
	var phases []phaseAt
	open := map[string][]time.Time{} // agent -> start times not yet stopped
	var spans []span
	agents := map[string]bool{}
	role := map[string]string{}
	tokens := map[string]int64{}
	for _, e := range events {
		if r.ID == "" {
			r.ID = e.Run
		}
		switch e.Kind {
		case run.KindPhase:
			if e.Phase != "" {
				phases = append(phases, phaseAt{e.Phase, e.Time})
			}
		case run.KindAgentStart:
			agents[e.Agent] = true
			open[e.Agent] = append(open[e.Agent], e.Time)
			if e.Role != "" {
				role[e.Agent] = e.Role
			}
		case run.KindAgentStop:
			agents[e.Agent] = true
			if e.Role != "" && role[e.Agent] == "" {
				role[e.Agent] = e.Role
			}
			if q := open[e.Agent]; len(q) > 0 {
				spans = append(spans, span{q[0], e.Time})
				open[e.Agent] = q[1:]
			}
			tokens[e.Agent] = max(tokens[e.Agent], e.Tokens)
		case run.KindLabel:
			if e.Role != "" {
				role[e.Agent] = e.Role
			}
		case run.KindTriage:
			r.Tier = e.Name
		case run.KindLoopBack:
			r.LoopBacks[e.From]++
		case run.KindGuardDeny:
			r.Denies[e.Name]++
		case run.KindGate:
			if e.Name == "test" {
				r.TestMS = max(r.TestMS, e.DurationMS)
			}
		case run.KindNote:
			if e.Name == LeadTokensNote {
				r.LeadTokens = max(r.LeadTokens, e.Tokens)
			}
		}
	}
	delete(agents, "")
	r.Agents = len(agents)
	for _, q := range open { // still running at the last event
		for _, t := range q {
			spans = append(spans, span{t, last})
		}
	}
	r.Peak = peak(spans)
	for a, t := range tokens {
		if a == "" || t == 0 {
			continue
		}
		ro := role[a]
		if ro == "" {
			ro = "unknown"
		}
		r.RoleTokens[ro] = append(r.RoleTokens[ro], t)
	}
	for _, ts := range r.RoleTokens {
		slices.Sort(ts)
	}
	merged := union(spans)
	for i, p := range phases {
		end := last
		if i+1 < len(phases) {
			end = phases[i+1].t
		}
		if end.Before(p.t) {
			continue
		}
		name := string(p.p)
		dur := end.Sub(p.t)
		r.PhaseMin[name] += minutes(dur)
		r.LeadMin[name] += minutes(dur - covered(merged, p.t, end))
	}
	return r
}

// peak is the most spans open at once (an end sorts before a start at the
// same instant).
func peak(spans []span) int {
	type pt struct {
		t time.Time
		d int
	}
	pts := make([]pt, 0, 2*len(spans))
	for _, s := range spans {
		if s.to.After(s.from) {
			pts = append(pts, pt{s.from, 1}, pt{s.to, -1})
		}
	}
	sort.SliceStable(pts, func(i, j int) bool {
		if !pts[i].t.Equal(pts[j].t) {
			return pts[i].t.Before(pts[j].t)
		}
		return pts[i].d < pts[j].d
	})
	cur, best := 0, 0
	for _, p := range pts {
		cur += p.d
		best = max(best, cur)
	}
	return best
}

// union merges overlapping spans into sorted, disjoint ones.
func union(spans []span) []span {
	s := slices.Clone(spans)
	sort.Slice(s, func(i, j int) bool { return s[i].from.Before(s[j].from) })
	var out []span
	for _, x := range s {
		if !x.to.After(x.from) {
			continue
		}
		if n := len(out); n > 0 && !x.from.After(out[n-1].to) {
			if x.to.After(out[n-1].to) {
				out[n-1].to = x.to
			}
			continue
		}
		out = append(out, x)
	}
	return out
}

// covered is how long the disjoint spans overlap [from, to].
func covered(spans []span, from, to time.Time) time.Duration {
	var d time.Duration
	for _, s := range spans {
		lo, hi := s.from, s.to
		if lo.Before(from) {
			lo = from
		}
		if hi.After(to) {
			hi = to
		}
		if hi.After(lo) {
			d += hi.Sub(lo)
		}
	}
	return d
}
