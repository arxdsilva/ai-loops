# Platform: Google Antigravity (tier C: generic subagents; solo fallback)

Read with `skills/work/SKILL.md`. Facts: `specs/PLATFORMS.md` §2-4. Covers both the Antigravity IDE app and CLI (`agy`).

## Spawn
- There are no role subagents. Spawn a generic subagent with `invoke_subagent` whose prompt STARTS with the role file's body (`$(loops path agents)/<role>.md`, body only, no frontmatter), followed by the prompt header (Run dir, Specs, Templates, Loops, Role, Agent id, Wave, Language, Charter) and the plan block.
- No per-role model or tool restriction is enforced by the platform.
- **Tier D solo fallback:** if subagents are unavailable, play each role yourself in phase order (read the role file first, write the same run-dir files, and re-read ONLY that role's input files before each QA or review pass).

## Parallel wave
Issue all `invoke_subagent` calls of a wave in the same turn *(unverified: concurrent execution)*; otherwise run them sequentially, producers first. In solo fallback, run lanes one after another, producers first.

## Wait
Wait for each subagent's result (`manage_subagents` / tool response). In solo fallback, finish each role's output file before the next.

## Loop-backs
While a subagent is still running, message it with `send_message` *(unverified)*. Once it has finished, spawn a NEW `invoke_subagent` with the role body, its previous report (`reports/<id>.md`), the QA or review file, and a ≤10-line history (or re-enter the author role in solo fallback).

## Labelling lanes
Antigravity has no `SubagentStart`/`SubagentStop` hook events, so lanes come ONLY from labels. After each spawn: `loops run label --agent <id, or the Agent id> --name <Agent id> --role <role> --wave <n>` (or `solo-<n>` in solo fallback), and record `agent_start`/`agent_stop` with `loops run event`.

## Ask the user
Ask in chat and wait (Approve / Request changes / Stop).

## Stop agents
Cancel or stop the subagent via `manage_subagents` or the Antigravity UI; re-spawn with a tighter block. (In solo fallback, nothing to stop.)

## Hooks and guards
- Native (`.agents/hooks.json` in project scope, `~/.gemini/config/hooks.json` in user scope; commands pass `--platform antigravity`): `SessionStart` context, `PreToolUse` deny (`read-guard`, `test-guard`, `toolchain-env`; `model-guard` is a no-op), `PostToolUse` trace and `post-edit` (`gofmt`/`go vet` for Go), `Stop` gate.
- Not available: `PostToolUseFailure`, `SubagentStart`/`SubagentStop`, per-role model/tool enforcement, hook input rewriting *(unverified)*.

## Live view
`loops watch` in a side terminal; `loops report`.

## Install and verify
```
npx ai-loops --target antigravity
# or: loops install --target antigravity [--scope project|user]
```
Project scope writes `.agents/skills/`, `.agents/hooks.json` (commands pinned to `.loops/plugin/bin/loops`), and `.loops/plugin/` (`agents/<role>.md`, `bin/loops`, `specs/`, `templates/`, `ROLES.md`). User scope writes `~/.gemini/config/skills/`, `~/.gemini/config/hooks.json` (commands pinned to `~/.local/share/loops/plugin/bin/loops`), and `~/.local/share/loops/plugin/`. Rules come from `AGENTS.md` (Antigravity also reads `GEMINI.md` and `.agents/rules/*.md`).

After installing or updating, run `/skills reload` in the Antigravity app or `agy` CLI. In `agy`, each skill is also exposed as a slash command (`/work`, `/init`, `/status`, `/report`). Verify: `loops version`; `loops path agents`.

## Hardware and budgets
The `hardware-researcher` runs as a normal `invoke_subagent` (`loops hw --write`), or by the lead in solo fallback. Token budgets (`--budget`) are advisory on this platform; agent-count and role limits still apply.

## Update
Not automatic: `<plugin root>/bin/loops update --apply --platform antigravity` prints this manual route (PLATFORMS §9).
`npx ai-loops --target antigravity --force` (or `install.sh --target antigravity --force`; `loops update --platform antigravity` prints the route). Run `/skills reload` after updating.

## Model fallback
Roles ask for their tier, and the planner and plan-reviewer ask for the strongest reasoning model. If the platform rejects a model or does not expose per-subagent model selection, run with the best model available here or the session model and record a `model-fallback` note (WORKFLOW §3a).

## Run-scoped toolchain env
Unscoped `go …` commands on this platform are denied by `toolchain-env` (`go-env`, because hook input rewriting is unverified), so always run Go through `loops go …` or `loops exec -- <cmd>`. Either way they use the run's own Go cache, which `loops run end` deletes.

Other language packs: their commands are not rewritten here, so run their build and test commands through `loops exec -- <cmd>` to use the run-scoped env.
