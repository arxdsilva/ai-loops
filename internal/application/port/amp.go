package port

import (
	"io/fs"

	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
)

const ampReadme = `# loops for Amp

Tier C (generic subagents). Installed with ` + "`loops install --target amp`" + `.

- Skills land in .agents/skills/; rules come from AGENTS.md.
- Use the work skill as the /work equivalent: ask Amp to "use the work skill: <task>"
  (init, status and report likewise).
- Roles: spawn a generic Task subagent whose prompt starts with the body of
  the role file (` + "`loops path agents`" + ` gives the directory). Per-role models and tool
  restrictions are not enforced.
- Loop-backs: Amp can message the same subagent where supported; otherwise spawn a
  new one with its previous report and the QA or review file.
- No hooks in v1: guards, tracing and the stop gate do not run. ` + "`loops watch`" + `
  and ` + "`loops report`" + ` still work from the run-dir files.
`

// buildAmp emits platforms/amp/ (specs/PLATFORMS.md §3.6): skills laid out
// for placement under .agents/skills/.
func buildAmp(p plugin.Plugin, src fs.FS, o Options) ([]plugin.File, []string, error) {
	base, err := baseFiles(p, src, plugin.Amp, o)
	if err != nil {
		return nil, nil, err
	}
	files := append(append(toAgentsSkills(base), roleFiles(p)...), plugin.File{Path: "README.md", Data: []byte(ampReadme)})
	sortByPath(files)
	warns := []string{
		"amp: role subagents are not emitted; roles run as generic Task subagents with the role file body as prompt, so per-role model and tool restrictions are not enforced",
		"amp: no hooks; read-guard, model-guard, trace and stop-gate do not run",
		"amp: slash commands are the skills themselves (no command files)",
	}
	return files, warns, nil
}
