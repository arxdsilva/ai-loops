package hook_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var (
	buildOnce sync.Once
	binPath   string
	buildErr  error
)

func binary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "loops-bin")
		if err != nil {
			buildErr = err
			return
		}
		binPath = filepath.Join(dir, "loops")
		cmd := exec.Command("go", "build", "-o", binPath, "./cmd/loops")
		cmd.Dir = filepath.Join("..", "..", "..", "..")
		if out, err := cmd.CombinedOutput(); err != nil {
			buildErr = fmt.Errorf("%v: %s", err, out)
		}
	})
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	return binPath
}

// runLoops executes the binary in repo with extra env, returning exit code, stdout, stderr.
func runLoops(t *testing.T, repo, stdin string, extra []string, args ...string) (int, string, string) {
	t.Helper()
	cmd := exec.Command(binary(t), args...)
	cmd.Dir = repo
	env := []string{}
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "READ_GUARD") || strings.HasPrefix(kv, "LOOPS_GATE") || strings.HasPrefix(kv, "CLAUDE_PROJECT_DIR") {
			continue
		}
		env = append(env, kv)
	}
	cmd.Env = append(append(env, "CLAUDE_PROJECT_DIR="+repo), extra...)
	cmd.Stdin = strings.NewReader(stdin)
	var o, e bytes.Buffer
	cmd.Stdout, cmd.Stderr = &o, &e
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return code, o.String(), e.String()
}

func repoWithBig(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, n := range map[string]int{"pkg/big.go": 600, "pkg/read600.go": 600} {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(strings.Repeat("x\n", n)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

func TestProcessGuards(t *testing.T) {
	repo := repoWithBig(t)
	bash := `{"tool_name":"Bash","tool_input":{"command":"cat pkg/big.go"},"cwd":"` + repo + `"}`
	read := `{"tool_name":"Read","tool_input":{"file_path":"` + repo + `/pkg/read600.go"},"cwd":"` + repo + `"}`
	cases := []struct {
		name, hook, in string
		env            []string
		code           int
		stderr         string
	}{
		{"bash deny", "read-guard", bash, nil, 2, "read_guard:"},
		{"guard off", "read-guard", bash, []string{"READ_GUARD=off"}, 0, ""},
		{"max view 700", "read-guard", bash, []string{"READ_GUARD_MAX_VIEW_LINES=700"}, 0, ""},
		{"read 600 deny", "read-guard", read, nil, 2, "read_guard:"},
		{"read min 700", "read-guard", read, []string{"READ_GUARD_MIN_LINES=700"}, 0, ""},
		{"not json", "read-guard", "not json", nil, 0, ""},
		{"agent no model", "model-guard", `{"tool_name":"Agent","tool_input":{"prompt":"x"}}`, nil, 2, "model_guard:"},
	}
	for _, c := range cases {
		code, _, errs := runLoops(t, repo, c.in, c.env, "hook", c.hook)
		if code != c.code || !strings.Contains(errs, c.stderr) {
			t.Errorf("%s: code %d stderr %q", c.name, code, errs)
		}
	}
}

func TestProcessTraceAndStopGate(t *testing.T) {
	repo := t.TempDir()
	edit := `{"hook_event_name":"PostToolUse","tool_name":"Edit","tool_input":{"file_path":"` + repo + `/x.go"},"cwd":"` + repo + `"}`
	stop := `{"hook_event_name":"Stop"}`

	// No run: trace and stop-gate do nothing.
	if code, o, _ := runLoops(t, repo, edit, nil, "hook", "trace"); code != 0 || o != "" {
		t.Fatal(code, o)
	}
	if _, err := os.Stat(filepath.Join(repo, ".loops", "runs")); err == nil {
		t.Fatal("trace without run created runs dir")
	}

	if code, _, e := runLoops(t, repo, "", nil, "run", "start", "--title", "proc test"); code != 0 {
		t.Fatal(e)
	}
	if code, _, _ := runLoops(t, repo, edit, nil, "hook", "trace"); code != 0 {
		t.Fatal(code)
	}
	logs, _ := filepath.Glob(filepath.Join(repo, ".loops", "runs", "*", "events.jsonl"))
	if len(logs) != 1 {
		t.Fatal(logs)
	}
	b, _ := os.ReadFile(logs[0])
	if !strings.Contains(string(b), `"kind":"step"`) || !strings.Contains(string(b), `"target":"x.go"`) {
		t.Fatalf("%s", b)
	}

	code, o, _ := runLoops(t, repo, stop, nil, "hook", "stop-gate")
	if code != 0 || !strings.Contains(o, `"decision":"block"`) {
		t.Fatalf("expected block: %d %q", code, o)
	}
	if _, o, _ := runLoops(t, repo, stop, []string{"LOOPS_GATE=off"}, "hook", "stop-gate"); o != "" {
		t.Fatalf("LOOPS_GATE=off: %q", o)
	}
	if code, _, e := runLoops(t, repo, "", nil, "run", "event", "--kind", "gate", "--name", "all", "--ok", "true"); code != 0 {
		t.Fatal(e)
	}
	if _, o, _ := runLoops(t, repo, stop, nil, "hook", "stop-gate"); o != "" {
		t.Fatalf("expected pass after green gate: %q", o)
	}
	if code, o, _ := runLoops(t, repo, "{}", []string{"LOOPS_PLUGIN_ROOT=/plug"}, "hook", "session-start"); code != 0 || !strings.Contains(o, "Active run") {
		t.Fatalf("%d %q", code, o)
	}
}
