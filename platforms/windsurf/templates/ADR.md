# ADR template

Write an ADR only for a decision that changes a contract: a port, the persisted or wire schema, the concurrency model, the command set, the consistency model (`specs/ARCHITECTURE.md §26`). Save as `docs/adr/NNNN-<slug>.md`. Decisions, not tutorials; no reasoning transcripts. At most one page.

```markdown
# ADR NNNN: <decision in a few words>

Status: proposed|accepted|superseded by NNNN | Date: <YYYY-MM-DD>

## Context
<forces and constraints, 3-6 lines>

## Decision
<what we do>

## Consequences
<what gets easier, what gets harder, what must change>

## Alternatives considered
- <option>: <why not>

## Impact
- Consistency (ordering, idempotency, clock/RNG):
- Performance:
- Persistence / compatibility (schema, API, events, config):
```
