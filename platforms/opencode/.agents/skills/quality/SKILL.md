---
name: quality
description: Use for the QA quality pass of a loops run, or whenever measuring changed code for CRAP (complexity x coverage), structural duplication or mutation-test survivors - runs crapper, dryer and mutator (or the pack's native mutation tool), maps each finding to its owning lane, and writes qa/quality.r<N>.md.
---

# Quality pass

The rules and bars are `specs/ENGINEERING.md §12-§14`; the place in the run is `specs/WORKFLOW.md §2.4`. This skill is the how-to. One `qa-verifier` runs it as Agent id `qa-quality`, once per QA round, after every lane is VERIFIED and before the full gate. A single pass sees duplication across lanes, and it keeps parallel runs from overwriting one another's `.metrics/` output.

## Load

- `research/language.json` (the packs and their roots), the plan's waves table (each lane's `Owns` globs), and the reports in `reports/`.
- `loops:lang-<id>` for each pack: its `testing.md` names the native mutation tool, if any.

## Tools

[crapper](https://github.com/unclebob/crapper), [dryer](https://github.com/unclebob/dryer) and [mutator](https://github.com/unclebob/mutator) cover Clojure, Java, Go, TypeScript, Rust and Python.
- **Install.** Each is a checkout whose launcher creates its own `.venv` on first use. Clone them side by side (mutator reads `../crapper`), and put each checkout directory on `PATH`. A symlink to the launcher breaks its `.venv` lookup.
- **Detect.** Run `command -v crapper dryer mutator`. A missing tool is recorded as `not installed: <tool> (install: git clone https://github.com/unclebob/<tool>)` and does not block.
- **Run in the run's environment:** `loops exec --pack <id> -- <tool> …`. The tools run the project's tests and coverage, and `loops exec` gives them the run's caches and tmp dir, never the user's. Name the pack explicitly: these tools belong to no pack, so without `--pack` they would run with the user's own caches. In a repo with several packs, run each tool once per pack, on that pack's changed files.

Other packs: use the project's configured native tool from `skills/lang-<id>/testing.md` (PIT, Stryker, Stryker.NET, cargo-mutants, mutant, Infection, Mull). Never add one to the project here: the `<id>-engineer` owns tool config. With no tool, record `review only` and apply the hollow-test rule (ENGINEERING §12) by reading the changed tests.

## Scope

The run's changed source files: tracked and untracked changes against the run's base. That is the set `git status --porcelain` shows while the run is uncommitted (loops' default), so the tools' `--changed` selects it. Otherwise pass the files explicitly. Test files, generated files and the pack's `coverage.exclude` are out of scope. Only functions the run **added or changed** are blocking. Read their line ranges from `git diff -U0`. Findings in other functions of the same files are notes.

## Steps

1. **CRAP.** Run `loops exec --pack <id> -- crapper --changed`.
   - It regenerates coverage, prints a table (worst first) and writes `.metrics/crap.edn`.
   - A changed function with CRAP ≥ 10 is a finding. Fix direction: split it, test the untested branches, or both.
   - A flat dispatch (`switch`, `case`, `match` over cases) may stay above the bar when the owner states why. The finding then becomes a note.
2. **Duplication.** Run `loops exec --pack <id> -- dryer --changed --threshold 0.8`.
   - It exits 0 after a report, so read the report.
   - A pair with similarity ≥ 0.8 where at least one side is changed code is a finding. Fix direction: one place for the decision (ENGINEERING §1).
   - A pair that crosses lanes goes to the lane whose code is newer. If both are new, it goes to the producer lane (the earlier wave).
3. **Mutation.** Run `loops exec --pack <id> -- mutator --changed --max-workers <max_concurrent_test_runs from research/hardware.json>`.
   - Exit 0: every executed mutant was killed. Exit 3: a mutant survived. Exit 2: the baseline tests fail, which goes to the lead as a gate failure, not a quality finding.
   - Per changed function:
     - **survived:** a finding. Name the mutation, for example `> → >=` at `file:line`, and the missing assertion.
     - **uncovered:** a finding: no test reaches that line.
     - **unreachable:** code nothing calls and no test can reach is dead. Delete it.
   - Later rounds are differential: mutator reruns survivors and changed functions only.
   - Never delete `target/mutator-backup/`. The next run restores from it.
4. **Native tools** (other packs): run the configured goal scoped to the changed classes or modules, and read its report the same way.
5. **Map each finding to its lane** with the plan's `Owns` globs, the same lookup as a gate failure (`skills/work` §4.6).
6. Write `<Run dir>/qa/quality.r<N>.md` from `templates/QUALITY.md`, at most 500 words excluding output excerpts of up to 20 lines. Each finding is `Q<n>` with these fields:
   - `file:line`;
   - the function;
   - the lane;
   - the metric and its value;
   - the fix direction.

   Then list the notes, the tools that did not run and why, and the verdict.

## Verdict

- `VERIFIED` when there are no findings. It is also `VERIFIED` when no tool could run; the file then says so, and the reflection notes it.
- Otherwise `FAILED`. The lead records `loops run verdict --name quality:Q<n> --status FAILED --round N --ref qa/quality.r<N>.md` and sends each lane its findings (`loopback --from qa-quality --to <id>`).
- After the fixes, the same verifier re-runs the pass (round N+1).
- The loop limits of WORKFLOW §4 apply, and so does ENGINEERING §11 when a finding survives two rounds.

## Outputs and cleanup

The tools write `.metrics/` (the crapper and mutator snapshots, which uml-viewer reads) and `target/coverage/`. For the six packs these are pack artifacts, so `loops run end` removes them when the run created them. They are never committed. Do not write anything else in the repo.
