---
name: status
description: Show the current loops run status and optionally install the loops status line into the user's Claude Code settings. Use when the user runs /loops:status or asks what the run is doing.
disable-model-invocation: true
---

# /loops:status

1. Run `loops status` and show the output as is (add `--json` only if the user asks for details).
2. If no run is active, say so and stop unless the user wants the status line.
3. **Status line (ask first, AskUserQuestion: Install / Skip).** Offer to add a live status line to `~/.claude/settings.json`:
```json
{"statusLine":{"type":"command","command":"<ROOT>/bin/loops status --statusline","refreshInterval":2}}
```
   with `ROOT=$(loops path root)` expanded to an absolute path.
4. On Install:
   - Read `~/.claude/settings.json` (treat missing as `{}`).
   - If a `statusLine` already exists, save its JSON to `~/.claude/loops-statusline.backup.json` and tell the user; ask again before replacing.
   - Edit only the `statusLine` key; preserve everything else. Write back valid JSON.
5. Tell the user how to revert: restore the backup value into `statusLine` (or delete the key) in `~/.claude/settings.json`.
