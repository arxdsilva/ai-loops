package hook

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/application/guard"
	"github.com/arxdsilva/ai-loops/internal/domain/workspace"
)

// testGuard (alias race-guard) denies Bash commands that run tests without
// their pack's race policy: Go always (-race on go test), plus every
// detected pack with race mode "flag" (guard.CheckTests). It is a hard
// gate: there is no environment opt-out. Go denials keep the guard name
// race-guard in events so older reports stay comparable.
//
// A Go race violation is not denied when toolchain-env will rewrite the same
// command on this platform (goPlan): that hook inserts -race itself, so the
// agent loses no turn. Two PreToolUse hooks must not both return updatedInput
// for one call, so this guard stays silent and toolchain-env alone rewrites.
func (h *handler) testGuard(ctx context.Context, p payload, out, errw io.Writer) int {
	if p.ToolName != "Bash" {
		return 0
	}
	var in toolInput
	if json.Unmarshal(p.ToolInput, &in) != nil {
		return 0
	}
	if reason := h.checkTests(p, in.Command); reason != "" {
		if strings.HasPrefix(reason, "race_guard:") {
			if _, ok := h.goPlan(ctx, p, in.Command); ok {
				return 0
			}
		}
		name := "test-guard"
		if strings.HasPrefix(reason, "race_guard:") {
			name = "race-guard"
		}
		return h.deny(ctx, p, name, reason, out, errw)
	}
	return 0
}

// checkTests is guard.CheckTests with the detected packs (Go only when
// detection is unavailable).
func (h *handler) checkTests(p payload, command string) string {
	if command == "" {
		return ""
	}
	ds, _ := h.detect(p)
	return guard.CheckTests(command, packsOf(ds))
}

// goRewrite is toolchain-env's plan for one Go command.
type goRewrite struct {
	command string // the command with -race added where it was missing
	scoped  string // command scoped to the run's cache
	action  goEnvAction
	cache   string
	tmp     string
}

// goPlan reports whether toolchain-env will rewrite cmd on this platform and
// how. It is the one predicate both guards use: a run is active, the platform
// rewrites this shape (goEnvPolicy), cmd uses go unscoped, ScopeGo yields a
// scoped command, and, when cmd breaks the Go race policy, guard.AddRace can
// fix it without leaving another pack's violation behind.
func (h *handler) goPlan(ctx context.Context, p payload, cmd string) (goRewrite, bool) {
	var r goRewrite
	if cmd == "" {
		return r, false
	}
	dir := h.runScratch(ctx)
	if dir == "" {
		return r, false
	}
	r.cache, r.tmp = workspace.GoCache(dir), workspace.GoTmp(dir)
	r.command = cmd
	if h.checkTests(p, cmd) != "" {
		fixed, ok := guard.AddRace(cmd)
		if !ok || h.checkTests(p, fixed) != "" {
			return r, false
		}
		r.command = fixed
	}
	use := guard.InspectGo(r.command, r.cache)
	if !use.Invokes || !use.Unscoped {
		return r, false
	}
	r.action = goEnvPolicy(h.platform, use)
	if r.action == goEnvDeny {
		return r, false
	}
	r.scoped = guard.ScopeGo(r.command, r.cache, r.tmp)
	if guard.InspectGo(r.scoped, r.cache).Unscoped {
		return r, false
	}
	return r, true
}
