---
name: debugging
description: Use when investigating or fixing a backend bug, regression, flaky test or production incident in a backend service in any language pack - five steps (reproduce, localize, reduce, fix, guard) that end in a regression test that fails before the fix and passes after it, shipped with the fix.
---

# Debugging and regression reproduction

## Load

- `specs/CONTEXT_DISCIPLINE.md §27, §29-30` (logs, error reports, repro artifacts).
- Your language: `loops:lang-<id>` (its `testing.md` for repeat and seed flags, `concurrency.md` for race tooling). Go: `specs/languages/go/RULES.md §13, §24` (ordering, observational logging).

## Goal

Turn a vague bug into a minimal reproducible artifact, fix its cause, and leave a test that keeps it fixed. Avoid megabytes of logs. Every bug fix ships with a regression test (`specs/TESTING.md §1`); the steps below are how you get one.

## Gather a compact bug identity

```text
command or request (method, path, body trimmed)
input / fixture / seed
build (commit, version) and config
expected vs actual
request or trace ID, failing step
relevant log window (<= 20 lines)
```

## The five steps

Do them in order. Each ends with something a different agent can check.

1. **Reproduce.** Make the bug happen on demand at the cheapest level: a unit test, then a fake-backed application test, then the running service. Fill in the bug identity above. Ends with: a command that shows the wrong result.
2. **Localize.** Find the **first divergence**, the earliest step where actual differs from expected: bisect the inputs, `git bisect`, compare two runs, or step through the layers inward from the handler (transport, application, domain, adapter, dependency), checking the boundary values at each port. Ends with: the owning package and `file:line`.
3. **Reduce.** Shrink the input and the setup until removing anything makes the bug vanish, then write that case as a test at the lowest level that still shows it (TESTING §1). Run it and watch it fail with the bug's symptom, not a typo or a missing import (`specs/ENGINEERING.md §7`). Ends with: a failing test. This is the guard's red.
4. **Fix.** Read the owning package and the invariant the bug broke, and fix the cause, not the symptom. A guard clause that only hides the crash is not a fix (`specs/ENGINEERING.md §10`). If two fixes on one premise have failed, attack the premise instead (§11). Ends with: the smallest change that turns the test green.
5. **Guard.** The guard is a **regression test that fails before the fix and passes after it, committed with the fix** (`specs/TESTING.md §1`). It is the step-3 test, not a guard clause in the code:
   - **fails before:** its failure before the fix is recorded under `## Red first` in your claims report, with the bug's symptom;
   - **passes after:** it passes now, run repeatedly under the pack's race policy (repeat runs, shuffled order with the failing seed, the race flag or substitute; Go: `-count=N -race -shuffle=on`), with the nearby tests and benchmarks;
   - **committed with the fix:** it is in your lane's change next to the fix, so it lands in the same commit (`specs/ENGINEERING.md §18`). Never leave it for a later lane or a follow-up.

   QA fails a bug-fix claim whose guard is missing any of the three (`agents/qa-verifier.md` step 6a).

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

The fix and its guard test, in one change. The claims report has the bug-fix claim with the guard test as its evidence, the guard's `## Red first` line, and a short root-cause note (the cause, why it was possible, how the test pins it), plus any invariant or doc update the cause revealed. No reasoning transcripts.

## Common rationalizations

The rows are binding (`ROLES.md` "Common rationalizations").

| Excuse | Why it's wrong | What to do instead |
|---|---|---|
| "I know what it is; no need to reproduce it." | An unreproduced fix cannot be shown to work (ENGINEERING §10). | Reproduce it at the cheapest level first (step 1). |
| "A null check stops the crash." | A guard around the symptom is not a fix (ENGINEERING §10). | Find the first divergence (step 2) and fix the cause (step 4). |
| "It only fails sometimes; it's flaky." | A flaky test is a bug (TESTING §9). | Repeat it under the race policy with the failing seed until it reproduces. |
| "I'll add the regression test after the fix." | A test written after the fix never showed its red (ENGINEERING §7). | Write the failing test before the fix (step 3). |
| "The fix is tiny; it doesn't need a guard." | Every bug fix ships with its regression test (TESTING §1), and QA fails the claim without one. | Write the guard (step 5); a tiny fix gets a tiny test. |
| "More logs will find it." | A log dump is not a repro (`specs/CONTEXT_DISCIPLINE.md §27`). | Localize by layer and reduce the input (steps 2-3). |
| "It passed once, so it's fixed." | One green run proves little for an intermittent bug. | Run the guard repeatedly under the race policy, with the failing seed (step 5). |
| "The second fix failed too; one more tweak will do it." | The same check failing twice means the premise is wrong (ENGINEERING §11). | Follow ENGINEERING §11 before a third attempt. |
