# PR body template

Written by `product-reviewer` as `pr.md`: at most about 40 lines before the `<details>` blocks. The footer line stays literal.

```markdown
## Summary
- <what changed and why>
- <...2-4 bullets>

## Acceptance criteria
- [x] AC-1 <criteria> - proof: <one-line result, command>
- [x] AC-2 <criteria> - proof: <...>

## How to verify
<commands a reviewer can run>

New-code coverage: <X>% (min 70%), tests run under each pack's race policy

## Risk / rollback
<what could go wrong; how to revert; migrations and their direction>

## Decisions and assumptions
- <each `loops run decide` the lead or user took, and each charter assumption, one line: what and why; or "none">

## Open follow-ups
- <item or "none"; an unmet AC of a draft PR goes here with its proof>

<details><summary>Proof AC-1 <criteria></summary>

<trimmed command, output and log excerpt>

</details>

Run report: report.html (attached in run dir)
```
