# loops for Google Antigravity

Tier C (generic subagents; solo fallback). Installed with:

    npx ai-loops --target antigravity
    loops install --target antigravity [--scope project|user]

- Project scope places skills in .agents/skills/, hooks in .agents/hooks.json,
  and shared plugin files (specs, templates, role files, bin/loops shim) in
  .loops/plugin/.
- User scope places skills in ~/.gemini/config/skills/, hooks in
  ~/.gemini/config/hooks.json, and shared plugin files in
  ~/.local/share/loops/plugin/.
- Rules come from AGENTS.md.
- Antigravity app and CLI (`agy`): both load skills from .agents/skills/ and
  ~/.gemini/config/skills/. Run `/skills reload` after installing or updating;
  `agy` also exposes skills as slash commands (/work, /init, /status, /report).
- .agents/hooks.json registers SessionStart, PreToolUse, PostToolUse and Stop
  with `--platform antigravity`. `loops install` rewrites the hook commands to
  the plugin-home shim; using the package without the installer needs `loops`
  on PATH.
- Subagents: spawn generic subagents (`invoke_subagent`) whose prompt starts with
  the role file body (`loops path agents` gives the directory). Per-role models
  and tool restrictions are not enforced, subagent resume is not assumed, and
  `skills/work` falls back to solo mode when subagents are unavailable.
