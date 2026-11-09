# Task packet

Establish it before coding (internally, or in the plan's "Task packet" section). One line per field; leave out fields that do not apply. Do not repeat rules already in the root `AGENTS.md` or the specs (`specs/CONTEXT_DISCIPLINE.md §5`).

```text
Task ID:
Objective:
Owner package:
Allowed / forbidden packages:
Skills (skills/<name>):
Read (root AGENTS.md, nearest package AGENTS.md, spec sections):
Input / output interfaces (ports, commands, DTOs):
Acceptance tests:
Benchmarks / hot path / performance constraints:
Consistency impact (ordering, idempotency, clock/RNG, migrations):
Compatibility impact (API, persisted schema, events, config):
Verification commands:
```
