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

## Tests and coverage
- `<command>` -> <pass/fail>, coverage <n>% for <pkg> (before <m>%)

## Coverage
- New-code coverage: <n>% (min 70%), from `loops gate --only test,coverage --pkgs <owned>`; tests run with `-race`

## Still stubbed
- <what and why, or "nothing">

## Deviations from the plan
- <what and why, or "none">

## Problems seen in others' files
- <path:line - problem, or "none">
```
