# What loops covers today (v0.4.1)

This is the reference for the `loops today` column in every language file:
- **covered**: a language-neutral rule already exists. Cite it as `spec §n` or a skill name.
- **go-only**: the rule exists, but only in Go terms or Go tooling.
- **missing**: nothing in loops says it.

Paths are relative to the repo root.

## Content

| Area | Where | Language-neutral? |
|---|---|---|
| Workflow: phases, waves, contract first, disjoint ownership, claims, QA loop-backs, product proofs, reflection | `specs/WORKFLOW.md` §1–§4, §7, §8; `skills/work`, `skills/parallel-work`, `skills/review-integration` | **neutral**, except the Go commands in §1 (Go cache), §2.2 (`GOFLAGS=-p=2`), §2.3 (wave 0 runs `go get` and `go build`/`vet`) and §6 (gates) |
| Context and parallel discipline | `specs/CONTEXT_DISCIPLINE.md`, `specs/PARALLEL_WORK.md` | **neutral** |
| Hexagonal architecture | `specs/HEXAGONAL_ARCHITECTURE.md` §1–§30 | Concepts are **neutral**: model, dependency rule, consumer-owned ports, inbound and outbound ports, commands and queries, a single owner of state, error taxonomy, domain events, state machines, persistence, config and time boundaries, DI, ADRs, smells. The layout and the examples are **go-only**: §4 repository structure (`internal/domain`, `cmd/`), §14 `context.Context`, §15 `pkg/`, §22–§23 init order and `cmd`, §25 arch lint via `loops arch`. |
| SOLID | `specs/SOLID_GO.md` §1–§11 | The principles are **neutral**. The vocabulary and the limits are **go-only**: §8 counts methods, interfaces, files and params in Go units; §9 is about package-level vars. |
| Go language rules | `specs/GO_BEST_PRACTICES.md` §1–§54 | **go-only** as written. Many topics have a neutral core that other languages need too: §8–9 errors, §10 context and cancellation, §11–12 concurrency, §13/§20/§21/§34 deterministic time, RNG and ordering, §24 logging, §26 doc comments, §28 config loading, §30 testing, §31 benchmarks, §38 security, §41–§47 domain language, state transitions, validation at boundaries, schema migration, idempotency, observability, §48–§49 dependency policy, §51–§52 review checklists. |
| Consistency (clock and RNG injection, ordering, idempotency, migrations, atomic writes) | `skills/consistency` | **mostly neutral** |
| Jobs, sagas, leases, state machines | `skills/jobs-and-state-machines` | **mostly neutral** |
| Testing (unit, table, fuzz, bench, always race) | `skills/testing`, `ROLES.md` shared implementer rules | **go-only**: it cites GO_BEST §30–31 and `httptest`. The hard gate rule (≥70% coverage of new code, race) is stated neutrally in `ROLES.md`, but it is enforced only for Go. |
| Performance | `skills/performance` | **go-only** (benchstat, pprof, GO_BEST §22/§31–§33/§53) |
| Features, debugging, docs | `skills/feature`, `skills/debugging`, `skills/documentation` | The procedures are **neutral**; the wording is "Go service" |
| Project rules win over loops rules | `WORKFLOW` §1, `agents/practices-researcher.md` | **neutral**. But it only looks for `.golangci.*` and `.editorconfig` among linter configs. |
| Roles | `ROLES.md`, `agents/*.md` (17 roles) | Researchers, planner, reviewers, product-reviewer and reflector are **neutral**. The architect, the six engineers and the qa-verifier are **go-only**: `go test`/`vet`, `go.mod`, `GOCACHE`, and `internal/...` and `*_test.go` ownership globs. |
| Repository creation and management | none | **missing**: no rules on project layout per ecosystem, lockfiles, toolchain pinning, release and publishing, CI shape, Renovate/Dependabot, or hygiene files. Dependency policy exists only for Go (GO_BEST §48–§49). |
| Security | GO_BEST §38 | **go-only**; no OWASP mapping and no SAST step |
| API and messaging resilience (timeouts, retries, outbox) | partly in `skills/jobs-and-state-machines` and `skills/consistency` | **partial / neutral** |
| Observability | GO_BEST §24 and §47 | **go-only** (no OpenTelemetry guidance) |
| Packaging and deploy | none | **missing** |

## Tooling (binary `loops`)

| Mechanism | Where | Language-neutral? |
|---|---|---|
| Gate: build, gofmt, vet, staticcheck, arch, test with race and coverage, govulncheck | `internal/application/gate/gate.go` (`runCheck` is a switch) | **go-only**. The loop, Skip/Only/FailFast and event recording are neutral. |
| Coverage of new code ≥70% against a git base | `internal/application/gate/coverage.go`, `internal/domain/coverage`, `internal/adapters/outbound/gocover` | Diff parsing and evaluation are **neutral**. Profile parsing (Go coverprofile only), `.go` and `_test.go` candidates, `DefaultExclude` and the generated-code marker are **go-only**. |
| Arch checker (dependency direction and SOLID limits as warnings) | `internal/application/archcheck` (`Parser` port), `internal/domain/archrule`, `internal/adapters/outbound/goparse` | The layer and size rules are **neutral**. The Go parser, third-party detection, forbidden `os`/`net/http` in the domain, purity calls (`time.Now`, `math/rand`, `go` statements), `init()`, and the `cmd/<x>/main.go` rule are **go-only**. |
| Post-edit format and lint | hook `go-post-edit` | **go-only** (`.go` files, `gofmt`/`goimports`, `go vet`) |
| Test-race guard | hook `race-guard`, `internal/application/guard/race.go` | **go-only**. The shell parsing behind it is neutral. |
| Run-scoped build cache and cleanup | hook `go-env`, `internal/domain/workspace`, `internal/adapters/outbound/runspace`, `loops go`, `loops run env` | **go-only** (`GOCACHE`/`GOTMPDIR`; artifacts `*.test`, `*.prof`, …). The baseline and cleanup decisions, and ELF, Mach-O and PE detection, are neutral. |
| Stop gate after source edits | `internal/domain/run/fold.go` (`GoDirty`), hook `stop-gate` | **go-only** (fires only after `.go` edits) |
| Hardware sizing | `internal/domain/hw` | The formula is **neutral**; the `GOFLAGS -p=2` hint and `go_version` are **go-only**. |
| Plan check (cap, disjoint ownership) | `internal/application/plancheck` | **neutral** |
| Read guard, model guard, tracing, report, status line, mod pane | hooks and adapters | **neutral** |
| Language detection | none | **missing**: `init` requires `go.mod` |

## Hard rules that every pack must keep

From `ROLES.md` and `WORKFLOW` §6:
- new code has unit tests, with **≥70% coverage of new code**;
- tests always run with the race detector or **the language's equivalent** (TAXONOMY CONC-04);
- project rules win, and every override is logged.
