package hook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

var allPlatforms = []string{"claude", "codex", "cursor", "gemini", "copilot", "windsurf"}

func fixture(t *testing.T, platform, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", platform, name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func (e *env) runP(platform, name, in string) (int, string, string) {
	var o, er strings.Builder
	code := e.h.Handle(context.Background(), name, platform, strings.NewReader(in), &o, &er)
	return code, o.String(), er.String()
}

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) + "\n" }

func sessionText(entry string) string {
	return "loops plugin active. Plugin root: /plug. Work tickets with " + entry + " (WORKFLOW: /plug/specs/WORKFLOW.md)." +
		" loops binary: /plug/bin/loops — if `loops` is not on PATH, call it by that absolute path (or `go install github.com/arxdsilva/ai-loops/cmd/loops@latest`)."
}

func TestPlatformSessionStart(t *testing.T) {
	entry := map[string]string{"claude": "/loops:work", "copilot": "/loops:work", "gemini": "/loops:work",
		"codex": "$work", "cursor": "/work", "opencode": "/loops/work", "windsurf": "the work skill"}
	for p, en := range entry {
		text := sessionText(en)
		want := mustJSON(map[string]any{"hookSpecificOutput": map[string]string{"hookEventName": "SessionStart", "additionalContext": text}})
		switch p {
		case "cursor":
			want = mustJSON(map[string]string{"additional_context": text})
		case "copilot":
			want = mustJSON(map[string]string{"additionalContext": text})
		}
		fx := "claude"
		if p != "opencode" {
			fx = p
		}
		e := setup("")
		e.vars["LOOPS_PLUGIN_ROOT"] = "/plug"
		c, o, _ := e.runP(p, "session-start", fixture(t, fx, "session-start"))
		if c != 0 || o != want {
			t.Errorf("%s: %d %q", p, c, o)
		}
	}
}

func TestPlatformReadGuardDeny(t *testing.T) {
	for _, p := range allPlatforms {
		e := setup("r")
		c, o, er := e.runP(p, "read-guard", fixture(t, p, "read-guard"))
		if c != 2 || o != "" || !strings.Contains(er, "read_guard:") || len(e.rec.evs) != 1 || e.rec.evs[0].Kind != run.KindGuardDeny {
			t.Errorf("%s: %d %q %q %+v", p, c, o, er, e.rec.evs)
		}
	}
}

func TestPlatformModelGuard(t *testing.T) {
	for _, p := range []string{"claude", "copilot"} {
		e := setup("r")
		c, _, er := e.runP(p, "model-guard", fixture(t, p, "model-guard"))
		if c != 2 || !strings.Contains(er, "model_guard:") {
			t.Errorf("%s: %d %q", p, c, er)
		}
	}
	for _, p := range []string{"cursor", "gemini", "opencode"} { // no documented model field: no-op
		e := setup("r")
		if c, _, er := e.runP(p, "model-guard", fixture(t, "cursor", "model-guard")); c != 0 || er != "" {
			t.Errorf("%s: %d %q", p, c, er)
		}
	}
	e := setup("r")
	if c, _, er := e.runP("codex", "model-guard", `{"tool_name":"Agent","tool_input":{"prompt":"x"}}`); c != 2 || er == "" {
		t.Errorf("codex: %d %q", c, er)
	}
	e = setup("r") // non-Agent tools are never judged
	if c, _, _ := e.runP("gemini", "model-guard", fixture(t, "gemini", "read-guard")); c != 0 {
		t.Error("gemini read judged by model guard")
	}
}

func TestPlatformGoPostEdit(t *testing.T) {
	reason := "go-post-edit: gofmt failed for /repo/pkg/a.go:\nsyntax error"
	failOut := map[string]string{
		"claude":   mustJSON(map[string]string{"decision": "block", "reason": reason}),
		"codex":    mustJSON(map[string]string{"decision": "block", "reason": reason}),
		"windsurf": mustJSON(map[string]string{"decision": "block", "reason": reason}),
		"cursor":   mustJSON(map[string]string{"additional_context": reason}),
		"copilot":  mustJSON(map[string]string{"additionalContext": reason}),
		"gemini":   mustJSON(map[string]any{"hookSpecificOutput": map[string]string{"hookEventName": "AfterTool", "additionalContext": reason}}),
	}
	for _, p := range allPlatforms {
		e := setup("r")
		c, o, _ := e.runP(p, "go-post-edit", fixture(t, p, "edit"))
		if c != 0 || o != "" || fmt.Sprint(e.cmd.calls) != "[gofmt -w /repo/pkg/a.go go vet ./pkg]" {
			t.Errorf("%s ok: %d %q %v", p, c, o, e.cmd.calls)
		}
		e = setup("r")
		e.cmd.fail = map[string]string{"gofmt": "syntax error"}
		_, o, _ = e.runP(p, "go-post-edit", fixture(t, p, "edit"))
		if o != failOut[p] {
			t.Errorf("%s fail: %q want %q", p, o, failOut[p])
		}
	}
}

func TestPlatformTrace(t *testing.T) {
	for _, p := range allPlatforms {
		e := setup("r")
		e.runP(p, "trace", fixture(t, p, "edit"))
		if len(e.rec.evs) != 1 {
			t.Fatalf("%s: %+v", p, e.rec.evs)
		}
		g := e.rec.evs[0]
		wantName := "Edit"
		if p == "cursor" {
			wantName = "Write"
		}
		if g.Name != wantName || g.Target != "pkg/a.go" || g.OK == nil || !*g.OK {
			t.Errorf("%s: %+v", p, g)
		}
	}
	if e := setup("r"); true { // role/agent/duration folding
		e.runP("cursor", "trace", fixture(t, "cursor", "edit"))
		if e.rec.evs[0].DurationMS != 42 {
			t.Errorf("cursor duration: %+v", e.rec.evs[0])
		}
		e.runP("codex", "trace", fixture(t, "codex", "edit"))
		if g := e.rec.evs[1]; g.Agent != "a1" || g.Role != "worker" {
			t.Errorf("codex agent: %+v", g)
		}
	}
	for _, p := range []string{"claude", "cursor", "gemini", "copilot"} {
		e := setup("r")
		e.runP(p, "trace", fixture(t, p, "edit-failure"))
		if len(e.rec.evs) != 1 || e.rec.evs[0].OK == nil || *e.rec.evs[0].OK || e.rec.evs[0].Target != "pkg/a.go" {
			t.Errorf("%s failure: %+v", p, e.rec.evs)
		}
	}
}

func TestPlatformTraceAgent(t *testing.T) {
	for _, p := range []string{"claude", "codex", "cursor"} {
		e := setup("r")
		e.runP(p, "trace-agent", fixture(t, p, "agent-start"))
		if len(e.rec.evs) != 1 || e.rec.evs[0].Kind != run.KindAgentStart || e.rec.evs[0].Agent != "a1" {
			t.Errorf("%s: %+v", p, e.rec.evs)
		}
	}
	e := setup("r") // Cursor subagentStop carries no subagent id: nothing to record
	e.runP("cursor", "trace-agent", fixture(t, "cursor", "agent-stop"))
	if len(e.rec.evs) != 0 {
		t.Errorf("%+v", e.rec.evs)
	}
}

func TestPlatformStopGate(t *testing.T) {
	r := run.StopReason(run.State{GoDirty: true})
	block := mustJSON(map[string]string{"decision": "block", "reason": r})
	want := map[string]string{
		"claude": block, "codex": block, "copilot": block, "windsurf": block,
		"cursor": mustJSON(map[string]string{"followup_message": r}),
		"gemini": mustJSON(map[string]string{"decision": "deny", "reason": r}),
	}
	for _, p := range allPlatforms {
		e := setup("r")
		e.rec.state = run.State{GoDirty: true}
		if c, o, _ := e.runP(p, "stop-gate", fixture(t, p, "stop")); c != 0 || o != want[p] {
			t.Errorf("%s: %d %q", p, c, o)
		}
		if _, o, _ := e.runP(p, "stop-gate", fixture(t, p, "stop-active")); o != "" {
			t.Errorf("%s active: %q", p, o)
		}
	}
}

func TestDetectPlatform(t *testing.T) {
	cases := []struct {
		flag string
		env  map[string]string
		want string
	}{
		{"cursor", nil, "cursor"}, {"OpenCode", nil, "opencode"}, {"devin", nil, "windsurf"},
		{"", map[string]string{"CURSOR_PROJECT_DIR": "/x", "CLAUDE_PROJECT_DIR": "/x"}, "cursor"},
		{"", map[string]string{"CURSOR_PLUGIN_ROOT": "/x"}, "cursor"},
		{"", map[string]string{"GEMINI_CWD": "/x"}, "gemini"},
		{"", map[string]string{"DEVIN_PROJECT_DIR": "/x"}, "windsurf"},
		{"", map[string]string{"PLUGIN_ROOT": "/x"}, "codex"},
		{"", map[string]string{"PLUGIN_ROOT": "/x", "CLAUDE_PROJECT_DIR": "/y"}, "claude"},
		{"", nil, "claude"}, {"bogus", nil, "claude"},
	}
	for _, c := range cases {
		if got := detect(c.flag, func(k string) string { return c.env[k] }); got != c.want {
			t.Errorf("%q %v: %s want %s", c.flag, c.env, got, c.want)
		}
	}
}

func TestProjectDirPerPlatform(t *testing.T) {
	for p, k := range map[string]string{"cursor": "CURSOR_PROJECT_DIR", "gemini": "GEMINI_PROJECT_DIR", "windsurf": "DEVIN_PROJECT_DIR"} {
		e := setup("r")
		e.vars = map[string]string{k: "/repo"}
		e.cmd.fail = nil
		e.runP(p, "go-post-edit", `{"tool_name":"Edit","tool_input":{"file_path":"pkg/a.go"}}`)
		if fmt.Sprint(e.cmd.calls) != "[gofmt -w /repo/pkg/a.go go vet ./pkg]" {
			t.Errorf("%s: %v", p, e.cmd.calls)
		}
	}
	e := setup("r") // cursor workspace_roots fallback
	e.vars = nil
	e.runP("cursor", "go-post-edit", `{"tool_name":"Write","tool_input":{"path":"pkg/a.go"},"workspace_roots":["/repo"]}`)
	if len(e.cmd.calls) != 2 {
		t.Errorf("%v", e.cmd.calls)
	}
}

func TestOpenCodeIsClaudeShaped(t *testing.T) {
	e := setup("r")
	c, _, er := e.runP("opencode", "read-guard", fixture(t, "claude", "read-guard"))
	if c != 2 || !strings.Contains(er, "read_guard:") {
		t.Fatal(c, er)
	}
}

func TestApplyPatchTarget(t *testing.T) {
	if got := patchTarget("*** Update File: a.txt\n*** Add File: pkg/b.go\n"); got != "pkg/b.go" {
		t.Error(got)
	}
	if got := patchTarget("*** Delete File: a.txt\n"); got != "a.txt" {
		t.Error(got)
	}
	if patchTarget("nothing") != "" {
		t.Error("none")
	}
}

func TestStopGateReleasesWithoutProgress(t *testing.T) {
	for _, p := range allPlatforms {
		e := setup("r")
		e.rec.state = run.State{GoDirty: true}
		in := fixture(t, p, "stop")
		for i := 0; i < 3; i++ {
			if _, o, _ := e.runP(p, "stop-gate", in); o == "" {
				t.Fatalf("%s: block %d missing", p, i)
			}
		}
		if _, o, _ := e.runP(p, "stop-gate", in); o != "" {
			t.Errorf("%s: 4th stop should release, got %q", p, o)
		}
		last := e.rec.evs[len(e.rec.evs)-1]
		if last.Kind != run.KindNote || last.Name != "stop-gate" || last.Note != "released after 3 blocks without progress" {
			t.Errorf("%s: %+v", p, last)
		}
		// still released on a further stop; a tool call restores blocking
		if _, o, _ := e.runP(p, "stop-gate", in); o != "" {
			t.Errorf("%s: stays released", p)
		}
		e.runP(p, "trace", fixture(t, p, "edit"))
		if _, o, _ := e.runP(p, "stop-gate", in); o == "" {
			t.Errorf("%s: progress should re-arm the gate", p)
		}
	}
	e := setup("r") // blocks with steps between never release
	e.rec.state = run.State{GoDirty: true}
	for i := 0; i < 5; i++ {
		if _, o, _ := e.runP("claude", "stop-gate", fixture(t, "claude", "stop")); o == "" {
			t.Fatalf("block %d", i)
		}
		e.runP("claude", "trace", fixture(t, "claude", "edit"))
	}
}

func TestSubagentStopWithoutIDClosesNewestSameRole(t *testing.T) {
	e := setup("r")
	t0 := time.Unix(100, 0)
	e.rec.state = run.State{Lanes: []run.Lane{
		{Agent: "old", Role: "explore", Running: true, Started: t0},
		{Agent: "new", Role: "explore", Running: true, Started: t0.Add(time.Second)},
		{Agent: "other", Role: "planner", Running: true, Started: t0.Add(2 * time.Second)},
		{Agent: "done", Role: "explore", Running: false, Started: t0.Add(3 * time.Second)},
	}}
	e.runP("cursor", "trace-agent", fixture(t, "cursor", "agent-stop"))
	if len(e.rec.evs) != 1 || e.rec.evs[0].Kind != run.KindAgentStop || e.rec.evs[0].Agent != "new" || e.rec.evs[0].Role != "explore" {
		t.Fatalf("%+v", e.rec.evs)
	}
	e = setup("r") // no matching running lane: nothing recorded
	e.rec.state = run.State{Lanes: []run.Lane{{Agent: "x", Role: "planner", Running: true}}}
	e.runP("cursor", "trace-agent", fixture(t, "cursor", "agent-stop"))
	if len(e.rec.evs) != 0 {
		t.Fatalf("%+v", e.rec.evs)
	}
}

func TestMultiFilePatchRunsEveryGoFile(t *testing.T) {
	patch := "*** Begin Patch\n*** Update File: pkg/a.go\n*** Add File: README.md\n*** Update File: pkg/b.go\n*** Update File: other/c.go\n*** End Patch"
	in, _ := json.Marshal(map[string]any{"hook_event_name": "PostToolUse", "tool_name": "apply_patch", "cwd": "/repo", "tool_input": map[string]string{"command": patch}})
	e := setup("r")
	e.h.d.Files = fakeFiles{lines: map[string]int{"/repo/pkg/a.go": 1, "/repo/pkg/b.go": 1, "/repo/other/c.go": 1}}
	e.runP("codex", "go-post-edit", string(in))
	want := "[gofmt -w /repo/pkg/a.go gofmt -w /repo/pkg/b.go gofmt -w /repo/other/c.go go vet ./pkg go vet ./other]"
	if fmt.Sprint(e.cmd.calls) != want {
		t.Fatalf("%v", e.cmd.calls)
	}
	if len(e.rec.evs) != 3 {
		t.Fatalf("%+v", e.rec.evs)
	}
	e = setup("r") // a vet failure marks every file in that package
	e.h.d.Files = fakeFiles{lines: map[string]int{"/repo/pkg/a.go": 1, "/repo/pkg/b.go": 1, "/repo/other/c.go": 1}}
	e.cmd.fail = map[string]string{"go vet ./pkg": "bad"}
	_, o, _ := e.runP("copilot", "go-post-edit", strings.Replace(string(in), `"cwd"`, `"x"`, 1))
	if !strings.Contains(o, "go vet failed for /repo/pkg/a.go") || *e.rec.evs[0].OK || *e.rec.evs[1].OK || !*e.rec.evs[2].OK {
		t.Fatalf("%q %+v", o, e.rec.evs)
	}
}

func TestSessionStartWritesPluginRoot(t *testing.T) {
	dir := t.TempDir()
	e := setup("")
	e.vars = map[string]string{"CLAUDE_PROJECT_DIR": dir, "LOOPS_PLUGIN_ROOT": "/plug"}
	e.h.d.WriteFile = nil                  // real atomic write
	e.runP("codex", "session-start", "{}") // no .loops/ yet: nothing written
	if _, err := os.Stat(filepath.Join(dir, ".loops")); err == nil {
		t.Fatal("littered a project without .loops/")
	}
	if err := os.Mkdir(filepath.Join(dir, ".loops"), 0o755); err != nil {
		t.Fatal(err)
	}
	if c, o, _ := e.runP("codex", "session-start", "{}"); c != 0 || !strings.Contains(o, "loops binary: /plug/bin/loops") {
		t.Fatalf("%d %q", c, o)
	}
	b, err := os.ReadFile(filepath.Join(dir, ".loops", "plugin-root"))
	if err != nil || string(b) != "/plug" {
		t.Fatalf("%q %v", b, err)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".loops", ".plugin-root-*")); len(left) != 0 {
		t.Fatal(left)
	}
	// a failing writer never fails the hook
	e.h.d.WriteFile = func(string, []byte) error { return errors.New("ro") }
	if c, o, _ := e.runP("codex", "session-start", "{}"); c != 0 || o == "" {
		t.Fatalf("%d %q", c, o)
	}
	// without a plugin root: nothing written, no binary sentence
	dir2 := t.TempDir()
	e = setup("")
	e.vars = map[string]string{"CLAUDE_PROJECT_DIR": dir2}
	e.h.d.WriteFile = nil
	e.runP("claude", "session-start", "{}")
	if _, err := os.Stat(filepath.Join(dir2, ".loops")); err == nil {
		t.Fatal("wrote without root")
	}
}

func TestRelativeReadPathMadeAbsolute(t *testing.T) {
	gem := `{"hook_event_name":"BeforeTool","tool_name":"read_file","cwd":"/repo","tool_input":{"file_path":"big.go"}}`
	e := setup("r")
	if c, _, er := e.runP("gemini", "read-guard", gem); c != 2 || !strings.Contains(er, "read_guard:") {
		t.Fatalf("gemini relative: %d %q", c, er)
	}
	e = setup("r") // no cwd: project dir
	if c, _, _ := e.runP("gemini", "read-guard", `{"tool_name":"read_file","tool_input":{"file_path":"big.go"}}`); c != 2 {
		t.Fatal("project dir fallback")
	}
	e = setup("r")
	e.runP("gemini", "trace", `{"hook_event_name":"AfterTool","tool_name":"read_file","cwd":"/repo/pkg","tool_input":{"file_path":"a.go"}}`)
	if e.rec.evs[0].Target != "pkg/a.go" {
		t.Fatalf("%+v", e.rec.evs)
	}
}

func TestSessionStartWritesUserLevelPluginRoot(t *testing.T) {
	home := t.TempDir()
	e := setup("")
	e.vars = map[string]string{"CLAUDE_PROJECT_DIR": t.TempDir(), "LOOPS_PLUGIN_ROOT": "/plug", "HOME": home}
	e.h.d.WriteFile = nil
	e.runP("gemini", "session-start", "{}") // project has no .loops/: user file still written
	if b, err := os.ReadFile(filepath.Join(home, ".local", "state", "loops", "plugin-root")); err != nil || string(b) != "/plug" {
		t.Fatalf("%q %v", b, err)
	}
	xdg := t.TempDir()
	e.vars["XDG_STATE_HOME"] = xdg
	e.runP("gemini", "session-start", "{}")
	if b, err := os.ReadFile(filepath.Join(xdg, "loops", "plugin-root")); err != nil || string(b) != "/plug" {
		t.Fatalf("xdg %q %v", b, err)
	}
	e.h.d.WriteFile = func(string, []byte) error { return errors.New("ro") }
	if c, o, _ := e.runP("gemini", "session-start", "{}"); c != 0 || o == "" {
		t.Fatal("write failure must be silent")
	}
}
