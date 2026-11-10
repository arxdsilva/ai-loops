---
name: performance
description: Use when optimizing or reviewing latency, throughput, CPU, memory, allocations, queries, data layout, update frequency or concurrency in a backend service in any language pack, based on measurement; the benchmark and profiler workflow (Go: benchstat and pprof).
---

# Performance and scale

## Load

- Your language: `loops:lang-<id>` and its `idioms.md`, `testing.md` (benchmark harness) and `concurrency.md`. Go: `specs/languages/go/RULES.md §22` allocation, `§31` benchmarks, `§32` complexity, `§33` data layout, `§53` escalation.
- `specs/ARCHITECTURE.md §12` ownership (what may be parallel).

## Escalation order

```text
1. eliminate O(N^2) and global scans (N+1 queries, full-table scans)
2. reduce update or polling frequency
3. add indexes
4. add dirty invalidation
5. skip inactive state
6. batch operations
7. reduce allocations
8. improve data layout and locality
9. cache (with explicit invalidation)
10. parallelise pure work
11. micro-optimise only with evidence
```

Do not jump to concurrency, caching or unsafe code first.

## Scale mindset

Every hot path answers:

```text
N (items, rows, requests per second):
frequency:
complexity:
index / query plan:
invalidation trigger:
allocations:
parallelisable pure work:
```

## Measurement

Before and after, record: time per op, bytes and allocations per op where the runtime reports them, input size, concurrency, and for services p50/p99 latency and throughput. Run the pack's benchmark harness several times on each side (`skills/lang-<id>/testing.md`) and compare statistically, not one run against one run. Profile CPU, memory and contention with the language's profiler before changing code. Go example:

```bash
go test -run=^$ -bench=BenchmarkX -benchmem -count=10 ./pkg > old.txt
# change
go test -run=^$ -bench=BenchmarkX -benchmem -count=10 ./pkg > new.txt
benchstat old.txt new.txt
```

Go also has `pprof` (CPU, heap, allocs, mutex, block), execution traces and `-gcflags='-m=2'` for escapes (confirm with a benchmark). Never claim an improvement without the before/after numbers.

## Data rules

Prefer dense arrays (Go slices), stable compact IDs, bit flags, caller-provided scratch buffers where proven useful, and queries bounded by an index. Avoid pointer-heavy graphs, repeated whole-table scans, temporary maps per request, string keys in hot loops, a thread, task or goroutine per item, and allocating iterator abstractions without evidence.

## Database and I/O

Check query plans (`EXPLAIN`); batch round trips; paginate with keyset where deep offsets appear; bound result sizes; set timeouts; reuse connections and clients.

## Concurrency

Parallelise only pure or snapshot-based work. A parallel stage reads an immutable view, runs on a bounded pool, writes only to per-task results, derives randomness per task, and is committed in deterministic order (`skills/consistency`). Keep it only if the A/B shows a gain that matters, and ship an equality test at pool sizes 1, 2 and 8. Check it under the pack's race policy (WORKFLOW §6).

## Regression rule

A change that meaningfully worsens a core benchmark must be fixed or carry a documented trade-off in the claims report. Unsafe code (Go `unsafe`, Rust `unsafe`, native extensions) needs an ADR, benchmark proof, tests and a portability analysis.
