# Reflection template

Written by the `reflector` as `reflection.md` from `loops status --json` and the run files. Facts and decisions only, no reasoning transcripts.

```markdown
# Reflection: <run id>

## Outcome
<done|abandoned|failed; PR link; rounds used>

## Loop-backs and root causes
- <from -> to, round N: root cause>

## Premises attacked
- <round, check: the premise in one sentence, and what replaced it | or: none>

## Quality pass
- <findings fixed by metric (CRAP, duplication, mutation); tools not run and why; pre-existing debt noted>

## Gate failures
- <check: cause, fixed by>

## What the plan missed
- <gap and which plan section should have caught it>

## Gotchas worth remembering
- <durable fact, with path:line or spec ref>

## Overrides
<count of project rules applied over loops rules; details in `overrides.md` (posted as a PR comment), or: none>

## Cleanup
<Cleanup: N files, X MB freed; kept for review: <paths or none>>

## Memory writes
- <path / kind / tier> | or: ai-memory not connected

## Suggested harness changes
- <role, skill, spec or template change>
```
