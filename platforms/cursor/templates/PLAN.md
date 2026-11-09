# Plan template

Written by the `planner` as `plan.v<N>.md` (`specs/WORKFLOW.md §2.2`). Keep every heading; the `plan-reviewer` checks completeness against `specs/PARALLEL_WORK.md §1`. Claim ids (C1...) are local to an agent and appear in reports as `<agent-id>:C<n>`. Say each thing once; point at `file:line` instead of pasting code.

```markdown
# Plan v<N>: <title>

## Summary
<3-6 lines: what changes, which layers, why this approach>

## Acceptance criteria
- AC-1 (source: ticket|derived): <observable outcome>
- AC-2 (source: ticket|derived): <observable outcome>

## Task packet
<filled templates/TASK_PACKET.md>

## Research highlights
- <fact> (<path:line | URL | spec section>)

## Contract (wave 0, architect)
- Ports / interfaces (signatures):
- Typed commands / DTOs / read models:
- Stubs so every package compiles:
- Dependencies (go get):
- Shared config (Makefile, CI, cmd/**, internal/compose/**):
- Done when: `go build ./...` and `go vet ./...` pass.

## Waves

### Wave 1: <goal>
| id | role | model - reason | job | owns | must not touch | tests | claims |
|---|---|---|---|---|---|---|---|
| W1-domain | domain-engineer | sonnet - rules against a written contract | <scope> | internal/domain/<x>/** | <globs> | GOFLAGS=-p=2 go test -race -count=1 -cover ./internal/domain/<x>/... | C1, C2 |

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
<same table and blocks>

## QA pairing
| implementer | qa-verifier | ACs served |
|---|---|---|

## Proof plan
- AC-1: <how product-reviewer proves it through the real interface: command, expected output; edge cases: invalid/empty/oversized input, auth, not-found, duplicates/retries, concurrency, downstream failure>

## Risks and open questions
- <risk or question, with owner>
```
