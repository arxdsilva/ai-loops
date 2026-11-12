# Platform: Gemini CLI (tier B: subagents, no resume)

Read with `skills/work/SKILL.md`. Facts: `specs/PLATFORMS.md` §2-4. Commands are `/loops:work` etc.

## Spawn
- Delegate with `@<role>` (for example `@domain-engineer`) or the subagent tool, with the prompt header (Run dir, Specs, Templates, Role, Agent id, Wave) plus the plan block.
- Agents are `kind: local` with a `tools` list (edit tools omitted for read-only roles).

## Parallel wave
Delegate the wave's agents in the same turn where Gemini runs subagents concurrently *(unverified)*; otherwise sequentially, producers first.

## Wait
Wait for each subagent's final message.

## Loop-backs (re-spawn)
No resume. Spawn a NEW subagent of the same role with its previous report, the QA or review file, and a ≤10-line history.

## Labelling lanes
Gemini has no SubagentStart/Stop events, so lanes come ONLY from labels: after every spawn run `loops run label --agent <id, or a stable name such as the Agent id> --name <Agent id> --role <role> --wave <n>`, and record `agent_start`/`agent_stop` with `loops run event` yourself.

## Ask the user
Ask in chat and wait (Approve / Request changes / Stop).

## Stop agents
Interrupt from the CLI; re-spawn with a tighter block.

## Hooks and guards
- Native: BeforeTool deny, AfterTool trace and gofmt, AfterAgent stop gate, SessionStart context.
- Not available: SubagentStart/Stop, resume, status line.

## Live view
`loops watch` in a side terminal; `loops report`.

## Install and verify
```
git clone https://github.com/arxdsilva/ai-loops
gemini extensions install ./ai-loops/platforms/gemini   # no subdirectory install; `link` for development
```
*(unverified: subdirectory installs; otherwise clone and `gemini extensions install ./ai-loops/platforms/gemini`)*. Needs `loops` on PATH (prebuilt; `install.sh`). Verify: `loops version`.

## Hardware and budgets
The `hardware-researcher` runs as a normal subagent (`loops hw --write`). Token budgets (`--budget`) are advisory on this platform; agent-count and role limits still apply.

## Update
Not automatic: `<plugin root>/bin/loops update --apply --platform gemini` prints this manual route (PLATFORMS §9).
`git -C ai-loops pull && gemini extensions update loops` (or run `loops update --platform gemini`).

## Model fallback
Roles ask for their tier, and the planner and plan-reviewer ask for the strongest reasoning model. If the platform rejects a model, re-spawn the role with the best model available here, or the session model. Record a `model-fallback` note (WORKFLOW §3a).

## Run-scoped toolchain env
Go commands on this platform are rewritten automatically by the BeforeTool hook. Either way they use the run's own Go cache, which `loops run end` deletes.

Other language packs: the toolchain-env hook prefixes their build and test commands with the run-scoped env (`loops lang show <id>` → `cache_env`).
