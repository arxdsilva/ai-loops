package hook

import (
	"context"
	"time"

	"github.com/arxdsilva/ai-loops/internal/application/updatecheck"
	"github.com/arxdsilva/ai-loops/internal/domain/update"
)

// updateBudget bounds the whole update check inside session-start.
const updateBudget = 1600 * time.Millisecond

// updateNotice returns the notice to show, or nil. It is silent and fails
// open: opt-outs, errors and slow networks all yield nil.
func (h *handler) updateNotice(ctx context.Context) *updatecheck.Notice {
	if h.d.Update == nil || h.d.Version == "" {
		return nil
	}
	if update.OptOut(h.d.Env("LOOPS_NO_UPDATE_CHECK"), h.d.Env("CI"), h.d.UpdateOff) != "" {
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
func vtag(v string) string { return update.Tag(v) }
