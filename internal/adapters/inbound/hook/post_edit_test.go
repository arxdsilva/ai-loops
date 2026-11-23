package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A multi-file patch applied from a subdirectory resolves its extra paths
// against the cwd, not the project root; detection and ownership stay on the
// project root (PR review: patch checks use wrong files).
func TestMultiFilePatchFromSubdirectoryUsesCwd(t *testing.T) {
	project := t.TempDir()
	sub := filepath.Join(project, "pkg")
	if err := os.MkdirAll(filepath.Join(project, ".loops"), 0o755); err != nil {
		t.Fatal(err)
	}
	patch := "*** Begin Patch\n*** Update File: a.go\n*** Update File: b.go\n*** End Patch"
	msg, _ := json.Marshal(map[string]any{
		"hook_event_name": "PostToolUse", "tool_name": "apply_patch", "cwd": sub,
		"tool_input": map[string]string{"command": patch},
	})
	e := setup("r")
	// No project-dir env var: the root comes from `projroot.Find(cwd)`, which
	// stops at the .loops marker above the subdirectory.
	e.vars = map[string]string{}
	// Only the cwd-relative files exist; a root-relative one does not.
	e.h.d.Files = fakeFiles{lines: map[string]int{
		filepath.Join(sub, "a.go"): 1,
		filepath.Join(sub, "b.go"): 1,
	}}
	e.runP("codex", "go-post-edit", string(msg))
	calls := fmt.Sprint(e.cmd.calls)
	if !strings.Contains(calls, filepath.Join(sub, "a.go")) || !strings.Contains(calls, filepath.Join(sub, "b.go")) {
		t.Fatalf("patch files not resolved against cwd: %v", e.cmd.calls)
	}
	if strings.Contains(calls, filepath.Join(project, "a.go")) {
		t.Fatalf("patch file resolved against the project root: %v", e.cmd.calls)
	}
}
