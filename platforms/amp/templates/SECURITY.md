# Security pass template

Written by the `security-reviewer` as Agent id `qa-security`, as `qa/security.r<N>.md` (`agents/security-reviewer.md`, `specs/WORKFLOW.md §2.4`). Only a new finding (absent from the run base) blocks; the rest are notes. Excerpts are at most 20 lines.

````markdown
# Security r<N>

Scope: <changed packages or files> | Base: <commit> | Base scan: <ran | fallback: flagged line changed in git diff -U0 | not run: <why>> | Counts (rule/file, base vs tree): <match | mismatch: <detail>>

## Tools
- <pack> <tool>: how it ran: <gate | pinned go run <module>@<version> | skipped: missing | skipped: stale | none: <pack's security.none reason>> - `<command>` -> exit <n>

## Findings
### S1: <tool> <rule id>, severity <level> - <lane id> - <blocking | note>
- Where: <file:line> in <function>
- New: <yes, absent from the base scan | no | yes, by fallback: flagged line changed>
- Fix direction: <change to ... | justify on the same line with `// #nosec G<id> -- reason`>
- Output (<=20 lines, optional):
  ```
  <excerpt>
  ```

## Notes
- Pre-existing findings (present in the base scan): <list, or "none">
- Suppressions in changed code: <file:line and reason, or "none">. An unexplained suppression is a blocking finding (`specs/ENGINEERING.md §12`).

Blocking: <n> | Notes: <m>

Verdict: VERIFIED|FAILED
````
