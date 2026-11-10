# Quality pass template

Written by the `qa-verifier` running as `qa-quality`, as `qa/quality.r<N>.md` (`skills/quality`, `specs/ENGINEERING.md §13`). Only functions the run added or changed are findings; the rest are notes. Excerpts are at most 20 lines.

````markdown
# Quality r<N>

Scope: <changed source files, or the `--changed` set> | Base: <ref>

## Tools
- crapper: <ran | not installed | n/a for <pack>> - `<command>` -> exit <n>
- dryer: <ran | not installed | n/a> - `<command>` -> exit <n>
- mutator: <ran | not installed | n/a> - `<command>` -> exit <n>, killed <k> / survived <s> / uncovered <u>
- native (<pack>): <tool and command | none: review only>

## Findings
### Q1: <metric> <value> - <lane id>
- Where: <file:line> in <function>
- Metric: <CRAP 14.2 (CC 6, coverage 40%) | duplicate of <file:line> at 0.91 | survived mutant `<from> -> <to>` | uncovered | dead code>
- Fix direction: <split / test the branch at file:line / merge into <place> / assert <result> / delete>
- Output (<=20 lines, optional):
  ```
  <excerpt>
  ```

## Notes
- <pre-existing debt outside changed functions, accepted flat dispatch with the owner's reason, or "none">

Verdict: VERIFIED|FAILED
````
