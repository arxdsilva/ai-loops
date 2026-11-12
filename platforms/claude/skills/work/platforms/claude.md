# Platform: Claude Code (tier A: resumable subagents)

Read with `skills/work/SKILL.md`. Facts: `specs/PLATFORMS.md` §2-4.

## Spawn
- `Agent` tool with `subagent_type: "loops:<role>"` (for example `loops:domain-engineer`) and `run_in_background: true`.
- Pass `model` (`haiku|sonnet|opus`) only to escalate, under `--models=cheap`, or for a fallback.
- **Fallback** (WORKFLOW §3a). If a spawn fails because the role's model isn't available (an error naming the model, entitlement or capacity), re-spawn the same role with the next-best `model`. Reasoning roles go opus → sonnet → `inherit`; others go one tier down. Record a `model-fallback` note, and tell the user when the planner ran below opus.
- The prompt is the header (Run dir, Specs, Templates, Role, Agent id, Wave) plus the agent's plan block.

## Parallel wave
Put every `Agent` call of a wave in ONE message. Research (3 agents) is one message too.

## Wait
Completion notifications arrive on their own. Do not poll. Do not enter plan mode while agents run.

## Loop-backs (resume)
`SendMessage` to the same agent (use the `agentId` from the Agent tool result) with the QA or review file path. The agent keeps its context. If it was stopped, spawn anew with its report and a ≤10-line history.

## Labelling lanes
Right after each spawn, in the same turn as the spawn result:
`loops run label --agent <agentId from the Agent tool result> --name <Agent id> --role <role> --wave <n>`

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
`claude plugin marketplace update ai-loops && claude plugin update loops@ai-loops` (or run `loops update claude`).

## Run-scoped toolchain env
Go commands on this platform are rewritten automatically (the hook rewrites the command; a lone `go build|test|vet|list` is auto-approved). Either way they use the run's own Go cache, which `loops run end` deletes.

Other language packs: the toolchain-env hook prefixes their build and test commands with the run-scoped env (`loops lang show <id>` → `cache_env`).
