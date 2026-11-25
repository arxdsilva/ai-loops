# Platform: Claude Code (tier A: resumable subagents)

Read with `skills/work/SKILL.md`. Facts: `specs/PLATFORMS.md` §2-4.

## Spawn
- `Agent` tool with `subagent_type: "loops:<role>"` (for example `loops:domain-engineer`) and `run_in_background: true`.
- Pass `model` (`haiku|sonnet|opus`) only to escalate, under `--models=cheap`, or for a fallback. A model above the role's tier needs a `Model reason:` prompt line, or `model-guard` denies the spawn.
- **Fallback** (WORKFLOW §3a). If a spawn fails because the role's model isn't available (an error naming the model, entitlement or capacity), re-spawn the same role with the next-best `model`. Reasoning roles go opus → sonnet → `inherit`; others go one tier down. Record a `model-fallback` note, and tell the user when the planner ran below opus.
- The prompt is the header (Run dir, Specs, Templates, Role, Agent id, Wave) plus the agent's plan block.
- Run the full gate of the QA tail as a background Bash call (`run_in_background: true`); its completion arrives like an agent's.

## Parallel wave
Put every `Agent` call of a wave in ONE message. Research (3 agents) is one message too.

## Wait
Completion notifications arrive on their own. Do not poll. Do not enter plan mode while agents run.

## Context
Every lead turn re-reads the whole session. Start each run in a fresh session; when the hook says the session was already large at `loops run start`, tell the user once that `/clear` and "continue the loops run" resumes it from the brief.

## Loop-backs (resume)
`SendMessage` to the same agent (use the `agentId` from the Agent tool result) with the QA or review file path. The agent keeps its context. If it was stopped, spawn anew with its report and a ≤10-line history.

## Labelling lanes
Automatic: the `trace` hook records the label from the `Agent` call (the prompt's `Agent id:`, `Role:` and `Wave:` lines, the result's `agentId` and resolved model). Do not run `loops run label` yourself; keep the header lines exact. If the brief shows a running lane with no name, label that one by hand.

## Ask the user
`AskUserQuestion` (Approve / Request changes / Stop; Push branch?; how to proceed after limits).

## Stop agents
`TaskStop` on the agent id once its lane is VERIFIED and it will not be resumed, and at run end.

## Hooks and guards (all native)
- read-guard and model-guard (PreToolUse deny), post-edit (the pack's formatter and linter, e.g. gofmt + go vet), trace and trace-agent (SubagentStart/Stop lanes), stop-gate (blocks finishing with changed source files of any pack; `LOOPS_GATE=off` is the user's escape hatch), session-start context.
- Role tool restrictions come from agent frontmatter.

## Live view
- Status line (`/loops:status` installs it) and the VS Code/terminal live pane.
- `loops watch` in a side terminal and `loops report` (HTML) also work.

## Install and verify
```
/plugin marketplace add arxdsilva/ai-loops
/plugin install loops@ai-loops
```
Verify: `loops version` resolves (the root shim builds from the checkout; needs Go), then `/loops:init`.

## Hardware and budgets
The `hardware-researcher` runs as a normal subagent (`loops hw --write`). Token budgets (`--budget`) are measured from agent transcripts on this platform.

## Update
`claude plugin marketplace update ai-loops && claude plugin update loops@ai-loops` (or run `loops update --platform claude`).
Applied automatically at work start: `<plugin root>/bin/loops update --apply --platform claude` (PLATFORMS §9). Reload with `/reload-plugins` or a new session.

## Run-scoped toolchain env
Go commands on this platform are rewritten automatically (the hook rewrites the command; a lone `go build|test|vet|list` is auto-approved). Either way they use the run's Go environment: a per-run GOTMPDIR, which `loops run end` deletes, and the build cache every run shares (WORKFLOW §1). A `go test` without `-race` is rewritten to add it rather than denied.

Other language packs: the toolchain-env hook prefixes their build and test commands with the run-scoped env (`loops lang show <id>` → `cache_env`).
