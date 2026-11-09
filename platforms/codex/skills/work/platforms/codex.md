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
Needs Go. Verify: `loops version`; `ls .codex/agents`.
