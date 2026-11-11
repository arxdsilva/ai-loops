# loops: the multi-agent coding workflow that proves its work

**Give your AI coding agent a ticket. Get back a pull request with proof that every acceptance criterion works.**

[![CI](https://github.com/arxdsilva/ai-loops/actions/workflows/ci.yml/badge.svg)](https://github.com/arxdsilva/ai-loops/actions/workflows/ci.yml) [![release](https://img.shields.io/github/v/release/arxdsilva/ai-loops?sort=semver)](https://github.com/arxdsilva/ai-loops/releases) [![license: MIT](https://img.shields.io/badge/license-MIT-green)](LICENSE) ![dependencies: 0](https://img.shields.io/badge/dependencies-0-blue)

**loops** is an open-source, multi-agent workflow plugin for **Claude Code, GitHub Copilot, Codex, Cursor, Gemini CLI, OpenCode, Amp, Windsurf and Cline**. One ticket (GitHub, GitLab, Jira, Linear or plain text) becomes a team of specialist AI agents: they research, plan, build in parallel, re-test each other's work, prove each requirement by hand and open the PR. It is built to be the best multi-agent workflow for one job: shipping backend changes you can trust without re-checking them yourself. It supports 15 backend languages.

<p align="center">
  <img src="docs/media/loops-demo.svg" alt="Sample loops run in a terminal: /loops:work #42 goes through intake, research, plan (reviewer sends round 1 back, approves round 2), parallel build, QA catches a failing claim and sends it back until verified, the gate passes with the race detector and 84% new-code coverage, all four acceptance criteria are proven, and PR #43 opens with proofs after 31 minutes and zero questions" width="100%">
</p>
<p align="center"><sub>A sample run. Video: <a href="docs/media/loops-demo.mp4">MP4</a> · <a href="docs/media/loops-demo.webm">WebM</a></sub></p>

## Quick start (Claude Code)

```sh
/plugin marketplace add arxdsilva/ai-loops
/plugin install loops@ai-loops
```

Then, in your repository:

```sh
/loops:init       # writes .loops/config.json, adds rules to AGENTS.md, checks dependencies
/loops:work #42   # or a Jira/Linear key, a GitLab URL, or plain text: "add partial refunds"
```

That's it. Nothing else to install: the plugin downloads one checksummed `loops` binary on first use. You need `gh` or `glab` for tickets and PRs.

<details>
<summary><b>Copilot, Codex, Cursor, Gemini CLI, OpenCode, Amp, Windsurf, Cline</b></summary>

```sh
copilot plugin install arxdsilva/ai-loops                         # GitHub Copilot CLI (VS Code: "Chat: Install Plugin from Source")
codex plugin marketplace add arxdsilva/ai-loops                   # Codex, then install loops in the desktop app
curl -fsSL https://raw.githubusercontent.com/arxdsilva/ai-loops/main/install.sh | sh -s -- --target codex
gemini extensions install ./ai-loops/platforms/gemini             # Gemini CLI, after cloning this repo
curl -fsSL https://raw.githubusercontent.com/arxdsilva/ai-loops/main/install.sh | sh -s -- --target opencode   # or amp, windsurf, cline
```

Cursor: import `arxdsilva/ai-loops` from the marketplace. Every platform, its capabilities and how to update: [docs/GUIDE.md](docs/GUIDE.md#install).
</details>

## How it works

<p align="center">
  <img src="docs/media/loops-workflow.svg" alt="The loops workflow: Research, Plan, Build, Verify, Prove and Reflect, with review loops between plan and reviewer and between engineers and QA, ending in a pull request with proofs" width="100%">
</p>

1. **Research.** Agents read the web, best practices, your code and your machine's limits before anything is decided.
2. **Plan.** A planner writes the plan and a second agent challenges it until it holds up.
3. **Build.** An architect fixes the shared contracts, then specialist engineers work in parallel, each in its own files.
4. **Verify.** A separate QA agent per engineer re-runs every claim and sends failures back with a reproduction. Then the gate runs: build, lint, architecture rules, tests under the race detector, ≥70% coverage of new code.
5. **Prove.** A product reviewer tries every acceptance criterion by hand and opens the PR with the evidence attached.
6. **Reflect.** Lessons are saved for the next run.

One **master agent** (your main session) runs all six. It keeps the scope in a charter, re-reads the run's recorded state at every phase, and decides instead of asking. Each decision is recorded and listed in the PR. You are asked only where you opt in, for example `--ask plan,pr`.

## What you get

**A PR you can review in minutes.** Each acceptance criterion links to its proof, each engineer's claims show who verified them and in which round, and every decision the agents took is listed.

<p align="center">
  <img src="docs/media/loops-report-proof.png" alt="loops run report: verdict table where a claim FAILED in round 1 and was VERIFIED in round 2, acceptance criteria AC-1 to AC-4 each linked to a proof file, and the gate table with build, format, vet, lint, arch, test and coverage all passing with 84.2% new-code coverage" width="85%">
</p>

**A run you can follow.** A status line, a live pane and `loops watch` show the phase, the agents running and their QA state. `loops report` writes one offline HTML file with the whole timeline.

<p align="center">
  <img src="docs/media/loops-report-lanes.png" alt="loops run report: phase durations from intake to reflect, and one swim lane per agent (researchers, planner, plan reviewer, architect, domain and persistence engineers) with model, tokens and QA status" width="85%">
</p>

<sub>Screenshots of a sample run's report (`loops report`).</sub>

## Why engineers trust it

AI code that is "almost right" is the top frustration of 66% of developers, and 45% say debugging AI code takes longer than writing it ([Stack Overflow 2025](https://survey.stackoverflow.co/2025/ai)). Google's DORA 2025 report finds AI raises throughput and instability together, so checks on what comes out matter more ([InfoQ summary](https://www.infoq.com/news/2025/09/dora-state-of-ai-in-dev-2025)). loops is designed around that:

- **Claims are re-tested, not trusted.** The engineer who wrote the code never verifies it. [ROLES.md](ROLES.md)
- **The gate cannot be talked past.** New code needs unit tests with ≥70% coverage, every test runs under the language's race detector or its substitute, and the session cannot end while sources changed since the last green gate. [specs/TESTING.md](specs/TESTING.md)
- **Make it work, then make it right.** Failing test first, data modelled before code, small ordered commits. After two failed fixes on one premise, the agent must name and drop that premise, adapted from Robert C. Martin's [ub-mode](https://github.com/unclebob/ub-mode). [specs/ENGINEERING.md](specs/ENGINEERING.md)
- **Your rules win.** AGENTS.md, CLAUDE.md, linter configs and your existing layout beat loops' defaults, and every override is reported on the PR.
- **Nothing hidden.** Every tool call, verdict, loop-back and decision lands in `.loops/runs/<id>/events.jsonl`.

## For product managers and leads

- **Acceptance criteria are the contract.** Each one is proven by hand before the PR opens. Unclear tickets get the narrowest reading, written down as an assumption you can see.
- **Predictable phases, visible progress.** The same phases every time, with a live status and a shareable HTML report.
- **Less back-and-forth.** No questions mid-run unless you ask for a checkpoint (`--ask plan` to approve the plan, `--ask pr` before the PR).
- **Cost under control when you want it.** Optional caps on tokens, agents and roles per phase (`--budget implement=2M`).

## What is the best multi-agent coding workflow? How loops compares

The right tool depends on the job. For turning tickets into PRs you can merge with confidence, these are the differences that matter (built in by default, as of November 2026, from each project's own docs; [corrections welcome](https://github.com/arxdsilva/ai-loops/issues)):

| | **loops** | One agent session (Claude Code, Copilot coding agent) | Spec and persona frameworks (Spec Kit, BMAD-METHOD) | Orchestrators (claude-flow, oh-my-claudecode) |
|---|---|---|---|---|
| Ticket in, PR out, end to end | ✅ | partial | — | partial |
| Plan challenged by a second agent | ✅ | — | partial | partial |
| Separate agent re-tests every claim, with repro | ✅ | — | add-on | partial |
| Each acceptance criterion proven, proof in the PR | ✅ | — | — | — |
| Hard gate: new-code coverage + race detector | ✅ ≥70% | — | — | — |
| One master agent holds scope; asks only where you opt in | ✅ | — | — | partial |
| Parallel specialists with disjoint file scopes | ✅ | — | — | ✅ |
| Agent tools supported | 9 | 1 | many | 1–2 |

## Languages

Python, TypeScript/JavaScript, Java, C++, C#, PHP, Go, Ruby, Swift, Kotlin, Rust, Scala, Elixir, Erlang and Clojure. Each has its own engineer role, rule book, gate checks, coverage format and race policy, detected automatically (polyglot repos work). Every language was verified end to end in its official Docker image. [Details](docs/GUIDE.md#languages)

## Good to know

- **It spends more tokens than a single agent.** Several agents research, review and re-test. Set budgets if that matters to you.
- **It is built for backend services.** The proof step drives your service (HTTP, CLI, tests), not a browser UI.
- **Some tools get less.** Resumable subagents, hooks and the live pane depend on the platform; Windsurf and Cline run in solo mode. [Capability matrix](docs/GUIDE.md#capability-matrix-condensed)
- **It never pushes to main, force-pushes or merges.** It works on a branch and opens a PR (a draft when something is left open).

## FAQ

### Is loops free?
Yes. MIT licensed. You pay only for your own agent's model usage.

### How is it different from Claude Code subagents?
Subagents are a building block. loops is the workflow on top: the roles, the order, the review loops, the gate, the proofs and the record of each run.

### Does it work without GitHub?
Yes. Tickets can come from GitLab, Jira, Linear or plain text, and PRs can be GitLab merge requests.

### Can I approve the plan or the PR myself?
Yes. Add `--ask plan`, `--ask pr` or `--ask all`, put `"ask"` in `.loops/config.json`, or just say so in the request.

### Does it follow my team's conventions?
Yes. Project rules always win, and each time one overrides a loops rule it is logged and posted as a PR comment.

### Which models does it use?
Planning, plan review, architecture and product review run on the strongest model your tool offers; engineers and QA on a faster one. An engineer whose work fails twice is relaunched one model tier up. [specs/WORKFLOW.md](specs/WORKFLOW.md)

## Learn more

- [docs/GUIDE.md](docs/GUIDE.md): install on every platform, updating, hooks, parallelism, limits, layout.
- [specs/WORKFLOW.md](specs/WORKFLOW.md): the full workflow spec. [ROLES.md](ROLES.md): the 32 agent roles.
- [specs/PLATFORMS.md](specs/PLATFORMS.md): how each agent tool is supported.
- [llms.txt](llms.txt): a map of this repo for AI assistants.

Contributions and issues are welcome. If loops saved you a review, a ⭐ helps others find it.

## License

[MIT](LICENSE) © 2026 Arthur Silva
