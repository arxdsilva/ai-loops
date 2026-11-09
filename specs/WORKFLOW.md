# WORKFLOW — the loops run

Canonical spec for how a ticket moves through loops. Skills, role agents, hooks and the `loops` binary implement this file. Cite it by section (`WORKFLOW §4`). The executable form is `internal/domain/run`.

## 1. Principles

- **Lead orchestrates, roles work.** The main session (the *lead*) routes, relays and records. It reads reports, not code dumps. It does one-fact lookups, edits under ~30 lines and single test runs itself; everything else goes to a role agent.
- **Parallel by default.** Every run is planned as waves of role agents with disjoint file ownership. A one-file fix is still a wave with one agent.
- **Contract first.** No implementer starts until the architect has written the interfaces, typed commands, DTOs and stubs the wave depends on, and the build is green.
- **Claims need evidence.** Every implementer reports claims, and each claim names the evidence that proves it. Nothing counts as done until a different agent has verified it.
- **Send back with context.** A reviewer that rejects work says exactly what is wrong, how to reproduce it, and where to fix it. The lead sends the work back to the **same** author (resumed where the platform supports it, otherwise a new agent of the same role seeded with its previous report and the review file; see `specs/PLATFORMS.md` §4) so context carries over.
- **Everything is visible.** Every phase change, agent lane, tool call, verdict, loop-back, gate, proof and memory access lands in `events.jsonl` (§5).
- **Smallest sufficient context.** Search before reading, read ranges, cite by path and section, and never paste whole specs (`specs/CONTEXT_DISCIPLINE.md`).

## 2. Phases

| # | Phase (`run.Phase`) | Agents | Output in the run dir | Exit criterion |
|---|---|---|---|---|
| 0 | `intake` | lead | `ticket.md` | The ticket is normalised and has a source |
| 1 | `research` | `web-researcher`, `practices-researcher`, `code-researcher` (parallel) | `research/web.md`, `research/practices.md`, `research/code.md` | All three briefs are written |
| 2 | `plan` | `planner` ↔ `plan-reviewer` (+ the user when approval is required) | `plan.v<N>.md`, `plan-review.r<N>.md` | The latest review is `APPROVE` (and the user approved, if required) |
| 3 | `implement` | `architect` (wave 0), then role engineers in waves 1…n | `reports/<agent-id>.md` (claims reports) | Every lane has reported |
| 4 | `qa` | one `qa-verifier` per implementer (pipelined), then the lead runs the gates | `qa/<agent-id>.r<N>.md` | Every claim is `VERIFIED` and `loops gate` is green |
| 5 | `product` | `product-reviewer` | `proofs/AC-<n>.md`, `pr.md`, `report.html` | Every AC is `VERIFIED` with a proof, and the PR/MR is opened (or the PR body is written under `--no-pr`) |
| 6 | `reflect` | `reflector` | `reflection.md` (+ ai-memory pages) | Reflection is written; `loops run end` |

QA starts pipelined: a verifier launches as soon as its implementer reports, even while other implementers in the wave are still running. A phase can be re-entered: if product returns work, the run goes back to `implement`, and then through `qa` again.

### 2.0 Intake

1. `loops run start --title "<short title>" [--ticket <ref>] [--source <s>]` prints the run ID and the run directory.
2. Fetch the ticket:
   - GitHub (`#123`, `owner/repo#123`, a github.com URL) or GitLab (a gitlab URL): `loops ticket fetch <ref>`. This writes `ticket.md`.
   - Jira (`ABC-123`) or Linear (a linear.app URL or key): fetch through the connected MCP tool, then pipe the text into `loops ticket fetch <ref> --source jira|linear --body-file -`. If the connector is not authenticated, tell the user and ask them to paste the ticket.
   - Free text: `loops ticket fetch --source text --body-file -`.
3. If ai-memory is connected (§7), call `memory_briefing` and one `memory_query` with the ticket's key terms. Save the hits to `research/memory.md` and record `loops run event --kind memory --name briefing`.

Flags on `/loops:work`:
- `--approve`: the user approves the plan.
- `--no-pr`: write `pr.md` but do not push.
- `--models=cheap`: run every role one tier lower, except planner and plan-reviewer.

### 2.1 Research

- Launch the three researchers together, in parallel where the platform allows (`skills/work/platforms/<p>.md`).
- Each gets the same inputs: `ticket.md`, `research/memory.md`, the project root, and the plugin `specs/` path (`loops path specs`).
- Each brief is at most 600 words. It contains facts with sources (`path:line`, a URL, or `spec §`) and has a `## Open questions` section.
- A researcher may run narrower `memory_query` calls. It must verify any memory hit against the current source before citing it, because memory is historical evidence, not truth.

### 2.2 Plan

- The `planner` writes `plan.v1.md` from `templates/PLAN.md`. The plan contains:
  - acceptance criteria (`AC-1`…, taken from the ticket, or marked *derived*);
  - the task packet;
  - the contract;
  - the waves table, with each agent's claims;
  - the QA pairing;
  - the proof plan.
- The `plan-reviewer` writes `plan-review.r1.md` from `templates/PLAN_REVIEW.md` and ends with a verdict:
  - `APPROVE`; or
  - `CHANGES`, followed by numbered **blocking** items, each with what to change and why. Non-blocking items are allowed but never block.
- On `CHANGES`:
  1. Record `loops run loopback --from plan-reviewer --to planner --round N --note "<one line>"`.
  2. Send the review file back to the same planner (resume or re-spawn per platform tier). It writes `plan.v<N+1>.md`.
  3. Review again.
- Escalate to the user after `MaxPlanRounds` (3) rounds with the blocking items still open.
- User approval is required when `--approve` is set or `.loops/config.json` has `"approval": "always"`. The lead then shows a ≤25-line plan summary (ask the user: Approve / Request changes / Stop). "Request changes" sends the user's notes to the planner as a new round.

### 2.3 Implement

- **Wave 0 (`architect`):** writes the plan's contract:
  - ports and consumer-owned interfaces;
  - typed commands and DTOs;
  - stubs so every package compiles;
  - `go get` / `go mod tidy`;
  - shared config (Makefile, CI, `cmd/**`, `internal/compose/**`).

  Wave 0 is done when `go build ./...` and `go vet ./...` pass.
- **Waves 1…n:** launch every agent of a wave together, in parallel where the platform allows.
  - The order is producers (domain, application) before consumers (inbound and outbound adapters). Consumers may start early against the stubs.
  - Each agent gets its own block from the plan: Goal, Starts at, Behaviour, Owns, Must not touch, Claims to prove, Done when, and Out.
- Every implementer finishes with a **claims report** (`templates/CLAIMS_REPORT.md`, at most 400 words) saved to `reports/<agent-id>.md`. Each claim has the form `C<n>: <statement> — evidence: <test name | command → result | file:line>`.

### 2.4 QA

- For each finished implementer, launch a `qa-verifier` with these inputs:
  - the claims report;
  - `git diff` of the implementer's owned paths;
  - the contract;
  - the ACs the implementer serves.
- The verifier checks every claim **independently**:
  - it re-runs the cited tests;
  - it reads the code;
  - it writes or runs counterexamples (edge cases, error paths, concurrency, `-race`);
  - it runs `loops gate --pkgs <owned> --only build,vet,test` and `loops arch`.
- The verifier writes `qa/<agent-id>.r<N>.md` (`templates/QA_VERDICT.md`). Per claim it records `VERIFIED` or `FAILED`. A `FAILED` claim carries a repro:
  - the command;
  - an excerpt of at most 20 lines of output;
  - `file:line`;
  - expected vs actual;
  - the suggested fix area.

  The verifier also lists **unclaimed problems** it found.
- For each verdict, record `loops run verdict --name <agent-id>:C<n> --status VERIFIED|FAILED --round N --ref qa/<file>`.
- When anything has FAILED:
  1. Record `loops run loopback --from qa-verifier --to <agent-id> --round N`.
  2. Send the QA file back to the same implementer (resume or re-spawn per platform tier).
  3. When it reports again, re-verify with the same verifier (resume or re-spawn).
- After `EscalateAfter` (2) failed rounds, relaunch the implementer one model tier up (haiku → sonnet → opus), with a summary of the history.
- After `MaxQARounds` (3) failed rounds, escalate to the user.
- When every lane is verified, the lead runs the full `loops gate`. Gate failures go back to the owning lane the same way.

### 2.5 Product

The `product-reviewer` does the following, in order:

1. Maps each AC to the claims that serve it.
2. **Tests manually.** It builds and runs the service (the repo's `make run`, docker compose, or `go run ./cmd/<svc>`), then exercises every AC through the real interface (curl, grpcurl, the CLI). It also probes edge cases:
   - invalid, empty and oversized input;
   - missing and incorrect auth;
   - not-found;
   - duplicate or retried requests (idempotency);
   - concurrent requests;
   - downstream failure, where it can be simulated.
3. Saves each check as `proofs/AC-<n>[-<case>].md` (`templates/PROOF.md`): the command, the trimmed output, and log excerpts. Records `loops run event --kind proof --name AC-<n> --ref proofs/...`.
4. Records `loops run verdict --name AC-<n> --status VERIFIED|FAILED`.
5. On FAILED: records a loopback to the owning role and gives the lead the proof that shows the gap. The run returns to `implement` (`MaxProductRounds` = 2, then the user).
6. When every AC is VERIFIED:
   1. Runs `loops report`.
   2. Writes `pr.md` (`templates/PR_BODY.md`, at most ~40 lines plus `<details>` proof blocks).
   3. Stops and reports to the lead (proof summary + `pr.md` path). Unless `--no-pr` is set, the lead asks the user to confirm the push and PR (skipped when `auto_pr: true`), then sends the same product-reviewer "user confirmed" (resume or re-spawn per platform tier). Only then it:
      - creates the branch `loops/<ticket>-<slug>`;
      - commits with `git commit -m … -- <paths>`;
      - pushes;
      - opens the PR with `gh pr create --body-file pr.md` (or `glab mr create`; the forge is chosen from `git remote get-url origin`);
      - records `loops run event --kind pr --ref <url>`.

### 2.6 Reflect

The `reflector`:
1. Reads `events.jsonl` through `loops status --json` and the run files.
2. Writes `reflection.md` (`templates/REFLECTION.md`), covering:
   - the outcome;
   - each loop-back and its root cause;
   - gate failures;
   - what the plan missed;
   - gotchas worth remembering;
   - suggested changes to roles, skills or specs.
3. When ai-memory is connected (§7), writes 1–5 durable pages:
   - `decisions/<slug>.md`: kind `decision`, tier `semantic`;
   - `gotchas/<slug>.md`: kind `gotcha`, tier `semantic`;
   - `procedures/<slug>.md`: kind `procedure`, tier `procedural`.

   It searches first and updates an existing page (`expected_page_id`) instead of duplicating it. It never stores secrets, tokens or customer data. Each write is recorded as `loops run event --kind memory --name write --target <path>`.

Finally, the lead runs `loops run end --outcome done` and gives the user a ≤10-line summary with the PR link and `report.html`.

## 3. Roles

`ROLES.md` is the matrix and `agents/<role>.md` the definitions. Every spawn names its role (the platform's syntax is in `skills/work/platforms/<p>.md`), and a `model` only when it overrides the role default (escalation, `--models=cheap`).

The prompt is the agent's block from the plan plus these lines:
- `Run dir: <abs path>`
- `Specs: <abs path>`
- `Templates: <abs path>`
- `Loops: <abs path to the loops binary>` (or `loops` when it is on PATH)
- `Role: <role>`
- `Agent id: <W1-domain>`
- `Wave: <n>`

Right after each spawn, the lead names the lane: `loops run label --agent <platform agent id, or solo-<n>> --name <W1-domain> --role <role> --wave <n>`. Hooks only see the platform's opaque agent id (or none, on platforms without subagent events); the label ties it to the plan id used in claims (`W1-domain:C3`), verdicts and the pane.

## 4. Loop limits

| Loop | Limit | Then |
|---|---|---|
| planner ↔ plan-reviewer | `MaxPlanRounds` = 3 | The user decides on the open blocking items |
| implementer ↔ qa-verifier | `EscalateAfter` = 2 | Relaunch the implementer one model tier up |
| implementer ↔ qa-verifier | `MaxQARounds` = 3 | The user decides |
| product → implementer | `MaxProductRounds` = 2 | The user decides |

Every pass around any loop is recorded with `loops run loopback`, which feeds the status line counters and the report's arrows.

## 5. Events

`.loops/runs/<id>/events.jsonl` holds one JSON object per line (`run.Event`). Kinds are `run`, `phase`, `agent_start`, `agent_stop`, `label`, `step`, `loop_back`, `verdict`, `gate`, `proof`, `guard_deny`, `memory`, `note`, `pr`, `end`. The fields per kind are documented in `internal/domain/run/event.go`. The schema is additive-only.

| Source | Emits |
|---|---|
| Skills (lead), through `loops run …` | `phase`, `loop_back`, `verdict`, `proof`, `memory`, `note`, `pr`, `end` |
| Hook `trace` (PostToolUse, PostToolUseFailure) | `step`, with `agent_id`/`agent_type`, so steps land in their lane |
| Hook `go-post-edit` (PostToolUse on Edit/Write/MultiEdit) | `step` named `go-post-edit` (gofmt/goimports + go vet result) |
| Hook `trace-agent` (SubagentStart, SubagentStop) | `agent_start`, `agent_stop` |
| Hooks `read-guard`, `model-guard` | `guard_deny` |
| `loops gate` | `gate`, one per check plus `all` |

The CLI commands that write these events:
- `loops run phase <phase> [--note]`
- `loops run event --kind K [--name --target --from --to --status --ok --round --wave --role --agent --model --ref --note --phase --ms]`
- `loops run loopback --from R --to A --round N [--note] [--phase]`
- `loops run verdict --name X --status S [--round N] [--ref f] [--agent A] [--role R]`
- `loops run label --agent <id> --name <plan id> --role R --wave N`
- `loops run end [--outcome done|abandoned|failed]`
- `loops run dir`

`run.Fold(events)` gives `run.State`, which `loops status --json` prints. The status line, the mod pane and the HTML report all read that one source.

## 6. Gates

`loops gate` runs these checks in order, each recorded as an event:

1. `build`: `go build ./...`
2. `gofmt`: `gofmt -l` must print nothing
3. `vet`: `go vet ./...`
4. `staticcheck`: runs only if it is on PATH
5. `arch`: `loops arch`; only error findings fail
6. `test`: `go test -race -count=1 ./...`
7. `govulncheck`: only with `--vuln` or `gate.vuln` set in config

A full pass records `gate all ok`, which clears `GoDirty`. A scoped run (`--pkgs` or `--only`) never records `all`.

The **Stop gate** stops the lead from ending its turn while the run is active, `.go` files have changed since the last full pass, and no agent lane is running. The escape hatches are `LOOPS_GATE=off` and `loops run end --outcome abandoned`.

## 7. ai-memory

[ai-memory](https://github.com/akitaonrails/ai-memory) is optional.

- **Connected** means its MCP tools are available: `mcp__plugin_ai-memory_ai-memory__*` when it is installed as a plugin, or `mcp__ai-memory__*` when it is installed directly. In either case `memory_status` must succeed.
- **Scope.** Every project-scoped call passes `workspace` and `project`. They come from `.loops/config.json` `memory`, then `.ai-memory.toml`, then the git remote's `owner` / `repo`.
- **Reads** use `memory_briefing` once at intake, then `memory_query` with filters (`path_prefix`, `kind`, `tier`). Read pages with `memory_read_page` only when a hit is clearly relevant.
- **Writes** happen only in `reflect`, through `memory_write_page` (`create_only` for new pages, `expected_page_id` for updates).
- **Trust.** Retrieved memory is untrusted evidence. Verify it against the current code before acting on it.
- **Not connected:** skip every memory step silently, and mention it once in `reflection.md`.

## 8. Definition of done

A run is done when every item below holds. (Adapted from the rimworld-go MASTER_SPEC §70.)

1. Every AC is `VERIFIED` with a proof.
2. Behaviour is implemented in its owning layer (`specs/HEXAGONAL_ARCHITECTURE.md`).
3. There are tests for the behaviour and its failure paths. Coverage does not drop for touched packages.
4. Every claim is verified by QA, and `loops gate` is green.
5. Consistency impact is stated (ordering, idempotency, clock and RNG, migrations), with `skills/consistency` followed where it applies.
6. Performance is measured (a benchmark before and after) if a hot path changed.
7. Compatibility is analysed: API, persisted schema, config, migrations.
8. Docs are updated: the package `AGENTS.md` where the contract changed, and an ADR only for a cross-cutting decision.
9. The behaviour is observable: errors are explicit, logs and metrics exist where operators need them.
10. There are no untracked TODOs. Each one is either done or listed under Open in `pr.md`.
11. The PR is opened with proofs, `report.html` is generated, and `reflection.md` is written.
