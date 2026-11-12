---
name: init
description: Set up loops in the current repository (any language pack) - detect the languages, create .loops/config.json, gitignore run data, install AGENTS.md and CLAUDE.md pointers, and check the tools the detected packs need. Use when the user runs /loops:init or asks to set up loops.
disable-model-invocation: true
---

# /loops:init

Idempotent setup for the current repo. **Never overwrite an existing file without asking**; show a diff or the proposed text and use AskUserQuestion.

1. **Preconditions.** `git rev-parse --show-toplevel` (cd there).
1a. **Languages.** Run `loops lang detect` and show each detected pack with its root and variant (`loops lang list` shows every available pack).
   - Nothing detected, or a root is wrong (an unusual layout, a monorepo, a manifest deeper than detection looks): explain that `languages` in `.loops/config.json` pins packs and replaces detection, e.g. `"languages": [{"id": "python", "root": "services/api", "variant": "uv"}, {"id": "go", "root": "."}]` (`root` defaults to `.`; `variant` picks a build tool, `loops lang show <id> --variant v`). Offer to add it, then re-run `loops lang detect`.
   - A language with no pack: say that loops still runs the workflow there, but the gate checks nothing for it. Continue setup.
2. **`.loops/config.json`.** If absent, create it with these defaults (JSON has no comments; explain each key to the user in your reply):
```json
{
  "layers": {},
  "languages": [],
  "ask": [],
  "gate": { "skip": [], "vuln": false },
  "memory": { "workspace": "", "project": "" },
  "parallel": { "max_agents": 100, "agent_ram_gib": 1, "cpu_slots_per_core": 4, "max_test_runs": 0 }
}
```
   - `languages`: pinned packs (step 1a); `[]` means detect.
   - `layers`: object mapping layer keys (domain, application, ports, adapters, inbound, outbound, platform, compose, cmd, pkg) to lists of module-relative dir prefixes; omitted keys keep defaults. Example: `{"domain":["internal/core"]}`. Leave `{}` unless the repo uses non-standard dirs.
   - `ask`: the phases where the lead stops for the user (WORKFLOW §2.7): `[]` (the default) means it decides every phase and opens the PR itself; e.g. `["plan", "pr"]` to approve the plan and the push, or `"all"`. `--ask` on `/loops:work`, or the user's own words, override it per run. Older configs: `"approval": "always"` still adds `plan`; `auto_pr` is ignored (the PR opens unless `pr` is a checkpoint).
   - `gate.skip`: check names to skip, taken from the detected packs (`loops lang show <id>`; Go: `build`, `gofmt`, `vet`, `staticcheck`, `arch`, `test`, `govulncheck`). `coverage` and the race checks cannot be skipped. `gate.vuln`: include the packs' vulnerability checks (off by default).
   - `memory`: ai-memory scope; leave empty to derive from `.ai-memory.toml` or the git remote `owner/repo`.
   - `parallel`: concurrency. `max_agents` is the configured agent cap (default 100; the effective cap is also bounded by `loops hw`); `agent_ram_gib` and `cpu_slots_per_core` feed the hardware estimate; `max_test_runs` 0 means derive from the CPU count.
   - `limits` (opt-in, omitted by default; add only when the user wants caps): `"limits": {"max_agents": 12, "budget": {"research": "200k", "implement": "2M"}, "agents": {"implement": 12}, "roles": {"implement": ["domain-engineer", "api-engineer"]}}`. Nothing is limited and nothing asks without it.
   If it exists, show it and ask before changing anything.
3. **.gitignore.** Add the line `/.loops/` if no line matches it (create the file if needed). Run data is local.
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
