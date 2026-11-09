package cli

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

func TestBudgetNoRunFitsSilently(t *testing.T) {
	e := rcSetup(t, "")
	e.app.Limits = run.Limits{MaxAgents: 1}
	if code := e.do("budget", "check", "--phase", "implement", "--spawn", "5"); code != 0 || e.out.Len() != 0 || e.err.Len() != 0 {
		t.Fatalf("code %d out %q err %q", code, e.out, e.err)
	}
	if e.do("budget", "show") != 0 || !strings.Contains(e.out.String(), "no active run") {
		t.Fatalf("show without run: %q", e.out)
	}
}

func TestBudgetCheckNoLimits(t *testing.T) {
	e := rcSetup(t, "")
	e.do("run", "start", "--title", "x")
	e.do("run", "phase", "implement")
	n := len(e.events())
	if code := e.do("budget", "check", "--phase", "implement", "--spawn", "100", "--role", "r"); code != 0 || e.out.Len() != 0 {
		t.Fatalf("code %d out %q", code, e.out)
	}
	if code := e.do("budget", "check", "--verbose"); code != 0 || !strings.Contains(e.out.String(), "fits") {
		t.Fatalf("verbose, default phase: code %d out %q err %q", code, e.out, e.err)
	}
	if len(e.events()) != n {
		t.Fatal("a fitting check records nothing")
	}
	if e.do("budget", "show") != 0 || !strings.Contains(e.out.String(), "no limits set") {
		t.Fatalf("show: %q", e.out)
	}
}

func TestBudgetCheckBlocksAndRecords(t *testing.T) {
	e := rcSetup(t, "")
	e.do("run", "start", "--title", "x")
	// config limits; the run's recorded limits win field by field.
	e.app.Limits = run.Limits{MaxAgents: 10, Phases: map[run.Phase]run.PhaseLimit{run.PhaseImplement: {MaxTokens: 1_000_000, MaxAgents: 3}}}
	if e.do("run", "event", "--kind", "limits", "--note", `{"phases":{"implement":{"max_tokens":2000000}}}`) != 0 {
		t.Fatal(e.err)
	}
	e.do("run", "phase", "implement")
	e.do("run", "event", "--kind", "agent_start", "--agent", "a1", "--role", "domain-engineer")
	e.do("run", "event", "--kind", "agent_stop", "--agent", "a1", "--ok", "true")
	if code := e.do("budget", "check", "--phase", "implement", "--spawn", "2", "--role", "domain-engineer"); code != 0 {
		t.Fatalf("1 opened + 2 fits 3: code %d out %q", code, e.out)
	}
	if code := e.do("budget", "check", "--phase", "implement", "--spawn", "3", "--role", "domain-engineer"); code != 3 {
		t.Fatalf("1 opened + 3 > 3: code %d out %q err %q", code, e.out, e.err)
	}
	if !strings.Contains(e.out.String(), "implement agent budget: 1 opened + 3 new would exceed 3") || !strings.Contains(e.out.String(), "ask the user") {
		t.Fatalf("reason: %q", e.out)
	}
	b := e.events()[len(e.events())-1]
	if b.Kind != run.KindBudget || b.Phase != run.PhaseImplement || b.Role != "domain-engineer" || b.Name != run.LimitAgents || !strings.Contains(b.Note, "1 opened + 3 new") {
		t.Fatalf("budget event: %+v", b)
	}
	if e.do("budget", "show") != 0 {
		t.Fatal(e.err)
	}
	out := e.out.String()
	for _, want := range []string{"run cap: 0 running / 10", "implement", "0 / 2M", "1 / 3", "blocked spawns: 1", "implement agent budget"} {
		if !strings.Contains(out, want) {
			t.Errorf("show lacks %q:\n%s", want, out)
		}
	}
}

func TestBudgetCheckTokens(t *testing.T) {
	e := rcSetup(t, "")
	e.do("run", "start", "--title", "x")
	e.do("run", "event", "--kind", "limits", "--note", `{"phases":{"research":{"max_tokens":200000,"roles":["web-researcher"]}}}`)
	e.do("run", "phase", "research")
	e.app.Tracker.Record(t.Context(), run.Event{Kind: run.KindAgentStart, Agent: "a1", Role: "web-researcher"})
	e.app.Tracker.Record(t.Context(), run.Event{Kind: run.KindAgentStop, Agent: "a1", Tokens: 210_000})
	if code := e.do("budget", "check", "--phase", "research", "--role", "web-researcher"); code != 3 || !strings.Contains(e.out.String(), "research token budget spent: 210k of 200k") {
		t.Fatalf("code %d out %q", code, e.out)
	}
	if code := e.do("budget", "check", "--phase", "research", "--role", "code-researcher"); code != 3 || !strings.Contains(e.out.String(), "research allows only web-researcher (not code-researcher)") {
		t.Fatalf("code %d out %q", code, e.out)
	}
	if e.do("budget", "show") != 0 || !strings.Contains(e.out.String(), "210k / 200k") || !strings.Contains(e.out.String(), "web-researcher") {
		t.Fatalf("show: %q", e.out)
	}
}

func TestBudgetUsage(t *testing.T) {
	e := rcSetup(t, "")
	for _, args := range [][]string{
		{"budget"}, {"budget", "nope"}, {"budget", "check", "--phase", "bogus"}, {"budget", "check", "--spawn", "0"},
		{"budget", "check", "extra"}, {"budget", "check", "--bad"}, {"budget", "show", "x"},
		{"budget", "parse", "--budget", "bogus=1k"}, {"budget", "parse", "--agents", "implement=x"},
		{"budget", "parse", "--roles", "a,b"}, {"budget", "parse", "--max-agents", "-2"}, {"budget", "parse", "x"},
	} {
		if code := e.do(args...); code != 2 {
			t.Errorf("%v: code %d err %q", args, code, e.err)
		}
	}
}

func TestBudgetParse(t *testing.T) {
	e := rcSetup(t, "")
	if code := e.do("budget", "parse", "--budget", "research=200k,implement=2M", "--agents", "implement=12", "--roles", "implement=domain-engineer,api-engineer", "--max-agents", "8"); code != 0 {
		t.Fatal(e.err)
	}
	var got run.Limits
	if err := json.Unmarshal(e.out.Bytes(), &got); err != nil {
		t.Fatalf("%v: %q", err, e.out)
	}
	want := run.Limits{MaxAgents: 8, Phases: map[run.Phase]run.PhaseLimit{
		run.PhaseResearch:  {MaxTokens: 200_000},
		run.PhaseImplement: {MaxTokens: 2_000_000, MaxAgents: 12, Roles: []string{"domain-engineer", "api-engineer"}},
	}}
	if !reflect.DeepEqual(got, want) || strings.Count(e.out.String(), "\n") != 1 {
		t.Fatalf("got %q", e.out)
	}
	// the parsed JSON round-trips through `run event --kind limits`.
	e.do("run", "start", "--title", "x")
	e.do("budget", "parse", "--budget", "research=200k,implement=2M", "--agents", "implement=12", "--roles", "implement=domain-engineer,api-engineer", "--max-agents", "8")
	note := strings.TrimSpace(e.out.String())
	if e.do("run", "event", "--kind", "limits", "--note", note) != 0 {
		t.Fatal(e.err)
	}
	st, _ := e.app.Tracker.State(t.Context(), "")
	if !reflect.DeepEqual(st.Limits, want) {
		t.Fatalf("state limits %+v", st.Limits)
	}
	if e.do("budget", "parse") != 0 || strings.TrimSpace(e.out.String()) != "{}" {
		t.Fatalf("empty: %q", e.out)
	}
}

func TestRunEventLimitsValidation(t *testing.T) {
	e := rcSetup(t, "")
	e.do("run", "start", "--title", "x")
	n := len(e.events())
	for _, note := range []string{
		"", "{", "[]", `{"max_agents":"3"}`, `{"phases":{"nope":{"max_tokens":1}}}`, `{"max_agent":3}`,
		`{"max_agents":-1}`, `{"phases":{"qa":{"max_tokens":-5}}}`, `{"phases":{"qa":{"max_agents":-1}}}`, `{} {}`,
	} {
		if code := e.do("run", "event", "--kind", "limits", "--note", note); code != 2 {
			t.Errorf("%q: code %d", note, code)
		}
	}
	if len(e.events()) != n {
		t.Fatal("invalid limits must not be recorded")
	}
	if e.do("run", "event", "--kind", "limits", "--note", `{}`) != 0 || e.do("run", "event", "--kind", "limits", "--note", `{"max_agents":4}`) != 0 {
		t.Fatal(e.err)
	}
}
