// Package qafile parses the QA files the QA roles write from templates/
// (QA_VERDICT.md, QUALITY.md, SECURITY.md) into the verdicts and loop-backs
// `loops run ingest` records (WORKFLOW §2.4). It is pure: no I/O.
package qafile

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
)

// Kind is which pass a QA file belongs to.
type Kind string

const (
	KindLane     Kind = "lane"     // <agent-id>.r<N>.md: one implementer's claims
	KindQuality  Kind = "quality"  // quality.r<N>.md
	KindSecurity Kind = "security" // security.r<N>.md
)

// Roles that record the verdicts and send the loop-backs.
const (
	RoleVerifier = "qa-verifier"
	RoleQuality  = "qa-quality"
	RoleSecurity = "qa-security"
)

// Item is one heading: a claim (C<n>), a quality finding (Q<n>) or a
// security finding (S<n>).
type Item struct {
	ID       string // "C1", "Q2", "S3"
	Status   string // lane claims: VERIFIED or FAILED; findings: FAILED
	Lane     string // quality and security: the owning lane id
	Blocking bool   // security: blocking (false = note); lane claims: failed; quality: true
	Title    string // the heading text after the id
}

// Doc is a parsed QA file.
type Doc struct {
	Kind    Kind
	Name    string // agent id for a lane file, else "quality" or "security"
	Round   int
	Base    string // the file's basename
	Items   []Item
	Verdict string // the final "Verdict:" line (VERIFIED or FAILED), "" when absent
}

var (
	nameRe    = regexp.MustCompile(`^(.+)\.r(\d+)\.md$`)
	headingRe = regexp.MustCompile(`^###\s+([CQS])(\d+):\s*(.*?)\s*$`)
	verdictRe = regexp.MustCompile("^[*_`\\s]*Verdict[*_`]*:[*_`\\s]*(VERIFIED|FAILED)[*_`\\s]*$")
)

// Parse reads a QA file's content; file is its path or basename (the round
// and the pass come from the basename).
func Parse(file, content string) (Doc, error) {
	base := path.Base(strings.ReplaceAll(file, "\\", "/"))
	m := nameRe.FindStringSubmatch(base)
	if m == nil {
		return Doc{}, fmt.Errorf("%q is not a QA file name (want <agent-id>.r<N>.md, quality.r<N>.md or security.r<N>.md)", base)
	}
	round, _ := strconv.Atoi(m[2])
	if round < 1 {
		return Doc{}, fmt.Errorf("%q: the round must be 1 or more", base)
	}
	d := Doc{Name: m[1], Round: round, Base: base, Kind: KindLane}
	switch m[1] {
	case "quality":
		d.Kind = KindQuality
	case "security":
		d.Kind = KindSecurity
	}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimRight(line, " \t\r")
		if v := verdictRe.FindStringSubmatch(line); v != nil {
			d.Verdict = v[1]
			continue
		}
		h := headingRe.FindStringSubmatch(line)
		if h == nil || !headingMatches(d.Kind, h[1]) {
			continue
		}
		it, err := parseItem(d.Kind, h[1]+h[2], h[3])
		if err != nil {
			return Doc{}, err
		}
		d.Items = append(d.Items, it)
	}
	if len(d.Items) == 0 && d.Verdict == "" {
		return Doc{}, errors.New("no claims, findings or Verdict line found (is this file written from its template?)")
	}
	return d, nil
}

func headingMatches(k Kind, letter string) bool {
	switch k {
	case KindQuality:
		return letter == "Q"
	case KindSecurity:
		return letter == "S"
	}
	return letter == "C"
}

// rcut splits s at the last " - ".
func rcut(s string) (before, after string, ok bool) {
	i := strings.LastIndex(s, " - ")
	if i < 0 {
		return s, "", false
	}
	return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+3:]), true
}

func parseItem(k Kind, id, rest string) (Item, error) {
	bad := func(want string) (Item, error) {
		return Item{}, fmt.Errorf("heading %s: want %s, got %q", id, want, rest)
	}
	switch k {
	case KindLane:
		title, st, ok := rcut(rest)
		st = strings.ToUpper(strings.Trim(st, "*_` "))
		if !ok || (st != "VERIFIED" && st != "FAILED") {
			return bad("`<statement> - VERIFIED|FAILED`")
		}
		return Item{ID: id, Status: st, Title: title, Blocking: st == "FAILED"}, nil
	case KindQuality:
		title, lane, ok := rcut(rest)
		lane = strings.Trim(lane, "*_` ")
		if !ok || lane == "" {
			return bad("`<metric> <value> - <lane id>`")
		}
		return Item{ID: id, Status: "FAILED", Lane: lane, Title: title, Blocking: true}, nil
	}
	head, flag, ok := rcut(rest)
	flag = strings.ToLower(strings.Trim(flag, "*_` "))
	if !ok || (flag != "blocking" && flag != "note") {
		return bad("`<tool> <rule>, severity <l> - <lane id> - blocking|note`")
	}
	title, lane, ok := rcut(head)
	lane = strings.Trim(lane, "*_` ")
	if !ok || lane == "" {
		return bad("`<tool> <rule>, severity <l> - <lane id> - blocking|note`")
	}
	return Item{ID: id, Status: "FAILED", Lane: lane, Title: title, Blocking: flag == "blocking"}, nil
}

// Verdict is one verdict event to record.
type Verdict struct {
	Name, Status, Role string
}

// LoopBack is one loop_back event to record.
type LoopBack struct {
	From, To, Note string
}

// Plan is what a Doc records: the verdicts and the loop-backs (all at
// Doc.Round, with ref qa/<Base>).
type Plan struct {
	Verdicts  []Verdict
	LoopBacks []LoopBack
}

// Plan derives the events from the parsed file.
func (d Doc) Plan() Plan {
	switch d.Kind {
	case KindQuality:
		return d.findingsPlan(RoleQuality, "quality")
	case KindSecurity:
		return d.findingsPlan(RoleSecurity, "security")
	}
	var p Plan
	var failed []string
	for _, it := range d.Items {
		p.Verdicts = append(p.Verdicts, Verdict{Name: d.Name + ":" + it.ID, Status: it.Status, Role: RoleVerifier})
		if it.Status == "FAILED" {
			failed = append(failed, it.ID)
		}
	}
	if len(failed) > 0 || d.Verdict == "FAILED" {
		note := "verdict FAILED"
		if len(failed) > 0 {
			note = strings.Join(failed, ",")
		}
		p.LoopBacks = append(p.LoopBacks, LoopBack{From: RoleVerifier, To: d.Name, Note: note})
	}
	return p
}

// findingsPlan serves the quality and security passes: each blocking finding
// is a FAILED verdict "<pass>:<id>" and a loop-back per distinct lane; a pass
// with no blocking finding records "<pass>" with the file's Verdict.
func (d Doc) findingsPlan(role, pass string) Plan {
	var p Plan
	var lanes []string
	ids := map[string][]string{}
	for _, it := range d.Items {
		if !it.Blocking {
			continue
		}
		p.Verdicts = append(p.Verdicts, Verdict{Name: pass + ":" + it.ID, Status: "FAILED", Role: role})
		if _, seen := ids[it.Lane]; !seen {
			lanes = append(lanes, it.Lane)
		}
		ids[it.Lane] = append(ids[it.Lane], it.ID)
	}
	for _, l := range lanes {
		p.LoopBacks = append(p.LoopBacks, LoopBack{From: role, To: l, Note: strings.Join(ids[l], ",")})
	}
	if len(p.Verdicts) == 0 && d.Verdict != "" {
		p.Verdicts = append(p.Verdicts, Verdict{Name: pass, Status: d.Verdict, Role: role})
	}
	return p
}
