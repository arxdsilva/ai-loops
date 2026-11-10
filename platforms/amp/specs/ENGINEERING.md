# ENGINEERING — make it work, then make it right

The engineering discipline every loops run follows, phase by phase. **Make it work** shapes the plan and the code. **Make it right** shapes QA. **Practice** shapes how the run is kept and delivered. Cite as `ENGINEERING §n`. A rule that already lives in another spec is cited here, not restated.

Adapted from [ub-mode](https://github.com/unclebob/ub-mode) by Robert C. Martin (MIT License, © 2026 Robert C. Martin), itself partly based on poteto's rules. The quality tools in §13 are his [crapper](https://github.com/unclebob/crapper), [dryer](https://github.com/unclebob/dryer) and [mutator](https://github.com/unclebob/mutator).

| § | Rule | Phase | Who applies it | Where it shows |
|---|---|---|---|---|
| 1 | Smallest edit, one decision in one place | plan, implement | planner, implementers | lane scope; dryer finding (§13) |
| 2 | Data shape first | plan | planner, architect | plan `## Design`; wave 0 contract |
| 3 | Subtract before you add | plan | planner | plan `## Design` |
| 4 | Redesign, don't bolt on | plan | planner, plan-reviewer | plan `## Design`; blocking review item |
| 5 | Every unit ends checkable | plan | planner, plan-reviewer | waves; claims |
| 6 | Build the lever | plan, implement | planner, implementers | plan `## Design`; a tool lane |
| 7 | Test first, and see the red | implement | implementers | claims report `## Red first` |
| 8 | Model the domain; parse at the boundary | implement | architect, domain and adapter engineers | types; glossary; QA finding |
| 9 | Idempotent operations, one writer | plan, implement | planner, implementers | `specs/ARCHITECTURE.md §10, §12, §16` |
| 10 | Fix root causes | QA | implementers, qa-verifier | QA finding |
| 11 | Attack the premise | every loop | lead, the looping author | `## Premise`; `premise` note event |
| 12 | Test behaviour; no hollow tests | QA | qa-verifier | QA `## Test honesty` |
| 13 | CRAP, duplication, mutation | QA | `qa-quality` | `qa/quality.r<N>.md` |
| 14 | Refactor under a pin | implement, QA | implementers, qa-verifier | claims evidence |
| 15–18 | Log, context, cleanup, ordered commits | every phase | lead, product-reviewer, reflector | events, `reflection.md`, commits |

## Make it work

### 1. Smallest edit, one decision in one place

- Ship the smallest change that meets the acceptance criteria. Unrelated refactors stay out (`skills/feature` Avoid).
- A rule, constant, mapping or default lives in exactly one place. A second copy is a defect, whoever wrote the first. The quality pass reports structural copies (§13).

### 2. Data shape first

- Get the types, states and their transitions right before the logic. The plan's `## Design` names them, and the architect writes them in wave 0 (contract first, WORKFLOW §1).
- Do first what every later step depends on: the contract, the toolchain, a lever (§6).
- If a concurrent writer would matter, do not share the state (§9).

### 3. Subtract before you add

The plan names what the change removes or simplifies before it adds anything: dead code, a flag nobody sets, a layer or wrapper that no longer earns its place, a special case the new design makes unnecessary. Write "nothing to remove" with the reason only after looking. A subtraction may be its own lane in an early wave. It stays inside the ticket's area (§1).

### 4. Redesign, don't bolt on

When a requirement does not fit the current design, the plan describes the design you would have built had the requirement been there from the start. It then reaches that design in waves. Signs of a bolt-on, each a blocking plan-review item:
- a boolean or mode parameter that switches behaviour inside a shared function;
- a special case keyed on one caller;
- a parallel copy of a type or a function;
- a second path that skips the checks of the first.

### 5. Every unit ends checkable

- Aim each lane at an end state a different agent can verify (claims, WORKFLOW §1). Finish one unit and check it before the next depends on it (producers before consumers, WORKFLOW §2.3).
- A planned break between waves is allowed: an expand-then-contract migration, or an old path kept until its callers move. The plan names it and how it is undone, and the run ends with it undone. The gate is green at the end of the run, and the build is green at the end of every wave.

### 6. Build the lever

When a lane would repeat one transformation across many sites, or the work is not obvious at a glance:
1. do one site by hand;
2. write the tool (a script, codemod or generator) and check that its output on that site matches the hand-made one;
3. run it on the rest.

The plan's `## Design` names the lever and the lane that builds it. The tool lives in the run's tmp dir (`loops run env`) unless the project wants to keep it.

### 7. Test first, and see the red

Strict TDD (`specs/TESTING.md §10`):
1. Write the failing test before the code.
2. Watch it fail **for the reason you expect**. A test that fails on a typo or a missing import has not shown anything yet.
3. Make the smallest change that turns it green, then refactor.

The claims report records one line per new test under `## Red first`, giving the test and the failure it showed before the change. When an honest test is impractical, do not write a dishonest one to reach the coverage floor (`specs/TESTING.md §8.1`). Make the code testable instead, usually by moving the decision into a pure function behind its boundary (§8).

### 8. Model the domain; parse at the boundary

- **Illegal states are unrepresentable:** value objects, sum types or sealed hierarchies, smart constructors. A value that cannot be invalid needs no check downstream.
- **Glossary:** each domain term has one name and one meaning, kept in the project's glossary or in the domain package's `AGENTS.md` `## Glossary` (`templates/PACKAGE_AGENTS.md`). The plan's contract lists new terms. If the code is already clear and local, leave it.
- **Boundary discipline:** parse and validate outside data once, where it enters (`specs/ARCHITECTURE.md §10`); trust it inside; keep the business decision in a pure function.
- **Do not silence the compiler:** no blanket suppression (`//nolint`, `# type: ignore`, `@SuppressWarnings`, `!!`, an `any` or `unsafe` cast) without a reason on the same line. An unexplained one is a QA finding.

### 9. Idempotent operations, one writer

- Running an operation twice, or again after a crash halfway through, ends in the same state (`specs/ARCHITECTURE.md §10, §16`; `skills/consistency`).
- Separate before you serialize. If two actors do not need the same mutable thing, give each its own. If they do, allow one writer, enforced by the structure, not by convention (`specs/ARCHITECTURE.md §12`).

## Make it right

### 10. Fix root causes

Reproduce it, fix the cause, and keep the regression test (`skills/debugging`). A guard that only hides the crash is not a fix: a null check, a catch-all, a retry or a default around the symptom. QA marks a fix FAILED when it has no reproducing test and no stated cause.

### 11. Attack the premise

When two fixes that share one premise fail the same check, the third attempt is not another fix. The trigger is the same claim, gate check, AC or blocking plan item failing in two rounds of any loop (WORKFLOW §4). The author does three things first:
1. Writes the premise both fixes shared, in one sentence, under `## Premise` in its report.
2. Names the actors (functions, types, services, lanes) that hold the conflicting responsibility.
3. Removes the assignment that puts it in more than one place, or that puts it in the wrong place. That is a redesign of that piece (§4), and it may need a plan revision.

The lead records it with `loops run event --kind note --name premise --note "<sentence>"`, passes it to any escalated agent, and the reflector lists it.

### 12. Test behaviour; no hollow tests

Tests call the code the way its user does and check what that user would see (`specs/TESTING.md §3`). **The hollow-test rule:** if a test would still pass with every function under test returning nothing (its zero value, null, an empty collection, no effect), it proves nothing. QA marks the claim it supports FAILED. Mutation testing (§13) makes this mechanical.

### 13. CRAP, duplication, mutation

The quality pass (WORKFLOW §2.4) measures the code the run added or changed:

| Metric | Bar | Fix |
|---|---|---|
| CRAP = CC² × (1 − coverage)³ + CC, per function | < 10 | split the function, test it, or both. A flat dispatch (`switch`, `case`, `match`) may stay above the bar with a reason. |
| Structural duplication between two forms | similarity < 0.8 | merge into one place (§1) |
| Mutation | every covered mutant killed | **survivor:** a test that fails for it. **Uncovered:** a test that reaches it. **Unreachable:** delete the dead code. |

- **Tools.** crapper, dryer and mutator cover Clojure, Java, Go, TypeScript, Rust and Python. Other packs use the project's configured native tools (`skills/lang-<id>/testing.md`: PIT, Stryker, cargo-mutants, mutant, Infection, …). With neither, §12 applies by review. How to run them: `skills/quality`.
- **Scope.** Only functions the run added or changed are blocking. Pre-existing debt elsewhere is a note for the reflection.

### 14. Refactor under a pin

1. Pin what the code does: tests that pass before you start.
2. Change the structure.
3. Prove the behaviour did not change: the same tests pass, with their assertions unchanged.

A refactor lane's claims cite the pinning tests. A lane that changes structure and assertions together is two lanes.

## Practice

### 15. Keep a log

Every action of the run lands in `events.jsonl` (WORKFLOW §5): phases, lanes, verdicts, loop-backs, gates, proofs. A decision and its reason is a `loops run event --kind note`. The report (`loops report`) and `reflection.md` are the log reported at the end.

### 16. Guard the context window

The lead keeps the main thread small and sends the bulk to role agents, keeping their summaries (WORKFLOW §1; `specs/CONTEXT_DISCIPLINE.md`).

### 17. Clean up

Find what is safe to remove, confirm nothing in use needs it, then reclaim it. Uncommitted work waits for a decision (WORKFLOW §2.6 step 5: unknown new files are listed for review, never deleted).

### 18. Small ordered commits

The product-reviewer commits the run as small commits in dependency order: wave 0 first, then each wave's lanes. Each commit has:
- a conventional subject (`specs/REPO_PRACTICES.md §10`);
- a body that says why, what changed, and how it was proven (claim ids, the QA file, the AC proofs).

It then pushes the branch and opens the PR (WORKFLOW §2.5). A project rule to push straight to the main line wins (WORKFLOW §1).
