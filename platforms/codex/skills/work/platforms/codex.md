# Platform: Codex (tier B: subagents, no resume)

Read with `skills/work/SKILL.md`. Facts: `specs/PLATFORMS.md` §2-4. Commands are `$work`, `$init`, `$status`, `$report`.

## Spawn
- Say: "spawn a subagent named `<role>`" with the prompt header (Run dir, Specs, Templates, Role, Agent id, Wave) plus the plan block.
- The role agents are TOMLs in `.codex/agents/` (or `~/.codex/agents/`). A plugin cannot bundle subagents, so run `loops install --target codex` once.
- Model per role is set in the TOML only if mapped (`loops port build --models`).

## Parallel wave
Spawn the whole wave in one request ("spawn subagents W1-domain, W1-application, ...") where Codex runs them concurrently.

## Wait
Wait for the subagents' results before the next step.

## Loop-backs (re-spawn)
No resume. Spawn a NEW subagent of the same role with its previous report, the QA or review file, and a ≤10-line history.

## Labelling lanes
SubagentStart/Stop hooks create lanes. Label with `loops run label --agent <subagent id if shown> --name <Agent id> --role <role> --wave <n>`.

## Ask the user
Ask in chat and wait (Approve / Request changes / Stop).

## Stop agents
Let them finish or interrupt from the Codex UI; re-spawn with a tighter block.

## Hooks and guards
- Native: PreToolUse deny, PostToolUse trace and gofmt, SubagentStart/Stop, Stop gate, SessionStart. Matchers are `*`; the binary filters tool names (`shell`, `apply_patch` *(verify)*).
- Plugin hooks run only after the user trusts them (`/hooks`). Without trust there is no guard.
- Not available: per-role tool restriction, resume, status line.

## Live view
`loops watch` in a side terminal; `loops report`.

## Install and verify
```
codex plugin marketplace add arxdsilva/ai-loops
# install `loops` from the marketplace, then:
loops install --target codex
```
Needs `loops` on PATH (prebuilt; `install.sh`). Verify: `loops version`; `ls .codex/agents`.

## Hardware and budgets
The `hardware-researcher` runs as a normal subagent (`loops hw --write`). Token budgets (`--budget`) are measured only if the SubagentStop payload carries `agent_transcript_path` *(unverified)*; otherwise they are advisory. Agent, role and concurrency limits always apply.

## Update
`codex plugin marketplace upgrade && curl -fsSL https://raw.githubusercontent.com/arxdsilva/ai-loops/main/install.sh | sh -s -- --target codex --force`, then update the plugin in the desktop app (or run `loops update codex`).

## Model fallback
Roles ask for their tier, and the planner and plan-reviewer ask for the strongest reasoning model. If the platform rejects a model, re-spawn the role with the best model available here, or the session model. Record a `model-fallback` note (WORKFLOW §3a).

## Run-scoped toolchain env
Go commands on this platform are rewritten automatically when the command is a lone `go build|test|vet|list`; anything else is denied, so use `loops go …`. Either way they use the run's own Go cache, which `loops run end` deletes.

Other language packs: their commands are not rewritten here, so run their build and test commands through `loops exec -- <cmd>` to use the run-scoped env.
