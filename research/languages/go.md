# Go: backend practices and loops gap analysis

| | |
|---|---|
| Pack id | `go` |
| Current stable / LTS | `1.27` (released 2026-08-19; local toolchain here is 1.27.3) / none (support window: each Go major is supported until two newer majors exist, so 1.27 and 1.26 are supported as of 2026-11; 1.25 has just aged out) |
| Dominant runtimes / frameworks | Go runtime (gc toolchain); `net/http` + `ServeMux` (1.22 patterns), chi, gin, echo, Fiber; gRPC / Connect; pgx, sqlc, GORM, ent |
| As of | 2026-11 |
| Researcher | `B1-go` |
| Verifier | (filled in by B2) |

> **Summary:** loops is already a strong Go pack: formatting, vet, staticcheck, `-race` tests, new-code coverage, arch checking and a run-scoped build cache are all enforced mechanically, and GO_BEST/HEXAGONAL/SOLID say most of the idiom rules once. The gaps are the "around the code" topics (repo creation, multi-module and `go.work` repos, `tool` directives, CI shape, release, containers, persistence, API, OTel) and a layer of 2024-2026 language and library changes that make parts of GO_BEST outdated (stdlib-first list, `os.Root`, `b.Loop`, `t.Context`, `synctest`, iterators, `errors.AsType`, JSON v2, UUID in std). The gate also breaks silently in multi-module repos (`./...` does not cross module boundaries) and has no SAST, no `-shuffle`, no cross-package coverage and no integration-tag run.

Rules follow [TAXONOMY.md](TAXONOMY.md), using the row format `ID | Rule | Why | Sev | Source | loops today | Enforce`. Gaps are judged against [LOOPS_BASELINE.md](LOOPS_BASELINE.md). `covered` rows cite the spec section (`GB` = `specs/GO_BEST_PRACTICES.md`, `HX` = `specs/HEXAGONAL_ARCHITECTURE.md`, `SG` = `specs/SOLID_GO.md`, `WF` = `specs/WORKFLOW.md`). Because loops is Go-first, rows loops already says in Go terms are marked `covered` (not `go-only`) with the spec cited; `go-only` is used where the rule exists only as a Go tool in the binary and nothing neutral says it.

Source shorthand used in rows: **GoMod** = [Go Modules Reference](https://go.dev/ref/mod); **Layout** = [Organizing a Go module](https://go.dev/doc/modules/layout); **cmd/go** = [pkg.go.dev/cmd/go](https://pkg.go.dev/cmd/go); **CRC** = [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments); **GSG** = [Google Go Style Guide](https://google.github.io/styleguide/go/); **Uber** = [Uber Go Style Guide](https://github.com/uber-go/guide); **Effective Go** = [go.dev/doc/effective_go](https://go.dev/doc/effective_go); **1.25 / 1.26 / 1.27 notes** = `https://go.dev/doc/go1.25`, `/go1.26`, `/go1.27`.

## 1. REPO: repository and project management

Canonical tree for a service (single module, hexagonal; see ARCH-02 for the layer mapping):

```text
orders/
  go.mod                  # module github.com/acme/orders ; go 1.26.0 ; toolchain go1.26.4 ; tool (...)
  go.sum                  # committed, always
  .golangci.yml           # version: "2"
  .editorconfig  .gitattributes  .gitignore  CODEOWNERS  LICENSE  README.md  AGENTS.md
  Makefile | Taskfile.yml # thin: fmt lint test cover vuln build image
  .github/workflows/ci.yml  .github/dependabot.yml (or renovate.json)
  cmd/
    orders-api/main.go    # wiring only, <= 80 lines
    orders-worker/main.go
  internal/
    domain/  application/  adapters/{inbound,outbound}/  compose/  platform/
  api/                    # openapi.yaml / *.proto / buf.yaml (contract source of truth)
  db/migrations/  db/queries/  sqlc.yaml
  deploy/                 # Dockerfile, k8s/helm, compose.yaml
  docs/adr/
  testdata/               # per-package testdata/ dirs are ignored by ./...
```

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `GO-REPO-01` | Put a service as `cmd/<bin>/main.go` plus everything else under `internal/`; export packages only when another module must import them, and then split that code into its own module. [REPO-01] | `internal/` is compiler-enforced privacy; the official server layout is exactly this. | MUST | [Layout](https://go.dev/doc/modules/layout) | covered (HX §4, §15) | arch |
| `GO-REPO-02` | Do not create `pkg/`, `utils/`, `common/`, `models/` or a top-level `src/`; the golang-standards "project-layout" repo is not an official standard. [REPO-01] | Nothing in the toolchain needs them; they invite dumping grounds. | AVOID | [Layout](https://go.dev/doc/modules/layout), GB §3 (loops spec) | covered (GB §3, §41; HX §15 allows `pkg/` as stdlib-only) | review |
| `GO-REPO-03` | Default to one `go.mod` at the repo root. Add a second module only for a separately versioned/released library, a tool with a different dependency set, or a generated client; each gets its own `go.mod` and version tags `path/vX.Y.Z`. [REPO-01, REPO-02] | Multi-module repos multiply tidy/upgrade/CI cost; the module is the unit of versioning. | SHOULD | [GoMod](https://go.dev/ref/mod#vcs-version), [Layout](https://go.dev/doc/modules/layout) | missing | review |
| `GO-REPO-04` | In a multi-module repo, run every check per module: `./...` stops at a nested `go.mod`. Loop `go work edit -json` (or `find . -name go.mod`) and run build/vet/test in each module directory. [REPO-02, REPO-08] | A root-level `go test ./...` silently skips nested modules: the gate would be green with untested code. | MUST | [cmd/go](https://pkg.go.dev/cmd/go) (`go help packages`: patterns never cross a module boundary), [GoMod](https://go.dev/ref/mod) | missing (gate runs `./...` at the project root only) | gate |
| `GO-REPO-05` | Use `go.work` only for local multi-module development; do not commit it (add `go.work` and `go.work.sum` to `.gitignore`) unless every module in the repo is developed only against each other. CI sets `GOWORK=off` or builds each module standalone. [REPO-02] | A committed `go.work` overrides parent workspaces and lets CI test the wrong dependency versions. | SHOULD | [GoMod: workspaces](https://go.dev/ref/mod#workspaces), [Tutorial](https://go.dev/doc/tutorial/workspaces) | missing | review |
| `GO-REPO-06` | Bootstrap with `go mod init <module-path>` where the path is the repo URL; the module path decides import paths forever, so choose it first. Day-one files: README, LICENSE, `.gitignore`, `.golangci.yml`, CI workflow, `AGENTS.md`, one passing test. Note: the 1.26 release notes say `go mod init` writes `go 1.(N-1).0`, but go1.26.4 and go1.27.3 were observed writing the full local version (`go 1.27.3`); lower it deliberately with `go get go@1.N.0` if you must support an older toolchain. [REPO-03] | Renaming a module later breaks every import. The `go` line is a hard minimum for consumers. | MUST | [GoMod](https://go.dev/ref/mod#go-mod-init), [1.26 notes](https://go.dev/doc/go1.26) (claim not borne out in practice) | missing | review |
| `GO-REPO-07` | Commit `go.mod` and `go.sum` for apps and libraries alike (Go has no separate lockfile; `go.sum` holds checksums, and `go.mod` MVS selection is the lock). Never commit `vendor/` unless the project needs hermetic builds, and then keep it verified in CI with `go mod vendor && git diff --exit-code`. [REPO-04] | MVS is deterministic from `go.mod`; `go.sum` guards integrity through the checksum DB. | MUST | [GoMod](https://go.dev/ref/mod#go-sum-files), [vendoring](https://go.dev/ref/mod#vendoring) | missing | gate |
| `GO-REPO-08` | CI runs `go mod tidy -diff` (exit non-zero if `go.mod`/`go.sum` need changes) and `go mod verify`. [REPO-04, REPO-08] | Catches forgotten tidy, unused dependencies and tampered cache entries. | SHOULD | [cmd/go: go mod tidy -diff / verify](https://pkg.go.dev/cmd/go) | missing | gate |
| `GO-REPO-09` | Update dependencies with Dependabot (`package-ecosystem: gomod`) or Renovate's `gomod` manager (it also runs `go mod tidy`), grouped by minor/patch, with `govulncheck` and `-race` tests as required checks; never auto-merge a major bump. [REPO-04] | Automated, small, reviewed upgrades beat a yearly big-bang. | SHOULD | [Renovate gomod](https://docs.renovatebot.com/modules/manager/gomod/), [Dependabot ecosystems](https://docs.github.com/en/code-security/dependabot/ecosystems-supported-by-dependabot/supported-ecosystems-and-repositories) | covered | review |
| `GO-REPO-10` | Version by SemVer through git tags `vMAJOR.MINOR.PATCH`; a v2+ module changes its module path to end in `/v2`. Check a library's API diff with `gorelease` before tagging. Applications are released by tag from CI. [REPO-05] | The module proxy and MVS resolve versions from tags; breaking changes without a path change break consumers. | MUST | [Version numbers](https://go.dev/doc/modules/version-numbers), [Release workflow](https://go.dev/doc/modules/release-workflow), [gorelease](https://pkg.go.dev/golang.org/x/exp/cmd/gorelease) | covered (AGENTS.md release flow in this repo; no generic rule) | review |
| `GO-REPO-11` | Pin the toolchain in `go.mod`: `go 1.26.0` is the minimum language version (a hard floor since 1.21), `toolchain go1.26.4` is the suggested exact toolchain; CI uses `actions/setup-go` with `go-version-file: go.mod`. Use `GOTOOLCHAIN=local` in air-gapped builds. [REPO-06] | Reproducible compiler version; the `go` line alone is only a lower bound. | MUST | [Toolchains](https://go.dev/doc/toolchain), [GoMod](https://go.dev/ref/mod#go-mod-file-toolchain) | missing | gate |
| `GO-REPO-12` | Track dev tools in `go.mod` with the Go 1.24 `tool` directive (`go get -tool honnef.co/go/tools/cmd/staticcheck@<version>`, run as `go tool staticcheck`) instead of `tools.go` blank imports or `@latest` installs. [REPO-04, REPO-06] | One pinned version per repo, checksummed, no PATH assumptions; `go install tool` installs them all. | SHOULD | [GoMod: tool directive](https://go.dev/ref/mod#go-mod-file-tool), [1.24 tools](https://go.dev/doc/modules/managing-dependencies#tools) | missing (gate looks for `staticcheck`/`govulncheck` on PATH; WF §2.3 `go get`) | gate |
| `GO-REPO-13` | `.gitignore` for Go: `*.test`, `*.out`, `*.prof`, `*.pprof`, `*.cov`, `coverage*.txt`, `/bin/`, `/dist/`, `go.work`, `go.work.sum`, `__debug_bin*`, `.env`. Add `.gitattributes` `*.go text eol=lf` and mark generated files `linguist-generated`. [REPO-07] | Keeps profiling/coverage output out of commits (same list loops cleans up). | SHOULD | [GitHub Go.gitignore](https://github.com/github/gitignore/blob/main/Go.gitignore), [cmd/go](https://pkg.go.dev/cmd/go) | go-only (`workspace.IsArtifactName` cleanup list) | hook |
| `GO-REPO-14` | CI shape per PR: setup-go with module cache -> `go mod tidy -diff` -> format check -> `go vet` -> `golangci-lint run` -> `go build ./...` -> `go test -race -count=1 -shuffle=on -covermode=atomic -coverprofile` -> `govulncheck ./...` -> (main only) build image + sign. Matrix over the oldest supported and the newest Go on `ubuntu` and, if you ship it, `macos`/`windows`. [REPO-08] | Fast failures first; matrix proves the `go` floor in `go.mod`. | SHOULD | [setup-go](https://github.com/actions/setup-go), [golangci-lint-action](https://github.com/golangci/golangci-lint-action), [govulncheck-action](https://github.com/golang/govulncheck-action) | covered | review |
| `GO-REPO-15` | Use conventional commits; the release job derives the next tag from them (as `loops release` does here); sign release artifacts and publish checksums/provenance (cosign/SLSA). [REPO-09, REPO-05] | Reproducible, attributable releases; Go toolchains themselves are reproducible. | SHOULD | [Reproducible toolchains](https://go.dev/blog/rebuild), [SLSA](https://slsa.dev/), [cosign](https://github.com/sigstore/cosign) | covered (AGENTS.md release flow) | review |

## 2. TOOL: toolchain

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `GO-TOOL-01` | Target the two newest majors (1.27 and 1.26 as of 2026-11); releases come every February and August; install through `go` itself (`GOTOOLCHAIN=auto` downloads the version `go.mod` asks for) or `mise`/`asdf`. [TOOL-01] | Only the two latest majors get security fixes. | SHOULD | [Release policy](https://go.dev/doc/devel/release), [Toolchains](https://go.dev/doc/toolchain), [1.27 notes](https://go.dev/doc/go1.27) | missing | review |
| `GO-TOOL-02` | Format with `gofmt` (check argv: `gofmt -l .`, fail if it prints anything); use `gofumpt` or `goimports` only if the project standardises on one, preferably through golangci-lint v2 `formatters:`. Re-run `gofmt` after upgrading to 1.27 (alignment fix may reflow files). [TOOL-02] | One canonical format; CI enforces, reviewers do not. | MUST | [gofmt](https://pkg.go.dev/cmd/gofmt), [1.27 notes](https://go.dev/doc/go1.27), [gofumpt](https://github.com/mvdan/gofumpt) | covered (GB §2; gate `gofmt -l`, hook `go-post-edit` runs `gofmt -w`/`goimports -w`) | gate |
| `GO-TOOL-03` | Run `go vet ./...` (it also compiles tests); 1.25 added `waitgroup` and `hostport` analyzers and 1.27 turns `stdversion` on for `go test`. [TOOL-03] | Vet is the compiler-adjacent correctness floor. | MUST | [cmd/vet](https://pkg.go.dev/cmd/vet), [1.25 notes](https://go.dev/doc/go1.25), [1.27 notes](https://go.dev/doc/go1.27) | covered (GB §2; gate `vet`, required) | gate |
| `GO-TOOL-04` | Use `staticcheck ./...` or golangci-lint **v2** (`version: "2"`, `linters.default: standard`, explicit `enable:`, `formatters:` section, `exclusions.generated: lax`); never mix v1 and v2 configs (`golangci-lint migrate`). Required minimum set: `errcheck govet staticcheck unused ineffassign` + `errorlint revive gosec bodyclose contextcheck noctx sloglint forbidigo depguard copyloopvar`. [TOOL-03] | v2 is the supported config format; the extra linters catch the classic Go footguns (unclosed bodies, missing ctx, `time.Now`). | SHOULD | [golangci-lint config](https://golangci-lint.run/docs/configuration/file/), [migration](https://golangci-lint.run/docs/product/migration-guide/), [staticcheck](https://staticcheck.dev/docs/) | covered | gate |
| `GO-TOOL-05` | Type checking is the compiler plus `go vet`; there is no separate checker. Keep `go vet` on test files and build tags used in CI (`-tags=integration`). [TOOL-04] | `go build ./...` alone does not compile `_test.go` or tag-gated files. | MUST | [cmd/go](https://pkg.go.dev/cmd/go) | covered | gate |
| `GO-TOOL-06` | Scan with `govulncheck ./...` in CI (source mode, reachability-aware; exit non-zero when vulnerabilities are called) and `govulncheck -mode=binary ./app` on the release artifact; with `-format sarif`/`openvex` it exits 0, so gate on the findings yourself. [TOOL-05] | Reports only vulnerabilities your code reaches, from the Go vulnerability DB. | MUST | [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck), [go.dev/doc/security/vuln](https://go.dev/doc/security/vuln/) | covered | gate |
| `GO-TOOL-07` | Produce an SBOM with `cyclonedx-gomod` or `syft`, and check licenses with `go-licenses`/`trivy`; attach to the release. [TOOL-05] | Compliance and incident response. | SHOULD | [cyclonedx-gomod](https://github.com/CycloneDX/cyclonedx-gomod), [go-licenses](https://github.com/google/go-licenses) | missing | review |
| `GO-TOOL-08` | Use `gopls` (it bundles the analyzers, `gofmt` on save, `goimports` organise-imports); do not run a second formatter on save. Run `go fix -diff ./...` periodically (Go 1.26 rewrote `go fix` into a modernizer suite; 1.27 adds `atomictypes`, `slicesbackward`, ...). [TOOL-06] | Format-on-save fights and stale idioms cause diff noise. | SHOULD | [gopls](https://go.dev/gopls/), [go fix blog](https://go.dev/blog/gofix), [1.27 notes](https://go.dev/doc/go1.27) | missing | gate |
| `GO-TOOL-09` | Use `gotestsum --format testname --junitfile` in CI for readable output and JUnit; it still must pass `-race` (loops' race-guard recognises `gotestsum`/`richgo`). [TOOL-02] | Better failure triage than raw `go test`. | SHOULD | [gotestsum](https://github.com/gotestyourself/gotestsum) | covered (race-guard wrapper list) | hook |

## 3. ARCH: architecture

Mapping of loops layers onto the Go layout (ARCH-02), as already specified in HX §4:

```text
internal/
  domain/<ctx>/            # entities, value objects, domain services; std lib only; no os, net/http, time.Now, rand globals, go stmts
  application/<usecase>/   # use cases + consumer-owned ports (interfaces declared next to the user)
  adapters/
    inbound/{http,grpc,cli,consumer}/   # decode, authn/z, call a use case, map errors
    outbound/{postgres,kafka,httpclient,clock}/   # implement the ports; may use third-party libs
  compose/                 # builds the object graph (only cmd imports it)
  platform/                # config, logging, telemetry setup
cmd/<bin>/main.go          # <=80 lines: parse flags/env, call compose, run, shutdown
```

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `GO-ARCH-01` | Choose the style deliberately: modular monolith with hexagonal packages is the default for a Go service; split into services only along a team or scaling boundary; CQRS/event sourcing only where the read/write models truly diverge; the actor model is expressed with goroutines owning state and channels (one owner per state). [ARCH-01] | Go favours small, explicit, in-process composition; distribution is a cost. | SHOULD | [Layout](https://go.dev/doc/modules/layout), [Effective Go: share by communicating](https://go.dev/doc/effective_go#sharing) | covered (HX §2, §10-12, §16; GB §11) | review |
| `GO-ARCH-02` | The domain imports only the standard library and other domain packages; no `os`, `net/http`, `database/sql`, no `time.Now`, global `math/rand`, no `go` statements. [ARCH-06] | Pure domain is testable without I/O. | MUST | [GSG: package design](https://google.github.io/styleguide/go/decisions), HX §3 (loops spec) | covered (HX §3, GB §13/§20/§21; `loops arch` errors on `os`, `net/http`, `time.Now`, global `math/rand(/v2) | arch |
| `GO-ARCH-03` | Direction is domain <- application <- adapters <- compose <- cmd; outbound adapters never import inbound ones; `internal/` makes it a compile error for outside modules to reach in. [ARCH-03, ARCH-04] | Dependency rule. | MUST | [Layout](https://go.dev/doc/modules/layout), [`internal` rule](https://go.dev/doc/go1.4#internalpackages) | covered (HX §3; arch checker) | arch |
| `GO-ARCH-04` | Package name = directory name (last path element), lowercase, no underscores; an import is `import "mod/path/pkg"` or aliased/grouped in an `import (...)` block; import cycles are a compile error. The arch checker resolves imports by module path + directory. [ARCH-03] | Needed for the checker's resolver `module-path`. | MUST | [CRC: package names](https://go.dev/wiki/CodeReviewComments#package-names), [Spec: packages](https://go.dev/ref/spec#Packages) | covered (HX §4; goparse) | arch |
| `GO-ARCH-05` | Enforce the rules with `loops arch`; in a project without loops use `go-arch-lint` (YAML component rules), `arch-go`, or golangci-lint `depguard` (deny lists per package glob). [ARCH-04] | A dependency rule that is not checked drifts. | SHOULD | [go-arch-lint](https://github.com/fe3dback/go-arch-lint), [arch-go](https://github.com/arch-go/arch-go), [depguard in golangci-lint](https://golangci-lint.run/docs/linters/) | covered (HX §25; gate `arch`) | arch |
| `GO-ARCH-06` | Wire by hand in `cmd/<bin>` through a `compose` package: constructors take interfaces, return concrete types, no globals, no `init()` registration, no service locator, no reflection DI. Use `wire` only when the graph is huge. [ARCH-05] | Explicit graph is readable and testable; fx/dig hide it at runtime. | MUST | [Uber: avoid init()](https://github.com/uber-go/guide/blob/master/style.md#avoid-init), [wire](https://github.com/google/wire) | covered (HX §21-23; SG §8a; arch rules `init`, `global`, `cmd`) | arch |
| `GO-ARCH-07` | Interfaces are small and defined by the consumer, next to the code that calls them; return structs, accept interfaces; no `Mock...` interface for every type. [ARCH-07] | Implicit satisfaction makes consumer-owned ports free. | MUST | [CRC: interfaces](https://go.dev/wiki/CodeReviewComments#interfaces), [Effective Go](https://go.dev/doc/effective_go#interfaces) | covered (GB §5, §6; SG §6-7; arch iface size warning) | arch |
| `GO-ARCH-08` | Treat the loops size limits as warnings: type 40 methods, struct 25 fields (with >= min methods), interface 8 methods, package 12000 lines / 60 files, `cmd/<x>/main.go` 80 lines and no string literal over 40 runes (errors). [ARCH-08] | Size is a proxy for responsibility count. | SHOULD | [GSG: package size](https://google.github.io/styleguide/go/best-practices), repo `internal/domain/archrule` | covered (SG §8) | arch |
| `GO-ARCH-09` | DDD tactical: aggregates are struct types behind one package; reference other aggregates by ID; value objects are comparable value types; repositories are consumer-owned ports with `Get/Save` by aggregate; domain events are plain structs published through a port. [ARCH-01] | Fits Go's value types and package encapsulation. | SHOULD | [Three Dots Labs: DDD in Go](https://threedots.tech/post/ddd-lite-in-go-introduction/) (corroboration), GB §16 | covered (GB §16; HX §9-11, §16) | review |
| `GO-ARCH-10` | Event-driven and messaging code lives in inbound (consumer) and outbound (publisher) adapters; the application sees only a port such as `Publisher.Publish(ctx, Event)`; use the outbox pattern for DB+broker atomicity. [ARCH-01] | Keeps broker libraries out of the core. | SHOULD | [Microservices.io outbox](https://microservices.io/patterns/data/transactional-outbox.html) | covered | review |
| `GO-ARCH-11` | Serverless (Lambda/Cloud Functions): the handler is an inbound adapter; cold-start cost favours a small static binary (`provided.al2023` runtime), no `init()` work, lazy clients. [ARCH-01] | Same hexagon; adapter is the handler. | SHOULD | [aws-lambda-go](https://github.com/aws/aws-lambda-go) | missing | review |
| `GO-ARCH-12` | Use the framework's types (chi router, gRPC server, sqlc `Queries`) only inside adapters; generated code lives in the adapter that owns it and is never imported from domain/application. [ARCH-02] | Frameworks change; ports do not. | MUST | GB §37 (loops spec), [Connect](https://connectrpc.com/docs/go/getting-started) | covered (GB §37, §48, §44) | arch |

## 4. IDIOM: what to do and what to avoid

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `GO-IDIOM-01` | Return `error` as the last value; wrap with `fmt.Errorf("verb noun %q: %w", x, err)`; handle an error once (return or log); never discard with `_` without a comment. Test with `errors.Is`/`errors.As`, or in 1.26+ `errors.AsType[*MyErr](err)`. [IDIOM-01] | Context chains and typed matching. | MUST | [Working with errors](https://go.dev/blog/go1.13-errors), [CRC: errors](https://go.dev/wiki/CodeReviewComments#handle-errors), [1.26 notes](https://go.dev/doc/go1.26) | covered (GB §8, §9) | gate |
| `GO-IDIOM-02` | Expected business outcomes are result types, not errors; sentinel/typed errors only when callers branch on them; combine with `errors.Join`. [IDIOM-01] | Errors are for failures. | SHOULD | [Effective Go: errors](https://go.dev/doc/effective_go#errors) | covered (GB §8-9; HX §13) | review |
| `GO-IDIOM-03` | Panic only for programmer invariants; recover only at an edge (HTTP middleware, goroutine entry) and convert to an error response. Never `log.Fatal`/`os.Exit` outside `main`. [IDIOM-01] | Library code that exits cannot be tested or shut down cleanly. | MUST | [Effective Go: panic](https://go.dev/doc/effective_go#panic), [Uber: don't panic](https://github.com/uber-go/guide/blob/master/style.md#dont-panic) | covered (GB §25) | review |
| `GO-IDIOM-04` | Make the zero value useful; use `*T` only for optionality at a boundary; since 1.26 `new(expr)` can set a pointer field inline (`Age: new(yearsSince(b))`); beware a nil interface holding a nil pointer (`err != nil` is true). [IDIOM-02] | Go has no Option type; typed-nil traps. | SHOULD | [Go FAQ: nil error](https://go.dev/doc/faq#nil_error), [1.26 notes](https://go.dev/doc/go1.26), GB §7 (loops spec) | covered (GB §7) | review |
| `GO-IDIOM-05` | Value types: small immutable values (IDs, money, timestamps) are passed by value; never expose a mutable slice/map from a struct (clone with `slices.Clone`/`maps.Clone`); no pointer to loop var needed since Go 1.22 (per-iteration loop variables). [IDIOM-03] | Aliasing bugs; the old `tt := tt` copy is obsolete. | SHOULD | [Fixing For Loops in Go 1.22](https://go.dev/blog/loopvar-preview), GB §14-15 (loops spec) | covered (GB §14, §15; loopvar missing) | review |
| `GO-IDIOM-06` | Generics: use for container/algorithm duplication (`slices`, `maps`, `cmp`, `iter`); no generic frameworks; generic type aliases (1.24), self-referential constraints (1.26) and generic methods (1.27, methods may now declare own type parameters; interface methods still may not) exist but do not make them a reason to add type parameters. [IDIOM-04] | Readability first. | SHOULD | [Generics intro](https://go.dev/blog/intro-generics), [1.27 notes](https://go.dev/doc/go1.27), [1.26 notes](https://go.dev/doc/go1.26) | covered (GB §17) | review |
| `GO-IDIOM-07` | Enums are typed constants with `iota` and a meaningful zero value (`Unknown`); there are no sum types, so close the set with an unexported method on an interface and a `default: panic`/error in `switch`; use `exhaustive` linter. [IDIOM-04] | Compiler cannot check exhaustiveness. | SHOULD | [Effective Go: constants](https://go.dev/doc/effective_go#constants), [exhaustive](https://github.com/nishanths/exhaustive) | covered (GB §18) | gate |
| `GO-IDIOM-08` | Iterators: prefer `iter.Seq`/`iter.Seq2` and `for range seq` for lazy sequences, `slices.Collect`, `maps.Keys`, `slices.Sorted(maps.Keys(m))`; keep indexed loops in measured hot paths. [IDIOM-04] | Range-over-func (1.23) replaces callback/channel iterators; pull iterators need `iter.Pull` + `defer stop()`. | SHOULD | [Range over function types](https://go.dev/blog/range-functions), [pkg iter](https://pkg.go.dev/iter) | missing (GB §39 only warns about closure iterators) | review |
| `GO-IDIOM-09` | Release every resource with `defer` right after acquiring it: `defer rows.Close()`, `defer resp.Body.Close()` (and read it to EOF to reuse the connection), `defer cancel()` after `WithCancel/WithTimeout`, `defer tx.Rollback()`. Check the `Close` error for writers. [IDIOM-05] | Leaks of connections, files and contexts. | MUST | [Effective Go: defer](https://go.dev/doc/effective_go#defer), [bodyclose](https://github.com/timakin/bodyclose) | covered | gate |
| `GO-IDIOM-10` | Strings: build with `strings.Builder`/`bytes.Buffer`, not `+=` in loops; split with `strings.Cut`/`CutLast` (1.27); use `time.Time` in UTC internally, format with `time.RFC3339`; compare times with `Equal`; never use `==` on `time.Time`. [IDIOM-06] | Quadratic concatenation; monotonic-clock/zone surprises. | SHOULD | [time package](https://pkg.go.dev/time), GB §20, §23 (loops spec) | covered (GB §20, §23) | review |
| `GO-IDIOM-11` | Maps: never rely on iteration order (randomised), iterate `slices.Sorted(maps.Keys(m))` when order matters; writing to a nil map panics; maps are not safe for concurrent writes. [IDIOM-06, CONC-02] | Common nondeterminism bug. | MUST | [Go maps in action](https://go.dev/blog/maps), GB §13-14, §34 (loops spec) | covered (GB §13, §14, §34) | review |
| `GO-IDIOM-12` | Naming: MixedCaps, short receivers (1-2 letters, consistent), initialisms uppercase (`ID`, `URL`, `HTTP`), no `Get` prefix, no stutter (`order.Book` not `order.OrderBook`), package comments `// Package x ...`. [IDIOM-07] | Official style. | SHOULD | [CRC](https://go.dev/wiki/CodeReviewComments), [GSG](https://google.github.io/styleguide/go/decisions), [Effective Go: names](https://go.dev/doc/effective_go#names) | covered (GB §3, §26, §41) | gate |
| `GO-IDIOM-13` | Exported API surface is minimal; every exported name has a doc comment starting with its name; deprecate with a `// Deprecated: ...` paragraph. [IDIOM-07] | `go doc`/pkg.go.dev render these; staticcheck flags use of deprecated names. | SHOULD | [Go Doc Comments](https://go.dev/doc/comment) | covered (GB §4, §26) | gate |
| `GO-IDIOM-14` | Reflection and `any`: no `any`/`map[string]any` in domain/application APIs; reflection only in serialisation/tooling adapters; avoid `unsafe`, cgo (disables the pure-Go static build and `CGO_ENABLED=0` cross-compiles) and `//go:linkname` outside adapters with an ADR. [IDIOM-08] | Type safety; `-race` needs cgo while static builds need it off, so keep the core cgo-free. | AVOID | [CRC](https://go.dev/wiki/CodeReviewComments), GB §17, §37 (loops spec) | covered (GB §17, §37) | review |
| `GO-IDIOM-15` | `init()` is for nothing but trivial registration of database/sql-style drivers in `cmd`; never use it in libraries, domain, or adapters. Package-level mutable state is forbidden. [IDIOM-09] | Hidden order dependence and test pollution. | AVOID | [Uber: avoid mutable globals / init](https://github.com/uber-go/guide/blob/master/style.md#avoid-mutable-globals) | covered (SG §9; arch `init`, `global`) | arch |
| `GO-IDIOM-16` | Slices: know that `append` may alias the caller's backing array (use full slice expressions `s[:n:n]` or `slices.Clone`), that re-slicing a huge array retains it, and that `copy` copies `min(len)`; preallocate with `make([]T, 0, n)`. [IDIOM-09] | Aliasing and retention footguns. | SHOULD | [Slices usage and internals](https://go.dev/blog/slices-intro), GB §14, §40 (loops spec) | covered (GB §14, §40) | review |
| `GO-IDIOM-17` | Use built-ins and std helpers from 1.21+: `min`, `max`, `clear`, `slices.*`, `maps.*`, `cmp.Compare`, `sync.OnceValue`; delete hand-written equivalents (`go fix` modernizers suggest them). [IDIOM-04] | Less code and fewer bugs. | SHOULD | [1.21 notes](https://go.dev/doc/go1.21), [go fix](https://go.dev/blog/gofix) | missing | gate |
| `GO-IDIOM-18` | Struct literals use field names; keep `sync.Mutex`/`sync.WaitGroup` as values and never copy a struct containing one (`go vet copylocks`); embed interfaces/structs sparingly (embedding promotes the whole method set into your API). [IDIOM-09] | Copying locks breaks them silently. | MUST | [vet copylocks](https://pkg.go.dev/cmd/vet), [Uber: embedding](https://github.com/uber-go/guide/blob/master/style.md#embedding-in-structs) | covered (vet in gate) | gate |
| `GO-IDIOM-19` | Use `context.Context` only for deadlines, cancellation and request-scoped values; first parameter, named `ctx`; never in a struct, never nil; use `context.WithCancelCause`/`WithoutCancel`/`AfterFunc` where they fit; avoid `context.Value` for optional parameters. [IDIOM-01] | The official rule; cause-aware cancellation aids debugging. | MUST | [Contexts and structs](https://go.dev/blog/context-and-structs), [pkg context](https://pkg.go.dev/context), [CRC: contexts](https://go.dev/wiki/CodeReviewComments#contexts) | covered (GB §10; HX §14) | gate |

## 5. CONC: concurrency and the `-race` equivalent

**Loops race policy: `mode = flag`** with `go test -race -count=1 [-covermode=atomic -coverprofile=<path>] ./...` (equivalently `GOFLAGS=-race`; the hook `race-guard` denies any `go test` / `gotestsum` / `richgo` line without `-race` and accepts `-race=false` only as a denial). Prerequisites: `CGO_ENABLED=1` and a C compiler (race detector uses cgo); supported on linux/{amd64,arm64,ppc64le,s390x,loong64,riscv64 (1.26)}, darwin/{amd64,arm64}, windows/amd64, freebsd/netbsd amd64. Tune with `GORACE="halt_on_error=1 log_path=..."`.

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `GO-CONC-01` | Concurrency model: goroutines multiplexed on OS threads (GMP); `GOMAXPROCS` defaults to the cgroup CPU limit since 1.25 and updates dynamically, so drop `uber-go/automaxprocs`; no GIL. [CONC-01] | Correct sizing in containers without a library. | SHOULD | [1.25 notes](https://go.dev/doc/go1.25) | missing | review |
| `GO-CONC-02` | One owner per piece of mutable state; share memory by communicating, or guard with a small `sync.Mutex`/`RWMutex` that is never held across I/O; prefer `sync/atomic` typed values (`atomic.Int64`, `atomic.Pointer[T]`) at boundaries. [CONC-02] | Eliminates races by design. | MUST | [Effective Go: concurrency](https://go.dev/doc/effective_go#concurrency), [Memory model](https://go.dev/ref/mem) | covered (GB §11-12; HX §12) | review |
| `GO-CONC-03` | Every goroutine has an owner that waits for or cancels it: use `errgroup.WithContext` (with `SetLimit`) or `sync.WaitGroup` (1.25 adds `wg.Go(func(){...})`, removing the `Add`/`Done` misuse the `waitgroup` vet analyzer reports); no `go` in domain. [CONC-02, CONC-06] | Goroutine leaks and lost errors. | MUST | [errgroup](https://pkg.go.dev/golang.org/x/sync/errgroup), [1.25 notes](https://go.dev/doc/go1.25) | covered (GB §11; arch `go` rule) | arch |
| `GO-CONC-04` | Run unit tests with the race detector in every run, local and CI: `go test -race -count=1 ./...`. Never set `CGO_ENABLED=0` for tests. [CONC-03, CONC-04] | The only built-in data-race detector; finds races only on executed paths. | MUST | [Race detector](https://go.dev/doc/articles/race_detector) | covered (GB §2, §12, §30; WF §6; gate; hook `race-guard`) | gate |
| `GO-CONC-05` | Expect 5-10x memory and 2-20x time overhead under `-race`; give the gate generous timeouts, run `-race` packages in parallel (`-p`), and for long-lived services also run a race-built binary under load (`go build -race`). [CONC-03] | Avoids flaky timeouts and covers integration paths. | SHOULD | [Race detector](https://go.dev/doc/articles/race_detector) | covered | gate |
| `GO-CONC-06` | Detect goroutine leaks in tests with `go.uber.org/goleak` (`goleak.VerifyTestMain`), and in production with the `goroutineleak` pprof profile (GA in 1.27; `/debug/pprof/goroutineleak`). [CONC-03] | Leaks are not data races; neither tool covers the other. | SHOULD | [goleak](https://pkg.go.dev/go.uber.org/goleak), [1.27 notes](https://go.dev/doc/go1.27) | missing | review |
| `GO-CONC-07` | Pass `ctx` to every blocking call; select on `ctx.Done()` when sending/receiving on channels; bound channels and worker pools (backpressure); close channels only from the sender. [CONC-05] | Unbounded fan-out and send-on-closed panics. | MUST | [Pipelines and cancellation](https://go.dev/blog/pipelines), GB §10-11 (loops spec) | covered (GB §10, §11) | review |
| `GO-CONC-08` | Never copy locks, never `time.Sleep` for synchronisation, never call `wg.Add` inside the goroutine, never share a loop variable pre-1.22 semantic assumptions; start goroutines only where cancellation is wired. [CONC-05] | Classic race sources. | MUST | [vet copylocks / waitgroup](https://pkg.go.dev/cmd/vet), [1.25 notes](https://go.dev/doc/go1.25) | covered | gate |
| `GO-CONC-09` | Graceful shutdown: `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)` -> stop accepting -> `http.Server.Shutdown(ctx)` with a deadline -> drain workers -> close pools; see OPS-04. [CONC-06] | In-flight work completes; Kubernetes sends SIGTERM. | MUST | [os/signal.NotifyContext](https://pkg.go.dev/os/signal#NotifyContext), [Server.Shutdown](https://pkg.go.dev/net/http#Server.Shutdown) | missing (GB §11 mentions owner/cancel only) | review |
| `GO-CONC-10` | Test time-dependent concurrent code with `testing/synctest` (GA in 1.25: `synctest.Test`, `synctest.Wait`, fake clock inside the bubble; 1.27 adds `synctest.Sleep`); injected `Clock` stays the rule for authoritative code. [CONC-03, TEST-09] | Deterministic, instant tests; but `sync.Mutex` and real I/O are not durably blocking in a bubble. | SHOULD | [synctest blog](https://go.dev/blog/synctest), [pkg](https://pkg.go.dev/testing/synctest), [1.25 notes](https://go.dev/doc/go1.25) | missing (GB §13, §20 say inject a Clock) | review |

## 6. TEST: testing

**Coverage recipe:** tool = `go test` itself; format = Go text profile (`mode: atomic`, lines `file.go:startLine.startCol,endLine.endCol numStmts count`), statement granularity; argv = `go test -race -count=1 -covermode=atomic -coverprofile=<path> ./...` (the default mode is `set`, `atomic` when `-race`; `-coverpkg=./...` is needed to credit a package for tests in other packages); coverage of new code = loops itself intersects the profile with `git diff <base>` changed `.go` lines (`internal/application/gate/coverage.go`, ≥70%, excluding `cmd/**`, `internal/compose/**`, `**/testdata/**`, `**/*.pb.go`, `**/*_gen.go`, `**/zz_generated*` and files carrying `// Code generated ... DO NOT EDIT.`). Integration binaries: `go build -cover` + `GOCOVERDIR=dir` + `go tool covdata textfmt -i=dir -o profile.txt`.

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `GO-TEST-01` | Use the `testing` package; add `testify/require` only if the project accepts the dependency (stdlib `if got != want { t.Errorf(...) }` with `go-cmp` for diffs is the default); integration/e2e through `httptest`, Testcontainers, or a compose stack. [TEST-01] | Standard-library-first; fewer deps. | SHOULD | [testing](https://pkg.go.dev/testing), [GSG: test](https://google.github.io/styleguide/go/decisions#tests) | covered (GB §30, §48; skills/testing) | review |
| `GO-TEST-02` | Tests live beside the code in `x_test.go`; name `TestType_Behaviour`; external test packages `package x_test` for API-level tests; fixtures in `testdata/` (ignored by `./...`); recognise tests by `**/*_test.go`. [TEST-02] | Go toolchain conventions. | MUST | [go test](https://pkg.go.dev/cmd/go#hdr-Test_packages) | covered (GB §30) | gate |
| `GO-TEST-03` | Table-driven tests with named subtests (`t.Run(tc.name, ...)`) when cases share structure; use `t.Helper()`, `t.Cleanup`, `t.TempDir()`, `t.Setenv`; per-iteration loop variables (1.22+) make `tc := tc` unnecessary. [TEST-03] | Idiomatic; sharper failures. | SHOULD | [Table-driven tests](https://go.dev/wiki/TableDrivenTests), [loopvar](https://go.dev/blog/loopvar-preview) | covered (GB §30) | review |
| `GO-TEST-04` | Use `t.Context()` (1.24: cancelled just before `Cleanup` functions run) for goroutines and calls a test starts, instead of `context.Background()`. [TEST-03, TEST-09] | Goroutines end with the test. | SHOULD | [1.24 notes](https://go.dev/doc/go1.24), [testing.T.Context](https://pkg.go.dev/testing#T.Context) | missing | review |
| `GO-TEST-05` | Doubles: hand-written fakes and stubs for consumer-owned ports (in `_test.go` or a `<pkg>test` package); generate mocks (`mockery`, `gomock`) only when a call-sequence assertion is the point; do not mock what you own at the domain level; prefer a fake in-memory repository. [TEST-04] | Fakes survive refactors. | SHOULD | [GSG: test doubles](https://google.github.io/styleguide/go/best-practices#test-doubles-and-helpers) | covered | review |
| `GO-TEST-06` | Coverage: `go test -race -count=1 -covermode=atomic -coverprofile=cover.out ./...`; add `-coverpkg=./...` to credit cross-package tests; inspect with `go tool cover -func`; exclude generated code by header; gate new code at ≥70% by statement. [TEST-06] | Hard rule in ROLES/WF §6. | MUST | [cmd/go testflags](https://pkg.go.dev/cmd/go), [Coverage for Go](https://go.dev/doc/build-cover) | covered (WF §6, GB §30; gate `test`+`coverage`, `-coverpkg` not passed) | gate |
| `GO-TEST-07` | New-code coverage is computed by loops from the Go profile against `git diff`; `diff-cover` does not read Go profiles (convert with `gocover-cobertura` if you must); keep `-coverpkg` consistent between runs. [TEST-07] | Only loops has the Go profile parser. | MUST | [Coverage for Go](https://go.dev/doc/build-cover), repo `internal/domain/coverage` | covered (gate coverage check) | gate |
| `GO-TEST-08` | Integration coverage: build with `go build -cover`, run with `GOCOVERDIR`, convert with `go tool covdata textfmt`; unrecovered panics lose the data; use `-coverpkg=<module>` (not `main`). [TEST-06] | Coverage of binaries, not just `go test`. | SHOULD | [Coverage for Go](https://go.dev/doc/build-cover) | missing | review |
| `GO-TEST-09` | Fuzz parsers, decoders, validators, money math, DTO round-trips with native fuzzing (`func FuzzX(f *testing.F)`, `go test -fuzz=FuzzX -fuzztime=30s ./pkg`); commit the minimised failing inputs under `testdata/fuzz/`. Property tests: `testing/quick` (frozen) or `pgregory.net/rapid`. [TEST-08] | Native fuzzing needs no dependency. | SHOULD | [Fuzzing](https://go.dev/doc/security/fuzz/), [rapid](https://github.com/flyingmutant/rapid) | covered (GB §30 fuzz; skills/testing) | review |
| `GO-TEST-10` | Run tests with `-shuffle=on` (print the seed) and without cache (`-count=1`); tests must not depend on order; `t.Parallel()` only for tests with no shared mutable state. [TEST-09] | Surfaces order-dependent tests and flakes. | SHOULD | [cmd/go -shuffle](https://pkg.go.dev/cmd/go) (`go help testflag`) | missing (gate uses `-count=1` only) | gate |
| `GO-TEST-11` | Inject the clock and RNG (`math/rand/v2` `rand.New(rand.NewPCG(s1, s2))`; `testing/cryptotest.SetGlobalRandom` for crypto randomness, 1.26); no `time.Sleep` for sync; use `synctest` or channels. [TEST-09] | Determinism. | MUST | [math/rand/v2](https://pkg.go.dev/math/rand/v2), [1.26 notes](https://go.dev/doc/go1.26) | covered (GB §13, §20, §21, §30) | arch |
| `GO-TEST-12` | Database/HTTP integration tests use real dependencies: `testcontainers-go` for Postgres/Kafka, `httptest.NewServer`/`NewRecorder` for handlers, transaction-per-test or schema-per-test isolation; gate behind a build tag (`//go:build integration`) or `-short`, run in CI with `-tags=integration`. [TEST-05] | In-memory SQL fakes drift from the real engine. | SHOULD | [testcontainers-go](https://golang.testcontainers.org/), [httptest](https://pkg.go.dev/net/http/httptest) | missing (no integration run in gate) | gate |
| `GO-TEST-13` | Bug fixes start with a failing test; new code ships with unit tests; tests assert behaviour not implementation; avoid asserting on log text. [TEST-03] | Regression safety. | MUST | [GSG: test](https://google.github.io/styleguide/go/decisions#tests) | covered (GB §30; ROLES) | review |
| `GO-TEST-14` | Fast loop: `go test -run 'TestName' ./pkg`, `-failfast`, `-skip`, `-short`; `gotestsum --watch`; `go test -json` for tooling; 1.26 `-artifacts` + `t.ArtifactDir()` stores test outputs. [TEST-10] | Quick TDD cycle. | SHOULD | [cmd/go testflags](https://pkg.go.dev/cmd/go), [1.26 notes](https://go.dev/doc/go1.26) | covered | review |
| `GO-TEST-15` | Mutation testing (`go-mutesting`, `gremlins`) is optional and nightly only. [TEST-08] | Costly. | AVOID | [gremlins](https://github.com/go-gremlins/gremlins) | missing | review |

## 7. PERF: performance and profiling

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `GO-PERF-01` | Benchmarks use `for b.Loop() { ... }` (1.24; keeps results alive, excludes setup before the loop and cleanup after, no manual `ResetTimer`, runs the function once per `-count`; in 1.26 it no longer blocks inlining) and `b.ReportAllocs()`; compare with `benchstat old.txt new.txt` after `-count=10`. [PERF-01] | `for i := 0; i < b.N; i++` needs manual dead-code guards. | SHOULD | [testing.B.Loop](https://pkg.go.dev/testing#B.Loop), [benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat), [1.26 notes](https://go.dev/doc/go1.26) | covered | review |
| `GO-PERF-02` | Profile before optimising: `go test -cpuprofile cpu.out -memprofile mem.out -bench .`, `go tool pprof -http=:0`, `net/http/pprof` behind an internal port, `runtime/trace`; Go 1.25 flight recorder keeps a rolling trace window; Parca/Pyroscope for continuous profiling. [PERF-02] | Evidence-based tuning. | MUST | [Diagnostics](https://go.dev/doc/diagnostics), [Flight recorder](https://go.dev/blog/flight-recorder) | covered (GB §31, §53; skills/performance) | review |
| `GO-PERF-03` | Memory/GC: Green Tea GC is the default in 1.26 (10-40% less GC overhead); set `GOMEMLIMIT` (below the container limit) with `GOGC` as the knob; small-object (<80 B) allocation is up to 30% cheaper in 1.27 (about 1% overall). Do not hand-tune before measuring. [PERF-03] | Defaults changed; old advice (GOGC=off) is stale. | SHOULD | [GC guide](https://go.dev/doc/gc-guide), [Green Tea](https://go.dev/blog/greenteagc), [1.26 notes](https://go.dev/doc/go1.26) | missing | review |
| `GO-PERF-04` | Use PGO for hot services: collect a representative CPU profile, commit it as `default.pgo` in the main package directory; `go build` picks it up (`-pgo=auto` is the default); expect 2-14% gains and refresh it per release. [PERF-03] | Free inlining/devirtualisation wins. | SHOULD | [PGO](https://go.dev/doc/pgo) | missing | review |
| `GO-PERF-05` | Hot-path mistakes: `fmt.Sprintf` in loops, `defer` in a tight loop body, `[]byte`<->`string` copies, boxing into `interface{}`, unpreallocated slices/maps, `regexp.MustCompile` inside functions, reflection-based JSON on hot paths, global lock contention, `time.After` in a loop (use `time.NewTimer`+`Reset`). [PERF-04] | Common allocation sources. | SHOULD | [Uber: performance](https://github.com/uber-go/guide/blob/master/style.md#performance), GB §22 (loops spec) | covered (GB §22, §39) | review |
| `GO-PERF-06` | Escape analysis: `go build -gcflags='-m=2'`; layout: `fieldalignment` analyzer; avoid pointer-heavy graphs in hot data. [PERF-04] | Confirms allocations. | SHOULD | [cmd/compile](https://pkg.go.dev/cmd/compile), [fieldalignment](https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/fieldalignment) | covered (GB §15, §31) | review |
| `GO-PERF-07` | Startup and AOT: Go compiles to native code (no JIT warm-up); shrink binaries with `-ldflags="-s -w"` and `-trimpath`; do not use UPX in production. [PERF-03] | Fast starts by default. | SHOULD | [cmd/link](https://pkg.go.dev/cmd/link) | missing | review |

## 8. SEC: security

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `GO-SEC-01` | SQL: parameterised queries only (`db.QueryContext(ctx, "... WHERE id = $1", id)`), never `fmt.Sprintf` into SQL; identifiers via allow-list; sqlc/pgx give typed params. Commands: `exec.CommandContext(ctx, name, args...)` with no shell and no user-controlled name; templates: `html/template` (contextual autoescape), never `text/template` for HTML. [SEC-01] | The only injection defences that hold. | MUST | [OWASP SQL Injection Prevention](https://cheatsheetseries.owasp.org/cheatsheets/SQL_Injection_Prevention_Cheat_Sheet.html), [OWASP Go-SCP](https://github.com/OWASP/Go-SCP) | covered (GB §38 parameterised queries) | gate |
| `GO-SEC-02` | Parsing: cap sizes with `http.MaxBytesReader`, `io.LimitReader`, `json.Decoder.DisallowUnknownFields`; use a typed struct not `map[string]any`; `encoding/gob` and `yaml` of untrusted data are risky; XML: leave external entity handling off (Go's `encoding/xml` has no DTD resolution) but still cap depth/size; JSON v2 (std in 1.27) rejects invalid UTF-8 and duplicate names by default. [SEC-02] | Resource exhaustion and ambiguity. | MUST | [1.27 notes](https://go.dev/doc/go1.27), GB §28, §38 (loops spec) | covered (GB §28, §38 bounded input) | review |
| `GO-SEC-03` | Path traversal: open user-named files through `os.Root` (`os.OpenRoot(dir)`, `root.Open(name)`, `os.OpenInRoot`) (1.24), not string `filepath.Join` + prefix checks; `filepath.IsLocal` for pure validation; since Go 1.25 `Root` also has `Symlink`, `Rename`, `Link`, `Readlink`, `Chmod`, `MkdirAll`, `RemoveAll`, `ReadFile` and `WriteFile`. [SEC-03] | `os.Root` is symlink/`..`-escape-resistant. | MUST | [os.Root blog](https://go.dev/blog/osroot) (1.24), [os.Root API](https://pkg.go.dev/os#Root), [Go-SCP](https://github.com/OWASP/Go-SCP) | covered | review |
| `GO-SEC-04` | SSRF/outbound: validate URLs against an allow-list, resolve and block private/link-local ranges in a `net.Dialer.Control` hook, set timeouts; use `net.JoinHostPort` (vet `hostport`), not `Sprintf("%s:%d")`. Open redirects: allow-list targets. CSRF: SameSite cookies + token for browser clients; CORS: explicit origins. [SEC-03] | Standard web risks. | MUST | [OWASP SSRF Prevention](https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html), [1.25 notes](https://go.dev/doc/go1.25) | missing | review |
| `GO-SEC-05` | Servers set `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`, `MaxHeaderBytes`; never use `http.ListenAndServe` bare in production (gosec G112). [SEC-03] | Slowloris and exhaustion. | MUST | [net/http Server](https://pkg.go.dev/net/http#Server), [gosec rules](https://github.com/securego/gosec) | missing | gate |
| `GO-SEC-06` | Secrets: load from the environment/secret manager at the edge in `cmd`/`platform`; never log or commit them; keep them in typed config with a `String()` that redacts. [SEC-04] | OWASP secrets management. | MUST | [OWASP Secrets Management](https://cheatsheetseries.owasp.org/cheatsheets/Secrets_Management_Cheat_Sheet.html), [12-factor config](https://12factor.net/config) | covered (GB §28, §38; HX §19) | review |
| `GO-SEC-07` | Crypto: `crypto/rand` (and `rand.Text()` since 1.24) for tokens, `math/rand/v2` only for non-security randomness; passwords with `golang.org/x/crypto/argon2` (id) or `bcrypt`; `crypto/*` std only, no MD5/SHA-1/DES/RC4, `crypto/subtle.ConstantTimeCompare`; TLS 1.2+ with std defaults (hybrid ML-KEM on by default since 1.24/1.26); FIPS via `GOFIPS140`. [SEC-04] | Misused crypto is the usual vulnerability. | MUST | [OWASP Password Storage](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html), [argon2](https://pkg.go.dev/golang.org/x/crypto/argon2), [FIPS 140-3](https://go.dev/doc/security/fips140) | covered | review |
| `GO-SEC-08` | Validate at the inbound adapter with an explicit function or `go-playground/validator` struct tags; produce typed commands; domain trusts invariants. [SEC-05] | One validation point. | MUST | [validator v10](https://pkg.go.dev/github.com/go-playground/validator/v10), [OWASP Input Validation](https://cheatsheetseries.owasp.org/cheatsheets/Input_Validation_Cheat_Sheet.html) | covered (GB §44; HX §7, §10) | review |
| `GO-SEC-09` | Supply chain: keep `GOFLAGS=-mod=readonly` (default), verify with `go mod verify`, rely on the checksum DB (`GONOSUMDB`/`GOPRIVATE` only for private modules), pin GitHub Actions by SHA, and prefer module proxy mirrors for private deps; the Go design mitigates typosquatting by immutable versions. [SEC-06] | Go has no install scripts, but dependency swaps happen. | MUST | [How Go mitigates supply chain attacks](https://go.dev/blog/supply-chain), [GoMod](https://go.dev/ref/mod#authenticating) | covered | gate |
| `GO-SEC-10` | SAST: run `gosec -exclude-generated ./...` (exit 1 on findings; add `-fmt sarif -out gosec.sarif` for code scanning) or enable the `gosec` linter in golangci-lint; add Semgrep Go rules if wanted. [SEC-07] | Catches weak crypto, bare listeners, SQL concatenation, file perms. | SHOULD | [gosec](https://github.com/securego/gosec) | missing | gate |
| `GO-SEC-11` | Authn/z at the inbound adapter; pass an authenticated principal inward as data; never trust headers like `X-Forwarded-For` without a trusted proxy list. [SEC-03] | Keeps auth out of the domain. | MUST | [OWASP Go-SCP](https://github.com/OWASP/Go-SCP) | covered (GB §38) | review |

## 9. DATA: persistence

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `GO-DATA-01` | Default to SQL-first: `sqlc` (SQL in `db/queries/*.sql` -> typed Go) with `pgx/v5` for Postgres; a query builder (`squirrel`, `goqu`) for dynamic filters; GORM/ent only where the team accepts their magic. Keep generated code in the outbound adapter. [DATA-01] | Typed, reviewable SQL; no runtime reflection cost. | SHOULD | [sqlc](https://docs.sqlc.dev/en/latest/), [pgx](https://github.com/jackc/pgx) | missing (HX §18 persistence boundary only) | review |
| `GO-DATA-02` | Migrations are versioned SQL files, forward-only in prod, run by `goose`, `golang-migrate` or Atlas as a separate step (job/init container), never from `init()` or on every pod start; follow expand -> backfill -> switch -> contract. [DATA-02] | Zero-downtime deploys. | MUST | [goose](https://github.com/pressly/goose), [golang-migrate](https://github.com/golang-migrate/migrate), [Atlas](https://atlasgo.io/) | covered | review |
| `GO-DATA-03` | Transaction boundary belongs to the application use case through a port (`UnitOfWork.Do(ctx, func(ctx, tx) error)`); adapters never leak `*sql.Tx` or `pgx.Tx` into the domain; `defer tx.Rollback()`; pick the isolation level explicitly for money-moving code. [DATA-03] | Atomic commands. | MUST | [database/sql](https://pkg.go.dev/database/sql), HX §18 (loops spec) | covered (HX §10, §18; GB §44) | review |
| `GO-DATA-04` | Connection pool: `pgxpool` (or `db.SetMaxOpenConns/SetMaxIdleConns/SetConnMaxLifetime/SetConnMaxIdleTime`) sized from the DB limit; always use `*Context` methods with a deadline; close `rows` and check `rows.Err()`. [DATA-05] | Pool exhaustion is the top outage cause. | MUST | [pgxpool](https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool), [DB.SetMaxOpenConns](https://pkg.go.dev/database/sql#DB.SetMaxOpenConns) | missing | review |
| `GO-DATA-05` | N+1: batch with `WHERE id = ANY($1)`, joins, or sqlc `:many`; with GORM use `Preload`/`Joins`; log slow queries. [DATA-04] | ORM loops over a collection. | SHOULD | [sqlc](https://docs.sqlc.dev/en/latest/) | missing | review |
| `GO-DATA-06` | Map DB rows to domain types in the adapter; persisted DTOs are separate from domain types; use `sql.Null*`/`pgtype` or pointers for nullable columns, converted at the boundary; always `ORDER BY` with a unique tie-break. [DATA-01] | Isolation and deterministic ordering. | MUST | [database/sql nulls](https://pkg.go.dev/database/sql#NullString) | covered (GB §29, §34, §44; HX §18) | review |
| `GO-DATA-07` | Optimistic concurrency with a `version` column and `UPDATE ... WHERE version = $n` (check rows affected); pessimistic with `SELECT ... FOR UPDATE SKIP LOCKED` for queues. [DATA-03] | Lost updates. | SHOULD | [pgx](https://github.com/jackc/pgx) | covered | review |
| `GO-DATA-08` | Run `sqlc vet`/`sqlc diff` and migration-lint (`atlas migrate lint`, `squawk`) in CI so generated code and migrations cannot drift. [DATA-02] | Generated code must match SQL. | SHOULD | [sqlc CLI](https://docs.sqlc.dev/en/latest/reference/cli.html), [Atlas lint](https://atlasgo.io/versioned/lint) | missing | gate |
| `GO-DATA-09` | Redis/caches live behind a port; document invalidation and revision (GB §36); use `context` deadlines on every call. [DATA-05] | Cache coherence. | SHOULD | [go-redis](https://redis.uptrace.dev/) | covered (GB §36) | review |

## 10. API: APIs and integration

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `GO-API-01` | HTTP: use the stdlib `net/http` with the Go 1.22 `ServeMux` (`mux.HandleFunc("GET /orders/{id}", h)`, `r.PathValue("id")`, `{$}`, `{rest...}`; most-specific pattern wins, conflicts panic at registration; since 1.26 trailing-slash redirect is 307); add chi (stdlib-compatible) when you need route groups/middleware ergonomics; gin/echo/Fiber only if the team already uses them. [API-01] | Stdlib router now covers methods and wildcards. | SHOULD | [Routing enhancements](https://go.dev/blog/routing-enhancements), [ServeMux](https://pkg.go.dev/net/http#ServeMux), [1.26 notes](https://go.dev/doc/go1.26) | missing | review |
| `GO-API-02` | Middleware is `func(http.Handler) http.Handler`: order = recover -> request-id -> logging/tracing -> auth -> limits; handlers decode, call one use case, encode; they hold no business logic. [API-01] | Thin inbound adapter. | MUST | [net/http](https://pkg.go.dev/net/http), HX §7 (loops spec) | covered (GB §25, §44; HX §7, §10) | review |
| `GO-API-03` | Contracts: OpenAPI-first with `oapi-codegen` (server interface + types) or protobuf-first with `buf` + Connect/gRPC; run `buf lint`/`buf breaking` in CI; never hand-edit generated code. [API-02] | A single contract source and breaking-change detection. | SHOULD | [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen), [Connect Go](https://connectrpc.com/docs/go/getting-started), [buf](https://buf.build/docs/), [grpc-go](https://grpc.io/docs/languages/go/) | covered | gate |
| `GO-API-04` | Errors at the edge: map domain results/validation/infra errors to status codes in one place; return RFC 9457 `application/problem+json`; never leak internals; gRPC/Connect codes via a single mapper. [API-03] | Uniform clients. | MUST | [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) | covered (HX §13; GB §8) | review |
| `GO-API-05` | Version APIs in the path (`/v1/`) or proto package (`acme.orders.v1`); additive changes only inside a version; keep deprecated fields. [API-02] | Compatibility. | SHOULD | [Connect versioning](https://connectrpc.com/docs/go/getting-started), GB §45 (loops spec) | covered (GB §45) | review |
| `GO-API-06` | Resilience: every outbound call has a context deadline and an `http.Client{Timeout}`; retry only idempotent calls with exponential backoff + jitter (`cenkalti/backoff`); circuit breaker (`sony/gobreaker`) in the adapter; rate-limit with `golang.org/x/time/rate`; idempotency keys for POST. [API-04] | Prevents cascading failure. | MUST | [x/time/rate](https://pkg.go.dev/golang.org/x/time/rate), [backoff](https://github.com/cenkalti/backoff), [gobreaker](https://github.com/sony/gobreaker) | covered | review |
| `GO-API-07` | HTTP client: one shared `*http.Client` (never `http.DefaultClient`, no timeout), custom `Transport` with `MaxIdleConnsPerHost`, `defer resp.Body.Close()` and drain the body, propagate context with `http.NewRequestWithContext`, wrap with `otelhttp.NewTransport`. [API-06] | Connection reuse and leak avoidance. | MUST | [net/http Client](https://pkg.go.dev/net/http#Client), [otelhttp](https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp) | covered | gate |
| `GO-API-08` | Messaging: `segmentio/kafka-go` or `twmb/franz-go` for Kafka, `nats.go` for NATS, `amqp091-go` for RabbitMQ, AWS SDK v2 for SQS; handlers are idempotent (at-least-once), commit offsets after processing, and send via the outbox. [API-05] | Redelivery is normal. | MUST | [franz-go](https://github.com/twmb/franz-go), [kafka-go](https://github.com/segmentio/kafka-go), [NATS](https://nats.io/) | covered | review |
| `GO-API-09` | gRPC/Connect servers: set deadlines and max message sizes, use interceptors for auth/logging/tracing, implement health (`grpc.health.v1`) and reflection only in non-prod. [API-01] | Production safety. | SHOULD | [gRPC Go](https://grpc.io/docs/languages/go/), [Connect](https://connectrpc.com/docs/go/getting-started) | missing | review |
| `GO-API-10` | JSON: use `encoding/json` (v1 API, backed by v2 in 1.27 unless `GOEXPERIMENT=nojsonv2`); adopt `encoding/json/v2` for new code only after reading the behaviour diffs (stricter UTF-8/dup keys, `omitzero`), and set `DisallowUnknownFields` at the edge. [API-03] | v2 is stable-ish in std but its option surface is new. | SHOULD | [1.27 notes](https://go.dev/doc/go1.27), [1.25 notes](https://go.dev/doc/go1.25) | missing | review |

## 11. OBS: observability

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `GO-OBS-01` | Log with `log/slog` (JSON handler in prod): typed attrs via `LogAttrs`/`slog.String`, the `*Context` methods so handlers can add trace ids, a logger passed in or taken from `ctx`, `slog.NewMultiHandler` (1.26) to fan out; lint with `sloglint`; no secrets or PII; one correlation id per request. [OBS-01] | Std structured logging since 1.21. | MUST | [slog](https://go.dev/blog/slog), [pkg slog](https://pkg.go.dev/log/slog), [OWASP Logging](https://cheatsheetseries.owasp.org/cheatsheets/Logging_Cheat_Sheet.html), [sloglint](https://github.com/go-simpler/sloglint) | covered (GB §24) | gate |
| `GO-OBS-02` | Metrics: OpenTelemetry metrics API or `prometheus/client_golang`; names `snake_case` with unit suffixes (`_seconds`, `_bytes`, `_total`), bounded label cardinality (no user ids); RED metrics per endpoint. [OBS-02] | Prometheus conventions; label explosions kill TSDBs. | SHOULD | [Prometheus client_golang](https://pkg.go.dev/github.com/prometheus/client_golang/prometheus), [naming](https://prometheus.io/docs/practices/naming/) | covered | review |
| `GO-OBS-03` | Tracing: OpenTelemetry Go SDK (`go.opentelemetry.io/otel`); traces and metrics are stable, logs are beta; initialise in `platform`, export OTLP, wrap servers/clients with `otelhttp`/`otelgrpc`, propagate W3C `traceparent` through `ctx`; spans only in adapters and use-case entry points. [OBS-03] | Context propagation is Go's `ctx`. | SHOULD | [OTel Go](https://opentelemetry.io/docs/languages/go/), [instrumentation](https://opentelemetry.io/docs/languages/go/instrumentation/) | missing | review |
| `GO-OBS-04` | Health: `/livez` (process up, no deps) and `/readyz` (deps ready, fails during shutdown); expose build info (`debug.ReadBuildInfo`, `-ldflags -X`); no business logic. [OBS-04] | Kubernetes probes. | MUST | [K8s probes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/) | covered | review |
| `GO-OBS-05` | Observability must not change behaviour: logs/metrics/traces never feed back into decisions. [OBS-01] | Determinism. | MUST | [slog](https://pkg.go.dev/log/slog), GB §24, §47 | covered (GB §24, §47) | review |
| `GO-OBS-06` | Expose `pprof` and `/metrics` on a separate, internal-only listener. [OBS-02] | Info leak. | MUST | [net/http/pprof](https://pkg.go.dev/net/http/pprof) | missing | review |
| `GO-OBS-07` | Add `slog.Handler` wrapper that injects `trace_id`/`span_id` from `ctx` (or use the OTel slog bridge `otelslog`). [OBS-03] | Log-trace correlation. | SHOULD | [otelslog](https://pkg.go.dev/go.opentelemetry.io/contrib/bridges/otelslog) | missing | review |

## 12. OPS: configuration, packaging and deployment

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `GO-OPS-01` | Config: 12-factor env vars, parsed once in `cmd`/`platform` into a typed struct (`caarlos0/env`, `sethvargo/go-envconfig`, or `koanf`), validated at startup with all errors listed; provide `-check`. [OPS-01] | Fail fast. | MUST | [12-factor config](https://12factor.net/config), [go-envconfig](https://pkg.go.dev/github.com/sethvargo/go-envconfig) | covered (GB §28; HX §19) | review |
| `GO-OPS-02` | Package as one static binary: `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=$V" -o bin/app ./cmd/app`; container: multi-stage Dockerfile (`golang:<ver>` builder with `--mount=type=cache` for `GOCACHE` and `GOMODCACHE`, final `gcr.io/distroless/static` or `scratch`, non-root user, CA certs); or `ko build`; use `goreleaser` for binaries. [OPS-02] | Tiny, reproducible images. | SHOULD | [distroless](https://github.com/GoogleContainerTools/distroless), [ko](https://ko.build/), [goreleaser](https://goreleaser.com/) | missing | review |
| `GO-OPS-03` | If the binary needs cgo (race build, sqlite), use a `-race`/test-only image; the production image stays `CGO_ENABLED=0`. [OPS-02] | Race detector needs cgo. | SHOULD | [Race detector](https://go.dev/doc/articles/race_detector) | missing | review |
| `GO-OPS-04` | Lifecycle: `signal.NotifyContext` for SIGINT/SIGTERM, `Server.Shutdown(ctx)` with a timeout below `terminationGracePeriodSeconds`, readiness flips first, workers drain, then exit 0. [OPS-04] | Zero-dropped-request rollouts. | MUST | [Server.Shutdown](https://pkg.go.dev/net/http#Server.Shutdown) | missing | review |
| `GO-OPS-05` | Run-scoped caches for loops: point `GOCACHE` and `GOTMPDIR` into the run directory (loops does this through hook `go-env` and `loops go`); leave `GOMODCACHE` and `GOPATH` shared so nothing is re-downloaded; tool caches such as `GOLANGCI_LINT_CACHE` may be redirected the same way. [OPS-05] | Parallel agents do not poison each other's build cache. | MUST | [go env](https://pkg.go.dev/cmd/go#hdr-Print_Go_environment_information), [golangci-lint cache](https://golangci-lint.run/docs/configuration/cli/) | covered (WF §1/§2.2, hook `go-env`; `GOMODCACHE` deliberately shared) | hook |
| `GO-OPS-06` | Artifacts a Go build/test leaves behind: `*.test`, `*.test.exe`, `*.out`, `*.prof`, `*.pprof`, `*.cov`, `*.exe`, `coverage*.txt`, `cover*.out`, `__debug_bin*`, plus any `-o` build outputs; cleaned by loops cleanup. [OPS-06] | Keeps the tree and commits clean. | SHOULD | [cmd/go](https://pkg.go.dev/cmd/go) | covered (`workspace.IsArtifactName`) | hook |
| `GO-OPS-07` | Cross-compile with `GOOS/GOARCH` and `CGO_ENABLED=0`; build reproducibly (`-trimpath`, pinned toolchain, `-buildvcs`) and embed version via `runtime/debug.ReadBuildInfo`. [OPS-02] | Reproducibility. | SHOULD | [Reproducible builds](https://go.dev/blog/rebuild), [cmd/go -trimpath](https://pkg.go.dev/cmd/go) | missing | review |
| `GO-OPS-08` | Set `GOMEMLIMIT` and CPU requests/limits in the manifest; Go 1.25 reads the cgroup CPU limit for `GOMAXPROCS`. [OPS-02] | Prevents OOM kills and throttling. | SHOULD | [GC guide](https://go.dev/doc/gc-guide), [1.25 notes](https://go.dev/doc/go1.25) | missing | review |
| `GO-OPS-09` | Scan the image (`trivy`/`grype`), sign with `cosign`, generate provenance in CI. [OPS-02] | Supply chain. | SHOULD | [cosign](https://github.com/sigstore/cosign) | missing | review |
| `GO-OPS-10` | Native/AOT: n/a, Go already produces native binaries; ship the plain static binary instead of WASM/`GOOS=wasip1`. [OPS-03] | No runtime to ahead-of-time compile. | SHOULD | [Go on WASI](https://go.dev/blog/wasi) | n/a | review |
| `GO-OPS-11` | When running `go test -race` in Docker/CI use an image with a C compiler (`golang:<ver>` has gcc; `alpine` needs `gcc musl-dev`). [OPS-02] | `-race` silently unavailable otherwise. | SHOULD | [Race detector](https://go.dev/doc/articles/race_detector) | missing | review |

## 13. DOC: documentation and review

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `GO-DOC-01` | Doc comments per `go doc` rules: package comment, exported names start with the name, `// Deprecated:`, runnable `Example` functions; browse with `go doc pkg@version` (1.27); publish through pkg.go.dev for libraries. [DOC-01] | Examples are compiled and tested. | SHOULD | [Go Doc Comments](https://go.dev/doc/comment), [1.27 notes](https://go.dev/doc/go1.27) | covered (GB §26) | gate |
| `GO-DOC-02` | Repo docs: README (what, run, test, config), ADRs under `docs/adr/` (MADR), `CHANGELOG.md` generated from conventional commits (git-cliff/goreleaser), `AGENTS.md` per package for domain language and complexity notes. [DOC-02] | Context for humans and agents. | SHOULD | [ADR](https://adr.github.io/madr/), [goreleaser changelog](https://goreleaser.com/customization/changelog/) | covered (HX §26; GB §32, §42) | review |
| `GO-DOC-03` | Reviewers check first: error wrapping and handling, context propagation, goroutine ownership, defer/Close, naming and package boundaries, test table quality, `-race` evidence, benchmark evidence, API/schema compatibility. [DOC-03] | The CRC list plus loops checklist. | SHOULD | [CRC](https://go.dev/wiki/CodeReviewComments), [GSG](https://google.github.io/styleguide/go/) | covered (GB §51, §52; skills/review-integration) | review |
| `GO-DOC-04` | Keep an `Example`/`ExampleXxx_suffix` test for each public use case; examples run under `go test`. [DOC-01] | Docs cannot rot. | SHOULD | [testing examples](https://pkg.go.dev/testing#hdr-Examples) | missing | review |
| `GO-DOC-05` | Generate API docs from the contract (OpenAPI via Redoc/Scalar, protos via buf docs), not by hand. [DOC-01] | Single source. | SHOULD | [buf docs](https://buf.build/docs/) | missing | review |

## 14. FW: frameworks

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `GO-FW-01` | Commonly used: `net/http` stdlib (+chi), gin, echo, Fiber, gorilla/mux, gRPC/Connect; frameworks are optional in Go. No authoritative 2026 usage ranking was found (the 2025 Go survey post has no framework data), so treat this list as qualitative. [FW-01] | Tooling matches usage. | SHOULD | [Go Developer Survey 2025](https://go.dev/blog/survey2025) (no framework data), [chi](https://github.com/go-chi/chi), [gin](https://github.com/gin-gonic/gin) | missing | review |
| `GO-FW-02` | chi: stdlib-compatible; routers and middleware in the inbound adapter; `chi.URLParam` or `r.PathValue`. gin/echo: handler signatures differ from `http.Handler`; wrap them so use cases never see `*gin.Context`. Connect: handlers implement generated interfaces, one adapter per service. [FW-02] | Keeps framework types in adapters. | MUST | [chi](https://github.com/go-chi/chi), [Connect](https://connectrpc.com/docs/go/getting-started) | covered (GB §44, §48; HX §7) | arch |
| `GO-FW-03` | ORM/data frameworks: GORM (`AutoMigrate` is not a migration strategy), ent (generated), sqlc, bun; project rules win over loops rules when the project already uses one. [FW-02] | Do not fight the existing stack. | SHOULD | [GORM](https://gorm.io/docs/), [ent](https://entgo.io/) | covered (WF §1 project rules win) | review |
| `GO-FW-04` | Footguns: `gin.Context` leaking into domain, GORM auto-migrate in production, global `http.DefaultServeMux` (registered by importing `net/http/pprof`), `fx`/`dig` hiding the graph, gorilla/mux method mismatches. [FW-03] | Common framework traps. | AVOID | [net/http/pprof](https://pkg.go.dev/net/http/pprof) | covered | review |
| `GO-FW-05` | Kafka/NATS/queue frameworks (Watermill, Benthos) are adapters; domain stays unaware. [FW-02] | Hexagonal. | SHOULD | [Watermill](https://watermill.io/) | missing | review |
| `GO-FW-06` | Prefer new std capabilities over third-party where equal: `slog`, `ServeMux`, `slices`/`maps`, `iter`, `encoding/json/v2`, `uuid` (1.27, import path `"uuid"`: `New`, `NewV7`, `Parse`), `os.Root`, `errors.AsType`. [FW-01] | Fewer dependencies. | SHOULD | [1.27 notes](https://go.dev/doc/go1.27), [1.26 notes](https://go.dev/doc/go1.26) | missing (GB §48 predates them) | review |

## Top 10 footguns

1. **Multi-module `./...`.** Bad: `go test ./...` at the root of a repo with nested `go.mod`s. Fix: run per module (`go work edit -json` list). ([cmd/go](https://pkg.go.dev/cmd/go))
2. **`-race` with `CGO_ENABLED=0` or no C compiler.** Bad: the race run fails or is skipped in Alpine/CI. Fix: a cgo-capable test image, keep `-race` mandatory. ([Race detector](https://go.dev/doc/articles/race_detector))
3. **Cached tests hide failures.** Bad: `go test ./...` serves cache for unchanged inputs. Fix: `-count=1` in the gate. ([cmd/go](https://pkg.go.dev/cmd/go))
4. **Closing the response body.** Bad: `resp, _ := http.Get(u)` with no `Close`. Fix: `defer resp.Body.Close()` and drain. ([bodyclose](https://github.com/timakin/bodyclose))
5. **Nil-interface-with-nil-pointer.** Bad: `func f() error { var e *MyErr; return e }` is non-nil. Fix: return `nil` explicitly. ([Go FAQ](https://go.dev/doc/faq#nil_error))
6. **Default HTTP client/server without timeouts.** Bad: `http.Get`, `http.ListenAndServe`. Fix: custom `Client{Timeout}` and `Server{ReadHeaderTimeout...}`. ([net/http](https://pkg.go.dev/net/http))
7. **Goroutine leaks.** Bad: sending on an unbuffered channel nobody reads, no `ctx.Done()` case. Fix: owner + `errgroup` + `goleak`, or 1.27 `goroutineleak` profile. ([1.27 notes](https://go.dev/doc/go1.27))
8. **Map iteration and slice aliasing.** Bad: decision on `for k := range m`, or `append` to a sub-slice overwriting the parent. Fix: sort keys; `slices.Clone` / full slice expressions. ([Slices](https://go.dev/blog/slices-intro))
9. **`time.Now()`/global rand in business code.** Bad: untestable and nondeterministic. Fix: inject `Clock`/`*rand.Rand`; `synctest` for timers. ([synctest](https://go.dev/blog/synctest))
10. **Migrations and generated code drift.** Bad: `AutoMigrate` on boot; hand-edited `*.pb.go`/sqlc output. Fix: versioned SQL migration job; regenerate in CI and `git diff --exit-code`. ([sqlc](https://docs.sqlc.dev/en/latest/))

## 15. LOOPS: tooling data

The `checks` array mirrors `internal/application/gate/gate.go` exactly (names, order, argv). `arch` and `coverage` are internal checks, not argv. `proposed_checks` are NOT run today.

```json
{
  "id": "go",
  "name": "Go",
  "versions": {"stable": "1.27", "lts": null, "as_of": "2026-11"},
  "detect": [{"file": "go.mod"}],
  "source_globs": ["**/*.go"],
  "test_globs": ["**/*_test.go"],
  "generated": {
    "globs": ["**/*.pb.go", "**/*_gen.go", "**/zz_generated*"],
    "header_regex": "^// Code generated .* DO NOT EDIT\\.$"
  },
  "checks": [
    {"name": "build", "kind": "build", "argv": ["go", "build", "./..."], "required": true, "tool": "go"},
    {"name": "gofmt", "kind": "format", "argv": ["gofmt", "-l", "."], "required": false, "tool": "gofmt", "fails_on": "output", "note": "gate filters vendor/ and testdata/ lines; config-skippable"},
    {"name": "vet", "kind": "lint", "argv": ["go", "vet", "./..."], "required": true, "tool": "go"},
    {"name": "staticcheck", "kind": "lint", "argv": ["staticcheck", "./..."], "required": false, "tool": "staticcheck", "install": "go install honnef.co/go/tools/cmd/staticcheck@latest", "note": "skipped when not on PATH or too old for the toolchain"},
    {"name": "arch", "kind": "arch", "argv": null, "required": false, "tool": "loops", "note": "internal check: `loops arch`; config-skippable"},
    {"name": "test", "kind": "test", "argv": ["go", "test", "-race", "-count=1", "-covermode=atomic", "-coverprofile={coverage}", "./..."], "required": true, "tool": "go"},
    {"name": "coverage", "kind": "coverage", "argv": null, "required": true, "tool": "loops", "note": "internal: new-code coverage >= 70% against the git base; not skippable"},
    {"name": "govulncheck", "kind": "vuln", "argv": ["govulncheck", "./..."], "required": false, "tool": "govulncheck", "install": "go install golang.org/x/vuln/cmd/govulncheck@latest", "note": "runs only when enabled and installed"}
  ],
  "proposed_checks": [
    {"proposal": true, "name": "tidy", "kind": "deps", "argv": ["go", "mod", "tidy", "-diff"], "fails_on": "exit", "tool": "go"},
    {"proposal": true, "name": "verify", "kind": "deps", "argv": ["go", "mod", "verify"], "fails_on": "exit", "tool": "go"},
    {"proposal": true, "name": "fix", "kind": "lint", "argv": ["go", "fix", "-diff", "./..."], "fails_on": "exit", "tool": "go", "note": "advisory modernizers (1.26+)"},
    {"proposal": true, "name": "golangci-lint", "kind": "lint", "argv": ["golangci-lint", "run"], "tool": "golangci-lint", "install": "see https://golangci-lint.run/docs/welcome/install/ (v2 config)"},
    {"proposal": true, "name": "gosec", "kind": "sast", "argv": ["gosec", "-exclude-generated", "./..."], "fails_on": "exit", "tool": "gosec", "install": "go install github.com/securego/gosec/v2/cmd/gosec@latest"},
    {"proposal": true, "name": "test-shuffle", "kind": "test", "argv": ["go", "test", "-race", "-count=1", "-shuffle=on", "./..."], "tool": "go"},
    {"proposal": true, "name": "test-integration", "kind": "test", "argv": ["go", "test", "-race", "-count=1", "-tags=integration", "./..."], "tool": "go"},
    {"proposal": true, "name": "govulncheck-binary", "kind": "vuln", "argv": ["govulncheck", "-mode=binary", "{binary}"], "tool": "govulncheck"}
  ],
  "multi_module": {"proposal": true, "discover": ["go", "work", "edit", "-json"], "fallback": "find . -name go.mod", "run_checks_per_module": true},
  "test_commands": [{"argv_prefix": ["go", "test"]}, {"argv_prefix": ["gotestsum"]}, {"argv_prefix": ["richgo", "test"]}],
  "race": {"mode": "flag", "flags": ["-race"], "env": {"GOFLAGS": "-race"}, "reason": "Go's built-in race detector; needs CGO_ENABLED=1 and a C compiler", "substitute": null},
  "coverage": {
    "format": "gocover",
    "path": "{coverage}",
    "granularity": "statement",
    "mode_flag": ["-covermode=atomic"],
    "profile_flag": ["-coverprofile={coverage}"],
    "default_exclude": ["cmd/**", "internal/compose/**", "**/testdata/**", "**/*.pb.go", "**/*_gen.go", "**/zz_generated*"],
    "min_new_code_percent": 70,
    "integration": {"proposal": true, "build": ["go", "build", "-cover"], "env": {"GOCOVERDIR": "{dir}"}, "convert": ["go", "tool", "covdata", "textfmt", "-i={dir}", "-o={coverage}"]}
  },
  "post_edit": {
    "format_file": ["gofmt", "-w", "{file}"],
    "format_file_preferred": ["goimports", "-w", "{file}"],
    "lint_dir": ["go", "vet", "./{dir}"]
  },
  "cache_env": {"GOCACHE": "cache", "GOTMPDIR": "tmp"},
  "env_hints": {"GOFLAGS": "-p=2"},
  "artifacts": ["*.test", "*.test.exe", "*.out", "*.prof", "*.pprof", "*.cov", "*.exe", "coverage*.txt", "cover*.out", "__debug_bin*"],
  "arch": {
    "import_regex": "^\\s*(?:import\\s+)?(?:[A-Za-z_.]+\\s+)?\"([^\"]+)\"",
    "resolver": "module-path",
    "native_tool": null,
    "forbidden_in_domain": ["os", "net/http"],
    "forbidden_in_domain_note": "stdUnder match: also subpackages (os/exec, net/http/httptest). database/sql is NOT forbidden today (proposal).",
    "third_party_forbidden_in": ["domain", "application", "pkg"],
    "impure_calls": ["time.Now", "math/rand (global funcs)", "math/rand/v2 (global funcs)", "go statement"],
    "other_errors": ["init()", "package-level mutable state", "cmd/<x>/main.go over 80 lines or string literal over 40 runes", "only cmd imports compose"],
    "size_warnings": {"type_methods": 40, "struct_fields": 25, "interface_methods": 8, "package_lines": 12000, "package_files": 60}
  },
  "test_jobs": "-p=2",
  "config_files": [".golangci.yml", ".golangci.yaml", ".golangci.toml", ".golangci.json", ".editorconfig", "staticcheck.conf", "go.mod", "go.work", "sqlc.yaml", "buf.yaml"],
  "sources": {
    "checks": "https://pkg.go.dev/cmd/go",
    "race": "https://go.dev/doc/articles/race_detector",
    "coverage": "https://go.dev/doc/build-cover",
    "govulncheck": "https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck",
    "gofmt": "https://pkg.go.dev/cmd/gofmt",
    "vet": "https://pkg.go.dev/cmd/vet",
    "staticcheck": "https://staticcheck.dev/docs/",
    "go_mod_tidy_verify_fix": "https://pkg.go.dev/cmd/go",
    "golangci_lint": "https://golangci-lint.run/docs/configuration/file/",
    "gosec": "https://github.com/securego/gosec",
    "modules": "https://go.dev/ref/mod",
    "toolchains": "https://go.dev/doc/toolchain"
  }
}
```

## Proposed updates to GO_BEST_PRACTICES

Outdated, wrong or missing items in `specs/GO_BEST_PRACTICES.md`, by section. P = suggested priority.

| § | Item | Problem / update | P |
|---|---|---|---|
| §2 | Check list | Add `go mod tidy -diff`, `-shuffle=on`, golangci-lint v2 (`version: "2"`, `formatters:`) as an alternative to bare `staticcheck`, and `go tool <name>` for tools declared with the `tool` directive (1.24). Note `go vet` also compiles tests but `-tags` variants need their own run. | P1 |
| §2, §38 | Tools on PATH | "when installed" leaves the gate green without a linter or vulnerability scan. Prefer pinned `tool` directives; make `govulncheck` and a SAST step (`gosec`) part of CI. | P0 |
| §8, §9 | `errors.Is/As` | Add `errors.AsType[T]` (1.26), `errors.Join`. The example `errors.New("insufficient funds")` as "Bad" is fine, but add that sentinels remain right for programmer-visible conditions. | P2 |
| §10 | Context | Add `context.WithCancelCause`, `WithoutCancel`, `AfterFunc`, and `t.Context()` in tests. | P2 |
| §11, §12 | Concurrency | Add `errgroup.SetLimit`, `sync.WaitGroup.Go` (1.25), `atomic.Int64`/`atomic.Pointer[T]` types (1.19; 1.27 `go fix atomictypes`), goroutine leak tooling (`goleak`, 1.27 `goroutineleak` profile), per-iteration loop variables (1.22), container-aware `GOMAXPROCS` (1.25). | P1 |
| §13, §20 | Clock injection | Still the rule for authoritative code, but add `testing/synctest` (GA 1.25, `synctest.Sleep` 1.27) for goroutine/timer tests, and `testing/cryptotest.SetGlobalRandom` (1.26). | P1 |
| §14, §34 | Maps and slices | Add `slices`, `maps`, `cmp`, `clear`, `min`/`max` (1.21+) and `slices.Sorted(maps.Keys(m))` (1.23) as the way to sort keys. | P2 |
| §17 | Generics | Add generic type aliases (1.24), self-referential type parameters (1.26), generic methods (1.27) and the `iter` package; state when each is acceptable. | P1 |
| §21 | Randomness | Name `math/rand/v2` explicitly (`rand.New(rand.NewPCG(...))`) and `rand.Text` (1.24) for tokens; `crypto/*` APIs ignore the `rand` parameter since 1.26. | P2 |
| §24 | Logging | Example uses untyped key-value pairs; prefer `LogAttrs`/`slog.Attr` or `sloglint` with `attr-only`; use `*Context` methods; mention `slog.NewMultiHandler` (1.26) and `slog.DiscardHandler` for tests. No OpenTelemetry guidance (add OBS: traces, metrics, `otelhttp`, trace-id in logs). | P1 |
| §26 | Doc comments | Cite `go.dev/doc/comment`; add runnable `Example` functions and `// Deprecated:`. | P2 |
| §28 | Config | Add typed env parsing (`go-envconfig`) and `-check` is already there; add `os.Root` for file-based content loading. | P2 |
| §29 | File I/O | "Use `filepath`, normalise, reject traversal" is outdated: use `os.Root`/`os.OpenInRoot` (1.24) and `filepath.IsLocal`. | P0 |
| §30 | Testing | Add `t.Context()`, `t.Cleanup`, `t.TempDir`, `-shuffle=on`, `-coverpkg`, integration tags and Testcontainers, `testing/synctest`, `T.ArtifactDir` (1.26). Loop-variable copy (`tt := tt`) no longer needed. The "Other languages" table duplicates what the language packs will own; replace by a pointer to the packs. | P1 |
| §30 | Coverage | Say that the recipe does not pass `-coverpkg` and so credits only a package's own tests; document the cross-package trade-off. | P1 |
| §31 | Benchmarks | Replace "`b.ResetTimer()`; keep results alive" with `for b.Loop()` (1.24; 1.26 no longer defeats inlining); add PGO (`default.pgo`), `GOMEMLIMIT`, the 1.25 flight recorder and the default Green Tea GC (1.26). | P1 |
| §37 | Build tags, cgo | Add: `-race` requires cgo; keep cgo out of the production image but present in the test image. | P1 |
| §38 | Security | Thin: add OWASP Go-SCP mapping, `gosec`, HTTP server timeouts (`ReadHeaderTimeout`), `MaxBytesReader`, SSRF, `html/template`, `crypto/rand`/argon2, `os.Root`, supply-chain (`go mod verify`, checksum DB, SHA-pinned actions), FIPS 140-3 module. | P0 |
| §39 | Iterators | "Be cautious with closure-based iterators in hot loops" is incomplete: range-over-func (`iter.Seq`, 1.23) is the idiomatic iterator and the compiler inlines it in many cases; keep the benchmark requirement but stop discouraging it in non-hot code. | P1 |
| §40 | Memory | Mention `GOMEMLIMIT` and the 1.26 default GC change; pre-sized maps remain fine. | P2 |
| §48 | Stdlib first | The list is stale: std now has `log/slog`, `ServeMux` method/wildcard routing (1.22), `slices`/`maps`/`iter`, `os.Root`, `encoding/json/v2` (1.27) and a `uuid` package (1.27). The example "a UUID from `crypto/rand`" does not count as step 1 is wrong from 1.27 on; re-check each example against the newest release. | P0 |
| §49 | Updates / go.mod | "Only the architect touches `go.mod` and `go.sum`" now also covers `tool` and `toolchain` lines, and `go.work`. Add Dependabot/Renovate, `go mod tidy -diff`, `go mod verify`, and the rule that multi-module repos upgrade per module. | P1 |
| §50 | Custom lint rules | Add `depguard`/`forbidigo`/`sloglint` in golangci-lint v2 and `go/analysis` custom analyzers, and `go fix` modernizers. Note `loops arch` does not flag `database/sql` in domain though the pack suggests it. | P1 |
| §51, §52 | Checklists | Add: module hygiene (`tidy -diff`), `-shuffle`, SAST, OpenTelemetry/trace propagation, container build, migration rollout. | P2 |
| new | Repo and ops | No section on repository layout (`cmd/`, `internal/`, single vs. multi-module, `go.work`), toolchain pinning (`go`/`toolchain`), releases, CI shape, containers (distroless, `ko`), persistence (sqlc, pgx, migrations), API (ServeMux, Connect, OpenAPI), observability (OTel). Add one short section each, or per-topic skills. | P0 |
| §41, §48 | `pkg/` | GB §41 (`pkg/common`) and §48 (`pkg` stdlib-only) and HX §4/§15 allow `pkg/` as stdlib-only; official layout guidance does not use it. Recommend avoiding it by default. | P2 |

## Gaps vs loops

Every rule that isn't `covered`, most important first.

| Rule ID | Gap | loops today | Priority | Where loops should fix it |
|---|---|---|---|---|
| `GO-REPO-04` | Gate runs `./...` at the root only; nested modules are never built or tested | missing | P0 | gate: iterate modules from `go work edit -json` / `go.mod` discovery |
| `GO-SEC-10` | No SAST step (`gosec`) | missing | P0 | gate: add optional `gosec` check (proposed_checks), required when configured |
| `GO-SEC-03` | `os.Root` not named; GB §29 recommends outdated traversal checks | covered (GB §29, §38) | P0 | spec GB §29, §38; skill testing/security |
| `GO-SEC-05` | HTTP server timeouts not required | missing | P0 | spec GB §38 + lint (`gosec` G112) |
| `GO-TOOL-06` | `govulncheck` is optional and silent when absent; no binary mode | covered (GB §38, §49) | P0 | gate: required in CI profile; install via `tool` directive |
| `GO-REPO-11`, `GO-REPO-12` | No toolchain pin or `tool` directive guidance; tools resolved from PATH `@latest` | missing | P0 | spec (new REPO section); gate: prefer `go tool <name>` when declared |
| `GO-REPO-07`, `GO-REPO-08` | No `go mod tidy -diff` / `go mod verify` check | missing | P1 | gate: add `tidy` check |
| `GO-TEST-10` | No `-shuffle=on` | missing | P1 | gate test argv (flag-compatible with the race-guard) |
| `GO-TEST-12` | No integration run (`-tags=integration`) and no Testcontainers guidance | missing | P1 | gate: optional second test run; skill testing |
| `GO-TEST-06` | `-coverpkg` not used, cross-package tests are not credited | covered (GB §30) | P1 | gate coverage; spec GB §30 |
| `GO-TEST-08` | No integration-binary coverage recipe | missing | P2 | skill testing |
| `GO-ARCH-02` | `database/sql` not forbidden in domain though the taxonomy example lists it | covered (HX §3) | P1 | arch checker (`archrule/imports.go`) |
| `GO-TOOL-04` | No golangci-lint v2 support: config files `.golangci.yml/yaml` are read, but not `.toml/.json`; no run | go-only (GB §2) | P1 | gate: optional `golangci-lint run`; practices-researcher list |
| `GO-TOOL-08` | `go fix -diff` modernizers not run | missing | P2 | gate (advisory) |
| `GO-REPO-03`, `GO-REPO-05` | No multi-module / `go.work` policy | missing | P1 | spec (new REPO section) |
| `GO-REPO-09`, `GO-REPO-10`, `GO-REPO-14` | No Dependabot/Renovate, release or CI-shape rules | missing | P1 | spec + `templates/` |
| `GO-DATA-01..05`, `GO-DATA-08` | No persistence guidance (sqlc, pgx, migrations, pools, N+1) | missing | P1 | skill (new `go-persistence`) |
| `GO-API-01`, `GO-API-03`, `GO-API-07`, `GO-API-09`, `GO-API-10` | No HTTP/gRPC/OpenAPI/HTTP client guidance | missing | P1 | skill (new `go-api`) |
| `GO-OBS-03`, `GO-OBS-06`, `GO-OBS-07`, `GO-OBS-01` | No OpenTelemetry, no sloglint, no trace-log correlation | missing | P1 | spec GB §24, §47; skill |
| `GO-OPS-02`, `GO-OPS-03`, `GO-OPS-07..09`, `GO-OPS-11` | No packaging, container or reproducible-build rules | missing | P1 | skill (new `go-deploy`) |
| `GO-CONC-01`, `GO-CONC-06`, `GO-CONC-09`, `GO-CONC-10` | Container GOMAXPROCS, goleak/goroutineleak, graceful shutdown, synctest | missing | P1 | spec GB §11, §13, §20 |
| `GO-TEST-04`, `GO-TEST-03` | `t.Context()`, `t.Cleanup`, loopvar | missing | P2 | skill testing |
| `GO-PERF-01`, `GO-PERF-03`, `GO-PERF-04`, `GO-PERF-07` | `b.Loop`, GC/GOMEMLIMIT, PGO, link flags | covered (GB §31) | P2 | skill performance |
| `GO-IDIOM-04..06`, `GO-IDIOM-08`, `GO-IDIOM-17` | New language features (`new(expr)`, iterators, generic methods, built-ins) | missing | P2 | spec GB §17, §39 |
| `GO-SEC-02`, `GO-SEC-04`, `GO-SEC-07`, `GO-SEC-09` | SSRF, XML/JSON limits, crypto selection, supply-chain checks | missing | P1 | spec GB §38 |
| `GO-TOOL-07`, `GO-OPS-09` | SBOM, license and image scanning | missing | P2 | CI template / skill |
| `GO-ARCH-09..11` | DDD tactical, outbox, serverless guidance | covered (HX §16) | P2 | spec HX |
| `GO-FW-01..06` | Framework conventions and std-vs-third-party | missing | P2 | skill (new `go-frameworks`) |
| `GO-TEST-15` | Mutation testing | missing | P2 | skill testing |

## Open questions

1. **Coverage scope and `-coverpkg`.** Should loops pass `-coverpkg=./...` so cross-package tests count? It raises coverage numbers and runtime (and changes which `.go` lines are "covered" for new-code evaluation); sources (go.dev/doc/build-cover, cmd/go) only say the default is the main module's packages. Needs a maintainer decision.
2. **Staticcheck vs golangci-lint v2.** Both are common; golangci-lint v2 bundles staticcheck. Which should be the required linter, and may the gate use `go tool staticcheck` when a `tool` directive exists? Not settled.
3. **`encoding/json/v2` status (resolved).** The 1.27 release notes say it is available without a GOEXPERIMENT and that `encoding/json` is now backed by it, but I did not verify whether the v2 API surface is covered by the Go 1 compatibility promise; treat it as new until the Go team states so.
4. **`uuid` in the standard library (1.27).** Resolved by the verifier: top-level import path `"uuid"` (`New`, `NewV4`, `NewV7`, `Parse`, `MustParse`).
9. **Status semantics.** This file marks rules that GO_BEST/HEXAGONAL/SOLID state in Go terms as `covered` (citing the spec), whereas LOOPS_BASELINE classes GO_BEST as `go-only`; the synthesizer should reconcile this with the other packs.
5. **Multi-module gate.** Should loops treat each module as its own check set, and what is the gate's single pass/fail event schema (additive-only) for per-module results?
6. **Does `-race` run on arm64 Linux/macOS agents without cgo?** The race doc says cgo plus a C compiler are needed; a gate preflight that reports "race unavailable" instead of failing might be required for hardened sandboxes.
7. **Go version table.** Third-party pages disagree on the latest patch of 1.27 (1.27.1 vs 1.27.3); the local toolchain here is 1.27.3. Re-check at pack-build time.
8. **Several non-go.dev sources** (ADR/MADR, microservices.io, threedots) are corroborating blogs; the `go.dev/blog/survey2025` URL was not opened, so the "dominant frameworks" numbers must be re-sourced.

## Verification

- **Verdict: `fail`, round 1.**
  - What holds up: the gate mirror, the arch data, the coverage recipe and the race policy are exact. The spot run passed. The many Go 1.24–1.27 API claims are correct.
  - Why it fails:
    - 24 rule rows use the disallowed `loops today` value "covered in part";
    - two sources are weak or contradicted;
    - one claim is contradicted by the shipped toolchain.
- **Completeness:** all 14 sections, Top 10, LOOPS, Gaps, Open questions, plus an extra "Proposed updates to GO_BEST_PRACTICES" section. Rule counts: REPO 15, TOOL 9, ARCH 12, IDIOM 19, CONC 10, TEST 15, PERF 7, SEC 11, DATA 9, API 10, OBS 7, OPS 11, DOC 5, FW 6, so every minimum is met. Lint: no ERR. The LOOPS block parses and has every required key. `proposed_checks`, `multi_module`, `env_hints` and the extra `coverage` and `arch` keys are explained in the section intro and Open questions, so they are acceptable.
- **Links:** `links.sh` reported `FAIL 503` for five `github.com/uber-go/guide/blob/master/style.md#…` anchors. A recheck of style.md returned 200 (GitHub throttling, transient), so these are not findings. No other failures among the 193 URLs.
- **Sources opened (8 fetched; release-note claims also checked against the local toolchain):**
  - https://go.dev/doc/go1.27: yes. Confirms the new `uuid` package, `encoding/json/v2` with `encoding/json` backed by v2 (opt out with `GOEXPERIMENT=nojsonv2`; "the v1 API will continue to be supported"), the gofmt alignment change, `stdversion` in `go test`, `goroutineleak` GA, `synctest.Sleep`, generic methods (interface methods excluded), `strings.CutLast`, `go doc pkg@version`, and the `atomictypes`/`slicesbackward` modernizers. The small-allocation claim is "up to 30%" for allocations under 80 bytes and about 1% overall, so "~30% cheaper" in GO-PERF-03 overstates it. The notes give no day for the 2026-08-19 release date.
  - https://go.dev/doc/go1.26: yes for `errors.AsType`, `new(expr)`, the ServeMux 307 redirect, race on riscv64, Green Tea by default, `cryptotest.SetGlobalRandom`, crypto ignoring the `rand` argument, `b.Loop` no longer blocking inlining, `slog.NewMultiHandler`, `T.ArtifactDir`, the `go fix` modernizers and self-referential type parameters. Its `go mod init` statement does not match the shipped toolchain (finding 4).
  - https://go.dev/doc/articles/race_detector: yes. Platforms, the cgo and C-compiler requirement, 5-10x memory and 2-20x time overhead, and the `GORACE` options all match.
  - https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck: yes. It exits non-zero on findings in text mode and 0 for json/sarif/openvex, and supports `-mode binary`.
  - https://go.dev/doc/build-cover: yes. Confirms `go build -cover`, `GOCOVERDIR`, `covdata textfmt`, data lost on panic, main-module defaults, and `-coverpkg` taking import paths rather than `main`.
  - https://golangci-lint.run/docs/configuration/file/: yes. Confirms `version: "2"`, `linters.default: standard`, `formatters:`, `exclusions.generated: lax`, and the four config file names.
  - https://go.dev/blog/osroot: weak. It supports `os.Root`, `OpenRoot`, `OpenInRoot` and `IsLocal`, but its "does not support … symbolic links and renaming" is from the Go 1.24 era and is false in 1.27 (finding 5).
  - https://go.dev/blog/survey2025: no. The page exists (dated 2026-01-21) but reports no web-framework usage, so it does not support GO-FW-01 (finding 6).
  - Local `go doc` on go1.27.3 confirms the signatures of `errors.AsType`, `strings.CutLast`, `synctest.Sleep`, `B.Loop`, `T.ArtifactDir`, `T.Context`, `slog.NewMultiHandler`, `cryptotest.SetGlobalRandom`, `rand.Text`, `WaitGroup.Go` and `os.OpenInRoot`, and the `uuid` package. `runtime/pprof` has the `goroutineleak` profile, and the modernize pass set includes `atomictypes`, `slicesbackward`, `errorsastype`, `newexpr` and `waitgroupgo`.
- **Tooling truth:**
  - The `checks` array matches `internal/application/gate/gate.go` exactly:
    - `CheckNames` order is build, gofmt, vet, staticcheck, arch, test, coverage, govulncheck;
    - `RequiredChecks` = build, vet, test, coverage;
    - gofmt filters `vendor/` and `testdata/`;
    - staticcheck runs only when it is on PATH, and a stale version is skipped;
    - govulncheck needs `o.Vuln` and the binary on PATH;
    - the test check adds `-covermode=atomic -coverprofile=` only when `CoverProfile` is set;
    - `--skip coverage` is refused (`gate_cmd.go`).
  - `coverage.default_exclude` and `min 70` match `internal/domain/coverage/evaluate.go`, and the generated regex matches `diff.go`.
  - The arch data matches `internal/domain/archrule`:
    - `os` and `net/http` are forbidden via `stdUnder`, and `database/sql` is not forbidden;
    - third-party imports are banned in domain, application and pkg;
    - the limits are 40/25/8/12000/60, with cmd at 80 lines and 40 runes.
  - `artifacts` matches `workspace.IsArtifactName`. `race-guard` wrappers (`gotestsum`, `richgo`) and the `-race=false` forms match `guard/race.go`.
- **Spot run:** image `golang:1.27` (go1.27.3 linux/arm64):
  - `gofmt -l .`: exit 0, no output.
  - `go build ./...`: exit 0.
  - `go vet ./...`: exit 0.
  - `go test -race -count=1 -covermode=atomic -coverprofile=/tmp/cover.out ./...`: exit 0. The profile starts with `mode: atomic`.
  - `go test -race -count=1 -shuffle=on ./...`: exit 0.
  - `go mod tidy -diff`: exit 0.
  - `go fix -diff ./...`: exit 0.
  - A program importing `"uuid"` and `"encoding/json/v2"` built with no GOEXPERIMENT and printed `36 {"a":1}`.
  - `go list encoding/json/v2` works on go1.27.3. Its files carry `//go:build goexperiment.jsonv2`, and `internal/buildcfg` sets the `JSONv2` baseline to true.
- **Findings:**
  1. **24 rule rows use "covered in part", which is not allowed** (always a finding). The rows are GO-REPO-09, GO-REPO-14, GO-TOOL-04, GO-TOOL-05, GO-TOOL-06, GO-ARCH-10, GO-IDIOM-09, GO-CONC-05, GO-CONC-08, GO-TEST-05, GO-TEST-14, GO-PERF-01, GO-SEC-03, GO-SEC-07, GO-SEC-09, GO-DATA-02, GO-DATA-07, GO-API-03, GO-API-06, GO-API-07, GO-API-08, GO-OBS-02, GO-OBS-04 and GO-FW-04. The Gaps table also uses "covered in part", "partial" and "missing / partial". Fix: use `go-only` or `missing` and put the partial explanation in the Gap column.
  2. **Text outside the parentheses in `loops today`.** GO-IDIOM-04, GO-IDIOM-06, GO-TEST-03, GO-CONC-03 and GO-OBS-01 have values such as "covered (GB §7); `new(expr)` missing". Fix: keep only the value plus a parenthesised citation, and move "X missing" into the Gaps table.
  3. **Status semantics differ from TAXONOMY.** The preamble redefines `covered` as "loops says it in Go terms", while LOOPS_BASELINE classes GO_BEST as `go-only`. This is defensible for the Go pack, but record it in Open questions so the synthesizer reconciles it with the other packs.
  4. **GO-REPO-06: the `go mod init` claim is false for the shipped toolchains.**
     - The 1.26 notes say `go mod init` writes `go 1.(N-1).0`. In practice, `golang:1.26` (go1.26.4) wrote `go 1.26.4`, and `golang:1.27` and local go1.27.3 (also with `GOTOOLCHAIN=local`) wrote `go 1.27.3`.
     - The 1.27.3 source `cmd/go/internal/modload/init.go` `CreateModFile` calls `addGoStmt(…, gover.Local())`.
     - Fix: drop the note, or say that the documented N-1 default is not in 1.26.4/1.27.3 and that `go get go@1.N.0` lowers the line.
  5. **GO-SEC-03: "`root` does not yet create symlinks or rename" is outdated.** On go1.27.3, `go doc os.Root` lists `Symlink`, `Rename`, `Link`, `Readlink`, `Chmod`, `Chown`, `Chtimes`, `MkdirAll`, `RemoveAll`, `ReadFile` and `WriteFile` (added in 1.25). Fix: remove the limitation and cite https://pkg.go.dev/os#Root or the 1.25 notes as well as the 1.24 blog.
  6. **GO-FW-01: unsupporting source.** go.dev/blog/survey2025 has no framework data, and the JetBrains link is the blog root. Fix: cite a page that reports Go web-framework usage (for example JetBrains' State of Developer Ecosystem Go section), or drop the usage claim.
  7. **Proposed-updates table, row "§3 `pkg/`".** GB §3 (Package naming) does not mention `pkg/`; `pkg` appears in GB §41 (`pkg/common`), §48 (stdlib-only `pkg`) and HX §4/§15. Fix the § reference. Every other § cited in that table was checked against `specs/GO_BEST_PRACTICES.md` and matches.
  8. **Stray file.** `research/languages/go.md-E` (a BSD `sed -i -E` backup, 09:37) holds an older draft whose rows link loops spec sections to unrelated go.dev pages (for example `[GB §7](https://go.dev/doc/effective_go)`). Delete it so it is not picked up as a pack.
  9. **Open questions 3 and 4 can be closed.** `uuid` is a top-level std package with import path `"uuid"`: `New`, `NewV4`, `NewV7`, `Parse`, `MustParse`, `Nil`, `Max`, RFC 9562. JSON v2 is on by default in 1.27 with the `nojsonv2` opt-out. Put the import path into GO-FW-06 and the GB §48 row.
  10. **Minor accuracy fixes.**
      - GO-PERF-03 "~30% cheaper": write "up to 30% for allocations under 80 bytes, about 1% overall" (1.27 notes).
      - GO-REPO-12 pins `staticcheck@v0.7.0`, but the latest is v0.8.2 (2026.2.2, 2026-10-12, proxy.golang.org). Use a placeholder or the current version.
      - GO-OPS-10 uses Sev `AVOID` on an `n/a` item. AVOID must name an alternative, so use SHOULD with the rule text.

### Round 2

- **Verdict: `pass`, round 2.** Lint has no ERR. `links.sh` printed no FAIL; it shows 13 `blocked 429` from pkg.go.dev (rate limit, not a finding). All 10 findings are resolved, and `go.md-E` is deleted (coordinator).
- **LOOPS block:** parses with jq. The `checks` names and order are still build, gofmt, vet, staticcheck, arch, test, coverage, govulncheck. The test argv is unchanged (`go test -race -count=1 -covermode=atomic -coverprofile={coverage} ./...`).
- **Findings:**
  1. "covered in part": **resolved.** No "covered in part" or "partial" remains in the body or the Gaps table. Under the coordinator's policy, rules the Go spec states count as `covered` for Go.
  2. Text outside the parentheses: **resolved.** Every `loops today` value is now one of the four values, optionally followed by a parenthesised citation.
  3. Status semantics: **resolved.** They are recorded in Open questions (item 9).
  4. GO-REPO-06: **resolved.** It now states the observed behaviour (go1.26.4 and go1.27.3 write the full local version) and gives the `go get go@1.N.0` workaround.
  5. GO-SEC-03: **resolved.** It lists the 1.25 `Root` methods and cites https://pkg.go.dev/os#Root.
  6. GO-FW-01: **resolved.** The list is now explicitly qualitative, and the survey post is cited as having no framework data.
  7. GB table `pkg/` row: **resolved.** It now cites §41 and §48 plus HX §4/§15, which matches the spec.
  8. Stray `go.md-E`: **resolved.** It is deleted.
  9. `uuid` and json/v2: **resolved.** GO-FW-06 gives the import path `"uuid"`, and Open questions 3 and 4 are marked resolved.
  10. Minor fixes: **resolved.**
      - GO-PERF-03 now reads "<80 B, up to 30%, about 1% overall".
      - GO-REPO-12 uses `@<version>`.
      - GO-OPS-10 Sev is now SHOULD and names an alternative.
- **Non-blocking cosmetic notes:**
  - Open questions are out of order (item 9 sits between 4 and 5).
  - Item 8 still says the survey URL "was not opened" and that the numbers "must be re-sourced", although GO-FW-01 now handles this.
