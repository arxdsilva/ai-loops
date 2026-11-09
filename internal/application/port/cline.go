package port

import (
	"io/fs"

	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
)

// buildCline emits platforms/cline/ (specs/PLATFORMS.md §3.6): skills under
// .agents/skills/ and .cline/skills/.
func buildCline(p plugin.Plugin, src fs.FS, o Options) ([]plugin.File, []string, error) {
	base, err := baseFiles(p, src, plugin.Cline, o)
	if err != nil {
		return nil, nil, err
	}
	files := append(append(toAgentsSkills(base, ".cline/skills/"), roleFiles(p)...), plugin.File{Path: "README.md", Data: []byte(clineReadme)})
	sortByPath(files)
	warns := []string{
		"cline: no subagents; the work skill runs in solo mode (one agent plays each role in turn)",
		"cline: no hooks; guards, tracing and the stop gate do not run",
		"cline: slash commands (workflows) are not emitted; use the skills directly",
	}
	return files, warns, nil
}

const clineReadme = `# loops for Cline

Tier D (no subagents): solo mode. Installed with ` + "`loops install --target cline`" + `.

- Skills land in .agents/skills/ and .cline/skills/; rules come from AGENTS.md.
- Use the work skill as the /work equivalent: ask Cline to "use the work skill: <task>"
  (init, status and report likewise).
- Solo mode: the lead plays each role in turn in phase order, writes the same run-dir
  files and records events with ` + "`loops run label --agent solo-<n>`" + ` and ` + "`loops run event`" + `.
  Before each QA or review pass it re-reads only that role's files.
- No hooks in v1. ` + "`loops watch`" + ` and ` + "`loops report`" + ` work from the run-dir files.
`
