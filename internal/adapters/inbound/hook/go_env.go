package hook

import (
	"context"
	"encoding/json"
	"io"
	"os"

	"github.com/arxdsilva/ai-loops/internal/application/guard"
	"github.com/arxdsilva/ai-loops/internal/domain/workspace"
)

const goEnvMsg = "go_env: this run uses its own Go cache — run `loops go <args>` or prefix `eval \"$(loops run env)\";` (WORKFLOW §2.6)"

// goEnvAction is how a platform handles an unscoped go command.
type goEnvAction int

const (
	goEnvDeny         goEnvAction = iota // input rewriting unsupported or unverified
	goEnvRewrite                         // rewrite; the normal permission flow decides
	goEnvRewriteAllow                    // rewrite and approve (only gate commands)
)

// goEnvPolicy picks the action per platform (facts in dialect.go). Codex
// and Cursor only accept a rewrite together with an allow decision, so
// there it is limited to a lone `go build|test|vet|list`; everything else
// is denied with the way out.
func goEnvPolicy(platform string, use guard.GoUse) goEnvAction {
	switch platform {
	case "claude":
		if use.Gate {
			return goEnvRewriteAllow
		}
		return goEnvRewrite
	case "codex", "cursor":
		if use.Gate {
			return goEnvRewriteAllow
		}
	case "gemini", "windsurf":
		return goEnvRewrite
	}
	return goEnvDeny
}

// goEnv scopes go commands to the active run's GOCACHE/GOTMPDIR. With no
// active run, a non-go command, an already-scoped command, or a command
// race-guard denies (race-guard reports that one), it stays silent.
func (h *Handler) goEnv(ctx context.Context, d dialect, p payload, out, errw io.Writer) int {
	if p.ToolName != "Bash" {
		return 0
	}
	var in map[string]any
	if json.Unmarshal(p.ToolInput, &in) != nil {
		return 0
	}
	cmd, _ := in["command"].(string)
	if cmd == "" || guard.CheckRace(cmd) != "" {
		return 0
	}
	id, err := h.d.Rec.Current(ctx)
	if err != nil || id == "" {
		return 0
	}
	dir, err := h.d.Rec.Dir(ctx, id)
	if err != nil || dir == "" {
		return 0
	}
	cache, tmp := workspace.GoCache(dir), workspace.GoTmp(dir)
	use := guard.InspectGo(cmd, cache)
	if !use.Invokes || !use.Unscoped {
		return 0
	}
	action := goEnvPolicy(h.platform, use)
	scoped := guard.ScopeGo(cmd, cache, tmp)
	if action == goEnvDeny || guard.InspectGo(scoped, cache).Unscoped {
		return h.deny(ctx, p, "go-env", goEnvMsg, errw)
	}
	for _, dd := range []string{cache, tmp} {
		if os.MkdirAll(dd, 0o755) != nil {
			return 0 // go would fail on a missing GOTMPDIR: leave the call alone
		}
	}
	in["command"] = scoped
	d.rewriteTool(out, in, action == goEnvRewriteAllow)
	return 0
}
