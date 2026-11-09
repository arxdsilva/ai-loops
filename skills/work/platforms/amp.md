# Platform: Amp (tier C: generic subagents)

Read with `skills/work/SKILL.md`. Facts: `specs/PLATFORMS.md` §2-4.

## Spawn
- There are no role subagents. Use the built-in `Task` tool with a prompt that STARTS with the role file's body: `$(loops path agents)/<role>.md` (body only, no frontmatter), followed by the prompt header (Run dir, Specs, Templates, Role, Agent id, Wave) and the plan block.
- No per-role model or tool restriction.

## Parallel wave
Issue all `Task` calls of a wave in the same turn.

## Wait
Wait for each Task's result.

## Loop-backs
Amp may allow messaging a subagent (adapted) *(unverified)*; the safe path is to spawn a NEW Task with the role body, its previous report, the QA or review file, and a ≤10-line history.

## Labelling lanes
No hooks: lanes come only from labels. After each Task: `loops run label --agent <id, or the Agent id> --name <Agent id> --role <role> --wave <n>`; record `agent_start`/`agent_stop` with `loops run event`.

## Ask the user
Ask in chat and wait (Approve / Request changes / Stop).

## Stop agents
Cancel the Task from the Amp UI; re-spawn with a tighter block.

## Hooks and guards
None in v1 (no deny, no trace, no gofmt, no stop gate). The lead runs `loops gate` itself before ending, and `loops arch` after wave 0.

## Live view
`loops watch` in a side terminal; `loops report`. Events come from your `loops run …` calls only.

## Install and verify
```
loops install --target amp
```
Writes `.agents/skills/`; rules come from AGENTS.md. Needs Go. Verify: `loops version`; `loops path agents`.

## Hardware and budgets
The `hardware-researcher` runs as a normal subagent (`loops hw --write`). Token budgets (`--budget`) are advisory on this platform; agent-count and role limits still apply.
