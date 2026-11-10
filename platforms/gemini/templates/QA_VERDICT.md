# QA verdict template

Written by `qa-verifier` as `qa/<agent-id>.r<N>.md`. Verify every claim independently: re-run the cited tests, read the code, try counterexamples (edge cases, error paths, concurrency under the pack's race policy). Excerpts are at most 20 lines.

````markdown
# QA <agent-id> r<N>

## Claims
### C1: <statement> - VERIFIED|FAILED
- Checked: <what you ran or read>
- (FAILED only)
  - Repro: `<command>`
  - Output (<=20 lines):
    ```
    <excerpt>
    ```
  - Location: <file:line>
  - Expected vs actual: <expected> / <actual>
  - Suggested fix area: <module/function>

## Unclaimed problems
- <file:line - problem, or "none">

## Gate results
- `loops gate --pkgs <owned> --only <the pack's build, lint and test checks>` (Go: `build,vet,test`): <pass/fail>
- `loops gate --only <test checks>,coverage --pkgs <owned>`: new-code coverage <n>% (min 70%), race policy <pass/fail>; uncovered `file:line` (FAILED if below 70% or new code lacks tests)
- `loops arch`: <pass/fail, warnings>

Verdict: VERIFIED|FAILED
````
