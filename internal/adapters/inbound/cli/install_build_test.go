package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
)

// buildPackage is the real builder behind `loops install`: it must produce
// a complete package from the canonical sources (here, this repository).
func TestBuildPackageFromRepoSources(t *testing.T) {
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude-plugin", "plugin.json")); err != nil {
		t.Skip("repository root not found")
	}
	files, _, err := buildPackage(os.DirFS(root), plugin.OpenCode)
	if err != nil {
		t.Fatalf("buildPackage: %v", err)
	}
	have := map[string]bool{}
	for _, f := range files {
		have[f.Path] = true
	}
	for _, want := range []string{"bin/loops", "ROLES.md", "LICENSE", ".opencode/agents/loops/planner.md", ".agents/skills/work/SKILL.md"} {
		if !have[want] {
			t.Errorf("opencode package lacks %s", want)
		}
	}
	if _, _, err := buildPackage(os.DirFS(t.TempDir()), plugin.OpenCode); err == nil {
		t.Error("buildPackage on an empty tree must fail")
	}
}
