# Platform: Cursor (tier A: resumable subagents)

Read with `skills/work/SKILL.md`. Facts: `specs/PLATFORMS.md` §2-4.

## Spawn
- Use the Task tool / subagents by name (`<role>`, from the package's `agents/<role>.md`) with the prompt header (Run dir, Specs, Templates, Role, Agent id, Wave) plus the plan block.
- Wave roles (researchers, engineers, test-engineer, docs-writer, qa-verifier) are generated with `is_background: true`. The planner, plan-reviewer, architect, product-reviewer and reflector run in the foreground.
- `readonly` roles cannot edit; model is `inherit` unless mapped.

## Parallel wave
Issue all Task calls of a wave in the same turn.

## Wait
Wait for each subagent's result; do not proceed on partial reports.

## Loop-backs (resume)
Resume the **same** subagent by its agent id, with the QA or review file path. If that id is gone, fall back to re-spawning (tier B): give a new agent its previous report, the QA or review file and a ≤10-line history.

## Labelling lanes
subagentStart/subagentStop hooks give lanes. Label with `loops run label --agent <subagent id> --name <Agent id> --role <role> --wave <n>`. Keep the id: you need it to resume the agent.

## Ask the user
Ask in chat and wait (Approve / Request changes / Stop).

## Stop agents
Cancel from the Cursor UI; re-spawn with a tighter block.

## Hooks and guards
- Native: preToolUse deny, postToolUse trace and gofmt, subagentStart/Stop, stop gate (`followup_message`, `loop_limit` 5).
- Native: session context (`additional_context`), plus the always-on rule `rules/loops.mdc`.
- Not available: status line.

## Live view
`loops watch` in a side terminal; `loops report`.

## Install and verify
Cursor marketplace / team marketplace import of `arxdsilva/ai-loops` (package `platforms/cursor`). Local dev: copy it to `~/.cursor/plugins/local/loops`. Needs `loops` on PATH (prebuilt; `install.sh`). Verify: `loops version`.

## Hardware and budgets
The `hardware-researcher` runs as a normal subagent (`loops hw --write`). Token budgets (`--budget`) are advisory on this platform; agent-count and role limits still apply.

## Update
Not automatic: `<plugin root>/bin/loops update --apply --platform cursor` prints this manual route (PLATFORMS §9).
Dashboard > Plugins & MCPs > marketplace > Refresh, then start a new agent (or run `loops update --platform cursor`).

## Model fallback
Roles ask for their tier, and the planner and plan-reviewer ask for the strongest reasoning model. If the platform rejects a model, re-spawn the role with the best model available here, or the session model. Record a `model-fallback` note (WORKFLOW §3a).

## Run-scoped toolchain env
Go commands on this platform are rewritten automatically when the command is a lone `go build|test|vet|list`; anything else is denied, so use `loops go …`. Either way they use the run's own Go cache, which `loops run end` deletes.

Other language packs: their commands are not rewritten here, so run their build and test commands through `loops exec -- <cmd>` to use the run-scoped env.
