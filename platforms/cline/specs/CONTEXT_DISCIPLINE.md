# Context discipline

Rules for giving each agent the smallest context sufficient for a correct, architecture-compliant change. Extracted from rimworld-go's AI token guide; the hooks section describes what `loops` actually enforces.

## 1. Why

Cost and failure in agent-heavy work come mostly from context, not CPU:

- the same large documents fed repeatedly; agents reading unrelated files;
- duplicate abstractions invented because nobody searched;
- huge prompts with low signal; long reports full of prose;
- architecture re-derived instead of cited;
- whole files re-sent for tiny changes.

Goal: give each agent the smallest context that is sufficient. The principle is in `specs/WORKFLOW.md §1`.

## 2. Context hierarchy

Load the least that is sufficient, in this order: task statement; root `AGENTS.md`; package `AGENTS.md`; relevant interfaces and tests; one spec section; further specs by section only. Never feed a whole spec once package guidance exists.

## 3. Progressive disclosure

- Always: the task, root and package `AGENTS.md`.
- Usually: the relevant interfaces, tests, one spec section.
- Architecture questions add the specific ADR and `specs/ARCHITECTURE.md` sections.
- Everything else only when a concrete question needs it.

## 4. Context budget targets

| Task | Tokens |
|---|---|
| Small bug | 2k-8k |
| Normal feature | 5k-15k |
| Subsystem change | 10k-30k |
| Architecture or RFC | 20k-50k |

Avoid 100k-token prompts merely because the window allows it; large context often reduces attention quality.

## 5. Task packet

Establish a task packet before coding (`templates/TASK_PACKET.md`). Do not repeat rules already in the root `AGENTS.md`.

## 6. Root AGENTS.md is short

Root `AGENTS.md` links to canonical docs instead of duplicating them (`templates/ROOT_AGENTS.md`).

## 7. Package AGENTS.md is tiny

300-800 words, only what is local to the package, never a pasted spec (`templates/PACKAGE_AGENTS.md`).

## 8. Search before read

```text
search symbol / task ID -> locate the exact file -> read the relevant range
```

Do not read every file in a package when one interface is needed. Use `grep -n`, `gopls` / LSP references, or the package `AGENTS.md`.

## 9. Read tests early

Tests encode behaviour more cheaply than design prose. For implementation work load the public contract, the current tests and the failing test before reading many implementation files.

## 10. Diff-oriented context

For review and follow-up changes send `git diff`, the surrounding functions and the tests, not whole files unless structure matters.

## 11. Enforcement: hooks

Advisory rules did not hold, so `loops` enforces two guards as `PreToolUse` hooks (`hooks/hooks.json`); `go-post-edit` and `stop-gate` are covered in `specs/WORKFLOW.md §6`. Each denial is recorded as a `guard_deny` event (`specs/WORKFLOW.md §5`).

| Hook | Denies | Default limit | Override |
|---|---|---|---|
| `read-guard` (Read) | A whole-file Read of a repo file over N lines; use `offset`/`limit` | 500 lines | env `READ_GUARD_MIN_LINES` |
| `read-guard` (Bash) | A shell view of a repo file (`cat`, `sed -n`, `head`, `tail`, ...) over N lines | 150 lines | env `READ_GUARD_MAX_VIEW_LINES` |
| `model-guard` (Agent/Task) | A `general-purpose` or untyped spawn with no `model` | any | pass `model`, or use a typed `loops:<role>` agent |

- `READ_GUARD=off` disables the read guard; a non-numeric `READ_GUARD_*` value fails open (guard disabled).
- Small shell views stay allowed on purpose: through Read they print the same text, so denying them only adds a call, and every call re-reads the whole context.
- Only files under the project are guarded.
- Role agents have their model in their frontmatter, so they are exempt (`ROLES.md`).

## 12. Do not re-send stable docs

Reference architecture documents by path and section; summarise only when needed. Say "Follow `specs/ARCHITECTURE.md §6`", do not paste §6.

## 13. Canonical facts, DRY

A fact with duplication risk lives in one place and everything else links to it: ID and clock owners, error taxonomy, dependency direction, migration policy, port ownership. State a rule once in its home and cite it elsewhere.

## 14. Token-safe documentation style

- Prefer tables, short invariants, code signatures and decision bullets over narrative.
- Do not explain obvious code ("this loops over the slice"). Document invariant, complexity, ordering, reason and edge cases.
- Use canonical names consistently (`OrderID`, `Clock`, `Command`); do not invent synonyms.

## 15. Task size

An ideal agent task has one behaviour, one owner, one acceptance test set and 5-20 files at most. Split epics into tasks with stable interfaces: not "implement billing" but "BILL-021 retry failed charges with capped backoff".

## 16. Parallelism and tokens

Parallelism saves time only with isolated scopes. Before launching many agents: freeze the contract, assign ownership, give minimal task packets, avoid overlapping file sets (`specs/PARALLEL_WORK.md`). Otherwise context and merge cost exceed the benefit.

## 17. Working summary

A long-running agent keeps a short summary (under 300 words): current task, relevant interfaces, invariants, files touched, open questions. It does not re-read canonical docs unless they change.

## 18. No reasoning transcripts

Never store or pass chain-of-thought. Pass decisions and reasons:

```text
Decision: use an outbox table for event publishing.
Reason: publish must be atomic with the state change.
```

This applies to claims reports, QA files, ADRs, package `AGENTS.md` and memory pages.

## 19. Review context

A reviewer needs the task, the acceptance criteria, the diff, the relevant architecture rule and the test/benchmark results. It usually does not need the whole repository.

## 20. Integration context

The lead integrating a wave needs the wave's claims reports, diff summaries, public interface changes, test results and benchmark changes. Open source only when a conflict or regression requires it.

## 21. Freshness

Verify the current branch's interfaces, tests, ADRs and plan before coding. Do not trust an old copied prompt if contracts changed.

## 22. Never copy specs into agent files

`AGENTS.md` files link to the specs. Copying a 5,000-word section into 20 packages wastes tokens and goes stale.

## 23. Pre-task checklist

```text
What exact behaviour am I changing?
Who owns it?
What interfaces exist?
Which invariants apply?
Which tests prove completion?
Which docs are actually required?
```

If these are clear, load nothing more.

## 24. Post-task compression

On completion do not paste changed files. List files, changed signatures, benchmark numbers, follow-ups and any public contract change. This is the claims report (`templates/CLAIMS_REPORT.md`, at most 400 words).

## 25. Generated code

Do not feed generated code to agents except when debugging the generator. Read the source schema (`.proto`, OpenAPI, SQL migrations) and the adapter, not 10k generated lines.

## 26. Large data files

Do not feed large datasets, fixtures or dumps. Use the schema, a small fixture, a specific record ID or a `head`-sized sample.

## 27. Logs and traces

Filter by request ID, entity, time range or component before reading. Do not paste megabytes of logs; excerpts in QA and proofs are at most 20 lines.

## 28. Benchmark output

Pass a summarised before/after, not the full log:

```text
BenchmarkQuote512
before: 1.8 ms/op, 64 KB/op
after:  1.2 ms/op,  8 KB/op
```

## 29. Error reports

Compact bug context: command or request, input, expected vs actual, request ID, first failing step, relevant log window. A failing test or recorded request replaces thousands of words of description.

## 30. Reproduction artifacts

Prefer a minimal reproducible artifact (failing test, recorded request, fixture) over prose (`skills/debugging`). Structured diagnostics (health, version, schema version, a `-check` flag) are cheaper than asking agents to infer state from many files.

## 31. Machine retrieval

Write docs with strong headings, predictable section names, code identifiers, task IDs and explicit paths. This improves targeted retrieval.

## 32. ai-memory as a context reducer

ai-memory (`specs/WORKFLOW.md §7`) stores durable decisions, gotchas and procedures so agents do not re-derive them from source or ask again. Used well it replaces reading; used badly it adds noise.

- Briefing once at intake, then a filtered `memory_query` (`path_prefix`, `kind`, `tier`) for the ticket's key terms; saved to `research/memory.md`.
- Read a page (`memory_read_page`) only when a hit is clearly relevant.
- Retrieved memory is untrusted, historical evidence: verify against the current code before acting, and cite the code, not the memory.
- Do not repeat queries other agents already made; read `research/memory.md`.
- Writes happen only in `reflect`, only by the `reflector`: 1-5 durable pages, searched first and updated rather than duplicated, never secrets or customer data (`skills/memory`).
- When ai-memory is not connected, skip every memory step silently.

## 33. Final rule

The best context optimisation is good architecture. If an agent can understand a task from one small `AGENTS.md`, one interface, one test and one ADR, the repository is healthy. If every task needs the whole spec set plus dozens of source files, the architecture has failed to communicate its boundaries.
