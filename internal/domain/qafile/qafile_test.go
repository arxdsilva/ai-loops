package qafile

import (
	"reflect"
	"strings"
	"testing"
)

const laneFile = "# QA W1-domain r1\n\n## Claims\n" +
	"### C1: Order total sums line items - VERIFIED\n- Checked: ran the order tests\n" +
	"### C2: Rejects negative qty - or zero - FAILED\n- Repro: `run it`\n  - Output (<=20 lines):\n    ```\n    --- FAIL\n    ```\n" +
	"### C3: Idempotent replay - VERIFIED\n\n## Gate results\n- coverage 82%\n\nVerdict: FAILED\n"

const qualityFile = "# Quality r1\n\n## Findings\n" +
	"### Q1: CRAP 14.2 - W1-domain\n- Where: a.go:1\n" +
	"### Q2: survived mutant `<` -> `<=` - W2-api\n" +
	"### Q3: duplicate of x.go:3 at 0.91 - W1-domain\n\nVerdict: FAILED\n"

const securityFile = "# Security r2\n\n## Findings\n" +
	"### S1: gosec G304, severity medium - W1-domain - blocking\n" +
	"### S2: govulncheck GO-2024-1, severity low - W2-api - note\n" +
	"### S3: gosec G101, severity high - W2-api - blocking\n" +
	"### S4: gosec G104, severity low - W1-domain - blocking\n\nBlocking: 3 | Notes: 1\n\nVerdict: FAILED\n"

func TestParseNames(t *testing.T) {
	for _, tc := range []struct {
		file string
		kind Kind
		name string
		r    int
		ok   bool
	}{
		{"qa/W1-domain.r1.md", KindLane, "W1-domain", 1, true},
		{"/runs/x/qa/a1b2c3.r12.md", KindLane, "a1b2c3", 12, true},
		{"quality.r3.md", KindQuality, "quality", 3, true},
		{"qa/security.r2.md", KindSecurity, "security", 2, true},
		{"W1.md", "", "", 0, false},
		{"W1.r0.md", "", "", 0, false},
		{"W1.rx.md", "", "", 0, false},
		{".r1.md", "", "", 0, false},
	} {
		d, err := Parse(tc.file, "Verdict: VERIFIED\n")
		if (err == nil) != tc.ok {
			t.Errorf("%s: err %v", tc.file, err)
			continue
		}
		if tc.ok && (d.Kind != tc.kind || d.Name != tc.name || d.Round != tc.r) {
			t.Errorf("%s: %+v", tc.file, d)
		}
	}
}

func TestParseErrors(t *testing.T) {
	for name, tc := range map[string]struct{ file, body, want string }{
		"empty":         {"W1.r1.md", "# QA\n\nnothing here\n", "no claims"},
		"unfilled":      {"W1.r1.md", "### C1: x - VERIFIED|FAILED\n", "heading C1"},
		"no status":     {"W1.r1.md", "### C1: statement only\n", "heading C1"},
		"quality lane":  {"quality.r1.md", "### Q1: CRAP 12\n", "heading Q1"},
		"security flag": {"security.r1.md", "### S1: gosec G1, severity low - W1 - maybe\n", "heading S1"},
		"security lane": {"security.r1.md", "### S1: gosec - blocking\n", "heading S1"},
	} {
		_, err := Parse(tc.file, tc.body)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestParseIgnoresPlaceholderVerdict(t *testing.T) {
	d, err := Parse("W1.r1.md", "### C1: x - VERIFIED\nVerdict: VERIFIED|FAILED\n")
	if err != nil || d.Verdict != "" || len(d.Items) != 1 {
		t.Fatalf("%+v %v", d, err)
	}
}

func TestPlan(t *testing.T) {
	v := func(n, s, r string) Verdict { return Verdict{Name: n, Status: s, Role: r} }
	for name, tc := range map[string]struct {
		file, body string
		want       Plan
	}{
		"lane failed claim": {"qa/W1-domain.r1.md", laneFile, Plan{
			Verdicts:  []Verdict{v("W1-domain:C1", "VERIFIED", RoleVerifier), v("W1-domain:C2", "FAILED", RoleVerifier), v("W1-domain:C3", "VERIFIED", RoleVerifier)},
			LoopBacks: []LoopBack{{From: RoleVerifier, To: "W1-domain", Note: "C2"}},
		}},
		"lane all verified but verdict failed": {"W1.r2.md", "### C1: a - VERIFIED\n\nVerdict: FAILED\n", Plan{
			Verdicts:  []Verdict{v("W1:C1", "VERIFIED", RoleVerifier)},
			LoopBacks: []LoopBack{{From: RoleVerifier, To: "W1", Note: "verdict FAILED"}},
		}},
		"lane clean": {"W1.r1.md", "### C1: a - VERIFIED\nVerdict: VERIFIED\n", Plan{
			Verdicts: []Verdict{v("W1:C1", "VERIFIED", RoleVerifier)},
		}},
		"quality findings": {"quality.r1.md", qualityFile, Plan{
			Verdicts:  []Verdict{v("quality:Q1", "FAILED", RoleQuality), v("quality:Q2", "FAILED", RoleQuality), v("quality:Q3", "FAILED", RoleQuality)},
			LoopBacks: []LoopBack{{RoleQuality, "W1-domain", "Q1,Q3"}, {RoleQuality, "W2-api", "Q2"}},
		}},
		"quality clean": {"quality.r2.md", "## Findings\n\nVerdict: VERIFIED\n", Plan{
			Verdicts: []Verdict{v("quality", "VERIFIED", RoleQuality)},
		}},
		"security blocking only": {"security.r2.md", securityFile, Plan{
			Verdicts:  []Verdict{v("security:S1", "FAILED", RoleSecurity), v("security:S3", "FAILED", RoleSecurity), v("security:S4", "FAILED", RoleSecurity)},
			LoopBacks: []LoopBack{{RoleSecurity, "W1-domain", "S1,S4"}, {RoleSecurity, "W2-api", "S3"}},
		}},
		"security notes only": {"security.r1.md", "### S1: gosec G1, severity low - W1 - note\n\nVerdict: VERIFIED\n", Plan{
			Verdicts: []Verdict{v("security", "VERIFIED", RoleSecurity)},
		}},
	} {
		d, err := Parse(tc.file, tc.body)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := d.Plan(); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s:\n got %+v\nwant %+v", name, got, tc.want)
		}
	}
}
