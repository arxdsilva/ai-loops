package guard_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/application/guard"
)

func TestCheckWrite_ReadOnlyRoles(t *testing.T) {
	const (
		projectDir = "/repo"
		runDir     = "/repo/.loops/runs/r1"
	)
	readOnly := []string{
		"code-researcher",
		"hardware-researcher",
		"plan-reviewer",
		"planner",
		"practices-researcher",
		"product-reviewer",
		"qa-verifier",
		"reflector",
		"security-reviewer",
		"web-researcher",
	}

	for _, role := range readOnly {
		wantDeny := fmt.Sprintf("write_guard: role %s is read-only and may only write inside the run directory", role)
		cases := []struct {
			name     string
			role     string
			filePath string
			want     string
		}{
			{"inside runDir", role, filepath.Join(runDir, "artifact.md"), ""},
			{"nested inside runDir", role, filepath.Join(runDir, "research", "code.md"), ""},
			{"prefixed inside runDir", "loops:" + role, filepath.Join(runDir, "artifact.md"), ""},
			{"outside runDir", role, filepath.Join(projectDir, "internal", "x.go"), wantDeny},
			{"prefixed outside runDir", "loops:" + role, filepath.Join(projectDir, "internal", "x.go"), wantDeny},
			{"whitespace padded outside", "  loops:" + role + "  ", filepath.Join(projectDir, "README.md"), wantDeny},
		}
		for _, tc := range cases {
			t.Run(role+"/"+tc.name, func(t *testing.T) {
				got := guard.CheckWrite(tc.role, tc.filePath, runDir, projectDir)
				if got != tc.want {
					t.Fatalf("CheckWrite(%q, %q, %q, %q) = %q, want %q", tc.role, tc.filePath, runDir, projectDir, got, tc.want)
				}
			})
		}
	}
}

func TestCheckWrite_ImplementersAndFailOpen(t *testing.T) {
	const (
		projectDir = "/repo"
		runDir     = "/repo/.loops/runs/r1"
	)
	cases := []struct {
		name       string
		role       string
		filePath   string
		runDir     string
		projectDir string
	}{
		{"domain-engineer outside runDir", "domain-engineer", "/repo/internal/domain/x.go", runDir, projectDir},
		{"loops domain-engineer outside runDir", "loops:domain-engineer", "/repo/internal/domain/x.go", runDir, projectDir},
		{"docs-writer outside runDir", "docs-writer", "/repo/README.md", runDir, projectDir},
		{"architect outside runDir", "architect", "/repo/internal/application/port/x.go", runDir, projectDir},
		{"unknown role outside runDir", "custom-role", "/repo/main.go", runDir, projectDir},
		{"empty role", "", "/repo/main.go", runDir, projectDir},
		{"whitespace role", "   ", "/repo/main.go", runDir, projectDir},
		{"bare loops prefix", "loops:", "/repo/main.go", runDir, projectDir},
		{"empty filePath", "planner", "", runDir, projectDir},
		{"empty runDir", "planner", "/repo/main.go", "", projectDir},
		{"empty runDir even for reflector", "reflector", "/repo/.loops/memory/p.md", "", projectDir},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := guard.CheckWrite(tc.role, tc.filePath, tc.runDir, tc.projectDir); got != "" {
				t.Fatalf("got %q, want empty string", got)
			}
		})
	}
}

func TestCheckWrite_ReflectorAndContainment(t *testing.T) {
	const (
		projectDir = "/repo"
		runDir     = "/repo/.loops/runs/r1"
	)
	denyMsg := func(role string) string {
		return fmt.Sprintf("write_guard: role %s is read-only and may only write inside the run directory", role)
	}

	cases := []struct {
		name       string
		role       string
		filePath   string
		runDir     string
		projectDir string
		want       string
	}{
		{"reflector inside memory", "reflector", "/repo/.loops/memory/decisions.md", runDir, projectDir, ""},
		{"reflector nested memory", "loops:reflector", "/repo/.loops/memory/gotchas/g1.md", runDir, projectDir, ""},
		{"reflector memory root", "reflector", "/repo/.loops/memory", runDir, projectDir, ""},
		{"reflector inside runDir", "reflector", "/repo/.loops/runs/r1/reflection.md", runDir, projectDir, ""},
		{"reflector outside memory under .loops", "reflector", "/repo/.loops/config.json", runDir, projectDir, denyMsg("reflector")},
		{"reflector sibling memory prefix", "reflector", "/repo/.loops/memory2/x.md", runDir, projectDir, denyMsg("reflector")},
		{"reflector memory traversal escape", "reflector", "/repo/.loops/memory/../config.json", runDir, projectDir, denyMsg("reflector")},
		{"reflector without projectDir", "reflector", "/repo/.loops/memory/decisions.md", runDir, "", denyMsg("reflector")},
		{"non-reflector denied in memory", "code-researcher", "/repo/.loops/memory/decisions.md", runDir, projectDir, denyMsg("code-researcher")},
		{"planner denied in memory", "planner", "/repo/.loops/memory/decisions.md", runDir, projectDir, denyMsg("planner")},

		// withinDir edge cases: traversal, sibling-prefix, clean, relative/absolute mismatch
		{"traversal escaping runDir", "planner", "/repo/.loops/runs/r1/../r2/plan.v1.md", runDir, projectDir, denyMsg("planner")},
		{"nested traversal escaping runDir", "qa-verifier", "/repo/.loops/runs/r1/qa/../../other.md", runDir, projectDir, denyMsg("qa-verifier")},
		{"parent dir of runDir", "planner", "/repo/.loops/runs", runDir, projectDir, denyMsg("planner")},
		{"parent via dotdot", "planner", "/repo/.loops/runs/r1/..", runDir, projectDir, denyMsg("planner")},
		{"sibling prefix dir", "code-researcher", "/repo/.loops/runs/r10/x", runDir, projectDir, denyMsg("code-researcher")},
		{"internal dotdot staying inside", "planner", "/repo/.loops/runs/r1/sub/../plan.v1.md", runDir, projectDir, ""},
		{"trailing slash on runDir", "planner", "/repo/.loops/runs/r1/plan.v1.md", "/repo/.loops/runs/r1/", projectDir, ""},
		{"dot segment in target", "planner", "/repo/.loops/runs/r1/./plan.v1.md", runDir, projectDir, ""},
		{"exact runDir path", "planner", "/repo/.loops/runs/r1", runDir, projectDir, ""},
		{"relative target vs abs runDir", "planner", "plan.v1.md", runDir, projectDir, denyMsg("planner")},
		{"both relative inside", "planner", "runs/r1/plan.v1.md", "runs/r1", projectDir, ""},
		{"both relative outside", "planner", "runs/r10/plan.v1.md", "runs/r1", projectDir, denyMsg("planner")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := guard.CheckWrite(tc.role, tc.filePath, tc.runDir, tc.projectDir)
			if got != tc.want {
				t.Fatalf("CheckWrite(%q, %q, %q, %q) = %q, want %q", tc.role, tc.filePath, tc.runDir, tc.projectDir, got, tc.want)
			}
		})
	}
}
