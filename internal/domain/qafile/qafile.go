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
	flag     string // security: the blocking/note flag as written, "" when absent
}

// Doc is a parsed QA file.
type Doc struct {
	Kind    Kind
	Name    string // the lane a lane file verifies ("" for qa-all), else "quality" or "security"
	Round   int
	Base    string // the file's basename
	Items   []Item
	Verdict string // the final "Verdict:" line (VERIFIED or FAILED), "" when absent
}

var (
	nameRe = regexp.MustCompile(`^(.+)\.r(\d+)\.md$`)
	// An item line: a heading, bullet or table row that opens with an id,
	// optionally lane-qualified for a verifier that covers several lanes
	// (qa-all): "### C1: x - VERIFIED", "- C1a VERIFIED: x", "| C2 x | FAILED |",
	// "- W1-api:C3: x - FAILED".
	itemRe      = regexp.MustCompile("^[\\s#>*_|`-]*(?:([A-Za-z][\\w.-]*):)?([CQS]\\d+[a-z]?)\\b[*_`]*[:.)]?\\s*(.*)$")
	statusRe    = regexp.MustCompile(`\b(VERIFIED|FAILED)\b`)
	verdictRe   = regexp.MustCompile("^[\\s#>*_`-]*Verdict[*_`]*\\s*:(.*)$")
	laneRe      = regexp.MustCompile(`\bW\d+[a-z]?(?:-[A-Za-z0-9]+)*\b`)
	flagRe      = regexp.MustCompile(`(?i)\b(non-blocking|blocking|note)\b`)
	claimIDRe   = regexp.MustCompile(`\bC\d+[a-z]?\b`)
	notFailedRe = regexp.MustCompile(`(?i)\b(not (a )?|no )FAILED\b`)
	resolvedRe  = regexp.MustCompile(`(?i)\b(VERIFIED|fixed|resolved|closed)\b`)
	noneRe      = regexp.MustCompile(`(?i)^[\s*_-]*(none|no (blocking )?findings)\b`)
)

// Parse reads a QA file's content; file is its path or basename (the round
// and the pass come from the basename). It reads the templates' headings and
// the shapes verifiers write in practice (bullets, table rows, free verdict
// lines); a file with neither an item nor a Verdict line is an error.
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
	case "quality", "qa-quality":
		d.Kind, d.Name = KindQuality, "quality"
	case "security", "qa-security":
		d.Kind, d.Name = KindSecurity, "security"
	case "qa-all", "all":
		d.Name = "" // every item names its lane
	default:
		// The verifier's own Agent id is qa-<lane>; the lane is what it verifies.
		d.Name = strings.TrimPrefix(m[1], "qa-")
	}
	seen := map[string]int{}
	add := func(it Item) {
		k := it.Lane + ":" + it.ID
		if i, ok := seen[k]; ok {
			// A claim mentioned again keeps its first reading, except that
			// FAILED wins: a missed failure costs more than a re-check.
			if d.Kind == KindLane && it.Status == "FAILED" {
				d.Items[i].Status, d.Items[i].Blocking = "FAILED", true
			}
			return
		}
		seen[k] = len(d.Items)
		d.Items = append(d.Items, it)
	}
	var failedInVerdict []string
	none := false
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimRight(line, " \t\r")
		if v := verdictRe.FindStringSubmatch(line); v != nil {
			if st := status(v[1]); st != "" {
				d.Verdict = st
			}
			failedInVerdict = append(failedInVerdict, failedIDs(v[1])...)
			continue
		}
		if noneRe.MatchString(line) {
			none = true
		}
		h := itemRe.FindStringSubmatch(line)
		if h == nil || !headingMatches(d.Kind, h[2][:1]) {
			continue
		}
		if it, ok := parseItem(d, h[1], h[2], h[3]); ok {
			add(it)
		}
	}
	if d.Kind == KindLane && d.Name != "" {
		for _, id := range failedInVerdict {
			add(Item{ID: id, Status: "FAILED", Lane: d.Name, Blocking: true, Title: "named FAILED in the Verdict line"})
		}
	}
	if d.Kind != KindLane && d.Verdict == "" && none && len(d.Items) == 0 {
		d.Verdict = "VERIFIED" // "## Findings" followed by "None."
	}
	if len(d.Items) == 0 && d.Verdict == "" {
		return Doc{}, errors.New("no claims, findings or Verdict line found (is this file written from its template?)")
	}
	if d.Kind == KindSecurity {
		for i, it := range d.Items {
			if it.flag == "" {
				// No blocking/note flag: the file's verdict decides.
				d.Items[i].Blocking = d.Verdict == "FAILED"
			}
		}
	}
	return d, nil
}

// failedIDs are the claim ids a Verdict line names as FAILED: "FAILED (C2a,
// C2c)" or "C1, C2 VERIFIED. C4 FAILED on x" give C2a, C2c and C4. A
// clause stating both words is split at commas first.
func failedIDs(text string) []string {
	var out []string
	for _, clause := range regexp.MustCompile(`[.;]\s`).Split(text, -1) {
		parts := []string{clause}
		if strings.Contains(clause, "VERIFIED") && strings.Contains(clause, "FAILED") {
			parts = strings.Split(clause, ",")
		}
		for _, part := range parts {
			if !strings.Contains(part, "FAILED") || negated(part) {
				continue
			}
			out = append(out, claimIDRe.FindAllString(part, -1)...)
		}
	}
	return out
}

// negated reports a "not FAILED", "not a FAILED" or "no FAILED" aside.
func negated(s string) bool {
	return notFailedRe.MatchString(s) && !strings.Contains(notFailedRe.ReplaceAllString(s, ""), "FAILED")
}

// laneStatus is a claim line's status: FAILED anywhere in it (a failed
// sub-claim fails the claim), else VERIFIED, else "".
func laneStatus(s string) string {
	if strings.Contains(s, "VERIFIED|FAILED") {
		return ""
	}
	if strings.Contains(s, "FAILED") && !negated(s) {
		return "FAILED"
	}
	if strings.Contains(s, "VERIFIED") {
		return "VERIFIED"
	}
	return ""
}

// status is the VERIFIED or FAILED a text states, "" for none or for the
// template's placeholder "VERIFIED|FAILED".
func status(s string) string {
	if strings.Contains(s, "VERIFIED|FAILED") {
		return ""
	}
	return statusRe.FindString(s)
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

// lane is the owning lane a finding names: the template's " - <lane id>"
// segment, else the first plan-style lane id (W1-domain) in the text.
func lane(rest string) string {
	parts := strings.Split(rest, " - ")
	for i := len(parts) - 1; i >= 1; i-- {
		p := strings.Trim(parts[i], "*_` ")
		if p != "" && !strings.ContainsAny(p, " ,") && !flagRe.MatchString(p) {
			return p
		}
	}
	return laneRe.FindString(rest)
}

// resolved reports a finding a later round marks fixed ("- Q1 (W1-x)
// VERIFIED. bioRow is extracted").
func resolved(rest string) bool {
	return resolvedRe.MatchString(rest) && !strings.Contains(rest, "FAILED")
}

func parseItem(d Doc, qual, id, rest string) (Item, bool) {
	title := strings.TrimSpace(rest)
	switch d.Kind {
	case KindLane:
		st := laneStatus(rest)
		if st == "" {
			return Item{}, false
		}
		l := d.Name
		if qual != "" {
			l = qual
		}
		if l == "" {
			return Item{}, false // qa-all needs lane-qualified ids
		}
		return Item{ID: id, Status: st, Lane: l, Title: title, Blocking: st == "FAILED"}, true
	case KindQuality:
		if resolved(rest) {
			return Item{ID: id, Status: "VERIFIED", Lane: laneRe.FindString(rest), Title: title}, true
		}
		return Item{ID: id, Status: "FAILED", Lane: lane(rest), Title: title, Blocking: true}, true
	}
	if resolved(rest) {
		return Item{ID: id, Status: "VERIFIED", Lane: laneRe.FindString(rest), Title: title, flag: "resolved"}, true
	}
	it := Item{ID: id, Status: "FAILED", Lane: lane(rest), Title: title}
	if f := flagRe.FindAllString(rest, -1); len(f) > 0 {
		it.flag = strings.ToLower(f[len(f)-1])
		it.Blocking = it.flag == "blocking"
	}
	return it, true
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
	var lanes []string
	failed := map[string][]string{}
	for _, it := range d.Items {
		p.Verdicts = append(p.Verdicts, Verdict{Name: it.Lane + ":" + it.ID, Status: it.Status, Role: RoleVerifier})
		if _, ok := failed[it.Lane]; !ok {
			lanes = append(lanes, it.Lane)
			failed[it.Lane] = nil
		}
		if it.Status == "FAILED" {
			failed[it.Lane] = append(failed[it.Lane], it.ID)
		}
	}
	if d.Name != "" && len(lanes) == 0 {
		lanes = []string{d.Name}
	}
	for _, l := range lanes {
		switch {
		case len(failed[l]) > 0:
			p.LoopBacks = append(p.LoopBacks, LoopBack{From: RoleVerifier, To: l, Note: strings.Join(failed[l], ",")})
		case d.Verdict == "FAILED" && d.Name != "":
			p.LoopBacks = append(p.LoopBacks, LoopBack{From: RoleVerifier, To: l, Note: "verdict FAILED"})
		}
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
	blocking := 0
	for _, it := range d.Items {
		if it.Status == "VERIFIED" {
			p.Verdicts = append(p.Verdicts, Verdict{Name: pass + ":" + it.ID, Status: "VERIFIED", Role: role})
			continue
		}
		if !it.Blocking {
			continue
		}
		blocking++
		p.Verdicts = append(p.Verdicts, Verdict{Name: pass + ":" + it.ID, Status: "FAILED", Role: role})
		if it.Lane == "" {
			continue // no owning lane named: the verdict stands, the lead routes it
		}
		if _, seen := ids[it.Lane]; !seen {
			lanes = append(lanes, it.Lane)
		}
		ids[it.Lane] = append(ids[it.Lane], it.ID)
	}
	for _, l := range lanes {
		p.LoopBacks = append(p.LoopBacks, LoopBack{From: role, To: l, Note: strings.Join(ids[l], ",")})
	}
	if blocking == 0 && d.Verdict != "" {
		p.Verdicts = append(p.Verdicts, Verdict{Name: pass, Status: d.Verdict, Role: role})
	}
	return p
}
