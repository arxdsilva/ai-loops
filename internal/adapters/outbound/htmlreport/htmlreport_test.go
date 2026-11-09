package htmlreport

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/insurgencygames/ai-loops/internal/domain/run"
)

func tp(b bool) *bool { return &b }

var t0 = time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)

func at(m int) time.Time { return t0.Add(time.Duration(m) * time.Minute) }

func fixture() (run.State, []run.Event) {
	s := run.State{
		ID: "r1", Title: "Add <b>rate</b> limiting", Ticket: "https://github.com/acme/api/issues/42", Source: "github",
		Started: t0, Updated: at(95), Phase: run.PhaseReflect, Ended: true, Outcome: run.OutcomeDone,
		PR: "https://github.com/acme/api/pull/43", PlanRounds: 2, Steps: 4, Errors: 1, Denials: 1, MemoryQ: 1, MemoryW: 1,
		PhaseLog: []run.PhaseEntry{
			{Phase: run.PhaseIntake, At: t0}, {Phase: run.PhaseResearch, At: at(5)}, {Phase: run.PhasePlan, At: at(15)},
			{Phase: run.PhaseImplement, At: at(30)}, {Phase: run.PhaseQA, At: at(60)}, {Phase: run.PhaseImplement, At: at(70)},
			{Phase: run.PhaseQA, At: at(75)}, {Phase: run.PhaseProduct, At: at(85)}, {Phase: run.PhaseReflect, At: at(90)},
		},
		Lanes: []run.Lane{
			{Agent: "a1", Label: "W1-domain", Role: "domain-engineer", Model: "sonnet", Wave: 1, Steps: 3, Started: at(30), Stopped: at(58), OK: tp(true), QA: run.StatusVerified, QARounds: 2},
			{Agent: "a2", Role: "qa-verifier", Started: at(60), Running: true, Steps: 1},
		},
		Gates:     []run.GateResult{{Name: "build", OK: true, DurationMS: 900}, {Name: "vet", OK: false, Note: "bad <thing>"}},
		Verdicts:  []run.VerdictInfo{{Name: "plan", Status: run.StatusApprove, Phase: run.PhasePlan, Round: 2}, {Name: "W1-domain:C1", Status: run.StatusVerified, Phase: run.PhaseQA}, {Name: "AC1", Status: run.StatusVerified, Phase: run.PhaseProduct}},
		LoopBacks: []run.LoopBack{{From: "qa-verifier", To: "domain-engineer", Phase: run.PhaseQA, Round: 1, Reason: "claim C1 false <x>"}},
		Proofs:    []run.ProofInfo{{Name: "AC1", Ref: "proofs/ac1.txt", Note: "429"}, {Name: "AC2", Ref: "../../etc/passwd"}},
	}
	ev := []run.Event{
		{Time: at(31), Kind: run.KindStep, Agent: "a1", Name: "Edit", Target: "internal/limiter.go", OK: tp(true), DurationMS: 12},
		{Time: at(32), Kind: run.KindStep, Agent: "a1", Name: "Bash", Target: "go test", OK: tp(false), DurationMS: 800},
		{Time: at(33), Kind: run.KindStep, Name: "Read", Target: "ticket.md", OK: tp(true)},
		{Time: at(61), Kind: run.KindGate, Name: "build", OK: tp(true), DurationMS: 900},
		{Time: at(62), Kind: run.KindGuardDeny, Agent: "a1", Name: "no-main-edit", Note: "denied edit"},
		{Time: at(63), Kind: run.KindMemory, Name: "query", Target: "limiter", Note: "found 2"},
	}
	return s, ev
}

func render(t *testing.T, s run.State, ev []run.Event, dir string) string {
	t.Helper()
	var b bytes.Buffer
	if err := New().Render(&b, s, ev, dir); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestRenderFullRun(t *testing.T) {
	s, ev := fixture()
	out := render(t, s, ev, "testdata/run1")
	for _, want := range []string{
		"loops report", "Add &lt;b&gt;rate&lt;/b&gt; limiting", `href="https://github.com/acme/api/issues/42"`, `href="https://github.com/acme/api/pull/43"`,
		"1h35m", "done", "1× re-entered", "W1-domain", "domain-engineer", "sonnet", "W1",
		"QA VERIFIED r2", "qa-verifier → domain-engineer", "claim C1 false &lt;x&gt;", "Plan rounds", "AC1", `href="proofs/ac1.txt"`,
		"vet", "bad &lt;thing&gt;", "History (1)", "internal/limiter.go", `id="lane-filter"`, `data-lane="W1-domain"`, `data-lane="lead"`,
		"no-main-edit", "found 2", "Reflection", "Went well.", "Closes #42", "Plan v2", "prefers-color-scheme",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(out, "# Plan v1") {
		t.Error("older plan inlined")
	}
	// escaping: ticket.md contains a script tag
	if strings.Contains(out, "<script>alert(1)</script>") {
		t.Error("unescaped ticket script")
	}
	if !strings.Contains(out, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Error("ticket text not escaped-inlined")
	}
	if strings.Contains(out, "etc/passwd\"") && strings.Contains(out, `href="../`) {
		t.Error("unsafe proof link")
	}
	if strings.Contains(out, `href="../../etc`) {
		t.Error("traversal proof linked")
	}
	t.Logf("%d bytes", len(out))
}

var extRe = regexp.MustCompile(`(?i)<(script|link|img|iframe)[^>]+(src|href)\s*=\s*["']?(https?:)?//`)

func TestOfflineAndWellFormed(t *testing.T) {
	s, ev := fixture()
	for name, out := range map[string]string{
		"full":  render(t, s, ev, "testdata/run1"),
		"empty": render(t, run.State{}, nil, ""),
	} {
		if m := extRe.FindString(out); m != "" {
			t.Errorf("%s: external resource %q", name, m)
		}
		for _, bad := range []string{"@import", "url(http", "<link"} {
			if strings.Contains(out, bad) {
				t.Errorf("%s: contains %q", name, bad)
			}
		}
		if err := checkBalanced(out); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		if !strings.HasPrefix(out, "<!doctype html>") || !strings.Contains(out, `name="viewport"`) {
			t.Errorf("%s: doctype/viewport", name)
		}
	}
}

func TestRenderEmptyAndPartial(t *testing.T) {
	out := render(t, run.State{}, nil, "")
	for _, want := range []string{"(untitled run)", "in progress", "No lanes recorded", "No verdicts", "No gates run", "No steps recorded", "Intake"} {
		if !strings.Contains(out, want) {
			t.Errorf("empty: missing %q", want)
		}
	}
	// lane without start/stop, ticket that is not a URL, missing dir
	s := run.State{ID: "x", Ticket: "GH-1", Lanes: []run.Lane{{Agent: "z", Running: true}}, Phase: run.PhasePlan,
		PhaseLog: []run.PhaseEntry{{Phase: run.PhasePlan, At: t0}}, PR: "javascript:alert(1)"}
	out = render(t, s, nil, "/definitely/missing")
	if !strings.Contains(out, "ticket GH-1") || strings.Contains(out, `href="javascript`) {
		t.Errorf("partial: ticket/pr link handling")
	}
	if strings.Contains(out, "NaN") || strings.Contains(out, "Inf") {
		t.Error("NaN in output")
	}
	if err := checkBalanced(out); err != nil {
		t.Error(err)
	}
}

func TestGanttGeometry(t *testing.T) {
	s, _ := fixture()
	lv := laneViews(s)
	if len(lv) != 2 {
		t.Fatal(len(lv))
	}
	if lv[0].Left != "31.58" || lv[0].Class != "ok" || lv[1].Class != "run" {
		t.Errorf("%+v", lv[0])
	}
	// running lane extends to the end
	if lv[1].Left != "63.16" || lv[1].Width != "36.84" {
		t.Errorf("%+v", lv[1])
	}
}

func TestPhaseTimes(t *testing.T) {
	s, _ := fixture()
	pv := phaseViews(s)
	if len(pv) != 7 || pv[0].Time != "5m00s" || pv[3].Time != "35m00s" || pv[3].Reentries != 1 || pv[4].Reentries != 1 {
		t.Errorf("%+v", pv)
	}
	if pv[6].Class != "done" { // run ended: no phase is "current"
		t.Errorf("ended class %q", pv[6].Class)
	}
}

func TestSafeRel(t *testing.T) {
	for in, want := range map[string]string{"proofs/a.txt": "proofs/a.txt", "./a.txt": "a.txt", "../a": "", "/etc/x": "", "http://x/y": "", "": "", "a/../../b": ""} {
		if got := safeRel(in); got != want {
			t.Errorf("safeRel(%q)=%q want %q", in, got, want)
		}
	}
}

func TestFmtDur(t *testing.T) {
	for d, want := range map[time.Duration]string{-1: "", 0: "0s", 5 * time.Second: "5s", 65 * time.Second: "1m05s", 65 * time.Minute: "1h05m"} {
		if got := fmtDur(d); got != want {
			t.Errorf("%v: %q want %q", d, got, want)
		}
	}
}

func TestStepCapAndLatestPlan(t *testing.T) {
	var ev []run.Event
	for i := 0; i < stepCap+5; i++ {
		ev = append(ev, run.Event{Kind: run.KindStep, Name: "Read", OK: tp(true)})
	}
	out := render(t, run.State{}, ev, "")
	if !strings.Contains(out, "showing first 3000") {
		t.Error("no truncation note")
	}
	if got := latestPlan("testdata/run1"); got != "plan.v2.md" {
		t.Errorf("latest plan %q", got)
	}
}

// checkBalanced is a minimal tokenizer: every non-void open tag must close in order.
func checkBalanced(doc string) error {
	void := map[string]bool{"meta": true, "br": true, "link": true, "img": true, "input": true, "hr": true}
	raw := map[string]bool{"style": true, "script": true, "pre": false}
	var stack []string
	i := 0
	for i < len(doc) {
		j := strings.IndexByte(doc[i:], '<')
		if j < 0 {
			break
		}
		i += j
		if strings.HasPrefix(doc[i:], "<!") {
			i += strings.IndexByte(doc[i:], '>') + 1
			continue
		}
		end := strings.IndexByte(doc[i:], '>')
		if end < 0 {
			return errString("unterminated tag")
		}
		tag := doc[i+1 : i+end]
		i += end + 1
		if strings.HasPrefix(tag, "/") {
			name := strings.ToLower(strings.TrimSpace(tag[1:]))
			if len(stack) == 0 || stack[len(stack)-1] != name {
				return errString("mismatched </" + name + ">")
			}
			stack = stack[:len(stack)-1]
			continue
		}
		name := strings.ToLower(strings.FieldsFunc(tag, func(r rune) bool { return r == ' ' || r == '\n' || r == '\t' || r == '/' })[0])
		if void[name] {
			continue
		}
		if raw[name] {
			k := strings.Index(strings.ToLower(doc[i:]), "</"+name+">")
			if k < 0 {
				return errString("unclosed " + name)
			}
			i += k + len(name) + 3
			continue
		}
		stack = append(stack, name)
	}
	if len(stack) != 0 {
		return errString("unclosed: " + strings.Join(stack, ","))
	}
	return nil
}

type errString string

func (e errString) Error() string { return string(e) }
