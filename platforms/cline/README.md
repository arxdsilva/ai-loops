# loops for Cline

Tier D (no subagents): solo mode. Installed with `loops install --target cline`.

- Skills land in .agents/skills/ and .cline/skills/; rules come from AGENTS.md.
- Use the work skill as the /work equivalent: ask Cline to "use the work skill: <task>"
  (init, status and report likewise).
- Solo mode: the lead plays each role in turn in phase order, writes the same run-dir
  files and records events with `loops run label --agent solo-<n>` and `loops run event`.
  Before each QA or review pass it re-reads only that role's files.
- No hooks in v1. `loops watch` and `loops report` work from the run-dir files.
