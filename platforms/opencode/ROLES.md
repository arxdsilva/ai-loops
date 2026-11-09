# Roles

Each role is a plugin agent in `agents/<role>.md`, spawned as `loops:<role>` on Claude Code and through each platform's own mechanism elsewhere (`skills/work/platforms/<platform>.md`, `specs/PLATFORMS.md §4`). Its frontmatter `model` is the default tier. The lead passes `model` only to escalate a role (WORKFLOW §4) or under `--models=cheap`. A guard test keeps this table in sync with the agents' frontmatter.

The "Owns (default)" column is the default scope. The plan narrows it per agent with explicit globs, and every implementer's file ownership is disjoint from every other implementer's.

| Role | Phase | Model | Edits code | Owns (default) | Can | Cannot |
|---|---|---|---|---|---|---|
| web-researcher | research | sonnet | no | `research/web.md` | WebSearch, WebFetch, ai-memory query | Edit the repo |
| practices-researcher | research | sonnet | no | `research/practices.md` | Read plugin specs, skills and repo docs; ai-memory query; official Go docs | Edit the repo |
| code-researcher | research | haiku | no | `research/code.md` | grep, find, ranged Read, LSP; ai-memory query | Edit the repo, judge correctness |
| hardware-researcher | research | haiku | no | `research/hardware.{json,md}` | Run `loops hw --write`, then reply with the one-line recommendation | Anything else (3 turns max) |
| planner | plan | opus | no | `plan.v<N>.md` | Read the briefs and repo; write the plan | Edit code |
| plan-reviewer | plan | opus | no | `plan-review.r<N>.md` | Read; write the review | Edit the plan or code |
| architect | implement (wave 0) | opus | yes | Interfaces and ports, typed commands and DTOs, stubs, `go.mod`/`go.sum`, `cmd/**`, `internal/compose/**`, Makefile, CI | `go get`, `go mod tidy`; shared config | Implement behaviour beyond stubs |
| domain-engineer | implement | sonnet | yes | `internal/domain/**` | Pure rules, entities, value objects, domain services and their tests | Import adapters, application, `os`, third-party code; `time.Now`, global `math/rand`, `go` statements |
| application-engineer | implement | sonnet | yes | `internal/application/**` | Use cases, orchestration, consumer-owned ports, and their tests with fakes | Import adapters; put business rules here |
| api-engineer | implement | sonnet | yes | `internal/adapters/inbound/**` | HTTP/gRPC/CLI handlers, request validation, DTO↔command mapping, error→status mapping | Mutate domain state outside commands; leak transport types inward |
| persistence-engineer | implement | sonnet | yes | `internal/adapters/outbound/{db,store,repo,cache,queue}*/**`, `migrations/**` | Repositories, migrations, caches, queue producers and consumers, and their integration tests | Leak driver or DB types past a port; write a migration without a version |
| integration-engineer | implement | sonnet | yes | `internal/adapters/outbound/<client>/**` | Clients for external APIs and brokers, retries and timeouts, contract tests with fakes | Leak library types past a port |
| test-engineer | implement | sonnet | yes | `**/*_test.go` and `testdata/**` within its scope, `test/**`, benchmarks | Integration, e2e, fuzz and benchmark tests; fixtures | Weaken or delete a test; edit non-test code |
| docs-writer | implement | haiku | docs only | Package `AGENTS.md`, `README.md` sections, `docs/adr/**` | Write docs from facts the lead supplies | Edit code; invent facts |
| qa-verifier | qa | sonnet | no | `qa/<agent-id>.r<N>.md` | Read; run tests, `loops gate --pkgs`, `loops arch`; put counterexample test snippets in its QA file (never in the repo) | Edit repo code (it reports; the owner fixes) |
| product-reviewer | product | opus | no | `proofs/**`, `pr.md`, `report.html` | Run the service; curl, grpcurl, CLI; `loops report`; branch, commit, push; `gh` or `glab` PR only after the lead resumes it with the user's confirmation (or `auto_pr`) | Edit code (unmet ACs go back) |
| reflector | reflect | sonnet | no | `reflection.md` | Read the run dir; ai-memory query and write | Edit code; store secrets in memory |

## Project rules first (every role)

The target project's own rules always take precedence over loops rules (WORKFLOW §1). When you follow one instead of a loops rule, record it right away with `loops run override --rule … --project … --note …`. The reflector reports every override to the user as a PR comment.

## Rules shared by every implementer (architect and the engineers)

These come from rimworld-go PARALLEL_WORK §3.

- **Strict TDD.** Write the failing test, make it pass, then refactor. Never weaken or delete a test to get green.
- **Targeted checks only:** `GOFLAGS=-p=2 go test -race -count=1 -cover ./<owned>/...` and `go vet ./<owned>/...`. Never `go test ./...` or `make all`; the lead and QA run the full gates.
- **Ownership.**
  - Never edit outside your owned globs.
  - Report failures you see in other agents' files instead of fixing them.
  - Never touch `go.mod`/`go.sum` or shared config. Only the architect does; everyone else reports missing deps.
  - No `git add`/`commit`/`push`. Only product-reviewer commits, after the lead confirms.
- **Producers not done?** Develop against the contract and stubs, then report what is still stubbed.
- **Paths.** Use absolute paths; cwd may change between calls.
- **Report.** Finish with a claims report (`templates/CLAIMS_REPORT.md`, at most 400 words). Every claim names its evidence.

## Model tiers

These come from rimworld-go PARALLEL_WORK §6.

| Tier | Model | Use for |
|---|---|---|
| 1 | haiku | Mechanical, fully specified work: locating code, inventories, docs from supplied facts |
| 2 | sonnet | Implementation against a written contract, tests, adapters, QA of ordinary code |
| 3 | opus | Planning and plan review, contracts, consistency and concurrency, migrations, product judgement |

Escalate one tier after two failed QA rounds (WORKFLOW §4). Never start higher "to be safe".
