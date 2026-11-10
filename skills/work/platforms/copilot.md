# Platform: GitHub Copilot (tier B: subagents, no resume)

Read with `skills/work/SKILL.md`. Facts: `specs/PLATFORMS.md` §2-4. Copilot reads the Claude layout (`agents/`, `skills/`, `hooks/hooks.json`) directly.

## Spawn
- Invoke the custom agent by its name (`<role>`, from `agents/<role>.md`) through Copilot's subagent/agent delegation, with the prompt header (Run dir, Specs, Templates, Role, Agent id, Wave) plus the plan block.
- Per-role model comes from the agent frontmatter. Do not pass a model override.

## Parallel wave
Delegate all agents of a wave in the same turn where Copilot allows parallel delegation *(unverified)*; otherwise run them one after another in the producers-first order.

## Wait
Wait for each subagent's final message before the next phase step.

## Loop-backs (re-spawn)
Resume is not available. Spawn a NEW agent of the same role with: its previous report (`reports/<id>.md`), the QA or review file, and a ≤10-line history summary. The run-dir files carry the context.

## Labelling lanes
SubagentStart/Stop hooks give lanes automatically. Still label: `loops run label --agent <agent id from the hook/tool result, if shown> --name <Agent id> --role <role> --wave <n>`. If no id is visible, skip; lanes then come from hook events only.

## Ask the user
Ask in chat and wait for the reply (Approve / Request changes / Stop).

## Stop agents
Let subagents finish; cancel from the Copilot UI if one stalls, then re-spawn with a tighter block.

## Hooks and guards
- Native (PreToolUse deny, PostToolUse trace and gofmt, SubagentStart/Stop, Stop gate, SessionStart): same `hooks/hooks.json` as Claude, `${CLAUDE_PLUGIN_ROOT}` aliased.
- Not available: resume, status line, live pane.

## Live view
`loops watch` in a side terminal; `loops report` for HTML.

## Install and verify
```
copilot plugin install arxdsilva/ai-loops
```
or VS Code: *Chat: Install Plugin from Source*. Needs `loops` on PATH (prebuilt; `install.sh`). Verify: `loops version`, then run the work skill.

## Hardware and budgets
The `hardware-researcher` runs as a normal subagent (`loops hw --write`). Token budgets (`--budget`) are measured only if the SubagentStop payload carries `agent_transcript_path` *(unverified)*; otherwise they are advisory. Agent, role and concurrency limits always apply.

## Update
`copilot plugin marketplace update ai-loops && copilot plugin update loops` (or run `loops update copilot`).

## Model fallback
Roles ask for their tier, and the planner and plan-reviewer ask for the strongest reasoning model. If the platform rejects a model, re-spawn the role with the best model available here, or the session model. Record a `model-fallback` note (WORKFLOW §3a).

## Run-scoped toolchain env
Go commands on this platform are not rewritten (the hook denies), so always run Go through `loops go …`. Either way they use the run's own Go cache, which `loops run end` deletes.

Other language packs: their commands are not rewritten here, so run their build and test commands through `loops exec -- <cmd>` to use the run-scoped env.
