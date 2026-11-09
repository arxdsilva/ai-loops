# Platform: OpenCode (tier B: subagents, no resume)

Read with `skills/work/SKILL.md`. Facts: `specs/PLATFORMS.md` §2-4.

## Spawn
- Use the `subagent` tool (or `@`-mention) with agent `loops/<role>` (from `.opencode/agents/loops/<role>.md`) and the prompt header (Run dir, Specs, Templates, Role, Agent id, Wave) plus the plan block.
- Read-only roles deny edit via `permissions`.

## Parallel wave
Issue the wave's subagent calls in the same turn *(unverified: concurrency)*; otherwise sequentially, producers first.

## Wait
Wait for each subagent's result.

## Loop-backs (re-spawn)
No resume. Spawn a NEW subagent of the same role with its previous report, the QA or review file, and a ≤10-line history.

## Labelling lanes
No subagent events: lanes come only from labels. After each spawn: `loops run label --agent <id, or the Agent id> --name <Agent id> --role <role> --wave <n>`.

## Ask the user
Ask in chat and wait (Approve / Request changes / Stop).

## Stop agents
Interrupt from the OpenCode UI; re-spawn with a tighter block.

## Hooks and guards
- Adapted: `.opencode/plugins/loops.ts` shim pipes `tool.execute.before/after` into `loops hook … --platform opencode` (pre-tool deny, post-edit trace and gofmt) *(unverified API version)*; session context adapted.
- Not available: SubagentStart/Stop, Stop gate (the lead must run `loops gate` itself before ending), resume, status line.

## Live view
`loops watch` in a side terminal; `loops report`.

## Install and verify
```
loops install --target opencode
```
Writes `.opencode/agents/loops/`, `.opencode/commands/loops/`, `.opencode/plugins/loops.ts`, `.agents/skills/`. Needs Go. Verify: `loops version`; AGENTS.md is read natively.

## Hardware and budgets
The `hardware-researcher` runs as a normal subagent (`loops hw --write`). Token budgets (`--budget`) are advisory on this platform; agent-count and role limits still apply.
