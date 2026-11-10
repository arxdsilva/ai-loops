---
name: debugging
description: Use when investigating a backend bug, regression, flaky test or production incident in a Go service - reduce it to a minimal reproducible artifact, find the first divergence, fix the root cause and keep a regression test.
---

# Debugging and regression reproduction

## Load

- `specs/CONTEXT_DISCIPLINE.md §27, §29-30` (logs, error reports, repro artifacts).
- `specs/languages/go/RULES.md §13, §24` (ordering, observational logging).

## Goal

Turn a vague bug into a minimal reproducible artifact: a failing test, a recorded request, a fixture or a seed. Avoid megabytes of logs.

## Gather a compact bug identity

```text
command or request (method, path, body trimmed)
input / fixture / seed
build (commit, version) and config
expected vs actual
request or trace ID, failing step
relevant log window (<= 20 lines)
```

## Workflow

1. Reproduce at the cheapest level (unit test, then fake-backed application test, then running service).
2. Find the **first divergence**: the earliest step where actual differs from expected (bisect inputs, `git bisect`, compare two runs, step through layers inward from the handler).
3. Narrow by layer: transport, application, domain, adapter, dependency. Check the boundary values at each port.
4. Reduce the input and the setup until removing anything makes the bug vanish.
5. Write the failing test or fixture first.
6. Inspect the owning package and its invariant; fix the root cause, not the symptom.
7. Verify repeatedly (`-count=N`, `-race`, shuffled order); run nearby tests and benchmarks.

## Logs

Filter by request ID, entity ID, time range or component. Logging stays observational: it must not change ordering, randomness or state (`specs/languages/go/RULES.md §24`). Do not add logging that would stay in the fix unless operators need it.

## Common classes to check early

- stale revision or cache committed; missing invalidation;
- lease or lock not released; deleted entity still referenced;
- implicit map or storage ordering; tie-break missing;
- RNG or clock coupling; time zone or DST;
- retry without idempotency key; duplicate delivery;
- transaction boundary too wide or too narrow; read-after-write on a replica;
- context cancelled too early; goroutine leak; data race (`-race`);
- migration or schema mismatch; omitted field in a DTO;
- worker completion order leaking into results.

## Deliverable

The fix leaves a regression test or fixture, a concise root-cause note in the claims report (cause, why it was possible, how the test pins it), and any invariant or doc update the cause revealed. No reasoning transcripts.
