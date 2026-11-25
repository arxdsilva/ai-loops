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
		"empty":     {"W1.r1.md", "# QA\n\nnothing here\n", "no claims"},
		"unfilled":  {"W1.r1.md", "### C1: x - VERIFIED|FAILED\n", "no claims"},
		"no status": {"W1.r1.md", "### C1: statement only\n", "no claims"},
	} {
		_, err := Parse(tc.file, tc.body)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestParseShapesSeenInRuns covers the shapes verifiers wrote in recorded
// runs: bullets, table rows, free verdict lines, a qa- prefixed file name,
// lane-qualified ids for qa-all, and findings with no lane or flag.
func TestParseShapesSeenInRuns(t *testing.T) {
	type lb struct{ to, note string }
	for name, tc := range map[string]struct {
		file, body string
		verdicts   []string
		lbs        []lb
	}{
		"bullets": {"W1-gate.r1.md", "## Claims\n- C1 PackChecks alone: VERIFIED. TestX\n- C2 Missing: FAILED, see repro\n- Note (not a FAILED): x\n",
			[]string{"W1-gate:C1=VERIFIED", "W1-gate:C2=FAILED"}, []lb{{"W1-gate", "C2"}}},
		"suffix ids and free verdict": {"qa-W1-api.r1.md", "Verdict: FAILED (C2a). All other claims VERIFIED.\n- C1a VERIFIED: x\n- C2a FAILED: y\n",
			[]string{"W1-api:C1a=VERIFIED", "W1-api:C2a=FAILED"}, []lb{{"W1-api", "C2a"}}},
		"table": {"W1-body.r1.md", "| Claim | Verdict | Evidence |\n|---|---|---|\n| C1 Poke gate | VERIFIED | ok |\n| C2 Never | VERIFIED | ok |\n",
			[]string{"W1-body:C1=VERIFIED", "W1-body:C2=VERIFIED"}, nil},
		"duplicate mention keeps the first": {"W1.r1.md", "| C1 x | VERIFIED | a |\n### C1: x - VERIFIED\n",
			[]string{"W1:C1=VERIFIED"}, nil},
		"verdict FAILED without a failed claim": {"W1.r2.md", "- C1 VERIFIED: x\nVerdict: FAILED (coverage 60%)\n",
			[]string{"W1:C1=VERIFIED"}, []lb{{"W1", "verdict FAILED"}}},
		"qa-all": {"qa-all.r1.md", "- W1-a:C1 VERIFIED: x\n- W2-b:C1 FAILED: y\n- C9 VERIFIED: unqualified, skipped\n",
			[]string{"W1-a:C1=VERIFIED", "W2-b:C1=FAILED"}, []lb{{"W2-b", "C1"}}},
		"quality without a lane": {"quality.r1.md", "### Q1: CRAP 12 in parse\n### Q2: dup 0.9 - W1-x\nVerdict: FAILED\n",
			[]string{"quality:Q1=FAILED", "quality:Q2=FAILED"}, []lb{{"W1-x", "Q2"}}},
		"sub-claim failed, named in the verdict": {"W1-decoders.r1.md", "- C4 VERIFIED for every site. One FAILED sub-claim, below.\n- C5 VERIFIED: x\n## Verdict: C1, C5 VERIFIED. C4 FAILED on the bound (keys.go:50).\n",
			[]string{"W1-decoders:C4=FAILED", "W1-decoders:C5=VERIFIED"}, []lb{{"W1-decoders", "C4"}}},
		"no FAILED claims": {"W3-fuzz.r1.md", "- C1 VERIFIED: ok\nC1-C5 VERIFIED (5/5). No FAILED claims.\n",
			[]string{"W3-fuzz:C1=VERIFIED"}, nil},
		"quality round 2 resolves": {"quality.r2.md", "Verdict: FAILED (Q3 residual only; Q1 and Q2 VERIFIED)\n- Q1 (W1-skills) VERIFIED. extracted\n- Q2 fixed: gone\n- Q3 (W0-architect) FAILED, the CRAP part only.\n",
			[]string{"quality:Q1=VERIFIED", "quality:Q2=VERIFIED", "quality:Q3=FAILED"}, []lb{{"W0-architect", "Q3"}}},
		"findings none": {"quality.r1.md", "## Findings\nNone.\n", []string{"quality=VERIFIED"}, nil},
		"quality clean": {"quality.r1.md", "Verdict: VERIFIED (no blocking findings), notes N1-N3.\n",
			[]string{"quality=VERIFIED"}, nil},
		"security free heading": {"security.r2.md", "### S1: shell-expanded run text in W1-cli main.go\n### S2: gosec G104 - W2-x - note\nVerdict: FAILED\n",
			[]string{"security:S1=FAILED"}, []lb{{"W1-cli", "S1"}}},
		"security unflagged and clean": {"security.r1.md", "### S1: gosec G1 - W1\nVerdict: VERIFIED\n",
			[]string{"security=VERIFIED"}, nil},
	} {
		d, err := Parse(tc.file, tc.body)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		p := d.Plan()
		var got []string
		for _, v := range p.Verdicts {
			got = append(got, v.Name+"="+v.Status)
		}
		if strings.Join(got, " ") != strings.Join(tc.verdicts, " ") {
			t.Errorf("%s: verdicts %v, want %v", name, got, tc.verdicts)
		}
		var gl []lb
		for _, l := range p.LoopBacks {
			gl = append(gl, lb{l.To, l.Note})
		}
		if len(gl) != len(tc.lbs) {
			t.Errorf("%s: loop-backs %v, want %v", name, gl, tc.lbs)
			continue
		}
		for i := range gl {
			if gl[i] != tc.lbs[i] {
				t.Errorf("%s: loop-backs %v, want %v", name, gl, tc.lbs)
			}
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
