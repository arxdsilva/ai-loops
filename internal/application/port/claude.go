package port

import (
	"fmt"
	"io/fs"

	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
)

// buildClaude emits platforms/claude/ (specs/PLATFORMS.md §3.1): the
// repository root is the Claude Code plugin, so the package is a README
// with the install steps.
func buildClaude(p plugin.Plugin, src fs.FS, o Options) ([]plugin.File, []string, error) {
	readme := fmt.Sprintf(`# %[1]s for Claude Code

The repository root is the Claude Code plugin (`+"`.claude-plugin/`, `agents/`, `skills/`, `hooks/hooks.json`"+`); nothing is generated for Claude. See specs/PLATFORMS.md §3.1.

## Install

    /plugin marketplace add arxdsilva/ai-loops
    /plugin install %[1]s@ai-loops

Commands: `+"`/%[1]s:work`, `/%[1]s:init`, `/%[1]s:status`, `/%[1]s:report`"+`.

The root `+"`bin/loops`"+` shim builds `+"`cmd/loops`"+` from the checkout on first use (Go required).
`, p.Name)
	return []plugin.File{marker(o), {Path: "README.md", Data: []byte(readme)}}, nil, nil
}
