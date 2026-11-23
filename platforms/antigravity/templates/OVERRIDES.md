# Overrides template

Written by the `reflector` as `overrides.md` from `State.Overrides` (`loops status --json`), then posted as a PR/MR comment when there is at least one override, by the reflector or, when reflect ran first, by the product-reviewer (`specs/WORKFLOW.md §2.5`, §2.6). Never put it in the PR description.

```markdown
## Project rules applied over loops defaults

This project's own rules take precedence over loops' defaults; these were applied:

| loops rule | project rule | where | why |
|---|---|---|---|
| <rule> | <rule, file:line> | <agent id / path> | <reason> |

_loops run <run id>_
```
