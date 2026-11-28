# loops for Claude Code

Multi-agent workflow that turns a ticket into a pull request with proof: research, plan with a reviewer agent, parallel specialist engineers, an independent QA agent per engineer, every acceptance criteria proven, reflection. One master agent, opt-in checkpoints, a ≥70% new-code coverage and race-detector gate, 15 backend languages, live status and HTML report.

This is the Claude plugin directory package of [github.com/arxdsilva/ai-loops](https://github.com/arxdsilva/ai-loops). It is the same plugin as the marketplace install, except that it downloads nothing: it runs a `loops` binary you install once.

## Install

1. Install loops from the Claude Code plugin directory.
2. Install the `loops` binary once, in a terminal: `npx ai-loops bin` (Node 18+). It downloads the release binary for your OS from GitHub, checks it against the release's `checksums.txt` and puts it in `~/.local/bin`. Without Node: `go install github.com/arxdsilva/ai-loops/cmd/loops@v0.21.0`, or the other ways in the [install guide](https://github.com/arxdsilva/ai-loops/blob/main/docs/GUIDE.md#install).
3. In your repository: `/loops:init`, then `/loops:work <ticket>`.

Prefer automatic setup? `/plugin marketplace add arxdsilva/ai-loops` and `/plugin install loops@ai-loops` install the same plugin with a shim that downloads the binary on first use.

## What it runs on your machine

Everything goes through `bin/loops`, a shell script that runs the `loops` program installed on your PATH or in `~/.local/bin` (step 2 above; keep it at this package's version, v0.21.0, by re-running that step after an update). It downloads nothing. Without it every hook does nothing, and session start tells Claude to ask you to install it.

- **Hooks** (`hooks/hooks.json`). Each one runs `"${CLAUDE_PLUGIN_ROOT}/bin/loops" hook <name>` and fails open: no active run, bad input or an internal error means no effect. None of them sends anything off your machine. The [guide](https://github.com/arxdsilva/ai-loops/blob/main/docs/GUIDE.md#guardrails-hooks) has the details and the switches.

  | Hook | Event (matcher) | What it does | What it changes |
  |---|---|---|---|
  | session-start | SessionStart | Tells Claude where the plugin lives, which run is active, and when an update exists | Adds text to the session's context; no files |
  | read-guard | PreToolUse (Read, Bash) | Denies a whole-file read of a very large file; Claude reads it in ranges | Nothing; it can only deny the call |
  | test-guard | PreToolUse (Bash) | Denies a Go or Swift test run without its race flag, except a Go run that toolchain-env fixes | Nothing; it can only deny the call |
  | toolchain-env | PreToolUse (Bash) | Points the language toolchain at the run's cache directories (Go's build cache is shared across runs, in your temp dir); adds `-race` to a Go test run that lacks it | Rewrites that command's environment variables and, for `go test`, adds the `-race` flag; denies a Go command it cannot scope where the platform cannot rewrite it |
  | model-guard | PreToolUse (Agent, Task) | Denies a general subagent launched without a model, and a loops role launched above its model tier without a `Model reason:` line | Nothing; it can only deny the call |
  | post-edit | PostToolUse (Edit, Write, MultiEdit) | Runs the language's formatter and linter on the edited file | Formats that file in place; sends lint failures back to Claude |
  | trace | PostToolUse, PostToolUseFailure (every tool) | Records the tool call in the run's log, names each spawned agent's lane, and after `loops run start` reads the end of the session transcript to tell Claude when the session is already large | Appends lines to `.loops/runs/<id>/events.jsonl`; may add one line of text to the session's context |
  | trace-agent | SubagentStart, SubagentStop | Records each agent's start and stop | Appends one line to the same log |
  | stop-gate | Stop | Keeps Claude working while sources changed since the last green gate, and counts the session's own tokens since the run started, from its transcript | Appends a `lead-tokens` line to the run's log; otherwise it can only block the stop |

- **Live panes (mod, `mod/`)**. It adds the `/loops-pane` command, which shows the active run beside the chat, and `/loops-backlog`, which shows a backlog's lanes, queue and outcomes.
  - *What it reads:* `.loops/status.json` in your project (the active run's state, which the plugin's hooks keep up to date). When the project runs a backlog, it also reads `.loops/backlog/state.json` and each running lane's `.loops/status.json` in that lane's worktree, which sits beside your checkout. It reads them every 2 seconds and when you type either command. It starts no program.
  - *What it reads from Claude Code:* only that `/loops-pane` or `/loops-backlog` was typed (`command.run`). It reads no conversation text and no tool calls.
  - *What it sends, and where:* nothing leaves your machine. The command's output is drawn in the pane and kept nowhere else.
- **Status line** (`settings.json`): `bin/loops status --subagent`.
- **Programs**: `git`; `gh` or `glab` for tickets and PRs (`gh` also files the reflector's harness suggestions as issues, only on the loops plugin repo itself, searching open issues first; if `gh` fails the text stays in `reflection.md`); your language's formatter, linter and test runner during `loops gate` and post-edit; the language pack's security scanners (Go: govulncheck, gosec) when installed, in the QA security pass (`loops gate --security`); and, at the start of `/loops:work`, the `claude` or `copilot` plugin update commands (`claude plugin update`, `copilot plugin update`) through `loops update --apply`; in backlog mode only, `git worktree add/remove` beside your checkout, one headless agent per lane (`claude -p … --permission-mode auto` unless you pass `--agent`), and `gh` to list issues and open PRs, label issues, comment, assign and file findings, all in the repository you point it at.
- **Network**: none from this package. During a run, your agents use your ticket and PR tools (`gh`, `glab`, connectors) and web research; the `loops` binary checks proxy.golang.org for a newer version at most once a day, plus a fresh check and `claude plugin update` at the start of `/loops:work` (turn both off with `"update_check": false` in `.loops/config.json` or `LOOPS_NO_UPDATE_CHECK=1`). Run records stay in `.loops/runs/` in your repository.

MIT license.
