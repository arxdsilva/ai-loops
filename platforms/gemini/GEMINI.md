# loops plugin (always-on context)

`loops` runs a ticket through six visible phases (research, plan, implement, QA, product proof, reflect) staffed by role agents, for backends in 15 languages (`loops lang list`; the repo's are `loops lang detect`). If `loops` is not on PATH, use the absolute path given in the session context (`<plugin root>/bin/loops`), or `go install github.com/arxdsilva/ai-loops/cmd/loops@latest`.

- **Entry point:** run the `work` skill with a ticket ref or text. On Claude Code, Copilot and Gemini it is `/loops:work`; on Cursor `/work`; on Codex `$work`; on OpenCode `/loops/work`; on Amp, Windsurf and Cline, the `work` skill. Other commands: `init`, `status`, `report`.
- **Before starting**, the work skill tells you to read `platforms/<your platform>.md` beside it.
- **Run dir:** `.loops/` (config in `.loops/config.json`, runs in `.loops/runs/<id>/`). Everything is recorded in `events.jsonl`.
- **Plugin files:** `loops path specs` (also `templates`, `skills`, `agents`) prints the absolute path. Spec of the workflow: `specs/WORKFLOW.md`; roles: `ROLES.md`.
- **Non-negotiables:** the repo's `AGENTS.md`, `specs/ARCHITECTURE.md` (dependency direction, SOLID) and `specs/TESTING.md` (≥70% of new code covered, the pack's race policy). Check with `loops arch`; gate with `loops gate`, which runs every detected pack's checks. Each language's rules: `specs/languages/<id>/RULES.md` and the `lang-<id>` skill.
- **Toolchain commands** are scoped to the run's caches by the hooks; where a hook denies, use `loops exec -- <cmd>`.
- **Project rules win:** the repo's own rules (AGENTS.md, CLAUDE.md, `.cursor/rules`, CONTRIBUTING, linter configs) take precedence over loops' defaults. When you follow one instead of a loops rule, record it: `loops run override --rule "<loops rule>" --project "<project rule, file:line>" --note "<what you did>"`.
- **Context discipline:** search first, read ranges, never paste whole files (`specs/CONTEXT_DISCIPLINE.md`).
- **Live view:** run `loops watch` in a side terminal; `loops report` writes an HTML report.
- **The lead is the master agent:** it owns every phase, the scope (`charter.md` in the run dir) and every decision, and asks the user only at the checkpoints they opted into (WORKFLOW §2.7). After a compaction or a resumed session, re-anchor with `loops run brief` before acting.
- **Never** commit or push unless you are the product-reviewer and the lead sent you the go.

Run the loops workflow with `/loops:work <task>` or by activating the `work` skill (skills/work/SKILL.md).
