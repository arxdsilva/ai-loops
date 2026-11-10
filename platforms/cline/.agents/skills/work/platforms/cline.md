# Platform: Cline (tier D: solo mode)

Read with `skills/work/SKILL.md`. Facts: `specs/PLATFORMS.md` §2-4. Commands are Cline workflows.

## Spawn
There are no subagents. You (the lead) play each role in turn, in phase order. Read the role's file first (`loops path agents` → `<role>.md`, body only) and follow it.

## Parallel wave
Not possible. Run lanes one after another, producers first.

## Wait
Nothing to wait for.

## Loop-backs
Re-enter the author role with the QA or review file and the same limits (WORKFLOW §4). Before each QA or review pass, re-read ONLY that role's input files to approximate a fresh context.

## Labelling lanes
For each role turn: `loops run label --agent solo-<n> --name <Agent id> --role <role> --wave <n>`, with `agent_start`/`agent_stop` via `loops run event`.

## Ask the user
Ask in chat and wait (Approve / Request changes / Stop).

## Stop agents
Nothing to stop.

## Hooks and guards
None in v1 (no deny, trace, gofmt, stop gate, session context). Run `loops gate` yourself before ending and `loops arch` after wave 0; AGENTS.md carries the rules.

## Live view
`loops watch` in a side terminal; `loops report`. Events come from your `loops run …` calls only.

## Install and verify
```
loops install --target cline
```
Writes `.agents/skills/` and `.cline/skills/`. Needs `loops` on PATH (prebuilt; `install.sh`). Verify: `loops version`.

## Hardware and budgets
Tier D: no subagents, so the lead runs `loops hw --write` itself. Token budgets (`--budget`) are advisory on this platform; agent-count and role limits still apply.

## Update
`go install github.com/arxdsilva/ai-loops/cmd/loops@latest && loops install --target cline --force` (or run `loops update cline`).

## Model fallback
Roles ask for their tier, and the planner and plan-reviewer ask for the strongest reasoning model. If the platform rejects a model, re-spawn the role with the best model available here, or the session model. Record a `model-fallback` note (WORKFLOW §3a).

## Run-scoped toolchain env
Go commands on this platform are not enforced (no hooks), so always run Go through `loops go …`. Either way they use the run's own Go cache, which `loops run end` deletes.

Other language packs: their commands are not rewritten here, so run their build and test commands through `loops exec -- <cmd>` to use the run-scoped env.
