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
- **Best-effort.** They are young, single-maintainer projects. A tool that fails to run is recorded as such and never patched in place (Go: `specs/languages/go/RULES.md` §57).
- **Install.** Each is a checkout whose launcher creates its own `.venv` on first use. Clone them side by side (mutator reads `../crapper`), and put each checkout directory on `PATH`. A symlink to the launcher breaks its `.venv` lookup.
- **Detect.** Run `command -v crapper dryer mutator`. A missing tool is recorded as `not installed: <tool> (install: git clone https://github.com/unclebob/<tool>)` and does not block.
- **Run in the run's environment:** `loops exec --pack <id> -- <tool> …`. The tools run the project's tests and coverage, and `loops exec` gives them the run's caches and tmp dir, never the user's. Name the pack explicitly: these tools belong to no pack, so without `--pack` they would run with the user's own caches. In a repo with several packs, run each tool once per pack, on that pack's changed files.

Other packs: use the project's configured native tool from `skills/lang-<id>/testing.md` (PIT, Stryker, Stryker.NET, cargo-mutants, mutant, Infection, Mull). Never add one to the project here: the `<id>-engineer` owns tool config. With no tool, record `review only` and apply the hollow-test rule (ENGINEERING §12) by reading the changed tests.

**Lint and security are not here.** Lint runs in per-lane QA (the pack's lint checks; Go: `vet`, `staticcheck`). Security tools run in the security pass (`agents/security-reviewer.md`).

## Scope

The run's changed source files: tracked and untracked changes against the run's base. That is the set `git status --porcelain` shows while the run is uncommitted (loops' default), so the tools' `--changed` selects it. Otherwise pass the files explicitly. Test files, generated files and the pack's `coverage.exclude` are out of scope. Only functions the run **added or changed** are blocking. Read their line ranges from `git diff -U0`. Findings in other functions of the same files are notes.

## Isolation

The pass runs beside the full gate, the security pass and product proofs (WORKFLOW §2.4), and mutation rewrites source files while it runs. So run every tool in a private copy of the tree, never in the project itself:
```
T="$(cat <Run dir>/scratch 2>/dev/null || echo <Run dir>)/quality-tree"
git worktree add --detach "$T" HEAD
git diff HEAD --binary | git -C "$T" apply
git ls-files --others --exclude-standard -z | tar --null -T - -cf - | tar -xf - -C "$T"
```
Run each tool from the project root through `loops exec`, so it keeps the run's environment, and change into the copy inside it: `loops exec --pack <id> -- sh -c 'cd "<T>" && <tool> …'`. Paths in the tools' output are relative to the copy's root, which is the project's. Keep the copy across rounds, so later rounds stay differential: at round N>1, refresh it with `git -C "$T" reset -q --hard HEAD && git -C "$T" clean -fdq -e .metrics/ -e target/`, then the last two lines above again. When your verdict is VERIFIED, remove it: `git worktree remove --force "$T"`.

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
5. **Map each finding to its lane** with the plan's `Owns` globs, the same lookup as a gate failure (`skills/work` §4, step 7).
6. Write `<Run dir>/qa/quality.r<N>.md` from `templates/QUALITY.md`, at most 500 words excluding output excerpts of up to 20 lines. Each finding is `Q<n>` with these fields:
   - `file:line`;
   - the function;
   - the lane;
   - the metric and its value;
   - the fix direction.

   Then list the notes, the tools that did not run and why, and the verdict.

## Verdict

- `VERIFIED` when there are no findings. It is also `VERIFIED` when no tool could run; the file then says so, and the reflection notes it.
- Otherwise `FAILED`. Record it yourself: `loops run ingest <Run dir>/qa/quality.r<N>.md` records each `quality:Q<n>` verdict and one `loopback --from qa-quality --to <id>` per lane (or the `quality` VERIFIED verdict); the lead sends each lane its findings.
- After the fixes, the same verifier re-runs the pass (round N+1).
- The loop limits of WORKFLOW §4 apply, and so does ENGINEERING §11 when a finding survives two rounds.

## Outputs and cleanup

The tools write `.metrics/` (the crapper and mutator snapshots, which uml-viewer reads) and `target/coverage/` inside the copy, never in the project, and the copy goes with the run's scratch dir. Do not write anything in the project.

## Common rationalizations

The rows are binding (`ROLES.md` "Common rationalizations").

| Excuse | Why it's wrong | What to do instead |
|---|---|---|
| "The tool isn't installed, so there is nothing to do." | A missing tool is recorded; the hollow-test rule still applies (ENGINEERING §12). | Record `not installed: <tool>`; with no mutation tool at all, read the changed tests. |
| "That surviving mutant doesn't matter." | Every covered mutant is killed (ENGINEERING §13); a survivor is a missing assertion. | Name the mutation at `file:line` and the assertion the lane must add. |
| "CRAP is high, but the code works." | The bar is < 10 for changed functions (ENGINEERING §13). | Report the finding; only a flat dispatch with a stated reason becomes a note. |
| "The duplicate crosses lanes, so it's nobody's." | Seeing cross-lane copies is why one pass runs (Steps, 2). | Send it to the newer lane, or to the producer lane when both are new. |
| "It's one line; I'll fix it myself." | QA never edits code; the owner fixes (`agents/qa-verifier.md`). | Map it to its lane and write the fix direction. |
| "Mutation is slow; skip it this round." | Later rounds are already differential (Steps, 3). | Run it with `--max-workers` from `research/hardware.json`. |
| "The lane fixed it twice; pass it to unblock the run." | Two failed rounds on one finding trigger ENGINEERING §11, not a waiver. | Keep it `FAILED` and send the lane to §11. |
