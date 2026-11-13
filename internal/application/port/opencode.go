package port

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
)

const opencodeShim = `// loops shim for OpenCode: pipes tool.execute.before/after into
// ` + "`loops hook <name> --platform opencode`" + ` (JSON on stdin).
//
// ASSUMED API (docs read 2026-11-09, https://opencode.ai/docs/build/plugins):
// V2 ` + "`Plugin.define`" + ` with ` + "`ctx.tool.hook(\"execute.before\" | \"execute.after\")`" + `;
// events expose ` + "`tool`" + ` and ` + "`input`" + `. The docs do not say that throwing blocks a
// call; this shim assumes it does. Older V1 OpenCode releases expect a different
// export shape; adjust this file if hooks do not fire. Hooks fail open.
import { Plugin } from "@opencode/plugin"
import { spawnSync } from "node:child_process"

const LOOPS = process.env.LOOPS_BIN ?? "loops"
const BEFORE: string[] = @BEFORE@
const AFTER: string[] = @AFTER@

function run(name: string, payload: unknown) {
  return spawnSync(LOOPS, ["hook", name, "--platform", "opencode"], {
    input: JSON.stringify(payload),
    encoding: "utf8",
    cwd: process.cwd(),
  })
}

export default Plugin.define({
  id: "loops",
  async setup(ctx) {
    const before = await ctx.tool.hook("execute.before", (e: any) => {
      for (const name of BEFORE) {
        const r = run(name, { hook_event_name: "PreToolUse", tool_name: e.tool, tool_input: e.input, cwd: process.cwd() })
        if (r.status === 2) throw new Error((r.stderr || "blocked by loops").trim())
      }
    })
    const after = await ctx.tool.hook("execute.after", (e: any) => {
      for (const name of AFTER) {
        run(name, { hook_event_name: "PostToolUse", tool_name: e.tool, tool_input: e.input, cwd: process.cwd() })
      }
    })
    return async () => {
      await before.dispose()
      await after.dispose()
    }
  },
})
`

const opencodeReadme = `# loops for OpenCode

Tier B (subagents, no resume). Installed with ` + "`loops install --target opencode`" + `;
files are laid out as they land in a project:

- .opencode/agents/loops/<role>.md: subagents (read-only roles deny edit).
- .opencode/commands/loops/<cmd>.md: /loops/work, /loops/init, ... ($ARGUMENTS).
- .opencode/plugins/loops.ts: hook shim (needs ` + "`loops`" + ` on PATH, or LOOPS_BIN).
- .agents/skills/: the loops skills. OpenCode reads AGENTS.md natively.

Only tool.execute.before/after hooks are shimmed: read-guard and model-guard can
deny, post-edit and trace run after tools. There is no stop gate, session
context or subagent lane tracking.

Roles inherit the session model unless built with ` + "`loops port build --models`" + `.
`

// buildOpenCode emits platforms/opencode/ (specs/PLATFORMS.md §3.5): the
// files laid out as `loops install --target opencode` places them in a
// project.
func buildOpenCode(p plugin.Plugin, src fs.FS, o Options) ([]plugin.File, []string, error) {
	base, err := baseFiles(p, src, plugin.OpenCode, o)
	if err != nil {
		return nil, nil, err
	}
	files := append(toAgentsSkills(base), roleFiles(p)...)
	var warns []string

	for _, a := range p.Agents {
		fm := [][2]string{
			{"description", quoteStr(a.Description)},
			{"mode", "subagent"},
		}
		if m := model(o, a.Tier); m != "" {
			fm = append(fm, [2]string{"model", quoteStr(m)})
		}
		extra := ""
		if a.ReadOnly {
			extra = "permissions:\n  - action: edit\n    resource: \"*\"\n    effect: deny\n  - action: edit\n    resource: \".loops/**\"\n    effect: allow\n"
		} else if len(a.Tools) > 0 {
			warns = append(warns, fmt.Sprintf("opencode: role %s Claude tool allowlist not mapped", a.Name))
		}
		files = append(files, plugin.File{
			Path: ".opencode/agents/loops/" + a.Name + ".md",
			Data: []byte(fmBlock(fm, extra) + roleBody(a)),
		})
	}

	for _, s := range commandSkills(p) {
		fm := [][2]string{{"description", quoteStr(s.Description)}}
		body := "Use the " + s.Name + " skill and follow it. $ARGUMENTS\n"
		files = append(files, plugin.File{
			Path: ".opencode/commands/loops/" + s.Name + ".md",
			Data: []byte(fmBlock(fm, "") + body),
		})
	}

	before, after := hookNames(p, plugin.EventPreToolUse), hookNames(p, plugin.EventPostToolUse, plugin.EventPostToolUseFailure)
	files = append(files, plugin.File{
		Path: ".opencode/plugins/loops.ts",
		Data: []byte(strings.NewReplacer("@BEFORE@", quoteList(before), "@AFTER@", quoteList(after)).Replace(opencodeShim)),
	})
	warns = append(warns,
		"opencode: hooks are a TypeScript shim over tool.execute.before/after only; SessionStart, Stop and subagent lanes are not delivered",
		"opencode: plugin API version assumed (V2 Plugin.define, ctx.tool.hook); denial by throwing is not confirmed by the docs")

	files = append(files, plugin.File{Path: "README.md", Data: []byte(opencodeReadme)})
	sortByPath(files)
	return files, warns, nil
}

// hookNames returns the distinct handler names bound to the events, in order.
func hookNames(p plugin.Plugin, evs ...plugin.Event) []string {
	var out []string
	seen := map[string]bool{}
	for _, h := range p.Hooks {
		for _, e := range evs {
			if h.Event == e && !seen[h.Name] {
				seen[h.Name] = true
				out = append(out, h.Name)
			}
		}
	}
	return out
}

func quoteList(ss []string) string {
	q := make([]string, len(ss))
	for i, s := range ss {
		q[i] = quoteStr(s)
	}
	return "[" + strings.Join(q, ", ") + "]"
}
