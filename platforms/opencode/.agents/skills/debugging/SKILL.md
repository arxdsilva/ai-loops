---
name: debugging
description: Use when investigating a backend bug, regression, flaky test or production incident in a backend service in any language pack - reduce it to a minimal reproducible artifact, find the first divergence, fix the root cause and keep a regression test.
---

# Debugging and regression reproduction

## Load

- `specs/CONTEXT_DISCIPLINE.md §27, §29-30` (logs, error reports, repro artifacts).
- Your language: `loops:lang-<id>` (its `testing.md` for repeat and seed flags, `concurrency.md` for race tooling). Go: `specs/languages/go/RULES.md §13, §24` (ordering, observational logging).

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
6. Inspect the owning package and its invariant; fix the root cause, not the symptom. A guard that only hides the crash is not a fix (`specs/ENGINEERING.md §10`). If two fixes on one premise have failed, attack the premise instead (§11).
7. Verify repeatedly under the pack's race policy: repeat runs, shuffled order with the failing seed, the race flag or substitute (Go: `-count=N -race -shuffle=on`); run nearby tests and benchmarks.

## Logs

Filter by request ID, entity ID, time range or component. Logging stays observational: it must not change ordering, randomness or state. Do not add logging that would stay in the fix unless operators need it.

## Common classes to check early

- stale revision or cache committed; missing invalidation;
- lease or lock not released; deleted entity still referenced;
- implicit hash-map or storage ordering; tie-break missing;
- RNG or clock coupling; time zone or DST;
- retry without idempotency key; duplicate delivery;
- transaction boundary too wide or too narrow; read-after-write on a replica;
- cancellation too early; leaked thread, task or goroutine; data race (the pack's race policy);
- migration or schema mismatch; omitted field in a DTO;
- worker completion order leaking into results.

## Deliverable

The fix leaves a regression test or fixture, a concise root-cause note in the claims report (cause, why it was possible, how the test pins it), and any invariant or doc update the cause revealed. No reasoning transcripts.

## Common rationalizations

The rows are binding (`ROLES.md` "Common rationalizations").

| Excuse | Why it's wrong | What to do instead |
|---|---|---|
| "I know what it is; no need to reproduce it." | An unreproduced fix cannot be shown to work (ENGINEERING §10). | Reproduce it at the cheapest level first. |
| "A null check stops the crash." | A guard around the symptom is not a fix (ENGINEERING §10). | Find the first divergence and fix the cause. |
| "It only fails sometimes; it's flaky." | A flaky test is a bug (TESTING §9). | Repeat it under the race policy with the failing seed until it reproduces. |
| "I'll add the regression test after the fix." | A test written after the fix never showed its red (ENGINEERING §7). | Write the failing test or fixture before the fix (step 5). |
| "More logs will find it." | A log dump is not a repro (`specs/CONTEXT_DISCIPLINE.md §27`). | Narrow by layer and reduce the input (steps 3-4). |
| "It passed once, so it's fixed." | One green run proves little for an intermittent bug. | Verify repeatedly: shuffled order, the failing seed, the race policy (step 7). |
| "The second fix failed too; one more tweak will do it." | The same check failing twice means the premise is wrong (ENGINEERING §11). | Follow ENGINEERING §11 before a third attempt. |
