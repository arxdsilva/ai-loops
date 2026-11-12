# Claims report template

Every implementer finishes with this, saved as `reports/<agent-id>.md`. At most 400 words. Each claim has one line and names evidence a different agent can re-run. Do not paste code or reasoning.

```markdown
# Claims report: <agent-id>

Agent: <agent-id> | Role: <role> | Wave: <n>

## Changed files
- <path> (<added|modified|deleted>)

## Public contracts touched
- <signature or type, or "none">

## Claims
- C1: <statement> - evidence: <test name | command -> result | file:line>
- C2: <statement> - evidence: <...>

## Red first (specs/ENGINEERING.md §7)
- <test name> failed before the change: <the failure, one line> | or: refactor pinned by <tests> (no new behaviour)

## Tests and coverage
- `<command>` -> <pass/fail>, coverage <n>% for <pkg> (before <m>%)

## Coverage
- New-code coverage: <n>% (min 70%), from `loops gate --only <test checks>,coverage --pkgs <owned>`; tests run under the pack's race policy (<flag/build/substitute>)

## Premise (only when the same claim or finding failed twice, specs/ENGINEERING.md §11)
- <the premise both fixes shared, one sentence>; held by <actors>; removed by <change>

## Still stubbed
- <what and why, or "nothing">

## Deviations from the plan
- <what and why, or "none">

## Problems seen in others' files
- <path:line - problem, or "none">
```
