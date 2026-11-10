# Package AGENTS.md template

A package (module) `AGENTS.md` says only what is local to its module, in 300-800 words. Shared rules (inward dependencies, headless domain, injected clock/RNG, the race policy) live in the root `AGENTS.md` and the specs; never repeat them here. Leave out a section with nothing package-specific; never write "None". Written by `docs-writer` from facts the lead supplies, or by the implementer when a contract changes.

```markdown
# <path relative to the repo root>

## Purpose
What the package owns and why it exists.

## Owns
Its types and their roles; method or field counts only when near a limit (specs/SOLID.md §8).

## Depends on
Imports beyond the layer default (third-party libraries, sibling leaves).

## Must not depend on
Module-specific bans: the parent module, siblings above it, modules that import it.

## Contracts
What callers rely on, each with the test that pins it.

## Invariants
What must stay true, each with its test.

## Hot paths
Per-request or per-item code, with its benchmark and numbers.

## Tests
Where they live, guards in other packages, anything special about running them.

## Notes
Known design constraints, and recipes for extending the package.
```
