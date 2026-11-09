# PR body template

Written by `product-reviewer` as `pr.md`: at most about 40 lines before the `<details>` blocks. The footer line stays literal.

```markdown
## Summary
- <what changed and why>
- <...2-4 bullets>

## Acceptance criteria
- [x] AC-1 <criterion> - proof: <one-line result, command>
- [x] AC-2 <criterion> - proof: <...>

## How to verify
<commands a reviewer can run>

## Risk / rollback
<what could go wrong; how to revert; migrations and their direction>

## Open follow-ups
- <item or "none">

<details><summary>Proof AC-1 <criterion></summary>

<trimmed command, output and log excerpt>

</details>

Run report: report.html (attached in run dir)
```
