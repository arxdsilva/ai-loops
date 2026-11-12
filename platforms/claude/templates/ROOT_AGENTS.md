# Root AGENTS.md template

`/loops:init` installs this as the target repository's `AGENTS.md` (merging with an existing one, never overwriting it). Replace `<module>` and keep the file short: it links to the specs instead of copying them (`specs/CONTEXT_DISCIPLINE.md §6`).

```markdown
# <module>: agent instructions

Read only what the current task needs.

## Where things are
- `.loops/`: loops config and run directories (`.loops/config.json`, `.loops/runs/<id>/`).
- Loops plugin specs and skills: run `loops path specs` for the absolute path (`ARCHITECTURE.md`, `SOLID.md`, `TESTING.md`, `REPO_PRACTICES.md`, `CONTEXT_DISCIPLINE.md`, `WORKFLOW.md`, and `languages/<id>/RULES.md` for each language below).
- Languages: <ids from `loops lang detect`, with their roots>.
- Package contracts: the nearest package `AGENTS.md`. Decisions: `docs/adr/`.

## Non-negotiables
- Dependencies point inward: adapters -> application -> domain.
- The domain stays headless: no HTTP, SQL, queue, filesystem or vendor types.
- Code that depends on an adapter, or an adapter that depends on the application, uses a small single-purpose consumer-owned port. Concrete types are built only in the wiring-only composition root.
- SOLID limits (operations, fields, port size, module size) are advisory warnings from `loops arch`; registration at load time, runtime-assigned module-level state and logic in the composition root are errors.
- Search before creating a shared primitive: this repo, then the standard library, then a well-known library.
- Third-party libraries only in the composition root, adapters and `platform`; never in the domain or application layers.
- Injected clock, RNG and IDs in authoritative code: no wall-clock reads, global random draws or spawned concurrent work in the domain.
- No global scans: use indexes, batching and invalidation.
- Need-, threat- or threshold-driven behaviour starts and stops at different thresholds (or has a minimum duration or cooldown) and ships a no-flap test.
- Hard gate: all new code has unit tests, new-code coverage is at least 70%, and every test run uses the race detector or the language pack's race substitute (`loops gate`, the test-guard hook). Never weaken or delete a test to get green.
- State changes enter as typed commands; retried operations carry an idempotency key.
- Remove unused code, config and loaders in the change that makes them unused.
- Read long files by `offset`/`limit`; hooks deny whole-file Reads over 500 lines and shell views over 150 lines.

## Precedence
This repository's own rules (this file, `CLAUDE.md`, `.cursor/rules`, CONTRIBUTING, linter configs) win over the loops defaults above. When an agent follows a project rule instead of a loops rule it records it with `loops run override --rule "<loops rule>" --project "<project rule, file:line>" --note "<what was done>"`; the list is posted on the PR as a comment.

## Workflow
Tickets go through `/loops:work <ticket>`. Status: `/loops:status`. Report: `/loops:report`.
```
