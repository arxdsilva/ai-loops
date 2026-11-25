package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

func writeQA(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "qa", name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func eventsOf(e *rcEnv, kind run.Kind) []run.Event {
	var out []run.Event
	for _, ev := range e.events() {
		if ev.Kind == kind {
			out = append(out, ev)
		}
	}
	return out
}

const ingestLane = "# QA W1-domain r1\n## Claims\n" +
	"### C1: a - VERIFIED\n### C2: b - VERIFIED\n### C3: c - FAILED\n### C4: d - VERIFIED\n### C5: e - VERIFIED\n### C6: f - VERIFIED\n### C7: g - VERIFIED\n" +
	"\nVerdict: FAILED\n"

func TestRunIngestLane(t *testing.T) {
	e := rcSetup(t, "")
	e.do("run", "start", "--title", "x")
	e.do("run", "phase", "qa")
	f := writeQA(t, "W1-domain.r1.md", ingestLane)
	if code := e.do("run", "ingest", f); code != 0 {
		t.Fatal(code, e.err)
	}
	want := "ingested qa/W1-domain.r1.md: 6 VERIFIED, 1 FAILED (C3); loopback qa-verifier -> W1-domain r1\n"
	if e.out.String() != want {
		t.Fatalf("out %q", e.out)
	}
	vs := eventsOf(e, run.KindVerdict)
	if len(vs) != 7 {
		t.Fatalf("%d verdicts", len(vs))
	}
	if v := vs[2]; v.Name != "W1-domain:C3" || v.Status != "FAILED" || v.Round != 1 || v.Ref != "qa/W1-domain.r1.md" || v.Role != "qa-verifier" || v.Phase != run.PhaseQA {
		t.Fatalf("%+v", v)
	}
	lb := eventsOf(e, run.KindLoopBack)
	if len(lb) != 1 || lb[0].From != "qa-verifier" || lb[0].To != "W1-domain" || lb[0].Round != 1 || lb[0].Note != "C3" || lb[0].Phase != run.PhaseQA {
		t.Fatalf("%+v", lb)
	}
	// the state reads the same names the lead records by hand
	st := run.Fold(e.events())
	if len(run.OpenFailures(st)) != 1 || len(st.LoopBacks) != 1 {
		t.Fatalf("%+v", st)
	}

	// idempotent
	n := len(e.events())
	if code := e.do("run", "ingest", f); code != 0 || len(e.events()) != n {
		t.Fatalf("re-ingest: code %d, %d new events", code, len(e.events())-n)
	}
	if !strings.HasSuffix(strings.TrimSpace(e.out.String()), "(already recorded)") {
		t.Fatalf("out %q", e.out)
	}
}

func TestRunIngestJSONAndVerdictOnlyFail(t *testing.T) {
	e := rcSetup(t, "")
	e.do("run", "start", "--title", "x")
	f := writeQA(t, "W2.r2.md", "### C1: a - VERIFIED\nVerdict: FAILED\n")
	if code := e.do("run", "ingest", f, "--json"); code != 0 {
		t.Fatal(code, e.err)
	}
	var got ingestJSON
	if err := json.Unmarshal(e.out.Bytes(), &got); err != nil {
		t.Fatal(err, e.out)
	}
	if got.Verified != 1 || got.Failed != 0 || got.Round != 2 || got.Recorded != 2 || len(got.LoopBacks) != 1 || got.LoopBacks[0].Note != "verdict FAILED" || got.Pass != "lane" {
		t.Fatalf("%+v", got)
	}
}

func TestRunIngestQuality(t *testing.T) {
	e := rcSetup(t, "")
	e.do("run", "start", "--title", "x")
	f1 := writeQA(t, "quality.r1.md", "### Q1: CRAP 14 - W1\n### Q2: dup 0.9 - W2\n### Q3: CRAP 12 - W1\nVerdict: FAILED\n")
	if code := e.do("run", "ingest", f1); code != 0 {
		t.Fatal(code, e.err)
	}
	want := "ingested qa/quality.r1.md: 0 VERIFIED, 3 FAILED (Q1, Q2, Q3); loopback qa-quality -> W1 r1, qa-quality -> W2 r1\n"
	if e.out.String() != want {
		t.Fatalf("out %q", e.out)
	}
	if lb := eventsOf(e, run.KindLoopBack); len(lb) != 2 || lb[0].Note != "Q1,Q3" {
		t.Fatalf("%+v", lb)
	}
	if len(run.OpenFailures(run.Fold(e.events()))) != 3 {
		t.Fatal("open failures")
	}
	// round 2 is clean: the pass is VERIFIED and the earlier findings close
	f2 := writeQA(t, "quality.r2.md", "## Findings\n\nVerdict: VERIFIED\n")
	if code := e.do("run", "ingest", f2); code != 0 {
		t.Fatal(code, e.err)
	}
	if !strings.Contains(e.out.String(), "4 VERIFIED, 0 FAILED") {
		t.Fatalf("out %q", e.out)
	}
	st := run.Fold(e.events())
	if open := run.OpenFailures(st); len(open) != 0 {
		t.Fatalf("still open: %+v", open)
	}
	if len(st.LoopBacks) != 2 {
		t.Fatalf("loop-backs %d", len(st.LoopBacks))
	}
}

func TestRunIngestSecurity(t *testing.T) {
	e := rcSetup(t, "")
	e.do("run", "start", "--title", "x")
	body := "### S1: gosec G304, severity medium - W1 - blocking\n### S2: govulncheck X, severity low - W2 - note\n### S3: gosec G101, severity high - W1 - blocking\nVerdict: FAILED\n"
	f := writeQA(t, "security.r1.md", body)
	if code := e.do("run", "ingest", f); code != 0 {
		t.Fatal(code, e.err)
	}
	want := "ingested qa/security.r1.md: 0 VERIFIED, 2 FAILED (S1, S3); loopback qa-security -> W1 r1\n"
	if e.out.String() != want {
		t.Fatalf("out %q", e.out)
	}
	if vs := eventsOf(e, run.KindVerdict); len(vs) != 2 || vs[0].Name != "security:S1" || vs[0].Role != "qa-security" {
		t.Fatalf("%+v", vs)
	}
	// notes only: nothing recorded for S2, the pass is VERIFIED
	e2 := rcSetup(t, "")
	e2.do("run", "start", "--title", "x")
	f2 := writeQA(t, "security.r1.md", "### S1: gosec G1, severity low - W2 - note\nVerdict: VERIFIED\n")
	if code := e2.do("run", "ingest", f2); code != 0 {
		t.Fatal(code, e2.err)
	}
	vs := eventsOf(e2, run.KindVerdict)
	if len(vs) != 1 || vs[0].Name != "security" || vs[0].Status != "VERIFIED" || len(eventsOf(e2, run.KindLoopBack)) != 0 {
		t.Fatalf("%+v", vs)
	}
}

func TestRunIngestErrors(t *testing.T) {
	e := rcSetup(t, "")
	good := writeQA(t, "W1.r1.md", "### C1: a - VERIFIED\nVerdict: VERIFIED\n")
	if code := e.do("run", "ingest", good); code != 1 || !strings.Contains(e.err.String(), "no active run") {
		t.Fatalf("no run: %d %q", code, e.err)
	}
	e.do("run", "start", "--title", "x")
	for name, tc := range map[string]struct{ file, body, want string }{
		"empty":                  {"W1.r1.md", "# QA\nnothing\n", "no claims"},
		"bad name":               {"notes.md", "### C1: a - VERIFIED\n", "not a QA file name"},
		"no round":               {"W1.md", "Verdict: VERIFIED\n", "not a QA file name"},
		"claim without a status": {"W1.r1.md", "### C1: a\n", "no claims"},
	} {
		f := writeQA(t, tc.file, tc.body)
		if code := e.do("run", "ingest", f); code != 2 || !strings.Contains(e.err.String(), tc.want) {
			t.Errorf("%s: code %d err %q", name, code, e.err)
		}
	}
	if e.do("run", "ingest") != 2 || e.do("run", "ingest", "a", "b") != 2 {
		t.Error("arg count")
	}
	if code := e.do("run", "ingest", filepath.Join(t.TempDir(), "missing.r1.md")); code != 1 {
		t.Errorf("missing file: %d", code)
	}
	if len(eventsOf(e, run.KindVerdict)) != 0 {
		t.Error("an error recorded verdicts")
	}
}

func TestRunPhasePrintsCheckpoint(t *testing.T) {
	e := rcSetup(t, "")
	e.do("run", "start", "--title", "x")
	e.do("run", "ask", "plan,qa")
	for ph, want := range map[string]string{"plan": "Checkpoint: ask\n", "implement": "Checkpoint: auto\n", "qa": "Checkpoint: ask\n"} {
		if code := e.do("run", "phase", ph); code != 0 {
			t.Fatal(code, e.err)
		}
		if !strings.HasSuffix(e.out.String(), want) {
			t.Errorf("phase %s: %q", ph, e.out)
		}
		// the same answer `loops run checkpoint <p>` gives
		e.do("run", "checkpoint", ph)
		if "Checkpoint: "+e.out.String() != want {
			t.Errorf("checkpoint %s: %q", ph, e.out)
		}
	}
	e2 := rcSetup(t, "")
	if code := e2.do("run", "phase", "plan"); code != 1 {
		t.Errorf("no run: %d", code)
	}
}

// --- stats ---

type fakeLogs struct {
	dirs   []string
	events map[string][]run.Event
}

func (f fakeLogs) Dirs(string) ([]string, error)        { return f.dirs, nil }
func (f fakeLogs) Events(d string) ([]run.Event, error) { return f.events[d], nil }
func statsAt(m int, e run.Event) run.Event {
	e.Time = time.Date(2026, 11, 9, 10, m, 0, 0, time.UTC)
	return e
}

func statsApp(t *testing.T, n int) *rcEnv {
	e := rcSetup(t, "")
	logs := fakeLogs{events: map[string][]run.Event{}}
	for i := 0; i < n; i++ {
		d := fmt.Sprintf("/runs/20261109-10%02d-r%d", i, i)
		logs.dirs = append(logs.dirs, d)
		logs.events[d] = []run.Event{
			statsAt(0, run.Event{Kind: run.KindRun, Run: filepath.Base(d)}),
			statsAt(0, run.Event{Kind: run.KindPhase, Phase: run.PhaseImplement}),
			statsAt(1, run.Event{Kind: run.KindAgentStart, Agent: "a", Role: "domain-engineer"}),
			statsAt(5, run.Event{Kind: run.KindAgentStop, Agent: "a", Tokens: int64(1000 * (i + 1))}),
			statsAt(6, run.Event{Kind: run.KindLoopBack, From: "qa-verifier"}),
			statsAt(7, run.Event{Kind: run.KindNote, Name: "lead-tokens", Tokens: 500}),
			statsAt(10, run.Event{Kind: run.KindEnd}),
		}
	}
	logs.dirs = append(logs.dirs, "/runs/empty")
	e.app.Runs = logs
	return e
}

func TestStatsText(t *testing.T) {
	e := statsApp(t, 30)
	if code := e.do("stats"); code != 0 {
		t.Fatal(code, e.err)
	}
	lines := strings.Split(strings.TrimRight(e.out.String(), "\n"), "\n")
	if len(lines) > 60 {
		t.Errorf("%d lines for 30 runs", len(lines))
	}
	first := "20261109-1000-r0 tier=- wall=10.0m agents=1 peak=1 tok=1.0k lead=500 lb=1 test=0ms denies=0 | implement:10.0(6.0)"
	if lines[0] != first {
		t.Errorf("run line %q", lines[0])
	}
	out := e.out.String()
	for _, want := range []string{"-- 30 runs, median wall 10.0m", "median per phase: implement 10.0(6.0)", "domain-engineer", "lead"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
}

func TestStatsJSONAndDirs(t *testing.T) {
	e := statsApp(t, 2)
	if code := e.do("stats", "--json", "/runs/20261109-1001-r1"); code != 0 {
		t.Fatal(code, e.err)
	}
	var got statsJSON
	if err := json.Unmarshal(e.out.Bytes(), &got); err != nil {
		t.Fatal(err, e.out)
	}
	if len(got.Runs) != 1 || got.Runs[0].ID != "20261109-1001-r1" || got.Summary.Runs != 1 || got.Runs[0].TotalTokens() != 2000 {
		t.Fatalf("%+v", got)
	}
	// a run with no events is skipped; none at all is not an error
	e.app.Runs = fakeLogs{dirs: []string{"/x"}}
	if code := e.do("stats"); code != 0 || !strings.Contains(e.out.String(), "no runs") {
		t.Fatalf("%d %q", code, e.out)
	}
	if code := e.do("stats", "--json"); code != 0 || !strings.Contains(e.out.String(), `"runs": []`) {
		t.Fatalf("%d %q", code, e.out)
	}
	e.app.Runs = nil
	if e.do("stats") != 1 || e.do("stats", "--bogus") != 2 {
		t.Error("nil reader / bad flag")
	}
}

func TestFmtTok(t *testing.T) {
	for n, want := range map[int64]string{0: "0", 999: "999", 1500: "1.5k", 12000: "12k", 2_500_000: "2.5M"} {
		if got := fmtTok(n); got != want {
			t.Errorf("fmtTok(%d) = %q, want %q", n, got, want)
		}
	}
}
