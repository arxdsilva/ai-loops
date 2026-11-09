package hook

import (
	"context"
	"encoding/json"
	"io"

	"github.com/arxdsilva/ai-loops/internal/application/guard"
)

// raceGuard denies Bash commands that run go test without -race. It is a
// hard gate: there is no environment opt-out.
func (h *Handler) raceGuard(ctx context.Context, p payload, errw io.Writer) int {
	if p.ToolName != "Bash" {
		return 0
	}
	var in toolInput
	if json.Unmarshal(p.ToolInput, &in) != nil {
		return 0
	}
	if reason := guard.CheckRace(in.Command); reason != "" {
		return h.deny(ctx, p, "race-guard", reason, errw)
	}
	return 0
}
