---
name: init
description: Set up loops in the current repository (any language pack) - detect the languages, run loops init (the committed .loops/config.json), install AGENTS.md and CLAUDE.md pointers, and check the tools the detected packs need. Use when the user runs /loops:init or asks to set up loops.
disable-model-invocation: true
---

# /loops:init

Idempotent setup for the current repo. **Never overwrite an existing file without asking**; show a diff or the proposed text and use AskUserQuestion.

1. **Preconditions.** `git rev-parse --show-toplevel` (cd there).
1a. **Languages.** Run `loops lang detect` and show each detected pack with its root and variant (`loops lang list` shows every available pack).
   - Nothing detected, or a root is wrong (an unusual layout, a monorepo, a manifest deeper than detection looks): explain that `languages` in `.loops/config.json` pins packs and replaces detection, e.g. `"languages": [{"id": "python", "root": "services/api", "variant": "uv"}, {"id": "go", "root": "."}]` (`root` defaults to `.`; `variant` picks a build tool, `loops lang show <id> --variant v`). Offer to add it, then re-run `loops lang detect`.
   - A language with no pack: say that loops still runs the workflow there, but the gate checks nothing for it. Continue setup.
2. **`.loops/config.json`.** Run `loops init` (WORKFLOW §5). It writes the minimal file below when absent and keeps an existing one; `loops run start` refuses a project without it. Every value is the default, written out so the team sees and reviews its choices:
```json
{
  "version": 1,
  "languages": [],
  "ask": [],
  "triage": "auto",
  "gate": { "coverage_min": 70 }
}
```
   Explain each key in your reply:
   - `languages`: pinned packs (step 1a); `[]` means detect.
   - `ask`: the phases where the lead stops for the user (WORKFLOW §2.7): `[]` means it decides every phase and opens the PR itself; e.g. `["plan", "pr"]`, or `"all"`. `--ask` on `/loops:work`, or the user's own words, override it per run.
   - `triage`: `"auto"` sizes each run to the ticket (WORKFLOW §2.8); `"off"` makes every run full.
   - `gate.coverage_min`: the new-code coverage floor; it can only be raised above 70.
   Other keys are optional; add one only when the user wants it: `layers` (arch layer dir prefixes, e.g. `{"domain":["internal/core"]}`), `gate.skip` (check names from `loops lang show <id>`; `coverage` and the race checks cannot be skipped), `gate.vuln`, `gate.base`, `memory` (`workspace`, `project`, `store`: `"files"` picks `.loops/memory/`, WORKFLOW §7), `parallel` (`max_agents`, `agent_ram_gib`, `cpu_slots_per_core`, `max_test_runs`), `limits` (opt-in caps: `{"max_agents": 12, "budget": {"implement": "2M"}}`), `update_check`. `parallel` and `limits` describe a machine more than a project: suggest them only when the whole team shares the setting.
   If the file existed, `loops init` left it alone: show it and ask before changing anything.
3. **Git.** `.loops/.gitignore` keeps run data and caches out of git and re-includes `config.json`: tell the user to commit `.loops/config.json` (the first `/loops:work` PR commits it otherwise). If other `.loops/` files are tracked from an older setup, suggest `git rm -r --cached .loops` and re-adding `config.json` (the user decides; never run it).
   - Optional file store (WORKFLOW §7): `loops memory init` creates `.loops/memory/` and re-includes it in git. Mention it once in your report; it is not a question.
4. **AGENTS.md.** If absent, copy `$(loops path templates)/ROOT_AGENTS.md` to `./AGENTS.md`. If present, offer to append a "loops" section (the relevant headings from the template); do not replace.
5. **CLAUDE.md.** If absent, create it with exactly:
```
# Claude

@AGENTS.md
```
   If present without `@AGENTS.md`, offer to append that line.
6. **Tool check.** For each detected pack, `loops lang show <id>` lists its `checks` with `tool`, `required`, `default` and `install`. Run `command -v` for each distinct tool, plus `gh` (GitHub PRs) and `glab` (GitLab MRs), and print a table: tool, pack, present/missing, effect of missing:
   - a required check's tool: the gate fails (or uses the check's logged fallback);
   - an optional check's tool: the gate skips it and prints its `install` hint;
   - a `default: off` check (vulnerability scans): only matters when enabled.
   Also print each pack's race policy (`race.mode` and `race.reason`), since a policy the host cannot run fails the gate (e.g. Go's race detector needs cgo and a C compiler).
   - ai-memory: report whether MCP tools named `mcp__plugin_ai-memory_ai-memory__*` or `mcp__ai-memory__*` are available. Optional; without it memory steps are skipped.
   - `loops version` to confirm the binary.
7. **Validate**: `jq . .loops/config.json` must succeed (`loops` warns on stderr when the file fails to parse).
8. **Report** in at most 10 lines: languages and roots, files created, files left alone, tools missing, and next step `/loops:work <ticket>`.
