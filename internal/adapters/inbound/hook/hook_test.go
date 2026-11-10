package hook

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

type fakeRec struct {
	evs   []run.Event
	cur   string
	state run.State
}

func (r *fakeRec) Record(_ context.Context, e run.Event) error { r.evs = append(r.evs, e); return nil }
func (r *fakeRec) Current(context.Context) (string, error)     { return r.cur, nil }
func (r *fakeRec) State(context.Context, string) (run.State, error) {
	if r.cur == "" {
		return run.State{}, errors.New("none")
	}
	return r.state, nil
}
func (r *fakeRec) Events(context.Context, string) ([]run.Event, error) { return r.evs, nil }
func (r *fakeRec) Dir(_ context.Context, id string) (string, error)    { return "/d/" + id, nil }

type fakeFiles struct {
	lines map[string]int
}

func (f fakeFiles) Lines(p string) (int, error)   { return f.lines[p], nil }
func (f fakeFiles) Exists(p string) bool          { _, ok := f.lines[p]; return ok }
func (f fakeFiles) Glob(string) ([]string, error) { return nil, nil }

type fakeCmd struct {
	calls []string
	fail  map[string]string
	has   bool
}

func (c *fakeCmd) Run(_ context.Context, dir, name string, args ...string) (string, error) {
	call := name + " " + strings.Join(args, " ")
	c.calls = append(c.calls, call)
	for k, v := range c.fail {
		if strings.HasPrefix(call, k) {
			return v, errors.New("exit 1")
		}
	}
	return "", nil
}
func (c *fakeCmd) Has(string) bool { return c.has }

type env struct {
	h    *Handler
	rec  *fakeRec
	cmd  *fakeCmd
	vars map[string]string
}

func setup(cur string) *env {
	e := &env{rec: &fakeRec{cur: cur}, cmd: &fakeCmd{}, vars: map[string]string{"CLAUDE_PROJECT_DIR": "/repo"}}
	e.h = New(Deps{Rec: e.rec, Files: fakeFiles{lines: map[string]int{"/repo/big.go": 600, "/repo/pkg/a.go": 10}}, Cmd: e.cmd,
		Env: func(k string) string { return e.vars[k] }, WriteFile: func(string, []byte) error { return nil }})
	return e
}

func (e *env) run(name, in string) (int, string, string) {
	var o, er bytes.Buffer
	code := e.h.Handle(context.Background(), name, "", strings.NewReader(in), &o, &er)
	return code, o.String(), er.String()
}

func TestFailOpen(t *testing.T) {
	e := setup("")
	for _, n := range []string{"session-start", "read-guard", "model-guard", "go-post-edit", "trace", "trace-agent", "stop-gate", "unknown"} {
		if c, o, er := e.run(n, "not json"); c != 0 || o != "" || er != "" {
			t.Errorf("%s: %d %q %q", n, c, o, er)
		}
	}
}

// panicFiles blows up mid-hook, after the guard may have written output.
type panicFiles struct{ fakeFiles }

func (panicFiles) Lines(string) (int, error) { panic("boom") }

// A panic anywhere fails open: exit 0, nothing on stdout or stderr.
func TestPanicFailsOpen(t *testing.T) {
	e := setup("r")
	e.h = New(Deps{Rec: e.rec, Files: panicFiles{}, Cmd: e.cmd, Env: func(k string) string { return e.vars[k] }})
	read := `{"tool_name":"Read","tool_input":{"file_path":"/repo/big.go"},"cwd":"/repo","agent_id":"a1"}`
	if c, o, er := e.run("read-guard", read); c != 0 || o != "" || er != "" {
		t.Fatalf("%d %q %q", c, o, er)
	}
	// Without a panic the buffered output still reaches the writers.
	e = setup("r")
	if c, _, er := e.run("read-guard", read); c != 2 || !strings.Contains(er, "read_guard:") {
		t.Fatalf("%d %q", c, er)
	}
}

func TestSessionStart(t *testing.T) {
	e := setup("")
	if c, o, _ := e.run("session-start", "{}"); c != 0 || o != "" {
		t.Fatalf("%q", o)
	}
	e.vars["LOOPS_PLUGIN_ROOT"] = "/plug"
	_, o, _ := e.run("session-start", "{}")
	if !strings.Contains(o, `"hookEventName":"SessionStart"`) || !strings.Contains(o, "Plugin root: /plug. Work tickets with /loops:work (WORKFLOW: /plug/specs/WORKFLOW.md).") || strings.Contains(o, "Active run") {
		t.Fatalf("%q", o)
	}
	e.rec.cur = "r1"
	e.rec.state = run.State{Phase: run.PhasePlan, Lanes: []run.Lane{{Running: true}, {}}}
	_, o, _ = e.run("session-start", "{}")
	if !strings.Contains(o, "Active run r1: phase plan, 1 lanes running, run dir /d/r1.") {
		t.Fatalf("%q", o)
	}
}

func TestReadGuard(t *testing.T) {
	read := `{"tool_name":"Read","tool_input":{"file_path":"/repo/big.go"},"cwd":"/repo","agent_id":"a1"}`
	e := setup("r")
	c, _, er := e.run("read-guard", read)
	if c != 2 || !strings.Contains(er, "read_guard:") {
		t.Fatal(c, er)
	}
	if len(e.rec.evs) != 1 || e.rec.evs[0].Kind != run.KindGuardDeny || e.rec.evs[0].Name != "read-guard" || e.rec.evs[0].Agent != "a1" || e.rec.evs[0].Note == "" {
		t.Fatalf("%+v", e.rec.evs)
	}
	e.vars["READ_GUARD_MIN_LINES"] = "700"
	if c, _, _ := e.run("read-guard", read); c != 0 {
		t.Fatal("min lines")
	}
	e.vars["READ_GUARD_MIN_LINES"] = "x"
	if c, _, _ := e.run("read-guard", read); c != 0 {
		t.Fatal("bad int")
	}
	e.vars = map[string]string{"CLAUDE_PROJECT_DIR": "/repo", "READ_GUARD": "off"}
	if c, _, _ := e.run("read-guard", read); c != 0 {
		t.Fatal("off")
	}
	e.vars = map[string]string{"READ_GUARD_MAX_VIEW_LINES": "y"}
	if c, _, _ := e.run("read-guard", `{"tool_name":"Bash","tool_input":{"command":"ls"}}`); c != 0 {
		t.Fatal("bad view")
	}
}

func TestModelGuard(t *testing.T) {
	e := setup("r")
	c, _, er := e.run("model-guard", `{"tool_name":"Agent","tool_input":{"prompt":"x"}}`)
	if c != 2 || !strings.Contains(er, "model_guard:") || e.rec.evs[0].Name != "model-guard" {
		t.Fatal(c, er, e.rec.evs)
	}
	if c, _, _ := e.run("model-guard", `{"tool_name":"Agent","tool_input":{"subagent_type":"loops:planner"}}`); c != 0 {
		t.Fatal("typed allowed")
	}
}

func TestGoPostEdit(t *testing.T) {
	in := `{"tool_name":"Edit","tool_input":{"file_path":"/repo/pkg/a.go"},"agent_id":"a1"}`
	e := setup("r")
	c, o, _ := e.run("go-post-edit", in)
	if c != 0 || o != "" || fmt.Sprint(e.cmd.calls) != "[gofmt -w /repo/pkg/a.go go vet ./pkg]" {
		t.Fatalf("%d %q %v", c, o, e.cmd.calls)
	}
	if s := e.rec.evs[0]; s.Name != "go-post-edit" || s.OK == nil || !*s.OK || s.Target != "/repo/pkg/a.go" {
		t.Fatalf("%+v", s)
	}
	e = setup("r")
	e.cmd.has = true
	e.run("go-post-edit", in)
	if e.cmd.calls[0] != "goimports -w /repo/pkg/a.go" {
		t.Fatal(e.cmd.calls)
	}
	e = setup("r")
	e.cmd.fail = map[string]string{"gofmt": "a.go:1:1: syntax error"}
	_, o, _ = e.run("go-post-edit", in)
	if !strings.Contains(o, `"decision":"block"`) || !strings.Contains(o, "gofmt failed") || !strings.Contains(o, "syntax error") || len(e.cmd.calls) != 1 || *e.rec.evs[0].OK {
		t.Fatalf("%q %v", o, e.cmd.calls)
	}
	e = setup("r")
	e.cmd.fail = map[string]string{"go vet": strings.Repeat("l\n", 50)}
	_, o, _ = e.run("go-post-edit", in)
	if !strings.Contains(o, "go vet failed") || strings.Count(o, `\n`) > 31 {
		t.Fatalf("%q", o)
	}
	for _, skip := range []string{
		`{"tool_input":{"file_path":"/repo/x.txt"}}`, `{"tool_input":{"file_path":"/repo/missing.go"}}`, `{"tool_input":{}}`,
	} {
		e = setup("r")
		if _, o, _ := e.run("go-post-edit", skip); o != "" || len(e.cmd.calls) != 0 {
			t.Fatalf("%s", skip)
		}
	}
	e = setup("r") // relative path resolves against repo
	e.run("go-post-edit", `{"tool_input":{"file_path":"pkg/a.go"}}`)
	if len(e.cmd.calls) != 2 {
		t.Fatal(e.cmd.calls)
	}
}

func TestTrace(t *testing.T) {
	cases := []struct {
		name, in string
		want     run.Event
	}{
		{"edit rel", `{"hook_event_name":"PostToolUse","tool_name":"Edit","tool_input":{"file_path":"/repo/pkg/a.go"},"agent_id":"a1","agent_type":"loops:domain-engineer","duration_ms":12.7}`,
			run.Event{Agent: "a1", Role: "domain-engineer", Name: "Edit", Target: "pkg/a.go", DurationMS: 12}},
		{"outside repo", `{"tool_name":"Read","tool_input":{"file_path":"/etc/hosts"}}`, run.Event{Name: "Read", Target: "/etc/hosts"}},
		{"notebook", `{"tool_name":"NotebookEdit","tool_input":{"notebook_path":"/repo/n.ipynb"}}`, run.Event{Name: "NotebookEdit", Target: "n.ipynb"}},
		{"bash", `{"tool_name":"Bash","tool_input":{"command":"go test ./...\nsecond"}}`, run.Event{Name: "Bash", Target: "go test ./..."}},
		{"bash long", `{"tool_name":"Bash","tool_input":{"command":"` + strings.Repeat("x", 150) + `"}}`, run.Event{Name: "Bash", Target: strings.Repeat("x", 100)}},
		{"grep", `{"tool_name":"Grep","tool_input":{"pattern":"foo"}}`, run.Event{Name: "Grep", Target: "foo"}},
		{"agent", `{"tool_name":"Agent","tool_input":{"subagent_type":"Explore","description":"look"}}`, run.Event{Name: "Agent", Target: "Explore: look"}},
		{"webfetch", `{"tool_name":"WebFetch","tool_input":{"url":"http://u"}}`, run.Event{Name: "WebFetch", Target: "http://u"}},
		{"websearch", `{"tool_name":"WebSearch","tool_input":{"query":"q"}}`, run.Event{Name: "WebSearch", Target: "q"}},
		{"mcp", `{"tool_name":"mcp__x__y","tool_input":{}}`, run.Event{Name: "mcp__x__y", Target: "mcp__x__y"}},
		{"other", `{"tool_name":"TodoWrite","tool_input":{}}`, run.Event{Name: "TodoWrite"}},
	}
	for _, c := range cases {
		e := setup("r")
		e.run("trace", c.in)
		if len(e.rec.evs) != 1 {
			t.Fatalf("%s: %d events", c.name, len(e.rec.evs))
		}
		g := e.rec.evs[0]
		if g.Kind != run.KindStep || g.OK == nil || !*g.OK {
			t.Errorf("%s: %+v", c.name, g)
		}
		g.Kind, g.OK = "", nil
		if g != c.want {
			t.Errorf("%s: got %+v want %+v", c.name, g, c.want)
		}
	}
	e := setup("r")
	e.run("trace", `{"hook_event_name":"PostToolUseFailure","tool_name":"Bash","tool_input":{"command":"x"}}`)
	e.run("trace", `{"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"x"},"tool_response":{"interrupted":true}}`)
	if *e.rec.evs[0].OK || *e.rec.evs[1].OK {
		t.Fatal("failure/interrupted should be !OK")
	}
	e = setup("")
	e.run("trace", `{"tool_name":"Read"}`)
	if len(e.rec.evs) != 0 {
		t.Fatal("no run: nothing")
	}
}

func TestTraceAgent(t *testing.T) {
	e := setup("r")
	e.run("trace-agent", `{"hook_event_name":"SubagentStart","agent_id":"a1","agent_type":"loops:planner"}`)
	e.run("trace-agent", `{"hook_event_name":"SubagentStop","agent_id":"a1","agent_type":"Explore"}`)
	e.run("trace-agent", `{"hook_event_name":"SubagentStop","agent_type":"x"}`)
	e.run("trace-agent", `{"hook_event_name":"Other","agent_id":"a"}`)
	if len(e.rec.evs) != 2 {
		t.Fatalf("%+v", e.rec.evs)
	}
	s, p := e.rec.evs[0], e.rec.evs[1]
	if s.Kind != run.KindAgentStart || s.Agent != "a1" || s.Role != "planner" || p.Kind != run.KindAgentStop || p.Role != "Explore" || !*p.OK {
		t.Fatalf("%+v %+v", s, p)
	}
}

func TestStopGate(t *testing.T) {
	e := setup("r")
	e.rec.state = run.State{GoDirty: true}
	_, o, _ := e.run("stop-gate", `{"hook_event_name":"Stop"}`)
	if !strings.Contains(o, `"decision":"block"`) || !strings.Contains(o, "loops gate") {
		t.Fatalf("%q", o)
	}
	if _, o, _ := e.run("stop-gate", `{"stop_hook_active":true}`); o != "" {
		t.Fatal("active")
	}
	e.vars["LOOPS_GATE"] = "off"
	if _, o, _ := e.run("stop-gate", `{}`); o != "" {
		t.Fatal("off")
	}
	e.vars = nil
	e.rec.state = run.State{}
	if _, o, _ := e.run("stop-gate", `{}`); o != "" {
		t.Fatal("clean")
	}
	e.rec.cur = ""
	if _, o, _ := e.run("stop-gate", `{}`); o != "" {
		t.Fatal("no run")
	}
}

func TestRaceGuard(t *testing.T) {
	bash := func(c string) string {
		return `{"tool_name":"Bash","tool_input":{"command":"` + c + `"}}`
	}
	e := setup("r")
	c, _, er := e.run("race-guard", bash("go test ./..."))
	if c != 2 || !strings.Contains(er, "race_guard:") || len(e.rec.evs) != 1 || e.rec.evs[0].Name != "race-guard" || e.rec.evs[0].Kind != run.KindGuardDeny {
		t.Fatalf("%d %q %+v", c, er, e.rec.evs)
	}
	e = setup("r")
	e.h.d.Env = func(string) string { return "off" } // no opt-out of any kind
	if c, _, _ := e.run("race-guard", bash("go test ./...")); c != 2 {
		t.Error("env opted out of a hard gate")
	}
	for _, in := range []string{bash("go test -race ./..."), `{"tool_name":"Read","tool_input":{"file_path":"x"}}`, `{"tool_name":"Bash","tool_input":"bad"}`} {
		if c, _, er := setup("r").run("race-guard", in); c != 0 || er != "" {
			t.Errorf("%s: %d %q", in, c, er)
		}
	}
}
