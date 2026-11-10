# Plan template

Written by the `planner` as `plan.v<N>.md` (`specs/WORKFLOW.md §2.2`). Run `loops plan check` on it before handing it over. Keep every heading; the `plan-reviewer` checks completeness against `specs/PARALLEL_WORK.md §1`. Claim ids (C1...) are local to an agent and appear in reports as `<agent-id>:C<n>`. Say each thing once; point at `file:line` instead of pasting code.

```markdown
# Plan v<N>: <title>

## Summary
<3-6 lines: what changes, which layers, why this approach>

## Design (specs/ENGINEERING.md §2-§6)
- Data shape: <types, states and transitions this change adds or alters; how illegal states are made unrepresentable>
- Subtract first: <what is removed or simplified before anything is added, or "nothing to remove: <why>">
- Fit: <fits the current design | redesign: the design we would have built with this requirement from the start, and which waves reach it>
- Planned breaks: <intermediate state between waves (expand/contract, old path kept) and how it is undone | none>
- Levers: <repetitive work turned into a tool: the site done by hand, the lane that builds the tool, how its output is checked | none>

## Acceptance criteria
- AC-1 (source: ticket|derived): <observable outcome>
- AC-2 (source: ticket|derived): <observable outcome>

## Task packet
<filled templates/TASK_PACKET.md>

## Research highlights
- <fact> (<path:line | URL | spec section>)

## Concurrency
- Hardware cap: <max_parallel_agents> (source: research/hardware.json)
- Configured cap: <parallel.max_agents from .loops/config.json, default 100>
- Opt-in limit: <limits.max_agents or none>
- Effective cap: <min of the three>
- Peak agents per wave: W1=<n>, W2=<n> (never above the effective cap; larger waves are split into sub-waves W1a, W1b)
- Duplicated roles: <e.g. W1-domain-orders / W1-domain-payments: disjoint globs internal/domain/orders/**, internal/domain/payments/**> | none
- Test-run concurrency: <max_concurrent_test_runs>, each with the pack's env_hints (Go: GOFLAGS=-p=2)

## Contract (wave 0, architect + <id>-engineer)
- Languages: <id (root, variant) per pack from research/language.json, e.g. go (., -); typescript (web/, -)>
- Ports / interfaces (signatures):
- Typed commands / DTOs / read models:
- Domain terms (glossary, specs/ENGINEERING.md §8): <term: meaning, for each new term | none>
- Stubs so every package compiles:
- Dependencies (manifest + lockfile, frozen-install clean; Go: go get):
- Toolchain and repo files (<id>-engineer): pins, format/lint/typecheck configs, test and coverage prerequisites (specs/REPO_PRACTICES.md):
- Shared config (Makefile, CI, the composition root):
- Done when: each pack's build and lint checks pass: `loops gate --only <checks>` (Go: `--only build,vet`).

## Waves

### Wave 1: <goal>
| id | role | language | model - reason | job | owns | must not touch | tests (test files per agent) | claims |
|---|---|---|---|---|---|---|---|---|
| W1-domain | domain-engineer | go | sonnet - rules against a written contract | <scope> | internal/domain/<x>/** | <globs> | GOFLAGS=-p=2 go test -race -count=1 -cover ./internal/domain/<x>/... | C1, C2 |

`language` is the pack id the lane works in (passed as `Language:`; the agent loads `loops:lang-<id>`). Every new module or file has a named test file in the `tests` column, run under the pack's race policy; new-code coverage target is at least 70% (specs/TESTING.md §8).

**W1-domain: <short title>**
- Goal: <one sentence>
- Starts at: <file:line>
- Behaviour: <numbered rules, edge cases included>
- Owns: <globs>
- Must not touch: <globs, and who owns them>
- Claims to prove (C1...): <C1: statement; C2: statement>
- Done when: <tests that prove it>
- Out: <what looks in scope and is not, and who owns it>

### Wave 2: <goal>
<same table and blocks. A wave over the effective cap is split into sub-waves (Wave 1a, Wave 1b) and ids keep the wave letter, e.g. W1a-domain.>

## QA pairing
| implementer | qa-verifier | ACs served |
|---|---|---|

## Proof plan
- AC-1: <how product-reviewer proves it through the real interface: command, expected output; edge cases: invalid/empty/oversized input, auth, not-found, duplicates/retries, concurrency, downstream failure>

## Project rules that supersede loops
<accepted overrides copied from research/practices.md: loops rule | project rule | where; or "none">

## Risks and open questions
- <risk or question, with owner>
```
