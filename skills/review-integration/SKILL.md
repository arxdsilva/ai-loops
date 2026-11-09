---
name: review-integration
description: Use when reviewing a plan, a diff or a wave of agent work against architecture, consistency, performance and test standards - plan-reviewer, qa-verifier and product-reviewer procedure, reject list and multi-agent integration.
---

# Review and integration

## Load

Read only the acceptance criteria, the diff, the affected package `AGENTS.md`, the reported tests and benchmarks. Definition of done: `specs/WORKFLOW.md §8`. Open more source only when needed (`specs/CONTEXT_DISCIPLINE.md §19`).

## Review order

1. correctness
2. architecture ownership (`specs/HEXAGONAL_ARCHITECTURE.md §29`)
3. consistency: clock, RNG, ordering, idempotency (`skills/consistency`)
4. invariants
5. data complexity and performance
6. concurrency and `-race`
7. persistence, API and config compatibility
8. tests: behaviour named, failure paths, no weakened tests
9. Go API and readability (`specs/GO_BEST_PRACTICES.md §51`)
10. documentation: package `AGENTS.md` for changed contracts
11. observability: errors explicit, logs and metrics where operators need them

## Reject

- an unrelated broad refactor;
- a weakened or deleted test;
- a duplicate canonical type;
- domain depending on an adapter, transport or storage type;
- global mutable state, `init()` registration;
- a goroutine per item without a bound;
- `time.Now` or global RNG in authoritative code;
- decisions from map iteration;
- a silent persisted-schema or API break;
- a retried operation without an idempotency key;
- a new hot global scan or N+1 query;
- a performance claim without a benchmark;
- unbounded external input;
- hand-edited generated code;
- business rules in a handler or repository;
- a claim without evidence, or evidence the reviewer cannot re-run.

## Reviewing a plan (plan-reviewer)

Use `templates/PLAN_REVIEW.md`. Check completeness against `specs/PARALLEL_WORK.md §1`: contract, waves table, per-agent blocks, disjoint ownership, models with reasons, QA pairing, proof plan. Findings are numbered, each with what, why and a suggested change; only blockers block.

## Verifying claims (qa-verifier)

Independently re-run the evidence, read the code, attack the claim with edge cases and `-race`, and report per claim `VERIFIED` or `FAILED` with a repro (`templates/QA_VERDICT.md`). List unclaimed problems separately. Do not fix code.

## Multi-agent integration

Before accepting a wave: collect claims reports; compare interface changes; find overlapping files; resolve each canonical contract once; merge in dependency order; run `loops gate` and the affected benchmarks (`specs/PARALLEL_WORK.md §6`). Never keep two competing abstractions to "clean up later".

## Report check

Each report states: changed contracts, invariants, tests, benchmark delta, consistency impact, compatibility impact, limits and follow-ups. Do not request chain-of-thought.
