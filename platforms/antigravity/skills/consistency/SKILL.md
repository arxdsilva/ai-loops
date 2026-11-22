---
name: consistency
description: Use when implementing or reviewing code whose results must be repeatable and safe under retries and concurrency - injected clock and RNG, ordering and tie-breaks, idempotency keys, worker pipelines, schema migrations with versions and fixtures, atomic writes, untrusted persisted input.
---

# Consistency: repeatable, retry-safe, concurrency-safe

## Load

- Your language: `loops:lang-<id>` (clock, RNG and concurrency idioms). Go: `specs/languages/go/RULES.md §13` (ordering and clocks), `§20` time, `§21` randomness, `§45` migrations, `§46` idempotency.
- `specs/ARCHITECTURE.md §12` one owner per state, `§18` persistence, `§20` time boundary.

## Procedure

1. **Clock.** Authoritative code takes a `Clock` port; no `time.Now` outside `cmd`, adapters and `platform`. Tests use a fake clock and never sleep.
2. **Randomness.** No global `math/rand`. Inject an RNG or ID generator; derive randomness from stable inputs so an unrelated draw does not shift later outcomes. Never couple the number of draws to an iteration count.
3. **Ordering.** Where records, actors or results can conflict, define a tie-break (priority, then sequence, then ID; `ORDER BY` with a unique column). Never rely on hash-map iteration, thread or task completion order, or storage order unless guaranteed.
4. **Idempotency.** Anything retried (webhook, queue redelivery, client retry) carries an idempotency key; applying it twice has the same effect as once. Store the key and result atomically with the state change; define the key's TTL and the behaviour on key reuse with a different payload.
5. **Worker pipeline.** Workers compute from immutable inputs and never mutate live state:

```text
snapshot -> request(sequence, revision) -> parallel compute -> collect
-> deterministic order -> validate revision -> commit on the owner
```

6. **Schema migrations.** Versioned and explicit; never infer from "field missing" when a version exists. Prefer expand, backfill, switch, contract. Test with historical fixtures and corrupt/truncated input. Never "best effort" corrupt authoritative state.
7. **Atomic writes.** Files: temp file, fsync where policy requires, rename. Databases: one transaction per command; outbox for events that must publish with the change.
8. **Untrusted persisted or external input.** Bound sizes, counts, nesting, string lengths, decompression expansion, referenced IDs and schema versions (Go: `specs/languages/go/RULES.md §28, §38`).
9. **Hashes and checksums.** Stable ordering, fixed numeric encoding, no pointers, no wall-clock fields.

## Tests to add

Any consistency-sensitive change adds one or more of: repeat-run equality; retry/duplicate delivery equality; round trip (write, read, equal); migration fixtures old to new; worker-count equality (vary the pool size and scheduler threads, e.g. Go `GOMAXPROCS`); concurrent writers under the pack's race policy.

## Review red flags

`time.Now()` in domain; global RNG; draw count tied to iteration count; decisions from map range; channel receive order deciding outcomes; a worker mutating shared state; retry without idempotency key; migration inferred from missing fields; non-atomic write; unbounded external input; hashing raw structs with padding or pointers.
