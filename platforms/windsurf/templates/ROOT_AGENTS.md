# Root AGENTS.md template

`/loops:init` installs this as the target repository's `AGENTS.md` (merging with an existing one, never overwriting it). Replace `<module>` and keep the file short: it links to the specs instead of copying them (`specs/CONTEXT_DISCIPLINE.md §6`).

```markdown
# <module>: agent instructions

Read only what the current task needs.

## Where things are
- `.loops/`: loops config and run directories (`.loops/config.json`, `.loops/runs/<id>/`).
- Loops plugin specs and skills: run `loops path specs` for the absolute path (`GO_BEST_PRACTICES.md`, `HEXAGONAL_ARCHITECTURE.md`, `SOLID_GO.md`, `CONTEXT_DISCIPLINE.md`, `WORKFLOW.md`).
- Package contracts: the nearest package `AGENTS.md`. Decisions: `docs/adr/`.

## Non-negotiables
- Dependencies point inward: adapters -> application -> domain.
- The domain stays headless: no HTTP, SQL, queue, filesystem or vendor types.
- Code that depends on an adapter, or an adapter that depends on the application, uses a small single-purpose consumer-owned interface. Concrete types are built only in a wiring-only `cmd/*/main.go` or `internal/compose`.
- SOLID limits (methods, fields, interface size, package size) are advisory warnings from `loops arch`; `init()` registration, runtime-assigned package vars and logic in `cmd` are errors.
- Search before creating a shared primitive: this repo, then the standard library, then a well-known library.
- Third-party libraries only in `cmd`, adapters and `platform`; never in `domain`, `application` or `pkg`.
- Injected clock and RNG in authoritative code: no `time.Now`, global `math/rand` or `go` statements in the domain.
- No global scans: use indexes, batching and invalidation.
- Need-, threat- or threshold-driven behaviour starts and stops at different thresholds (or has a minimum duration or cooldown) and ships a no-flap test.
- Run tests with `-race`, always. Never weaken or delete a test to get green.
- State changes enter as typed commands; retried operations carry an idempotency key.
- Remove unused code, config and loaders in the change that makes them unused.
- Read long files by `offset`/`limit`; hooks deny whole-file Reads over 500 lines and shell views over 150 lines.

## Workflow
Tickets go through `/loops:work <ticket>`. Status: `/loops:status`. Report: `/loops:report`.
```
