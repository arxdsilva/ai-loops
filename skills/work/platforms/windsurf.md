# Platform: Windsurf / Devin Local (tier D: solo mode)

Read with `skills/work/SKILL.md`. Facts: `specs/PLATFORMS.md` §2-4.

## Spawn
There are no subagents. You (the lead) play each role in turn, in phase order: researchers, planner, plan-reviewer, architect, implementers by wave, qa-verifier per lane, product-reviewer, reflector. Read the role's file first (`loops path agents` → `<role>.md`, body only) and follow it.

## Parallel wave
Not possible. Run the wave's lanes one after another, producers first.

## Wait
Nothing to wait for; finish each role's output file before the next.

## Loop-backs
Re-enter the author role with the QA or review file and the same limits (WORKFLOW §4). Before each QA or review pass, re-read ONLY the files that role is given (report, owned diff, contract, ACs) to approximate a fresh context; do not rely on your memory of writing the code.

## Labelling lanes
For each role turn: `loops run label --agent solo-<n> --name <Agent id> --role <role> --wave <n>`, then `loops run event --kind agent_start …` before and `agent_stop` after (use the exact event syntax from `loops run event --help`).

## Ask the user
Ask in chat and wait (Approve / Request changes / Stop).

## Stop agents
Nothing to stop.

## Hooks and guards
- Native (`.devin/hooks.v1.json`, Claude event names): PreToolUse deny, PostToolUse trace and gofmt, SessionStart context.
- Adapted: Stop gate *(unverified)*. Not available: SubagentStart/Stop, resume, status line.

## Live view
`loops watch` in a side terminal; `loops report`.

## Install and verify
```
loops install --target windsurf
```
Writes `.agents/skills/`, `.devin/hooks.v1.json`. Needs Go. Verify: `loops version`; AGENTS.md is read natively.

## Hardware and budgets
Tier D: no subagents, so the lead runs `loops hw --write` itself. Token budgets (`--budget`) are advisory on this platform; agent-count and role limits still apply.

## Update
`go install github.com/arxdsilva/ai-loops/cmd/loops@latest && loops install --target windsurf --force` (or run `loops update windsurf`).
