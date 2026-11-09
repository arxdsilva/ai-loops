package hook

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/application/guard"
	"github.com/arxdsilva/ai-loops/internal/domain/workspace"
)

// dirRec is a recorder whose run directory is a real temp dir.
type dirRec struct {
	fakeRec
	dir string
}

func (r *dirRec) Dir(context.Context, string) (string, error) { return r.dir, nil }

func goEnvSetup(t *testing.T, cur string) (*env, string) {
	t.Helper()
	e := setup(cur)
	rd := filepath.Join(t.TempDir(), "runs", "r1")
	rec := &dirRec{fakeRec: fakeRec{cur: cur}, dir: rd}
	e.h.d.Rec = rec
	return e, rd
}

// bashPayload is a PreToolUse shell call in each platform's documented shape.
func bashPayload(platform, cmd string) string {
	switch platform {
	case "cursor":
		return mustJSON(map[string]any{"hook_event_name": "preToolUse", "tool_name": "Shell", "tool_input": map[string]any{"command": cmd, "working_directory": "/repo"}, "cwd": "/repo"})
	case "gemini":
		return mustJSON(map[string]any{"hook_event_name": "BeforeTool", "tool_name": "run_shell_command", "tool_input": map[string]any{"command": cmd, "dir_path": "."}, "cwd": "/repo"})
	case "windsurf":
		return mustJSON(map[string]any{"hook_event_name": "PreToolUse", "tool_name": "exec", "tool_input": map[string]any{"command": cmd}})
	case "copilot":
		args, _ := json.Marshal(map[string]any{"command": cmd})
		return mustJSON(map[string]any{"toolName": "bash", "toolArgs": string(args), "cwd": "/repo"})
	case "codex":
		return mustJSON(map[string]any{"hook_event_name": "PreToolUse", "tool_name": "Bash", "tool_input": map[string]any{"command": cmd}, "cwd": "/repo"})
	}
	return mustJSON(map[string]any{"hook_event_name": "PreToolUse", "tool_name": "Bash", "tool_input": map[string]any{"command": cmd, "description": "build", "timeout": 120000}, "cwd": "/repo"})
}

// rewritten extracts the rewritten command and the allow decision from a
// platform's output.
func rewritten(t *testing.T, platform, out string) (cmd string, allow bool, input map[string]any) {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("%s: bad JSON %q", platform, out)
	}
	switch platform {
	case "cursor":
		input, _ = v["updated_input"].(map[string]any)
		allow = v["permission"] == "allow"
	case "gemini":
		hso, _ := v["hookSpecificOutput"].(map[string]any)
		if hso["hookEventName"] != "BeforeTool" {
			t.Fatalf("gemini event %v", hso)
		}
		input, _ = hso["tool_input"].(map[string]any)
		allow = hso["permissionDecision"] == "allow"
	default:
		hso, _ := v["hookSpecificOutput"].(map[string]any)
		if hso["hookEventName"] != "PreToolUse" {
			t.Fatalf("%s event %v", platform, hso)
		}
		input, _ = hso["updatedInput"].(map[string]any)
		allow = hso["permissionDecision"] == "allow"
	}
	cmd, _ = input["command"].(string)
	return cmd, allow, input
}

func TestGoEnvNoRunPassthrough(t *testing.T) {
	for _, p := range allPlatforms {
		e, rd := goEnvSetup(t, "")
		if c, o, er := e.runP(p, "go-env", bashPayload(p, "go build ./...")); c != 0 || o != "" || er != "" {
			t.Errorf("%s: %d %q %q", p, c, o, er)
		}
		if _, err := os.Stat(rd); err == nil {
			t.Errorf("%s: created the run dir without a run", p)
		}
	}
}

func TestGoEnvIgnoresNonGoAndScopedCommands(t *testing.T) {
	e, rd := goEnvSetup(t, "r1")
	cache := workspace.GoCache(rd)
	for _, p := range allPlatforms {
		for _, cmd := range []string{
			"ls -la", "go version", "go mod tidy", "loops go test -race ./...",
			"GOCACHE=" + cache + " go build ./...",
			"export GOCACHE='" + cache + "'; cd x && go test -race ./...",
			`eval "$(loops run env)"; go vet ./...`,
			"go test ./...", // race-guard denies it; go-env stays silent
		} {
			if c, o, er := e.runP(p, "go-env", bashPayload(p, cmd)); c != 0 || o != "" || er != "" {
				t.Errorf("%s %q: %d %q %q", p, cmd, c, o, er)
			}
		}
	}
	if c, o, _ := e.runP("claude", "go-env", mustJSON(map[string]any{"tool_name": "Read", "tool_input": map[string]any{"file_path": "/x"}})); c != 0 || o != "" {
		t.Fatal(c, o)
	}
	if c, o, _ := e.runP("claude", "go-env", `{"tool_name":"Bash","tool_input":{"command":""}}`); c != 0 || o != "" {
		t.Fatal(c, o)
	}
}

func TestGoEnvPerPlatform(t *testing.T) {
	type want int
	const (
		rewriteAllow want = iota
		rewrite
		deny
	)
	cases := []struct {
		cmd  string
		plat map[string]want
	}{
		{"go test -race ./...", map[string]want{
			"claude": rewriteAllow, "codex": rewriteAllow, "cursor": rewriteAllow,
			"gemini": rewrite, "windsurf": rewrite, "copilot": deny, "opencode": deny,
		}},
		{"cd sub && go build ./...", map[string]want{
			"claude": rewrite, "codex": deny, "cursor": deny,
			"gemini": rewrite, "windsurf": rewrite, "copilot": deny, "opencode": deny,
		}},
		{"go run ./cmd/x", map[string]want{
			"claude": rewrite, "codex": deny, "cursor": deny,
			"gemini": rewrite, "windsurf": rewrite, "copilot": deny, "opencode": deny,
		}},
	}
	for _, c := range cases {
		for p, w := range c.plat {
			e, rd := goEnvSetup(t, "r1")
			payload := bashPayload(p, c.cmd)
			if p == "opencode" {
				payload = bashPayload("claude", c.cmd)
			}
			code, out, errOut := e.runP(p, "go-env", payload)
			if w == deny {
				if code != 2 || out != "" || !strings.Contains(errOut, "loops go <args>") || !strings.Contains(errOut, "loops run env") {
					t.Errorf("%s %q: want deny, got %d %q %q", p, c.cmd, code, out, errOut)
				}
				if n := len(e.h.d.Rec.(*dirRec).evs); n != 1 || e.h.d.Rec.(*dirRec).evs[0].Name != "go-env" {
					t.Errorf("%s: deny not recorded: %+v", p, e.h.d.Rec.(*dirRec).evs)
				}
				continue
			}
			if code != 0 || errOut != "" {
				t.Errorf("%s %q: %d %q", p, c.cmd, code, errOut)
				continue
			}
			got, allow, input := rewritten(t, p, out)
			if want := guard.ScopeGo(c.cmd, workspace.GoCache(rd), workspace.GoTmp(rd)); got != want {
				t.Errorf("%s %q: rewrote to %q, want %q", p, c.cmd, got, want)
			}
			if allow != (w == rewriteAllow) {
				t.Errorf("%s %q: allow=%v", p, c.cmd, allow)
			}
			if guard.CheckRace(got) != "" {
				t.Errorf("%s: race verdict changed: %q", p, got)
			}
			if fi, err := os.Stat(workspace.GoTmp(rd)); err != nil || !fi.IsDir() {
				t.Errorf("%s: GOTMPDIR not created: %v", p, err)
			}
			switch p { // unchanged fields survive (Claude replaces the whole input)
			case "claude":
				if input["description"] != "build" || input["timeout"] != float64(120000) {
					t.Errorf("claude input lost fields: %v", input)
				}
			case "cursor":
				if input["working_directory"] != "/repo" {
					t.Errorf("cursor input lost fields: %v", input)
				}
			}
		}
	}
}

func TestGoEnvConflictingCacheDenied(t *testing.T) {
	e, _ := goEnvSetup(t, "r1")
	code, out, errOut := e.runP("claude", "go-env", bashPayload("claude", "GOCACHE=/elsewhere go build ./..."))
	if code != 2 || out != "" || !strings.Contains(errOut, "go_env:") {
		t.Fatalf("%d %q %q", code, out, errOut)
	}
}

func TestGoEnvMkdirFailureFailsOpen(t *testing.T) {
	e, rd := goEnvSetup(t, "r1")
	if err := os.MkdirAll(filepath.Dir(rd), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rd, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	if c, o, er := e.runP("claude", "go-env", bashPayload("claude", "go build ./...")); c != 0 || o != "" || er != "" {
		t.Fatalf("%d %q %q", c, o, er)
	}
}
