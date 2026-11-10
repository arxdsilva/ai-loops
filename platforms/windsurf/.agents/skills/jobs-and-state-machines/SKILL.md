---
name: jobs-and-state-machines
description: Use when implementing or reviewing background jobs, workflows, sagas, retries, schedulers, leases or locks, or any entity with a lifecycle (orders, payments, subscriptions) - explicit state machines, typed outcomes, hysteresis and indexed work discovery.
---

# Jobs and state machines

## Load

- `specs/languages/go/RULES.md §43` state transitions, `§34` ordering, `§53` escalation.
- `specs/ARCHITECTURE.md §17` state machines, `§12` ownership.

## State machines

Model lifecycles as explicit states with one transition function that checks the source state.

```go
type PaymentState uint8
const (Pending PaymentState = iota; Authorised; Captured; Refunded; Failed)
```

Each state defines:

- entry condition;
- action;
- completion;
- failure (with a typed reason);
- resource lifecycle (leases and locks held).

Transitions are visible functions or events, not ad hoc field assignments from several packages. Illegal transitions return a typed error and are tested.

## Typed outcomes

Expected failures (declined, out of stock, not eligible, lease lost) are typed results, not generic Go errors. Infrastructure errors are wrapped errors (`specs/languages/go/RULES.md §8`).

## Leases and locks

Support explicit lease keys (entity, row, queue partition, external resource).

A lease must release on: successful completion; cancellation; owner crash (TTL expiry); target deletion; replacement by another job; invalidation.

- Leases have a TTL and a heartbeat or fencing token; a stale owner cannot commit.
- Exactly one owner may hold an exclusive lease.
- Quantity moves (stock, balance) are atomic with respect to the location/ownership invariant: a unit exists in exactly one place at a time.

## Work discovery

Never: for every worker, scan every possible task. Use indexed candidates by type, partition, availability and urgency, then:

```text
filter -> cheap score -> small finalist set -> exact check -> acquire lease
```

Deterministic tie-break among candidates (priority, then sequence, then ID).

## Priority

Use explicit priority layers (for example forced, critical, scheduled, normal, background). Do not bury priority in callback order.

## Hysteresis and flapping

Any behaviour driven by a threshold or signal (autoscaling, circuit breakers, retries, alerts, throttles) uses different start and stop thresholds, or a minimum duration or cooldown, and ships a **no-flap test**: feed a signal oscillating around the threshold and assert the state changes at most once per cooldown.

Expensive searches run bucketed or invalidation-driven; emergency checks may run more often.

## Retries

Capped exponential backoff with jitter from an injected RNG and clock; retry only idempotent or keyed operations; a dead-letter path for exhausted retries.

## Tests

Cover: priority ordering; conflicting leases; cancellation cleanup; target deletion cleanup; no duplicate quantities; no two exclusive owners; unreachable-candidate fallback; deterministic tie-break; no-flap; illegal transitions; large-N discovery benchmark.
