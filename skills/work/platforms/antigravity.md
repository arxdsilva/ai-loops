# Platform: Google Antigravity (tier A: resumable custom subagents)

Read with `skills/work/SKILL.md`. Facts: `specs/PLATFORMS.md` §2-4. Covers both the Antigravity IDE app and CLI (`agy`).

## Spawn
- Custom subagent files (`agents/<role>.md` in the native plugin, `.agents/agents/<role>.md` in project scope, `~/.gemini/config/agents/<role>.md` in user scope) set `subagent: true`, `mainAgent: false`, `model: inherit` (unless built with `loops port build --models`), `commandExecutionPolicy: "auto"`, and explicit role-tailored `tools:` allowlists (`hardware-researcher`: `[run_command, send_message]`; `web-researcher`: `[view_file, write_to_file, run_command, grep_search, find_by_name, list_dir, send_message, search_web]`; other read-only roles: same list excluding `replace_file_content` and `search_web`; implementers: full list including `replace_file_content`). Read-only roles enforce `write_to_file` run-dir scoping via `PreToolUse` `write-guard`, while `run_command` remains instruction-enforced via the body note.
- Spawn with `invoke_subagent` targeting the role (`<role>`), starting the prompt with the prompt header (Run dir, Specs, Templates, Loops, Role, Agent id, Wave, Language, Charter) and the agent's plan block (or input file list).
- If subagents are unavailable, fall back to Tier D solo mode (`specs/PLATFORMS.md` §4).

## Parallel wave
Issue all `invoke_subagent` calls of a wave in the same turn *(unverified: concurrent execution)*; otherwise run them sequentially, producers first.

## Wait
Wait for each subagent's result (`manage_subagents` / tool response). Idle subagents remain resumable with full prior context.

## Loop-backs (resume)
Resume the **same** idle (or running) subagent via `send_message` with the QA or review file path; idle subagents auto-wake with full context. If the subagent was killed or is unavailable, spawn a new `invoke_subagent` with its previous report (`reports/<id>.md`), the QA or review file, and a ≤10-line history.

## Labelling lanes
Antigravity has no `SubagentStart`/`SubagentStop` hook events, so lanes come ONLY from labels. After each spawn: `loops run label --agent <id, or the Agent id> --name <Agent id> --role <role> --wave <n>`, and record `agent_start`/`agent_stop` with `loops run event`.

## Ask the user
Ask in chat and wait (Approve / Request changes / Stop).

## Stop agents
Cancel or stop the subagent via `manage_subagents` or the Antigravity UI once verified and no longer needed; re-spawn with a tighter block if stuck.

## Hooks and guards
- Native (`hooks.json` in native plugin; `.agents/hooks.json` in project scope, `~/.gemini/config/hooks.json` in user scope; commands pass `--platform antigravity`): `SessionStart`, `PreToolUse` (Antigravity tool regex matchers `view_file`, `run_command`, `replace_file_content`, `multi_replace_file_content`, `write_to_file`, `invoke_subagent`), `PostToolUse`, `Stop`.
- Exit-0 stdout JSON protocol (non-zero exits fail open):
  - `PreToolUse` deny (`read-guard`, `test-guard`, `write-guard`, conflicting `go-env`): `{"decision":"deny","reason":...}` (`model-guard` is a no-op). `write-guard` matches `replace_file_content|write_to_file|multi_replace_file_content` and restricts read-only roles to `<run-dir>/**` (and `.loops/memory/**` for `reflector`).
  - `PreToolUse` rewrite (`toolchain-env`): `{"overwrite":{...}}` (shallow merge into `toolCall.args`).
  - `Stop` gate: `{"decision":"continue","reason":...}`.
  - `SessionStart` and `post-edit` failure: `{"injectSteps":[{"userMessage":...}]}` (`PostToolUse` `injectSteps` *(unverified)*).
- Not available: `PostToolUseFailure`, `SubagentStart`/`SubagentStop`. For read-only roles, `run_command` remains instruction-enforced.

## Live view
`loops watch` in a side terminal; `loops report`.

## Install and verify
Native plugin (from a repo clone):
```
agy plugin validate ./platforms/antigravity
agy plugin install ./platforms/antigravity
```
Or workspace / user installer:
```
npx ai-loops --target antigravity
# or: loops install --target antigravity [--scope project|user]
```
Project scope writes `.agents/{skills,agents,rules,hooks.json}` (hook commands pinned to `.loops/plugin/bin/loops`) and `.loops/plugin/` (`bin/loops`, `specs/`, `templates/`, `ROLES.md`, `skills/`, `agents/`, `plugin.json`). User scope writes `~/.gemini/config/{skills,agents,rules,hooks.json}` (hook commands pinned to `~/.local/share/loops/plugin/bin/loops`) and `~/.local/share/loops/plugin/`. Rules live in `rules/AGENTS.md` (Antigravity also reads `AGENTS.md` / `GEMINI.md`).

After installing or updating, run `/skills reload` in the Antigravity app or `agy` CLI. In `agy`, each skill is also exposed as a slash command (`/work`, `/init`, `/status`, `/report`). Verify: `loops version`; `loops path agents`.

## Hardware and budgets
The `hardware-researcher` runs as a normal `invoke_subagent` (`loops hw --write`). Token budgets (`--budget`) are advisory on this platform; agent-count and role limits still apply.

## Update
Not automatic: `<plugin root>/bin/loops update --apply --platform antigravity` prints the manual route (PLATFORMS §9). Native plugin: pull and re-run `agy plugin install ./platforms/antigravity`. Installer: `npx ai-loops --target antigravity --force` (or `install.sh --target antigravity --force`; `loops update --platform antigravity` prints the route). Run `/skills reload` after updating.

## Model fallback
Roles ask for their tier, and the planner and plan-reviewer ask for the strongest reasoning model. Generated agents default to `model: inherit` unless built with `loops port build --models`. If a mapped model is rejected, fall back to the session model and record a `model-fallback` note (WORKFLOW §3a).

## Run-scoped toolchain env
`toolchain-env` rewrites Go and other detected pack commands automatically via `{"overwrite":{"CommandLine":...}}` (or `"command"` fallback) so they use the run's own caches, which `loops run end` deletes. Conflicting `GOCACHE` values are denied (`{"decision":"deny","reason":...}`): use `loops go …` or `loops exec -- <cmd>`.
