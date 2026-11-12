# WORKFLOW — the loops run

Canonical spec for how a ticket moves through loops. Skills, role agents, hooks and the `loops` binary implement this file. Cite it by section (`WORKFLOW §4`). The executable form is `internal/domain/run`.

## 1. Principles

- **One master agent owns the run.** The main session, the *lead*, is the run's master agent. It runs every phase from intake to `loops run end` and is the only one that moves the run between phases, spawns and stops agents, records decisions and talks to the user. It holds the scope in `charter.md` and re-anchors on the run's recorded state at every phase change, after a compaction and in a resumed session, never on its memory (§2.7).
- **Lead orchestrates, roles work.** The lead routes, relays and records. It reads reports, not code dumps. It does one-fact lookups, edits under ~30 lines and single test runs itself; everything else goes to a role agent.
- **Autonomous by default.** The lead decides; it does not ask. The user is asked only at the checkpoints they opt into, per phase (§2.7). Every decision the lead takes in the user's place is recorded and shows in the report and the PR.
- **Parallel by default.** Every run is planned as waves of role agents with disjoint file ownership. A one-file fix is still a wave with one agent.
- **Contract first.** No implementer starts until the architect has written the interfaces, typed commands, DTOs and stubs the wave depends on, and the build is green.
- **Claims need evidence.** Every implementer reports claims, and each claim names the evidence that proves it. Nothing counts as done until a different agent has verified it.
- **Send back with context.** A reviewer that rejects work says exactly what is wrong, how to reproduce it, and where to fix it. The lead sends the work back to the **same** author (resumed where the platform supports it, otherwise a new agent of the same role seeded with its previous report and the review file; see `specs/PLATFORMS.md` §4) so context carries over.
- **Project rules win.** The target repository's own rules always take precedence over loops' specs, skills and templates. That means its AGENTS.md, CLAUDE.md, `.cursor/rules`, CONTRIBUTING, linter configs and established architecture. Any agent that follows a project rule instead of a loops rule records it immediately: `loops run override --rule "<loops rule>" --project "<project rule, file:line>" --note "<what was done>"`. The reflector posts the list as a PR comment (§2.6).
- **Everything is visible.** Every phase change, agent lane, tool call, verdict, loop-back, gate, proof and memory access lands in `events.jsonl` (§5).
- **Runs leave no trace.** Each run builds with its own build caches and tmp dirs in a private scratch folder outside the project (`$TMPDIR/loops-<uid>/<run id>-<suffix>`, recorded in the run dir's `scratch` file; a run without one keeps them in the run dir), one set per detected language pack (the pack's `cache_env` and `cache_argv`; Go: `GOCACHE`, `GOTMPDIR`). `loops run end` deletes them along with the build leftovers agents created (the pack's `artifacts`; §2.6, §5). Download and package caches stay shared (the pack's `shared_env`), and the user's global caches are never touched.
- **Make it work, then make it right.** Every phase applies `specs/ENGINEERING.md`:
  - plan: smallest edit, data shape first, subtract before adding, redesign instead of bolting on, levers (§1–§6);
  - implement: test first with the red seen, the domain modelled, idempotency (§7–§9);
  - QA: root causes, no hollow tests, the quality pass (§10–§14);
  - every loop: attack the premise after two failed rounds (§11).
- **Smallest sufficient context.** Search before reading, read ranges, cite by path and section, and never paste whole specs (`specs/CONTEXT_DISCIPLINE.md`).

## 2. Phases

| # | Phase (`run.Phase`) | Agents | Output in the run dir | Exit criteria |
|---|---|---|---|---|
| 0 | `intake` | lead | `ticket.md`, `charter.md` | The ticket is normalised and has a source; the charter and the checkpoints are recorded |
| 1 | `research` | `web-researcher`, `practices-researcher`, `code-researcher`, `hardware-researcher` (parallel) | `research/web.md`, `research/practices.md`, `research/code.md`, `research/hardware.{json,md}` | All four outputs are written |
| 2 | `plan` | `planner` ↔ `plan-reviewer` (+ the user at a `plan` checkpoint) | `plan.v<N>.md`, `plan-review.r<N>.md` | The latest review is `APPROVE` (and the user approved, at a checkpoint) |
| 3 | `implement` | `architect` (wave 0), then role engineers in waves 1…n | `reports/<agent-id>.md` (claims reports) | Every lane has reported |
| 4 | `qa` | one `qa-verifier` per implementer (pipelined), one `qa-quality` pass, then the lead runs the gates | `qa/<agent-id>.r<N>.md`, `qa/quality.r<N>.md` | Every claim is `VERIFIED`, the quality pass is `VERIFIED`, and `loops gate` is green |
| 5 | `product` | `product-reviewer` | `proofs/AC-<n>.md`, `pr.md`, `report.html` | Every AC is `VERIFIED` with a proof, and the PR/MR is opened (or the PR body is written under `--no-pr`) |
| 6 | `reflect` | `reflector` | `reflection.md` (+ ai-memory pages) | Reflection is written; `loops run end` |

QA starts pipelined: a verifier launches as soon as its implementer reports, even while other implementers in the wave are still running. A phase can be re-entered: if product returns work, the run goes back to `implement`, and then through `qa` again.

### 2.0 Intake

1. `loops run start --title "<short title>" [--ticket <ref>] [--source <s>]` prints the run ID and the run directory. It also creates the run's scratch folder outside the project and prepares the run-scoped caches and tmp dirs in it (Go: `<scratch>/go/{cache,tmp}`) and records a baseline of the untracked files already present, so cleanup (§2.6) can tell what the run created.
2. Fetch the ticket:
   - GitHub (`#123`, `owner/repo#123`, a github.com URL) or GitLab (a gitlab URL): `loops ticket fetch <ref>`. This writes `ticket.md`.
   - Jira (`ABC-123`) or Linear (a linear.app URL or key): fetch through the connected MCP tool, then pipe the text into `loops ticket fetch <ref> --source jira|linear --body-file -`. If the connector is not authenticated, tell the user and ask them to paste the ticket.
   - Free text: `loops ticket fetch --source text --body-file -`.
3. If ai-memory is connected (§7), call `memory_briefing` and one `memory_query` with the ticket's key terms. Save the hits to `research/memory.md` and record `loops run event --kind memory --name briefing`.
4. Record the checkpoints: `loops run ask [<list>]` (§2.7). With no list it applies `.loops/config.json`.
5. Write `charter.md` from `templates/CHARTER.md` (§2.7): the goal, the ACs as the ticket states them, what is in and out of scope, constraints, and the assumptions taken where the ticket is unclear. The ACs are final when the plan is approved.

Flags on `/loops:work`:
- `--ask <phases>`: the phases where the lead stops for the user (§2.7), e.g. `--ask plan,pr` or `--ask all`.
- `--approve`: the older spelling of `--ask plan`.
- `--no-pr`: write `pr.md` but do not push.
- `--models=cheap`: run every role one tier lower, except planner and plan-reviewer.

### 2.1 Research

- Launch the four researchers together, in parallel where the platform allows (`skills/work/platforms/<p>.md`).
- **`hardware-researcher`** runs exactly one command, `loops hw --write`, which writes `research/hardware.json` and `research/hardware.md`, then replies with the one-line recommendation. It is the cheapest agent in the run. On platforms without subagents (tier D), the lead runs `loops hw --write` itself.
- **`practices-researcher`** first scans the project's own rules and lists every conflict with loops' rules under `## Project rules that supersede loops` (§1).
- **`code-researcher`** first records the detected languages: `loops lang detect --write` writes `research/language.json` (each pack's `id`, root and build-tool variant). A pin in `.loops/config.json` `languages` overrides detection. The planner, the gate and the hooks use that list; each language's pack is `specs/languages/<id>/` and its skill `loops:lang-<id>`.
- Each gets the same inputs: `ticket.md`, `research/memory.md`, the project root, and the plugin `specs/` path (`loops path specs`).
- Each brief is at most 600 words. It contains facts with sources (`path:line`, a URL, or `spec §`) and has a `## Open questions` section.
- A researcher may run narrower `memory_query` calls. It must verify any memory hit against the current source before citing it, because memory is historical evidence, not truth.

### 2.2 Plan

- **Parallelism.** The planner sizes the waves to the effective cap = min(`parallel.max_agents` in `.loops/config.json` (default **100**), `research/hardware.json` `max_parallel_agents`, opt-in `limits.max_agents`).
  - The same role may run as several agents in one wave (`W1-domain-orders`, `W1-domain-payments`) when their owned globs are disjoint. Each gets its own qa-verifier.
  - A wave larger than the cap is split into sub-waves (`W1a`, `W1b`).
  - At most `max_concurrent_test_runs` agents run tests at once, each with the pack's `env_hints` (Go: `GOFLAGS=-p=2`) so one test run doesn't take every core. That is the `loops hw` recommendation, set in config by `parallel.max_test_runs`.
  - The plan's **Concurrency** block states the cap and the peak per wave. The planner and the plan-reviewer both run `loops plan check plan.v<N>.md`, and any finding blocks.
- The `planner` writes `plan.v1.md` from `templates/PLAN.md`. The plan contains:
  - acceptance criteria (`AC-1`…, taken from the ticket, or marked *derived*);
  - the task packet;
  - the contract;
  - the design (`## Design`): the data shape, what is removed first, whether the change fits the current design or what redesign it needs, planned breaks and levers (`specs/ENGINEERING.md §2–§6`);
  - the waves table, with each agent's claims;
  - the QA pairing;
  - the proof plan.
- The `plan-reviewer` writes `plan-review.r1.md` from `templates/PLAN_REVIEW.md` and ends with a verdict:
  - `APPROVE`; or
  - `CHANGES`, followed by numbered **blocking** items, each with what to change and why. Non-blocking items are allowed but never block. A missing or empty `## Design`, a bolt-on (`specs/ENGINEERING.md §4`) and a lane whose end state no other agent can check (§5) are blocking.
- On `CHANGES`:
  1. Record `loops run loopback --from plan-reviewer --to planner --round N --note "<one line>"`.
  2. Send the review file back to the same planner (resume or re-spawn per platform tier). It writes `plan.v<N+1>.md`.
  3. Review again.
- After `MaxPlanRounds` (3) rounds with blocking items still open, the lead rules on each one (§2.7), or asks the user when `plan` is a checkpoint.
- **Scope check.** Before it accepts an approved plan, the lead checks it against `charter.md`: every AC is served by at least one lane, no lane serves nothing in scope, and every out-of-scope item stays out. A gap goes back to the planner as a new round. The lead then adds the plan version and the final ACs to the charter.
- At a `plan` checkpoint the lead shows a ≤25-line plan summary (ask the user: Approve / Request changes / Stop). "Request changes" sends the user's notes to the planner as a new round. The answer is recorded with `loops run decide --by user`.

### 2.3 Implement

- **Wave 0 (`architect`, plus one `<id>-engineer` per detected language that needs toolchain work):**
  - the architect writes the plan's contract: ports and consumer-owned interfaces, typed commands and DTOs, stubs so every module compiles, the composition root;
  - the `<id>-engineer` sets up the toolchain and repo files: manifest and lockfile (adding dependencies in frozen-compatible form), toolchain pin, format, lint and typecheck configs, CI, and the test and coverage prerequisites the pack needs (`specs/REPO_PRACTICES.md`, `skills/lang-<id>`). Without one, the architect does it.

  Wave 0 is done when the pack's build and lint checks pass: `loops gate --only <the pack's build, lint and typecheck checks>` (Go: `loops gate --only build,vet`).
- **Waves 1…n:** launch every agent of a wave together, in parallel where the platform allows.
  - The order is producers (domain, application) before consumers (inbound and outbound adapters). Consumers may start early against the stubs.
  - Each agent gets its own block from the plan: Goal, Starts at, Behaviour, Owns, Must not touch, Claims to prove, Done when, and Out.
- Every implementer finishes with a **claims report** (`templates/CLAIMS_REPORT.md`, at most 400 words) saved to `reports/<agent-id>.md`. Each claim has the form `C<n>: <statement> — evidence: <test name | command → result | file:line>`. Under `## Red first` the report gives each new test and the failure it showed before the change (`specs/ENGINEERING.md §7`).

### 2.4 QA

- For each finished implementer, launch a `qa-verifier` with these inputs:
  - the claims report;
  - `git diff` of the implementer's owned paths;
  - the contract;
  - the ACs the implementer serves.
- The verifier checks every claim **independently**:
  - it re-runs the cited tests;
  - it reads the code;
  - it writes or runs counterexamples (edge cases, error paths, concurrency, under the pack's race policy);
  - it runs `loops gate --pkgs <owned> --only <the pack's build, lint and test checks>` (Go: `build,vet,test`) and `loops arch`.
- The verifier writes `qa/<agent-id>.r<N>.md` (`templates/QA_VERDICT.md`). Per claim it records `VERIFIED` or `FAILED`. A `FAILED` claim carries a repro:
  - the command;
  - an excerpt of at most 20 lines of output;
  - `file:line`;
  - expected vs actual;
  - the suggested fix area.

  The verifier also lists **unclaimed problems** it found. It also checks **test honesty** (`specs/ENGINEERING.md §10, §12, §8`):
  - a test that would still pass with every function under test returning nothing fails the claim it supports;
  - a fix that only guards the symptom, with no reproducing test, fails;
  - an unexplained compiler or lint suppression fails.
- For each verdict, record `loops run verdict --name <agent-id>:C<n> --status VERIFIED|FAILED --round N --ref qa/<file>`.
- When anything has FAILED:
  1. Record `loops run loopback --from qa-verifier --to <agent-id> --round N`.
  2. Send the QA file back to the same implementer (resume or re-spawn per platform tier).
  3. When it reports again, re-verify with the same verifier (resume or re-spawn).
- After `EscalateAfter` (2) failed rounds, relaunch the implementer one model tier up (haiku → sonnet → opus), with a summary of the history.
- After `MaxQARounds` (3) failed rounds, the lead decides (§2.7), or asks the user when `qa` is a checkpoint.
- **Attack the premise.** When the same claim fails a second round, the implementer does not try a third fix on the same premise. Its next report states that premise under `## Premise`, and the lead records it (`specs/ENGINEERING.md §11`, §4).
- **Quality pass.** When every lane is verified, one `qa-verifier` with Agent id `qa-quality` runs `skills/quality` on the run's changed code.
  - It measures CRAP, structural duplication and mutation survivors (`specs/ENGINEERING.md §13`) and writes `qa/quality.r<N>.md` (`templates/QUALITY.md`).
  - Each finding names its owning lane. The lead records `loops run verdict --name quality:Q<n> …` and sends the findings back to that lane like a failed claim (`loopback --from qa-quality --to <agent-id>`), with the same limits (§4). That lane's verifier then re-verifies, and the pass runs again.
  - A tool that isn't installed is recorded and never blocks.
- Then the lead runs the full `loops gate`. Gate failures go back to the owning lane the same way.

### 2.5 Product

The `product-reviewer` does the following, in order:

1. Maps each AC to the claims that serve it.
2. **Tests manually.** It builds and runs the service (the repo's `make run`, docker compose, or the language's run command, e.g. `go run ./cmd/<svc>`), then exercises every AC through the real interface (curl, grpcurl, the CLI). It also probes edge cases:
   - invalid, empty and oversized input;
   - missing and incorrect auth;
   - not-found;
   - duplicate or retried requests (idempotency);
   - concurrent requests;
   - downstream failure, where it can be simulated.
3. Saves each check as `proofs/AC-<n>[-<case>].md` (`templates/PROOF.md`): the command, the trimmed output, and log excerpts. Records `loops run event --kind proof --name AC-<n> --ref proofs/...`.
4. Records `loops run verdict --name AC-<n> --status VERIFIED|FAILED`.
5. On FAILED: records a loopback to the owning role and gives the lead the proof that shows the gap. The run returns to `implement` (`MaxProductRounds` = 2, then the lead decides, §2.7).
6. When every AC is VERIFIED:
   1. Runs `loops report`.
   2. Writes `pr.md` (`templates/PR_BODY.md`, at most ~40 lines plus `<details>` proof blocks).
   3. Stops and reports to the lead (proof summary + `pr.md` path). Unless `--no-pr` is set, the lead sends the same product-reviewer "open the PR" (resume or re-spawn per platform tier). When `product` is a checkpoint, the lead first shows the user `pr.md` and asks. Only then it:
      - creates the branch `loops/<ticket>-<slug>`;
      - commits in small ordered commits, wave 0 first and then each wave's lanes. Each is `git commit -m … -- <paths>`, with a conventional subject and a body saying why, what changed and how it was proven (claim ids, QA file, AC proofs) (`specs/ENGINEERING.md §18`);
      - pushes;
      - opens the PR with `gh pr create --body-file pr.md` (or `glab mr create`; the forge is chosen from `git remote get-url origin`);
      - records `loops run event --kind pr --ref <url>`.

### 2.6 Reflect

The `reflector`:
1. Reads `events.jsonl` through `loops status --json` and the run files.
2. Writes `reflection.md` (`templates/REFLECTION.md`), covering:
   - the outcome;
   - each loop-back and its root cause, and every premise attacked (`specs/ENGINEERING.md §11`);
   - the quality pass: findings fixed, tools that did not run, and pre-existing debt noted;
   - gate failures;
   - what the plan missed;
   - gotchas worth remembering;
   - suggested changes to roles, skills or specs.
3. When ai-memory is connected (§7), writes 1–5 durable pages:
   - `decisions/<slug>.md`: kind `decision`, tier `semantic`;
   - `gotchas/<slug>.md`: kind `gotcha`, tier `semantic`;
   - `procedures/<slug>.md`: kind `procedure`, tier `procedural`.

   It searches first and updates an existing page (`expected_page_id`) instead of duplicating it. It never stores secrets, tokens or customer data. Each write is recorded as `loops run event --kind memory --name write --target <path>`.
4. **Project-rule overrides** (§1):
   1. Write `overrides.md` (`templates/OVERRIDES.md`) from `State.Overrides` (`loops status --json`). Each row gives the loops rule, the project rule, where it was applied, and the reason.
   2. If a PR or MR exists (`State.PR`) and there is at least one override, post the file as a **comment**: `gh pr comment <url> --body-file <Run dir>/overrides.md`, or `glab mr note <id> --message "$(cat <Run dir>/overrides.md)"`, using the absolute run-dir path. Never edit the PR description.
   3. Record `loops run event --kind note --name overrides-comment --ref overrides.md`.

   The PR step (§2.5) covers this comment. With no PR, or under `--no-pr`, only the file is written.

5. **Cleanup** (last step). Record what the end-of-run cleanup will free: `loops run clean --dry-run` shows the run's caches and tmp dirs and the build leftovers (each pack's `artifacts`, e.g. Go's `*.test`, binaries, coverprofiles, pprof files, compared against the intake baseline). Add a line to `reflection.md`: `Cleanup: N files, X MB freed; kept for review: <paths>`. Unknown new files are listed for review, never deleted; tracked and pre-existing files are never touched. Artifact directories are removed only inside a git work tree, only when new since the baseline, and only when they hold nothing authored: a directory with a tracked path, a nested `.git`, an unreadable entry, a script (shebang), a document (`*.md`, `*.rst`, `*.adoc`) or a non-generated source or test file is kept for review. A directory glob without a parent (`build/`) matches only at the pack root; packs write `**/target/` to match deeper. Files under `testdata/`, `fixtures/`, `test/`, `tests/` or `spec/` are never removed by an artifact glob. The actual deletion happens in `loops run end` (below) and is recorded as a `cleanup` note event.

Finally, the lead runs `loops run end --outcome done` and gives the user a ≤10-line summary with the PR link and `report.html`.

### 2.7 The lead: one master agent

One agent, the lead, controls the run from intake to end. Role agents work inside one phase and one scope; only the lead sees the whole run. This section is how it keeps the scope and the context across phases, and how little it needs the user.

**What only the lead does.**
- Moves the run between phases (`loops run phase`), and re-enters a phase when work comes back (§2).
- Spawns, resumes, labels and stops every agent, and relays facts between them.
- Holds the scope: writes `charter.md` and is its only writer.
- Takes the decisions the run turns on and records each one.
- Talks to the user. Role agents never ask the user anything; a question goes to the lead, which answers it from the charter, the plan or its own decision.

**Scope: `charter.md`.** Written at intake from `templates/CHARTER.md`, in the run dir:
- the goal in one or two sentences;
- the ACs as the ticket states them, final once the plan is approved;
- in scope and out of scope;
- constraints: project rules, deadlines, compatibility promises, the `--no-pr` choice;
- assumptions taken where the ticket is unclear, each also recorded as a decision.

Every spawn passes `Charter: <abs path>` (§3), and every role reads it first. An agent that finds work outside the scope puts it in its report's `Out` section and never does it. The lead checks each phase's output against the charter before it moves on:
- the plan covers every AC and nothing outside the scope (§2.2);
- QA findings and gate failures are routed only to lanes in the plan;
- product proves every AC in the charter, and `pr.md` lists the assumptions and the lead's decisions under Open follow-ups.

**Context: re-anchor, never recall.** The run's state lives in `events.jsonl` and the run dir, not in the lead's context:
- `loops run phase <p>` records the phase and prints the brief: the run, the charter path, the phase path, the checkpoints, running agents, open failures, the gate, the latest decisions and the next step. `loops run brief [--json]` prints it at any time.
- After a compaction or in a resumed session, the `session-start` hook adds the same brief to the lead's context. The lead continues from it and does not restart finished phases.
- When the brief and the lead's memory disagree, the brief wins. The lead opens a file the brief names only when a decision needs it (§1).

**Checkpoints: the user is opt-in, per phase.** By default there are none: the lead runs every phase without asking. The user opts in to any phase, or `all`:

| Checkpoint | When the lead stops | What it shows; the choices |
|---|---|---|
| `intake` | after the charter is written | the charter; Continue / Edit scope / Stop |
| `research` | when the four briefs exist | their `## Open questions`; the user answers or says Continue |
| `plan` (alias `approve`) | after `APPROVE`, and when plan rounds run out | a ≤25-line plan summary or the open blocking items; Approve / Request changes / Stop |
| `implement` | after wave 0 and after each wave reports | the lanes and their claims; Continue / Stop |
| `qa` | when the gate is green, and when QA rounds run out | the verdicts and the quality pass, or the failing claim; Continue / a decision / Stop |
| `product` (alias `pr`) | before the push and PR, and when product rounds run out | `pr.md`, or the unmet AC; Open the PR / Hold / Stop |
| `reflect` | before `loops run end` | `reflection.md` and the memory pages to write; End / Edit |

Where they come from, strongest first:
1. the user's own words in the request ("let me approve the plan", "ask me before the PR", "check with me at every phase");
2. `--ask <phases>` (and `--approve`) on `/loops:work`;
3. `.loops/config.json` `ask` (a list or a string), plus the older `"approval": "always"`.

An explicit list (1 or 2) replaces the config's for the run, so `--ask none` turns a configured checkpoint off once.

The lead records them once at intake with `loops run ask [<list>]`; with no list, the config applies. At each boundary `loops run checkpoint <phase>` prints `ask` or `auto`. When the user later says "ask me from now on" or "stop asking", the lead records a new `loops run ask`. Each answer at a checkpoint is recorded: `loops run decide --by user --topic <phase> --note "<answer>"`.

**Decisions instead of questions.** Where the workflow used to stop for the user, the lead decides, unless that phase is a checkpoint:

| Situation | The lead's default |
|---|---|
| The ticket is unclear | Take the narrowest reading that meets the stated goal, write it under the charter's `## Assumptions`, and record it. |
| No language pack detected | Continue; the gate checks nothing. Note it in the charter's constraints. |
| Plan rounds run out (§4) | Rule on each open blocking item, accepting or rejecting it with a reason. The planner writes the final version with the rulings; the reviewer does not re-open a ruled item. |
| QA rounds run out (§4) | Do one of three things. If the premise (ENGINEERING §11) shows the design is wrong, go back to `plan` for that lane. If no AC needs the claim, move it to Open follow-ups. If an AC cannot be met, end the run `failed` with the evidence. Never weaken a test, the gate or the race policy. |
| Product rounds run out (§4) | Open the PR as a draft (`gh pr create --draft`), with the unmet AC and its proof under Open follow-ups. Or end the run `failed` when nothing useful would ship. |
| An opt-in limit blocks a spawn (§4a) | Over a concurrency cap, wait for a slot. Over a token, agent or role limit, the limit is the user's constraint: let running agents finish, then end the run `abandoned` and say where it stopped. |
| A model is unavailable | Fall back (§3a). |
| The ticket cannot be read (a connector is unauthenticated, and there is no text) | The one stop that cannot be decided away: tell the user what is missing, and wait. |

Each decision is recorded: `loops run decide --topic <what> --note "<decision; reason>" [--ref <file>]`. Decisions show in the brief, the report and the PR's Open follow-ups, and the reflector reviews them.

**Messages to the user.** One line per phase transition (`plan: approved v2, 3 waves`), and the ≤10-line summary at the end. Outside a checkpoint, never a question.

**What the lead never decides away.** The pushes and PRs go to a branch, never to the main line, unless a project rule says so (§1). No force-push, no merge, no history rewrite. No weakened test, gate or race policy. No step outside the charter's scope.

## 3. Roles

`ROLES.md` is the matrix and `agents/<role>.md` the definitions. There are two kinds of implementer:
- **layer roles** (`architect`, `domain-engineer`, `application-engineer`, `api-engineer`, `persistence-engineer`, `integration-engineer`, `test-engineer`, `docs-writer`), which own a hexagonal layer in any language;
- **language roles**, one `<id>-engineer` for each pack in `specs/languages/` (`python-engineer`, `typescript-engineer`, `go-engineer`, …: 15 in all), which own the toolchain and repo files in wave 0 and take language-level lanes (version upgrades, dependency bumps, build migrations).

A lane is a layer role plus a language: the lead passes the `Language:` line, and the agent loads that pack's skill (`loops:lang-<id>`) for the toolchain, the race policy, coverage and idioms. In a repo with several packs, each lane has one language.

Every spawn names its role (the platform's syntax is in `skills/work/platforms/<p>.md`), and a `model` only when it overrides the role default (escalation, `--models=cheap`).

The prompt is the agent's block from the plan plus these lines:
- `Run dir: <abs path>`
- `Specs: <abs path>`
- `Templates: <abs path>`
- `Loops: <abs path to the loops binary>` (or `loops` when it is on PATH)
- `Role: <role>`
- `Agent id: <W1-domain>`
- `Wave: <n>`
- `Language: <id>` (implementers and QA; from `research/language.json`)
- `Charter: <abs path to charter.md>` (§2.7)

### 3a. Model fallback

Each role's tier (`ROLES.md`) is a preference. The planner and plan-reviewer default to **opus**, the strongest available reasoning model.
- If a spawn fails because the requested model is unavailable (not on the user's plan, not enabled for the org or platform, or over capacity), the lead re-spawns the same role with the next-best model the user has:
  - reasoning roles (planner, plan-reviewer, architect, product-reviewer, escalated roles): opus → sonnet → the session model;
  - other roles: one tier down at a time, then the session model.
- Platforms that don't use Claude model names use the strongest reasoning model in their own list, or the session model.
- Record each fallback: `loops run event --kind note --name model-fallback --role <role> --note "<wanted> unavailable → <used>"`, and label the lane with the model actually used (`loops run label … --model <used>`).
- Plan quality matters most, so tell the user in one line when the planner or plan-reviewer had to run below opus.
- Never stop the run because a preferred model is missing.

Right after each spawn, the lead names the lane: `loops run label --agent <platform agent id, or solo-<n>> --name <W1-domain> --role <role> --wave <n>`. Hooks only see the platform's opaque agent id (or none, on platforms without subagent events); the label ties it to the plan id used in claims (`W1-domain:C3`), verdicts and the pane.

## 4. Loop limits

| Loop | Limit | Then |
|---|---|---|
| planner ↔ plan-reviewer | `MaxPlanRounds` = 3 | The lead rules on the open blocking items (the user, at a `plan` checkpoint) |
| implementer ↔ qa-verifier | `EscalateAfter` = 2 | Relaunch the implementer one model tier up |
| implementer ↔ qa-verifier | `MaxQARounds` = 3 | The lead decides (the user, at a `qa` checkpoint) |
| product → implementer | `MaxProductRounds` = 2 | The lead decides (the user, at a `product` checkpoint) |

Every pass around any loop is recorded with `loops run loopback`, which feeds the status line counters and the report's arrows.

**Attack the premise** (`specs/ENGINEERING.md §11`) applies to every loop in the table. This covers a claim, a quality finding, a gate check, an AC, or a blocking plan item that fails in two rounds. The author's next report opens with `## Premise`: the premise both attempts shared, in one sentence, and the actors that hold the conflicting responsibility. The next attempt removes that assignment instead of patching again, and that can mean a plan revision. The lead records `loops run event --kind note --name premise --note "<sentence>"` and passes the premise to any agent escalated one tier up.

## 4a. Opt-in limits (only when the user asks)

The user may cap concurrent agents (`--max-agents N`), tokens per phase (`--budget research=200k,implement=2M`), agents per phase (`--agents implement=12`), or the roles allowed in a phase (`--roles implement=domain-engineer,api-engineer`). Limits come from `/loops:work` flags or `limits` in `.loops/config.json`. Without them, nothing is limited and nothing asks.

- At intake the lead builds the limits JSON with `loops budget parse …` from the user's flags, then records it: `loops run event --kind limits --note '<json>'`.
- Before every spawn the lead runs `loops budget check --phase <p> --spawn <n> --role <r>`.
  - Exit 0: the spawn fits.
  - Exit 3: a limit would be exceeded. The command records a `budget` event and prints the reason. The lead stops spawning in that phase and lets running agents finish. It then decides as §2.7 says, or, when that phase is a checkpoint, asks the user to raise the limit, continue past it once, or stop the run.
- Tokens are measured from each agent's own transcript (`agent_transcript_path`) when it stops. A resumed agent counts only its growth. Claude Code provides that transcript. On platforms that don't send a per-agent transcript, token budgets are advisory, while agent and role limits still apply everywhere (`specs/PLATFORMS.md` §8).
- `loops budget show` prints usage against the limits.

## 5. Events

`.loops/runs/<id>/events.jsonl` holds one JSON object per line (`run.Event`). `.loops/` is local: whatever creates it (a run, the installer, session start in a project that has one) adds a `.loops/.gitignore` of `*`, so git never lists it. Kinds are `run`, `phase`, `agent_start`, `agent_stop`, `label`, `override`, `limits`, `budget`, `step`, `loop_back`, `verdict`, `gate`, `proof`, `guard_deny`, `memory`, `note`, `pr`, `ask`, `decision`, `end`. The fields per kind are documented in `internal/domain/run/event.go`. The schema is additive-only.

| Source | Emits |
|---|---|
| Skills (lead), through `loops run …` | `phase`, `loop_back`, `verdict`, `proof`, `memory`, `note`, `pr`, `ask`, `decision`, `end` |
| Hook `trace` (PostToolUse, PostToolUseFailure) | `step`, with `agent_id`/`agent_type`, so steps land in their lane |
| Hook `post-edit` (PostToolUse on Edit/Write/MultiEdit; alias `go-post-edit`) | `step` with the pack's single-file format and lint result (Go: gofmt/goimports + go vet) |
| Hook `trace-agent` (SubagentStart, SubagentStop) | `agent_start`, `agent_stop` |
| Hooks `read-guard`, `model-guard`, `test-guard` (alias `race-guard`), `toolchain-env` (alias `go-env`) | `guard_deny` |
| `loops run end` / `loops run clean` | `note` named `cleanup` (what was freed) |
| `loops gate` | `gate`, one per check (with `pack` when several packs are detected) plus `all` |

The CLI commands that write these events:
- `loops run phase <phase> [--note]`: records the phase, then prints the brief (§2.7).
- `loops run ask [<list>]`: records the checkpoints (phase names, `approve`, `pr`, `all` or `none`; no list = `.loops/config.json` `ask`) and prints them.
- `loops run checkpoint <phase>`: prints `ask` when the lead stops for the user at that phase, else `auto`. Writes nothing.
- `loops run decide --topic T --note text [--by lead|user] [--ref f] [--phase P]`: records a decision.
- `loops run brief [--json]`: prints the lead's re-anchor: the run, the charter, the phase path, checkpoints, running agents, open failures, the gate, the latest decisions and the next step. Writes nothing.
- `loops run event --kind K [--name --target --from --to --status --ok --round --wave --role --agent --model --ref --note --phase --ms]`
- `loops run loopback --from R --to A --round N [--note] [--phase]`
- `loops run verdict --name X --status S [--round N] [--ref f] [--agent A] [--role R]`
- `loops run label --agent <id> --name <plan id> --role R --wave N [--model M]`
- `loops run override --rule <loops rule> --project <project rule> [--note] [--agent A] [--role R]`
- `loops budget check --phase P [--spawn N] [--role R]` (writes a `budget` event when it blocks)
- `loops budget parse [--max-agents N] [--budget p=200k,…] [--agents p=N,…] [--roles p=a,b]` prints the limits JSON for `loops run event --kind limits`. `loops budget show` prints usage against the limits.
- `loops hw [--json] [--write]` and `loops plan check <plan> [--cap N]`
- `loops run end [--outcome done|abandoned|failed] [--no-clean]`: ends the run and, unless `--no-clean`, deletes the run's caches and tmp dirs (the whole scratch folder) and the build leftovers created during the run, then records a `cleanup` note event with what was freed. Abandoned and failed runs are cleaned too. `--no-clean` keeps everything for debugging.
- `loops run clean [--dry-run] [--json]`: the same cleanup on demand; `--dry-run` only lists what would be deleted and what is kept for review.
- `loops run env [--shell sh|fish|powershell]`: prints the run-scoped environment of the detected packs (Go: `GOCACHE`, `GOTMPDIR`) as `export` lines for the chosen shell, so a shell can load them before running toolchain commands by hand.
- `loops exec [--pack id] -- <cmd…>`: runs any command with the run-scoped environment of a pack (its `cache_env` and `env`, with its `cache_argv` inserted after the tool name; default: the detected pack that uses the tool, else the only detected pack). Without an active run it runs the command plain. Works on every platform; use it wherever `toolchain-env` can't rewrite a command.
- `loops go <args>`: runs `go <args>` with the run's Go environment (the Go form of `loops exec`).
- `loops run dir`

Language packs (`specs/languages/<id>/pack.json`):
- `loops lang list [--json]`: the packs loops ships.
- `loops lang show <id> [--variant v]`: one pack, with a build-tool variant applied.
- `loops lang validate [pack.json…]`: checks pack files against the schema.
- `loops lang detect [--json] [--write] [--root dir]`: the packs that apply to the project (id, root, variant). `--write` saves `research/language.json` in the run dir (§2.1). `languages` in `.loops/config.json` pins them instead.

Gates:
- `loops gate [--pkgs p,…] [--only c,…] [--skip c,…] [--enable c,…] [--base ref] [--vuln] [--fail-fast]`: runs every detected pack's checks (§6). `--only`/`--skip`/`--enable` take check names; `--enable` turns on `default: off` checks; `--vuln` enables the vuln checks.
- `loops arch [--json] [--root dir]`: the architecture lint (`specs/ARCHITECTURE.md §25`).

`run.Fold(events)` gives `run.State`, which `loops status --json` prints. The status line, the mod pane and the HTML report all read that one source: every recorded event of the current run also rewrites `.loops/status.json` (`run.StatusDoc`, the same document; tool-call steps at most once a second), which the pane reads instead of starting a program, and the end of a run marks it inactive.

## 6. Gates

`loops gate` runs the checks of every detected language pack (§2.1), pack by pack, in the order each pack lists them (its `checks`). Each check is recorded as a `gate` event. The pipeline per pack is:

1. `deps`: a frozen install or lock verify, only in that form, first.
2. `build`, `format`, `lint`, `typecheck`: a format check is required only when the formatter's config is committed; optional tools that aren't installed are skipped with their install hint.
3. `arch`: `loops arch`, and the pack's native architecture check if it has one; only error findings fail (`specs/ARCHITECTURE.md §25`).
4. `test`: the pack's test checks, which also produce coverage and carry the race policy.
5. `coverage` (required, never skippable): at least 70% of new code is covered (`specs/TESTING.md §8.1`). `gate.coverage_min` in config may only raise the floor; `gate.coverage_exclude` adds exclusions to the pack's `coverage.exclude` (Go: `cmd/**`, `internal/compose/**`, `testdata`, generated files). It also fails when a changed file's new lines are all untested, and, for Go, when a changed package has no tests. The output lists uncovered `file:line` ranges.
6. `vuln`, `sast`, `sbom`: off by default (`default: off`); on with `--enable <check>`, `--vuln` or `gate.vuln` in config, and required in CI (`specs/REPO_PRACTICES.md §11`).

The Go pack reproduces the original gate exactly: `build` (`go build ./...`), `gofmt`, `vet`, `staticcheck` (if on PATH), `arch`, `test` (`go test -race -count=1 ./...`), `coverage`, `govulncheck` (with `--vuln`). `loops lang show <id>` prints any pack's checks.

QA scopes the same checks to one agent with `loops gate --only <checks> --pkgs <owned>` (Go: `--only test,coverage`).

**Race policy.** Every test run follows the pack's race policy (TAXONOMY CONC-04):
- `flag`: every test command carries the race flag, and the race-guard denies a test command without it.
- `build`: the full gate runs the pack's race build (its `race: true` test check), and it must pass.
- `none`: the full gate runs the pack's substitute checks (its `race: true` checks), and they must pass. The pack states, and the gate records:
  - the seed of every randomised run;
  - the parallelism used;
  - the repeat count applied to the changed tests.

  A pack whose substitute is static (Rust) names the lint checks that carry it.

A required race or substitute check that is skipped fails the full gate, as a skipped required check does today. There is no opt-out: the fix is a cheaper test, never a weaker policy. If the policy can't run on the host, the gate fails with "race policy unavailable: …". The modes per pack are in `specs/TESTING.md §8.2`.

**Hooks.** The new names are canonical; the old names stay as aliases, so existing hook configs keep working.

- **`test-guard`** (PreToolUse on Bash; alias `race-guard`) is the race-guard above. It matches commands against each pack's `test_commands`. It denies only in `flag` mode (Go `-race`, Swift `--sanitize=thread`): a test command without the pack's race flag (or race env, e.g. `GOFLAGS=-race`). In `build` and `none` mode it never denies; the policy is enforced by the full gate's `race.checks`, not per command.
- **`toolchain-env`** (PreToolUse on Bash; alias `go-env`) scopes every agent's toolchain command to the run's caches and tmp dirs (the pack's `cache_env`, `cache_argv`). For Go it rewrites the command where the platform's hooks can, and otherwise denies with a hint to use `loops go <args>` (or `loops exec -- <cmd>`) or `loops exec -- <cmd>`: the command is rewritten on Claude Code, Gemini CLI and Windsurf/Devin; on Codex and Cursor only a lone `go build|test|vet|list` is rewritten, and anything else is denied; on Copilot and OpenCode it is always denied (`specs/PLATFORMS.md` §5). For every other pack it only rewrites, on Claude Code, Gemini CLI and Windsurf/Devin, and otherwise fails open; it never denies. Agents there run toolchain commands through `loops exec -- <cmd>`. `loops gate` uses the run environment automatically. Agents never set cache variables themselves.
- **`post-edit`** (PostToolUse on Edit/Write/MultiEdit; alias `go-post-edit`) runs the pack's `post_edit` on the edited file: its single-file formatter, then its lint on the file's directory (Go: gofmt/goimports, then `go vet`). A failure is sent back to the agent. A pack with no single-file formatter has no post-edit step.

A full pass records `gate all ok`, which clears the dirty flags (`run.State.SourceDirty` for any pack's sources; `GoDirty` for `.go` files, kept for compatibility). A scoped run (`--pkgs`, `--only` or `--enable`) never records `all`.

The **Stop gate** (`stop-gate`) stops the lead from ending its turn while the run is active, a source file of any pack has changed since the last full pass (`run.State.SourceDirty`, JSON `source_dirty`; not only `.go` files), and no agent lane is running. The escape hatches are `LOOPS_GATE=off` and `loops run end --outcome abandoned`.

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
2. Behaviour is implemented in its owning layer (`specs/ARCHITECTURE.md`), and `loops arch` reports no errors.
3. Every new or changed module has unit tests, including failure paths; new-code coverage is at least 70% (`loops gate` coverage, `specs/TESTING.md §8.1`); all tests run under the pack's race policy (§6).
4. Every claim is verified by QA, the quality pass is `VERIFIED` (`specs/ENGINEERING.md §13`; a tool that isn't installed is noted, not required), and `loops gate` is green for every detected pack.
5. Consistency impact is stated (ordering, idempotency, clock and RNG, migrations), with `skills/consistency` followed where it applies.
6. Performance is measured (a benchmark before and after) if a hot path changed.
7. Compatibility is analysed: API, persisted schema, config, migrations, and the manifest and lockfile when dependencies changed (`specs/REPO_PRACTICES.md §3`).
8. Docs are updated: the package `AGENTS.md` where the contract changed, and an ADR only for a cross-cutting decision.
9. The behaviour is observable: errors are explicit, logs and metrics exist where operators need them.
10. There are no untracked TODOs. Each one is either done or listed under Open in `pr.md`.
11. The PR is opened with proofs, `report.html` is generated, and `reflection.md` is written.
12. The work stayed inside `charter.md`'s scope, and every decision the lead took in the user's place is recorded (`loops run decide`) and listed in `pr.md`.
