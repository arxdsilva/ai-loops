# loops compared with other multi-agent coding workflows

As of **November 2026**. Each cell comes from the tool's own README or docs, linked below. Where a page did not say, the cell reads *not documented*, not "no". Corrections are welcome as [issues](https://github.com/arxdsilva/ai-loops/issues), with a link to the page that shows it.

## Which tools are compared

Only tools that cover most of what loops does: a planned, multi-agent change with an implement → verify loop. We read about fifteen. These five qualified:

- **[GSD Core](https://github.com/open-gsd/gsd-core)** (successor to the archived [get-shit-done](https://github.com/gsd-build/get-shit-done)). It is the closest in shape: researchers, a plan checker, wave-parallel executors and a verify → fix loop.
- **[Superpowers](https://github.com/obra/superpowers)**: skills for brainstorm, plan, test-first build and per-task review.
- **[oh-my-claudecode](https://github.com/Yeachan-Heo/oh-my-claudecode)**: team plan → exec → verify → fix pipelines with hooks and model routing.
- **[BMAD-METHOD](https://github.com/bmad-code-org/BMAD-METHOD)**: plan and build with independent reviewers and a ticket tree.
- **[gstack](https://github.com/garrytan/gstack)**: plan reviews, real-browser QA and `/ship`.

Left out because they do a different job:
- Spec Kit, OpenSpec, Taskmaster and Agent OS write specs and break down tasks, with no verify loop.
- Ruflo (formerly claude-flow), wshobson/agents, SuperClaude and ECC are catalogues or swarm platforms with no orchestrated implement → verify loop.
- CCPM decomposes GitHub issues into tasks, with no verify step.
- Hosted agents (Copilot cloud agent, Codex cloud, Devin, Factory) turn issues into PRs, but their docs show no second-agent plan review or independent re-verification.

## Criteria

| Mark | Meaning |
|---|---|
| ✅ | Built in and on by default |
| opt-in | Ships with the tool, off until configured |
| prompt-only | The model is told to do it; nothing checks that it did |
| partial | Part of it, explained in the notes below |
| — | The docs say it doesn't, or show no such feature |
| not documented | We found nothing either way |

## The table

| | loops | GSD Core | Superpowers | oh-my-claudecode | BMAD-METHOD | gstack |
|---|---|---|---|---|---|---|
| Ticket in (GitHub, GitLab, Jira, Linear), PR out | ✅ | partial¹ | —² | partial³ | partial⁴ | partial⁵ |
| A second agent reviews the plan before code | ✅ | ✅ plan checker | not documented⁶ | ✅ critic | partial⁷ | ✅ `/autoplan` |
| Parallel implementers with disjoint file ownership | ✅ checked by `loops plan check` | ✅ waves and worktrees | —⁸ | partial⁹ | partial¹⁰ | partial¹¹ |
| A separate agent re-runs each implementer's claims | ✅ | partial¹² | partial¹³ | partial¹⁴ | partial¹⁵ | partial¹⁶ |
| Each acceptance criterion proven against the running app, proof in the PR | ✅ | opt-in¹⁷ | — | — | — | partial¹⁸ |
| Gates enforced by code, not the prompt (coverage floor, race detector, can't finish over a red gate) | ✅ | partial¹⁹ | prompt-only | partial²⁰ | prompt-only | partial²¹ |
| Test-first | ✅ | opt-in | ✅ (prompt-only) | opt-in (prompt-only) | not documented | partial |
| Every tool call and verdict logged, plus an HTML report | ✅ | partial²³ | partial²⁴ | partial²⁵ | partial²⁶ | partial²² |
| Cost controls | ✅ triage, model tiers, budgets | ✅ model profiles, routing | partial: model per task | ✅ model routing, opt-in budget | partial: size routing | partial: spend caps (Capy only) |
| Evals of its own agents | ✅ `loops eval` | not documented | ✅ | not documented | ✅ `bmad-eval` | ✅ |
| Language rules and gates | ✅ 15 languages | — | — | — | — | — |
| Agent tools supported | 10 | 15+ | 15 | Claude Code, other CLIs as workers | several | Claude Code, 10 experimental |
| GitHub stars (2026-11-24) | new (Nov 2026) | 11.8k (64.1k before the move) | 325.8k | 41.0k | 55.7k | 142.6k |

## Where the others are ahead

- **Adoption.** Every tool here has a far larger community than loops, which launched in November 2026. Superpowers and gstack have the most users and battle-testing.
- **Breadth of agent tools.** Superpowers runs on 15 and GSD Core on more; loops on 10.
- **Product planning.** BMAD's PRD and ticket tree, and GSD's phases and milestones, go further upstream than loops, which starts from a ticket.
- **Browser and UI work.** gstack's `/qa` drives a real browser. loops proves acceptance criteria against backend services (HTTP, CLI, tests), not a UI.
- **Cost per task.** A single agent, or Superpowers' inline mode, spends fewer tokens than any multi-agent run. loops spends more to buy independent verification. Its triage shrinks small tasks, and the cost section below has the measured numbers.

## Cost

loops measures cost from the transcripts of real runs, not from estimates. [`evals/cost.py`](../evals/cost.py) prices every model call of a run at its own model's rates: uncached input, cache writes, cache reads and output. It counts the lead session and every agent, then reports the cost per role, per run and per triage tier. Run it on your own repo: `python3 -I evals/cost.py <repo>`.

From 19 runs in one Go repository (November 2026, Claude Opus 5.5 lead, before the turn-cutting changes in `specs/CONTEXT_DISCIPLINE.md` §16a):

| Triage tier | Runs | Median per run |
|---|---|---|
| trivial | 1 | $1.20 |
| small | 2 | $14.83 |
| standard | 15 | $33.28 |

Where the money went:
- **Re-reading context: 49%.** Every tool call re-reads the agent's context.
- **New tool output: 30%.** Each result is written into the context.
- **Model output: 22%.**

So those changes cut turns rather than prompt size: batched lookups, capped output, and the planner working from the code researcher's brief instead of re-reading the code. Opus-tier roles were 82% of the spend.

Other tools' published figures are self-reported:
- Superpowers 6 reports builds "up to 60% cheaper" than its earlier release, and one fixture built for $6.24 with a candidate configuration it was still evaluating ([post](https://blog.fsck.com/2026/06/15/Superpowers-6/)).
- oh-my-claudecode says model routing "saves 30-50% on tokens", with no method given.
- GSD Core, BMAD and gstack publish no per-task cost.

## Notes and sources

GSD Core: [README](https://github.com/open-gsd/gsd-core), [the phase loop](https://github.com/open-gsd/gsd-core/blob/main/docs/explanation/the-phase-loop.md), [verify and ship](https://github.com/open-gsd/gsd-core/blob/main/docs/how-to/verify-and-ship.md), [configuration](https://github.com/open-gsd/gsd-core/blob/main/docs/CONFIGURATION.md), [features](https://github.com/open-gsd/gsd-core/blob/main/docs/FEATURES.md).
Superpowers: [README](https://github.com/obra/superpowers), [subagent-driven development](https://github.com/obra/superpowers/blob/main/skills/subagent-driven-development/SKILL.md).
oh-my-claudecode: [README](https://github.com/Yeachan-Heo/oh-my-claudecode), [reference](https://github.com/Yeachan-Heo/oh-my-claudecode/blob/main/docs/REFERENCE.md).
BMAD-METHOD: [README](https://github.com/bmad-code-org/BMAD-METHOD), [build a change](https://github.com/bmad-code-org/BMAD-METHOD/blob/main/docs/build/build-a-change.md), [autonomous loops](https://github.com/bmad-code-org/BMAD-METHOD/blob/main/docs/build/autonomous-development-loops.md), [ticket tree](https://github.com/bmad-code-org/BMAD-METHOD/blob/main/docs/plan/set-up-the-ticket-tree.md).
gstack: [README](https://github.com/garrytan/gstack).

1. GSD Core: `/gsd-ship` pushes and opens the PR; starting from a GitHub, Linear or Jira issue is a documented recipe, not a command.
2. Superpowers: the finishing skill offers merge, PR, keep or discard; there is no tracker intake.
3. oh-my-claudecode: PR skills (`pr`, `babysit`, `fix-ci`); no ticket-to-PR run.
4. BMAD: `bmad-build` accepts an issue URL and offers to push and open the PR when asked; its docs say its tracker store options (GitHub Issues, Jira, Linear) "still need a lot of testing".
5. gstack: `/ship` runs the tests and opens the PR; no tracker intake.
6. Superpowers: the plan goes to the user for sign-off.
7. BMAD: the human approves flagged plans; the light path skips the plan.
8. Superpowers: "Never dispatch multiple implementation subagents in parallel (conflicts)." (subagent-driven development skill). A separate skill runs parallel agents on independent failures, each scoped to a test file or subsystem.
9. oh-my-claudecode: parallel team workers and worktrees; no file-scope rules documented.
10. BMAD: "Exactly one ticket per invocation"; independent epic streams can run in parallel through an orchestrator, with no file-ownership rule.
11. gstack: `/captain` (Capy only) starts lanes on disjoint files; no ownership check documented.
12. GSD Core: a verifier checks requirement coverage and a UAT walk-through runs with the user; failures spawn debug agents and fix plans.
13. Superpowers: a fresh reviewer per task, but the controller is told: "Do not ask a reviewer to re-run tests the implementer already ran on the same code".
14. oh-my-claudecode: a separate `verifier` agent confirms completion before the loop exits; whether it re-runs the implementer's tests is not documented.
15. BMAD: independent reviewer subagents review the code; re-running the tests is not documented.
16. gstack: `/qa` reproduces, fixes and re-verifies; a separate agent is not documented.
17. GSD Core: `workflow.live_dom_uat` and `workflow.deep_verify` (a behaviour verifier for UI phases), both off by default.
18. gstack: real-browser QA; `/ship` puts the tests it ran and the review on the PR, not a proof per acceptance criterion.
19. GSD Core: a build and test gate runs `workflow.test_command` after merge, and some hooks block (commit checks, a secret-read guard); most hooks are advisory. No coverage floor, race gate or end-of-session gate is documented.
20. oh-my-claudecode: Stop hooks keep autopilot running until verified; no coverage or race gate.
21. gstack: hooks guard destructive commands (`/careful`, `/freeze`, `/guard`); the test gate in `/ship` is skill text.
22. gstack: a decision trail (`gstack-decision-trail`) and an evidence ledger (`gstack-evidence`); no per-call log.
23. GSD Core: `STATE.md`, summary and verification files; no per-call log.
24. Superpowers: a per-plan ledger.
25. oh-my-claudecode: session and replay logs under `.omc/`.
26. BMAD: the plan file records the run, its status and its risk score.

loops' own column: [WORKFLOW](../specs/WORKFLOW.md) (§2 phases, §6 gates, the Stop gate and the end gate), [TESTING](../specs/TESTING.md), [evals](../evals/README.md), [GUIDE](GUIDE.md).
