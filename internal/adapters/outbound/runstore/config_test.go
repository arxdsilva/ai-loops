package runstore

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/domain/hw"
	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".loops"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".loops", "config.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestLoadConfigParallelAndLimits(t *testing.T) {
	root := writeConfig(t, `{
  "approval": "always",
  "gate": {"skip": ["vuln"], "vuln": true},
  "parallel": {"max_agents": 40, "agent_ram_gib": 1.5, "cpu_slots_per_core": 2, "max_test_runs": 3},
  "limits": {
    "max_agents": 6,
    "phases": {
      "research": {"max_tokens": 200000},
      "implement": {"max_tokens": 2000000, "max_agents": 12, "roles": ["domain-engineer", "api-engineer"]}
    }
  }
}`)
	c, err := LoadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := (hw.Knobs{MaxAgents: 40, AgentRAMGiB: 1.5, CPUSlotsPerCore: 2, MaxTestRuns: 3}); c.Parallel != want {
		t.Fatalf("parallel: %+v", c.Parallel)
	}
	want := run.Limits{MaxAgents: 6, Phases: map[run.Phase]run.PhaseLimit{
		run.PhaseResearch:  {MaxTokens: 200_000},
		run.PhaseImplement: {MaxTokens: 2_000_000, MaxAgents: 12, Roles: []string{"domain-engineer", "api-engineer"}},
	}}
	if !reflect.DeepEqual(c.Limits, want) {
		t.Fatalf("limits: %+v", c.Limits)
	}
	if c.Approval != "always" || !c.Gate.Vuln || len(c.Gate.Skip) != 1 {
		t.Fatalf("other fields: %+v", c)
	}
}

func TestLoadConfigMissingAndEmptyBlocks(t *testing.T) {
	c, err := LoadConfig(t.TempDir())
	if err != nil || !c.Limits.IsZero() || c.Parallel != (hw.Knobs{}) {
		t.Fatalf("missing file: %+v %v", c, err)
	}
	c, err = LoadConfig(writeConfig(t, `{"layers":{}}`))
	if err != nil || !c.Limits.IsZero() || c.Parallel != (hw.Knobs{}) {
		t.Fatalf("no blocks: %+v %v", c, err)
	}
	if _, err := LoadConfig(writeConfig(t, `{"limits": {"max_agents": "six"}}`)); err == nil {
		t.Fatal("bad limits must fail")
	}
	if _, err := LoadConfig(writeConfig(t, `{`)); err == nil {
		t.Fatal("bad JSON must fail")
	}
}

func TestConfigUpdateCheck(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".loops"), 0o755); err != nil {
		t.Fatal(err)
	}
	if c, _ := LoadConfig(root); c.UpdateCheck != nil {
		t.Fatal("default must be nil (on)")
	}
	os.WriteFile(filepath.Join(root, ".loops", "config.json"), []byte(`{"update_check": false}`), 0o644)
	if c, err := LoadConfig(root); err != nil || c.UpdateCheck == nil || *c.UpdateCheck {
		t.Fatalf("%+v %v", c, err)
	}
}

func TestLoadConfigGateCoverage(t *testing.T) {
	c, err := LoadConfig(writeConfig(t, `{"gate": {"coverage_min": 85.5, "coverage_exclude": ["internal/legacy/**"], "base": "origin/develop"}}`))
	if err != nil {
		t.Fatal(err)
	}
	want := GateConfig{CoverageMin: 85.5, CoverageExclude: []string{"internal/legacy/**"}, Base: "origin/develop"}
	if !reflect.DeepEqual(c.Gate, want) {
		t.Fatalf("%+v", c.Gate)
	}
	if c, _ := LoadConfig(writeConfig(t, `{"gate": {}}`)); c.Gate.CoverageMin != 0 || c.Gate.Base != "" {
		t.Fatalf("defaults: %+v", c.Gate)
	}
}

func TestConfigAsk(t *testing.T) {
	cases := []struct{ body, want string }{
		{`{}`, ""},
		{`{"ask": "plan,product"}`, "plan,product"},
		{`{"ask": ["research", "pr"]}`, "research,pr"},
		{`{"approval": "always"}`, "plan"},
		{`{"ask": ["qa"], "approval": "always"}`, "qa,plan"},
		{`{"ask": "all", "auto_pr": true}`, "all"},
	}
	for _, c := range cases {
		cfg, err := LoadConfig(writeConfig(t, c.body))
		if err != nil {
			t.Fatalf("%s: %v", c.body, err)
		}
		if got := cfg.Checkpoints(); got != c.want {
			t.Errorf("%s: Checkpoints() = %q, want %q", c.body, got, c.want)
		}
	}
	if _, err := LoadConfig(writeConfig(t, `{"ask": 3}`)); err == nil || !strings.Contains(err.Error(), `"ask" must be`) {
		t.Fatalf("a number must fail with a clear message: %v", err)
	}
}
