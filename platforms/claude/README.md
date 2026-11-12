# loops for Claude Code

Multi-agent workflow that turns a ticket into a pull request with proof: research, plan with a reviewer agent, parallel specialist engineers, an independent QA agent per engineer, every acceptance criteria proven, reflection. One master agent, opt-in checkpoints, a ≥70% new-code coverage and race-detector gate, 15 backend languages, live status and HTML report.

This is the Claude plugin directory package of [github.com/arxdsilva/ai-loops](https://github.com/arxdsilva/ai-loops). It is the same plugin as the marketplace install, except that it downloads nothing: it runs a `loops` binary you install once.

## Install

1. Install loops from the Claude Code plugin directory.
2. Install the `loops` binary once, in a terminal: `npx ai-loops bin` (Node 18+). It downloads the release binary for your OS from GitHub, checks it against the release's `checksums.txt` and puts it in `~/.local/bin`. Without Node: `go install github.com/arxdsilva/ai-loops/cmd/loops@v0.9.5`, or the other ways in the [install guide](https://github.com/arxdsilva/ai-loops/blob/main/docs/GUIDE.md#install).
3. In your repository: `/loops:init`, then `/loops:work <ticket>`.

Prefer automatic setup? `/plugin marketplace add arxdsilva/ai-loops` and `/plugin install loops@ai-loops` install the same plugin with a shim that downloads the binary on first use.

## What it runs on your machine

Everything goes through `bin/loops`, a shell script that runs the `loops` program installed on your PATH or in `~/.local/bin` (step 2 above; keep it at this package's version, v0.9.5, by re-running that step after an update). It downloads nothing. Without it every hook does nothing, and session start tells Claude to ask you to install it.

- **Hooks** (`hooks/hooks.json`). Each one runs `"${CLAUDE_PLUGIN_ROOT}/bin/loops" hook <name>` and fails open: no active run, bad input or an internal error means no effect. None of them sends anything off your machine. The [guide](https://github.com/arxdsilva/ai-loops/blob/main/docs/GUIDE.md#guardrails-hooks) has the details and the switches.

  | Hook | Event (matcher) | What it does | What it changes |
  |---|---|---|---|
  | session-start | SessionStart | Tells Claude where the plugin lives, which run is active, and when an update exists | Adds text to the session's context; no files |
  | read-guard | PreToolUse (Read, Bash) | Denies a whole-file read of a very large file; Claude reads it in ranges | Nothing; it can only deny the call |
  | test-guard | PreToolUse (Bash) | Denies a Go or Swift test run without its race flag | Nothing; it can only deny the call |
  | toolchain-env | PreToolUse (Bash) | Points the language toolchain at the run's own cache directories | Rewrites that command's environment variables |
  | model-guard | PreToolUse (Agent, Task) | Denies a general subagent launched without a model | Nothing; it can only deny the call |
  | post-edit | PostToolUse (Edit, Write, MultiEdit) | Runs the language's formatter and linter on the edited file | Formats that file in place; sends lint failures back to Claude |
  | trace | PostToolUse, PostToolUseFailure (every tool) | Records the tool call in the run's log | Appends one line to `.loops/runs/<id>/events.jsonl` |
  | trace-agent | SubagentStart, SubagentStop | Records each agent's start and stop | Appends one line to the same log |
  | stop-gate | Stop | Keeps Claude working while sources changed since the last green gate | Nothing; it can only block the stop |

- **Live pane (mod, `mod/`)**. It adds the `/loops-pane` command and shows the active run beside the chat.
  - *What it runs:* one program, `${CLAUDE_PLUGIN_ROOT}/bin/loops status --json`, through `process.run`, every 2 seconds and when you type `/loops-pane`. That command reads the active run's state from `.loops/runs/` in your repository.
  - *What it reads from Claude Code:* only that `/loops-pane` was typed (`command.run`). It reads no conversation text and no tool calls.
  - *What it sends, and where:* nothing leaves your machine. The command's output is drawn in the pane and kept nowhere else.
- **Status line** (`settings.json`): `bin/loops status --subagent`.
- **Programs**: `git`; `gh` or `glab` for tickets and PRs; your language's formatter, linter and test runner during `loops gate` and post-edit.
- **Network**: none from this package. During a run, your agents use your ticket and PR tools (`gh`, `glab`, connectors) and web research; the `loops` binary checks proxy.golang.org for a newer version at most once a day (turn it off with `"update_check": false` in `.loops/config.json` or `LOOPS_NO_UPDATE_CHECK=1`). Run records stay in `.loops/runs/` in your repository.

MIT license.
