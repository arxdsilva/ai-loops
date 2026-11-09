// loops shim for OpenCode: pipes tool.execute.before/after into
// `loops hook <name> --platform opencode` (JSON on stdin).
//
// ASSUMED API (docs read 2026-11-09, https://opencode.ai/docs/build/plugins):
// V2 `Plugin.define` with `ctx.tool.hook("execute.before" | "execute.after")`;
// events expose `tool` and `input`. The docs do not say that throwing blocks a
// call; this shim assumes it does. Older V1 OpenCode releases expect a different
// export shape; adjust this file if hooks do not fire. Hooks fail open.
import { Plugin } from "@opencode/plugin"
import { spawnSync } from "node:child_process"

const LOOPS = process.env.LOOPS_BIN ?? "loops"
const BEFORE: string[] = ["read-guard", "model-guard"]
const AFTER: string[] = ["go-post-edit", "trace"]

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
