// Package plancheck validates a plan's waves table (templates/PLAN.md)
// against the effective parallelism cap and scope disjointness
// (specs/WORKFLOW.md §2.2, `loops plan check`). Pure: works on the plan
// text.
package plancheck

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Patterns for the wave headings and the wave prefix of an agent id.
var (
	headingRe = regexp.MustCompile(`^#{1,6}\s*Wave\s+([A-Za-z0-9]+)`)
	idWaveRe  = regexp.MustCompile(`^(W\d+[a-z]?)[-_]`)
)

// Agent is one row of a waves table.
type Agent struct {
	ID   string
	Role string
	Wave string // "W1", "W2a", …
	Owns []string
	Line int
}

// Finding is one problem.
type Finding struct {
	Line int    `json:"line"`
	Rule string `json:"rule"` // "cap" | "overlap" | "parse"
	Msg  string `json:"msg"`
}

// Report is the check result.
type Report struct {
	Agents   []Agent   `json:"-"`
	PeakWave string    `json:"peak_wave"`
	Peak     int       `json:"peak"`
	Cap      int       `json:"cap"`
	Findings []Finding `json:"findings"`
}

// OK reports whether there are no findings.
func (r Report) OK() bool { return len(r.Findings) == 0 }

// Check parses plan (markdown) and checks every wave's agent count against
// capAgents and that owned globs of agents in the same wave don't overlap.
//
// Grammar: a markdown table whose header has the columns id, role and owns
// (any order, case-insensitive). The wave of a row is its id prefix
// (`W1-x`, `W2a-y`) or else the nearest preceding `### Wave <X>` heading.
// Sub-waves (W1a, W1b) are separate waves. Owns cells list globs separated
// by commas or spaces, backticks optional.
func Check(plan string, capAgents int) Report {
	r := Report{Cap: capAgents}
	r.Agents = parse(plan)
	if len(r.Agents) == 0 {
		r.Findings = append(r.Findings, Finding{Line: 1, Rule: "parse", Msg: "no waves table found (need a table with id, role and owns columns)"})
		return r
	}
	var order []string
	byWave := map[string][]Agent{}
	for _, a := range r.Agents {
		if _, ok := byWave[a.Wave]; !ok {
			order = append(order, a.Wave)
		}
		byWave[a.Wave] = append(byWave[a.Wave], a)
	}
	for _, w := range order {
		ags := byWave[w]
		if len(ags) > r.Peak {
			r.Peak, r.PeakWave = len(ags), w
		}
		if capAgents > 0 && len(ags) > capAgents {
			r.Findings = append(r.Findings, Finding{Line: ags[capAgents].Line, Rule: "cap",
				Msg: fmt.Sprintf("wave %s has %d agents, cap is %d", w, len(ags), capAgents)})
		}
		for i := range ags {
			for j := i + 1; j < len(ags); j++ {
				if g1, g2, ok := overlapping(ags[i].Owns, ags[j].Owns); ok {
					r.Findings = append(r.Findings, Finding{Line: ags[j].Line, Rule: "overlap",
						Msg: fmt.Sprintf("wave %s: %s (%s) and %s (%s) own overlapping scopes", w, ags[i].ID, g1, ags[j].ID, g2)})
				}
			}
		}
	}
	sort.SliceStable(r.Findings, func(i, j int) bool { return r.Findings[i].Line < r.Findings[j].Line })
	return r
}

func normWave(s string) string {
	if strings.HasPrefix(s, "W") || strings.HasPrefix(s, "w") {
		return "W" + s[1:]
	}
	return "W" + s
}

func splitRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func parse(plan string) []Agent {
	var out []Agent
	lines := strings.Split(plan, "\n")
	heading := ""
	idCol, roleCol, ownsCol := -1, -1, -1
	inTable := false
	for n, raw := range lines {
		line := strings.TrimSpace(raw)
		if m := headingRe.FindStringSubmatch(line); m != nil {
			heading = normWave(m[1])
			inTable = false
			continue
		}
		if !strings.HasPrefix(line, "|") {
			inTable = false
			continue
		}
		cells := splitRow(line)
		if !inTable {
			idCol, roleCol, ownsCol = -1, -1, -1
			for i, c := range cells {
				switch strings.ToLower(strings.Trim(c, "` *")) {
				case "id":
					idCol = i
				case "role":
					roleCol = i
				case "owns":
					ownsCol = i
				}
			}
			inTable = idCol >= 0 && roleCol >= 0 && ownsCol >= 0
			continue
		}
		if strings.Trim(line, "|-: ") == "" {
			continue
		}
		if idCol >= len(cells) || roleCol >= len(cells) || ownsCol >= len(cells) {
			continue
		}
		id := strings.Trim(cells[idCol], "` ")
		wave := heading
		if m := idWaveRe.FindStringSubmatch(id); m != nil {
			wave = m[1]
		}
		if wave == "" {
			wave = "W?"
		}
		out = append(out, Agent{ID: id, Role: strings.Trim(cells[roleCol], "` "), Wave: wave, Owns: splitGlobs(cells[ownsCol]), Line: n + 1})
	}
	return out
}

func splitGlobs(cell string) []string {
	cell = strings.ReplaceAll(cell, "`", " ")
	var out []string
	for _, g := range strings.FieldsFunc(cell, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == ';' }) {
		switch strings.ToLower(g) {
		case "-", "none", "n/a", "\u2014", "\u2013":
			continue
		}
		out = append(out, g)
	}
	return out
}

// scope is a glob reduced to a path prefix; partial means the last segment
// is only a stem (cut at a wildcard mid-segment).
type scope struct {
	segs    []string
	partial bool
}

func toScope(glob string) scope {
	g := strings.TrimPrefix(glob, "./")
	cut := strings.IndexAny(g, "*?[{")
	partial := false
	if cut >= 0 {
		g = g[:cut]
		partial = !strings.HasSuffix(g, "/") && g != ""
	}
	var segs []string
	for _, s := range strings.Split(g, "/") {
		if s != "" {
			segs = append(segs, s)
		}
	}
	return scope{segs: segs, partial: partial && len(segs) > 0}
}

func (a scope) overlaps(b scope) bool {
	n := min(len(a.segs), len(b.segs))
	for i := 0; i < n; i++ {
		x, y := a.segs[i], b.segs[i]
		if x == y {
			continue
		}
		if a.partial && i == len(a.segs)-1 && strings.HasPrefix(y, x) {
			continue
		}
		if b.partial && i == len(b.segs)-1 && strings.HasPrefix(x, y) {
			continue
		}
		return false
	}
	return true
}

func overlapping(a, b []string) (string, string, bool) {
	for _, x := range a {
		sx := toScope(x)
		for _, y := range b {
			if sx.overlaps(toScope(y)) {
				return x, y, true
			}
		}
	}
	return "", "", false
}
