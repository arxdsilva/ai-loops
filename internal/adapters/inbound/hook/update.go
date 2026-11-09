package hook

import (
	"context"
	"strings"
	"time"

	"github.com/arxdsilva/ai-loops/internal/application/updatecheck"
)

// updateBudget bounds the whole update check inside session-start.
const updateBudget = 1600 * time.Millisecond

// updateNotice returns the notice to show, or nil. It is silent and fails
// open: opt-outs, errors and slow networks all yield nil.
func (h *Handler) updateNotice(ctx context.Context) *updatecheck.Notice {
	if h.d.Update == nil || h.d.UpdateOff || h.d.Version == "" {
		return nil
	}
	if h.d.Env("LOOPS_NO_UPDATE_CHECK") != "" {
		return nil
	}
	if ci := strings.ToLower(h.d.Env("CI")); ci == "true" || ci == "1" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, updateBudget)
	defer cancel()
	type res struct{ n *updatecheck.Notice }
	ch := make(chan res, 1)
	go func() {
		n, _ := h.d.Update.Check(ctx, h.d.Version, true)
		ch <- res{n}
	}()
	select {
	case r := <-ch:
		return r.n
	case <-ctx.Done():
		return nil
	}
}

// vtag renders a version with a leading "v".
func vtag(v string) string {
	if v == "" || strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}
