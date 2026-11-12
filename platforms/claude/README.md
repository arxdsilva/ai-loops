# loops for Claude Code

Multi-agent workflow that turns a ticket into a pull request with proof: research, plan with a reviewer agent, parallel specialist engineers, an independent QA agent per engineer, every acceptance criteria proven, reflection. One master agent, opt-in checkpoints, a ≥70% new-code coverage and race-detector gate, 15 backend languages, live status and HTML report.

This is the Claude plugin directory package of [github.com/arxdsilva/ai-loops](https://github.com/arxdsilva/ai-loops). It is the same plugin as the marketplace install, except that it downloads nothing: it runs a `loops` binary you install once.

## Install

1. Install loops from the Claude Code plugin directory.
2. Install the `loops` binary once, in a terminal: `npx ai-loops bin` (Node 18+). It downloads the release binary for your OS from GitHub, checks it against the release's `checksums.txt` and puts it in `~/.local/bin`. Without Node: `go install github.com/arxdsilva/ai-loops/cmd/loops@v0.9.1`, or the other ways in the [install guide](https://github.com/arxdsilva/ai-loops/blob/main/docs/GUIDE.md#install).
3. In your repository: `/loops:init`, then `/loops:work <ticket>`.

Prefer automatic setup? `/plugin marketplace add arxdsilva/ai-loops` and `/plugin install loops@ai-loops` install the same plugin with a shim that downloads the binary on first use.

## What it runs on your machine

Everything goes through `bin/loops`, a shell script that runs the `loops` binary on your PATH (or in `~/.local/bin` or the loops cache), preferring the one whose version matches this package (v0.9.1). It downloads nothing. Without a binary every hook does nothing, and session start tells Claude to ask you to install it.

- **Hooks** (`hooks/hooks.json`). Each one runs `"${CLAUDE_PLUGIN_ROOT}/bin/loops" hook <name>`, fails open (no run, bad input or an error means no effect), and changes only what the table says. The full table is in the [guide](https://github.com/arxdsilva/ai-loops/blob/main/docs/GUIDE.md#guardrails-hooks).

  | Hook | When | What it does or changes |
  |---|---|---|
  | session-start | session start | Adds context: where the plugin lives, the active run, an update notice |
  | read-guard | before Read, Bash | Denies whole-file reads of very large files; read in ranges |
  | test-guard | before Bash | Denies a Go or Swift test run without its race flag |
  | toolchain-env | before Bash | Rewrites toolchain commands to use the run's cache directories |
  | model-guard | before Agent | Denies a general subagent launched without a model |
  | post-edit | after Edit, Write | Runs the language's formatter and linter on the edited file |
  | trace, trace-agent | after each tool, agent start and stop | Appends one line to `.loops/runs/<id>/events.jsonl` |
  | stop-gate | Stop | Blocks finishing while sources changed since the last green gate |

- **Live pane (mod, `mod/`)**. It adds the `/loops` command, runs `bin/loops status --json` every 3 seconds and after tool and agent events, and draws the result. It sends nothing anywhere.
- **Status line** (`settings.json`): `bin/loops status --subagent`.
- **Programs**: `git`; `gh` or `glab` for tickets and PRs; your language's formatter, linter and test runner during `loops gate` and post-edit.
- **Network**: none from this package. During a run, your agents use your ticket and PR tools (`gh`, `glab`, connectors) and web research; the `loops` binary checks proxy.golang.org for a newer version at most once a day (turn it off with `"update_check": false` in `.loops/config.json` or `LOOPS_NO_UPDATE_CHECK=1`). Run records stay in `.loops/runs/` in your repository.

MIT license.
