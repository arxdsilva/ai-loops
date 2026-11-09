# ai-loops: agent instructions

This repo is the **`loops`** Claude Code plugin: skills, role agents, specs and templates, plus the `loops` Go binary they drive. Read only what your task needs.

## Where things are
- The workflow spec, which everything implements: `specs/WORKFLOW.md`. Roles: `ROLES.md` (matrix) and `agents/*.md` (definitions).
- Rules for Go code, here and in target repos: `specs/GO_BEST_PRACTICES.md`, `specs/HEXAGONAL_ARCHITECTURE.md`, `specs/SOLID_GO.md`. Agent rules: `specs/PARALLEL_WORK.md`, `specs/CONTEXT_DISCIPLINE.md`.
- Skills: `skills/<name>/SKILL.md`. The `work` skill is the orchestrator; the rest are reference skills that roles load.
- Templates the agents fill in: `templates/`.
- Plugin wiring: `.claude-plugin/`, `hooks/hooks.json`, `bin/loops` (a shim that builds `cmd/loops` on first use), `mod/` (the live pane), `settings.json`.
- Go tooling, laid out hexagonally:
  - `internal/domain/{run,archrule}`: pure code, no I/O, no wall clock;
  - `internal/application/{tracker,guard,gate,archcheck,ticket}`: use cases with consumer-owned ports;
  - `internal/adapters/{inbound/{cli,hook},outbound/{runstore,goparse,gotool,vcs,fsys,htmlreport}}`;
  - `internal/compose/loops` and `cmd/loops` (wiring only).

## Non-negotiables
- **Stdlib only.** `go.mod` has no requirements. Adding one needs a reason recorded in the README.
- **Dependencies point inward.** Adapters depend on application, application on domain. Application packages reach the OS only through ports. `go run ./cmd/loops arch` must report 0 errors.
- **Every `go test` runs with `-race`.** Never weaken a test to make it pass.
- **The `events.jsonl` schema is additive-only.** Never rename a `run.Kind`, a `run.Phase` value or a JSON field: the status line, the mod pane, the report and older runs all read them.
- **Hooks fail open.** Bad input, no active run, or an internal error means exit 0 with no output. Only `read-guard` and `model-guard` deny (exit 2), and only `stop-gate` and `go-post-edit` block (JSON `decision`).
- **Keep docs and code in step.** Specs, skills and agents name only `loops` commands and flags that exist (`internal/adapters/inbound/cli`). When you change a command, update `specs/WORKFLOW.md` §5 and every caller in the same change.
- **Cite, don't duplicate.** Say each rule once, in its spec, and cite it elsewhere (`WORKFLOW §4`). Skill bodies stay under ~220 lines and agent bodies under ~90.
- **No game content.** This was extracted from rimworld-go, and nothing game-specific belongs here.

## Gates before committing
`gofmt -l .` (must print nothing) · `go vet ./...` · `go test -race -count=1 ./...` · `go run ./cmd/loops arch` · `jq . hooks/hooks.json .claude-plugin/*.json settings.json`
