package guard_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/application/guard"
)

func TestCheckModelTier(t *testing.T) {
	tiers := map[string]string{"domain-engineer": "sonnet", "code-researcher": "haiku", "planner": "opus", "odd": "weird"}
	of := func(role string) string { return tiers[role] }
	const reason = "plan lane W2-domain: model column says opus (hard invariants)"
	tests := []struct {
		name string
		in   map[string]any
		deny bool
	}{
		{"same tier", map[string]any{"subagent_type": "loops:domain-engineer", "model": "sonnet"}, false},
		{"lower tier", map[string]any{"subagent_type": "loops:domain-engineer", "model": "haiku"}, false},
		{"opus role, lower model", map[string]any{"subagent_type": "loops:planner", "model": "sonnet"}, false},
		{"above tier", map[string]any{"subagent_type": "loops:domain-engineer", "model": "opus"}, true},
		{"haiku role to sonnet", map[string]any{"subagent_type": "loops:code-researcher", "model": "sonnet"}, true},
		{"full id above tier", map[string]any{"subagent_type": "loops:domain-engineer", "model": "claude-opus-5-5"}, true},
		{"full id same tier", map[string]any{"subagent_type": "loops:domain-engineer", "model": "claude-sonnet-5-5"}, false},
		{"unprefixed role", map[string]any{"subagent_type": "domain-engineer", "model": "opus"}, true},
		{"reason line allows", map[string]any{"subagent_type": "loops:domain-engineer", "model": "opus", "prompt": "Role: x\nModel reason: " + reason + "\nGoal: y"}, false},
		{"indented reason allows", map[string]any{"subagent_type": "loops:domain-engineer", "model": "opus", "prompt": "  Model reason: escalated after QA FAILED twice"}, false},
		{"empty reason denies", map[string]any{"subagent_type": "loops:domain-engineer", "model": "opus", "prompt": "Model reason:   \nGoal: y"}, true},
		{"reason mid-line denies", map[string]any{"subagent_type": "loops:domain-engineer", "model": "opus", "prompt": "see Model reason: x"}, true},
		{"inherit", map[string]any{"subagent_type": "loops:domain-engineer", "model": "inherit"}, false},
		{"unknown model", map[string]any{"subagent_type": "loops:domain-engineer", "model": "gpt-9"}, false},
		{"unknown role", map[string]any{"subagent_type": "loops:nobody", "model": "opus"}, false},
		{"role with unknown tier", map[string]any{"subagent_type": "loops:odd", "model": "opus"}, false},
		{"built-in agent", map[string]any{"subagent_type": "Explore", "model": "opus"}, false},
		{"general-purpose with model", map[string]any{"subagent_type": "general-purpose", "model": "opus"}, false},
		{"general-purpose without model", map[string]any{"subagent_type": "general-purpose"}, true},
		{"typed, no model", map[string]any{"subagent_type": "loops:domain-engineer"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := json.Marshal(tc.in)
			got := guard.CheckModel(b, of)
			if tc.deny != (got != "") {
				t.Fatalf("deny=%v, got %q", tc.deny, got)
			}
			if got != "" && !strings.HasPrefix(got, "model_guard:") {
				t.Fatalf("reason must start with model_guard: %q", got)
			}
		})
	}
	// The tier deny names the tier and the line to add.
	b, _ := json.Marshal(tests[3].in)
	got := guard.CheckModel(b, of)
	for _, want := range []string{"sonnet", "opus", "WORKFLOW §4", "Model reason: <reason>"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q lacks %q", got, want)
		}
	}
}
