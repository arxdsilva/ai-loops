---
name: clarify
description: Use at intake before the charter, or when a developer asks to pin down a ticket's details - scans the ticket against ten categories, infers every open point with a recommended answer and records each inference (asks only when the user opted in or a hard-to-undo point needs a human).
---

# Clarify

Adapted from GitHub Spec Kit's `/speckit.clarify` (MIT, https://github.com/github/spec-kit, templates/commands/clarify.md); play-back idea from obra/superpowers `brainstorming` (MIT). Original wording; no Spec Kit prose is copied.

The goal: a developer should be able to pin down every detail of a ticket before code is written. The skill finds the gaps, closes them with the recommended answer, and writes each one down so it can be reviewed or reversed later. Nobody is interrupted unless the user opted in or undoing an inference later would be hard.

## When

- At intake, after the ticket is fetched and before the charter is written (WORKFLOW §2.0). The lead runs it; role agents do not.
- Outside a run, when a developer asks to clarify a ticket or text. Then the output is the list only (step 7).
- Standalone: `/loops:clarify <ticket ref or text>` with no run infers and prints the list. Add `--ask` to be asked the open points instead (step 4). Inside a run, the mode follows the intake checkpoint.

## Inputs

1. `<run dir>/ticket.md` (the normalised ticket). Get the path with `loops run dir`.
2. `<run dir>/research/memory.md`, when it exists (ai-memory briefing, `loops:memory`).
3. Read-only repo facts: grep and ranged reads of the code the ticket names. Cite `path:line`.

Do not read whole files. Do not start research here; `research/code.md` and `research/practices.md` are other phases' work.

## Procedure

1. **Scan ten categories.** For each, mark the ticket `Clear`, `Partial` or `Missing`:
   1. scope: what is in and out;
   2. data model: entities, fields, identifiers, persisted shape;
   3. user or CLI flow: who triggers what, and what they see back;
   4. non-functional: performance, security, observability, limits;
   5. integrations: external APIs, brokers, files, other packages;
   6. edge and failure cases: empty, duplicate, concurrent, partial failure;
   7. constraints and trade-offs: compatibility, deadlines, rules the repo already sets;
   8. terminology: names the ticket uses that the code or docs define differently;
   9. completion signals: how the user will know it is done;
   10. placeholders: TBD, "etc.", unnamed options.

2. **Keep only points that matter.** A `Partial` or `Missing` point is recorded only if the answer would change an acceptance criterion, the plan or a test. Drop the rest without writing them down.

3. **Build each item.** For each kept point, write:
   - the question, and the recommended answer (the narrowest reading that meets the stated goal). Give 2-4 alternatives when the choice is real;
   - its source: a ticket line, a `path:line`, or a named project convention;
   - its reversibility: `easy` or `hard` (the hard list is in step 5).

4. **Choose the mode.**
   - **Ask** when `loops run checkpoint intake` prints `ask` (the user put `intake` in their checkpoints), or when the user asked in this request ("ask me", `--ask`). Ask at most 5 questions, one at a time, each with its recommended answer first. "yes" accepts the recommendation.
   - **Infer** otherwise. Inferring means accepting the recommended answer and recording it as `inferred`. This is the default: no question is asked. Open points beyond the 5-question cap are inferred and recorded like the rest, so no detail is left unrecorded.

5. **Hard-to-undo points.** A point is `hard` when the answer is one of:
   - a public API, a CLI name or flag;
   - a persisted schema, an event field, a migration;
   - deleting data or files;
   - an external side effect (publish, push, send, pay);
   - security, auth or permission posture;
   - a licence or a new dependency;
   - anything that moves the scope's edges.

   In **Ask** mode, a hard item is asked like any other. In **Infer** mode, a hard item is still asked when the lead is talking to a user who can answer now, because undoing the inference later would be costly. When no user can answer (a headless run), take the most reversible option and record it `inferred · hard`. Decisions show in the report and the PR (WORKFLOW §2.7).

6. **Record.** Write `<run dir>/clarifications.md`. Start each session with `## Session <YYYY-MM-DD>` (today's date, from the run). One line per item:

   `K<n> [category] Q: <question> → A: <answer> (inferred|asked · easy|hard · <source>)`

   - Numbering `K1, K2, ...` is per run and never reused.
   - Each **inferred** answer becomes a charter Assumption `A<n>` that cites its item (`A1: … (K3)`), and is logged with `loops run decide --topic assumption --note "K<n>: <answer>" --ref clarifications.md`.
   - Each **asked** answer is logged with `--by user` as well: `loops run decide --by user --topic assumption --note "K<n>: <answer>" --ref clarifications.md`.

7. **Outside a run.** Print the list in the reply, in the same line format. Write a file only when the user names one, and then add a `## Clarifications` section to that file rather than a new one.

8. **Play back.** Before the charter is written, give the user one short paragraph: the reading the run will follow and the `inferred` items they should know about. This is a summary, not a question, and it asks nothing.

## Stop rules

- Never ask a question that would not change an acceptance criterion, the plan or a test.
- Never ask more than 5 questions in one session. Inferred items are not capped, but each must still change an acceptance criterion, the plan or a test.
- Stop when every category is `Clear`, or its open points are recorded.
- Do not edit `ticket.md`. The ticket is the user's text; the clarifications are the record of what was inferred from it.

## What this skill does not do

- It does not write code or tests, and it does not change the plan. The plan and the charter take its answers as input.
- It does not decide hard-to-undo points silently in a session where a user can answer. Those are asked.
- It does not commit anything.
