# Ticket template

`ticket.md` in the run dir, written by `loops ticket fetch` or by the lead for free text. Keep this shape: the planner and reviewers read the headings. Acceptance criteria are copied from the ticket; if it has none, the planner derives them in the plan and marks them `derived`.

```markdown
# <Title>

Source: <github|gitlab|jira|linear|text> | Ref: <#123 | ABC-123 | none> | URL: <url or none>
Labels: <comma-separated or none>

## Acceptance criteria
- AC-1: <observable outcome>
- AC-2: <observable outcome>

## Body
<ticket text, verbatim>
```
