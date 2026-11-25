package hook

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/arxdsilva/ai-loops/internal/platform/scratch"

	"github.com/arxdsilva/ai-loops/internal/application/guard"
	"github.com/arxdsilva/ai-loops/internal/domain/workspace"
)

const goEnvMsg = "go_env: this run uses its own Go cache — run `loops go <args>` or `loops exec -- <cmd>` (WORKFLOW §2.6)"

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
	case "gemini", "windsurf", "antigravity":
		return goEnvRewrite
	}
	return goEnvDeny
}

// toolchainEnv (alias go-env) scopes toolchain commands to the active run.
// Go commands get the run's GOCACHE/GOTMPDIR with the per-platform policy
// (goEnvPolicy). Other detected packs' commands get their run-scoped env and
// cache flags (guard.PackScope) where the platform rewrites without an
// approval (Claude, Gemini, Windsurf, Antigravity); they are never denied.
// With no active run, an already-scoped command, or a command test-guard
// denies (test-guard reports that one), it stays silent.
func (h *handler) toolchainEnv(ctx context.Context, d dialect, p payload, out, errw io.Writer) int {
	if p.ToolName != "Bash" {
		return 0
	}
	var in map[string]any
	if json.Unmarshal(p.ToolInput, &in) != nil {
		return 0
	}
	cmd, _ := in["command"].(string)
	if cmd == "" {
		return 0
	}
	dir := h.runScratch(ctx)
	if dir == "" {
		return 0
	}
	goCmd := cmd
	if h.checkTests(p, cmd) != "" {
		// test-guard denies this command unless this hook rewrites it (goPlan is
		// the shared predicate): then it adds the missing -race itself.
		plan, ok := h.goPlan(ctx, p, cmd)
		if !ok {
			return 0
		}
		// goEnv leaves a call alone when it cannot create the run's dirs;
		// test-guard has already let this one through, so deny it here
		// rather than let it run without -race.
		for _, dd := range []string{plan.cache, plan.tmp} {
			if os.MkdirAll(dd, 0o755) != nil {
				return h.deny(ctx, p, "race-guard", h.checkTests(p, cmd), out, errw)
			}
		}
		goCmd = plan.command
	}
	if code, handled := h.goEnv(ctx, d, p, in, goCmd, dir, out, errw); handled {
		return code
	}
	h.packEnv(d, p, in, cmd, dir, out)
	return 0
}

// runScratch is the active run's scratch dir (where its caches live), or ""
// when no run is active or its directory is unknown.
func (h *handler) runScratch(ctx context.Context) string {
	id, err := h.recorder().Current(ctx)
	if err != nil || id == "" {
		return ""
	}
	dir, err := h.recorder().Dir(ctx, id)
	if err != nil || dir == "" {
		return ""
	}
	return scratch.Dir(dir)
}

// goEnv scopes go commands to the run's GOCACHE/GOTMPDIR. handled is false
// when cmd runs no unscoped go command.
//
// What stays denied on Claude (the only shapes ScopeGo cannot make safe):
// a command that sets its own GOCACHE (`GOCACHE=/x go build`) or removes it
// (`env -u GOCACHE go build`), because prefixing the run's cache would be
// overridden by, or fight with, the command's own setting; and anything whose
// scoped form InspectGo still reports unscoped. Every common shape (cd x &&
// go test, VAR=v go test, pipes, subshells, for loops, env FOO=1 go test,
// assignments before cd) is scoped with an `export` statement instead.
// Platforms without a safe rewrite (goEnvPolicy) deny every unscoped
// command: use `loops go` or `loops exec --`.
func (h *handler) goEnv(ctx context.Context, d dialect, p payload, in map[string]any, cmd, dir string, out, errw io.Writer) (code int, handled bool) {
	cache, tmp := workspace.GoCache(dir), workspace.GoTmp(dir)
	use := guard.InspectGo(cmd, cache)
	if !use.Invokes || !use.Unscoped {
		return 0, false
	}
	action := goEnvPolicy(h.platform, use)
	scoped := guard.ScopeGo(cmd, cache, tmp)
	if action == goEnvDeny || guard.InspectGo(scoped, cache).Unscoped {
		return h.deny(ctx, p, "go-env", goEnvMsg, out, errw), true
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
func (h *handler) packEnv(d dialect, p payload, in map[string]any, cmd, dir string, out io.Writer) {
	switch h.platform {
	case "claude", "gemini", "windsurf", "antigravity":
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
