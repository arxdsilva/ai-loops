package port

import (
	"fmt"
	"io/fs"

	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
)

// buildCopilot emits platforms/copilot/ (specs/PLATFORMS.md §3.1): Copilot
// reads the Claude layout at the repository root, so the package is a
// README with the install steps.
func buildCopilot(p plugin.Plugin, src fs.FS, o Options) ([]plugin.File, []string, error) {
	readme := fmt.Sprintf(`# %[1]s for GitHub Copilot

Copilot reads the Claude Code layout at the repository root (`+"`.claude-plugin/`, `hooks/hooks.json`, `agents/`, `skills/`"+`) and aliases `+"`${CLAUDE_PLUGIN_ROOT}`"+`, so no package is generated. See specs/PLATFORMS.md §3.1.

## Install

- Copilot CLI:

      copilot plugin install arxdsilva/ai-loops

- VS Code: run **Chat: Install Plugin from Source** and enter `+"`arxdsilva/ai-loops`"+`.

The root `+"`bin/loops`"+` shim builds `+"`cmd/loops`"+` from the checkout on first use (Go required).

## Caveats

- Subagents cannot be resumed: loop-backs spawn a new agent of the same role (PLATFORMS §4, tier B).
- No status line or live pane: use `+"`loops watch`"+` (terminal) or `+"`loops report`"+` (HTML).
`, p.Name)
	return []plugin.File{marker(o), {Path: "README.md", Data: []byte(readme)}}, nil, nil
}
