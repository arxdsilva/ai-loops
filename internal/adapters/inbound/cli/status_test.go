package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arxdsilva/ai-loops/internal/application/tracker"
	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

type fakeTracker struct {
	st     run.State
	events []run.Event
	dir    string
	none   bool
	empty  bool
	gotID  string
}

func (f *fakeTracker) Start(context.Context, tracker.StartInput) (string, error) { return "", nil }
func (f *fakeTracker) Record(context.Context, run.Event) error                   { return nil }
func (f *fakeTracker) End(context.Context, string) error                         { return nil }
func (f *fakeTracker) State(_ context.Context, id string) (run.State, error) {
	f.gotID = id
	if f.none {
		return run.State{}, tracker.ErrNoActiveRun
	}
	return f.st, nil
}
func (f *fakeTracker) Events(context.Context, string) ([]run.Event, error) {
	if f.none {
		return nil, errors.New("none")
	}
	if f.events == nil && !f.empty {
		return []run.Event{{Kind: run.KindRun}}, nil
	}
	return f.events, nil
}
func (f *fakeTracker) Dir(context.Context, string) (string, error) {
	if f.none {
		return "", tracker.ErrNoActiveRun
	}
	return f.dir, nil
}

type fakeReporter struct{ err error }

func (r fakeReporter) Render(w io.Writer, s run.State, _ []run.Event, _ string) error {
	if r.err != nil {
		return r.err
	}
	_, err := io.WriteString(w, "<html>"+s.ID+"</html>")
	return err
}

func ptrTrue() *bool { b := true; return &b }

// fixtureState is a realistic mid-QA run.
func fixtureState() run.State {
	t0 := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	return run.State{
		ID: "r1", Title: "Add rate limiting to the public API gateway", Ticket: "GH-42",
		Started: t0, Updated: t0.Add(65 * time.Minute),
		Phase: run.PhaseQA, PlanRounds: 2, Steps: 87,
		Lanes: []run.Lane{
			{Agent: "a1", Label: "W1-domain", Role: "domain-engineer", Wave: 1, Steps: 30, QA: run.StatusVerified},
			{Agent: "a2", Label: "W2-adapter", Role: "adapter-engineer", Wave: 2, Running: true, Steps: 12, QA: run.StatusFailed},
			{Agent: "a3", Role: "qa-verifier", Running: true, Steps: 5},
		},
		Gates:     []run.GateResult{{Name: "build", OK: true}, {Name: "vet", OK: false}},
		LoopBacks: []run.LoopBack{{From: "qa-verifier", To: "adapter-engineer", Phase: run.PhaseQA, Round: 1}},
	}
}

func newApp(tr *fakeTracker, stdin string) (*App, *bytes.Buffer, *bytes.Buffer) {
	out, errb := &bytes.Buffer{}, &bytes.Buffer{}
	return &App{Tracker: tr, Report: fakeReporter{}, In: strings.NewReader(stdin), Out: out, Err: errb}, out, errb
}

func TestStatusStatuslineActive(t *testing.T) {
	a, out, errb := newApp(&fakeTracker{st: fixtureState()}, `{"session_id":"s","model":{"display_name":"Opus"}}`)
	if code := a.Run(context.Background(), []string{"status", "--statusline"}); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if errb.Len() != 0 {
		t.Fatalf("stderr: %q", errb.String())
	}
	line := out.String()
	if strings.Count(line, "\n") != 1 {
		t.Fatalf("want one line: %q", line)
	}
	plain := stripANSI(line)
	for _, want := range []string{"▶ GH-42", "5/7 QA", "W2", "2/3 lanes", "QA 1✓ 1✗ 1↺", "plan r2", "build✓", "vet✗", "87 steps", "1h05m"} {
		if !strings.Contains(plain, want) {
			t.Errorf("missing %q in %q", want, plain)
		}
	}
	if !strings.Contains(line, ansiGreen) || !strings.Contains(line, ansiRed) || !strings.Contains(line, ansiYellow) {
		t.Errorf("expected colours: %q", line)
	}
	t.Log(plain)
}

func TestStatuslineOmitsEmptySegmentsAndTruncates(t *testing.T) {
	s := run.State{ID: "r", Title: "A very long title that exceeds twenty-four characters", Phase: run.PhaseIntake}
	plain := stripANSI(statusLine(statusInput{}, s, true))
	if strings.Contains(plain, "lanes") || strings.Contains(plain, "QA") || strings.Contains(plain, "plan r") || strings.Contains(plain, "steps") || strings.Contains(plain, "gates") {
		t.Errorf("empty segments shown: %q", plain)
	}
	if !strings.Contains(plain, "A very long title that …") {
		t.Errorf("title not truncated to 24: %q", plain)
	}
	if !strings.Contains(plain, "1/7 Intake") {
		t.Errorf("phase: %q", plain)
	}
}

func TestStatuslineIdleAndTolerantStdin(t *testing.T) {
	for _, in := range []string{"", "not json", "{}"} {
		a, out, errb := newApp(&fakeTracker{none: true}, in)
		if code := a.Run(context.Background(), []string{"status", "--statusline"}); code != 0 {
			t.Fatalf("exit %d", code)
		}
		if stripANSI(out.String()) != "loops · idle\n" || !strings.Contains(out.String(), ansiDim) || errb.Len() != 0 {
			t.Errorf("idle out=%q err=%q", out.String(), errb.String())
		}
	}
	a := &App{Tracker: &fakeTracker{none: true}, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}} // nil In
	if code := a.Run(context.Background(), []string{"status", "--statusline"}); code != 0 {
		t.Fatal("nil stdin must exit 0")
	}
}

func TestStatusJSONEnvelope(t *testing.T) {
	a, out, _ := newApp(&fakeTracker{st: fixtureState(), dir: "/runs/r1"}, "")
	if code := a.Run(context.Background(), []string{"status", "--json"}); code != 0 {
		t.Fatalf("exit %d", code)
	}
	var env struct {
		Active bool            `json:"active"`
		Dir    string          `json:"dir"`
		State  json.RawMessage `json:"state"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if !env.Active || env.Dir != "/runs/r1" {
		t.Errorf("env %+v", env)
	}
	var st run.State
	if err := json.Unmarshal(env.State, &st); err != nil || st.ID != "r1" || len(st.Lanes) != 3 {
		t.Errorf("state: %v %+v", err, st)
	}
	if !strings.Contains(out.String(), "\n  ") {
		t.Error("not indented")
	}
}

func TestStatusJSONNoRun(t *testing.T) {
	a, out, _ := newApp(&fakeTracker{none: true}, "")
	if code := a.Run(context.Background(), []string{"status", "--json"}); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if strings.TrimSpace(out.String()) != `{"active":false}` && strings.Join(strings.Fields(out.String()), "") != `{"active":false}` {
		t.Errorf("got %q", out.String())
	}
}

func TestStatusPlain(t *testing.T) {
	a, out, _ := newApp(&fakeTracker{st: fixtureState()}, "")
	if code := a.Run(context.Background(), []string{"status"}); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if strings.Contains(out.String(), "\x1b") || !strings.Contains(out.String(), "GH-42") || strings.Count(out.String(), "\n") != 1 {
		t.Errorf("plain: %q", out.String())
	}
	a, out, _ = newApp(&fakeTracker{none: true}, "")
	if code := a.Run(context.Background(), []string{"status"}); code != 0 || !strings.Contains(out.String(), "no active run") {
		t.Errorf("none: %d %q", code, out.String())
	}
	a, _, _ = newApp(&fakeTracker{none: true}, "")
	if code := a.Run(context.Background(), []string{"status", "--bogus"}); code != 2 {
		t.Errorf("bad flag exit %d", code)
	}
}

func TestStatusEndedPlain(t *testing.T) {
	s := fixtureState()
	s.Ended, s.Outcome = true, "done"
	if got := plainLine(s); !strings.HasPrefix(got, "■ GH-42") || !strings.HasSuffix(got, "· done") {
		t.Errorf("%q", got)
	}
}

func TestSubagentLines(t *testing.T) {
	in := `{"columns":200,"tasks":[
	 {"id":"t1","agentType":"loops:adapter-engineer","description":"implement the HTTP adapter","status":"running"},
	 {"id":"t2","agentType":"general-purpose","description":"x"},
	 {"id":"t3","agentType":"loops:researcher-web","description":"research rate limiting libraries in Go and compare their design tradeoffs"}]}`
	a, out, errb := newApp(&fakeTracker{st: fixtureState()}, in)
	if code := a.Run(context.Background(), []string{"status", "--subagent"}); code != 0 {
		t.Fatalf("exit %d", code)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 || errb.Len() != 0 {
		t.Fatalf("lines %q err %q", lines, errb.String())
	}
	var l1, l2 struct{ ID, Content string }
	_ = json.Unmarshal([]byte(lines[0]), &l1)
	_ = json.Unmarshal([]byte(lines[1]), &l2)
	if l1.ID != "t1" || l1.Content != "adapter-engineer · W2-adapter · 12 steps · QA failed" {
		t.Errorf("l1 %+v", l1)
	}
	if l2.ID != "t3" || !strings.HasPrefix(l2.Content, "researcher-web · research rate limiting libraries in Go … · 0 steps · QA pending") {
		t.Errorf("l2 %+v", l2)
	}
	t.Log(lines[0])
}

func TestSubagentTruncatesAndIdle(t *testing.T) {
	in := `{"columns":20,"tasks":[{"id":"t1","agentType":"loops:adapter-engineer"}]}`
	a, out, _ := newApp(&fakeTracker{st: fixtureState()}, in)
	a.Run(context.Background(), []string{"status", "--subagent"})
	var l struct{ Content string }
	_ = json.Unmarshal(out.Bytes(), &l)
	if n := len([]rune(l.Content)); n != 20 {
		t.Errorf("len %d: %q", n, l.Content)
	}
	for _, stdin := range []string{in, "", "garbage"} {
		a, out, errb := newApp(&fakeTracker{none: true}, stdin)
		if code := a.Run(context.Background(), []string{"status", "--subagent"}); code != 0 || out.Len() != 0 || errb.Len() != 0 {
			t.Errorf("idle: %d %q %q", code, out.String(), errb.String())
		}
	}
	a, out, _ = newApp(&fakeTracker{st: fixtureState()}, "garbage")
	if code := a.Run(context.Background(), []string{"status", "--subagent"}); code != 0 || out.Len() != 0 {
		t.Errorf("garbage stdin: %d %q", code, out.String())
	}
}

func TestElapsed(t *testing.T) {
	t0 := time.Now()
	if got := elapsed(run.State{Started: t0, Updated: t0.Add(12 * time.Minute)}); got != "12m" {
		t.Error(got)
	}
	if got := elapsed(run.State{Started: t0, Updated: t0.Add(65 * time.Minute)}); got != "1h05m" {
		t.Error(got)
	}
	if elapsed(run.State{}) != "" || truncRunes("abc", 1) != "a" || truncRunes("abc", 0) != "abc" {
		t.Error("edge")
	}
}

func TestReportWritesFileAtomically(t *testing.T) {
	dir := t.TempDir()
	tr := &fakeTracker{st: fixtureState(), dir: dir}
	a, out, _ := newApp(tr, "")
	if code := a.Run(context.Background(), []string{"report"}); code != 0 {
		t.Fatalf("exit %d", code)
	}
	want := filepath.Join(dir, "report.html")
	if strings.TrimSpace(out.String()) != want {
		t.Errorf("printed %q", out.String())
	}
	b, err := os.ReadFile(want)
	if err != nil || string(b) != "<html>r1</html>" {
		t.Errorf("file %q %v", b, err)
	}
	if ents, _ := os.ReadDir(dir); len(ents) != 1 {
		t.Errorf("temp files left: %v", ents)
	}
	// --run and --out
	custom := filepath.Join(dir, "x.html")
	a, out, _ = newApp(tr, "")
	if code := a.Run(context.Background(), []string{"report", "--run", "r9", "--out", custom}); code != 0 || tr.gotID != "r9" {
		t.Fatalf("exit/id %q", tr.gotID)
	}
	if _, err := os.Stat(custom); err != nil {
		t.Error(err)
	}
}

func TestReportErrors(t *testing.T) {
	a, _, errb := newApp(&fakeTracker{none: true}, "")
	if code := a.Run(context.Background(), []string{"report"}); code != 1 || errb.Len() == 0 {
		t.Errorf("no run: %d", code)
	}
	a, _, _ = newApp(&fakeTracker{st: fixtureState(), dir: t.TempDir()}, "")
	a.Report = fakeReporter{err: errors.New("boom")}
	if code := a.Run(context.Background(), []string{"report"}); code != 1 {
		t.Errorf("render err: %d", code)
	}
	a, _, _ = newApp(&fakeTracker{st: fixtureState(), dir: t.TempDir()}, "")
	if code := a.Run(context.Background(), []string{"report", "--out", "/nonexistent-dir/x.html"}); code != 1 {
		t.Errorf("write err: %d", code)
	}
	if code := a.Run(context.Background(), []string{"report", "--nope"}); code != 2 {
		t.Errorf("flag err: %d", code)
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func TestBlockedStdinDoesNotHang(t *testing.T) {
	for _, mode := range []string{"--statusline", "--subagent"} {
		pr, pw := io.Pipe() // never closed
		defer pw.Close()
		a, out, _ := newApp(&fakeTracker{st: fixtureState()}, "")
		a.In = pr
		start := time.Now()
		if code := a.Run(context.Background(), []string{"status", mode}); code != 0 {
			t.Fatalf("%s exit %d", mode, code)
		}
		if d := time.Since(start); d > 800*time.Millisecond {
			t.Errorf("%s took %v", mode, d)
		}
		if mode == "--statusline" && !strings.Contains(out.String(), "GH-42") {
			t.Errorf("no line: %q", out.String())
		}
	}
}

func TestBoundedReadStopsOnCompleteJSONAndCap(t *testing.T) {
	pr, pw := io.Pipe()
	defer pw.Close()
	go pw.Write([]byte(`{"columns":5}`))
	start := time.Now()
	if got := string(readBounded(pr)); got != `{"columns":5}` || time.Since(start) > 200*time.Millisecond {
		t.Errorf("%q %v", got, time.Since(start))
	}
	if n := len(readBounded(strings.NewReader(strings.Repeat("x", 200<<10)))); n != stdinCap {
		t.Errorf("cap %d", n)
	}
}

func TestReportUnknownRun(t *testing.T) {
	tr := &fakeTracker{empty: true, dir: t.TempDir()}
	for _, args := range [][]string{{"report", "--run", "nope", "--out", filepath.Join(tr.dir, "x.html")}, {"report", "--run", "nope"}} {
		a, out, errb := newApp(tr, "")
		if code := a.Run(context.Background(), args); code != 1 || out.Len() != 0 || !strings.Contains(errb.String(), `run "nope" not found`) {
			t.Errorf("%v: %d %q", args, code, errb.String())
		}
	}
	if _, err := os.Stat(filepath.Join(tr.dir, "x.html")); err == nil {
		t.Error("file written")
	}
}
