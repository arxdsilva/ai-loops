# ai-loops: agent instructions

This repo is the **`loops`** Claude Code plugin: skills, role agents, specs and templates, plus the `loops` Go binary they drive. Read only what your task needs.

## Where things are
- The workflow spec, which everything implements: `specs/WORKFLOW.md`. Roles: `ROLES.md` (matrix) and `agents/*.md` (definitions).
- Rules for Go code, here and in target repos: `specs/languages/go/RULES.md`, `specs/ARCHITECTURE.md`, `specs/SOLID.md`. Agent rules: `specs/PARALLEL_WORK.md`, `specs/CONTEXT_DISCIPLINE.md`.
- Skills: `skills/<name>/SKILL.md`. The `work` skill is the orchestrator; the rest are reference skills that roles load.
- Templates the agents fill in: `templates/`.
- Plugin wiring for Claude Code (and Copilot, which reads the same layout): `.claude-plugin/`, `hooks/hooks.json`, `bin/loops` (a shim that builds `cmd/loops` on first use), `mod/` (the live pane), `settings.json`.
- Other platforms: `specs/PLATFORMS.md`. `platforms/<name>/`, `.agents/plugins/marketplace.json` and `.cursor-plugin/marketplace.json` are **generated** by `go run ./cmd/loops port build` from the canonical sources (embedded through `embed.go`). Never edit them by hand. Regenerate whenever agents/, skills/, specs/, templates/, ROLES.md, hooks/hooks.json or plugin.json change; CI runs `loops port check`.
- Go tooling, laid out hexagonally:
  - `internal/domain/{run,archrule,plugin}`: pure code, no I/O, no wall clock;
  - `internal/application/{tracker,guard,gate,archcheck,ticket,port}`: use cases with consumer-owned ports (`port` = per-platform emitters and the install plan);
  - `internal/adapters/{inbound/{cli,hook},outbound/{runstore,goparse,gotool,vcs,fsys,htmlreport,portfs}}`;
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

- `docs/media/`: the readme's workflow graphic. Regenerate it with `python3 docs/media/make_svg.py docs/media`. For the video, run `node docs/media/record.mjs docs/media/loops-workflow.svg <tmp>/frames 15 30`, then encode with ffmpeg into `loops-workflow.mp4` and `loops-workflow.webm`.

## Gates before committing
`gofmt -l .` (must print nothing) · `go vet ./...` · `go test -race -count=1 ./...` · `go run ./cmd/loops arch` · `go run ./cmd/loops port check` · `jq . hooks/hooks.json .claude-plugin/*.json settings.json`

The module and repository are `github.com/arxdsilva/ai-loops`. CI fails if the previous owner name reappears anywhere.

## Releases
A release is a `v<version>` tag, and the version lives only in `.claude-plugin/plugin.json` (the `loops` binary embeds it; there is no version constant). The generated package shims `go install …/cmd/loops@v<version>`.

Versions bump automatically: on every push to main, the CI `release` job runs `loops release --push`, which picks the level from the commits since the last tag (conventional commits: `feat!:` or a `BREAKING CHANGE` body is major, `feat:` minor, anything else patch; while the major version is 0, breaking changes bump minor), rewrites `plugin.json`, regenerates `platforms/` and the marketplaces, then commits `release vX.Y.Z`, tags it and pushes. The release commit and tag make the next run skip, so it does not loop. Write conventional commit subjects so the level is right.

- Preview: `go run ./cmd/loops release --dry-run` (works on a dirty tree).
- Manual override locally: `loops release minor` (or `patch`/`major`) on a clean tree; add `--push` to publish.
- Never edit the version by hand.
