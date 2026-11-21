package port

import (
	"fmt"
	"io/fs"

	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
)

// antigravityEvents are the canonical hook events supported by Google
// Antigravity (.agents/hooks.json / ~/.gemini/config/hooks.json).
var antigravityEvents = map[plugin.Event]bool{
	plugin.EventSessionStart: true,
	plugin.EventPreToolUse:   true,
	plugin.EventPostToolUse:  true,
	plugin.EventStop:         true,
}

// antigravityMatcherEvents are the Antigravity hook events that accept a
// tool matcher field.
var antigravityMatcherEvents = map[plugin.Event]bool{
	plugin.EventPreToolUse:  true,
	plugin.EventPostToolUse: true,
}

const antigravityReadme = `# loops for Google Antigravity

Tier C (generic subagents; solo fallback). Installed with:

    npx ai-loops --target antigravity
    loops install --target antigravity [--scope project|user]

- Project scope places skills in .agents/skills/, hooks in .agents/hooks.json,
  and shared plugin files (specs, templates, role files, bin/loops shim) in
  .loops/plugin/.
- User scope places skills in ~/.gemini/config/skills/, hooks in
  ~/.gemini/config/hooks.json, and shared plugin files in
  ~/.local/share/loops/plugin/.
- Rules come from AGENTS.md.
- Antigravity app and CLI (` + "`agy`" + `): both load skills from .agents/skills/ and
  ~/.gemini/config/skills/. Run ` + "`/skills reload`" + ` after installing or updating;
  ` + "`agy`" + ` also exposes skills as slash commands (/work, /init, /status, /report).
- .agents/hooks.json registers SessionStart, PreToolUse, PostToolUse and Stop
  with ` + "`--platform antigravity`" + `. ` + "`loops install`" + ` rewrites the hook commands to
  the plugin-home shim; using the package without the installer needs ` + "`loops`" + `
  on PATH.
- Subagents: spawn generic subagents (` + "`invoke_subagent`" + `) whose prompt starts with
  the role file body (` + "`loops path agents`" + ` gives the directory). Per-role models
  and tool restrictions are not enforced, subagent resume is not assumed, and
  ` + "`skills/work`" + ` falls back to solo mode when subagents are unavailable.
`

// buildAntigravity emits platforms/antigravity/ (specs/PLATFORMS.md).
func buildAntigravity(p plugin.Plugin, src fs.FS, o Options) ([]plugin.File, []string, error) {
	base, err := baseFiles(p, src, plugin.Antigravity, o)
	if err != nil {
		return nil, nil, err
	}
	files := append(toAgentsSkills(base), roleFiles(p)...)
	files = append(files, emitJSON(".agents/hooks.json", antigravityHooks(p)))
	files = append(files, plugin.File{Path: "README.md", Data: []byte(antigravityReadme)})
	sortByPath(files)

	var warns []string
	for _, e := range []plugin.Event{plugin.EventPostToolUseFailure, plugin.EventSubagentStart, plugin.EventSubagentStop} {
		if hasEvent(p, e) {
			warns = append(warns, fmt.Sprintf("antigravity: %s has no Antigravity event; its hooks are dropped", e))
		}
	}
	warns = append(warns,
		"antigravity: role subagents are not emitted; Tier C generic subagents run with the role file body as prompt (no subagent resume, solo fallback available), so per-role model and tool restrictions are not enforced",
		"antigravity: hook commands are rewritten to the plugin-home shim by `loops install`; using the package without the installer needs `loops` on PATH",
		"antigravity: slash commands are the skills themselves (no command files)")
	return files, warns, nil
}

func antigravityHooks(p plugin.Plugin) map[string]any {
	hooks := map[string]any{"enabled": true}
	byEvent := hooksByEvent(p.Hooks)
	for _, ev := range eventOrder {
		hs := byEvent[ev]
		if !antigravityEvents[ev] || len(hs) == 0 {
			continue
		}
		entries := make([]any, 0, len(hs))
		for _, h := range hs {
			entry := map[string]any{
				"hooks": []any{map[string]any{
					"type":    "command",
					"command": "loops hook " + h.Name + " --platform antigravity",
					"timeout": h.TimeoutSec,
				}},
			}
			if antigravityMatcherEvents[ev] && h.Matcher != "" {
				entry["matcher"] = h.Matcher
			}
			entries = append(entries, entry)
		}
		hooks[string(ev)] = entries
	}
	return map[string]any{"loops": hooks}
}
