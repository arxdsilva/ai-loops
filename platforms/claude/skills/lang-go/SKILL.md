---
name: lang-go
description: Use when writing, reviewing, testing or setting up Go backend code in a loops run (the go language pack) - toolchain, layout, race policy, coverage, idioms and footguns.
---

# Go pack (`go`)

## When it applies

`loops lang detect` reports `go` for a root (a `go.mod` there). Other packs in the same repo have their own `lang-<id>` skill; a multi-module repo is one `go` root per `go.mod`, and `./...` stops at a nested module, so run every check in each module directory (`specs/languages/go/RULES.md §2`, `repo.md`). `loops lang show go` prints the pack.

## Project rules first

Follow the project's own config and conventions over everything below: `.golangci.yml` (or `.yaml`, `.toml`, `.json`), `.editorconfig`, the `go` and `toolchain` lines in `go.mod`, an existing router, ORM or logger, and the project's `AGENTS.md`. Record every deviation from a loops rule with `loops run override --rule "<loops rule>" --project "<project rule, file:line>" --note "<what you did>"` (`ROLES.md` "Project rules first"). Detect the version before using a feature: `go.mod` `go` line decides (generic methods need 1.27, `wg.Go` 1.25, `b.Loop` 1.24).

## Toolchain the gate runs

`loops gate` runs these in order; while iterating run one check: `loops gate --only <check> --pkgs <owned>`. Full suite is the lead's and QA's job.

| Check | Command | Required |
|---|---|---|
| build | `go build ./...` | yes |
| gofmt | `gofmt -l .` (fails on any output; `vendor/`, `testdata/` ignored) | no |
| vet | `go vet ./...` | yes |
| staticcheck | `staticcheck ./...` (skipped if absent or too old; hint: `go install honnef.co/go/tools/cmd/staticcheck@latest`) | no |
| arch | `loops arch` (internal, `specs/languages/go/ARCHITECTURE.md`) | no |
| test | `go test -race -count=1 -covermode=atomic -coverprofile=<cov> ./...` | yes |
| coverage | new-code coverage of the Go profile, min 70% (cannot be skipped) | yes |
| govulncheck | `govulncheck ./...` (off unless `--vuln`; install: `go install golang.org/x/vuln/cmd/govulncheck@latest`) | no |

The pack's `post_edit` hook runs `gofmt -w <file>` and `go vet ./<dir>` after an edit. Source is `**/*.go`, tests `**/*_test.go`; generated files (`*.pb.go`, `*_gen.go`, `zz_generated*`, `// Code generated ... DO NOT EDIT.`) are never edited by hand and never counted.

Beyond the gate, CI should also run `go mod tidy -diff`, `go mod verify`, `golangci-lint run` when configured, `gosec`, and `govulncheck` (RULES §2, §38). Prefer tools pinned with the `tool` directive: `go tool <name>`.

Go commands run in the run-scoped Go environment: plain `go ...` is scoped by the `go-env` hook, or use `loops go ...` if it denies with a hint. Never set `GOCACHE`; build binaries into the run's tmp dir (`GOTMPDIR` from `loops run env`). `GOMODCACHE` and `GOPATH` stay shared. `GOFLAGS=-p=2` keeps parallel agents from starving each other.

## Race policy (WORKFLOW §6)

The pack's mode is `flag`: every `go test` carries `-race`; the `race-guard` hook denies a `go test` (or `gotestsum`/`richgo` wrapper) without the race detector, including `-race=false` and a `GOFLAGS` that turns it off. `-race` needs cgo and a C compiler: if it fails to build, install gcc (or use a `golang:<ver>` image; alpine needs `gcc musl-dev`), never drop the flag. Expect 5-10x memory and 2-20x time; raise timeouts, never weaken the policy. Races only show on executed paths, so write the concurrent test. Detail: `concurrency.md`.

## Coverage recipe

`go test -race -count=1 -covermode=atomic -coverprofile={coverage} ./...` writes a `gocover` profile; the gate computes new-code coverage from it against `git diff` and excludes `cmd/**`, `internal/compose/**`, `**/testdata/**` and generated code. The recipe has no `-coverpkg`, so a package is credited only by its own tests: put tests next to the code they cover. Integration binaries: `go build -cover`, `GOCOVERDIR`, `go tool covdata textfmt` (`testing.md`).

Wave 0 prerequisites (architect or `go-engineer`; the only roles that touch `go.mod`/`go.sum`/shared config): `go.mod` with module path, `go` and `toolchain` lines; `.gitignore` entries for `*.test`, `*.out`, `*.prof`, `*.cov`, `coverage*.txt`; a `.golangci.yml` with `version: "2"` when the project lints; a `go build ./... && go vet ./...` clean skeleton. No coverage config file is required.

## Layers

```text
cmd/<bin>/main.go            wiring only, <= 80 lines
internal/
  domain/<pkg>/              pure: stdlib only; no os, net/http, time.Now, math/rand, go statements
  application/<usecase>/     use cases + consumer-owned ports; stdlib only
  adapters/inbound/{http,grpc,cli,queue}/   handlers, middleware, consumers
  adapters/outbound/{pg,httpclient,fs,...}/ repositories, clients; frameworks live here
  compose/                   builds the object graph; the only place that knows concrete types
  platform/                  config, logging, telemetry setup
```

Direction: domain <- application <- adapters <- compose <- cmd. `loops arch` enforces it (`specs/ARCHITECTURE.md`, `specs/languages/go/ARCHITECTURE.md`). Avoid `pkg/`, `utils/`, `common/`.

## Footguns

1. `go test ./...` at a multi-module root skips nested modules: run per module.
2. `-race` with `CGO_ENABLED=0` or no C compiler: install gcc, keep `-race`.
3. Cached test results hide failures: `-count=1`.
4. `resp, _ := http.Get(u)` without `defer resp.Body.Close()` (and drain): leaks connections.
5. `var e *MyErr; return e` is a non-nil `error`: return `nil` explicitly.
6. `http.Get`, `http.ListenAndServe` without timeouts: custom `Client{Timeout}`, `Server{ReadHeaderTimeout...}`.
7. Goroutine nobody waits for, send on a channel nobody reads: owner plus `errgroup`, `ctx.Done()`, `goleak`.
8. Decisions on map iteration order, `append` to a sub-slice aliasing the parent: sort keys, `slices.Clone`.
9. `time.Now()` or global rand in business code: inject `Clock` and `*rand.Rand`; `synctest` for timers.
10. `AutoMigrate` on boot, hand-edited `*.pb.go` or sqlc output: versioned migration job, regenerate in CI.

## Load on demand

- `repo.md`: layout, modules, `go.work`, toolchain pin, `tool` directive, CI shape, releases (RULES §2, §41, §48, §49).
- `architecture.md`: hexagonal mapping, ports, wiring, DDD and messaging in Go (`specs/languages/go/ARCHITECTURE.md`).
- `idioms.md`: errors, context, zero values, generics, iterators, naming, resources (RULES §7-§27).
- `testing.md`: table tests, fakes, fuzz, shuffle, integration, coverage, benchmarks (RULES §30, §31).
- `concurrency.md`: ownership, errgroup, shutdown, leaks, synctest, race triage (RULES §11-§13).
- `security.md`: input, SQL, paths with `os.Root`, servers, SSRF, crypto, supply chain (RULES §38).
- `data-api.md`: persistence, HTTP and gRPC APIs, observability, configuration and packaging (RULES §24, §28, §44, §45).
- `frameworks.md`: chi, gin, echo, Connect, GORM, ent, sqlc, and the std-first rule (RULES §48).
