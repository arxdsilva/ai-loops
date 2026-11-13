# Charter template

`charter.md` in the run dir: the run's scope, written by the lead at intake and kept by it alone (WORKFLOW §2.7). Every agent gets its path in the `Charter:` header line and reads it first. Keep it under ~60 lines. Decisions are not copied here: they live in `events.jsonl` (`loops run decide`) and show in `loops run brief`.

```markdown
# Charter: <run title>

Run: <run id> | Ticket: <ref or "text"> | Checkpoints: <loops run ask output, e.g. "none" or "plan,product">

## Goal
<one or two sentences: the outcome the user wants, in their terms>

## Acceptance criteria
- AC-1: <observable outcome, from ticket.md>
- AC-2: <...>
(final at plan approval: plan.v<N>.md; derived ACs are marked `derived`)

## In scope
- <behaviour, modules or interfaces this run may change>

## Out of scope
- <what this run must not change, even if it is tempting; follow-ups go to pr.md Open follow-ups>

## Constraints
- <project rules that bind this run (file:line), compatibility promises, deadlines, --no-pr, the language packs or "no pack detected">

## Assumptions
<!-- when from the clarify step, each cites its clarifications.md item, e.g. "(K3)" -->
- A1: <the reading taken where the ticket is unclear, and why it is the narrowest one that meets the goal> (decision recorded)

## Plan
<empty until approval; then "plan.v<N>.md approved: <waves> waves, <lanes> lanes">
```
