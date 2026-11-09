package hook_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fx(t *testing.T, platform, name, repo string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", platform, name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(b), "/repo", repo)
}

func TestProcessGeminiAndCursor(t *testing.T) {
	repo := repoWithBig(t)
	if err := os.WriteFile(filepath.Join(repo, "big.go"), []byte(strings.Repeat("x\n", 600)), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, p := range []struct{ plat, edit, stop, stopWant string }{
		{"gemini", "edit", "stop", `{"decision":"deny","reason":`},
		{"cursor", "edit", "stop", `{"followup_message":`},
	} {
		// deny: exit 2, reason on stderr, nothing on stdout
		code, o, e := runLoops(t, repo, fx(t, p.plat, "read-guard", repo), nil, "hook", "read-guard", "--platform", p.plat)
		if code != 2 || o != "" || !strings.Contains(e, "read_guard:") {
			t.Errorf("%s deny: %d %q %q", p.plat, code, o, e)
		}
		// no run: stop-gate silent
		if code, o, _ := runLoops(t, repo, fx(t, p.plat, "stop", repo), nil, "hook", "stop-gate", "--platform", p.plat); code != 0 || o != "" {
			t.Errorf("%s no run: %d %q", p.plat, code, o)
		}
		if code, _, e := runLoops(t, repo, "", nil, "run", "start", "--title", p.plat); code != 0 {
			t.Fatal(e)
		}
		// trace a .go edit, then the stop gate blocks in the platform's dialect
		if code, _, _ := runLoops(t, repo, fx(t, p.plat, p.edit, repo), nil, "hook", "trace", "--platform", p.plat); code != 0 {
			t.Fatal(code)
		}
		code, o, _ = runLoops(t, repo, fx(t, p.plat, p.stop, repo), nil, "hook", "stop-gate", "--platform", p.plat)
		if code != 0 || !strings.HasPrefix(o, p.stopWant) || strings.Count(o, "\n") != 1 || !strings.Contains(o, "loops gate") {
			t.Errorf("%s stop: %d %q", p.plat, code, o)
		}
		if _, o, _ := runLoops(t, repo, fx(t, p.plat, "stop-active", repo), nil, "hook", "stop-gate", "--platform", p.plat); o != "" {
			t.Errorf("%s stop-active: %q", p.plat, o)
		}
		// session context in the platform's dialect (exact JSON)
		_, o, _ = runLoops(t, repo, fx(t, p.plat, "session-start", repo), []string{"LOOPS_PLUGIN_ROOT=/plug"}, "hook", "session-start", "--platform", p.plat)
		if p.plat == "cursor" && !strings.HasPrefix(o, `{"additional_context":"loops plugin active. Plugin root: /plug.`) {
			t.Errorf("cursor session: %q", o)
		}
		if p.plat == "gemini" && !strings.HasPrefix(o, `{"hookSpecificOutput":{"additionalContext":"loops plugin active. Plugin root: /plug.`) {
			t.Errorf("gemini session: %q", o)
		}
		if code, _, e := runLoops(t, repo, "", nil, "run", "end"); code != 0 {
			t.Logf("run end: %s", e)
		}
	}
}

func TestProcessAutodetect(t *testing.T) {
	repo := repoWithBig(t)
	_ = os.WriteFile(filepath.Join(repo, "big.go"), []byte(strings.Repeat("x\n", 600)), 0o644)
	code, _, e := runLoops(t, repo, fx(t, "gemini", "read-guard", repo), []string{"GEMINI_PROJECT_DIR=" + repo}, "hook", "read-guard")
	if code != 2 || !strings.Contains(e, "read_guard:") {
		t.Errorf("%d %q", code, e)
	}
}
