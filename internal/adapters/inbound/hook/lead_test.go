package hook

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arxdsilva/ai-loops/internal/application/guard"
	"github.com/arxdsilva/ai-loops/internal/domain/run"
	"github.com/arxdsilva/ai-loops/internal/domain/workspace"
)

func agentPayload(prompt, model, response string) string {
	in := map[string]any{"subagent_type": "loops:domain-engineer", "description": "d", "prompt": prompt}
	if model != "" {
		in["model"] = model
	}
	return `{"hook_event_name":"PostToolUse","tool_name":"Agent","tool_input":` + mustJSON(in) + `,"tool_response":` + response + `}`
}

const spawnHeader = "Run dir: /r\nLoops: loops\nRole: domain-engineer\nAgent id: W1-domain\nWave: 1\nCharter: /r/charter.md\n"

func labels(e *env) []run.Event {
	var out []run.Event
	for _, ev := range e.rec.evs {
		if ev.Kind == run.KindLabel {
			out = append(out, ev)
		}
	}
	return out
}

func TestAutoLabel(t *testing.T) {
	async := `{"isAsync":true,"status":"async_launched","agentId":"a527d7b513abdcd48","description":"d","resolvedModel":"claude-haiku-5-5","prompt":"p"}`
	for _, c := range []struct {
		name, payload string
		want          *run.Event // nil: no label
	}{
		{"async launch", agentPayload(spawnHeader, "", async),
			&run.Event{Kind: run.KindLabel, Agent: "a527d7b513abdcd48", Name: "W1-domain", Role: "domain-engineer", Wave: 1, Model: "haiku"}},
		{"model from the call", agentPayload(spawnHeader, "opus", `{"agentId":"abc"}`),
			&run.Event{Kind: run.KindLabel, Agent: "abc", Name: "W1-domain", Role: "domain-engineer", Wave: 1, Model: "opus"}},
		{"resolved id wins over a tier name", agentPayload(spawnHeader, "sonnet", `{"agentId":"abc","resolvedModel":"claude-opus-5-5"}`),
			&run.Event{Kind: run.KindLabel, Agent: "abc", Name: "W1-domain", Role: "domain-engineer", Wave: 1, Model: "opus"}},
		{"text form", agentPayload(spawnHeader, "", `{"content":[{"type":"text","text":"Async agent launched.\nagentId: a1b2-c3 (internal)"}]}`),
			&run.Event{Kind: run.KindLabel, Agent: "a1b2-c3", Name: "W1-domain", Role: "domain-engineer", Wave: 1}},
		{"string response", agentPayload(spawnHeader, "", `"agentId: zz9"`),
			&run.Event{Kind: run.KindLabel, Agent: "zz9", Name: "W1-domain", Role: "domain-engineer", Wave: 1}},
		{"role falls back to subagent_type", agentPayload("Agent id: planner\n", "", async),
			&run.Event{Kind: run.KindLabel, Agent: "a527d7b513abdcd48", Name: "planner", Role: "domain-engineer", Model: "haiku"}},
		{"header line indented, CRLF", agentPayload("  Role: qa-verifier\r\n  Agent id: qa-W1\r\n  Wave: 2\r\n", "", async),
			&run.Event{Kind: run.KindLabel, Agent: "a527d7b513abdcd48", Name: "qa-W1", Role: "qa-verifier", Wave: 2, Model: "haiku"}},
		{"no Agent id line", agentPayload("Role: domain-engineer\nWave: 1\n", "", async), nil},
		{"empty Agent id", agentPayload("Agent id:   \n", "", async), nil},
		{"no agent id in the response", agentPayload(spawnHeader, "", `{"status":"completed"}`), nil},
		{"id mid-line only", agentPayload("see Agent id: x\n", "", async), nil},
		{"failure event", strings.Replace(agentPayload(spawnHeader, "", async), "PostToolUse", "PostToolUseFailure", 1), nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := setup("r")
			if code, o, er := e.run("trace", c.payload); code != 0 || o != "" || er != "" {
				t.Fatalf("%d %q %q", code, o, er)
			}
			var steps int
			for _, ev := range e.rec.evs {
				if ev.Kind == run.KindStep {
					steps++
				}
			}
			if steps != 1 {
				t.Errorf("the normal step event must stay: %+v", e.rec.evs)
			}
			got := labels(e)
			if c.want == nil {
				if len(got) != 0 {
					t.Fatalf("unexpected label %+v", got)
				}
				return
			}
			if len(got) != 1 || got[0] != *c.want {
				t.Fatalf("label = %+v, want %+v", got, *c.want)
			}
		})
	}
	// No active run: nothing is recorded at all.
	e := setup("")
	e.run("trace", agentPayload(spawnHeader, "", async))
	if len(e.rec.evs) != 0 {
		t.Fatalf("recorded without a run: %+v", e.rec.evs)
	}
	// A subagent's own Agent call is not the lead's spawn.
	e = setup("r")
	e.run("trace", strings.Replace(agentPayload(spawnHeader, "", async), `"tool_name"`, `"agent_id":"sub1","tool_name"`, 1))
	if len(labels(e)) != 0 {
		t.Fatalf("labelled a subagent's spawn: %+v", e.rec.evs)
	}
	// "Task" is the same tool.
	e = setup("r")
	e.run("trace", strings.Replace(agentPayload(spawnHeader, "", async), `"tool_name":"Agent"`, `"tool_name":"Task"`, 1))
	if len(labels(e)) != 1 {
		t.Fatalf("Task not labelled: %+v", e.rec.evs)
	}
}

// ---- transcripts ----

var t0 = time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)

func tline(typ string, sidechain bool, at time.Time, id string, in, out, cc, cr int) string {
	return fmt.Sprintf(`{"type":%q,"isSidechain":%v,"timestamp":%q,"message":{"id":%q,"role":%q,"usage":{"input_tokens":%d,"output_tokens":%d,"cache_creation_input_tokens":%d,"cache_read_input_tokens":%d}}}`,
		typ, sidechain, at.Format(time.RFC3339Nano), id, typ, in, out, cc, cr)
}

func writeTranscript(t *testing.T, lines ...string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "t.jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLeadTokens(t *testing.T) {
	path := writeTranscript(t,
		tline("assistant", false, t0.Add(-time.Hour), "old", 5000, 5000, 0, 0), // before the run
		tline("assistant", false, t0, "m1", 100, 50, 10, 40),                   // 200, at the start
		tline("assistant", false, t0.Add(time.Second), "m1", 100, 50, 10, 40),  // same message, second block
		tline("assistant", false, t0.Add(2*time.Second), "m1", 100, 60, 10, 40),
		tline("assistant", true, t0.Add(3*time.Second), "side", 9000, 9000, 0, 0), // a subagent's turn
		tline("user", false, t0.Add(4*time.Second), "u1", 777, 0, 0, 0),
		tline("assistant", false, t0.Add(5*time.Second), "", 1, 2, 3, 4), // no id: counted as is
		"not json with \"usage\" in it",
		`{"type":"assistant","isSidechain":false,"message":{"id":"nots","usage":{"input_tokens":99}}}`, // no timestamp
		`{"type":"summary","leafUuid":"x"}`,
	)
	if got, want := leadTokens(path, t0, 1<<20), int64(210+10); got != want {
		t.Errorf("since the run start: %d, want %d", got, want)
	}
	if got, want := leadTokens(path, time.Time{}, 1<<20), int64(10000+210+10+99); got != want {
		t.Errorf("no start time: %d, want %d", got, want)
	}
	for _, p := range []string{"", filepath.Join(t.TempDir(), "missing")} {
		if got := leadTokens(p, t0, 1<<20); got != 0 {
			t.Errorf("%q: %d, want 0 (fail open)", p, got)
		}
	}
	// Only the tail is read when the file exceeds the cap; a partial first line is dropped.
	one := tline("assistant", false, t0.Add(time.Second), "a", 10, 0, 0, 0)
	two := tline("assistant", false, t0.Add(2*time.Second), "b", 20, 0, 0, 0)
	tail := writeTranscript(t, one, two)
	if got := leadTokens(tail, t0, int64(len(two))+5); got != 20 {
		t.Errorf("tail read: %d, want 20", got)
	}
}

func stopPayload(path string) string {
	return `{"hook_event_name":"Stop","transcript_path":` + mustJSON(path) + `}`
}

func leadNotes(e *env) []run.Event {
	var out []run.Event
	for _, ev := range e.rec.evs {
		if ev.Kind == run.KindNote && ev.Name == run.NoteLeadTokens {
			out = append(out, ev)
		}
	}
	return out
}

func TestStopGateRecordsLeadTokens(t *testing.T) {
	path := writeTranscript(t, tline("assistant", false, t0.Add(time.Second), "m1", 100, 50, 10, 40))
	e := setup("r")
	e.rec.state = run.State{ID: "r", Started: t0}
	if c, o, er := e.run("stop-gate", stopPayload(path)); c != 0 || o != "" || er != "" {
		t.Fatalf("%d %q %q", c, o, er)
	}
	if n := leadNotes(e); len(n) != 1 || n[0].Tokens != 200 {
		t.Fatalf("notes %+v", e.rec.evs)
	}
	// Not grown since the last recorded value: nothing new.
	e = setup("r")
	e.rec.state = run.State{ID: "r", Started: t0, LeadTokens: 200}
	e.run("stop-gate", stopPayload(path))
	if n := leadNotes(e); len(n) != 0 {
		t.Fatalf("recorded without growth: %+v", n)
	}
	// Grown: recorded again.
	e = setup("r")
	e.rec.state = run.State{ID: "r", Started: t0, LeadTokens: 150}
	e.run("stop-gate", stopPayload(path))
	if n := leadNotes(e); len(n) != 1 || n[0].Tokens != 200 {
		t.Fatalf("growth not recorded: %+v", e.rec.evs)
	}
	// The recorded total folds into State.LeadTokens.
	if st := run.Fold([]run.Event{{Kind: run.KindRun, Run: "r", Time: t0}, {Kind: run.KindNote, Name: run.NoteLeadTokens, Tokens: 200}}); st.LeadTokens != 200 {
		t.Fatalf("fold: %d", st.LeadTokens)
	}
	// The stop decision is the same with and without the transcript.
	dirty := run.State{ID: "r", Started: t0, SourceDirty: true}
	e = setup("r")
	e.rec.state = dirty
	_, with, _ := e.run("stop-gate", stopPayload(path))
	e = setup("r")
	e.rec.state = dirty
	_, without, _ := e.run("stop-gate", `{"hook_event_name":"Stop"}`)
	if with == "" || with != without {
		t.Fatalf("stop output changed:\n%q\n%q", with, without)
	}
	// Ended run, no run, unreadable path, other platform: silent.
	e = setup("r")
	e.rec.state = run.State{ID: "r", Started: t0, Ended: true}
	e.run("stop-gate", stopPayload(path))
	e2 := setup("")
	e2.run("stop-gate", stopPayload(path))
	e3 := setup("r")
	e3.rec.state = run.State{ID: "r", Started: t0}
	e3.run("stop-gate", stopPayload(filepath.Join(t.TempDir(), "missing")))
	e4 := setup("r")
	e4.rec.state = run.State{ID: "r", Started: t0}
	e4.runP("codex", "stop-gate", stopPayload(path))
	for i, x := range []*env{e, e2, e3, e4} {
		if n := leadNotes(x); len(n) != 0 {
			t.Errorf("case %d recorded %+v", i, n)
		}
	}
}

func TestContextHint(t *testing.T) {
	heavy := writeTranscript(t,
		tline("assistant", false, t0, "m1", 10, 5, 0, 1000),
		tline("assistant", false, t0, "m2", 1000, 300, 2000, 160000), // 163000 of context
		tline("assistant", true, t0, "side", 1, 1, 1, 900000),        // subagent: ignored
		tline("user", false, t0, "u", 1, 1, 1, 900000),
	)
	light := writeTranscript(t, tline("assistant", false, t0, "m", 1000, 5, 0, 20000))
	edge := writeTranscript(t, tline("assistant", false, t0, "m", 0, 5, 0, 150000)) // exactly the threshold: no hint
	bashPayload := func(cmd, path, event string) string {
		return `{"hook_event_name":"` + event + `","tool_name":"Bash","transcript_path":` + mustJSON(path) +
			`,"tool_input":` + mustJSON(map[string]any{"command": cmd}) + `,"tool_response":{"stdout":"started r1"}}`
	}
	const want = `loops: this session already holds ~163k tokens of context, and the lead re-reads it on every turn of this run. ` +
		`Tell the user once, in one line: run /clear and say \"continue the loops run\" (the run re-anchors from its brief), or start the next run in a fresh session.`
	for _, cmd := range []string{
		`loops run start --title "x"`,
		`/Users/me/.claude/plugins/loops/bin/loops run start --ticket 4`,
		`"${CLAUDE_PLUGIN_ROOT}/bin/loops" run start`,
		`L=/x/bin/loops; $L run start --ticket 4`,
		`cd /repo && loops run start`,
	} {
		e := setup("r")
		c, o, er := e.run("trace", bashPayload(cmd, heavy, "PostToolUse"))
		if c != 0 || er != "" || !strings.Contains(o, `"hookEventName":"PostToolUse"`) || !strings.Contains(o, `"additionalContext":"`+want+`"`) {
			t.Errorf("%q: %d %q %q", cmd, c, o, er)
		}
	}
	for _, c := range []struct{ name, cmd, path, event string }{
		{"other loops command", `loops run status`, heavy, "PostToolUse"},
		{"mentioned in text", `echo run start`, heavy, "PostToolUse"},
		{"run starting elsewhere", `git run start`, heavy, "PostToolUse"},
		{"light session", `loops run start`, light, "PostToolUse"},
		{"threshold itself", `loops run start`, edge, "PostToolUse"},
		{"failed start", `loops run start`, heavy, "PostToolUseFailure"},
		{"no transcript", `loops run start`, "", "PostToolUse"},
		{"missing transcript", `loops run start`, filepath.Join(t.TempDir(), "x"), "PostToolUse"},
	} {
		e := setup("r")
		if code, o, er := e.run("trace", bashPayload(c.cmd, c.path, c.event)); code != 0 || o != "" || er != "" {
			t.Errorf("%s: %d %q %q", c.name, code, o, er)
		}
	}
	// Other platforms get no hint.
	if _, o, _ := setup("r").runP("codex", "trace", bashPayload(`loops run start`, heavy, "PostToolUse")); o != "" {
		t.Errorf("codex: %q", o)
	}
	// The step event is recorded either way.
	e := setup("r")
	e.run("trace", bashPayload(`loops run start`, heavy, "PostToolUse"))
	if len(e.rec.evs) != 1 || e.rec.evs[0].Kind != run.KindStep {
		t.Errorf("%+v", e.rec.evs)
	}
}

// A session restarted with /clear re-anchors from the run brief (WORKFLOW §2.7).
func TestSessionStartInjectsBriefForEverySource(t *testing.T) {
	for _, src := range []string{"startup", "resume", "clear", "compact"} {
		e := setup("r1")
		e.vars["LOOPS_PLUGIN_ROOT"] = "/plug"
		e.rec.state = run.State{ID: "r1", Phase: run.PhasePlan}
		_, o, _ := e.run("session-start", `{"hook_event_name":"SessionStart","source":"`+src+`"}`)
		for _, w := range []string{"Active run r1", "lead (master agent) of loops run", "do not restart finished phases"} {
			if !strings.Contains(o, w) {
				t.Errorf("source %s lacks %q: %q", src, w, o)
			}
		}
	}
}

// ---- model guard ----

func TestModelGuardTier(t *testing.T) {
	tiers := map[string]string{"domain-engineer": "sonnet", "planner": "opus"}
	agent := func(in map[string]any) string {
		return `{"hook_event_name":"PreToolUse","tool_name":"Agent","tool_input":` + mustJSON(in) + `}`
	}
	for _, c := range []struct {
		name string
		in   map[string]any
		deny bool
	}{
		{"above tier", map[string]any{"subagent_type": "loops:domain-engineer", "model": "opus", "prompt": "Role: x"}, true},
		{"above tier with reason", map[string]any{"subagent_type": "loops:domain-engineer", "model": "opus", "prompt": "Role: x\nModel reason: QA failed twice\n"}, false},
		{"at tier", map[string]any{"subagent_type": "loops:domain-engineer", "model": "sonnet"}, false},
		{"below tier", map[string]any{"subagent_type": "loops:planner", "model": "haiku"}, false},
		{"unknown role", map[string]any{"subagent_type": "loops:nobody", "model": "opus"}, false},
		{"general-purpose, no model", map[string]any{"subagent_type": "general-purpose"}, true},
	} {
		e := setup("r")
		e.h.d.AgentTier = func(role string) string { return tiers[role] }
		code, _, er := e.run("model-guard", agent(c.in))
		if c.deny != (code == 2) {
			t.Errorf("%s: %d %q", c.name, code, er)
		}
		if c.deny && !strings.HasPrefix(er, "model_guard:") {
			t.Errorf("%s: %q", c.name, er)
		}
	}
	// Without a tier source nothing above-tier is checked.
	e := setup("r")
	if c, _, _ := e.run("model-guard", agent(map[string]any{"subagent_type": "loops:domain-engineer", "model": "opus"})); c != 0 {
		t.Errorf("nil AgentTier denied: %d", c)
	}
}

// ---- go-env shapes ----

// Commands agents really send are scoped on Claude, never denied.
func TestGoEnvShapesClaude(t *testing.T) {
	for _, cmd := range []string{
		"cd x && go test -race ./...",
		"GOFLAGS=-p=2 go test -race -count=1 ./internal/...",
		"go test -race ./... 2>&1 | tail -20",
		"(cd a; go vet ./...)",
		"for p in a b; do go test -race ./$p/...; done",
		"env FOO=1 go test -race ./...",
		"L=/x/bin/loops; cd /repo && go build ./...",
		"cd x && go test ./...", // -race is added
		"go test -count=1 ./... 2>&1 | tail -20",
	} {
		t.Run(cmd, func(t *testing.T) {
			e, rd := goEnvSetup(t, "r1")
			if c, o, er := e.runP("claude", "test-guard", bashPayload("claude", cmd)); c != 0 || o != "" || er != "" {
				t.Fatalf("test-guard: %d %q %q", c, o, er)
			}
			code, out, errOut := e.runP("claude", "go-env", bashPayload("claude", cmd))
			if code != 0 || errOut != "" {
				t.Fatalf("denied: %d %q", code, errOut)
			}
			got, _, _ := rewritten(t, "claude", out)
			if guard.InspectGo(got, workspace.GoCache(rd)).Unscoped {
				t.Fatalf("still unscoped: %s", got)
			}
			if r := guard.CheckRace(got); r != "" {
				t.Fatalf("race policy broken: %s", got)
			}
		})
	}
}
