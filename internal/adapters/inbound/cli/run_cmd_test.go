package cli

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arxdsilva/ai-loops/internal/application/gate"
	"github.com/arxdsilva/ai-loops/internal/application/ticket"
	"github.com/arxdsilva/ai-loops/internal/application/tracker"
	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

type rcLog struct {
	m   map[string][]run.Event
	dir string
}

func (l *rcLog) Append(_ context.Context, id string, e run.Event) error {
	l.m[id] = append(l.m[id], e)
	return nil
}
func (l *rcLog) Create(_ context.Context, id string) error {
	if _, ok := l.m[id]; ok {
		return fs.ErrExist
	}
	l.m[id] = nil
	return nil
}
func (l *rcLog) Load(_ context.Context, id string) ([]run.Event, error) { return l.m[id], nil }
func (l *rcLog) Dir(id string) string                                   { return filepath.Join(l.dir, id) }

type rcPtr struct{ id string }

func (p *rcPtr) Current() (string, error)   { return p.id, nil }
func (p *rcPtr) SetCurrent(id string) error { p.id = id; return nil }
func (p *rcPtr) Clear() error               { p.id = ""; return nil }

type rcClock struct{}

func (rcClock) Now() time.Time { return time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC) }

type rcGate struct {
	opts gate.Options
	res  gate.Result
}

func (g *rcGate) Run(_ context.Context, o gate.Options) (gate.Result, error) {
	g.opts = o
	return g.res, nil
}

type rcTickets struct {
	src, ref, body string
	t              ticket.Ticket
}

func (f *rcTickets) Fetch(_ context.Context, s, r, b string) (ticket.Ticket, error) {
	f.src, f.ref, f.body = s, r, b
	return f.t, nil
}

type rcEnv struct {
	app      *App
	log      *rcLog
	ptr      *rcPtr
	out, err *bytes.Buffer
	gate     *rcGate
	tickets  *rcTickets
}

func rcSetup(t *testing.T, stdin string) *rcEnv {
	t.Helper()
	l := &rcLog{m: map[string][]run.Event{}, dir: t.TempDir()}
	p := &rcPtr{}
	e := &rcEnv{log: l, ptr: p, out: &bytes.Buffer{}, err: &bytes.Buffer{}, gate: &rcGate{}, tickets: &rcTickets{}}
	e.app = &App{
		Tracker: tracker.New(l, p, rcClock{}), Gate: e.gate, Tickets: e.tickets,
		Root: "/proj", In: strings.NewReader(stdin), Out: e.out, Err: e.err,
	}
	return e
}

func (e *rcEnv) do(args ...string) int {
	e.out.Reset()
	e.err.Reset()
	return e.app.Run(context.Background(), args)
}

func (e *rcEnv) events() []run.Event { return e.log.m[e.ptr.id] }

func TestRunStartAndDir(t *testing.T) {
	e := rcSetup(t, "")
	if code := e.do("run", "start", "--title", "Add orders", "--ticket", "#9", "--source", "github"); code != 0 {
		t.Fatal(code, e.err)
	}
	id := e.ptr.id
	if id != "20261109-1000-add-orders" || !strings.HasPrefix(e.out.String(), "run "+id+"\ndir ") {
		t.Fatalf("%q", e.out)
	}
	evs := e.events()
	if evs[0].Target != "#9" || evs[0].Note != "github" {
		t.Fatalf("%+v", evs[0])
	}
	if e.do("run", "dir") != 0 || strings.TrimSpace(e.out.String()) != e.log.Dir(id) {
		t.Fatal(e.out)
	}
	if e.do("run", "start") != 2 || !strings.Contains(e.err.String(), "--title") {
		t.Fatal(e.err)
	}
}

func TestRunNoActiveRun(t *testing.T) {
	e := rcSetup(t, "")
	for _, args := range [][]string{
		{"run", "phase", "plan"}, {"run", "event", "--kind", "note"}, {"run", "label", "--agent", "a", "--name", "n"},
		{"run", "loopback", "--from", "a", "--to", "b"}, {"run", "verdict", "--name", "x", "--status", "VERIFIED"},
		{"run", "end"}, {"run", "dir"},
	} {
		if code := e.do(args...); code != 1 || !strings.Contains(e.err.String(), "no active run") {
			t.Errorf("%v: code %d err %q", args, code, e.err)
		}
	}
}

func TestRunSubcommands(t *testing.T) {
	e := rcSetup(t, "")
	e.do("run", "start", "--title", "x")
	if e.do("run", "phase", "plan", "--note", "go") != 0 {
		t.Fatal(e.err)
	}
	if e.do("run", "phase", "--note", "n2", "implement") != 0 { // flag between/before positional
		t.Fatal(e.err)
	}
	if e.do("run", "phase", "bogus") != 2 || e.do("run", "phase") != 2 {
		t.Fatal("phase validation")
	}
	if e.do("run", "event", "--kind", "gate", "--name", "build", "--ok", "false", "--ms", "42", "--round", "2", "--wave", "3",
		"--target", "t", "--from", "f", "--to", "to", "--status", "s", "--role", "r", "--agent", "a", "--model", "m", "--ref", "rf", "--note", "nt", "--phase", "qa") != 0 {
		t.Fatal(e.err)
	}
	g := e.events()[len(e.events())-1]
	if g.Kind != run.KindGate || g.Name != "build" || g.OK == nil || *g.OK || g.DurationMS != 42 || g.Round != 2 || g.Wave != 3 ||
		g.Target != "t" || g.From != "f" || g.To != "to" || g.Status != "s" || g.Role != "r" || g.Agent != "a" || g.Model != "m" ||
		g.Ref != "rf" || g.Note != "nt" || g.Phase != run.PhaseQA {
		t.Fatalf("%+v", g)
	}
	if e.do("run", "event") != 2 || e.do("run", "event", "--kind", "x", "--ok", "maybe") != 2 || e.do("run", "event", "--kind", "x", "--phase", "zz") != 2 {
		t.Fatal("event validation")
	}
	if e.do("run", "label", "--agent", "ag1", "--name", "W1-d", "--role", "domain-engineer", "--wave", "1") != 0 {
		t.Fatal(e.err)
	}
	if l := e.events()[len(e.events())-1]; l.Kind != run.KindLabel || l.Agent != "ag1" || l.Name != "W1-d" || l.Wave != 1 {
		t.Fatalf("%+v", l)
	}
	if e.do("run", "label", "--agent", "x") != 2 {
		t.Fatal("label validation")
	}
	// loopback defaults to the current phase (implement)
	if e.do("run", "loopback", "--from", "qa-verifier", "--to", "W1-d", "--round", "1", "--note", "why") != 0 {
		t.Fatal(e.err)
	}
	if lb := e.events()[len(e.events())-1]; lb.Kind != run.KindLoopBack || lb.Phase != run.PhaseImplement || lb.Round != 1 || lb.Note != "why" {
		t.Fatalf("%+v", lb)
	}
	if e.do("run", "loopback", "--from", "a", "--to", "b", "--phase", "plan") != 0 || e.events()[len(e.events())-1].Phase != run.PhasePlan {
		t.Fatal("explicit phase")
	}
	if e.do("run", "loopback", "--from", "a") != 2 || e.do("run", "loopback", "--from", "a", "--to", "b", "--phase", "no") != 2 {
		t.Fatal("loopback validation")
	}
	if e.do("run", "verdict", "--name", "W1-d:C1", "--status", "verified", "--round", "1", "--ref", "qa/x.md", "--agent", "v1", "--role", "qa-verifier") != 0 {
		t.Fatal(e.err)
	}
	if v := e.events()[len(e.events())-1]; v.Kind != run.KindVerdict || v.Status != "VERIFIED" || v.Phase != run.PhaseImplement || v.Ref != "qa/x.md" || v.Agent != "v1" {
		t.Fatalf("%+v", v)
	}
	if e.do("run", "verdict", "--name", "x", "--status", "MAYBE") != 2 || e.do("run", "verdict", "--name", "x") != 2 {
		t.Fatal("verdict validation")
	}
	id := e.ptr.id
	if e.do("run", "end", "--outcome", "weird") != 2 || e.ptr.id == "" {
		t.Fatal("end validation")
	}
	if e.do("run", "end") != 0 || e.ptr.id != "" {
		t.Fatal(e.err)
	}
	if end := e.log.m[id][len(e.log.m[id])-1]; end.Kind != run.KindEnd || end.Status != "done" {
		t.Fatalf("%+v", end)
	}
	if e.do("run") != 2 || e.do("run", "nope") != 2 {
		t.Fatal("dispatch")
	}
}

func TestRunEndAbandoned(t *testing.T) {
	e := rcSetup(t, "")
	e.do("run", "start", "--title", "x")
	id := e.ptr.id
	if e.do("run", "end", "--outcome", "abandoned") != 0 || e.log.m[id][len(e.log.m[id])-1].Status != "abandoned" {
		t.Fatal(e.err)
	}
}

func TestGateCmd(t *testing.T) {
	e := rcSetup(t, "")
	e.app.GateSkip = []string{"vet"}
	e.app.GateVuln = true
	e.gate.res = gate.Result{OK: false, Checks: []gate.CheckResult{
		{Name: "build", OK: true, Duration: 1200 * time.Millisecond},
		{Name: "staticcheck", Skipped: true},
		{Name: "test", OK: false, Duration: 3400 * time.Millisecond, Output: "--- FAIL: TestX\nFAIL\n"},
	}}
	code := e.do("gate", "--pkgs", "./a/...,./b", "--only", "build,test", "--skip", "arch", "--fail-fast")
	if code != 1 {
		t.Fatal(code)
	}
	want := "✓ build 1.2s\n- staticcheck (skipped)\n✗ test 3.4s\n    --- FAIL: TestX\n    FAIL\n"
	if e.out.String() != want {
		t.Fatalf("%q", e.out)
	}
	o := e.gate.opts
	if o.Root != "/proj" || strings.Join(o.Pkgs, ",") != "./a/...,./b" || strings.Join(o.Only, ",") != "build,test" ||
		strings.Join(o.Skip, ",") != "arch" || strings.Join(o.ConfigSkip, ",") != "vet" || !o.Vuln || !o.FailFast {
		t.Fatalf("%+v", o)
	}
	e.gate.res = gate.Result{OK: true}
	e.app.GateSkip, e.app.GateVuln = nil, false
	if e.do("gate") != 0 || e.gate.opts.Vuln || len(e.gate.opts.Pkgs) != 0 {
		t.Fatal("green")
	}
	if e.do("gate", "--only", "tset") != 2 || !strings.Contains(e.err.String(), "valid: build, gofmt") || e.do("gate", "--skip", "nope") != 2 {
		t.Fatal("unknown check names")
	}
	e.gate.res = gate.Result{OK: true, Checks: []gate.CheckResult{{Name: "staticcheck", Skipped: true, Output: "too old"}}}
	e.do("gate")
	if e.out.String() != "- staticcheck (skipped)\n    too old\n" {
		t.Fatalf("%q", e.out)
	}
	if e.do("gate", "extra") != 2 || e.do("gate", "--nope") != 2 {
		t.Fatal("usage")
	}
}

func TestTicketCmd(t *testing.T) {
	e := rcSetup(t, "pasted body")
	e.tickets.t = ticket.Ticket{Source: "jira", Ref: "ABC-1", Title: "T", Body: "B"}
	e.do("run", "start", "--title", "x")
	if code := e.do("ticket", "fetch", "ABC-1", "--body-file", "-"); code != 0 {
		t.Fatal(code, e.err)
	}
	if e.tickets.src != "jira" || e.tickets.ref != "ABC-1" || e.tickets.body != "pasted body" {
		t.Fatalf("%+v", e.tickets)
	}
	if !strings.HasPrefix(e.out.String(), "# T\n") {
		t.Fatalf("%q", e.out)
	}
	dir := e.log.Dir(e.ptr.id)
	b, err := os.ReadFile(filepath.Join(dir, "ticket.md"))
	if err != nil || string(b) != e.out.String() {
		t.Fatalf("%v %q", err, b)
	}
	if n := e.events()[len(e.events())-1]; n.Kind != run.KindNote || n.Note != "ticket fetched" {
		t.Fatalf("%+v", n)
	}

	// --source flag, body file, flags after the ref, no active run.
	f := filepath.Join(t.TempDir(), "b.md")
	_ = os.WriteFile(f, []byte("file body"), 0o644)
	e.do("run", "end")
	if code := e.do("ticket", "fetch", "--source", "text", "--body-file", f); code != 0 {
		t.Fatal(code, e.err)
	}
	if e.tickets.src != "text" || e.tickets.body != "file body" || e.tickets.ref != "" {
		t.Fatalf("%+v", e.tickets)
	}
	e.do("ticket", "fetch", "#12", "--source", "gitlab")
	if e.tickets.src != "gitlab" || e.tickets.ref != "#12" {
		t.Fatalf("%+v", e.tickets)
	}
	e.do("ticket", "fetch", "#12")
	if e.tickets.src != "github" {
		t.Fatalf("detect: %+v", e.tickets)
	}

	if e.do("ticket") != 2 || e.do("ticket", "fetch", "a", "b") != 2 {
		t.Fatal("usage")
	}
	if e.do("ticket", "fetch", "--body-file", "/no/such/file") != 1 {
		t.Fatal("missing file")
	}
}

func TestRunOverride(t *testing.T) {
	e := rcSetup(t, "")
	if code := e.do("run", "override", "--rule", "R", "--project", "P"); code != 1 || !strings.Contains(e.err.String(), "no active run") {
		t.Fatalf("no run: %d %q", code, e.err)
	}
	e.do("run", "start", "--title", "x")
	e.do("run", "phase", "implement")
	if e.do("run", "override", "--rule", "SOLID §9", "--project", "AGENTS.md:12 testify", "--note", "used testify", "--agent", "a1", "--role", "domain-engineer") != 0 {
		t.Fatal(e.err)
	}
	o := e.events()[len(e.events())-1]
	if o.Kind != run.KindOverride || o.Name != "SOLID §9" || o.Target != "AGENTS.md:12 testify" || o.Note != "used testify" ||
		o.Agent != "a1" || o.Role != "domain-engineer" || o.Phase != run.PhaseImplement {
		t.Fatalf("%+v", o)
	}
	st, _ := e.app.Tracker.State(t.Context(), "")
	if len(st.Overrides) != 1 || st.Overrides[0].ProjectRule != "AGENTS.md:12 testify" {
		t.Fatalf("%+v", st.Overrides)
	}
	for _, args := range [][]string{
		{"run", "override", "--rule", "R"}, {"run", "override", "--project", "P"}, {"run", "override", "--rule", " ", "--project", "P"},
		{"run", "override", "--rule", "R", "--project", "P", "extra"}, {"run", "override", "--bogus"},
	} {
		if code := e.do(args...); code != 2 {
			t.Errorf("%v: %d", args, code)
		}
	}
}
