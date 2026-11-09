# ai-loops: the `loops` plugin

A Claude Code plugin for working on Go backends with agents. Every ticket goes through six visible phases, each staffed by specialised **role agents**. Review loops send work back with context until it is right. The run ends with a pull request backed by proofs, and a reflection saved to long-term memory.

```
/loops:work #123
  0 intake    ticket → ticket.md (GitHub · GitLab · Jira · Linear · free text)
  1 research  web-researcher ∥ practices-researcher ∥ code-researcher   (ai-memory first)
  2 plan      planner (opus) ⇄ plan-reviewer (opus)   [+ your approval with --approve]
  3 implement architect (contract, wave 0) → role engineers in parallel waves
  4 qa        a qa-verifier per engineer checks every claim ⇄ sends failures back with a repro
  5 product   product-reviewer tests every acceptance criterion by hand → proofs → PR/MR
  6 reflect   reflector → reflection.md + ai-memory pages
```

## Install

```sh
/plugin marketplace add insurgencygames/ai-loops
/plugin install loops@ai-loops
```

Requirements:
- **Go** is required. The plugin's `loops` binary is built from source on first use.
- **`gh`** or **`glab`** is needed to fetch tickets and open PRs/MRs.
- Optional:
  - `staticcheck` and `govulncheck`, for the extra gates;
  - [ai-memory](https://github.com/akitaonrails/ai-memory), for long-term memory across runs;
  - Jira or Linear MCP connectors, for tickets from those trackers.

Then, in your Go repo:

```sh
/loops:init      # .loops/config.json, AGENTS.md rules, .gitignore, dependency check
/loops:status    # optional: install the live status line
/loops:work #42  # or a GitLab URL, a Jira or Linear key, or plain text
```

## What you see while it runs

- **Status line.** For example: `▶ #42 add-orders · Implement W1 · 4/5 lanes · QA 2✓ 1↺ · plan r2 · gates build✓ vet✓ · 87 steps · 12m`.
- **Live pane.** Shown beside the chat in VS Code and in the terminal. It lists the phase strip with loop counters, one lane per role agent with its QA state, the latest steps, and the gates.
- **HTML report.** `loops report` writes `.loops/runs/<id>/report.html`, which works offline. It holds:
  - the phase timeline;
  - the role and QA lanes, with arrows for each loop-back;
  - every tool call;
  - gate results;
  - the matrix from acceptance criteria to proofs;
  - the PR and the reflection.

Everything is recorded in `.loops/runs/<id>/events.jsonl`, written by the skills (`loops run …`) and by the plugin's hooks.

## Guardrails (hooks)

| Hook | What it does |
|---|---|
| session-start | Tells Claude where the plugin lives and which run is active. |
| read-guard | Denies a whole-file Read of a file over 500 lines, and shell views over 150 lines of repo files. Read in ranges instead. Tune it with `READ_GUARD_MIN_LINES` and `READ_GUARD_MAX_VIEW_LINES`, or turn it off with `READ_GUARD=off`. |
| model-guard | Untyped or general-purpose agents must name a model. |
| go-post-edit | Runs `goimports` (or `gofmt`) and `go vet` on every edited `.go` file. It sends the failure back to Claude when either one fails. |
| trace, trace-agent | Records every tool call, and every agent start and stop, in the right agent's lane. |
| stop-gate | The lead can't finish while Go files have changed since the last green `loops gate`. This doesn't apply when no run is active, while agents are still running, or with `LOOPS_GATE=off`. |

`loops arch` enforces the hexagonal dependency direction and the SOLID rules (`specs/`). Size limits are reported as warnings only.

## Layout

- `agents/`: the 16 roles, summarised in `ROLES.md`.
- `skills/`: the `work` orchestrator, `init`, `status`, `report`, and the reference skills.
- `specs/`: the workflow spec, plus the Go and architecture rules.
- `templates/`: what the agents fill in.
- `cmd/loops` and `internal/`: the Go tool, stdlib only.

The rules and skills are extracted, game-agnostic, from the rimworld-go agent system.
