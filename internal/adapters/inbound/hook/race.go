package hook

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/application/guard"
)

// testGuard (alias race-guard) denies Bash commands that run tests without
// their pack's race policy: Go always (-race on go test), plus every
// detected pack with race mode "flag" (guard.CheckTests). It is a hard
// gate: there is no environment opt-out. Go denials keep the guard name
// race-guard in events so older reports stay comparable.
func (h *Handler) testGuard(ctx context.Context, p payload, errw io.Writer) int {
	if p.ToolName != "Bash" {
		return 0
	}
	var in toolInput
	if json.Unmarshal(p.ToolInput, &in) != nil {
		return 0
	}
	if reason := h.checkTests(p, in.Command); reason != "" {
		name := "test-guard"
		if strings.HasPrefix(reason, "race_guard:") {
			name = "race-guard"
		}
		return h.deny(ctx, p, name, reason, errw)
	}
	return 0
}

// checkTests is guard.CheckTests with the detected packs (Go only when
// detection is unavailable).
func (h *Handler) checkTests(p payload, command string) string {
	if command == "" {
		return ""
	}
	ds, _ := h.detect(p)
	return guard.CheckTests(command, packsOf(ds))
}
