package hook

import (
	"context"
	"encoding/json"
	"github.com/arxdsilva/ai-loops/internal/platform/scratch"
	"io"
	"os"
	"path/filepath"

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

// toolchainEnv (alias go-env) scopes toolchain commands to the active run.
// Go commands get the run's GOCACHE/GOTMPDIR with the per-platform policy
// (goEnvPolicy). Other detected packs' commands get their run-scoped env and
// cache flags (guard.PackScope) where the platform rewrites without an
// approval (Claude, Gemini, Windsurf); they are never denied. With no
// active run, an already-scoped command, or a command test-guard denies
// (test-guard reports that one), it stays silent.
func (h *Handler) toolchainEnv(ctx context.Context, d dialect, p payload, out, errw io.Writer) int {
	if p.ToolName != "Bash" {
		return 0
	}
	var in map[string]any
	if json.Unmarshal(p.ToolInput, &in) != nil {
		return 0
	}
	cmd, _ := in["command"].(string)
	if cmd == "" || h.checkTests(p, cmd) != "" {
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
	dir = scratch.Dir(dir) // the run's caches live in its scratch dir
	if code, handled := h.goEnv(ctx, d, p, in, cmd, dir, out, errw); handled {
		return code
	}
	h.packEnv(d, p, in, cmd, dir, out)
	return 0
}

// goEnv scopes go commands to the run's GOCACHE/GOTMPDIR. handled is false
// when cmd runs no unscoped go command.
func (h *Handler) goEnv(ctx context.Context, d dialect, p payload, in map[string]any, cmd, dir string, out, errw io.Writer) (code int, handled bool) {
	cache, tmp := workspace.GoCache(dir), workspace.GoTmp(dir)
	use := guard.InspectGo(cmd, cache)
	if !use.Invokes || !use.Unscoped {
		return 0, false
	}
	action := goEnvPolicy(h.platform, use)
	scoped := guard.ScopeGo(cmd, cache, tmp)
	if action == goEnvDeny || guard.InspectGo(scoped, cache).Unscoped {
		return h.deny(ctx, p, "go-env", goEnvMsg, errw), true
	}
	for _, dd := range []string{cache, tmp} {
		if os.MkdirAll(dd, 0o755) != nil {
			return 0, true // go would fail on a missing GOTMPDIR: leave the call alone
		}
	}
	in["command"] = scoped
	d.rewriteTool(out, in, action == goEnvRewriteAllow)
	return 0, true
}

// packEnv rewrites a command of a detected non-Go pack with that pack's
// run-scoped env and cache flags. It never denies: where the platform cannot
// rewrite without approving the call, the command runs as is (agents can
// use `loops exec`).
func (h *Handler) packEnv(d dialect, p payload, in map[string]any, cmd, dir string, out io.Writer) {
	switch h.platform {
	case "claude", "gemini", "windsurf":
	default:
		return
	}
	ds, ok := h.detect(p)
	if !ok {
		return
	}
	for _, det := range ds {
		if det.ID == "go" {
			continue
		}
		s := guard.NewPackScope(det.Pack, dir)
		if !s.Uses(cmd) {
			continue
		}
		scoped, ok := s.Scope(cmd)
		if !ok {
			return // already scoped, or nothing to add
		}
		base := workspace.PackDir(dir, det.ID)
		for _, dd := range []string{filepath.Join(base, "cache"), filepath.Join(base, "tmp")} {
			if os.MkdirAll(dd, 0o755) != nil {
				return
			}
		}
		in["command"] = scoped
		d.rewriteTool(out, in, false)
		return
	}
}
