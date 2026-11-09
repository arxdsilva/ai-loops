---
name: init
description: Set up loops in the current Go repository - create .loops/config.json, gitignore run data, install AGENTS.md and CLAUDE.md pointers, and check required tools. Use when the user runs /loops:init or asks to set up loops.
disable-model-invocation: true
---

# /loops:init

Idempotent setup for the current repo. **Never overwrite an existing file without asking**; show a diff or the proposed text and use AskUserQuestion.

1. **Preconditions.** `git rev-parse --show-toplevel` (cd there) and `test -f go.mod`. If not a Go repo, say so and stop.
2. **`.loops/config.json`.** If absent, create it with these defaults (JSON has no comments; explain each key to the user in your reply):
```json
{
  "layers": {},
  "approval": "never",
  "auto_pr": false,
  "gate": { "skip": [], "vuln": false },
  "memory": { "workspace": "", "project": "" },
  "parallel": { "max_agents": 100, "agent_ram_gib": 1, "cpu_slots_per_core": 4, "max_test_runs": 0 }
}
```
   - `layers`: object mapping layer keys (domain, application, ports, adapters, inbound, outbound, platform, compose, cmd, pkg) to lists of module-relative dir prefixes; omitted keys keep defaults. Example: `{"domain":["internal/core"]}`. Leave `{}` unless the repo uses non-standard dirs.
   - `approval`: `"never"` or `"always"` (`always` makes the lead ask before implementing; `--approve` does the same per run).
   - `auto_pr`: `true` skips the confirmation before push/PR. Keep `false` unless the user wants it.
   - `gate.skip`: checks to skip (`build`, `gofmt`, `vet`, `staticcheck`, `arch`, `test`, `govulncheck`); `gate.vuln`: run govulncheck.
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
6. **Tool check.** Run `command -v` for each and print a table of present/missing with the effect of missing:
   - `go` (required), `gh` (GitHub PRs), `glab` (GitLab MRs), `staticcheck` (skipped by the gate if missing; `go install honnef.co/go/tools/cmd/staticcheck@latest`).
   - ai-memory: report whether MCP tools named `mcp__plugin_ai-memory_ai-memory__*` or `mcp__ai-memory__*` are available. Optional; without it memory steps are skipped.
   - `loops version` to confirm the binary.
7. **Validate**: `jq . .loops/config.json` must succeed (`loops` warns on stderr when the file fails to parse).
8. **Report** in at most 10 lines: files created, files left alone, tools missing, and next step `/loops:work <ticket>`.
