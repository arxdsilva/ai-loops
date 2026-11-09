---
name: feature
description: Use when implementing a scoped backend feature or behaviour change in a Go service (new endpoint, use case, rule, consumer or job), or adding a package or command. Covers ownership, TDD, layering, consistency and the completion gate.
---

# Feature implementation

Specs live at the absolute path in your prompt (`loops path specs`). Cite by `specs/<FILE>.md §N`.

## Load

- Root `AGENTS.md`, the nearest package `AGENTS.md`, the public contracts and existing tests of the behaviour.
- Layering and ports: `specs/HEXAGONAL_ARCHITECTURE.md §3, §6`. Definition of done: `specs/WORKFLOW.md §8`.
- Context rules: `specs/CONTEXT_DISCIPLINE.md §8-10`.

## Steps

1. Restate the behaviour as observable acceptance criteria.
2. Identify the owning package; search for existing canonical types and primitives before adding any (`specs/GO_BEST_PRACTICES.md §48`).
3. Write or update the failing behavioural test first.
4. Implement the smallest complete change in the right layer: business rules in `internal/domain`, orchestration in `internal/application`, technology in `internal/adapters`. Nothing in HTTP handlers, queue consumers, SQL repositories or `cmd`.
5. Add wiring only where the feature crosses a port; construct concrete types at the composition root.
6. Check consistency (`skills/consistency`) and compatibility: API, persisted schema, events, config, migrations.
7. Run targeted tests with `-race`, then `go vet`; benchmark if a hot path changed (`skills/performance`).
8. Update the package `AGENTS.md` if a contract changed (`skills/documentation`).
9. Finish with the claims report (`templates/CLAIMS_REPORT.md`).

## New package or command

Create one only when a task needs it. Give it a `AGENTS.md` from `templates/PACKAGE_AGENTS.md` once it is real. `cmd/<tool>/main.go` is wiring only (`specs/SOLID_GO.md §8a`); only the architect edits `cmd/**` and `internal/compose/**` (`ROLES.md`).

## Answer before coding

```text
Owner / State read / State written / Transaction boundary / Update frequency / Ordering
Index or query used / Allocation behaviour / Persistence / Idempotency / Config surface / Observability
```

## Avoid

New global registries; transport-driven domain mutation; refactors unrelated to the acceptance criteria; duplicate IDs or types; global scans where an index exists; goroutine per item; wall-clock in domain; map-order decisions; a framework for one feature.

## Completion gate

`specs/WORKFLOW.md §8` and: failure paths explicit and tested; compatibility addressed; benchmarks exist or stay valid for hot changes; `loops arch` shows no errors.
