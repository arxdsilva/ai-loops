# loops for Amp

Tier C (generic subagents). Installed with `loops install --target amp`.

- Skills land in .agents/skills/; rules come from AGENTS.md.
- Use the work skill as the /work equivalent: ask Amp to "use the work skill: <task>"
  (init, status and report likewise).
- Roles: spawn a generic Task subagent whose prompt starts with the body of
  the role file (`loops path agents` gives the directory). Per-role models and tool
  restrictions are not enforced.
- Loop-backs: Amp can message the same subagent where supported; otherwise spawn a
  new one with its previous report and the QA or review file.
- No hooks in v1: guards, tracing and the stop gate do not run. `loops watch`
  and `loops report` still work from the run-dir files.
