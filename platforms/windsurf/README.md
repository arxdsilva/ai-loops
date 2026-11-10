# loops for Windsurf (Devin Local)

Tier D (no subagents): solo mode. Installed with `loops install --target windsurf`.

- Skills land in .agents/skills/; rules come from AGENTS.md.
- .devin/hooks.v1.json registers SessionStart, PreToolUse (read-guard, model-guard),
  PostToolUse (post-edit, trace) and Stop (stop-gate) with `--platform windsurf`.
  It runs `loops` from PATH. Run /hooks in Devin to confirm they loaded.
- Use the work skill as the /work equivalent: ask the agent to "use the work skill: <task>".
- Solo mode: the lead plays each role in turn, writing the same run-dir files and
  events (`loops run label --agent solo-<n>`). Subagent lanes are not tracked.
