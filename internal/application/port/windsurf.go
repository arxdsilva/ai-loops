package port

import (
	"fmt"
	"io/fs"

	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
)

// windsurfEvents are Devin Local hook events that map from canonical ones
// (https://docs.devin.ai/cli/extensibility/hooks.md); same names as Claude.
var windsurfEvents = map[plugin.Event]bool{
	plugin.EventSessionStart: true,
	plugin.EventPreToolUse:   true,
	plugin.EventPostToolUse:  true,
	plugin.EventStop:         true,
}

// buildWindsurf emits platforms/windsurf/ (specs/PLATFORMS.md §3.6).
func buildWindsurf(p plugin.Plugin, src fs.FS, o Options) ([]plugin.File, []string, error) {
	base, err := baseFiles(p, src, plugin.Windsurf, o)
	if err != nil {
		return nil, nil, err
	}
	files := append(toAgentsSkills(base), roleFiles(p)...)
	var warns []string

	hooks := map[string][]any{}
	seen := map[string]bool{}
	for _, h := range p.Hooks {
		if !windsurfEvents[h.Event] {
			continue
		}
		key := string(h.Event) + "/" + h.Name
		if seen[key] {
			continue
		}
		seen[key] = true
		hooks[string(h.Event)] = append(hooks[string(h.Event)], map[string]any{
			"hooks": []any{map[string]any{
				"type":    "command",
				"command": "loops hook " + h.Name + " --platform windsurf",
				"timeout": h.TimeoutSec,
			}},
		})
	}
	// The file is the hooks object itself, no wrapper key.
	files = append(files, emitJSON(".devin/hooks.v1.json", hooks))
	files = append(files, plugin.File{Path: "README.md", Data: []byte(windsurfReadme)})
	sortByPath(files)

	for _, e := range []plugin.Event{plugin.EventPostToolUseFailure, plugin.EventSubagentStart, plugin.EventSubagentStop} {
		if hasEvent(p, e) {
			warns = append(warns, fmt.Sprintf("windsurf: %s has no Devin event; its hooks are dropped", e))
		}
	}
	warns = append(warns,
		"windsurf: no subagents; the work skill runs in solo mode",
		"windsurf: hook commands are rewritten to the plugin-home shim by `loops install`; using the package without the installer needs `loops` on PATH",
		"windsurf: slash commands are the skills themselves (no command files)")
	return files, warns, nil
}

const windsurfReadme = `# loops for Windsurf (Devin Local)

Tier D (no subagents): solo mode. Installed with ` + "`loops install --target windsurf`" + `.

- Skills land in .agents/skills/; rules come from AGENTS.md.
- .devin/hooks.v1.json registers SessionStart, PreToolUse (read-guard, model-guard),
  PostToolUse (go-post-edit, trace) and Stop (stop-gate) with ` + "`--platform windsurf`" + `.
  It runs ` + "`loops`" + ` from PATH. Run /hooks in Devin to confirm they loaded.
- Use the work skill as the /work equivalent: ask the agent to "use the work skill: <task>".
- Solo mode: the lead plays each role in turn, writing the same run-dir files and
  events (` + "`loops run label --agent solo-<n>`" + `). Subagent lanes are not tracked.
`
