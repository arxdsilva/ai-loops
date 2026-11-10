---
name: feature
description: Use when implementing a scoped backend feature or behaviour change in any language pack (new endpoint, use case, rule, consumer or job), or adding a package, module or command. Covers ownership, TDD, layering, consistency and the completion gate.
---

# Feature implementation

Specs live at the absolute path in your prompt (`loops path specs`). Cite by `specs/<FILE>.md §N`.

## Load

- Root `AGENTS.md`, the nearest package `AGENTS.md`, the public contracts and existing tests of the behaviour.
- Layering and ports: `specs/ARCHITECTURE.md §3, §6`. Definition of done: `specs/WORKFLOW.md §8`.
- Context rules: `specs/CONTEXT_DISCIPLINE.md §8-10`.
- Your language (`Language:` in your prompt): `loops:lang-<id>` for the layout, commands and idioms.

## Steps

1. Restate the behaviour as observable acceptance criteria.
2. Identify the owning package; search for existing canonical types and primitives before adding any (Go: `specs/languages/go/RULES.md §48`).
3. Write or update the failing behavioural test first.
4. Implement the smallest complete change in the right layer: business rules in the domain layer, orchestration in the application layer, technology in the adapters (the pack's layout tree; Go: `internal/domain`, `internal/application`, `internal/adapters`). No business rules in HTTP handlers, queue consumers, SQL repositories or entry points.
5. Add wiring only where the feature crosses a port; construct concrete types at the composition root.
6. Check consistency (`skills/consistency`) and compatibility: API, persisted schema, events, config, migrations.
7. Run the pack's checks on your scope: `loops gate --only <check>,… --pkgs <owned>` while iterating, then `loops gate --pkgs <owned>` (the race policy is the pack's, WORKFLOW §6); benchmark if a hot path changed (`skills/performance`).
8. Update the package `AGENTS.md` if a contract changed (`skills/documentation`).
9. Finish with the claims report (`templates/CLAIMS_REPORT.md`).

## New package or command

Create one only when a task needs it. Give it a `AGENTS.md` from `templates/PACKAGE_AGENTS.md` once it is real. Entry points are wiring only (`specs/SOLID.md §8a`; Go: `cmd/<tool>/main.go`); only the architect edits entry points and the composition root (Go: `cmd/**`, `internal/compose/**`; `ROLES.md`).

## Answer before coding

```text
Owner / State read / State written / Transaction boundary / Update frequency / Ordering
Index or query used / Allocation behaviour / Persistence / Idempotency / Config surface / Observability
```

## Avoid

New global registries; transport-driven domain mutation; refactors unrelated to the acceptance criteria; duplicate IDs or types; global scans where an index exists; a thread, task or goroutine per item; wall-clock in domain; decisions from hash-map iteration order; a framework for one feature.

## Completion gate

`specs/WORKFLOW.md §8` and: failure paths explicit and tested; compatibility addressed; benchmarks exist or stay valid for hot changes; `loops arch` shows no errors.
