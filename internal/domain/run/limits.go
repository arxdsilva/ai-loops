package run

import (
	"fmt"
	"maps"
	"strconv"
	"strings"
)

// Limits are the user's opt-in limits (WORKFLOW §4a). The zero value means
// no limits: nothing is checked and nothing is shown.
type Limits struct {
	// MaxAgents caps concurrent agents for the whole run (0 = no extra cap;
	// the always-on cap is parallel.max_agents ∧ hardware).
	MaxAgents int                  `json:"max_agents,omitempty"`
	Phases    map[Phase]PhaseLimit `json:"phases,omitempty"`
}

// PhaseLimit limits one phase.
type PhaseLimit struct {
	MaxTokens int64    `json:"max_tokens,omitempty"` // tokens used by agents opened in this phase
	MaxAgents int      `json:"max_agents,omitempty"` // agents (lanes) opened in this phase, total
	Roles     []string `json:"roles,omitempty"`      // roles allowed in this phase (empty = any)
}

// IsZero reports whether no limit is set.
func (l Limits) IsZero() bool { return l.MaxAgents == 0 && len(l.Phases) == 0 }

// Merge returns l with every non-zero field of over applied on top, phase by
// phase and field by field. Neither input is modified.
func (l Limits) Merge(over Limits) Limits {
	out := Limits{MaxAgents: l.MaxAgents}
	if over.MaxAgents != 0 {
		out.MaxAgents = over.MaxAgents
	}
	if len(l.Phases)+len(over.Phases) == 0 {
		return out
	}
	out.Phases = maps.Clone(l.Phases)
	if out.Phases == nil {
		out.Phases = map[Phase]PhaseLimit{}
	}
	for p, o := range over.Phases {
		pl := out.Phases[p]
		if o.MaxTokens != 0 {
			pl.MaxTokens = o.MaxTokens
		}
		if o.MaxAgents != 0 {
			pl.MaxAgents = o.MaxAgents
		}
		if len(o.Roles) > 0 {
			pl.Roles = o.Roles
		}
		out.Phases[p] = pl
	}
	return out
}

// BudgetDecision is the outcome of CheckBudget.
type BudgetDecision struct {
	OK     bool
	Limit  string // "tokens" | "agents" | "roles" | "max_agents" when !OK
	Reason string
}

// Budget limit names (BudgetDecision.Limit, Event.Name for KindBudget).
const (
	LimitMaxAgents = "max_agents"
	LimitRoles     = "roles"
	LimitAgents    = "agents"
	LimitTokens    = "tokens"
)

// CheckBudget decides whether spawning n more agents of role in phase fits
// the limits given the run state (running lanes, phase lanes, phase
// tokens). Zero limits always fit. n below 1 counts as 1. Checks run in
// order: run cap, phase roles, phase agents, phase tokens.
func CheckBudget(s State, l Limits, phase Phase, n int, role string) BudgetDecision {
	if l.IsZero() {
		return BudgetDecision{OK: true}
	}
	n = max(n, 1)
	if l.MaxAgents > 0 {
		running := 0
		for _, ln := range s.Lanes {
			if ln.Running {
				running++
			}
		}
		if running+n > l.MaxAgents {
			return blocked(LimitMaxAgents, "%d running + %d new would exceed the run cap of %d agents", running, n, l.MaxAgents)
		}
	}
	pl, ok := l.Phases[phase]
	if !ok {
		return BudgetDecision{OK: true}
	}
	if len(pl.Roles) > 0 && !roleAllowed(pl.Roles, role) {
		if role == "" {
			return blocked(LimitRoles, "%s allows only %s; pass --role", phase, strings.Join(pl.Roles, ", "))
		}
		return blocked(LimitRoles, "%s allows only %s (not %s)", phase, strings.Join(pl.Roles, ", "), role)
	}
	if pl.MaxAgents > 0 {
		opened := 0
		for _, ln := range s.Lanes {
			if ln.Phase == phase {
				opened++
			}
		}
		if opened+n > pl.MaxAgents {
			return blocked(LimitAgents, "%s agent budget: %d opened + %d new would exceed %d", phase, opened, n, pl.MaxAgents)
		}
	}
	if pl.MaxTokens > 0 {
		if used := s.PhaseTokens[phase]; used >= pl.MaxTokens {
			return blocked(LimitTokens, "%s token budget spent: %s of %s", phase, FormatTokens(used), FormatTokens(pl.MaxTokens))
		}
	}
	return BudgetDecision{OK: true}
}

func blocked(limit, format string, args ...any) BudgetDecision {
	return BudgetDecision{Limit: limit, Reason: fmt.Sprintf(format, args...)}
}

// roleAllowed compares role names case-insensitively, ignoring a "loops:"
// plugin prefix on either side.
func roleAllowed(roles []string, role string) bool {
	role = strings.TrimPrefix(strings.TrimSpace(role), "loops:")
	if role == "" {
		return false
	}
	for _, r := range roles {
		if strings.EqualFold(strings.TrimPrefix(strings.TrimSpace(r), "loops:"), role) {
			return true
		}
	}
	return false
}

// FormatTokens renders a token count with a k/M/G suffix and at most one
// decimal ("950", "1.2k", "200k", "2.1M").
func FormatTokens(n int64) string {
	units := []struct {
		div    float64
		suffix string
	}{{1e9, "G"}, {1e6, "M"}, {1e3, "k"}}
	for _, u := range units {
		if float64(n) >= u.div {
			s := strconv.FormatFloat(float64(n)/u.div, 'f', 1, 64)
			return strings.TrimSuffix(s, ".0") + u.suffix
		}
	}
	return strconv.FormatInt(n, 10)
}

// ParseTokens parses a positive token count with an optional k/M/G suffix
// (case-insensitive, decimals allowed: "200k", "1.5M").
func ParseTokens(s string) (int64, error) {
	t := strings.TrimSpace(s)
	mult := 1.0
	if t != "" {
		switch t[len(t)-1] {
		case 'k', 'K':
			mult = 1e3
		case 'm', 'M':
			mult = 1e6
		case 'g', 'G':
			mult = 1e9
		}
		if mult != 1 {
			t = t[:len(t)-1]
		}
	}
	f, err := strconv.ParseFloat(t, 64)
	if err != nil || f*mult < 1 || f*mult > 1e15 {
		return 0, fmt.Errorf("invalid token count %q (want a positive number with optional k, M or G)", s)
	}
	return int64(f*mult + 0.5), nil
}

// ParseLimits builds Limits from the /loops:work flag syntax (WORKFLOW §4a):
// budget "research=200k,implement=2M", agents "implement=12", roles
// "implement=domain-engineer,api-engineer" (phases separated by ';', or by
// ',' when the next item has its own '='), and the run cap maxAgents.
func ParseLimits(budget, agents, roles string, maxAgents int) (Limits, error) {
	if maxAgents < 0 {
		return Limits{}, fmt.Errorf("max agents must be ≥ 0, got %d", maxAgents)
	}
	l := Limits{MaxAgents: maxAgents}
	set := func(p Phase, f func(*PhaseLimit)) {
		if l.Phases == nil {
			l.Phases = map[Phase]PhaseLimit{}
		}
		pl := l.Phases[p]
		f(&pl)
		l.Phases[p] = pl
	}
	err := eachPair("budget", budget, func(p Phase, v string) error {
		n, err := ParseTokens(v)
		if err != nil {
			return err
		}
		set(p, func(pl *PhaseLimit) { pl.MaxTokens = n })
		return nil
	})
	if err != nil {
		return Limits{}, err
	}
	err = eachPair("agents", agents, func(p Phase, v string) error {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return fmt.Errorf("agents: invalid count %q for %s (want a positive integer)", v, p)
		}
		set(p, func(pl *PhaseLimit) { pl.MaxAgents = n })
		return nil
	})
	if err != nil {
		return Limits{}, err
	}
	var cur Phase
	for _, group := range strings.Split(roles, ";") {
		cur = ""
		for _, item := range strings.Split(group, ",") {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if k, v, ok := strings.Cut(item, "="); ok {
				p, err := phaseKey("roles", k)
				if err != nil {
					return Limits{}, err
				}
				cur, item = p, strings.TrimSpace(v)
				if item == "" {
					return Limits{}, fmt.Errorf("roles: no role given for %s", p)
				}
			}
			if cur == "" {
				return Limits{}, fmt.Errorf("roles: %q has no phase (want phase=role,role)", item)
			}
			set(cur, func(pl *PhaseLimit) { pl.Roles = append(pl.Roles, item) })
		}
	}
	return l, nil
}

// eachPair calls f for every "phase=value" item of a comma-separated list.
func eachPair(flag, s string, f func(Phase, string) error) error {
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		k, v, ok := strings.Cut(item, "=")
		if !ok {
			return fmt.Errorf("%s: %q is not phase=value", flag, item)
		}
		p, err := phaseKey(flag, k)
		if err != nil {
			return err
		}
		if err := f(p, strings.TrimSpace(v)); err != nil {
			return err
		}
	}
	return nil
}

func phaseKey(flag, k string) (Phase, error) {
	p := Phase(strings.TrimSpace(k))
	if p.Index() < 0 {
		return "", fmt.Errorf("%s: unknown phase %q (want one of %v)", flag, k, Phases)
	}
	return p, nil
}
