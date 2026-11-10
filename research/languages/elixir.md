# Elixir: backend practices and loops gap analysis

| | |
|---|---|
| Pack id | `elixir` |
| Current stable / LTS | Elixir `1.20` (1.20.0 released 2026-06-03; latest `1.20.5`, 2026-10-29) / none (support window: only the newest minor gets bug fixes plus security patches; `1.16`-`1.19` get security patches only). Erlang/OTP `29` (GA 2026-05) and `28`; Elixir 1.20 needs OTP 27-29, 1.19 needs OTP 26-28 |
| Dominant runtimes / frameworks | BEAM (Erlang/OTP). Phoenix 1.8 (+ LiveView), Plug/Bandit, Ecto 3.1x, Oban, Broadway, Absinthe (GraphQL), Req (HTTP client) |
| As of | 2026-11 |
| Researcher | `B1-elixir` |
| Verifier | (filled in by B2) |

> **Summary:** A strong Elixir team ships a Mix project (umbrella only when two deployables must share code), with a committed `mix.lock`, `.tool-versions`, `mix format --check-formatted`, Credo `--strict`, optional Dialyzer, ExUnit with `async: true` by default, Ecto sandbox tests, and an OTP release in a container. The BEAM removes shared-memory data races, so there is **no race flag**; the honest substitute is async + random seed + `--repeat-until-failure`. Biggest gaps vs loops: (1) the gate, coverage parser, arch checker, hooks and cache env are all Go-only, and the built-in `mix test --cover` emits no lcov/Cobertura, so excoveralls is required for new-code coverage; (2) loops has no OTP guidance (supervision, when not to use a GenServer, ETS races); (3) repo management, toolchain pinning, migrations safety and release/packaging are entirely `missing`.

Rules follow [TAXONOMY.md](TAXONOMY.md), using the row format `ID | Rule | Why | Sev | Source | loops today | Enforce`. Gaps are judged against [LOOPS_BASELINE.md](LOOPS_BASELINE.md). Shorthand: `HEX_ARCH` = `specs/HEXAGONAL_ARCHITECTURE.md`, `WF` = `specs/WORKFLOW.md`.

Facts verified by fetching docs on 2026-11-10: Elixir 1.20.5 / OTP matrix, Mix env vars, `mix test` and `mix format` flags, `mix xref`, `mix compile`, Boundary 0.12.0, Credo 1.7.19, Dialyxir, mix_audit, `mix hex.audit` (Hex 2.7.0), Sobelow, StreamData 1.4.0, telemetry 1.4.2, excoveralls output paths, Ecto 3.14.3 / Phoenix 1.8.16 docs. Items I could not fetch are listed in "Open questions".

## 1. REPO: repository and project management

Canonical tree for a Phoenix service (single Mix project, the default; run `mix phx.new my_app`, or `mix new my_app --sup` without the web layer):

```
my_app/
├── mix.exs                  # project/0, application/0, deps/0, releases:, elixirc_paths
├── mix.lock                 # committed (apps AND libraries' dev lock is fine)
├── .tool-versions           # elixir 1.20.5-otp-28 / erlang 28.x  (asdf or mise)
├── .formatter.exs           # inputs, import_deps: [:ecto, :phoenix], plugins: [Phoenix.LiveView.HTMLFormatter]
├── .credo.exs  .dialyzer_ignore.exs  .sobelow-conf  coveralls.json  .gitignore  .editorconfig
├── config/                  # config.exs (build-time), dev.exs, test.exs, prod.exs, runtime.exs (ALL env/secrets)
├── lib/
│   ├── my_app/              # core: domain + application + outbound adapters (NO web deps)
│   │   ├── application.ex   # supervision tree = composition root
│   │   ├── repo.ex          # Ecto.Repo
│   │   └── <context>/ …
│   └── my_app_web/          # inbound adapter: endpoint, router, controllers, live, plugs, components
├── priv/repo/migrations/    # timestamped Ecto migrations; priv/repo/seeds.exs
├── test/{my_app,my_app_web}/**/*_test.exs   test/support/{data_case,conn_case,fixtures}.ex   test/test_helper.exs
├── rel/overlays/bin/{server,migrate}        # from `mix phx.gen.release`
└── Dockerfile  .github/workflows/ci.yml  .github/dependabot.yml
```

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `ELIXIR-REPO-01` | Use one Mix project with `lib/<app>/` (core) and `lib/<app>_web/` (web). Put tests in `test/` mirroring `lib/`, files named `*_test.exs`, helpers in `test/support/` compiled only in `:test` via `elixirc_paths(:test)`. [REPO-01] | Mix's generator and every tool (formatter, coverage, `--stale`) assume this layout. | MUST | [Intro to Mix](https://elixir.hexdocs.pm/introduction-to-mix.html); [Mix.Tasks.Test](https://mix.hexdocs.pm/Mix.Tasks.Test.html) (`test_paths`, `test_pattern`) | missing | review |
| `ELIXIR-REPO-02` | Start new services from `mix phx.new` (web) or `mix new NAME --sup` (OTP app with supervision tree); plain `mix new` has no supervisor or `mod:` callback. [REPO-03] | A supervised application skeleton is the unit of deployment on the BEAM. | SHOULD | [Mix.Tasks.New](https://mix.hexdocs.pm/Mix.Tasks.New.html) | missing | review |
| `ELIXIR-REPO-03` | Default to a single Mix project; reach for an umbrella (`mix new X --umbrella`, apps in `apps/`) only when several apps need independent deploy or dependency sets. Prefer `in_umbrella: true` deps over cross-app config. [REPO-02] | Umbrellas share one `mix.lock`/build and add config and CI complexity; a modular monolith with Boundary is usually enough. | SHOULD | [Mix.Tasks.New](https://mix.hexdocs.pm/Mix.Tasks.New.html); [Mix.Tasks.Test.Coverage](https://mix.hexdocs.pm/Mix.Tasks.Test.Coverage.html) (umbrella root config) | missing | review |
| `ELIXIR-REPO-04` | Commit `mix.lock` for applications. Libraries published to Hex commit it for dev reproducibility but never rely on it for consumers (it is excluded from the package). Never hand-edit it; use `mix deps.update`. [REPO-04] | Lock gives reproducible builds; Hex ignores a dependency's lock file. | MUST | [Hex publish](https://hex.hexdocs.pm/Mix.Tasks.Hex.Publish.html); [Mix.Tasks.Deps.Get](https://mix.hexdocs.pm/Mix.Tasks.Deps.Get.html) | missing | gate |
| `ELIXIR-REPO-05` | Pin versions in `mix.exs` with `~>` (pessimistic) ranges, in CI run `mix deps.get --check-locked` and `mix deps.unlock --check-unused`, and remove dead deps with `mix deps.clean --unlock --unused`. [REPO-04] | Detects drift between `mix.exs` and `mix.lock`, and unused dependencies. | SHOULD | [Deps.Unlock](https://mix.hexdocs.pm/Mix.Tasks.Deps.Unlock.html) (`--check-unused`); [Deps.Clean](https://mix.hexdocs.pm/Mix.Tasks.Deps.Clean.html) | missing | gate |
| `ELIXIR-REPO-06` | Automate updates with Dependabot (`package-ecosystem: "mix"`, version updates only) or Renovate. Do not expect Dependabot security alerts for Hex; rely on `mix deps.audit` / `mix hex.audit` in CI. [REPO-04] | GitHub lists Hex as version-updates only (as of 2026-11). | SHOULD | [Dependabot ecosystems](https://docs.github.com/en/code-security/dependabot/ecosystems-supported-by-dependabot/supported-ecosystems-and-repositories) | missing | review |
| `ELIXIR-REPO-07` | Follow SemVer; a Hex package needs a `package/0` (licenses, links), `@version`, and `mix hex.build` clean before `mix hex.publish`. Services are tagged `v<version>` and shipped as releases, not published. [REPO-05] | Hex is the registry; publishing is permanent apart from a short revert window. | SHOULD | [Hex.Publish](https://hex.hexdocs.pm/Mix.Tasks.Hex.Publish.html); [Hex.Build](https://hex.hexdocs.pm/Mix.Tasks.Hex.Build.html); [hex.pm publish](https://hex.pm/docs/publish) | missing | review |
| `ELIXIR-REPO-08` | Pin the toolchain in `.tool-versions` (asdf or mise) with full OTP major: `elixir 1.20.5-otp-28`, `erlang 28.x`; mirror it in CI and the Dockerfile, and set `elixir: "~> 1.19"` in `mix.exs`. [REPO-06] | Formatter output and compiler type warnings change between Elixir versions; OTP minor affects behaviour. | MUST | [asdf-elixir](https://github.com/asdf-vm/asdf-elixir); [mise elixir](https://mise.jdx.dev/lang/elixir.html); [mix format --check-formatted note](https://mix.hexdocs.pm/Mix.Tasks.Format.html) | missing | review |
| `ELIXIR-REPO-09` | Ignore `/_build/`, `/deps/`, `/cover/`, `/doc/`, `erl_crash.dump`, `*.ez`, `/priv/plts/*.plt*`, `/tmp/`, editor dirs (`.elixir_ls/`, `.lexical/`) and `*.beam`; commit `.formatter.exs`, `.credo.exs`, `coveralls.json`, `.sobelow-conf`. [REPO-07] | `mix new`/`phx.new` generate most of this list; the rest come from Dialyxir and ExUnit `tmp_dir`. | MUST | [Dialyxir](https://github.com/jeremyjh/dialyxir) (PLT in `priv/plts`); [ExUnit.Case tmp_dir](https://ex-unit.hexdocs.pm/ExUnit.Case.html) | missing | review |
| `ELIXIR-REPO-10` | CI order on every PR: `deps.get` (cached `deps/`+`_build/` keyed on `mix.lock`+`.tool-versions`) → `mix format --check-formatted` → `mix compile --warnings-as-errors` → `mix credo --strict` → `mix dialyzer` (PLT cached) → `mix test` with coverage → `mix deps.audit` + `mix hex.audit` → `mix sobelow --exit medium` → `mix release`. Matrix over the supported Elixir/OTP pair(s) for libraries only. [REPO-08] | Fast-fail ordering; compile warnings are cheap and catch type-checker findings. | MUST | [Mix.Tasks.Compile](https://mix.hexdocs.pm/Mix.Tasks.Compile.html); [Credo](https://credo.hexdocs.pm/Mix.Tasks.Credo.html) | missing | gate |
| `ELIXIR-REPO-11` | Use conventional commits. Elixir has no changelog generator standard; keep `CHANGELOG.md` for libraries; bump `@version` in `mix.exs` in one place and tag `v<version>` from it. [REPO-09] | `mix.exs` `version:` must match the tag; one source of truth. (`loops release` only manages the loops repo's own `plugin.json`.) | SHOULD | [Conventional Commits](https://www.conventionalcommits.org/); [Hex publish](https://hex.hexdocs.pm/Mix.Tasks.Hex.Publish.html) | missing | review |

## 2. TOOL: toolchain

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `ELIXIR-TOOL-01` | Target Elixir `1.19`/`1.20` on OTP `27`/`28`/`29`. Support matrix (2026-11): 1.20 → OTP 27-29 (bug fixes + security); 1.19 → OTP 26-28 and 1.18 → OTP 25-27 (security only). 1.19 requires OTP 28.1+ for OTP 28. Release cadence: roughly two minors per year, 1.20 on 2026-06-03. [TOOL-01] | New minors are the only ones receiving bug fixes. | MUST | [Compatibility and deprecations](https://elixir.hexdocs.pm/compatibility-and-deprecations.html); [v1.19 release](https://elixir-lang.org/blog/2025/10/16/elixir-v1-19-0-released/); [v1.20 release](https://elixir-lang.org/blog/2026/06/03/elixir-v1-20-0-released/) | missing | review |
| `ELIXIR-TOOL-02` | Use asdf or mise as the version manager; or the official `hexpm/elixir` Docker image tags. Never rely on the distro `elixir` package. [TOOL-01] | Distro packages lag by minors and pick arbitrary OTP. | SHOULD | [asdf-elixir](https://github.com/asdf-vm/asdf-elixir); [mise elixir](https://mise.jdx.dev/lang/elixir.html) | missing | review |
| `ELIXIR-TOOL-03` | The formatter is `mix format`; configure in `.formatter.exs` (`inputs`, `import_deps`, `subdirectories`, `plugins`, `line_length`, `locals_without_parens`). CI argv: `mix format --check-formatted` (non-zero on diff; `--no-exit` only reports). Format one file with `mix format <file>`. [TOOL-02] | One canonical style; the formatter ships with Elixir. `excludes` option exists since 1.19. | MUST | [Mix.Tasks.Format](https://mix.hexdocs.pm/Mix.Tasks.Format.html) | go-only (gate runs gofmt; WF §6) | gate |
| `ELIXIR-TOOL-04` | Run Credo: `mix credo --strict` (exit non-zero on issues; `--format json\|sarif\|oneline\|flycheck`). Commit `.credo.exs` (`mix credo gen.config`); version 1.7.19. [TOOL-03] | De-facto linter: consistency, readability, refactoring, design, warnings. | SHOULD | [Mix.Tasks.Credo](https://credo.hexdocs.pm/Mix.Tasks.Credo.html); [Credo config](https://credo.hexdocs.pm/config_file.html) | missing | gate |
| `ELIXIR-TOOL-05` | Treat compiler warnings as errors: `mix compile --warnings-as-errors` and `mix test --warnings-as-errors` (`mix test --warnings-as-errors` exits 1 when the suite would otherwise pass, and `--exit-status`+1, default 3, when tests also fail; spot-run on 1.20.5). [TOOL-03] | Since 1.17-1.20 the compiler type system reports real bugs as warnings; warnings are the first static analysis. | MUST | [Mix.Tasks.Compile](https://mix.hexdocs.pm/Mix.Tasks.Compile.html); [Mix.Tasks.Test](https://mix.hexdocs.pm/Mix.Tasks.Test.html) | missing | gate |
| `ELIXIR-TOOL-06` | Type checking is the compiler's built-in gradual set-theoretic type system (no annotations): 1.17 introduced it, 1.19 added call/protocol/anonymous-function inference, 1.20 (2026-06) infers types of all constructs, including guards. Type *signatures* are not available yet. Do not write type-system workarounds; fix the warning. Use `@spec` and Dialyzer only as an optional second layer. [TOOL-04] | Compile output is free analysis; `dynamic()` avoids false positives. | MUST | [v1.20 release](https://elixir-lang.org/blog/2026/06/03/elixir-v1-20-0-released/); [Gradual set-theoretic types](https://elixir.hexdocs.pm/gradual-set-theoretic-types.html); [Typespecs](https://elixir.hexdocs.pm/typespecs.html) | missing | gate |
| `ELIXIR-TOOL-07` | Dialyzer via `dialyxir` (`{:dialyxir, "~> 1.4", only: [:dev, :test], runtime: false}`); run `mix dialyzer` (`--format github` in CI). Cache the project PLT: `dialyzer: [plt_file: {:no_warn, "priv/plts/project.plt"}, plt_local_path: "priv/plts"]`; ignore file `.dialyzer_ignore.exs`. Make it optional (`required: false`): first PLT build is minutes. [TOOL-04] | PLT in `_build` is rebuilt per OTP/Elixir upgrade; cold run slows loops gates. | SHOULD | [Dialyxir](https://github.com/jeremyjh/dialyxir) | missing | gate |
| `ELIXIR-TOOL-08` | Vulnerability scan: `mix deps.audit` (mix_audit, `{:mix_audit, "~> 2.1", only: [:dev, :test], runtime: false}`, exit 1 on findings, `--format json`) plus `mix hex.audit` (Hex 2.7 reports retired packages and security advisories; exits non-zero). Supply `ignore_advisories` in Hex config for accepted risks. [TOOL-05] | Two advisory feeds (Elixir security-advisories repo + Hex.pm). | MUST | [mix_audit](https://github.com/mirego/mix_audit); [Hex.Audit](https://hex.hexdocs.pm/Mix.Tasks.Hex.Audit.html) | missing | gate |
| `ELIXIR-TOOL-09` | SBOM/licences: generate CycloneDX with the `sbom` Hex package (`mix sbom.cyclonedx`) and check licences from `mix hex.info`/package metadata; record the choice. [TOOL-05] | Compliance; the ecosystem has no built-in. | SHOULD | [Hex.pm package metadata](https://hex.pm/docs/publish) (licenses field); [sbom on Hex](https://hex.pm/packages/sbom) (`mix sbom.cyclonedx`) | missing | review |
| `ELIXIR-TOOL-10` | Editor: use [Expert](https://github.com/expert-lsp/expert) (official Elixir LSP) or ElixirLS/Lexical; set format-on-save to `mix format` only (disable competing formatters). [TOOL-06] | LSP format and `.formatter.exs` must agree; wrong plugin order breaks `.heex`. | SHOULD | [Mix.Tasks.Format `--stdin-filename`](https://mix.hexdocs.pm/Mix.Tasks.Format.html) | missing | review |

## 3. ARCH: architecture

Phoenix contexts are the idiomatic boundary; a context is just a module that owns a feature's data and logic. They map to hexagonal when you separate core from web and put ports (`@behaviour`) at the edges. The loops-layer mapping for a Phoenix service:

```
lib/my_app/                         # no Phoenix/Plug imports allowed anywhere below
├── application.ex                  # COMPOSITION ROOT: supervision tree, children, adapter wiring
├── orders.ex                       # APPLICATION: public context API (use cases); first arg = scope
├── orders/
│   ├── order.ex                    # DOMAIN: struct + pure functions (state transitions, invariants)
│   ├── money.ex                    # DOMAIN value object
│   ├── ports.ex                    # PORTS: @behaviour OrderStore, @behaviour Clock, @behaviour PaymentGateway (consumer-owned)
│   ├── services/place_order.ex     # APPLICATION: one use case, plain module (no GenServer)
│   └── adapters/
│       ├── ecto_order_store.ex     # OUTBOUND: implements OrderStore with Repo + schema
│       ├── order_schema.ex         # OUTBOUND: Ecto.Schema + changesets (persistence model)
│       └── stripe_gateway.ex       # OUTBOUND: Req-based client
├── repo.ex
lib/my_app_web/                     # INBOUND: controllers, LiveViews, channels, router, plugs; call Orders.* only
config/runtime.exs                  # picks adapters per environment (Application env) -- config, not code
test/support/mocks.ex               # Mox.defmock(MyApp.Orders.PaymentGatewayMock, for: MyApp.Orders.PaymentGateway)
```

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `ELIXIR-ARCH-01` | Choose the style by size: layered Phoenix contexts for CRUD-heavy apps; hexagonal (ports as behaviours, Ecto only in adapters) when the domain is rich or has several I/O boundaries; modular monolith via `Boundary` before microservices; CQRS/event sourcing only with Commanded and a proven need; the actor model is native (one process per aggregate/session) but not a default. [ARCH-01] | Contexts are Phoenix's default; hexagonal is a discipline layered on top, not a different framework. | SHOULD | [Phoenix contexts](https://phoenix.hexdocs.pm/contexts.html); HEX_ARCH (`specs/HEXAGONAL_ARCHITECTURE.md`) | covered (HEX_ARCH §1-§30 concepts) | review |
| `ELIXIR-ARCH-02` | Keep `lib/<app>/` free of `Phoenix.*`, `Plug.*` and `<App>Web.*` references; web calls the context API only, never `Repo` or schemas directly. [ARCH-02] | The web layer is an inbound adapter. | MUST | [Phoenix contexts](https://phoenix.hexdocs.pm/contexts.html); [Boundary](https://boundary.hexdocs.pm/Boundary.html) | go-only (rule is neutral, HEX_ARCH dependency rule, but the checker parses only Go) | arch |
| `ELIXIR-ARCH-03` | One module = one file; module `Foo.Bar` lives at `lib/foo/bar.ex`; nested namespaces mirror directories. Imports are `alias Foo.Bar`, `alias Foo.{Bar, Baz}`, `import`, `require`, `use`; calls can also be fully qualified without an alias. All modules are public (`def`) unless you mark functions `defp` or modules `@moduledoc false`. Cycles in module references are allowed at runtime but cause compile-time recompilation chains. [ARCH-03] | The arch checker must handle grouped aliases and implicit qualified calls; `@moduledoc false` marks internals. | MUST | [Style guide](https://github.com/christopheradams/elixir_style_guide); [Mix.Tasks.Xref](https://mix.hexdocs.pm/Mix.Tasks.Xref.html) | missing | arch |
| `ELIXIR-ARCH-04` | Enforce layers with the `boundary` library (v0.12.0): add `compilers: [:boundary] ++ Mix.compilers()`, declare `use Boundary, deps: [...], exports: [...]` in each context root module; violations are compiler warnings, so run `mix compile --warnings-as-errors`. [ARCH-04] | It is the ecosystem's `import-linter`; native, compile-time, no runtime cost. | MUST | [Boundary](https://boundary.hexdocs.pm/Boundary.html) | missing | arch |
| `ELIXIR-ARCH-05` | Also gate cycles: `mix xref graph --format cycles --fail-above 0` (and `--label compile-connected` for recompile-health). Use `mix xref graph --source/--sink` to explain a failure. [ARCH-04] | Cycles slow compilation and hide layering mistakes. | SHOULD | [Mix.Tasks.Xref](https://mix.hexdocs.pm/Mix.Tasks.Xref.html) | missing | gate |
| `ELIXIR-ARCH-06` | Wire dependencies manually in `Application.start/2` (child specs) and `config/runtime.exs` (adapter module per environment). Read config once at startup or pass it as arguments; avoid `Application.get_env` deep in domain code. Avoid service locators and compile-time `Application.compile_env` for per-request choices. [ARCH-05] | The supervision tree is the composition root; OTP has no DI container. | SHOULD | [Application](https://elixir.hexdocs.pm/Application.html); [Config](https://elixir.hexdocs.pm/Config.html); [Design anti-patterns](https://elixir.hexdocs.pm/design-anti-patterns.html) (application config for libraries) | covered (HEX_ARCH §21 DI) | review |
| `ELIXIR-ARCH-07` | Keep the domain pure: no `Repo`, `File`, `System`, `Logger`, `DateTime.utc_now`, `:rand`/`Enum.random`, `Process`/`Task`/`send`, `:ets` calls. Pass time, ids and randomness in as arguments or via a `Clock`/`IdGenerator` behaviour. Return tagged tuples. [ARCH-06] | Pure functions are trivially testable and need no processes. | MUST | [Library guidelines](https://elixir.hexdocs.pm/library-guidelines.html); [Process anti-patterns](https://elixir.hexdocs.pm/process-anti-patterns.html) | go-only (clock/RNG injection is neutral in the consistency skill, but purity checks are Go calls) | arch |
| `ELIXIR-ARCH-08` | Define ports as `@behaviour` with `@callback`s in the consumer (context) module; adapters declare `@behaviour` and `@impl true`; test doubles via `Mox.defmock(..., for: Behaviour)`. Keep each behaviour to a handful of callbacks. [ARCH-07] | Behaviours are Elixir's interfaces and the only thing Mox accepts. | SHOULD | [Mox](https://mox.hexdocs.pm/Mox.html); [Typespecs](https://elixir.hexdocs.pm/typespecs.html) | covered (consumer-owned ports, HEX_ARCH) | review |
| `ELIXIR-ARCH-09` | Do not use a process for code organization. A `GenServer`/`Agent`/`Task` is only for mutable runtime state, concurrency, or failure isolation; plain modules and functions are the default. Wrap every process behind one API module. [ARCH-08] | Processes serialize calls and add failure modes; modules cost nothing. | MUST | [Process anti-patterns](https://elixir.hexdocs.pm/process-anti-patterns.html); [GenServer](https://elixir.hexdocs.pm/GenServer.html) | missing | review |
| `ELIXIR-ARCH-10` | Size limits (loops-aligned): keep functions short and multi-clause; avoid parameter lists over 5 (use a keyword list/struct); keep structs under 32 fields (compiler map-optimisation limit); avoid `with` chains with complex `else`. [ARCH-08] | SOLID in Elixir is module cohesion and small pattern-matching clauses. | SHOULD | [Code anti-patterns](https://elixir.hexdocs.pm/code-anti-patterns.html); [Design anti-patterns](https://elixir.hexdocs.pm/design-anti-patterns.html) | go-only (SOLID_GO §8 counts in Go units) | arch |
| `ELIXIR-ARCH-11` | In a Phoenix 1.8 app pass a `scope` struct as the first argument of every context function and filter data by it; do not read user/org from process state. [ARCH-01] | Scopes are the framework's answer to OWASP broken access control and multi-tenancy. | SHOULD | [Phoenix scopes](https://phoenix.hexdocs.pm/scopes.html) | missing | review |

## 4. IDIOM: what to do and what to avoid

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `ELIXIR-IDIOM-01` | Return `{:ok, value}` / `{:error, reason}` for expected failures; raise only for programmer errors; supply `foo/1` and `foo!/1` pairs where the bang raises. Do not use exceptions for control flow. [IDIOM-01] | Tagged tuples compose with `with` and pattern matching. | MUST | [Design anti-patterns](https://elixir.hexdocs.pm/design-anti-patterns.html); [Style guide](https://github.com/christopheradams/elixir_style_guide) | go-only (GO_BEST §8-9 errors) | review |
| `ELIXIR-IDIOM-02` | "Let it crash": do not rescue/catch broadly to hide failure; handle the error cases you can act on and let supervisors restart on the rest. Never `rescue _ ->` without re-raise or logging. [IDIOM-01] | Supervision gives recovery to a known state; swallowing hides bugs. | MUST | [Supervisor](https://elixir.hexdocs.pm/Supervisor.html); [OTP design principles](https://www.erlang.org/doc/system/sup_princ.html) | missing | review |
| `ELIXIR-IDIOM-03` | Normalise error shapes in small private functions near the source; avoid large `else` blocks in `with`. Prefer `{:error, %MyApp.Error{}}` or atoms like `:not_found` over strings. [IDIOM-01] | `with ... else` with mixed shapes becomes unreadable. | SHOULD | [Code anti-patterns](https://elixir.hexdocs.pm/code-anti-patterns.html) | missing | review |
| `ELIXIR-IDIOM-04` | Absence is `nil` or `{:ok, nil}`/`:error`; use `Map.fetch/2` and `fetch!` for required keys. Use `map.key` for required keys, `map[:key]` for optional, and pattern matching otherwise. Never rely on truthiness with non-booleans; use `and/or/not` for booleans. [IDIOM-02] | Non-assertive access turns typos into `nil` bugs. | MUST | [Code anti-patterns](https://elixir.hexdocs.pm/code-anti-patterns.html) | missing | review |
| `ELIXIR-IDIOM-05` | All data is immutable. Model domain values with structs (`defstruct`, `@enforce_keys`) and use `%{struct \| key: val}` updates; do not mix maps and structs for the same concept. Avoid primitive obsession. [IDIOM-03] | Structs give enforced keys and pattern-match safety. | SHOULD | [Design anti-patterns](https://elixir.hexdocs.pm/design-anti-patterns.html) | missing | review |
| `ELIXIR-IDIOM-06` | Use pattern matching and multi-clause functions with guards; match explicit patterns and avoid catch-all `_` clauses that mask new cases. Put guard-only extraction in the signature and other values in the body. [IDIOM-04] | Pattern matching is the control-flow primitive; catch-alls hide missing cases. | SHOULD | [Code anti-patterns](https://elixir.hexdocs.pm/code-anti-patterns.html) | missing | review |
| `ELIXIR-IDIOM-07` | Write `@spec` for public functions, `@type`/`@typep` for data, and `@behaviour`+`@impl true` for callbacks. Specs are documentation and Dialyzer input; the compiler's type system does not read them. [IDIOM-04] | Specs document contracts; Elixir 1.20 has no signature syntax yet. | SHOULD | [Typespecs](https://elixir.hexdocs.pm/typespecs.html); [v1.20 release](https://elixir-lang.org/blog/2026/06/03/elixir-v1-20-0-released/) | missing | review |
| `ELIXIR-IDIOM-08` | Resources: use `File.open/2` with a function or `try ... after`; use supervisors for long-lived processes; use `Task.Supervisor` for fire-and-forget; link or monitor everything you spawn. Pool and close DB, HTTP and file handles. [IDIOM-05] | `spawn` without supervision leaks processes silently. | MUST | [Process anti-patterns](https://elixir.hexdocs.pm/process-anti-patterns.html) (unsupervised processes); [Task.Supervisor](https://elixir.hexdocs.pm/Task.Supervisor.html) | go-only (goroutine leaks, GO_BEST §11-12) | review |
| `ELIXIR-IDIOM-09` | Strings are UTF-8 binaries; use `String.*` (grapheme-aware) for text, `binary` functions for bytes; build output with iodata/`IO.iodata_to_binary` in hot paths. Use `DateTime` with UTC and the `tz`/`tzdata` database for zones; never `NaiveDateTime` for instants. Do not compare floats or use them for money (use `Decimal`). [IDIOM-06] | Binary/grapheme confusion and naive times are classic bugs. | SHOULD | [Style guide](https://github.com/christopheradams/elixir_style_guide); [Library guidelines](https://elixir.hexdocs.pm/library-guidelines.html) | covered (consistency skill, GO_BEST §13) | review |
| `ELIXIR-IDIOM-10` | Naming: `snake_case` functions/atoms/variables, `CamelCase` modules, `?` suffix for booleans, `is_` prefix for guard-safe, bang for raising. Prefix library modules with the library name (no namespace trespassing). `@moduledoc` immediately after `defmodule`; order: `@moduledoc`, `@behaviour`, `use`, `import`, `require`, `alias`, attrs, `defstruct`, `@type`, `@callback`. Prefer `@doc`/`@moduledoc` to comments. [IDIOM-07] | Official/de-facto guide; `mix format` does not reorder directives. | SHOULD | [Style guide](https://github.com/christopheradams/elixir_style_guide); [Code anti-patterns](https://elixir.hexdocs.pm/code-anti-patterns.html) | missing | gate |
| `ELIXIR-IDIOM-11` | Metaprogramming: write a macro only when a function cannot do it; prefer `use` with an explicit `__using__` that only injects small `import`/`@behaviour`; never create atoms at runtime from external input. [IDIOM-08] | Macros hide control flow and break tooling; dynamic atoms are never garbage-collected. | AVOID | [Code anti-patterns](https://elixir.hexdocs.pm/code-anti-patterns.html) (dynamic atom creation); [EEF hardening: atom exhaustion](https://security.erlef.org/secure_coding_and_deployment_hardening/) | missing | review |
| `ELIXIR-IDIOM-12` | Prefer `Enum`/`Stream` pipelines; use `\|>` for chains of 2+ calls starting from a bare value, never a single pipe. Use `Stream` or `Flow` only for large/lazy data, otherwise eager `Enum`. [IDIOM-09] | Readability convention enforced by Credo. | SHOULD | [Style guide](https://github.com/christopheradams/elixir_style_guide) | missing | gate |

## 5. CONC: concurrency and the `-race` equivalent

**Loops race policy:** `mode = none`. The BEAM has no shared-memory data races between processes; the substitute check is `mix test --repeat-until-failure 5 --warnings-as-errors` (async test modules, random seed printed by ExUnit, re-run the failing seed with `--seed N`), and ETS/message-order bugs are reviewed.

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `ELIXIR-CONC-01` | Know the model: lightweight isolated processes with private heaps, preemptive scheduling per scheduler thread, message passing, links/monitors; no GIL. Use one process per unit of concurrent state or fault domain (session, connection, aggregate), not per function. [CONC-01] | Isolation is what makes "let it crash" sound. | MUST | [GenServer](https://elixir.hexdocs.pm/GenServer.html); [OTP design principles](https://erlang.org/doc/system/design_principles.html) | missing | review |
| `ELIXIR-CONC-02` | Share state by messaging a process, not by sharing memory. Use `Agent`/`GenServer` for small state, `Registry` for process lookup by key, `Task`/`Task.Supervisor` for concurrent work, `DynamicSupervisor` for per-entity processes, `:ets` (`:public` only with careful ownership) or `:persistent_term` for read-heavy shared data. [CONC-02] | Each primitive has a failure mode; choose the smallest. | SHOULD | [Registry](https://elixir.hexdocs.pm/Registry.html); [Task](https://elixir.hexdocs.pm/Task.html); [DynamicSupervisor](https://elixir.hexdocs.pm/DynamicSupervisor.html) | go-only (GO_BEST §11-12 concurrency, channels/mutexes) | review |
| `ELIXIR-CONC-03` | There are no data races, but there are logical races: (a) check-then-act across two calls to a process or DB, (b) ETS read-modify-write with `:public` tables (use `:ets.update_counter`/`insert_new`/`select_replace` or serialise through one owner), (c) message ordering is guaranteed only per sender-receiver pair, (d) `Registry.lookup` then `GenServer.call` can hit a dead pid. Make operations atomic in one process or one DB transaction. [CONC-03] | These are the races that remain on the BEAM. | MUST | [Registry](https://elixir.hexdocs.pm/Registry.html); [Process anti-patterns](https://elixir.hexdocs.pm/process-anti-patterns.html); [OTP design principles](https://erlang.org/doc/system/design_principles.html) | missing | review |
| `ELIXIR-CONC-04` | Race policy for loops: no sanitizer exists. Run ExUnit with `async: true` modules by default (random order, `--max-cases` default 2x cores), add `mix test --repeat-until-failure N` with N≥5 to the gate, print the seed, and re-run failures with the same `--seed`. Concuerror is not a default: its last release (0.21.0, 2020) targets OTP 20.3-23.3, so treat it as stale and advisory only. [CONC-04] | Randomized order + repeated runs surface order and ETS races deterministically via the seed. | MUST | [Mix.Tasks.Test](https://mix.hexdocs.pm/Mix.Tasks.Test.html) (`--repeat-until-failure` since 1.17, `--seed`, `--max-cases`); [ExUnit.Case async](https://ex-unit.hexdocs.pm/ExUnit.Case.html); [Concuerror](https://github.com/parapluu/Concuerror) (stale, advisory) | missing | gate |
| `ELIXIR-CONC-05` | Give every `GenServer.call` a deliberate timeout (default 5000 ms, caller exits on timeout); never block in `init/1` (use `handle_continue/2`); never run your own `receive` inside callbacks; do not rely on `terminate/2` for cleanup (use links/monitors). [CONC-05] | Blocking callbacks stall callers; `terminate/2` is not guaranteed. | MUST | [GenServer](https://elixir.hexdocs.pm/GenServer.html) | missing | review |
| `ELIXIR-CONC-06` | Do not send large terms across processes: extract only needed fields (`conn.remote_ip`, not `conn`). Beware mailbox growth: bound queues, use `:max_heap_size`/backpressure (GenStage/Broadway/Oban queues). [CONC-05] | Messages are copied; unbounded mailboxes cause OOM. | SHOULD | [Process anti-patterns](https://elixir.hexdocs.pm/process-anti-patterns.html) | missing | review |
| `ELIXIR-CONC-07` | Background work: use Oban (Postgres-backed) for durable jobs, `Task.Supervisor.async_nolink` for in-memory fire-and-forget, Broadway for ingestion; shut down gracefully via supervision order (releases stop apps in reverse start order on SIGTERM). [CONC-06] | Durable jobs survive restarts; fire-and-forget does not. | SHOULD | [Oban](https://oban.hexdocs.pm/Oban.html); [Mix.Tasks.Release](https://mix.hexdocs.pm/Mix.Tasks.Release.html) (graceful shutdown); [Broadway](https://hexdocs.pm/broadway/Broadway.html) | covered (jobs-and-state-machines skill) | review |
| `ELIXIR-CONC-08` | Supervision design: put children in dependency order; choose `:one_for_one` by default, `:rest_for_one` for ordered dependencies, tune `max_restarts`/`max_seconds`; separate "error kernel" supervisors from workers. [CONC-02] | A restart storm should escalate, not loop. | SHOULD | [Supervisor](https://elixir.hexdocs.pm/Supervisor.html); [OTP supervision](https://www.erlang.org/doc/system/sup_princ.html) | missing | review |
| `ELIXIR-CONC-09` | Process-per-request is free: Phoenix/Bandit already isolate each request in a process; do not add pools of GenServers to "limit concurrency" except for external resources (use `NimblePool`/`Poolboy`-style libs deliberately). When *not* to use a GenServer: pure logic, a single function call, config lookup, serialising CPU-bound work through one process. [CONC-01] | A global GenServer is a single-lane bottleneck. | AVOID | [GenServer](https://elixir.hexdocs.pm/GenServer.html); [Process anti-patterns](https://elixir.hexdocs.pm/process-anti-patterns.html) | missing | review |

## 6. TEST: testing

**Coverage recipe:** tool = ExCoveralls (`excoveralls`) over Erlang `:cover`; format = **lcov** (`mix coveralls.lcov`, experimental) written to `cover/lcov.info` (statement/line granularity); exact argv `MIX_ENV=test mix coveralls.lcov`; new-code coverage = loops parses `DA:<line>,<hits>` records and intersects with `git diff <base>` changed lines (≥70%). The built-in `mix test --cover` writes no lcov/Cobertura.

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `ELIXIR-TEST-01` | Unit and integration tests use ExUnit; add `ExUnit.start()` in `test/test_helper.exs`; run with `mix test`; for HTTP use `Phoenix.ConnTest`/`Plug.Test`, for LiveView `Phoenix.LiveViewTest`. Doctests (`doctest MyApp.Mod`) check examples in `@doc`. [TEST-01] | Built in; no extra dependency. | MUST | [ExUnit.Case](https://ex-unit.hexdocs.pm/ExUnit.Case.html); [ExUnit.DocTest](https://ex-unit.hexdocs.pm/ExUnit.DocTest.html) | go-only (testing skill is Go) | review |
| `ELIXIR-TEST-02` | Test files end in `_test.exs` under `test/` (mirroring `lib/`); only `*_test.exs` is loaded as tests (`test_load_filters`), though since 1.19 the default `test_pattern` is `*.{ex,exs}`. Helpers go in `test/support/*.ex` (compiled) and `test_helper.exs`. Globs: `test/**/*_test.exs`, `apps/*/test/**/*_test.exs`. [TEST-02] | The glob drives the loops test-edit ownership and the stop gate. | MUST | [Mix.Tasks.Test](https://mix.hexdocs.pm/Mix.Tasks.Test.html) (`test_paths`, `test_pattern`, `test_load_filters`) | go-only (`*_test.go`) | gate |
| `ELIXIR-TEST-03` | Group with `describe "fun/arity"`; name tests as sentences; one behaviour per `test`; use `setup`/named setups (no nested describes), `@tag`/`@moduletag`/`@describetag` for filters; generate cases at compile time with `for` comprehensions for table-style tests. [TEST-03] | Describe blocks cannot nest; tags drive `--include/--exclude`. | SHOULD | [ExUnit.Case](https://ex-unit.hexdocs.pm/ExUnit.Case.html) | go-only (table-driven) | review |
| `ELIXIR-TEST-04` | Prefer fakes/in-memory adapters; use Mox with `@behaviour` ports (explicit contracts, `verify_on_exit!`) and `Mox.allow` for other processes; do not mock what you do not own (wrap third-party clients in a behaviour first). Set `async: true` only with `Mox` in private mode. [TEST-04] | Mox enforces the behaviour; global stubs break async. | SHOULD | [Mox](https://mox.hexdocs.pm/Mox.html) | covered (fakes over mocks, skills/testing neutral core) | review |
| `ELIXIR-TEST-05` | Database tests use the Ecto SQL Sandbox: `pool: Ecto.Adapters.SQL.Sandbox` in `config/test.exs`, `Sandbox.mode(Repo, :manual)` at the end of `test_helper.exs`, `Sandbox.start_owner!(Repo, shared: not tags[:async])` in the case template, `Sandbox.allow/3` for other processes. Async is safe on PostgreSQL only (MySQL risks deadlocks). Do not mock the DB. [TEST-05] | Each test owns a transaction rolled back at exit. | MUST | [Ecto SQL Sandbox](https://ecto-sql.hexdocs.pm/Ecto.Adapters.SQL.Sandbox.html); [Testing with Ecto](https://hexdocs.pm/ecto/testing-with-ecto.html) | missing | review |
| `ELIXIR-TEST-06` | Coverage: run `MIX_ENV=test mix coveralls.lcov` with `test_coverage: [tool: ExCoveralls]` and `cli/0` `preferred_envs` (`coveralls*: :test`) in `mix.exs`; output `cover/lcov.info`; also `coveralls.json` → `cover/excoveralls.json`, `coveralls.cobertura` → `cover/cobertura.xml`, `coveralls.html`, `coveralls.xml`. Set `coveralls.json` `minimum_coverage: 0` so loops owns the threshold. Exclude via `skip_files` regex and `# coveralls-ignore-start/-stop/-next-line`. The built-in `mix test --cover` (`:test_coverage` `:output` default `cover`, `:summary` threshold default 90, `:ignore_modules`, `:export`) prints a summary and writes only per-module HTML (`cover/Elixir.<Mod>.html`; spot-run on 1.20.5). [TEST-06] | The built-in tool has no lcov/Cobertura; excoveralls adds them. Line-based only, no branch coverage. | MUST | [excoveralls](https://github.com/parroty/excoveralls); [Mix.Tasks.Test](https://mix.hexdocs.pm/Mix.Tasks.Test.html) | go-only (gocover parser) | gate |
| `ELIXIR-TEST-07` | New-code coverage is not provided natively: loops computes it from `cover/lcov.info` (`SF:`, `DA:`) against `git diff -U0 <base>`; or use `diff-cover` (Python) on the lcov/Cobertura file. Ignore `test/**`, `priv/repo/migrations/**` and generated code. [TEST-07] | Needs a diff-intersection implementation. | MUST | [excoveralls](https://github.com/parroty/excoveralls) (paths); [diff-cover](https://github.com/Bachmann1234/diff_cover) (reads lcov and Cobertura). `SF:` paths in `cover/lcov.info` are absolute, so loops must normalise them to project-relative before matching the diff | go-only (internal/application/gate/coverage.go) | gate |
| `ELIXIR-TEST-08` | Property tests with StreamData (`use ExUnitProperties`, `property`/`check all`, 100 runs by default, shrinking up to 100 steps, seed = ExUnit seed). Use for parsers, serializers, state machines and invariants; keep deterministic example tests too. Mutation testing is not mainstream; treat as optional. [TEST-08] | Shrinking finds minimal counterexamples; `max_runs` can be raised in CI via config. | SHOULD | [ExUnitProperties](https://stream-data.hexdocs.pm/ExUnitProperties.html) | go-only (fuzz/bench in testing skill) | review |
| `ELIXIR-TEST-09` | Avoid flakiness: keep `async: true`; no global state in async modules (`Application.put_env`, working dir, named processes) or mark `async: false`; use `start_supervised!/1` (supervised per test), `tmp_dir` tag, `on_exit/1`, `assert_receive` with timeouts instead of `Process.sleep`; inject clocks; keep the printed seed. [TEST-09] | ExUnit runs modules concurrently in random order; the docs warn against global state. | MUST | [ExUnit.Case](https://ex-unit.hexdocs.pm/ExUnit.Case.html); [Mix.Tasks.Test](https://mix.hexdocs.pm/Mix.Tasks.Test.html) | covered (consistency skill: clock injection) | review |
| `ELIXIR-TEST-10` | Fast loop: `mix test path/to/file_test.exs:LINE`, `mix test --stale`, `mix test --failed`, `mix test --trace` (forces max-cases 1), `--slowest N`; watch mode via `mix_test_watch` (`mix test.watch`) or `mix_test_interactive`. Use `--partitions N` with `MIX_TEST_PARTITION` in CI and merge coverage with `mix test.coverage`. [TEST-10] | Single-file runs are the TDD loop; partitions parallelise CI. | SHOULD | [Mix.Tasks.Test](https://mix.hexdocs.pm/Mix.Tasks.Test.html); [mix-test.watch](https://github.com/lpil/mix-test.watch) | go-only | review |
| `ELIXIR-TEST-11` | Use ExMachina or hand-written fixture modules in `test/support/` for data; one factory per schema. Never share fixtures that mutate. [TEST-03] | Fixtures keep tests terse. | SHOULD | [ExMachina](https://hexdocs.pm/ex_machina/ExMachina.html) | missing | review |

## 7. PERF: performance and profiling

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `ELIXIR-PERF-01` | Benchmark with Benchee; compare runs statistically (`Benchee.run/2` with `time:`, `memory_time:`; save/compare with `save:`/`load:`); never benchmark in `dev` env or with `iex`. [PERF-01] | Benchee reports statistics and memory. | SHOULD | [Benchee](https://benchee.hexdocs.pm/Benchee.html) | go-only (benchstat) | review |
| `ELIXIR-PERF-02` | Profile with `mix profile.tprof` (OTP 27+) / `mix profile.eprof` / `mix profile.fprof`; for live systems use `:observer`, `:recon`, `:erlang.trace` via `recon_trace`, and Phoenix `telemetry` metrics. [PERF-02] | Native BEAM tooling, no restart. | SHOULD | [Profile.Tprof](https://mix.hexdocs.pm/Mix.Tasks.Profile.Tprof.html); [Profile.Eprof](https://mix.hexdocs.pm/Mix.Tasks.Profile.Eprof.html); [Profile.Fprof](https://mix.hexdocs.pm/Mix.Tasks.Profile.Fprof.html) | go-only (pprof) | review |
| `ELIXIR-PERF-03` | Tune the VM only after measuring: scheduler count (`+S`), `:max_heap_size`, ETS `read_concurrency`/`write_concurrency`, and release flags in `rel/vm.args.eex`. Startup is fast; use releases (no JIT warm-up issue; BEAM JIT is on by default from OTP 24). [PERF-03] | Defaults are good; GC is per process. | SHOULD | [Mix.Tasks.Release](https://mix.hexdocs.pm/Mix.Tasks.Release.html) | missing | review |
| `ELIXIR-PERF-04` | Avoid: `++` on large lists in loops (use prepend + reverse or iodata), `Enum.at/length` on lists in loops, calling `String.to_atom` on input, per-request `Process.sleep`, N+1 Ecto queries, large messages, unnecessary `Enum` over `Stream` for huge data, single global GenServer bottlenecks. [PERF-04] | Common BEAM hot-path traps. | AVOID | [Process anti-patterns](https://elixir.hexdocs.pm/process-anti-patterns.html); [Code anti-patterns](https://elixir.hexdocs.pm/code-anti-patterns.html) | missing | review |

## 8. SEC: security

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `ELIXIR-SEC-01` | SQL: use Ecto queries with `^` interpolation or `Repo.query/3` with parameters; use `fragment("... ?", ^x)`; never string-interpolate into `fragment/1` or `Repo.query`. Shell: `System.cmd(cmd, args_list)` (no shell) instead of `:os.cmd/1`. [SEC-01] | Parameterisation prevents injection. | MUST | [EEF hardening: external executables](https://security.erlef.org/secure_coding_and_deployment_hardening/); [Ecto.Repo](https://ecto.hexdocs.pm/Ecto.Repo.html) | go-only (GO_BEST §38) | gate |
| `ELIXIR-SEC-02` | Never call `binary_to_term/1` on untrusted data (use `[:safe]` and still prefer JSON/Protobuf); never `Code.eval_string`/`Code.eval_quoted` on input; avoid YAML loaders that create atoms. [SEC-02] | `binary_to_term` creates atoms and can exhaust memory. | MUST | [EEF hardening: serialisation](https://security.erlef.org/secure_coding_and_deployment_hardening/) | missing | gate |
| `ELIXIR-SEC-03` | Prevent atom exhaustion: use `String.to_existing_atom/1` or an explicit map from string to atom; never `String.to_atom/1` on external input. [SEC-02] | Over a third of EEF CNA CVEs involve atom exhaustion (as of 2026-11). | MUST | [Code anti-patterns](https://elixir.hexdocs.pm/code-anti-patterns.html); [EEF Security WG](https://security.erlef.org/) | missing | gate |
| `ELIXIR-SEC-04` | Web: keep `protect_from_forgery`/`Plug.CSRFProtection` and `put_secure_browser_headers` in the `:browser` pipeline; HEEx escapes by default (never `raw/1` on user input); validate paths with `Path.safe_relative/2` before file access; validate redirect targets; restrict CORS explicitly (`Corsica`/`CORSPlug`) and SSRF-check outbound URLs. [SEC-03] | Phoenix defaults are safe until bypassed. | MUST | [Phoenix security](https://phoenix.hexdocs.pm/security.html); [Plug.CSRFProtection](https://plug.hexdocs.pm/Plug.CSRFProtection.html) | missing | gate |
| `ELIXIR-SEC-05` | Secrets come from the environment in `config/runtime.exs`, never `config.exs`; never commit `SECRET_KEY_BASE`; generate with `mix phx.gen.secret`. Hash passwords with `argon2_elixir`, `bcrypt_elixir` or `pbkdf2_elixir` (as `mix phx.gen.auth` does); compare secrets with `Plug.Crypto.secure_compare/2`; random tokens from `:crypto.strong_rand_bytes/1`, not `:rand`. [SEC-04] | `:rand` is not cryptographically secure; compile-time config leaks into the release. | MUST | [Phoenix releases](https://phoenix.hexdocs.pm/releases.html); [EEF: timing attacks, crypto](https://security.erlef.org/secure_coding_and_deployment_hardening/) | missing | gate |
| `ELIXIR-SEC-06` | Validate at the adapter boundary with Ecto changesets (also schemaless `Ecto.Changeset.cast/4` with embedded schemas) or `NimbleOptions`; never pass raw `params` into the domain. Use strong params whitelisting via `cast(attrs, [:field])`. [SEC-05] | Cast lists permitted fields only. | MUST | [Ecto.Changeset](https://ecto.hexdocs.pm/Ecto.Changeset.html) | covered (HEX_ARCH validation at boundaries, GO_BEST §43) | review |
| `ELIXIR-SEC-07` | Supply chain: commit `mix.lock` (Hex stores checksums; `mix deps.get` verifies), review `mix deps.update` diffs, prefer Hex packages with owners and downloads, never use `git:` deps without a pinned `ref:`. Enable `HEX_NO_VERIFY_REPO_ORIGIN` only intentionally. [SEC-06] | Lock + checksum; Hex.pm completed a third-party audit. | MUST | [Hex tasks](https://hex.hexdocs.pm/Mix.Tasks.Hex.html); [EEF Security WG](https://security.erlef.org/) | missing | review |
| `ELIXIR-SEC-08` | Run Sobelow: `mix sobelow --exit medium` (`--threshold`, `--format json`, `--skip` with `# sobelow_skip`, `--save-config` → `.sobelow-conf`). It targets Phoenix; on non-Phoenix projects results are limited. [SEC-07] | Elixir/Phoenix SAST (SQLi, XSS, command injection, traversal, insecure config). | SHOULD | [Sobelow](https://github.com/nccgroup/sobelow) | missing | gate |
| `ELIXIR-SEC-09` | Deployment hardening: do not expose EPMD/distribution ports, set `RELEASE_COOKIE` secret, TLS with `verify_peer` and CA store (`:public_key.cacerts_get/0`), disable crash dumps in production. [SEC-04] | Distributed Erlang is full remote code execution if reachable. | MUST | [EEF hardening](https://security.erlef.org/secure_coding_and_deployment_hardening/) (distribution, ssl, crash dumps) | missing | review |

## 9. DATA: persistence

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `ELIXIR-DATA-01` | Use Ecto (query DSL + schemas + changesets) with `Repo` as the only DB gateway; fall back to `Repo.query` or `fragment` for SQL the DSL cannot express. Ecto is not an ORM: no identity map, no lazy loading; associations must be preloaded explicitly. Keep schemas inside the context's adapter layer when going hexagonal, and return domain structs. [DATA-01] | Explicit data access avoids ORM surprises. | SHOULD | [Ecto.Repo](https://ecto.hexdocs.pm/Ecto.Repo.html); [Phoenix contexts](https://phoenix.hexdocs.pm/contexts.html) | covered (HEX_ARCH persistence) | review |
| `ELIXIR-DATA-02` | Migrations: `mix ecto.gen.migration`, timestamped files in `priv/repo/migrations/`; prefer `change/0`; use `up/down` for irreversible operations (`modify/3` needs `:from`; `remove/3` not `remove/1`). Deploy via `bin/my_app eval "MyApp.Release.migrate"` (`mix phx.gen.release`). Ecto locks `schema_migrations`. [DATA-02] | Reversibility must be explicit. | MUST | [Ecto.Migration](https://ecto-sql.hexdocs.pm/Ecto.Migration.html); [Phoenix releases](https://phoenix.hexdocs.pm/releases.html) | covered (consistency skill: migrations) | review |
| `ELIXIR-DATA-03` | Zero-downtime: add columns nullable then backfill; create indexes `concurrently: true` with `@disable_ddl_transaction true` and `migration_lock: :pg_advisory_lock`; add constraints with `validate: false` then validate later; split rename/drop across releases. Follow Safe Ecto Migrations (and consider `excellent_migrations`/`strong_migrations`-style linters). [DATA-02] | Avoids table locks. | MUST | [Ecto.Migration](https://ecto-sql.hexdocs.pm/Ecto.Migration.html); [Safe Ecto Migrations](https://github.com/fly-apps/safe-ecto-migrations) | missing | review |
| `ELIXIR-DATA-04` | Transactions live in the application layer: use `Repo.transact/2` (function returning `{:ok, _}`/`{:error, _}`) for ordinary flows and `Ecto.Multi` for dynamic sets of operations; never call `Repo` from controllers. Default isolation is the DB default (read committed on PostgreSQL); use `SELECT ... FOR UPDATE` (`lock: "FOR UPDATE"`) for read-modify-write. [DATA-03] | Multi gives named steps and `{:error, op, value, changes}`; function form is simpler. | MUST | [Ecto.Multi](https://ecto.hexdocs.pm/Ecto.Multi.html) | covered (HEX_ARCH §transactions, consistency skill) | review |
| `ELIXIR-DATA-05` | Traps: always `preload`/`join` associations (`Ecto.Association.NotLoaded` otherwise); avoid N+1 in LiveView/templates; select only needed fields; paginate (keyset); never `Repo.all` unbounded; use `Repo.insert_all` for bulk (no changesets/timestamps autogen unless provided). [DATA-04] | Lazy loading does not exist: unloaded associations raise or look empty. | MUST | [Ecto.Repo](https://ecto.hexdocs.pm/Ecto.Repo.html) | missing | review |
| `ELIXIR-DATA-06` | Pools: set `pool_size` ≥ expected concurrency across nodes; tune `queue_target`/`queue_interval`, `timeout`, `ownership_timeout`; set statement timeouts; use PgBouncer in transaction mode only with `prepare: :unnamed`. Oban needs its own connections. [DATA-05] | DBConnection queues; defaults can starve under load. | SHOULD | [Ecto SQL Sandbox](https://ecto-sql.hexdocs.pm/Ecto.Adapters.SQL.Sandbox.html) (`ownership_timeout`); [Ecto.Repo](https://ecto.hexdocs.pm/Ecto.Repo.html) | missing | review |
| `ELIXIR-DATA-07` | Jobs with Oban: enqueue in the same transaction as the data change (`Oban.insert/2` in a Multi), make workers idempotent, use `unique:` options deliberately, test with `testing: :manual` or `:inline` and `Oban.Testing` (`assert_enqueued`, `perform_job`). [DATA-03] | Transactional outbox for free, but at-least-once. | MUST | [Oban testing](https://oban.hexdocs.pm/testing.html); [Oban unique jobs](https://oban.hexdocs.pm/unique_jobs.html); [Oban testing workers](https://oban.hexdocs.pm/testing_workers.html) | covered (jobs skill, idempotency GO_BEST §46) | review |

## 10. API: APIs and integration

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `ELIXIR-API-01` | HTTP stack: Phoenix on Bandit (default adapter in 1.8) over Plug; pipeline = endpoint plugs → router pipelines → controller; keep controllers thin and call context functions only; use `action_fallback` for error tuples. [API-01] | Plug is the composable request lifecycle. | SHOULD | [Phoenix](https://phoenix.hexdocs.pm/Phoenix.html); [Plug](https://plug.hexdocs.pm/Plug.html); [Bandit](https://bandit.hexdocs.pm/Bandit.html) | missing | review |
| `ELIXIR-API-02` | Contracts: code-first with `open_api_spex` (OpenAPI from controllers/schemas, `cast`/`validate` plugs) or OpenAPI-first; GraphQL with Absinthe; gRPC with `grpc` + `protobuf`; version via URL or header and keep old versions for a deprecation window. [API-02] | Specs stay in sync only when generated or validated. | SHOULD | [Phoenix](https://phoenix.hexdocs.pm/Phoenix.html) (router/scope); [open_api_spex](https://hex.pm/packages/open_api_spex) | missing | review |
| `ELIXIR-API-03` | Errors at the edge: map context errors to status codes in one place (fallback controller / `Phoenix.Controller` error views) and return RFC 9457 problem details; changeset errors → 422 via `Ecto.Changeset.traverse_errors/2`. [API-03] | Consistent API errors. | SHOULD | [Ecto.Changeset](https://ecto.hexdocs.pm/Ecto.Changeset.html); [Phoenix](https://phoenix.hexdocs.pm/Phoenix.html) | covered (HEX_ARCH error taxonomy) | review |
| `ELIXIR-API-04` | Resilience: set explicit timeouts and retries (`Req` retry with backoff), circuit breakers (`fuse`), rate limiting (`Hammer`/`PlugAttack`), and idempotency keys for non-idempotent POSTs. [API-04] | Remote calls fail; processes make timeouts cheap. | SHOULD | [Req](https://req.hexdocs.pm/Req.html) | covered (consistency/jobs skills, GO_BEST §46) | review |
| `ELIXIR-API-05` | Messaging: Broadway for Kafka/SQS/RabbitMQ consumers (ack after processing, batch, concurrency per stage); outbox via Oban/transactional inserts; handle at-least-once with idempotent handlers. [API-05] | Broadway provides backpressure and graceful shutdown. | SHOULD | [Broadway](https://hexdocs.pm/broadway/Broadway.html); [Oban](https://oban.hexdocs.pm/Oban.html) | covered (jobs skill, outbox) | review |
| `ELIXIR-API-06` | HTTP client: use `Req` (built on Finch/Mint) with a shared named client, connection pooling and per-request `receive_timeout`; avoid `HTTPoison` for new code (older Hackney-based). [API-06] | Req is the current recommended client in the Phoenix docs. | SHOULD | [Req](https://req.hexdocs.pm/Req.html); [Phoenix contexts](https://phoenix.hexdocs.pm/contexts.html) | missing | review |

## 11. OBS: observability

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `ELIXIR-OBS-01` | Log with `Logger` (8 levels, structured reports as maps/keyword lists, `Logger.metadata/1` for request ids); configure `:default_formatter` `format`/`metadata`; for JSON use a library ([`logger_json`](https://hex.pm/packages/logger_json)) since the built-in formatter has no JSON option; purge debug at compile time with `compile_time_purge_matching`; never log secrets or PII. [OBS-01] | Metadata is not printed unless configured. | MUST | [Logger](https://logger.hexdocs.pm/Logger.html) | go-only (GO_BEST §24) | review |
| `ELIXIR-OBS-02` | Emit metrics via `:telemetry.execute/3` with event names `[:app, :thing, :stop]`; use `:telemetry.span/3` for timed work; attach handlers once; keep handlers fast (they run synchronously in the caller); export with `telemetry_metrics` + a reporter (Prometheus via `peep`/`telemetry_metrics_prometheus`). [OBS-02] | Telemetry is the shared bus for Phoenix, Ecto and Oban. | SHOULD | [telemetry](https://telemetry.hexdocs.pm/readme.html); [Phoenix telemetry](https://phoenix.hexdocs.pm/telemetry.html) | missing | review |
| `ELIXIR-OBS-03` | Tracing with OpenTelemetry: traces are Stable, metrics and logs are Development for Erlang/Elixir (as of 2026-11). Use `opentelemetry_api`, `opentelemetry`, `opentelemetry_exporter` plus contrib `opentelemetry_phoenix`, `opentelemetry_ecto`, `opentelemetry_bandit`; propagate context across `Task`s by `OpenTelemetry.Ctx`. [OBS-03] | Context is per process, so spawned tasks need explicit propagation. | SHOULD | [OTel Erlang/Elixir](https://opentelemetry.io/docs/languages/erlang/) | missing | review |
| `ELIXIR-OBS-04` | Health: expose `/healthz` (liveness, no dependencies) and `/readyz` (DB and queues) through a plug before the router; Phoenix provides no built-in endpoint. Return 503 while draining. [OBS-04] | Orchestrators restart on liveness failure. | SHOULD | [Plug](https://plug.hexdocs.pm/Plug.html) | missing | review |

## 12. OPS: configuration, packaging and deployment

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `ELIXIR-OPS-01` | Config: `config/config.exs` is build-time; `config/runtime.exs` runs at boot (must `import Config`, no `import_config`, no `Mix`) and reads `System.get_env`/`System.fetch_env!` for secrets and URLs; avoid `compile_env` for deploy-time values. [OPS-01] | Compile-time config is baked into the release. | MUST | [Mix.Tasks.Release](https://mix.hexdocs.pm/Mix.Tasks.Release.html); [Config](https://elixir.hexdocs.pm/Config.html) | go-only (GO_BEST §28) | review |
| `ELIXIR-OPS-02` | Package with `MIX_ENV=prod mix release` (output `_build/prod/rel/<name>`, `bin/<name> start\|daemon\|eval\|rpc\|remote\|stop`); generate with `mix phx.gen.release --docker` (multi-stage: builder, slim runner running `/app/bin/server`); the release must be built on the target OS/libc; use a pinned `hexpm/elixir` builder and Debian slim runner. [OPS-02] | Self-contained: no Erlang/Elixir needed at runtime. | MUST | [Mix.Tasks.Release](https://mix.hexdocs.pm/Mix.Tasks.Release.html); [Phoenix releases](https://phoenix.hexdocs.pm/releases.html) | missing | review |
| `ELIXIR-OPS-03` | Native/AOT: n/a; the BEAM has a JIT only. Use `mix release` or Burrito/Bakeware-style single binaries only for CLIs. [OPS-03] | No NativeAOT story for servers. | SHOULD | [Mix.Tasks.Release](https://mix.hexdocs.pm/Mix.Tasks.Release.html) | n/a | review |
| `ELIXIR-OPS-04` | Lifecycle: SIGTERM/SIGINT or `bin/<name> stop` stops each application and its supervision tree in reverse start order; order children so ingress (Endpoint) starts last and stops first; Oban/Broadway drain on shutdown. Set `shutdown:` on children. [OPS-04] | Graceful drain of in-flight work. | MUST | [Mix.Tasks.Release](https://mix.hexdocs.pm/Mix.Tasks.Release.html); [Supervisor](https://elixir.hexdocs.pm/Supervisor.html) | go-only (GO_BEST graceful shutdown) | review |
| `ELIXIR-OPS-05` | For the loops runspace set `MIX_BUILD_ROOT=<run>/build` (preferred over `MIX_BUILD_PATH`, which skips env/target segments and overrides ROOT) and keep `deps/`, `MIX_HOME` and `HEX_HOME` shared: a fresh `MIX_HOME` loses the installed Hex/rebar archives, and a fresh `HEX_HOME` re-downloads packages. Cost: a per-run `MIX_BUILD_ROOT` recompiles every dependency each run (not a re-download, but the main cost). Also `MIX_OS_CONCURRENCY_LOCK` stays on (default) so parallel runs do not corrupt a shared build. [OPS-05] | Per-run build path avoids stale artifacts without re-downloading deps. | MUST | [Mix env vars](https://mix.hexdocs.pm/Mix.html); [Hex env vars](https://hex.hexdocs.pm/Mix.Tasks.Hex.html) | go-only (GOCACHE hook go-env) | hook |
| `ELIXIR-OPS-06` | Artifacts left behind: `_build/`, `deps/`, `cover/`, `doc/`, `tmp/` (ExUnit `tmp_dir`), `erl_crash.dump`, `*.beam`, `*.ez`, `priv/plts/*.plt`, `.elixir_ls/`, `priv/static/` (assets); cleanup must not delete `deps/` or `mix.lock`. [OPS-06] | Drives the loops cleanup list. | MUST | [Mix env vars](https://mix.hexdocs.pm/Mix.html); [Dialyxir](https://github.com/jeremyjh/dialyxir) | go-only (`*.test`, `*.prof` artifact globs) | hook |

## 13. DOC: documentation and review

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `ELIXIR-DOC-01` | API docs use `@moduledoc`/`@doc` (Markdown) and `@spec`, built with ExDoc (`mix docs`); `@moduledoc false` hides internals; add doctests (`iex> ... `) to public functions and run them with `doctest`. [DOC-01] | Docs and examples stay verifiable. | SHOULD | [Writing documentation](https://elixir.hexdocs.pm/writing-documentation.html); [ExDoc](https://ex-doc.hexdocs.pm/readme.html); [ExUnit.DocTest](https://ex-unit.hexdocs.pm/ExUnit.DocTest.html) | go-only (doc comments GO_BEST §26) | review |
| `ELIXIR-DOC-02` | README, ADRs and CHANGELOG as in the loops spec; for libraries README includes install snippet, usage, and `mix docs` link. [DOC-02] | Hex and HexDocs surface the README. | SHOULD | [Hex publish](https://hex.hexdocs.pm/Mix.Tasks.Hex.Publish.html) | covered (documentation skill, HEX_ARCH ADRs) | review |
| `ELIXIR-DOC-03` | Reviewers check first: unsupervised/unnecessary processes, `String.to_atom`, blocking `GenServer.call`, missing preload/N+1, queries outside contexts, `raw/1`, secrets in `config.exs`, `compile_env` misuse, missing `async`/Sandbox, mixed return shapes. [DOC-03] | Highest-yield Elixir review items. | SHOULD | [Process anti-patterns](https://elixir.hexdocs.pm/process-anti-patterns.html); [Code anti-patterns](https://elixir.hexdocs.pm/code-anti-patterns.html) | go-only (GO_BEST §51-52) | review |
| `ELIXIR-DOC-04` | Credo's `Credo.Check.Readability.ModuleDoc` and `Credo.Check.Design.TagTODO` surface missing docs/TODOs; enable them in `.credo.exs`. [DOC-01] | Mechanical enforcement of doc presence. | SHOULD | [Credo config](https://credo.hexdocs.pm/config_file.html) | missing | gate |

## 14. FW: frameworks

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `ELIXIR-FW-01` | Frameworks: Phoenix 1.8.x (+ LiveView, Plug, Bandit) dominates web; Ecto for data; Oban for jobs; Broadway for ingestion; Absinthe for GraphQL; Commanded for CQRS/ES; Nerves for embedded. Detect Phoenix via `{:phoenix, ...}` in `mix.exs` and `lib/*_web/`. [FW-01] | Usage evidence: Phoenix docs at 1.8.16, Ecto 3.14.3, Oban, Broadway docs current. | MUST | [Phoenix](https://phoenix.hexdocs.pm/Phoenix.html); [Oban](https://oban.hexdocs.pm/Oban.html); [Broadway](https://hexdocs.pm/broadway/Broadway.html) | missing | review |
| `ELIXIR-FW-02` | Follow Phoenix conventions when present: `lib/<app>_web/` for web, contexts under `lib/<app>/`, generators (`mix phx.gen.context`, `phx.gen.live`, `phx.gen.auth`), `test/support/{data_case,conn_case}.ex`; project rules win. Conflicts with hexagonal: generated contexts call `Repo` and return Ecto schemas, so the loops layout moves schemas into adapters and adds ports. [FW-02] | Generators produce the Phoenix layout; the loops layout is an opt-in refinement. | SHOULD | [Phoenix contexts](https://phoenix.hexdocs.pm/contexts.html); [Phoenix scopes](https://phoenix.hexdocs.pm/scopes.html) | covered (project rules win, WF §1) | review |
| `ELIXIR-FW-03` | Do not put business logic in LiveView callbacks, controllers or schemas; LiveViews hold UI state only and call contexts. Avoid `assign` of large structs (sent over the socket diff). [FW-03] | LiveView state is per-process memory. | MUST | [Phoenix contexts](https://phoenix.hexdocs.pm/contexts.html); [Process anti-patterns](https://elixir.hexdocs.pm/process-anti-patterns.html) | missing | review |
| `ELIXIR-FW-04` | Ecto changesets are a validation layer, not the domain: keep changesets in the adapter schema when hexagonal; keep cross-field business invariants in domain functions. [FW-02] | Changesets mix casting, validation and persistence concerns. | SHOULD | [Ecto.Changeset](https://ecto.hexdocs.pm/Ecto.Changeset.html) | missing | review |
| `ELIXIR-FW-05` | Phoenix footguns: Plug order matters in the endpoint; `runtime.exs` vs `config.exs`; assets build step required in releases (`mix assets.deploy`); `PHX_SERVER`/`bin/server` needed to start the web server in a release; verify `check_origin`. [FW-03] | Common deployment surprises. | SHOULD | [Phoenix releases](https://phoenix.hexdocs.pm/releases.html) | missing | review |

## Top 10 footguns

1. **Process as code organization.** Bad: a GenServer wrapping pure functions. Fix: plain module; use a process only for state/concurrency/isolation. ([Process anti-patterns](https://elixir.hexdocs.pm/process-anti-patterns.html))
2. **Dynamic atoms.** Bad: `String.to_atom(params["role"])`. Fix: `String.to_existing_atom/1` or an explicit map. ([Code anti-patterns](https://elixir.hexdocs.pm/code-anti-patterns.html))
3. **Unsupervised processes.** Bad: `spawn/1`, `Task.start/1` for long-lived work. Fix: `Task.Supervisor`/child spec in the tree. ([Process anti-patterns](https://elixir.hexdocs.pm/process-anti-patterns.html))
4. **Default 5 s call timeout & blocking `init/1`.** Bad: slow work in `init/1` or unbounded `GenServer.call`. Fix: `handle_continue/2`, explicit timeouts. ([GenServer](https://elixir.hexdocs.pm/GenServer.html))
5. **Non-assertive access.** Bad: `map[:id]` for a required key. Fix: `map.id` or pattern match. ([Code anti-patterns](https://elixir.hexdocs.pm/code-anti-patterns.html))
6. **Async tests with global state.** Bad: `async: true` with `Application.put_env`/named process. Fix: `async: false` or per-test isolation; re-run with `--seed N`. ([ExUnit.Case](https://ex-unit.hexdocs.pm/ExUnit.Case.html))
7. **Sandbox misuse.** Bad: spawned process without `Sandbox.allow/3`, or async on MySQL. Fix: `start_owner!`, `allow`, Postgres. ([Sandbox](https://ecto-sql.hexdocs.pm/Ecto.Adapters.SQL.Sandbox.html))
8. **Unsafe migrations.** Bad: `create index(:t, [:c])` on a big table in a transaction. Fix: `concurrently: true` + `@disable_ddl_transaction true` + advisory lock. ([Ecto.Migration](https://ecto-sql.hexdocs.pm/Ecto.Migration.html))
9. **Secrets and config at compile time.** Bad: `System.get_env` in `config.exs`. Fix: `config/runtime.exs`. ([Mix.Tasks.Release](https://mix.hexdocs.pm/Mix.Tasks.Release.html))
10. **Sending whole structs between processes.** Bad: `Task.async(fn -> use(conn) end)`. Fix: extract needed fields first. ([Process anti-patterns](https://elixir.hexdocs.pm/process-anti-patterns.html))

## 15. LOOPS: tooling data

The `env` key on a check is an extension of the TAXONOMY schema (no field exists for per-check environment); see Open questions.

```json
{
  "id": "elixir",
  "name": "Elixir",
  "versions": {"stable": "1.20", "lts": null, "as_of": "2026-11"},
  "detect": [{"file": "mix.exs"}],
  "source_globs": ["lib/**/*.ex", "apps/*/lib/**/*.ex"],
  "test_globs": ["test/**/*_test.exs", "apps/*/test/**/*_test.exs"],
  "generated": {"globs": ["**/*.pb.ex", "priv/repo/migrations/**", "_build/**", "deps/**"], "header_regex": null},
  "checks": [
    {"name": "deps", "kind": "build", "argv": ["mix", "deps.get", "--check-locked"], "required": true, "tool": "mix"},
    {"name": "compile", "kind": "build", "argv": ["mix", "compile", "--warnings-as-errors"], "required": true, "tool": "mix", "env": {"MIX_ENV": "test"}},
    {"name": "format", "kind": "format", "argv": ["mix", "format", "--check-formatted"], "required": false, "tool": "mix"},
    {"name": "credo", "kind": "lint", "argv": ["mix", "credo", "--strict"], "required": false, "tool": "credo", "install": "add {:credo, \"~> 1.7\", only: [:dev, :test], runtime: false} to mix.exs deps"},
    {"name": "xref-cycles", "kind": "arch", "argv": ["mix", "xref", "graph", "--format", "cycles", "--fail-above", "0"], "required": false, "tool": "mix"},
    {"name": "dialyzer", "kind": "typecheck", "argv": ["mix", "dialyzer"], "required": false, "tool": "dialyxir", "install": "add {:dialyxir, \"~> 1.4\", only: [:dev, :test], runtime: false} to mix.exs deps"},
    {"name": "test", "kind": "test", "argv": ["mix", "coveralls.lcov", "--warnings-as-errors"], "required": true, "tool": "excoveralls", "env": {"MIX_ENV": "test"}, "install": "add {:excoveralls, \"~> 0.18\", only: :test} and test_coverage: [tool: ExCoveralls] to mix.exs"},
    {"name": "test-repeat", "kind": "test", "argv": ["mix", "test", "--repeat-until-failure", "5", "--warnings-as-errors"], "required": false, "tool": "mix", "env": {"MIX_ENV": "test"}},
    {"name": "deps-audit", "kind": "vuln", "argv": ["mix", "deps.audit"], "required": false, "tool": "mix_audit", "install": "add {:mix_audit, \"~> 2.1\", only: [:dev, :test], runtime: false} to mix.exs deps"},
    {"name": "hex-audit", "kind": "vuln", "argv": ["mix", "hex.audit"], "required": false, "tool": "hex"},
    {"name": "sobelow", "kind": "lint", "argv": ["mix", "sobelow", "--exit", "medium"], "required": false, "tool": "sobelow", "install": "add {:sobelow, \"~> 0.13\", only: [:dev, :test], runtime: false} to mix.exs deps"}
  ],
  "test_commands": [
    {"argv_prefix": ["mix", "test"]},
    {"argv_prefix": ["mix", "coveralls"]},
    {"argv_prefix": ["mix", "coveralls.lcov"]},
    {"argv_prefix": ["mix", "coveralls.json"]},
    {"argv_prefix": ["mix", "test.watch"]}
  ],
  "race": {"mode": "none", "flags": [], "env": null, "reason": "BEAM processes share no memory, so there are no data races to detect; ETS and message-ordering races remain and have no sanitizer", "substitute": "mix test --repeat-until-failure 5 --warnings-as-errors with async: true modules, random order and the printed seed (re-run with --seed N)"},
  "coverage": {"format": "lcov", "path": "cover/lcov.info", "granularity": "line"},
  "post_edit": {"format_file": ["mix", "format", "{file}"], "lint_dir": null},
  "cache_env": {"MIX_BUILD_ROOT": "cache"},
  "artifacts": ["_build", "cover", "doc", "tmp", "erl_crash.dump", "*.beam", "*.ez", "priv/plts/*.plt", "priv/plts/*.plt.hash"],
  "arch": {
    "import_regex": "^\\s*(?:alias|import|require|use)\\s+([A-Z][A-Za-z0-9_.]*)",
    "resolver": "native",
    "native_tool": "mix compile --warnings-as-errors (compilers: [:boundary] ++ Mix.compilers()); mix xref graph --format cycles --fail-above 0",
    "forbidden_in_domain": ["Ecto.Repo", "Ecto.Query", "Phoenix", "Plug", "Logger", "File", "System", "Oban"],
    "impure_calls": ["DateTime.utc_now", "DateTime.now", "NaiveDateTime.utc_now", "Date.utc_today", "System.os_time", "System.system_time", "Enum.random", ":rand.", "spawn", "Task.async", "Task.start", "send(", ":ets."]
  },
  "test_jobs": "--max-cases 2",
  "config_files": [".formatter.exs", ".credo.exs", ".dialyzer_ignore.exs", ".sobelow-conf", "coveralls.json", ".tool-versions", ".editorconfig", "mix.exs", ".github/dependabot.yml"],
  "sources": {
    "checks": "https://mix.hexdocs.pm/Mix.Tasks.Test.html",
    "format": "https://mix.hexdocs.pm/Mix.Tasks.Format.html",
    "compile": "https://mix.hexdocs.pm/Mix.Tasks.Compile.html",
    "xref": "https://mix.hexdocs.pm/Mix.Tasks.Xref.html",
    "credo": "https://credo.hexdocs.pm/Mix.Tasks.Credo.html",
    "dialyzer": "https://github.com/jeremyjh/dialyxir",
    "coverage": "https://github.com/parroty/excoveralls",
    "audit": "https://github.com/mirego/mix_audit",
    "hex_audit": "https://hex.hexdocs.pm/Mix.Tasks.Hex.Audit.html",
    "sobelow": "https://github.com/nccgroup/sobelow",
    "race": "https://mix.hexdocs.pm/Mix.Tasks.Test.html",
    "cache_env": "https://mix.hexdocs.pm/Mix.html",
    "boundary": "https://boundary.hexdocs.pm/Boundary.html"
  }
}
```

## Gaps vs loops

| Rule ID | Gap | loops today | Priority | Where loops should fix it (spec / skill / role / gate / hook / arch) |
|---|---|---|---|---|
| `ELIXIR-TEST-06` / `-07` | No lcov parser; gate only reads Go coverprofile; built-in `mix test --cover` has no lcov | go-only | P0 | gate: add an lcov (`SF:`/`DA:`) parser behind the coverage port; run `mix coveralls.lcov`; take path `cover/lcov.info` |
| `ELIXIR-CONC-04` | Race policy has no `none` mode; no repeat-run substitute | missing | P0 | gate + WF §6: add `race.mode = none` with a `--repeat-until-failure` stress check; hook `race-guard` to accept it |
| `ELIXIR-TOOL-03`/`-04`/`-05`/`-08` | Gate `runCheck` is a Go switch: no `mix format --check-formatted`, `mix compile --warnings-as-errors`, Credo, `deps.audit`/`hex.audit` | go-only | P0 | gate: data-driven checks from the pack JSON |
| `ELIXIR-TEST-02` | Test recognition and test-edit ownership are `*_test.go` | go-only | P0 | spec WORKFLOW §2.3, stop-gate (`GoDirty`), test-guard: use pack `test_globs`/`source_globs` |
| `ELIXIR-OPS-05`/`-06` | Run cache is `GOCACHE`; Elixir needs `MIX_BUILD_ROOT` and artifact list incl. `_build`, `cover`, `tmp` | go-only | P0 | hook `go-env` → generic pack-driven env hook; `runspace` cleanup list |
| `ELIXIR-ARCH-04`/`-05` | No arch tool for Elixir; checker only parses Go | go-only | P1 | arch: `resolver: native` delegating to Boundary compile warnings + `mix xref` |
| `ELIXIR-REPO-04`/`-05`/`-08`/`-09` | No lockfile, pinning, hygiene, CI-shape rules | missing | P1 | new spec section "repository management" + `repo-management` reference skill |
| `ELIXIR-ARCH-07`/`-09`/`-10` | Purity forbidden list and size limits are Go units | go-only | P1 | arch config from pack (`forbidden_in_domain`, `impure_calls`); SOLID spec: units per language |
| `ELIXIR-CONC-01`..`-03`, `-05`, `-08`, `-09` | No OTP guidance (processes, supervision, GenServer misuse, ETS races) | missing | P1 | new `elixir-otp` reference skill; engineer/architect role text |
| `ELIXIR-SEC-01`..`-09` | No OWASP/atom-exhaustion/deserialization rules; no SAST step | missing | P1 | security spec + `sobelow` check + hook for `String.to_atom`/`binary_to_term` greps |
| `ELIXIR-DATA-02`/`-03`/`-05`/`-06` | Migration safety, Ecto traps and pool config not in consistency skill | missing | P1 | `consistency` skill + a data reference file |
| `ELIXIR-OPS-02`/`-04` | Packaging and release rules absent | missing | P1 | new `packaging` skill |
| `ELIXIR-TOOL-01`/`-02`/`-06`/`-07`/`-10`, `-REPO-08` | No toolchain/version rules; `.tool-versions` unrecognised by `init` | missing | P1 | `loops init` language detection (`mix.exs`) + pack versions |
| `ELIXIR-TEST-01`/`-03`/`-04`/`-05`/`-08`/`-09`/`-10`/`-11` | Testing skill is Go (httptest, table tests, fuzz) | go-only | P1 | `testing` skill split into core + per-language reference |
| `ELIXIR-REPO-06`/`-07`/`-11` | Dependabot/Renovate, publishing, changelogs | missing | P2 | repository-management skill |
| `ELIXIR-PERF-01`..`-04` | Performance skill is Go (benchstat, pprof) | go-only | P2 | `performance` skill reference per language |
| `ELIXIR-OBS-01`..`-04` | Observability guidance is Go and has no OTel | missing | P2 | observability reference + pack data |
| `ELIXIR-API-01`..`-06`, `ELIXIR-FW-01`..`-05` | Framework conventions (Phoenix, Oban, Broadway) | missing | P2 | framework section in pack skill; detect Phoenix |
| `ELIXIR-IDIOM-01`..`-12`, `ELIXIR-DOC-01`..`-04` | Idioms, doc tool (ExDoc) and review focus | go-only | P2 | pack RULES.md; review checklist per language |

## Open questions

1. **Built-in coverage output (resolved).** On 1.20.5 `mix test --cover` writes only per-module HTML in `cover/` plus a summary; no lcov/Cobertura. excoveralls is required.
2. **excoveralls passthrough (resolved).** Extra args such as `--warnings-as-errors` pass through to `mix test`. `output_dir` still lives in `coveralls.json`, so loops cannot redirect the path per run; `SF:` paths are absolute and must be normalised.
3. **`mix test` env (resolved).** `preferred_envs` in `cli/0` runs `mix coveralls.lcov` under `:test` without `MIX_ENV`. The per-check `env` key remains a TAXONOMY schema extension.
4. **Race substitute cost.** `--repeat-until-failure 5` multiplies test time by up to 5; maintainers must decide whether it is a hard gate or only runs on changed test files. Concuerror is stale (last release 2020, OTP 20.3-23.3) and is not recommended.
5. **OTP 29 GA date (resolved).** OTP 29.0 went GA on 2026-05-13 (GitHub `OTP-29.0`). Pin by `.tool-versions` regardless.
6. **Libraries.** `sbom`, `open_api_spex`, `logger_json`, Expert and diff-cover now have URLs. `Corsica`/`CORSPlug`, `fuse` (last release 2021), `Hammer`, `peep` and `excellent_migrations` were confirmed to exist by the verifier; `fuse` is stale.
7. **Credo per-file linting and post-edit.** `lint_dir` is `null` because I did not verify per-file Credo argv; `mix compile` per edit is too slow. Maintainers decide whether post-edit lint should be format-only.
8. **Boundary maintenance and umbrella.** Boundary 0.12.0 docs fetched; its activity level and compatibility with Elixir 1.20 type-checker warnings were not verified. Umbrella projects need per-app `coveralls.json` (skip_files does not work at the umbrella root) and `mix test.coverage` at the root.
9. **`MIX_HOME`/`HEX_HOME` defaults.** The Mix docs give `~/.mix` for MIX_HOME; the default Hex home was not stated on the fetched page. The advice not to relocate them is conservative.
10. **Sandbox and Oban config note.** Oban docs fetched cover `:manual`/`:inline` and `Oban.Testing`; the `drain_queue` helper was not on the fetched page.

## Verification

Verifier: `B2-beam`, 2026-11-10.

- **Verdict: `fail`, round 1.** The lint reports ERRs, there are two tooling inaccuracies, and five `loops today` values are wrong. No link fails, and the sources hold up well (11 opened, 1 weak). Fixes are small.
- **Completeness:** all 14 sections are present, plus Top 10, LOOPS, Gaps, Open questions and Verification. Rule counts: REPO 11, TOOL 10, ARCH 11, IDIOM 12, CONC 9, TEST 11, PERF 4, SEC 9, DATA 7, API 6, OBS 4, OPS 6, DOC 4, FW 5 (109 in total; every minimum is met). The LOOPS JSON parses. Lint (the current `lint.py`, which skips the Gaps table): `ERR 5 malformed rule rows` (see finding 1).
- **Links:** `links.sh` reports no `FAIL` and no `blocked` URLs. The `<pkg>.hexdocs.pm` hosts are correct: `hexdocs.pm/<pkg>` now 301-redirects to them.

**Sources opened**
1. https://elixir.hexdocs.pm/compatibility-and-deprecations.html: yes. 1.20 gets bug fixes and security patches; 1.19 and 1.18 get security patches only. The OTP ranges are 27-29, 26-28 and 25-27.
2. https://elixir-lang.org/blog/2025/10/16/elixir-v1-19-0-released/: yes. "Elixir v1.19 officially supports Erlang/OTP 28.1+" (TOOL-01).
3. https://elixir-lang.org/blog/2026/06/03/elixir-v1-20-0-released/: partly. It confirms inference of every construct, inference in guards, and that type signatures are not yet available. It does **not** say that inference works "across applications" (TOOL-06). Counted as the 1 weak source.
4. https://mix.hexdocs.pm/Mix.html: yes. `MIX_BUILD_ROOT` is the CI setting; `MIX_BUILD_PATH` ignores ROOT and is independent of env and target; `MIX_HOME` defaults to `~/.mix`; `MIX_OS_CONCURRENCY_LOCK` behaves as stated.
5. https://github.com/parroty/excoveralls: yes. lcov is labelled "(Experimental)" and written to `cover/lcov.info`; the JSON report goes to `cover/excoveralls.json` and Cobertura to `cover/cobertura.xml`; `output_dir`, `minimum_coverage` (default 0), `skip_files` and the ignore comments are all documented.
6. https://hex.hexdocs.pm/Mix.Tasks.Hex.Audit.html: yes. It checks retirements **and** advisories, exits non-zero, and supports `ignore_advisories`.
7. https://github.com/mirego/mix_audit: yes. The `~> 2.1` dep line, exit 1 on findings and `--format json` are all documented.
8. https://mix.hexdocs.pm/Mix.Tasks.Xref.html: yes. `--format cycles`, `--fail-above` and the `compile-connected` label all exist.
9. https://boundary.hexdocs.pm/Boundary.html: yes. `compilers: [:boundary] ++ Mix.compilers()`, `use Boundary, deps:, exports:`, enforced at compile time through warnings.
10. https://elixir.hexdocs.pm/process-anti-patterns.html: yes. All four anti-patterns cited in ARCH-09, IDIOM-08 and CONC-06 are on the page.
11. `sbom` package (Hex API and the 0.12.1 tarball): yes. The task is `mix sbom.cyclonedx`.

**Libraries the researcher cited from memory (Open question 6), checked through the Hex and GitHub APIs:**
- They all exist: `sbom` 0.12.1 (2026-11-04), `open_api_spex` 3.22.4, `logger_json` 7.0.4, `excellent_migrations` 0.1.10, `peep` 5.0.1, `hammer` 7.5.1, `corsica` 2.1.3, `cors_plug` 3.0.3 and `fuse` 2.5.0 (last release 2021).
- Expert is real, at `github.com/expert-lsp/expert` ("Official Elixir Language Server Protocol implementation").
- diff-cover reads both LCov and Cobertura (README).
- Concuerror's last release is 0.21.0 (2020), and its README badge says OTP 20.3-23.3.
- OTP 29.0 went GA on **2026-05-13** (GitHub release `OTP-29.0`), which settles Open question 5.

**Tooling truth**
- `mix format --check-formatted`: confirmed by the spot run (exit 1 on a diff, 0 when clean). Doc: Mix.Tasks.Format.
- `mix compile --warnings-as-errors`, `mix deps.get --check-locked`, `mix xref graph --format cycles --fail-above 0`: confirmed against the docs.
- `mix test --repeat-until-failure 5 --warnings-as-errors` (the race substitute): confirmed. `mix help test` says "*(since v1.17.0)*", and the run exited 0. `--warnings-as-errors` exits **1** when the suite would otherwise pass, and 3 only when tests also fail (see finding 3).
- `mix coveralls.lcov --warnings-as-errors`: confirmed, and **the extra arguments do pass through to `mix test`**. With a warning in a test file the run exited 1 ("Test suite aborted ... due to warnings while using the --warnings-as-errors option"); without the flag it exited 0. This settles Open question 2.
- `cli/0` `preferred_envs`: confirmed. `mix coveralls.lcov` ran under `:test` without `MIX_ENV`, which settles Open question 3.
- Coverage file: `cover/lcov.info` in lcov format (`TN:`/`SF:`/`DA:`/`LF:`/`LH:`), line granularity. Confirmed. Note: `SF:` holds an **absolute** path (`SF:/tmp/hello/lib/hello.ex`), so the loops lcov parser must make paths relative to the project root.
- Built-in `mix test --cover` in a fresh project with no excoveralls: it writes only `cover/Elixir.<Mod>.html` and a summary, with **no lcov or Cobertura**. `--export-coverage` writes `.coverdata`. This confirms the summary and Open question 1 (Elixir 1.20.5).
- `mix deps.audit`, `mix hex.audit`, `mix sobelow --exit medium`, `mix credo --strict`: confirmed against the docs, but not run.
- `race.mode = none` with a substitute: consistent with CONC-04.

**Spot run**
- Image: `elixir:latest`, which is Elixir 1.20.5 on OTP 29.1.2. Projects were built under `/tmp` inside the container, with no repo mount.
- In a fresh `mix new plain`:
  - `mix format --check-formatted` → 0;
  - `mix test --repeat-until-failure 5 --warnings-as-errors` → 0;
  - `mix test --cover` → 0, HTML only.
- In `hello`, with `{:excoveralls, "~> 0.18", only: :test}`, `test_coverage: [tool: ExCoveralls]` and `preferred_envs`:
  - `mix format --check-formatted` → 0;
  - `mix coveralls.lcov --warnings-as-errors` → 0, and `cover/lcov.info` is written;
  - `MIX_ENV=test mix coveralls.lcov --warnings-as-errors` → 0;
  - the same run with a warning in a test → 1.
- Side note: excoveralls 0.18.5 compiles with type warnings on Elixir 1.20. These don't affect `--warnings-as-errors`, because dependency warnings are not counted.

**Findings**
1. **Lint ERR, 5 malformed rows.**
   - `ELIXIR-TOOL-04`, `-IDIOM-05`, `-IDIOM-12` and `-OPS-02` contain an unescaped `|` inside backticks (`json|sarif|…`, `%{struct | key: val}`, `|>`, `start|daemon|…`). GitHub splits the cell on it, so these rows have 8-12 cells. Fix: escape the pipe as `\|`.
   - `ELIXIR-REPO-11` has no URL in Source (see finding 5).
2. **TOOL-06:** the claim that inference works "across applications" isn't in the cited v1.20 post. Drop it, or cite a source that says it.
3. **TOOL-05:** "exit status 3 from the test flag" is imprecise. `mix help test` (1.20.5) says it exits 1 when the suite would otherwise pass and `--exit-status`+1 (3 by default) when tests also fail; the spot run got 1. Reword it.
4. **CONC-04 (Source cell) and Open question 4:** "Concuerror unverified". Concuerror's last release is 0.21.0 (2020) and its README badge covers OTP 20.3-23.3, so recommending it for Elixir 1.20 on OTP 27-29 is unsupported. Mark it stale and advisory, as `erlang.md` CONC-03 does, or drop it, and cite https://github.com/parapluu/Concuerror.
5. **`loops today` misclassified:**
   - `ELIXIR-REPO-11` is marked `covered (AGENTS.md Releases)`. `AGENTS.md` governs the loops repo itself, and no spec, skill or agent tells target projects to use conventional commits (`grep -ri "conventional commit" specs skills agents ROLES.md` finds nothing). Also, `loops release` rewrites `.claude-plugin/plugin.json` (`internal/application/release/release.go`) and cannot own a service's `mix.exs` version. Fix: `missing`, give a URL source (https://www.conventionalcommits.org/), and remove the `loops release` advice.
   - `ELIXIR-ARCH-02` ("covered … but tooling go-only") and `ELIXIR-ARCH-07` ("covered … but go-only checks") are disguised `partial` values. Pick `go-only`, since the check exists only for Go, and explain in the Gap column.
   - `ELIXIR-ARCH-06` cites "HEX_ARCH §22 DI", but DI is §21 (§22 is initialization order). Fix the citation.
6. **Open questions 1, 2, 3 and 5 are now settled** (see Tooling truth). Update them, along with the "unverified" markers in TOOL-09, TOOL-10, API-02, OBS-01, TEST-07 and CONC-04: replace each with the real URL (`https://hex.pm/packages/sbom`, `https://github.com/expert-lsp/expert`, `https://hex.pm/packages/open_api_spex`, `https://hex.pm/packages/logger_json`, `https://github.com/Bachmann1234/diff_cover`).
7. **Header (minor):** "latest `1.20.5`, released 2026-06-03" conflates two releases. 1.20.0 shipped on 2026-06-03 and 1.20.5 on 2026-10-29 (GitHub releases). Fix the date.
8. **OPS-05 note (minor):** a per-run `MIX_BUILD_ROOT` recompiles every dependency on each run. That isn't a re-download, but it is the main cost. Say so in OPS-05 or in Open questions so the maintainers can choose.

### Round 2

Verifier: `B2-beam`, 2026-11-10.

- **Verdict: `pass`, round 2.**
  - `lint.py` reports no ERR (109 rules).
  - `links.sh` reports no FAIL.
  - The LOOPS JSON parses.
  - All 8 round-1 findings are resolved.
- Findings:
  1. Resolved. The pipes in TOOL-04, IDIOM-05, IDIOM-12 and OPS-02 are escaped (`\|`), and REPO-11 now cites URLs.
  2. Resolved. TOOL-06 no longer claims inference "across applications".
  3. Resolved. TOOL-05 now gives exit 1, or `--exit-status`+1 (default 3).
  4. Resolved. CONC-04 marks Concuerror as stale and advisory and links its repo.
  5. Resolved.
     - REPO-11 is `missing`, and the `loops release` advice is gone.
     - ARCH-02 and ARCH-07 are `go-only`, with the reason in the cell.
     - ARCH-06 cites §21.
  6. Resolved. Open questions 1, 2, 3 and 5 are marked as resolved. TOOL-09, TOOL-10, API-02, OBS-01 and TEST-07 carry real URLs. The `SF:` normalisation is noted.
  7. Resolved. The header now gives 1.20.0 as 2026-06-03 and 1.20.5 as 2026-10-29.
  8. Resolved. OPS-05 states the cost of recompiling dependencies.
- No re-spot-run was needed, because no argv changed.
