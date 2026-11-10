# loops for OpenCode

Tier B (subagents, no resume). Installed with `loops install --target opencode`;
files are laid out as they land in a project:

- .opencode/agents/loops/<role>.md: subagents (read-only roles deny edit).
- .opencode/commands/loops/<cmd>.md: /loops/work, /loops/init, ... ($ARGUMENTS).
- .opencode/plugins/loops.ts: hook shim (needs `loops` on PATH, or LOOPS_BIN).
- .agents/skills/: the loops skills. OpenCode reads AGENTS.md natively.

Only tool.execute.before/after hooks are shimmed: read-guard and model-guard can
deny, post-edit and trace run after tools. There is no stop gate, session
context or subagent lane tracking.

Roles inherit the session model unless built with `loops port build --models`.
