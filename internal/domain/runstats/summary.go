package runstats

import (
	"slices"
	"sort"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

// LeadRole is the row the lead's own tokens go under in a Summary.
const LeadRole = "lead"

// PhaseMedian is a phase's median minutes over the runs that entered it.
type PhaseMedian struct {
	Phase   string  `json:"phase"`
	Runs    int     `json:"runs"`
	Min     float64 `json:"min"`
	LeadMin float64 `json:"lead_only_min"`
}

// RoleRow is one role's tokens over all runs. The lead's row has no agents.
type RoleRow struct {
	Role   string  `json:"role"`
	Agents int     `json:"agents"`
	Tokens int64   `json:"tokens"`
	Median int64   `json:"median"` // per agent
	Share  float64 `json:"share"`  // of every role's tokens plus the lead's, 0..1
}

// Summary aggregates runs.
type Summary struct {
	Runs          int           `json:"runs"`
	MedianWallMin float64       `json:"median_wall_min"`
	Phases        []PhaseMedian `json:"phases"`
	Roles         []RoleRow     `json:"roles"`
}

// Summarize aggregates the runs: medians of wall time and of each phase, and
// the tokens per role across runs, biggest first.
func Summarize(runs []Run) Summary {
	s := Summary{Runs: len(runs)}
	var walls []float64
	phase := map[string][]float64{}
	lead := map[string][]float64{}
	perRole := map[string][]int64{}
	var leadTotal int64
	for _, r := range runs {
		walls = append(walls, r.WallMin)
		for p, m := range r.PhaseMin {
			phase[p] = append(phase[p], m)
			lead[p] = append(lead[p], r.LeadMin[p])
		}
		for ro, ts := range r.RoleTokens {
			perRole[ro] = append(perRole[ro], ts...)
		}
		leadTotal += r.LeadTokens
	}
	s.MedianWallMin = medianF(walls)
	for _, p := range run.Phases {
		if ms, ok := phase[string(p)]; ok {
			s.Phases = append(s.Phases, PhaseMedian{Phase: string(p), Runs: len(ms), Min: medianF(ms), LeadMin: medianF(lead[string(p)])})
		}
	}
	var all int64 = leadTotal
	for ro, ts := range perRole {
		var sum int64
		for _, t := range ts {
			sum += t
		}
		all += sum
		slices.Sort(ts)
		s.Roles = append(s.Roles, RoleRow{Role: ro, Agents: len(ts), Tokens: sum, Median: medianI(ts)})
	}
	if leadTotal > 0 {
		s.Roles = append(s.Roles, RoleRow{Role: LeadRole, Tokens: leadTotal})
	}
	for i := range s.Roles {
		if all > 0 {
			s.Roles[i].Share = float64(s.Roles[i].Tokens) / float64(all)
		}
	}
	sort.SliceStable(s.Roles, func(i, j int) bool {
		if s.Roles[i].Tokens != s.Roles[j].Tokens {
			return s.Roles[i].Tokens > s.Roles[j].Tokens
		}
		return s.Roles[i].Role < s.Roles[j].Role
	})
	return s
}

func medianF(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := slices.Clone(v)
	slices.Sort(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// medianI takes a sorted slice.
func medianI(s []int64) int64 {
	n := len(s)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}
