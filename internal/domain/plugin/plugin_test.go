package plugin_test

import (
	"slices"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
)

func TestPlatformsIncludesAntigravityUnique(t *testing.T) {
	if plugin.Antigravity != "antigravity" {
		t.Fatalf("Antigravity = %q, want %q", plugin.Antigravity, "antigravity")
	}
	if !slices.Contains(plugin.Platforms[:], plugin.Antigravity) {
		t.Fatalf("Platforms %v does not contain %q", plugin.Platforms, plugin.Antigravity)
	}
	if got := plugin.Platforms[len(plugin.Platforms)-1]; got != plugin.Antigravity {
		t.Fatalf("last platform = %q, want %q", got, plugin.Antigravity)
	}
	seen := make(map[plugin.Platform]bool, len(plugin.Platforms))
	for _, p := range plugin.Platforms {
		if p == "" {
			t.Fatal("Platforms contains empty platform")
		}
		if seen[p] {
			t.Fatalf("duplicate platform %q in %v", p, plugin.Platforms)
		}
		seen[p] = true
	}
}
