# PLATFORMS: loops on every coding agent

This is the canonical spec for porting loops beyond Claude Code. The emitters (`internal/application/port`), the hook dialects (`internal/adapters/inbound/hook`), `loops install`, and the per-platform notes in `skills/work/platforms/` implement it. The facts come from official docs read on 2026-11-09 (Antigravity on 2026-11-21). Items marked *(unverified)* must be confirmed against the docs before relying on them.

## 1. Principles

- **One source of truth.** The canonical plugin is the repository root, in Claude Code layout:
  - `agents/`, `skills/`, `specs/`, `templates/`, `ROLES.md`, `hooks/hooks.json`, `.claude-plugin/plugin.json`.
  - `embed.go` embeds it into the `loops` binary.
  - Generated packages under `platforms/<name>/` are never edited by hand. `loops port build` writes them, and `loops port check` fails CI when they are stale.
- **Shared formats first.**
  - **Agent Skills** (`SKILL.md`) and **AGENTS.md** work on every platform.
  - Shell hooks with JSON on stdin work on Claude, Codex, Copilot, Cursor and Gemini.
  - Everything else is translated per platform.
- **Degrade, never fork.** A platform missing a feature runs the same workflow (`specs/WORKFLOW.md`) with a documented fallback (§4). Run-dir files are the shared state, so any agent can pick up from them.
- **One binary.** Every package calls `loops`. Generated packages ship a shim that downloads the prebuilt, checksum-verified release binary on first use (§6), falling back to `go install <module>/cmd/loops@<version>` only when no prebuilt binary exists. The root (Claude) shim builds from the checkout.

## 2. Capability matrix

N = native, A = adapted (lossy or emulated), X = unavailable (falls back per §4).

| Feature | Claude | Copilot | Codex | Cursor | Gemini | OpenCode | Amp | Windsurf | Cline | Antigravity |
|---|---|---|---|---|---|---|---|---|---|---|
| Plugin package and install from git | N | N (reads the Claude layout) | N | N (marketplace) | N | A (installer) | A (installer) | A (installer) | A (installer) | N (plugin.json / installer) |
| Skills (SKILL.md) | N | N | N | N | N | N | N | N | N | N |
| Role subagents with per-role model | N | N | A (TOML, installed separately) | N | N | N | X | X | X | N |
| Per-role tool restriction | N | N | A (instruction) | A (instruction) | N (`tools`) | N (`permissions`) | X | X | X | A (tools allowlist + write-guard; shell by instruction) |
| Resume or message the same agent | N | X | X | N (by agent id) | X | X | A | X | X | N (`send_message`) |
| Slash commands | N (skills) | N (skills) | A (`$skill`) | N | N (TOML) | N (md) | A (skills) | A (skills) | A (workflows) | A (skills; `/work` in `agy`) |
| PreToolUse deny | N | N | N | N | N (`BeforeTool`) | A (TS shim) | X | N | X | N (exit 0 JSON) |
| PostToolUse trace and post-edit | N | N | N | N | N (`AfterTool`) | A (TS shim) | X | N | X | N (`injectSteps` *(unverified)*) |
| SubagentStart/Stop lanes | N | N | N | N | X | X | X | X | X | X |
| Stop gate | N | N | N | N (`followup_message`) | N (`AfterAgent`) | X | X | N | X | N (exit 0 JSON) |
| SessionStart context | N | N | N | A *(unverified field)* | N | A | X | N | X | N (`injectSteps`) |
| AGENTS.md | N | N | N | N | A (GEMINI.md imports it) | N | N | N | N | N |
| Status line | N | X | X | X | X | X | X | X | X | X |
| Live pane | N (mod) | X | X | X | X | X | X | X | X | X |
| Universal live view | `loops watch` (terminal) and `loops report` (HTML) on every platform |

## 3. Packages

These are the files each emitter writes under `platforms/<name>/`. Every package also contains:
- `skills/`: every canonical skill, copied unchanged except that `skills/work/platforms/` keeps only `<name>.md`;
- `specs/`, `templates/` and `ROLES.md`, copied;
- `bin/loops`: the `go install` shim (§6).

Every path the package needs comes from its root variable, so no package hard-codes absolute paths.

### 3.1 Claude Code and Copilot: the root
- **Claude:** `/plugin marketplace add arxdsilva/ai-loops`, then `/plugin install loops@ai-loops`. The repository root is the plugin.
- **Claude plugin directory: `platforms/claude/`.** The same plugin (agents, skills, specs, templates, hooks, settings, mod, manifest and icon copied unchanged) with a `bin/loops` that downloads and builds nothing, because the directory refuses a plugin that downloads a file and runs it. It runs the `loops` program found on PATH (its own `bin/` removed) or in `~/.local/bin`, which the user installs once with `npx ai-loops bin`; the shim names no package launcher and keeps each command readable on its own line, because the directory validator refuses what it cannot follow. Without one, hooks and the status line exit 0 with no effect and session start asks for the install. Its README reuses the readme's "What it runs on your machine" bullets. Submit this folder, not the root, to the directory.
- **Copilot:** `copilot plugin install arxdsilva/ai-loops`, or in VS Code run *Chat: Install Plugin from Source*. Copilot reads `.claude-plugin/`, `hooks/hooks.json`, `agents/` (Claude frontmatter) and `skills/`, and `${CLAUDE_PLUGIN_ROOT}` is aliased. No generated package is needed. The Copilot emitter only writes `platforms/copilot/README.md` with these steps.

### 3.2 Codex: `platforms/codex/`
- **`plugin.json`** (Agent Plugins 1.0):
  - `name`, `version`, `description`, `author`, `homepage`, `repository`, `license`, `keywords`;
  - `extensions.com.openai.hooks = "./hooks/hooks.json"`.
- **`hooks/hooks.json`:**
  - Claude nesting (event → matcher groups → `{type:"command", command, timeout}`), with the same events as the root.
  - Commands use `"${PLUGIN_ROOT}/bin/loops" hook <name> --platform codex`.
  - Matchers are `*`; the binary filters by normalised tool name.
- **`agents/<role>.toml`:** `name`, `description`, `developer_instructions` (the role body), `model` (only when Options.Models maps the tier). A plugin cannot bundle subagents, so `loops install --target codex` copies them to `.codex/agents/` or `~/.codex/agents/`.
- **Marketplace:** the repo-root `.agents/plugins/marketplace.json` has an entry whose source is the `platforms/codex` subdirectory. `loops port build` writes it alongside the package.
- **Install:**
  1. `codex plugin marketplace add arxdsilva/ai-loops`.
  2. Install `loops` from the ChatGPT/Codex desktop app. There is no CLI install command.
  3. Run `loops install --target codex` to add the agents.

  Commands become `$work` and `$init`.
- **Verified 2026-11-09:**
  - `plugin.json` carries `$schema` (agent-plugins.org 1.0.0) and `extensions["com.openai"]`.
  - Marketplace entries need `policy.installation`, `policy.authentication` and `category`. Their source is `{"source":"local","path":"./platforms/codex"}`, and the top level needs `interface.displayName`.
  - There is no `PostToolUseFailure` event. Hook timeouts are in seconds, and matchers are regexes.
  - Codex also sets `CLAUDE_PLUGIN_ROOT`, so autodetection must not rely on that variable alone.
  - A per-agent `sandbox_mode` is ignored, so read-only roles get an instruction line instead.
- **Caveat:** plugin hooks run only for manually installed plugins, after the user trusts them (`/hooks`). They also make the plugin ineligible for the public directory.

### 3.3 Cursor: `platforms/cursor/`
- **`.cursor-plugin/plugin.json`:** `name`, `description`, `version`, `author`, `homepage`, `repository`, `license`, `keywords`.
- **`agents/<role>.md`:** frontmatter `name`, `description`, `model` (the mapped id, or `inherit`), `readonly`, `is_background`, then the role body.
  - `readonly` is always `false`, because every role writes its own run-dir file. Read-only roles get a body note instead.
  - `is_background` is `true` for wave roles: researchers, engineers, test-engineer, docs-writer and qa-verifier.
- **`skills/`:** the four commands get `disable-model-invocation: true` *only* for init, status and report. `work` stays model-invocable.
- **`hooks/hooks.json`:**
  - `{"version":1,"hooks":{…}}` with camelCase events: `sessionStart`, `preToolUse`, `postToolUse`, `postToolUseFailure`, `subagentStart`, `subagentStop`, `stop`.
  - Each entry is `{"command":"\"${CURSOR_PLUGIN_ROOT}/bin/loops\" hook <name> --platform cursor","timeout":<s>}`.
  - `stop` gets `"loop_limit": 5`.
- **`rules/loops.mdc`:** `alwaysApply: true` with the plugin context (`templates/PLUGIN_CONTEXT.md`).
- **Marketplace:** the repo-root `.cursor-plugin/marketplace.json` points at `platforms/cursor`. `loops port build` writes it alongside the package.
- **Install:** the Cursor marketplace, or a team marketplace import. Marketplace entries need `owner.name` and a bare-path `source`. For local development, copy the package to `~/.cursor/plugins/local/loops` *(unverified path)*.
- **Verified 2026-11-09:**
  - Subagents can be resumed by agent id, so Cursor is tier A.
  - `loop_limit` also applies to `subagentStop`.
  - `stop` and `subagentStop` hooks run from the workspace root, so commands must use `${CURSOR_PLUGIN_ROOT}`.

### 3.4 Gemini CLI: `platforms/gemini/`
- **`gemini-extension.json`:** `name`, `version`, `description`, `contextFileName: "GEMINI.md"`.
- **`GEMINI.md`:** the plugin context, plus `@./AGENTS-loops.md`-style imports only if needed. Keep it short.
- **`agents/<role>.md`:** frontmatter `name`, `description`, `kind: local`, `model` (when mapped), and `tools` (§5 mapping: every tool except edit tools when ReadOnly; omit `tools` otherwise), then the body.
- **`commands/loops/<cmd>.toml`:** `description`, plus `prompt = "Activate the <cmd> skill and follow it. Arguments: {{args}}"`. These become `/loops:work` and the other commands.
- **`hooks/hooks.json`:**
  - Events: `SessionStart`, `BeforeTool`, `AfterTool`, `AfterAgent`.
  - Each entry is `{type:"command", command:"\"${extensionPath}/bin/loops\" hook <name> --platform gemini", timeout:<ms>}`.
  - Gemini has no subagent events, so lanes come from `loops run label` only.
- **Install:** Gemini has no subdirectory install, only `--ref`. Clone the repo and run `gemini extensions install ./ai-loops/platforms/gemini`, or use `gemini extensions link` for development.
- **Verified 2026-11-09:**
  - Extension hooks are `{"hooks":{…}}`, and `matcher` is a regex.
  - Agent frontmatter also allows `temperature`, `max_turns` and `timeout_mins`.
  - `search_file_content` is now `grep_search`.
  - `GEMINI_PROJECT_DIR` is undocumented, so use `cwd`.

### 3.5 OpenCode: `platforms/opencode/` (installed with `loops install --target opencode`)
- **`.opencode/agents/loops/<role>.md`:** frontmatter `description`, `mode: subagent`, `model` (when mapped), and `permissions` when ReadOnly. Permissions are an ordered list of `{action, resource, effect}` entries: deny edit on `*`, then allow edit on `.loops/**` *(whether the path matches relative or absolute is unverified)*. The body gets a note limiting writes to the run dir.
- **`.opencode/commands/loops/<cmd>.md`:** "Use the <cmd> skill. $ARGUMENTS". It runs as `/loops/work`, because subdirectories prefix with `/`.
- **`.opencode/plugins/loops.ts`:** a small shim that pipes `tool.execute.before` and `tool.execute.after` events into `loops hook … --platform opencode` *(unverified API version; keep it minimal and documented)*.
- **Skills** go to `.agents/skills/`.
- **Rules:** OpenCode reads AGENTS.md natively.

### 3.6 Amp, Windsurf, Cline (installed with `loops install --target <name>`)
- **Skills:** `.agents/skills/` (Amp and Windsurf), and `.cline/skills/` for Cline. Rules come from `AGENTS.md`, which all three read.
- **Role files:** `agents/<role>.md` at the package root. The installer copies them into the plugin home, so `loops path agents` resolves them (tiers C and D).
- **Windsurf (Devin Local):** `.devin/hooks.v1.json`.
  - The file is the hooks object itself, with no wrapper key.
  - Events: SessionStart, PreToolUse, PostToolUse, Stop. Timeouts are in seconds.
  - Commands get `--platform windsurf`. `loops install` rewrites them to the plugin home's absolute shim path.
  - No subagents, so the solo fallback applies (§4).
- **Amp:** no hooks in v1. Subagents use the built-in Task with role prompts (§4).
- **Cline:** no hooks in v1; solo fallback.

### 3.7 Google Antigravity: `platforms/antigravity/`
- **`plugin.json`:** `"$schema": "https://antigravity.google/schemas/v1/plugin.json"`, `name`, `displayName: "loops"`, `version`, `description`, and optional `author` (`{"name": ...}`), `homepage`, `license`, `keywords` (`repository` is omitted because the v1 schema sets `additionalProperties: false`).
- **`skills/`:** canonical skills at `skills/<name>/SKILL.md` (`skills/work/platforms/` keeps only `antigravity.md`).
- **`agents/<role>.md`:** frontmatter `name`, `description`, `subagent: true`, `mainAgent: false`, `model` (`inherit`, or mapped via `loops port build --models`), explicit role-tailored `tools:` allowlists (`hardware-researcher`: `[run_command, send_message]`; `web-researcher`: `[view_file, write_to_file, run_command, grep_search, find_by_name, list_dir, send_message, search_web]`; other read-only roles: same list excluding `replace_file_content` and `search_web`; implementers: full list including `replace_file_content`), and `commandExecutionPolicy: "auto"`. Read-only roles prepend the read-only instruction note to the body (covering `run_command`), while `write-guard` enforces run-dir scoping on file writes.
- **`rules/AGENTS.md`:** always-on plugin context (`templates/PLUGIN_CONTEXT.md`).
- **`hooks.json`:** `{"loops":{"enabled":true,"<Event>":...}}` for `SessionStart`, `PreToolUse`, `PostToolUse`, `Stop` (`PostToolUseFailure`, `SubagentStart`, `SubagentStop` are dropped). `SessionStart` and `Stop` use flat handler arrays `[{"type":"command","command":"./bin/loops hook <name> --platform antigravity","timeout":<s>}]` (plugin hooks run from `<plugin-root>`). `PreToolUse` and `PostToolUse` wrap handlers in `[{"matcher":"<regex>","hooks":[...]}]` using Antigravity tool regexes (`Read` → `view_file`, `Bash` → `run_command`, `Edit` → `replace_file_content`, `MultiEdit` → `multi_replace_file_content`, `Write` → `write_to_file`, `Agent`/`Task` → `invoke_subagent`; `"*"` or empty omits `"matcher"`). `PreToolUse` also registers `write-guard` (`replace_file_content|write_to_file|multi_replace_file_content`), restricting read-only roles to `<run-dir>/**` (and `.loops/memory/**` for `reflector`).
- **Install (app and `agy` CLI; verified 2026-11-21):**
  - Native plugin: `agy plugin validate ./platforms/antigravity` then `agy plugin install ./platforms/antigravity`.
  - Installer (`npx ai-loops --target antigravity` or `loops install --target antigravity [--scope project|user]`): `InstallPlan` places `skills/`, `agents/`, `rules/`, and `hooks.json` into `.agents/{skills,agents,rules,hooks.json}` (project scope, hook commands pinned to `.loops/plugin/bin/loops`) or `~/.gemini/config/{skills,agents,rules,hooks.json}` (user scope, hook commands pinned to `~/.local/share/loops/plugin/bin/loops`), plus shared files (`bin/loops`, `specs/`, `templates/`, `ROLES.md`, `skills/`, `agents/`, `plugin.json`, `LICENSE`, `README.md`) in `.loops/plugin/` or `~/.local/share/loops/plugin/`. Run `/skills reload` after installing or updating; `agy` exposes skills as `/work`, `/init`, `/status`, `/report`.
- **Subagents:** tier A custom subagents (`invoke_subagent` + `send_message` to idle/running subagents), with tier D solo fallback when subagents are unavailable (§4).

## 4. Degradation tiers (how `skills/work` runs on each platform)

| Tier | Platforms | Spawning roles | Loop-backs |
|---|---|---|---|
| **A: resumable subagents** | Claude, Cursor, Antigravity | Typed role agent (`loops:<role>` on Claude; the role subagent by name on Cursor; custom `agents/<role>.md` via `invoke_subagent` on Antigravity), in the background, a whole wave in one message | Resume the **same** agent (`SendMessage` on Claude, by agent id on Cursor, `send_message` to idle/running subagent on Antigravity) |
| **B: subagents, no resume** | Copilot, Codex, Gemini, OpenCode | The platform's subagent mechanism with the role's agent definition | Spawn a **new** agent of the same role. It gets its previous report, the QA or review file, and a ≤10-line history summary. The run-dir files carry the context |
| **C: generic subagents** | Amp | A generic subagent (`Task`) whose prompt starts with the role file's body (`loops path agents` → `<role>.md`) | Re-spawn as in B |
| **D: no subagents** | Windsurf, Cline (plus Antigravity solo fallback) | **Solo mode.** The lead plays each role in turn, in phase order. It writes the same run-dir files and records the same events (`loops run label --agent solo-<n> …`, then `agent_start`/`agent_stop` through `loops run event`). Before each QA or review pass it re-reads only the files that role is given, to approximate a fresh context | Same files and limits; the lead re-enters the author role |

Every tier records the same phases, verdicts and loop-backs. Only the parallelism and context isolation differ. On every tier the lead stays the master agent (WORKFLOW §2.7): it re-anchors with the brief that `loops run phase` prints, and with `loops run brief` after a compaction or a resumed session. Where the platform delivers SessionStart after a compaction (Claude Code's `compact` source), the `session-start` hook adds the brief to the lead's context itself. Each platform's mechanics (tool names, how to spawn, wait, ask the user, stop agents) live in `skills/work/platforms/<platform>.md`. The work skill tells the lead to read the one for its platform.

## 5. Hook dialects

Every hook command passes `--platform <p>` (for Antigravity, `--platform` also accepts aliases `agy` and `antigravity-cli`; no runtime env var is verified *(unverified)*). Without the flag, the binary autodetects the platform from the environment:
- `CURSOR_PLUGIN_ROOT` or `CURSOR_PROJECT_DIR` means cursor;
- `GEMINI_PROJECT_DIR` or `GEMINI_CWD` means gemini;
- `DEVIN_PROJECT_DIR` means windsurf;
- `PLUGIN_ROOT` without `CLAUDE_PROJECT_DIR` means codex;
- otherwise claude.

The adapter translates each platform's events into the canonical names and normalises tool names before any handler runs.

| Canonical | Claude/Copilot/Codex | Cursor | Gemini | Windsurf | Antigravity |
|---|---|---|---|---|---|
| SessionStart | SessionStart | sessionStart | SessionStart | SessionStart | SessionStart |
| PreToolUse | PreToolUse | preToolUse | BeforeTool | PreToolUse | PreToolUse |
| PostToolUse(+Failure) | PostToolUse / PostToolUseFailure | postToolUse / postToolUseFailure | AfterTool | PostToolUse | PostToolUse (no Failure event) |
| SubagentStart/Stop | SubagentStart / SubagentStop | subagentStart / subagentStop | (none) | (none) | (none) |
| Stop | Stop | stop | AfterAgent | Stop | Stop |

**Tool names** are normalised to the canonical `Read`, `Bash`, `Edit`, `Write`, `MultiEdit`, `Agent`, `Grep`, `Glob`, `WebFetch` and `WebSearch`:
- Gemini: `read_file` and `read_many_files` → Read; `run_shell_command` → Bash; `replace` → Edit; `write_file` → Write; `grep_search` and the older `search_file_content` → Grep; `glob` → Glob; `web_fetch` → WebFetch; `google_web_search` → WebSearch.
- Cursor (verified): `Shell` → Bash; `Read` → Read; `Write` → Write; `Grep` → Grep; `Task` → Agent; `Delete` and `MCP:<tool>` pass through unchanged.
- Codex (verified): the shell tool is `Bash`. `apply_patch` → Edit, with the patch text in `tool_input.command`; edited file paths are parsed from the patch.
- Devin: `exec` → Bash.
- Antigravity: `view_file` → Read; `run_command` → Bash; `replace_file_content` → Edit; `multi_replace_file_content` → MultiEdit; `write_to_file` → Write; `find_by_name` → Glob; `invoke_subagent` → Agent.

**Tool-input fields** are normalised to `file_path` (also from `absolute_path`, `AbsolutePath`, `path`, `Path`, `file`, `filePath`, `target_file`, and `TargetFile`) and `command` (also from `CommandLine`, `command_line`, and `cmd`).

**Outputs:**

| Decision | Claude/Copilot/Codex | Cursor | Gemini | Windsurf | Antigravity |
|---|---|---|---|---|---|
| Deny a tool | exit 2, reason on stderr | exit 2, reason on stderr (also `{"permission":"deny","user_message"…}`) | exit 2, reason on stderr | exit 2, reason on stderr | exit 0, stdout `{"decision":"deny","reason":R}` |
| Block stop | `{"decision":"block","reason":R}` | `{"followup_message":R}` | `{"decision":"deny","reason":R}` | `{"decision":"block","reason":R}` | exit 0, stdout `{"decision":"continue","reason":R}` |
| Session context | `{"hookSpecificOutput":{"hookEventName":"SessionStart","additionalContext":C}}` | `{"additional_context":C}` | as Claude | as Claude | exit 0, stdout `{"injectSteps":[{"userMessage":C}]}` |
| Post-edit failure | `{"decision":"block","reason":R}` | `{"additional_context":R}` | `{"hookSpecificOutput":{"additionalContext":R}}` (a deny would hide the tool output) | as Claude | exit 0, stdout `{"injectSteps":[{"userMessage":R}]}` *(unverified)* |

**Project dir:**
- Claude: `CLAUDE_PROJECT_DIR`;
- Cursor: `CURSOR_PROJECT_DIR`;
- Gemini: `GEMINI_PROJECT_DIR`;
- Windsurf: `DEVIN_PROJECT_DIR`;
- Antigravity: `workspacePaths[0]` folded into `cwd` (or `CLAUDE_PROJECT_DIR` if set);
- otherwise the payload's `cwd`.

## 6. The binary

**Finding the plugin root.** `loops path` looks in this order:
1. `LOOPS_PLUGIN_ROOT`, which the shims export;
2. `<project>/.loops/plugin-root`, which session-start writes when `.loops/` exists;
3. `<project>/.loops/plugin`, the project-scope plugin home;
4. `~/.local/share/loops/plugin`, the user-scope plugin home.
5. `${XDG_STATE_HOME:-~/.local/state}/loops/plugin-root`, written by every session-start. This covers a fresh project's first session.

**Symlinks.** Project-scope installs never write through a symlinked path component. User-scope installs may follow symlinks in `$HOME`, but `--force` never overwrites outside it.

**Install records.** Each install records what it placed in `<plugin home>/installed-<platform>.json`: paths relative to the install base, plus a sha256 of each file. A re-install removes only its own stale files, and only when they stay inside the base, pass through no symlink, and still match the recorded hash.

**Getting the binary (no Go needed).** Every release tag publishes prebuilt binaries as GitHub Release assets, built by `.github/workflows/release-assets.yml` and `scripts/build-release.sh`:
- assets: `loops-{darwin,linux}-{amd64,arm64}` and `loops-windows-{amd64,arm64}.exe`;
- `checksums.txt` alongside them.

Both shims, the root `bin/loops` and the generated `platforms/*/bin/loops`, find the binary in this order:
1. Resolve their own path through symlinks and export `LOOPS_PLUGIN_ROOT`.
2. **Root shim in a git checkout:** build from source (development). `LOOPS_PREFER_RELEASE=1` skips this step.
3. A non-script `loops` on PATH that reports the plugin version.
4. The cached `loops-<version>`, in `${CLAUDE_PLUGIN_DATA:-${PLUGIN_DATA:-${XDG_CACHE_HOME:-~/.cache}/loops}}`.
5. **Download** the asset for the OS and arch, verify its sha256 against `checksums.txt`, and cache it.
   - `LOOPS_RELEASE_BASE` overrides the URL prefix.
   - A wrong or missing checksum is refused and never falls through.
6. **Fallback:** `go install <module>/cmd/loops@v<version>`, or a source build for the root shim.
7. Otherwise, a clear error with a link to the install docs.

**Installer platforms.** `install.sh --target <p> [--scope] [--force] [--version] [--bin-dir]` (fetch it as `docs/GUIDE.md#install` shows, or run `npx ai-loops --target <p>`) downloads and verifies the binary the same way, puts it in `~/.local/bin`, and runs `loops install`.

The repository must be public for release downloads and for the update check. A release is a `v<version>` tag that matches `.claude-plugin/plugin.json`, the single version source.

## 7. Models

Tiers (`haiku`, `sonnet`, `opus`) are Claude names. Other platforms inherit the session model unless `loops port build --models <file.json>` maps them, e.g. `{"codex":{"opus":"<model id>"}}`. Model ids change too often to hard-code, so the generated packages in the repo leave the model unset, and each platform's notes tell users how to map tiers.

### 5.0 toolchain-env (alias go-env) rewrite support (verified 2026-11-09; Antigravity 2026-11-21; details in `dialect.go`)

| Platform | Unscoped `go …` command |
|---|---|
| Claude Code | Rewritten (`updatedInput`). Only a lone `go build`, `go test`, `go vet` or `go list` is auto-approved |
| Gemini CLI | Rewritten (`hookSpecificOutput.tool_input`) |
| Windsurf/Devin | Rewritten (`hookSpecificOutput.updatedInput`) |
| Antigravity | Rewritten (`{"overwrite":{...}}`, shallow merge into `toolCall.args`) |
| Codex, Cursor | A lone gate command is rewritten and approved. Anything else is denied, because a rewrite requires "allow" |
| Copilot, OpenCode | Denied. Use `loops go …` |
| Amp, Cline | No hooks. Agents use `loops go …` |

### 5.1 Verified dialect notes (2026-11-09; Antigravity 2026-11-21; full list with URLs in `internal/adapters/inbound/hook/dialect.go`)

- **Copilot camelCase events** (`sessionStart`, `preToolUse`, `postToolUse`, `agentStop`):
  - payloads use `toolName`, `toolArgs`, `toolResult` and `stopHookActive`, with no `hook_event_name`;
  - session context goes in a top-level `additionalContext`.
  - PascalCase events use the Claude shape. The root plugin's `hooks/hooks.json` is PascalCase.
- **Cursor:**
  - there is no `stop_hook_active`; `loop_count > 0` plays that role;
  - `subagentStart` carries `subagent_id` and `subagent_type`, but `subagentStop` has no id, so the newest running lane of that role is closed;
  - `duration` is in milliseconds, and `CLAUDE_PROJECT_DIR` is an alias of `CURSOR_PROJECT_DIR`.
- **Gemini:** `AfterAgent` carries `stop_hook_active`, and a deny there forces a retry. The env vars are `GEMINI_PROJECT_DIR`, `GEMINI_CWD` and `GEMINI_SESSION_ID`.
- **Devin (Windsurf):** there is no `stop_hook_active`. On every platform, the stop gate releases itself after 3 blocks in a row with no tool step between them.
- **Antigravity (verified 2026-11-21):** `SessionStart`, `PreToolUse`, `PostToolUse`, `Stop`; payloads carry `toolCall{name, args}` (object or JSON string), `workspacePaths`, and `conversationId`. Non-zero exits are logged and ignored (fail open), so every hook exits `0` and emits stdout JSON: `PreToolUse` deny `{"decision":"deny","reason":...}`, `PreToolUse` rewrite `{"overwrite":{...}}` (shallow merge into `toolCall.args`), `Stop` `{"decision":"continue","reason":...}`, `SessionStart` `{"injectSteps":[{"userMessage":...}]}`, and `PostToolUse` `post-edit` failure `{"injectSteps":[{"userMessage":...}]}` *(unverified)* (docs list `{}` for `PostToolUse`). Runtime env vars for autodetection remain undocumented *(unverified)*.

## 8. Budgets and hardware

The `hardware-researcher` is a normal haiku subagent on tiers A-C. On tier D the lead runs `loops hw --write` itself (WORKFLOW §2.1). Token budgets (`--budget`, WORKFLOW §4a) are measured from each agent's own transcript (`agent_transcript_path` on SubagentStop). Claude Code sends that field. Codex and Copilot are measured only if their SubagentStop payload carries it *(unverified)*. On every other platform, token budgets are advisory. Agent-count, role and concurrency limits are enforced by the lead on every platform.

## 9. Updates

Mechanism: the hook asks the Go module proxy for `@latest` of `github.com/arxdsilva/ai-loops`, at most once per 24h, and announces each version once. State lives in `~/.local/state/loops/update.json`. Opt-outs: `LOOPS_NO_UPDATE_CHECK=1`, `"update_check": false` in `.loops/config.json`, and `CI=true`. The notice carries the command below; `loops update [--platform p]` prints it.

### Auto-update at work start

`<plugin root>/bin/loops update --apply [--platform p]` (WORKFLOW §2.0 step 0) updates the installed plugin without asking.
- **Version:** the plugin's, read from `<root>/.claude-plugin/plugin.json`. The root comes from `LOOPS_PLUGIN_ROOT`, then `.loops/plugin-root`, then `~/.local/state/loops/plugin-root`. With none, it falls back to the binary's version and the line says so.
- **Check:** fresh. It ignores the 24h interval and the announce-once state, and does not change `last_notified`.
- **Routes:** only Claude Code and Copilot CLI run automatically, as the argv of the table above (no shell). It never runs `install.sh`, `curl | sh` or `loops install`. Every other platform gets a `manual` line with its route.
- **Budget and exit:** 60s for the route, always exit 0, exactly one line starting `loops update:`. Kinds: `skipped`, `up to date`, `updated`, `pending`, `failed`, `backoff`, `manual`, `check failed`. Two optional suffixes: `[version of this loops binary: <why>]` when no plugin root resolved, and `[this loops binary is <v>; call <root>/bin/loops]` when the binary differs from the plugin. The code in `internal/application/updatecheck/apply.go` and `internal/adapters/inbound/cli/update_cmd.go` has the exact wording.
- **Skips:** the opt-outs above, a `<root>/.git` checkout, and a `*-dev` version.
- **Attempts:** recorded in `update.json` as `applied`, `applied_at` and `apply_err`. The same version is not retried within 24h (`backoff`, or `pending` after a success); a newer version runs at once.
- **Binary:** this session keeps the old plugin, skills and shim binary alike. The new plugin's `bin/loops` resolves `loops-<V>` on first use. A `loops` on PATH at another version is not the plugin's; the line says to call the shim.
- **Reload:** Claude Code `/reload-plugins` or a new session; Copilot `/restart` or a new session. The lead relays the line once and continues on the current version (a non-zero exit, such as `unknown flag` from an older binary, also means continue).

Verified 2026-11-09 (Antigravity 2026-11-21):

| Platform | Command | Source |
|---|---|---|
| Claude Code | `claude plugin marketplace update ai-loops && claude plugin update loops@ai-loops` (in-app `/plugin marketplace update <name>`) | https://code.claude.com/docs/en/discover-plugins |
| Copilot CLI | `copilot plugin marketplace update ai-loops && copilot plugin update loops` | https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-plugin-reference |
| Codex | `codex plugin marketplace upgrade [name]` refreshes Git marketplaces only; no CLI plugin-update command is documented, so the plugin is updated in the desktop app and the binary and agents via the installer route | https://learn.chatgpt.com/docs/developer-commands?surface=cli , https://learn.chatgpt.com/docs/plugins |
| Cursor | No update command. Team marketplaces: Dashboard > Plugins & MCPs > Refresh (or Enable Auto Refresh); running agents keep old versions | https://cursor.com/docs/plugins |
| Gemini CLI | `git -C ai-loops pull && gemini extensions update loops` (`update --all` also exists; installs are copies) | https://geminicli.com/docs/extensions/reference/ |
| OpenCode, Amp, Windsurf, Cline | `npx ai-loops --target <p> --force` (or `install.sh --target <p> --force`) (prebuilt binary, no Go needed) | this repo |
| Antigravity | `agy plugin install ./platforms/antigravity` (native plugin) or `npx ai-loops --target antigravity --force` (installer) | https://antigravity.google/docs/build-plugins |

Unverified: that `gemini extensions update` works for a local-path install (docs imply it); Copilot's plugin name after `arxdsilva/ai-loops` install (assumed `loops`); the Codex marketplace name (omitted, so all are refreshed); that the Cursor marketplace entry auto-refreshes without the GitHub App.
