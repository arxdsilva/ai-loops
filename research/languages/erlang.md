# Erlang: backend practices and loops gap analysis

| | |
|---|---|
| Pack id | `erlang` |
| Current stable / LTS | OTP 29.1.x (29.0 released 2026-05-13; latest listed 29.1.2) / none (OTP has no LTS; security patches go to the last 3 majors: 29, 28, 27 as of 2026-11) |
| Dominant runtimes / frameworks | BEAM (ERTS) on OTP; rebar3 3.28; Cowboy 2.21 (HTTP/WebSocket), Elli, Nova 0.18; PropEr; Common Test |
| As of | 2026-11 |
| Researcher | `B1-erlang` |
| Verifier | (filled in by B2) |

> **Summary:** A strong Erlang team builds one OTP release out of a rebar3 project (umbrella `apps/` or a single app plus `lib`), commits `rebar.lock`, and gates on `rebar3 compile` with `warnings_as_errors`, `xref`, `dialyzer`, `erlfmt --check`, Elvis, then EUnit + Common Test + PropEr with `cover`. Isolation by process replaces the data-race model, so there is no `-race` flag; races that remain (ETS, message order, timing) are probed with PropEr statem, repeated and re-scheduled runs, and Concuerror, which is stale (last Hex release 2020, README claims OTP 20.3 to 23.3).
> Biggest loops gaps: (1) the gate, arch checker, post-edit hook, stop-gate and cache env are Go-only, and there is no race policy for a flagless ecosystem; (2) coverage needs a Cobertura reader (via `covertool`) and the `.erl`/`_SUITE.erl` candidate globs; (3) the "interfaces" and purity rules of HEXAGONAL/SOLID need an OTP translation (applications as layers, behaviours as ports, no processes in the domain).

Rules follow [TAXONOMY.md](TAXONOMY.md), using the row format `ID | Rule | Why | Sev | Source | loops today | Enforce`. Gaps are judged against [LOOPS_BASELINE.md](LOOPS_BASELINE.md).

Common source shorthands used below:
- OTP docs: <https://www.erlang.org/doc/>
- rebar3 docs: <https://rebar3.org/docs/>
- EEF Security WG hardening guide: <https://security.erlef.org/>

## 1. REPO: repository and project management

Canonical tree for a service (rebar3 umbrella, one OTP app per hexagon layer, one release):

```
my_service/
  rebar.config            # deps, profiles, relx, plugins, xref/dialyzer/cover/erlfmt config
  rebar.lock              # committed
  elvis.config            # Elvis rules
  .tool-versions          # erlang 29.1.2 / rebar 3.28.0  (asdf or mise)
  config/
    sys.config.src        # runtime config template (env substitution)
    vm.args.src
  apps/
    ms_domain/            # pure: records, types, functions. no processes
      src/ms_domain.app.src
      src/*.erl
      test/               # EUnit *_tests.erl and PropEr prop_*.erl
    ms_app/               # use cases, ports as -callback behaviours, gen_servers/gen_statems
      src/ms_app.app.src
      src/ms_app_sup.erl
      test/*_SUITE.erl
    ms_http/              # inbound adapter: Cowboy handlers, routes, validation
    ms_pg/                # outbound adapter: epgsql/pgo repository implementing a port
    ms_release/           # composition root: top app, sup tree, wiring, sys.config
      src/ms_release_app.erl
  test/                   # cross-app Common Test suites (system tests)
  priv/                   # sql migrations, static files
  scripts/  docs/  Dockerfile  .github/workflows/ci.yml
```

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `ERL-REPO-01` | Create projects with `rebar3 new app`, `lib`, `release` or `umbrella` and keep the standard OTP app layout: `src/`, `include/`, `priv/`, `test/`, and `<app>.app.src` in `src/`. [REPO-01] | rebar3, Common Test and relx find code only by this layout | MUST | [rebar3 getting started](https://rebar3.org/docs/getting-started/), [OTP applications](https://www.erlang.org/doc/system/applications.html) | missing | review |
| `ERL-REPO-02` | Use `apps/` (umbrella) when the service has several OTP apps; use `lib/` only for libraries you ship together. Keep one release per deployable. [REPO-02] | One app per layer makes app dependencies the architecture boundary | SHOULD | [Adopting Erlang: development](https://adoptingerlang.org/docs/development/), [rebar3 releases](https://rebar3.org/docs/deployment/releases/) | missing | review |
| `ERL-REPO-03` | Split a repo only at a deployable or release boundary; otherwise keep a rebar3 umbrella. Do not use Bazel or Nx for Erlang unless the org already runs it. [REPO-02] | rebar3 handles multi-app natively; extra build systems fight `_build` | SHOULD | [Adopting Erlang](https://adoptingerlang.org/), [Erlang.mk guide](https://erlang.mk/guide/) | missing | review |
| `ERL-REPO-04` | Commit `rebar.lock` for applications and releases; for a Hex library commit it too (it is not published) but declare ranges in `rebar.config`. [REPO-04] | The lock holds immutable git refs, versions and package hashes against hijacked mirrors | MUST | [rebar3 dependencies](https://rebar3.org/docs/configuration/dependencies/) | missing | gate |
| `ERL-REPO-05` | Declare deps with `"~> X.Y"` ranges for Hex packages, `{tag, ...}` or `{ref, ...}` (never `{branch, ...}`) for git deps. Use `{deps_error_on_conflict, true}`. [REPO-04] | Branch deps make `rebar.lock` the only reproducibility; conflicts are otherwise skipped silently (level-order, shallowest wins) | MUST | [rebar3 dependencies](https://rebar3.org/docs/configuration/dependencies/) | missing | review |
| `ERL-REPO-06` | Use `rebar3 upgrade <dep>` to bump one dep and `rebar3 tree` to inspect; never hand-edit `rebar.lock`. Remove unused deps with `rebar3_hank` or `rebar3 xref`. [REPO-04] | Keeps the transitive tree minimal and auditable | SHOULD | [rebar3 commands](https://rebar3.org/docs/commands/), [rebar3_hank](https://github.com/AdRoll/rebar3_hank) | missing | gate |
| `ERL-REPO-07` | Neither Renovate nor Dependabot supports rebar3 natively (Renovate has no rebar manager; Dependabot's Hex support is `mix` only), so schedule a CI job that runs `rebar3 upgrade` and opens a PR, or add a Renovate regex manager for `rebar.config`. [REPO-04] | Lockfile drift and old OTP-bound deps are the main supply-chain risk | SHOULD | [Renovate docs](https://docs.renovatebot.com/modules/manager/) | missing | review |
| `ERL-REPO-08` | Publish libraries to Hex with `rebar3 hex publish` (plugin `rebar3_hex`), SemVer, a `CHANGELOG.md`, `{licenses,...}` and `{links,...}` in `.app.src`; use Hex package scopes or a private repo for internal packages. [REPO-05] | Hex is the registry; metadata lives in `.app.src` | SHOULD | [Hex publish](https://hex.pm/docs/publish), [rebar3_hex](https://hexdocs.pm/rebar3_hex/) | missing | review |
| `ERL-REPO-09` | Pin the toolchain with `.tool-versions` (asdf or mise: `erlang`, `rebar`) and `{minimum_otp_vsn, "27"}` in `rebar.config`. Build CI from the same pin. [REPO-06] | OTP major drives compiler warnings, Dialyzer and JIT behaviour | MUST | [rebar3 config](https://rebar3.org/docs/configuration/configuration/) | missing | gate |
| `ERL-REPO-10` | `.gitignore`: `_build/`, `*.beam`, `erl_crash.dump`, `rebar3.crashdump`, `ct_logs/`, `*.coverdata`, `.rebar3/`, `*.plt`, `bom.xml`, `Mnesia.*/`, `cover/`, `*.tar.gz`. Add `.editorconfig` (4-space indent, no tabs). Commit `rebar.lock`, `elvis.config`. [REPO-07] | These files are artifacts; committing them poisons diffs and caches | MUST | [rebar3 basic usage](https://rebar3.org/docs/basic_usage/), [OTP crash dump](https://www.erlang.org/doc/apps/erts/crash_dump.html) | missing | hook |
| `ERL-REPO-11` | CI order: restore `_build` and `~/.cache/rebar3` cache keyed on `rebar.lock` + OTP version, then `rebar3 compile`, `fmt --check`, `lint`, `xref`, `dialyzer`, `eunit`, `ct`, `proper`, `cover`, `sbom`. Matrix: the OTP majors still patched (28, 29; 27 until dropped). [REPO-08] | Cheapest gates first; PLT and deps are the slow parts | SHOULD | [rebar3 commands](https://rebar3.org/docs/commands/), [OTP SECURITY.md](https://github.com/erlang/otp/blob/master/SECURITY.md) | missing | review |
| `ERL-REPO-12` | Use conventional commits; generate changelogs from them or keep `CHANGELOG.md`. [REPO-09] | loops release tooling already reads them | SHOULD | [Conventional Commits](https://www.conventionalcommits.org/) | missing | review |

## 2. TOOL: toolchain

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `ERL-TOOL-01` | Target OTP 28 or 29. OTP 29.0 shipped 2026-05-13 (29.1.2 current), OTP 28 latest patch 28.5.0.8, OTP 27 27.3.4.19; OTP has no LTS and patches only the last 3 majors; one major is released each year (May). Install with asdf or mise. [TOOL-01] | Running an unpatched major leaves known CVEs | MUST | [OTP downloads](https://www.erlang.org/downloads), [OTP SECURITY.md](https://github.com/erlang/otp/blob/master/SECURITY.md), [OTP 29.0 patch](https://www.erlang.org/patches/OTP-29.0) | missing | gate |
| `ERL-TOOL-02` | Format with erlfmt: add `{project_plugins, [erlfmt]}.` and `{erlfmt, [write]}`; CI runs `rebar3 fmt --check`. Opt a file out only with `%%% % @noformat` or a form with `%% erlfmt:ignore <reason>`. [TOOL-02] | One formatter ends style debate; erlfmt is feature-complete (1.0) | MUST | [erlfmt](https://github.com/WhatsApp/erlfmt) | go-only (gofmt gate and `go-post-edit` hook) | gate |
| `ERL-TOOL-03` | Compile with `{erl_opts, [debug_info, warnings_as_errors, warn_export_vars, warn_unused_import, warn_missing_spec]}`. Never ship with `nowarn_*` blanket options. [TOOL-03] | The compiler catches unused vars, bad guards, deprecated calls (OTP 29 warns on `-unsafe` functions) | MUST | [rebar3 config](https://rebar3.org/docs/configuration/configuration/), [OTP 29.0](https://www.erlang.org/patches/OTP-29.0) | go-only (`go vet`) | gate |
| `ERL-TOOL-04` | Run `rebar3 xref` with `{xref_checks, [undefined_function_calls, undefined_functions, locals_not_used, deprecated_function_calls, deprecated_functions]}`. [TOOL-03] | Finds calls to missing modules and dead exports, and checks the app-level dependency direction | MUST | [rebar3 config](https://rebar3.org/docs/configuration/configuration/), [Xref](https://www.erlang.org/doc/apps/tools/xref_chapter.html) | missing | gate |
| `ERL-TOOL-05` | Lint with Elvis through `rebar3_lint`: `rebar3 lint`; rulesets `erl_files`, `hrl_files`, `rebar_config`, `gitignore`; `warnings_as_errors` defaults to true so findings exit non-zero. Keep `elvis.config` in git and run the check only when it exists: `rebar3 lint` exits 1 with no `elvis.config` or `{elvis, ...}` entry (rebar3_lint 6.0.0). [TOOL-03] | Enforces nesting depth, naming, line length, no `try ... catch _:_` swallow | SHOULD | [elvis_core](https://github.com/inaka/elvis_core), [rebar3_lint](https://github.com/inaka/rebar3_lint) | go-only (staticcheck) | gate |
| `ERL-TOOL-06` | Type-check with Dialyzer: `rebar3 dialyzer` (use `--incremental` locally) with `{dialyzer, [{warnings, [unmatched_returns, error_handling, underspecs]}, {plt_extra_apps, [...]}]}`. [TOOL-04] | Success typing finds dead code and contract breaks; first run builds a PLT, cache it | MUST | [Dialyzer](https://www.erlang.org/doc/apps/dialyzer/dialyzer.html), [rebar3 commands](https://rebar3.org/docs/commands/) | missing | gate |
| `ERL-TOOL-07` | For gradual stricter typing use eqWAlizer through ELP: add `eqwalizer_support` dep, run `elp eqwalize-all`. It needs OTP 26+; the standalone repo is archived and development is in ELP. Treat it as optional on top of Dialyzer, never as a required gate. [TOOL-04] | eqWAlizer is stricter (sound) but has a different checking model and costs annotation effort | SHOULD | [eqWAlizer](https://github.com/WhatsApp/eqwalizer), [ELP](https://whatsapp.github.io/erlang-language-platform/) | missing | review |
| `ERL-TOOL-08` | Audit dependencies: `rebar3_audit` plugin (`rebar3 audit`, v0.2.8, 2026-09; it queries the GitHub Advisory Database, so it needs network access to api.github.com, optionally `GITHUB_TOKEN` for rate limits; single maintainer, and its Hex-linked repo URL is dead) and CycloneDX SBOM with `rebar3 sbom` (writes `./bom.xml`, `-F json` for `bom.json`). Pin rebar3 >= 3.27.0 (CVE-2026-21619 fixes). Check OSV-Scanner support for `rebar.lock` before relying on it. [TOOL-05] | Hex advisories and rebar3 CVEs exist; scanner support for `rebar.lock` is young | SHOULD | [rebar3_audit](https://hex.pm/packages/rebar3_audit), [rebar3_sbom](https://github.com/erlef/rebar3_sbom), [osv-scalibr PR 2404](https://github.com/google/osv-scalibr/pull/2404) | go-only (govulncheck) | gate |
| `ERL-TOOL-09` | Editor baseline: ELP (Erlang Language Platform) as LSP; turn off any second formatter on save so erlfmt wins. [TOOL-06] | Two formatters fight | SHOULD | [ELP](https://whatsapp.github.io/erlang-language-platform/), [erlfmt](https://github.com/WhatsApp/erlfmt) | missing | review |

## 3. ARCH: architecture

| Style | Erlang expression | Choose when |
|---|---|---|
| Hexagonal / clean | OTP apps as layers; `-callback` behaviours as ports; apps wired in a release app | default for services with business logic |
| Layered / MVC | Cowboy handler, service module, repository module in one app | thin CRUD, prototypes |
| DDD tactical | Records or maps as entities and value objects, one process (gen_server/gen_statem) per aggregate when it needs serialised state; repository = behaviour + adapter | complex domain with concurrent state per entity |
| CQRS / event sourcing | Event log in Postgres or Mnesia; projections as gen_servers; `gen_event` or `pg` for fan-out | audit or replay needs |
| Event-driven | `pg` process groups, `gen_event`, Kafka via `brod`, RabbitMQ via `amqp_client` | async integration |
| Modular monolith vs microservices | One release with many apps; distribution is for clusters of identical nodes, not tier-to-tier | default is a modular monolith release |
| Serverless | Rare; BEAM start-up and release size make it a poor fit | avoid |
| Actor model | Native: processes, mailboxes, supervisors | everywhere, but see CONC rules |

Mapping of loops layers onto an Erlang umbrella (app = layer; `.app.src` `applications` list and `xref` enforce direction):

```
apps/
  ms_domain/   -> internal/domain          pure functions, types, records. applications: [kernel, stdlib]
  ms_app/      -> internal/application     use cases, -callback ports, gen_server/gen_statem, supervisors
                                           applications: [kernel, stdlib, ms_domain]
  ms_http/     -> adapters/inbound         cowboy handlers; applications: [.., cowboy, ms_app, ms_domain]
  ms_pg/       -> adapters/outbound        epgsql/pgo implementing ms_app ports; applications: [.., epgsql, ms_app]
  ms_release/  -> internal/compose + cmd   release app: starts sup tree, injects adapter modules via app env
```

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `ERL-ARCH-01` | Make each hexagon layer its own OTP application; list only inward dependencies in `.app.src` `applications`. Domain lists `[kernel, stdlib]` only. [ARCH-01, ARCH-02] | The `applications` list is the start order and the declared boundary | MUST | [OTP applications](https://www.erlang.org/doc/system/applications.html), [HEXAGONAL §3-§4](../../specs/HEXAGONAL_ARCHITECTURE.md) | go-only (`internal/` layout, `loops arch`) | arch |
| `ERL-ARCH-02` | Modules are flat atoms: `module:function(...)` names the module only, so enforce app boundaries with xref queries or an import scan, with a per-app module prefix (`ms_pg_*`). The module name must equal the file name. [ARCH-03] | There is no `internal/` or visibility between apps; any exported function can be called | MUST | [Erlang reference: modules](https://www.erlang.org/doc/system/reference_manual.html), [Xref](https://www.erlang.org/doc/apps/tools/xref_chapter.html) | missing | arch |
| `ERL-ARCH-03` | Keep the exported API of each app small: `-export` only the facade; mark internals with `-doc false.` (OTP 27+) and `-export_type` only public types. [ARCH-03, ARCH-08] | Exports are the only visibility control; hidden docs signal internals | SHOULD | [Erlang reference: modules](https://www.erlang.org/doc/system/reference_manual.html), [documentation](https://www.erlang.org/doc/system/documentation.html) | missing | review |
| `ERL-ARCH-04` | Import cycles between apps are legal for the compiler but forbidden here: run xref and an app-graph check (`rebar3 tree`, `.app.src` `applications`). [ARCH-04] | Cycles break release start order and hot upgrades | MUST | [OTP applications](https://www.erlang.org/doc/system/applications.html), [rebar3 commands](https://rebar3.org/docs/commands/) | go-only (arch cycle check) | arch |
| `ERL-ARCH-05` | Define ports as `-callback` behaviours in the consuming app (`ms_app_repo`), implement in the adapter app, and inject the implementation module through app env or a function argument at the composition root. [ARCH-05, ARCH-07] | Behaviours are the idiomatic port; consumer owns the interface | MUST | [OTP design principles](https://www.erlang.org/doc/system/design_principles.html), [HEXAGONAL §6](../../specs/HEXAGONAL_ARCHITECTURE.md) | go-only (consumer-owned interfaces) | arch |
| `ERL-ARCH-06` | The composition root is the release app: `start/2` builds the supervision tree and wires adapters via `application:set_env`. No `application:get_env` calls inside the domain. [ARCH-05] | Config at the edge keeps the domain pure; avoids a service locator | MUST | [OTP applications](https://www.erlang.org/doc/system/applications.html), [rebar3 releases](https://rebar3.org/docs/deployment/releases/) | go-only (init order, `cmd/`) | arch |
| `ERL-ARCH-07` | Domain apps contain no processes, no `ets`, `file`, `gen_tcp`, `httpc`, `os:`, `erlang:now/system_time`, `os:timestamp`, `rand:` or `!`. Pass time and random values in as arguments. [ARCH-06] | Pure functions are trivial to property-test and cannot race | MUST | [HEXAGONAL §3](../../specs/HEXAGONAL_ARCHITECTURE.md), [Erlang efficiency/process guide](https://www.erlang.org/doc/system/eff_guide_processes.html) | go-only (forbidden `os`, `time.Now`, `go`) | arch |
| `ERL-ARCH-08` | Put a process around state only when state must be serialised or has its own lifecycle; do not make a gen_server per use case or "service" just for code organisation. [ARCH-01, ARCH-08] | Processes serialise calls and add a bottleneck and mailbox; modules and functions are free | AVOID | [OTP design principles](https://www.erlang.org/doc/system/design_principles.html), [Adopting Erlang](https://adoptingerlang.org/docs/development/) | missing | review |
| `ERL-ARCH-09` | Use `gen_statem` (callback mode `state_functions` or `handle_event_function`) for state machines and sagas; use `gen_server` for stateful servers; `gen_event` only for pluggable handlers; every long-lived process under a supervisor. [ARCH-01] | Gen behaviours give sys/trace, code upgrade and supervision for free | MUST | [gen_statem](https://www.erlang.org/doc/apps/stdlib/gen_statem.html), [gen_event](https://www.erlang.org/doc/apps/stdlib/gen_event.html), [supervisor principles](https://www.erlang.org/doc/system/sup_princ.html) | go-only (`skills/jobs-and-state-machines`) | review |
| `ERL-ARCH-10` | Choose supervisor strategy deliberately: `one_for_one` default, `rest_for_one` for ordered dependencies, `one_for_all` for tightly coupled siblings, `simple_one_for_one` replaced by dynamic `one_for_one` with `child_spec`; set `intensity` and `period`; use `significant` and `auto_shutdown` where needed. [ARCH-01] | Restart intensity decides when a crash escalates | MUST | [Supervisor principles](https://www.erlang.org/doc/system/sup_princ.html), [supervisor](https://www.erlang.org/doc/apps/stdlib/supervisor.html) | missing | review |
| `ERL-ARCH-11` | SOLID size heuristics for Erlang: modules under about 400 lines, functions under about 25 lines and 5 parameters; split large `handle_call` clauses into helper functions. [ARCH-08] | Erlang function-clause style bloats quickly | SHOULD | [Inaka Erlang guidelines](https://github.com/inaka/erlang_guidelines), [SOLID_GO §8](../../specs/SOLID_GO.md) | go-only (SOLID limits in Go units) | arch |

## 4. IDIOM: what to do and what to avoid

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `ERL-IDIOM-01` | "Let it crash": return `{ok, V}` / `{error, Reason}` for expected outcomes, and crash (match failure, `error/1`) on programmer errors and corrupt state; never catch to hide. Do not write defensive code for unspecified inputs. [IDIOM-01] | Supervisors restore a clean state; defensive code hides bugs | MUST | [Robustness (OTP)](https://www.erlang.org/doc/system/robustness.html), [LYSE errors and processes](https://learnyousomeerlang.com/errors-and-processes) | go-only (GO_BEST §8-9) | review |
| `ERL-IDIOM-02` | Avoid `catch Expr` and `try ... catch _:_ -> ok`; catch a specific `Class:Reason:Stacktrace` and re-raise with `erlang:raise/3`. [IDIOM-01] | Bare catches swallow `exit` and `throw` and lose stack traces | AVOID | [Inaka guidelines](https://github.com/inaka/erlang_guidelines), [Elvis rules](https://github.com/inaka/elvis_core) | go-only (never swallow errors) | gate |
| `ERL-IDIOM-03` | Use `try ... after` for cleanup and `monitor` or links for process cleanup; never leave open sockets, ports or files without an owner process. [IDIOM-05] | Resources are tied to the owning process; the VM closes ports when it dies | MUST | [Erlang reference: expressions](https://www.erlang.org/doc/system/expressions.html), [Robustness](https://www.erlang.org/doc/system/robustness.html) | go-only (defer/close) | review |
| `ERL-IDIOM-04` | Absence is an explicit value: `undefined`, `error`, `{ok, V}`, `none`; use `maps:find/2` and `maps:get/3` with defaults, not `maps:get/2` unless a crash is wanted. [IDIOM-02] | There is no null; `undefined` leaks silently | SHOULD | [maps](https://www.erlang.org/doc/apps/stdlib/maps.html) | go-only (nil handling) | review |
| `ERL-IDIOM-05` | Data is immutable: model entities as records or maps with `-type` specs; keep a record definition in one `.hrl` per app and never share it across apps (share through an accessor or a map). [IDIOM-03] | A shared record couples apps at compile time and breaks hot upgrade | SHOULD | [Erlang reference: records](https://www.erlang.org/doc/system/ref_man_records.html), [typespec](https://www.erlang.org/doc/system/typespec.html) | missing | review |
| `ERL-IDIOM-06` | Write a `-spec` for every exported function and `-type` for every exported data shape; add `-opaque` for value objects whose internals must stay hidden. [IDIOM-04, TOOL-04] | Specs feed Dialyzer, docs and ELP | MUST | [Types and function specifications](https://www.erlang.org/doc/system/typespec.html) | missing | gate |
| `ERL-IDIOM-07` | Use pattern matching in function heads and guards; use strict generators (`<:-`, OTP 28) in comprehensions so a non-matching element crashes instead of being skipped. Note old-style guard tests are scheduled for removal in OTP 30. [IDIOM-04] | Silent filtering in comprehensions hides data bugs | SHOULD | [OTP 28 highlights](https://www.erlang.org/blog/highlights-otp-28/), [OTP 29.0](https://www.erlang.org/patches/OTP-29.0) | missing | review |
| `ERL-IDIOM-08` | Strings are binaries (`<<"x"/utf8>>`) in the service layer; lists of integers only for legacy APIs. Use the `string` module (Unicode-aware, not `lists:` functions) and `uri_string`. Build output as iolists. [IDIOM-06] | Binary matching is fast, memory is small; charlists break Unicode and 8-byte-per-char cost | SHOULD | [Efficiency guide](https://www.erlang.org/doc/system/efficiency_guide.html), [uri_string](https://www.erlang.org/doc/apps/stdlib/uri_string.html) | missing | review |
| `ERL-IDIOM-09` | Time: use `erlang:system_time/1` / `erlang:monotonic_time/1` and `calendar` with UTC (`calendar:universal_time()`); avoid `now/0`, `erlang:now/0` and `os:timestamp` in domain code; inject a clock. [IDIOM-06] | Time warps and testability | SHOULD | [calendar](https://www.erlang.org/doc/apps/stdlib/calendar.html), [consistency skill](../../skills/consistency/SKILL.md) | covered (`skills/consistency` clock injection) | review |
| `ERL-IDIOM-10` | Naming: `snake_case` for modules, functions, atoms, variables `CamelCase`; module prefix per app; `-spec` after the doc; max 100 columns; `-moduledoc`/`-doc` on public modules. [IDIOM-07] | OTP and Inaka conventions; erlfmt enforces layout | SHOULD | [Inaka guidelines](https://github.com/inaka/erlang_guidelines), [OTP documentation](https://www.erlang.org/doc/system/documentation.html) | missing | gate |
| `ERL-IDIOM-11` | Macros and parse transforms: use macros only for constants, logging and `?assert*`; avoid parse transforms (hide code from Dialyzer, xref and ELP) and avoid `-compile(export_all)` and `-compile(nowarn_export_all)`. [IDIOM-08] | Metaprogramming defeats the static tools | AVOID | [Inaka guidelines](https://github.com/inaka/erlang_guidelines), [Erlang reference: macros](https://www.erlang.org/doc/system/reference_manual.html) | missing | gate |
| `ERL-IDIOM-12` | Footgun: never create atoms from external input (`list_to_atom`, `binary_to_atom`, `erlang:binary_to_term/1`). Use `binary_to_existing_atom/2` or a lookup table. [IDIOM-09] | The atom table is finite (default 1,048,576) and never garbage-collected: a crash of the whole VM | MUST | [EEF atoms](https://security.erlef.org/secure_coding_and_deployment_hardening/serialisation), [erlang module](https://www.erlang.org/doc/apps/erts/erlang.html) | missing | review |
| `ERL-IDIOM-13` | Footgun: `++` and `lists:append` in loops on the left-hand side are O(n); build lists with cons and `lists:reverse/1`; use `maps` for lookup, not `proplists`/`lists:keyfind` over large lists. [IDIOM-09] | Quadratic behaviour in hot paths | SHOULD | [Efficiency guide: functions](https://www.erlang.org/doc/system/eff_guide_functions.html) | missing | review |

## 5. CONC: concurrency and the `-race` equivalent

**Loops race policy:** `mode = none`. The BEAM has no data races on process-local data (share-nothing, copying messages), so there is no `-race` flag and no sanitizer build. Substitute check, enforced by loops: (1) run EUnit and Common Test twice, once with `ERL_FLAGS="+S 1:1"` and once with `ERL_FLAGS="+S 8:8"` (different scheduling), (2) repeat property tests (`rebar3 proper -n 500`), (3) every module with a gen_statem, ETS table or multi-process protocol has a PropEr stateful (`proper_statem`) test, and (4) Concuerror is allowed as an advisory check only (see CONC-03). This is a defect-sampling substitute, not a proof.

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `ERL-CONC-01` | Model: lightweight processes, preemptive schedulers (one per core), asynchronous message passing with copy semantics; large binaries (>64 bytes) are reference counted off-heap. Spawn per independent unit of work, not per request step. [CONC-01] | Isolation is the data-race defence | MUST | [Processes (reference manual)](https://www.erlang.org/doc/system/ref_man_processes.html), [Efficiency guide: processes](https://www.erlang.org/doc/system/eff_guide_processes.html) | go-only (goroutines in GO_BEST §11-12) | review |
| `ERL-CONC-02` | Share state by messages. Where shared mutable state is needed use ETS with `{write_concurrency, true}`, `{read_concurrency, true}` and a single owner process; use `persistent_term` only for read-mostly data (updates trigger a global GC scan). [CONC-02] | Races remain at ETS read-modify-write and message-ordering level | MUST | [ets](https://www.erlang.org/doc/apps/stdlib/ets.html), [persistent_term](https://www.erlang.org/doc/apps/erts/persistent_term.html) | go-only (mutex/channel rules in GO_BEST §11-12) | review |
| `ERL-CONC-03` | Race detection options: (a) none built in; (b) Concuerror (stateless model checking, systematic scheduling, `concuerror -m mod -t test`): last Hex release 0.21.0 (2020-10-31), README badge OTP 20.3 to 23.3, so it may not run on OTP 27 to 29, run it advisory-only; (c) PropEr stateful tests with parallel commands (`proper_statem:parallel_commands/1`); (d) repeat and re-schedule runs. [CONC-03] | No sanitizer exists; be honest about the gap | SHOULD | [Concuerror](https://github.com/parapluu/Concuerror), [Concuerror on Hex](https://hex.pm/packages/concuerror), [PropEr](https://github.com/proper-testing/proper) | missing | review |
| `ERL-CONC-04` | Policy: `loops gate` must run the test suites under at least two `+S` scheduler settings and record both runs; failures in either block. Treat any test passing only under one setting as flaky. [CONC-04] | Cheap proxy for timing-dependent races | MUST | [erl command (+S)](https://www.erlang.org/doc/apps/erts/erl_cmd.html), [rebar3 commands](https://rebar3.org/docs/commands/) | missing | gate |
| `ERL-CONC-05` | Never rely on message order across different senders; only the pairwise order between two processes is guaranteed. Use `gen_server:call` correlation, monitors and references (`make_ref`/`alias`), not sleeps. OTP 28 priority messages are opt-in; do not use them to fix ordering bugs. [CONC-05] | Ordering assumptions are the main logic race in Erlang | MUST | [Processes: signals](https://www.erlang.org/doc/system/ref_man_processes.html), [OTP 28 highlights](https://www.erlang.org/blog/highlights-otp-28/) | missing | review |
| `ERL-CONC-06` | Async pitfalls: every `gen_server:call` has a timeout (default 5000 ms, choose it); never call yourself; avoid `gen_server:call` cycles; do not do blocking IO in a gen_server that serves many callers: hand off to a worker or use `gen_server:reply/2`. [CONC-05] | Calls block; a stuck callback blocks all clients | MUST | [gen_server](https://www.erlang.org/doc/apps/stdlib/gen_server.html), [Adopting Erlang](https://adoptingerlang.org/docs/development/) | missing | review |
| `ERL-CONC-07` | Mailbox overload: bound queues (admission control, a pool such as `poolboy`/`pooler`, or a `max_heap_size` and `message_queue_data` flag on the process); selective `receive` over a large mailbox is O(n), avoid it; use `process_info(Pid, message_queue_len)` as a health metric. [CONC-05] | An unbounded mailbox grows until the VM dies, and burst load has no natural backpressure | MUST | [erlang module](https://www.erlang.org/doc/apps/erts/erlang.html), [Efficiency guide: processes](https://www.erlang.org/doc/system/eff_guide_processes.html), [recon](https://github.com/ferd/recon) | missing | review |
| `ERL-CONC-08` | Background work: use supervised workers, `proc_lib`/`gen_*` (not bare `spawn`), `erpc` for remote calls with timeout, and a graceful stop via `application:stop` and `init:stop()` with supervisor `shutdown` times. [CONC-06] | Bare `spawn` is invisible to supervisors and crash reports | MUST | [proc_lib](https://www.erlang.org/doc/apps/stdlib/proc_lib.html), [erpc](https://www.erlang.org/doc/apps/kernel/erpc.html) | go-only (graceful shutdown GO_BEST) | review |
| `ERL-CONC-09` | Distribution use: keep to horizontal clusters; do not couple application and DB tiers over Erlang distribution; use `pg` for groups. [CONC-02] | Distribution removes isolation between nodes | SHOULD | [EEF distribution](https://security.erlef.org/secure_coding_and_deployment_hardening/distribution) | missing | review |

## 6. TEST: testing

**Coverage recipe:** tool = OTP `cover` via rebar3. Enable with `{cover_enabled, true}.` and `{cover_export_enabled, true}.` in `rebar.config`. Run `rebar3 eunit --cover` then `rebar3 ct --cover` (data lands in `_build/test/cover/eunit.coverdata` and `ct.coverdata`); `rebar3 cover --verbose` prints a table and writes `_build/test/cover/index.html`; `rebar3 cover --min_coverage 70` fails below a threshold. Cobertura XML: `{project_plugins, [covertool]}.`, then `rebar3 covertool generate` with `{covertool, [{coverdata_files, ["eunit.coverdata", "ct.coverdata"]}]}`. Granularity is line (cover counts executions per line); no branch coverage. New-code coverage: loops reads the Cobertura XML (line hits per file), intersects it with `git diff` added lines against the base, and gates at >= 70%. covertool writes `_build/test/covertool/<app>.covertool.xml`, one file per OTP app (verified by spot run); loops merges the glob.

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `ERL-TEST-01` | Use EUnit for unit tests (`*_tests.erl` next to or under `test/`), Common Test for integration and system tests (`*_SUITE.erl`), PropEr for properties. Run with `rebar3 eunit`, `rebar3 ct`, `rebar3 proper`. [TEST-01] | EUnit is light; CT has suites, groups, config, hooks, logs | MUST | [EUnit](https://www.erlang.org/doc/apps/eunit/chapter.html), [Common Test](https://www.erlang.org/doc/apps/common_test/introduction.html), [rebar3_proper](https://github.com/ferd/rebar3_proper) | go-only (`go test`) | gate |
| `ERL-TEST-02` | Test file globs: `**/test/*_SUITE.erl`, `**/test/*_tests.erl`, `**/src/*_tests.erl`, `**/test/prop_*.erl`, `**/test/*_SUITE_data/**`; `rebar3 ct` compiles every `*_SUITE.erl` in `test/` but not recursively. [TEST-02] | loops must tell test files from code | MUST | [rebar3 ct](https://rebar3.org/docs/testing/ct/) | go-only (`*_test.go`) | hook |
| `ERL-TEST-03` | Style: one CT suite per use case with `all/0`, `groups/0`, `init_per_suite`/`end_per_suite`, `init_per_testcase`; test names `<function>_<condition>_<result>`; EUnit uses generators and fixtures (`{setup, ...}`, `{foreach, ...}`) rather than copy-paste. [TEST-03] | Fixtures give setup, teardown and parallel groups | SHOULD | [Common Test](https://www.erlang.org/doc/apps/common_test/introduction.html), [EUnit](https://www.erlang.org/doc/apps/eunit/chapter.html) | go-only (table-driven in `skills/testing`) | review |
| `ERL-TEST-04` | Prefer fakes (an ETS-backed or in-memory implementation of the port behaviour) over mocks; use `meck` only for third-party modules you cannot wrap, and `meck:unload` in teardown. [TEST-04] | Mocking global modules is process-global and breaks parallel tests | SHOULD | [meck](https://hex.pm/packages/meck), [OTP design principles](https://www.erlang.org/doc/system/design_principles.html) | go-only (fakes in `skills/testing`) | review |
| `ERL-TEST-05` | Integration tests use real Postgres (Testcontainers or docker compose) or a throw-away schema per CT suite; start the system under test with `application:ensure_all_started/1` in `init_per_suite` and stop it in `end_per_suite`. [TEST-05] | Adapters need a real DB; app start is the real wiring | SHOULD | [Common Test](https://www.erlang.org/doc/apps/common_test/introduction.html), [epgsql](https://hex.pm/packages/epgsql) | go-only (`httptest`) | review |
| `ERL-TEST-06` | Coverage: run `rebar3 eunit --cover` and `rebar3 ct --cover` with `{cover_enabled, true}`, then `rebar3 covertool generate` for Cobertura XML. Exclude generated modules with `{cover_excl_mods, [...]}` and apps with `cover_excl_apps`. [TEST-06] | Line coverage only; Cobertura is parseable | MUST | [rebar3 config](https://rebar3.org/docs/configuration/configuration/), [rebar3 commands](https://rebar3.org/docs/commands/), [covertool](https://github.com/covertool/covertool) | go-only (`go test -coverprofile`, `gocover` adapter) | gate |
| `ERL-TEST-07` | New-code coverage: loops parses the Cobertura XML from covertool, maps files to added lines from `git diff`, requires >= 70%. Keep cover data from both EUnit and CT in `coverdata_files`. [TEST-07] | Diff-cover supports Cobertura XML; native cover does not do diffs | MUST | [covertool](https://github.com/covertool/covertool), [diff-cover](https://github.com/Bachmann1234/diff_cover), [WORKFLOW §6](../../specs/WORKFLOW.md) | go-only (`internal/application/gate/coverage.go`, gocover parser) | gate |
| `ERL-TEST-08` | Property tests: PropEr (`?FORALL`, `proper_statem`, `proper_fsm`) via `rebar3 proper -n 500 -c`; set `{proper_opts, [{numtests, 500}]}`; use properties for pure domain functions and stateful models for gen_servers, gen_statems and ETS code. Use `proper_statem` with `parallel_commands` to hunt races. [TEST-08] | Properties find edge cases; stateful PropEr is the closest thing to race testing | SHOULD | [PropEr](https://github.com/proper-testing/proper), [rebar3_proper](https://github.com/ferd/rebar3_proper) | go-only (fuzz in `skills/testing`) | gate |
| `ERL-TEST-09` | Flakiness: no `timer:sleep` for sync (wait on a monitor or message); shuffle CT groups with `{group, g, [shuffle]}`; use a unique registered name or port per test; clock and RNG injected; `rebar3 ct --repeat N` to hunt flakes; parallel groups with `[parallel]` only for isolated tests. [TEST-09] | Sleeps and shared names cause nondeterminism | MUST | [Common Test](https://www.erlang.org/doc/apps/common_test/introduction.html), [rebar3 commands](https://rebar3.org/docs/commands/) | covered (`skills/consistency`, `skills/testing`) | review |
| `ERL-TEST-10` | TDD loop: `rebar3 eunit --module=m`, `rebar3 eunit --test=m:t_test`, `rebar3 ct --suite=test/x_SUITE --case=c`, `rebar3 ct --group=g`; use `rebar3 shell` plus `r3:do(eunit)`/`r3:do(compile)` (the plugin `rebar3_auto` or `rebar3_reload` for watch). [TEST-10] | Single-test runs avoid the full suite; rebar3 compiles incrementally | SHOULD | [rebar3 commands](https://rebar3.org/docs/commands/) | go-only (`go test -run`) | review |
| `ERL-TEST-11` | Use `-define(TEST, true)` sparingly; guard `-ifdef(TEST).` test code in `src/` and compile with the `test` profile. Tests run in profile `test` (`_build/test`). [TEST-02] | Test builds are separate from default builds | SHOULD | [rebar3 ct](https://rebar3.org/docs/testing/ct/) | missing | review |
| `ERL-TEST-12` | Emit JUnit XML in CI with `{ct_hooks, [cth_surefire]}` in `ct_opts` and `{eunit_opts, [{report, {eunit_surefire, [{dir, "."}]}}]}`. [TEST-01] | CI test reporting | SHOULD | [rebar3 config](https://rebar3.org/docs/configuration/configuration/) | missing | review |

## 7. PERF: performance and profiling

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `ERL-PERF-01` | Benchmark with `erlperf` (`timer:tc/3` is too coarse) (Hex) for continuous benchmarks and compare statistically across runs. [PERF-01] | `erlperf` measures throughput and concurrency | SHOULD | [erlperf](https://hex.pm/packages/erlperf), [Efficiency guide](https://www.erlang.org/doc/system/efficiency_guide.html) | go-only (benchstat) | review |
| `ERL-PERF-02` | Profile with `tprof` (OTP 27+: call time, call count, memory), `eprof`, `fprof` (heavy), `msacc` for scheduler utilisation, and `recon` in production; `observer` locally. [PERF-02] | `tprof` replaces most of eprof/cprof use | SHOULD | [tprof](https://www.erlang.org/doc/apps/tools/tprof.html), [msacc](https://www.erlang.org/doc/apps/runtime_tools/msacc.html), [recon](https://github.com/ferd/recon) | go-only (pprof) | review |
| `ERL-PERF-03` | Tuning: pick scheduler flags only from measurements (`+S`, `+sbwt`, `+hms` heap size, `+P` process limit, `+Q` ports, `+zdbbl` distribution buffer); keep the JIT on (default since OTP 24 on x86-64 and aarch64). [PERF-03] | Defaults are good; big changes need data | SHOULD | [erl command](https://www.erlang.org/doc/apps/erts/erl_cmd.html) | missing | review |
| `ERL-PERF-04` | Hot-path mistakes: big messages copied between processes (pass references or use ETS), `length(L)` in guards, `lists:nth` loops, `++` accumulation, `list_to_binary` on large iolists (send iolists to the socket), non-tail-recursive loops, huge `gen_server` state copied by `process_info`. [PERF-04] | Copy cost and mailbox growth dominate | SHOULD | [Efficiency guide](https://www.erlang.org/doc/system/efficiency_guide.html), [functions](https://www.erlang.org/doc/system/eff_guide_functions.html) | go-only (GO_BEST §22, §31-§33) | review |
| `ERL-PERF-05` | Use binary-matching patterns that keep match contexts (avoid re-creating sub-binaries in the middle) and keep binaries that outlive a request from retaining big parent binaries (`binary:copy/1`). [PERF-04] | Sub-binary leaks hold memory | SHOULD | [Efficiency guide: binaries](https://www.erlang.org/doc/system/binaryhandling.html) | missing | review |

## 8. SEC: security

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `ERL-SEC-01` | Use parameterised queries only (`epgsql:equery/3` with `$1`, `pgo:query/2` with params); never build SQL with `io_lib:format` or `++`. [SEC-01] | SQL injection | MUST | [OWASP SQLi](https://cheatsheetseries.owasp.org/cheatsheets/SQL_Injection_Prevention_Cheat_Sheet.html), [epgsql](https://hex.pm/packages/epgsql) | go-only (GO_BEST §38) | review |
| `ERL-SEC-02` | Never call `os:cmd/1` or `open_port({spawn, ...})` with user input; use `{spawn_executable, Path}` with an `args` list. [SEC-01] | Shell injection | MUST | [EEF hardening](https://security.erlef.org/), [erlang module](https://www.erlang.org/doc/apps/erts/erlang.html) | go-only | review |
| `ERL-SEC-03` | Decode untrusted `erlang:binary_to_term/2` only with `[safe]`, and never deserialise funs. `safe` prevents new atoms but "does not affect the deserialisation of functions and other unsafe terms", so prefer JSON, Protobuf or CBOR on the wire. Do not use `file:consult/1` on untrusted input. [SEC-02] | RCE through funs, atom-table exhaustion DoS | MUST | [EEF serialisation](https://security.erlef.org/secure_coding_and_deployment_hardening/serialisation), [binary_to_term/2](https://www.erlang.org/doc/apps/erts/erlang.html), [OWASP deserialization](https://cheatsheetseries.owasp.org/cheatsheets/Deserialization_Cheat_Sheet.html) | missing (GO_BEST §38 is Go) | review |
| `ERL-SEC-04` | Never create atoms from JSON keys or user strings: configure the JSON library to return binaries (`jsx` default is binary keys, `thoas`/`json` module keys as binaries) and validate with a schema. [SEC-02] | Atom exhaustion kills the VM | MUST | [EEF serialisation](https://security.erlef.org/secure_coding_and_deployment_hardening/serialisation), [jsx](https://hex.pm/packages/jsx) | missing | review |
| `ERL-SEC-05` | Distribution: a cookie is access control only (traffic is unencrypted, no MITM protection). Use TLS distribution with `{verify, verify_peer}` and mutual certs; bind EPMD and `inet_dist_use_interface` to an internal interface; never expose port 4369 or the distribution ports; do not log or commit `.erlang.cookie`; use a random cookie from the environment for releases. [SEC-04] | A leaked cookie is remote code execution | MUST | [EEF distribution](https://security.erlef.org/secure_coding_and_deployment_hardening/distribution), [ssl](https://www.erlang.org/doc/apps/ssl/ssl.html) | missing | review |
| `ERL-SEC-06` | Secrets come from the environment or a vault via `sys.config.src` and `vm.args.src` substitution, never from git; set `{secret, fun() -> ... end}` wrappers when logging config. [SEC-04] | Configs get logged in crash dumps and `sys:get_state` | MUST | [rebar3 releases](https://rebar3.org/docs/deployment/releases/), [OWASP](https://cheatsheetseries.owasp.org/) | go-only (GO_BEST §28) | review |
| `ERL-SEC-07` | Crypto: use OTP `crypto` and `public_key` (`crypto:strong_rand_bytes/1`, never `rand` for secrets); hash passwords with Argon2id or bcrypt via a maintained NIF (for example `bcrypt` or an Argon2 NIF from Hex), never plain `crypto:hash` (SHA-2) alone. [SEC-04] | `rand` is not cryptographic | MUST | [crypto](https://www.erlang.org/doc/apps/crypto/crypto.html), [OWASP passwords](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html), [rand](https://www.erlang.org/doc/apps/stdlib/rand.html) | go-only | review |
| `ERL-SEC-08` | HTTP clients and servers: enable TLS verification (`{verify, verify_peer}`, `{cacerts, public_key:cacerts_get()}` on OTP 25+, `customize_hostname_check`); validate redirects and URLs against SSRF; Cowboy needs explicit CORS and CSRF handling. [SEC-03] | `ssl` defaults to verify_peer since OTP 26 but older code and `httpc` options may not | MUST | [ssl](https://www.erlang.org/doc/apps/ssl/ssl.html), [OWASP SSRF](https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html) | missing | review |
| `ERL-SEC-09` | Validate input at the adapter (Cowboy handler or `cowboy_rest`) with a schema library (`jesse` for JSON Schema) or pattern-matching decoders, before calling use cases. [SEC-05] | Validation belongs at the edge | MUST | [jesse](https://hex.pm/packages/jesse), [OWASP validation](https://cheatsheetseries.owasp.org/cheatsheets/Input_Validation_Cheat_Sheet.html) | go-only | review |
| `ERL-SEC-10` | Supply chain: Hex packages have checksums recorded in `rebar.lock`; keep plugins pinned; review `rebar.config` hooks (`pre_hooks`, `provider_hooks`) in dependencies because they run shell commands at compile time. [SEC-06] | Compile-time hooks execute code | MUST | [rebar3 dependencies](https://rebar3.org/docs/configuration/dependencies/), [rebar3 hex](https://hexdocs.pm/rebar3_hex/) | missing | review |
| `ERL-SEC-11` | SAST: there is no gosec/sobelow-class Erlang tool in common use; use Dialyzer, Elvis, `rebar3_audit`, and an EEF-based Semgrep rule set where available; ELP's linter covers some diagnostics. [SEC-07] | Honest gap | SHOULD | [EEF Security WG](https://security.erlef.org/), [ELP](https://whatsapp.github.io/erlang-language-platform/) | missing | gate |

## 9. DATA: persistence

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `ERL-DATA-01` | Use raw SQL with a driver (`epgsql`, `pgo`; `mysql-otp` for MySQL) behind a repository behaviour; there is no dominant Erlang ORM. Mnesia, ETS and DETS are for in-memory or node-local state. [DATA-01] | Keeps SQL visible and the adapter thin | SHOULD | [epgsql](https://hex.pm/packages/epgsql), [pgo](https://hex.pm/packages/pgo), [Mnesia](https://www.erlang.org/doc/apps/mnesia/mnesia.html) | missing | review |
| `ERL-DATA-02` | Migrations: keep forward-only versioned `.sql` files in `priv/migrations` and run them with a language-neutral tool (Flyway, dbmate, sqitch) or an in-app runner at release start; no migrator is dominant in Erlang. Use expand-then-contract for zero downtime. [DATA-02] | Erlang lacks a standard migration framework | SHOULD | [Flyway docs](https://flywaydb.org/documentation/) | covered (`skills/consistency` migrations) | review |
| `ERL-DATA-03` | Transactions: begin and commit in the adapter, expose a use-case-level unit of work (a function taking a fun); keep the transaction body free of side effects. Mnesia: use `mnesia:transaction/1` and avoid dirty ops in the domain. [DATA-03] | Boundary is at the use case | MUST | [Mnesia](https://www.erlang.org/doc/apps/mnesia/mnesia.html), [HEXAGONAL](../../specs/HEXAGONAL_ARCHITECTURE.md) | covered (HEXAGONAL persistence) | review |
| `ERL-DATA-04` | Connections: use a pool (`pgo` has one; `epgsql_pool`/`poolboy` otherwise), set query and checkout timeouts, size the pool to the DB, and let a crashed connection process be restarted by its supervisor. [DATA-05] | Unpooled connections exhaust the DB | MUST | [pgo](https://hex.pm/packages/pgo), [supervisor principles](https://www.erlang.org/doc/system/sup_princ.html) | missing | review |
| `ERL-DATA-05` | Mnesia is not a general database: avoid network partitions (no automatic merge), limit table size (DETS 2 GB), and prefer Postgres for durable business data. [DATA-04] | Split-brain risk | AVOID | [Mnesia](https://www.erlang.org/doc/apps/mnesia/mnesia.html) | missing | review |

## 10. API: APIs and integration

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `ERL-API-01` | HTTP: Cowboy 2 (routes compiled with `cowboy_router:compile/1`, handlers `init/2`, middlewares, streams); use `cowboy_rest` for resources, plain handlers for RPC, `cowboy_websocket` for sockets; set `idle_timeout`, `max_keepalive` and request limits on the listener. [API-01] | Cowboy defaults are generic | MUST | [Cowboy](https://github.com/ninenines/cowboy), [Cowboy on Hex](https://hex.pm/packages/cowboy), [docs index](https://ninenines.eu/docs/) | go-only (`httptest`) | review |
| `ERL-API-02` | Contracts: OpenAPI first (with `cowboy_swagger` or generate handlers), JSON Schema validation with `jesse`, Protobuf with `gpb` or `grpcbox`, version in the path. [API-02] | A contract decouples clients | SHOULD | [cowboy_swagger](https://hex.pm/packages/cowboy_swagger), [jesse](https://hex.pm/packages/jesse) | missing | review |
| `ERL-API-03` | Errors: map domain error tuples to RFC 9457 problem details at the adapter; never leak crash reasons or stack traces in responses. [API-03] | Information disclosure | MUST | [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457), [Cowboy](https://github.com/ninenines/cowboy) | go-only (error taxonomy HEXAGONAL §10) | review |
| `ERL-API-04` | Resilience: timeouts on every `gun`/`hackney`/`httpc` call, retries with backoff and jitter only for idempotent calls, circuit breaker (`fuse`), rate limit (`jobs`, `throttle`), idempotency keys. [API-04] | Remote calls fail | MUST | [gun](https://hex.pm/packages/gun), [hackney](https://hex.pm/packages/hackney) | covered (`skills/jobs-and-state-machines`, `consistency`) | review |
| `ERL-API-05` | Messaging: `brod` for Kafka, `amqp_client` for RabbitMQ; use an outbox table and at-least-once handlers that are idempotent; commit offsets after processing. [API-05] | Broker delivery is at-least-once | MUST | [brod](https://hex.pm/packages/brod), [amqp_client](https://hex.pm/packages/amqp_client) | covered (`skills/consistency` idempotency/outbox) | review |
| `ERL-API-06` | HTTP client: `gun` (HTTP/1.1, 2, WebSocket, connection reuse) or `hackney`; `httpc` ships with OTP but needs explicit TLS and profile settings. Reuse connections, pass deadlines. [API-06] | `httpc` defaults are lax | SHOULD | [gun](https://hex.pm/packages/gun), [httpc](https://www.erlang.org/doc/apps/inets/httpc.html) | go-only (GO_BEST §10) | review |

## 11. OBS: observability

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `ERL-OBS-01` | Log with OTP `logger` (`?LOG_INFO(#{event => ..., id => Id})`, structured maps, domains, level filters); set a JSON formatter handler for production; include a correlation id in process metadata (`logger:update_process_metadata/1`). Never log secrets or PII. Do not use `io:format` for logging. [OBS-01] | `logger` is built in (OTP 21+), handlers are configurable | MUST | [Logger](https://www.erlang.org/doc/apps/kernel/logger_chapter.html) | go-only (GO_BEST §24) | review |
| `ERL-OBS-02` | Metrics: emit with `telemetry` (`telemetry:execute/3`, event names as atom lists) and export through a reporter such as `prometheus.erl`; name `snake_case` with units. [OBS-02] | `telemetry` is the BEAM-wide convention | SHOULD | [telemetry](https://hexdocs.pm/telemetry/), [prometheus.erl](https://hex.pm/packages/prometheus) | go-only (GO_BEST §47) | review |
| `ERL-OBS-03` | Tracing: OpenTelemetry Erlang (`opentelemetry_api` in code, `opentelemetry` SDK and `opentelemetry_exporter` in the release; list the exporter before the SDK and set the SDK `temporary`); `opentelemetry_cowboy` for Cowboy. Requires OTP 23+. [OBS-03] | Context propagates through the process dictionary, not across spawns; pass it explicitly | SHOULD | [opentelemetry-erlang](https://github.com/open-telemetry/opentelemetry-erlang), [OTel Erlang docs](https://opentelemetry.io/docs/languages/erlang/), [opentelemetry_cowboy](https://hex.pm/packages/opentelemetry_cowboy) | missing | review |
| `ERL-OBS-04` | Health: expose `/health/live` (process is up) and `/health/ready` (dependencies reachable) from Cowboy; readiness checks pools and supervisor children, not just HTTP 200. [OBS-04] | Orchestrators need distinct signals | SHOULD | [Cowboy](https://github.com/ninenines/cowboy) | missing | review |
| `ERL-OBS-05` | Production introspection: ship `recon` and `observer_cli` in the release; keep `sys:get_status` and `erlang:system_info` usable; never expose the remote shell publicly. [OBS-02] | Live debugging is a BEAM strength | SHOULD | [recon](https://github.com/ferd/recon) | missing | review |

## 12. OPS: configuration, packaging and deployment

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `ERL-OPS-01` | Config: use `config/sys.config.src` and `vm.args.src` with `${VAR}` or `${VAR:-default}` (rebar3 3.14+); read `application:get_env` only at the edge and convert to a typed record. [OPS-01] | 12-factor config; `sys.config.src` takes precedence over `sys.config` | MUST | [rebar3 releases](https://rebar3.org/docs/deployment/releases/) | go-only (GO_BEST §28) | review |
| `ERL-OPS-02` | Package as an OTP release: `rebar3 as prod tar` (prod mode strips debug info, includes ERTS, no symlinks). Docker: multi-stage with an `erlang` build image and a slim runtime image (Debian slim or Alpine with matching libc/OpenSSL). [OPS-02] | A release is the unit of deploy; ERTS must match the target | MUST | [rebar3 releases](https://rebar3.org/docs/deployment/releases/), [Adopting Erlang releases](https://adoptingerlang.org/docs/production/releases/) | missing | review |
| `ERL-OPS-03` | Native and AOT: not applicable; the BEAM JIT compiles at load time. Do not use escripts for services. [OPS-03] | n/a | AVOID | [Efficiency guide](https://www.erlang.org/doc/system/efficiency_guide.html) | n/a | review |
| `ERL-OPS-04` | Lifecycle: handle SIGTERM through `init:stop()`; set supervisor `shutdown` values and `{shutdown, ...}` in children that need drain; stop listeners before workers (Cowboy `ranch:suspend_listener`/`stop_listener`). Use hot upgrades (`relup`, `appup`) only for stateful systems with a tested upgrade path; otherwise rolling restarts. [OPS-04] | Hot upgrades are costly to test; most teams use blue-green | SHOULD | [Release handling](https://www.erlang.org/doc/system/release_handling.html), [rebar3 commands](https://rebar3.org/docs/commands/) | go-only (GO_BEST graceful shutdown) | review |
| `ERL-OPS-05` | Build caches: `REBAR_CACHE_DIR` (hex package and plugin cache, default `~/.cache/rebar3`; keep it shared across runs, because a per-run directory re-downloads every package and plugin and rebuilds the global Dialyzer PLT) and `REBAR_GLOBAL_CONFIG_DIR` (default `~/.config/rebar3`); the build dir is `_build` in the project. Package tarballs are checksummed against `rebar.lock`, so sharing is safe; `project_plugins` also go under it (verified in a spot run). Only the per-run build dir `_build` is isolated. [OPS-05] | Re-downloading the world on every run is slow; `_build` is the per-run state | MUST | [rebar3 base config](https://rebar3.org/docs/configuration/configuration/), [issue 2762](https://github.com/erlang/rebar3/issues/2762), [issue 2633](https://github.com/erlang/rebar3/issues/2633) | go-only (`GOCACHE`, `go-env` hook) | hook |
| `ERL-OPS-06` | Artifacts left behind: `_build/`, `*.beam`, `ebin/`, `ct_logs/`, `erl_crash.dump` (path via `ERL_CRASH_DUMP`), `rebar3.crashdump`, `*.coverdata`, `bom.xml`, `*.tar.gz`, `Mnesia.*`, `.rebar3/`, `*.plt` (the global PLT is in the cache dir). Clean with `rebar3 clean --all` and delete the rest. [OPS-06] | The loops cleanup list | MUST | [rebar3 basic usage](https://rebar3.org/docs/basic_usage/), [OTP crash dump](https://www.erlang.org/doc/apps/erts/crash_dump.html) | go-only (`*.test`, `*.prof`) | hook |

## 13. DOC: documentation and review

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `ERL-DOC-01` | Document with `-moduledoc` and `-doc` attributes (OTP 27+, Markdown) and generate with ExDoc (`rebar3 ex_doc`); EDoc remains for old code. Examples go in doctests only if you run them. [DOC-01] | Native docs replace EDoc since OTP 27 | SHOULD | [OTP documentation](https://www.erlang.org/doc/system/documentation.html), [ExDoc](https://hex.pm/packages/ex_doc) | go-only (GO_BEST §26) | review |
| `ERL-DOC-02` | README, ADRs and `CHANGELOG.md` per release; document the supervision tree in the README or an ADR. [DOC-02] | New contributors read the tree first | SHOULD | [Adopting Erlang](https://adoptingerlang.org/) | covered (`skills/documentation`, HEXAGONAL ADRs) | review |
| `ERL-DOC-03` | Review focus: crash vs handle, unsupervised processes, unbounded mailboxes, atom creation from input, call timeouts, `catch`-alls, record sharing across apps, missing `-spec`, ETS ownership. [DOC-03] | The Erlang-specific review checklist | MUST | [Inaka guidelines](https://github.com/inaka/erlang_guidelines), [Robustness](https://www.erlang.org/doc/system/robustness.html) | covered (`skills/review-integration`) | review |
| `ERL-DOC-04` | Keep `-spec`, `-callback` and `-opaque` current as the primary API documentation; Dialyzer fails the build when they drift. [DOC-01] | Types as docs | SHOULD | [typespec](https://www.erlang.org/doc/system/typespec.html) | missing | gate |

## 14. FW: frameworks

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `ERL-FW-01` | Dominant frameworks: Cowboy 2.21 (151.9 million all-time Hex downloads, updated 2026-11-09), Elli (small, HTTP/1.1), Nova (0.18.1, 2026-10-15; MVC-like on Cowboy), plus Phoenix on the Elixir side. Pick Cowboy for hexagonal services. [FW-01] | Cowboy is the de-facto base | SHOULD | [Cowboy on Hex](https://hex.pm/packages/cowboy), [Elli](https://github.com/elli-lib/elli), [Nova](https://hex.pm/packages/nova) | missing | review |
| `ERL-FW-02` | Cowboy: routes and handlers live in the inbound adapter app; handlers only decode, call a use case, encode; the Ranch listener is started by the release app. Nova imposes controllers and views under `src/controllers`, so keep use cases outside and call them from controllers. [FW-02] | Project rules win; framework layout is an adapter detail | MUST | [Cowboy](https://github.com/ninenines/cowboy), [Nova](https://github.com/novaframework/nova) | go-only (project rules win, WORKFLOW §1) | review |
| `ERL-FW-03` | Build tool: erlang.mk is the alternative to rebar3 (Makefile-based; targets for EUnit, CT, PropEr, Concuerror, Xref, Dialyzer, cover, relx). Use whichever the repo already has; a mixed repo needs both lock and Makefile checks. [FW-02] | Respect the project's build tool | SHOULD | [Erlang.mk guide](https://erlang.mk/guide/) | missing | review |
| `ERL-FW-04` | Framework footguns: Cowboy handler crash kills the request process (fine) but a crash in a long-lived `init/2` state leaks; `cowboy_rest` default callbacks differ from expectations (e.g. `content_types_accepted`); Ranch `max_connections` default 1024; WebSocket handlers need ping and idle timeouts. [FW-03] | Common production incidents | SHOULD | [Cowboy](https://github.com/ninenines/cowboy), [docs index](https://ninenines.eu/docs/) | missing | review |

## Top 10 footguns

1. **Atom exhaustion.** Bad: `binary_to_atom(Key, utf8)` or `list_to_atom/1` on request data. Fix: `binary_to_existing_atom/2` or a lookup table. ([EEF](https://security.erlef.org/secure_coding_and_deployment_hardening/serialisation))
2. **Unsafe `binary_to_term`.** Bad: `binary_to_term(Bin)` on network input. Fix: `binary_to_term(Bin, [safe])` and still never decode funs; use JSON or Protobuf. ([EEF](https://security.erlef.org/secure_coding_and_deployment_hardening/serialisation))
3. **Distribution cookie as security.** Bad: exposing EPMD (4369) and distribution ports with a default cookie. Fix: TLS distribution, bind to an internal interface. ([EEF](https://security.erlef.org/secure_coding_and_deployment_hardening/distribution))
4. **Unbounded mailbox.** Bad: a gen_server fed faster than it consumes. Fix: admission control, pools, bounded queues, `max_heap_size`, monitor `message_queue_len`. ([erlang module](https://www.erlang.org/doc/apps/erts/erlang.html))
5. **Selective receive over a large mailbox.** Bad: `receive {Ref, R} -> ...` with thousands of queued messages. Fix: use `gen_server:call` or aliases (OTP 24+ optimises receive on fresh references). ([Efficiency guide](https://www.erlang.org/doc/system/eff_guide_processes.html))
6. **Catching everything.** Bad: `try f() catch _:_ -> ok end`. Fix: let it crash or catch a specific class with the stacktrace. ([Robustness](https://www.erlang.org/doc/system/robustness.html))
7. **`gen_server:call` deadlock or default timeout.** Bad: call cycle or a call into a busy process with the 5 s default. Fix: cast, async reply, bounded timeouts. ([gen_server](https://www.erlang.org/doc/apps/stdlib/gen_server.html))
8. **Bare `spawn`.** Bad: `spawn(fun() -> work() end)` outside the supervision tree. Fix: `proc_lib` under a supervisor with a child spec. ([proc_lib](https://www.erlang.org/doc/apps/stdlib/proc_lib.html))
9. **ETS check-then-act.** Bad: `ets:lookup` then `ets:insert` from many processes. Fix: `ets:update_counter/4`, `ets:insert_new/2`, or a single owner process. ([ets](https://www.erlang.org/doc/apps/stdlib/ets.html))
10. **Sub-binary and large-message copies.** Bad: sending big terms between processes or keeping a small slice of a large binary. Fix: `binary:copy/1`, ETS or `persistent_term` for shared read-mostly data. ([Efficiency guide](https://www.erlang.org/doc/system/efficiency_guide.html))

## 15. LOOPS: tooling data

`rebar3` argv are from the rebar3 commands page; `rebar3 audit` and the Dialyzer cache path are flagged in Open questions; `REBAR_CACHE_DIR` and `REBAR_GLOBAL_CONFIG_DIR` stay shared (ERL-OPS-05), so only `ERL_CRASH_DUMP` is scoped per run. Coverage reads Cobertura XML produced by `covertool`.

```json
{
  "id": "erlang",
  "name": "Erlang",
  "versions": {"stable": "29.1", "lts": null, "as_of": "2026-11"},
  "detect": [{"file": "rebar.config"}, {"file": "rebar.lock"}, {"file": "erlang.mk"}],
  "source_globs": ["**/src/*.erl", "**/include/*.hrl", "**/src/*.app.src"],
  "test_globs": ["**/test/*_SUITE.erl", "**/test/*_tests.erl", "**/src/*_tests.erl", "**/test/prop_*.erl"],
  "generated": {"globs": ["**/_build/**", "**/*_pb.erl", "**/*_pb.hrl", "**/*_gen.erl", "**/*.yrl.erl"], "header_regex": "^%% (Generated|Autogenerated|This file is generated).*$"},
  "checks": [
    {"name": "compile", "kind": "build", "argv": ["rebar3", "compile"], "required": true, "tool": "rebar3"},
    {"name": "fmt", "kind": "format", "argv": ["rebar3", "fmt", "--check"], "required": false, "tool": "erlfmt", "install": "add {project_plugins, [erlfmt]} to rebar.config"},
    {"name": "xref", "kind": "lint", "argv": ["rebar3", "xref"], "required": true, "tool": "rebar3"},
    {"name": "elvis", "kind": "lint", "argv": ["rebar3", "lint"], "required": false, "tool": "elvis", "install": "add {project_plugins, [rebar3_lint]} to rebar.config and create elvis.config; run only when elvis.config exists", "when_file": "elvis.config"},
    {"name": "dialyzer", "kind": "typecheck", "argv": ["rebar3", "dialyzer"], "required": true, "tool": "dialyzer"},
    {"name": "eunit", "kind": "test", "argv": ["rebar3", "eunit", "--cover"], "required": true, "tool": "rebar3"},
    {"name": "ct", "kind": "test", "argv": ["rebar3", "ct", "--cover"], "required": true, "tool": "rebar3"},
    {"name": "proper", "kind": "test", "argv": ["rebar3", "proper", "-n", "100", "--cover"], "required": false, "tool": "rebar3_proper", "install": "add {project_plugins, [rebar3_proper]} and a proper dep in the test profile"},
    {"name": "cover", "kind": "coverage", "argv": ["rebar3", "cover", "--verbose"], "required": true, "tool": "rebar3"},
    {"name": "covertool", "kind": "coverage", "argv": ["rebar3", "covertool", "generate"], "required": true, "tool": "covertool", "install": "add {project_plugins, [covertool]} and {cover_export_enabled, true} to rebar.config"},
    {"name": "audit", "kind": "vuln", "argv": ["rebar3", "audit"], "required": false, "tool": "rebar3_audit", "install": "add {project_plugins, [rebar3_audit]} to rebar.config"},
    {"name": "sbom", "kind": "sbom", "argv": ["rebar3", "sbom"], "required": false, "tool": "rebar3_sbom", "install": "add {project_plugins, [rebar3_sbom]} to rebar.config"}
  ],
  "test_commands": [
    {"argv_prefix": ["rebar3", "eunit"]},
    {"argv_prefix": ["rebar3", "ct"]},
    {"argv_prefix": ["rebar3", "proper"]},
    {"argv_prefix": ["rebar3", "do"]}
  ],
  "race": {
    "mode": "none",
    "flags": [],
    "env": null,
    "variants": [{"ERL_FLAGS": "+S 1:1"}, {"ERL_FLAGS": "+S 8:8"}],
    "reason": "BEAM processes share nothing, so there is no data-race sanitizer or flag. ETS, message-order and timing races remain.",
    "substitute": "Run eunit and ct once per entry of variants (ERL_FLAGS='+S 1:1' then '+S 8:8'), repeat with rebar3 ct --repeat, require a proper_statem test for every gen_statem, ETS-backed or multi-process module; Concuerror advisory only (stale: last release 2020, OTP 20.3 to 23.3 per README)."
  },
  "coverage": {"format": "cobertura", "path": "_build/test/covertool/*.covertool.xml", "granularity": "line", "notes": "One Cobertura file per OTP app; loops merges the glob. Source data is _build/test/cover/{eunit,ct}.coverdata."},
  "post_edit": {"format_file": ["rebar3", "fmt", "-w", "{file}"], "lint_dir": ["rebar3", "xref"]},
  "cache_env": {"ERL_CRASH_DUMP": "tmp"},
  "artifacts": ["_build", "*.beam", "ct_logs", "erl_crash.dump", "rebar3.crashdump", "*.coverdata", "bom.xml", "bom.json", "Mnesia.*", ".rebar3"],
  "arch": {
    "import_regex": "\\b([a-z][a-zA-Z0-9_]*):[a-z][a-zA-Z0-9_]*\\(",
    "resolver": "module-name",
    "native_tool": "rebar3 xref",
    "forbidden_in_domain": ["gen_server", "gen_statem", "gen_event", "supervisor", "ets", "dets", "mnesia", "file", "gen_tcp", "gen_udp", "httpc", "ssl", "os", "application", "persistent_term", "logger"],
    "impure_calls": ["os:timestamp", "erlang:system_time", "erlang:monotonic_time", "erlang:now", "rand:", "crypto:strong_rand_bytes", "erlang:send", "spawn", "self()", "!"]
  },
  "test_jobs": null,
  "config_files": ["rebar.config", "elvis.config", ".editorconfig", ".tool-versions", "rebar.lock"],
  "sources": {
    "checks": "https://rebar3.org/docs/commands/",
    "race": "https://www.erlang.org/doc/system/ref_man_processes.html",
    "coverage": "https://github.com/covertool/covertool",
    "format": "https://github.com/WhatsApp/erlfmt",
    "lint": "https://github.com/inaka/rebar3_lint",
    "proper": "https://github.com/ferd/rebar3_proper",
    "sbom": "https://github.com/erlef/rebar3_sbom",
    "audit": "https://hex.pm/packages/rebar3_audit",
    "cache": "https://rebar3.org/docs/configuration/configuration/",
    "versions": "https://www.erlang.org/downloads"
  }
}
```

## Gaps vs loops

| Rule ID | Gap | loops today | Priority | Where loops should fix it (spec / skill / role / gate / hook / arch) |
|---|---|---|---|---|
| `ERL-CONC-04` / race policy | No race flag; substitute is multi-scheduler reruns plus PropEr statem; loops has no `race.mode = none` handling | go-only (race-guard) | P0 | gate: language pack `race` block with `substitute`; WORKFLOW §6 wording; `race-guard` hook accepts `none` |
| `ERL-TEST-06` / `-07` | Coverage needs `rebar3 covertool generate` and a Cobertura reader (glob `_build/test/covertool/*.covertool.xml`, one file per app) | go-only (gocover) | P0 | gate: Cobertura adapter, per-pack coverage recipe; `internal/application/gate/coverage.go` |
| `ERL-TEST-02` | Test globs (`*_SUITE.erl`, `*_tests.erl`, `prop_*.erl`) unknown to loops ownership and test guard | go-only | P0 | hook: test guard and `plancheck` ownership globs from pack |
| `ERL-TOOL-02`..`-08` | The gate runs Go commands; fmt, xref, Elvis, Dialyzer, audit are not run | go-only | P0 | gate: per-language `checks` from the pack; `internal/application/gate/gate.go` `runCheck` |
| `ERL-OPS-05` / `-06` | `go-env` and cleanup know only `GOCACHE`; Erlang needs `REBAR_CACHE_DIR`, `REBAR_GLOBAL_CONFIG_DIR`, `ERL_CRASH_DUMP`, `_build` cleanup | go-only | P0 | hook: `go-env`; `internal/domain/workspace`, pack `cache_env`/`artifacts` |
| `ERL-SEC-03` / `-04` / `-05` | No guidance on `binary_to_term [safe]`, atom exhaustion, distribution cookie | missing | P0 | skill: new `erlang-security` reference; reviewer checklist |
| `ERL-ARCH-01`..`-07` | HEXAGONAL layout is Go (`internal/`); Erlang needs app-as-layer mapping, behaviours as ports, domain purity list | go-only | P1 | spec: per-language arch mapping; arch: pack `arch` block, xref/native_tool support |
| `ERL-ARCH-08`..`-10` | OTP process design (when to use a process, supervisor strategies, gen_statem) not covered | missing | P1 | skill: `erlang-otp` reference; architect role |
| `ERL-CONC-05`..`-08` | Mailbox overload, call timeouts, ordering, bare spawn | missing | P1 | skill: concurrency reference; reviewer |
| `ERL-REPO-01`..`-11` | No repo, lockfile, toolchain, CI rules in loops | missing | P1 | spec: new REPO section; skill: repo-setup |
| `ERL-IDIOM-01`..`-13` | Let-it-crash, specs, strings as binaries, atoms | go-only | P1 | skill: language idioms; hook: post-edit `rebar3 fmt` |
| `ERL-TEST-08` | PropEr/statem not in `skills/testing` | go-only | P1 | skill: testing |
| `ERL-OBS-01`..`-05` | Logger, telemetry, OpenTelemetry guidance | missing | P2 | skill: observability |
| `ERL-DATA-01`..`-05` | No Erlang ORM, migrations convention | missing | P2 | skill: persistence reference |
| `ERL-API-01`..`-06`, `ERL-FW-01`..`-04` | Cowboy/Nova conventions | missing | P2 | skill: framework reference |
| `ERL-PERF-01`..`-05` | Profiling and efficiency guidance | go-only | P2 | skill: performance |
| `ERL-DOC-01`, `-04` | `-doc`/ExDoc conventions | go-only | P2 | skill: documentation |
| `ERL-OPS-01`..`-04` | Releases, sys.config.src, hot upgrades | missing | P2 | skill: packaging |

## Open questions

1. **Concuerror status.** The README badge says OTP 20.3 to 23.3 and the last Hex release is 0.21.0 (2020-10-31), although the repository has commits up to 2026-06 (spelling and CI fixes). I could not confirm it runs on OTP 27 to 29. Decision for the maintainers: treat it as advisory and optional, or drop it from the pack.
2. **covertool output** is `_build/test/covertool/<app>.covertool.xml`, one file per app (verified by B2); loops must merge the files. The pack schema's `coverage.path` is modelled as a glob.
3. **`rebar3 audit`** is real (0.2.8, GitHub Advisory Database, needs network; single maintainer). Unverified: whether OSV-Scanner reads `rebar.lock` (an osv-scalibr extractor PR exists).
4. **Race-policy gate cost.** Running the full suite twice (`+S 1:1`, `+S 8:8`) doubles test time. Maintainers may prefer one run plus PropEr statem for flagged modules.
5. **`ERL_FLAGS` in rebar3.** `ERL_FLAGS`/`ERL_AFLAGS` are honoured by `erl`; rebar3 runs tests inside a BEAM it starts, so `+S` must be set on the rebar3 invocation environment. Confirmed by B2: `ERL_FLAGS` reaches the BEAM under `rebar3 eunit` and `rebar3 ct`. The `race.variants` key is a proposed schema extension (a list of env maps, one test run each) because `race.env` is a single map.
6. **rebar3 cache variables.** `REBAR_CACHE_DIR` stays shared across runs (a per-run value re-downloads packages and rebuilds the PLT). Issues 2762 and 2633 concern global plugins and templates, not `project_plugins`.
7. **Hexagonal enforcement.** No native Erlang equivalent of ArchUnit exists. `rebar3 xref` with `xref_queries`, `.app.src` `applications` checks, or a loops-native import scan (`module:function(` regex) are all workable; the regex produces false positives on atoms and maps, so a real parser (`erl_scan`/`epp_dodger`) would be better.
8. **OTP 30 deprecations.** The 29.0 notes say old-style guard type tests are scheduled for removal in OTP 30; the exact list was not verified.
9. **Dominant migration tool.** I found no Erlang-native migration standard; the rule recommends a language-neutral tool. Confirm that matches the maintainers' policy.
11. **Schema extensions.** `race.variants` (one test run per env map) and `checks[].when_file` (run the check only when that file exists, e.g. `elvis.config`) are proposed additions to the TAXONOMY §15 schema.
10. **Dialyzer PLT location.** I recall the global PLT living in `~/.cache/rebar3/rebar3_<otp>_plt`, which the `REBAR_CACHE_DIR` mapping would cover, but I did not verify it in docs.

## Verification

Verifier: `B2-beam`, 2026-11-10.

- **Verdict: `fail`, round 1.** The lint has 1 ERR, the LOOPS block has a missing value and an invalid one, one claim is factually wrong (Renovate and Dependabot), and four `loops today` values break the rules. No link fails, and the version facts all check out.
- **Completeness:** all 14 sections are present, plus Top 10, LOOPS, Gaps, Open questions and Verification. Rule counts: REPO 12, TOOL 9, ARCH 11, IDIOM 13, CONC 9, TEST 12, PERF 5, SEC 11, DATA 5, API 6, OBS 5, OPS 6, DOC 4, FW 4 (112 in total; every minimum is met). The LOOPS JSON parses. Lint (current `lint.py`): `ERR 1 malformed rule rows` (finding 1).
- **Links:** `links.sh` reports no `FAIL` and no `blocked` URLs. Hex's own GitHub link for `rebar3_audit` (`github.com/erlef/rebar3_audit`) returns 404, but the file doesn't cite it.

**Sources opened**
1. https://www.erlang.org/downloads: yes. The latest is 29.1.2. GitHub releases show `OTP-29.1.2`, `OTP-28.5.0.8` and `OTP-27.3.4.19`, all dated 2026-10-14, and `OTP-29.0` dated **2026-05-13**. The header and TOOL-01 are correct.
2. https://github.com/erlang/otp/blob/master/SECURITY.md: yes. "Erlang/OTP supports the last 3 OTP releases with security updates and patches". Note: its table still lists 26-28; "29, 28, 27" follows from the policy text.
3. https://github.com/WhatsApp/erlfmt (and `doc/RebarUsage.md`): yes. `{project_plugins, [erlfmt]}`, `{erlfmt, [write]}`, `rebar3 fmt --check`, `rebar3 fmt <file>`, `%%% % @noformat`, `erlfmt:ignore`, and "released as version 1.0".
4. https://rebar3.org/docs/configuration/configuration/: yes. `REBAR_CACHE_DIR` ("override where Rebar3 stores cache data", default `~/.cache/rebar3`), `REBAR_GLOBAL_CONFIG_DIR` (`$HOME/.config/rebar3`), `minimum_otp_vsn`, `xref_checks`, `cover_enabled`, `cover_excl_mods`. `cover_export_enabled` is not on this page.
5. https://security.erlef.org/secure_coding_and_deployment_hardening/serialisation: yes, SEC-03's quote is verbatim: "The safe option does not affect the deserialisation of functions and other unsafe terms."
6. https://github.com/inaka/rebar3_lint plus the elvis_core README: yes. `rebar3 lint` works, and `warnings_as_errors` is "`true` (default)" in elvis_core. **But** the rebar3_lint README's claim that "default rules apply if no config" is out of date for 6.0.0 (see finding 6).
7. https://www.erlang.org/blog/highlights-otp-28/: yes. Strict generators (EEP 70, `<:-`) and opt-in priority messages.
8. https://hex.pm/packages/rebar3_audit, plus the 0.2.8 tarball's README and source: yes, the package exists (0.2.8, 2026-09-06, one owner, about 18k downloads). It reads `rebar.lock` and queries the **GitHub Advisory Database** (`https://api.github.com/advisories`) with optional `GITHUB_TOKEN`. Flags: `--level critical|high|medium|low` (default `low`), `--format human|json`, `--ignore GHSA-…`. This settles Open question 3.
9. https://hex.pm/packages/concuerror and the GitHub API: yes. 0.21.0 was the last Hex release (2020-10-31), and the README badge says "erlang 20.3 to 23.3". **But** the repo has commits as recent as 2026-06-24 (spelling and CI fixes), not "to 2023" as Open question 1 says.
10. Hex API, `cowboy`: yes. 2.21.0, 2026-11-09, 151,898,605 downloads. `nova`: 0.18.1, 2026-10-15. `erlfmt` 1.8.0, `covertool` 2.0.7.
11. GitHub releases, `erlang/rebar3`: yes. **3.28.0** was released 2026-10-21. (The `erlang:latest` image still ships 3.27.0.)
12. CVE-2026-21619 (MITRE record): yes. "Unsafe Deserialization of Erlang Terms in hex_core" lists rebar3 3.9.1 to <3.27.0 as affected, so TOOL-08's "pin >= 3.27.0" is right.
13. Renovate manager docs: **no** for REPO-07 (finding 4).

**Tooling truth.** All of these were confirmed by the spot run unless noted.
- `rebar3 compile`, `rebar3 xref`, `rebar3 dialyzer`: documented on rebar3.org. Not run, apart from compile.
- `rebar3 fmt --check`: exits 1 on unformatted code and 0 when clean. Note: the `.app.src` that a fresh `rebar3 new app` generates fails `fmt --check`.
- `rebar3 fmt -w {file}`: exit 0.
- `rebar3 eunit --cover` and `rebar3 ct --cover`: exit 0, and they write `_build/test/cover/{eunit,ct}.coverdata`.
- `rebar3 proper -n 100 --cover`: exit 0. `-n/--numtests` and `-c/--cover` are in `rebar3 help proper` (rebar3_proper 0.12.1).
- `rebar3 cover --verbose`: exit 0. It writes `_build/test/cover/index.html` and reads both coverdata files.
- `rebar3 cover --min_coverage 70`: exit 1 below the threshold. (`-m, --min_coverage` per `rebar3 help cover`.)
- `rebar3 ct --repeat`: exists (`rebar3 help ct`).
- **`rebar3 covertool generate`: the output is `_build/test/covertool/<app>.covertool.xml`**, Cobertura XML with one file per OTP app. In the run: "Writing output report '/tmp/hello/_build/test/covertool/hello.covertool.xml'". It is line-granular.
- `rebar3 sbom`: exit 0, and writes `./bom.xml` (rebar3_sbom 0.8.0).
- `rebar3 audit`: exit 0 with an empty lock ("No dependencies found in lock file."). It needs network access to api.github.com.
- `rebar3 lint`: **exit 1 when there is no elvis config.** rebar3_lint 6.0.0 printed "key 'config' is expected to exist and be a non-empty list".
- **`ERL_FLAGS` does reach the BEAM that rebar3 starts:**
  - `ERL_FLAGS="+S 1:1" rebar3 eunit` → `erlang:system_info(schedulers)` = 1;
  - `ERL_FLAGS="+S 1:1" rebar3 ct` → 1;
  - `rebar3 ct` with no flags → 12 (`nproc` 12).

  This settles Open question 5.
- **`REBAR_CACHE_DIR` hermeticity:** with `REBAR_CACHE_DIR=/tmp/rcache`, all project plugins (erlfmt, covertool, rebar3_audit, rebar3_proper, rebar3_lint, rebar3_sbom) and Hex packages went under `/tmp/rcache/hex`, and `~/.cache/rebar3` was never created. Issues 2762 and 2633 apply to global plugins, not `project_plugins`.
- Race policy `none` with a substitute: the reasoning holds. The two-scheduler run is mechanically feasible through `ERL_FLAGS`.

**Spot run**
- Image: `erlang:latest` (OTP 29.1.2, rebar 3.27.0). The project was `rebar3 new app hello` under `/tmp` in the container, with one EUnit test, one CT suite and one PropEr property. There was no repo mount.
- Exit codes:
  - `fmt --check` → 1 on the generated code, then 0 after `rebar3 fmt`;
  - `ERL_FLAGS="+S 1:1" rebar3 eunit --cover` → 0;
  - `ERL_FLAGS="+S 1:1" rebar3 ct --cover` → 0;
  - `rebar3 ct --cover` → 0;
  - `cover --verbose` → 0;
  - `cover --min_coverage 70` → 1 (14% total, because the generated `hello_app` and `hello_sup` are untested);
  - `covertool generate` → 0;
  - `audit` → 0;
  - `proper -n 100 --cover` → 0;
  - `sbom` → 0;
  - `lint` → 1 (no config).
- Side note: on OTP 29, rebar3_proper compiles with "'catch ...' is deprecated" warnings, which supports ERL-IDIOM-02.

**Findings**
1. **Lint ERR, 1 malformed row:** `ERL-REPO-01` has an unescaped `|` in `rebar3 new app`, `lib`, `release` or `umbrella`, which splits the row into 10 cells. Fix: escape it as `\|`.
2. **LOOPS `coverage.path` is null.** Fill it with `_build/test/covertool/*.covertool.xml` (one file per app; loops must merge them), and drop the "verify" note. Evidence: spot run, `covertool generate`. Also update Open question 2 and the coverage recipe line.
3. **LOOPS `race.env` is invalid:** `{"ERL_FLAGS": "+S 1:1 and +S 8:8 (run the test suites under both)"}` is prose in a machine field, and loops would export it verbatim. Fix: set `env` to null and express the two runs as a schema extension (for example `"variants": [{"ERL_FLAGS": "+S 1:1"}, {"ERL_FLAGS": "+S 8:8"}]`), explained in Open questions. The mechanism itself is confirmed (spot run).
4. **ERL-REPO-07 is factually wrong:** "Renovate or Dependabot: both read `rebar.config`". Dependabot's Hex support covers `mix` only (GitHub's supported-ecosystems page), and Renovate has no rebar manager (`docs.renovatebot.com/modules/manager/rebar/` and `/rebar3/` return 404; `mix` returns 200). Fix: say that neither supports rebar3 natively, and suggest a Renovate regex manager or `rebar3 update`/`rebar3 upgrade` in a scheduled CI job.
5. **`loops today` values that break the rules:**
   - `ERL-DOC-03` is `covered (…) partial`, and `partial` is disallowed.
   - `ERL-SEC-09` is `covered (GO_BEST §43) go-only`, which contradicts itself (GO_BEST is go-only per the baseline).
   - `ERL-CONC-04` is `missing (go-only race-guard)`, which is ambiguous.
   - In the Gaps table, `ERL-IDIOM-01..-13` is `go-only/missing`.
   - `ERL-REPO-12` is `covered (AGENTS.md Releases)`. AGENTS.md governs the loops repo itself, and no spec, skill or agent tells target projects to use conventional commits, so it should be `missing`.

   Fix: give each row a single valid value and move the nuance to the Gap column.
6. **ERL-TOOL-05 / LOOPS `elvis` check:** with rebar3_lint 6.0.0, `rebar3 lint` exits 1 when there is no `elvis.config` and no `{elvis, …}` entry. Its README's "default rule set" note is out of date. Fix: the `install` text must also say to add an `elvis.config`.
7. **ERL-OPS-05 and Open question 6:**
   - The claimed leak (plugins still landing in `~/.cache/rebar3`, so you'd need `HOME`) does not happen for `project_plugins` (spot run).
   - A per-run `REBAR_CACHE_DIR` makes every run re-fetch every Hex package and plugin and rebuild the global Dialyzer PLT, which goes against TAXONOMY OPS-05 ("without re-downloading the world").

   Fix: keep `REBAR_CACHE_DIR` shared (Hex tarballs are checksummed against `rebar.lock`), or justify the cost, and drop the `HOME` advice for project plugins.
8. **ERL-TOOL-07 severity:** `AVOID` is used on a rule that recommends optional use of eqWAlizer and names no alternative. Fix: `SHOULD` (optional), or rephrase it as an AVOID with an alternative.
9. **Open question 1 (minor):** Concuerror's repo had commits up to 2026-06-24, not 2023. The conclusion (stale release, advisory only) stands.
10. **Open question 3 is settled:** `rebar3 audit` uses the GitHub Advisory Database, needs network access (and `GITHUB_TOKEN` for rate limits), and its hex.pm repo link is dead with a single maintainer. Record those maturity caveats in TOOL-08.

### Round 2

Verifier: `B2-beam`, 2026-11-10.

- **Verdict: `fail`, round 2.**
  - `lint.py` reports no ERR (112 rules), `links.sh` reports no FAIL, and the LOOPS JSON parses.
  - 2 of the round-1 findings are still open (2, partly, and 7), and 1 new minor finding (11) came out of the fixes.
- **Re-spot-run** (`erlang:latest`, OTP 29.1.2), on a two-app umbrella (`um` with `apps/a1` and `apps/a2`):
  - `ERL_FLAGS="+S 1:1" rebar3 eunit --cover` → 0;
  - `ERL_FLAGS="+S 8:8" rebar3 eunit --cover` → 0;
  - `rebar3 covertool generate` → 0.

  It wrote `_build/test/covertool/{a1,a2,um}.covertool.xml`, so the glob `_build/test/covertool/*.covertool.xml` matches every app and the umbrella root. Both `race.variants` entries work.
- Findings:
  1. Resolved. REPO-01 no longer contains a pipe.
  2. **Partly open.** `coverage.path` is now the glob, and Open question 2 is updated. **But** the coverage recipe line (§6, just before the rules) still ends with "Output path of covertool for rebar3 is not stated in its README, see Open questions." Fix: replace that sentence with the verified path.
  3. Resolved. `race.env` is null, and `race.variants` is a valid extension explained in Open question 5.
  4. Resolved. REPO-07 now says neither Renovate nor Dependabot supports rebar3, and offers alternatives.
  5. Resolved. DOC-03, SEC-09, CONC-04 and REPO-12 each have a single valid value, and the Gaps row is `go-only`. Minor: REPO-12's Why ("loops release tooling already reads them") is still misleading. Drop it.
  6. Resolved. TOOL-05 and the `elvis` check say an `elvis.config` is required.
  7. **Open.** The OPS-05 text and Open question 6 now say to keep `REBAR_CACHE_DIR` shared, but the LOOPS `cache_env` still maps `"REBAR_CACHE_DIR": "cache"`, which is the per-run mapping (as `GOCACHE` is). Block and text contradict each other. Fix: remove `REBAR_CACHE_DIR` from `cache_env`, or add a field that says "shared".
  8. Resolved. TOOL-07 is now `SHOULD` (optional, never a gate).
  9. Resolved. Open question 1 now says the repo has commits up to 2026-06.
  10. Resolved. TOOL-08 records the GitHub Advisory Database, the network requirement, the single maintainer and the dead repo link.
  11. **New, minor.** The `elvis` check adds a `when_file` key, a schema extension that isn't explained in Open questions (the brief requires that). Fix: add one line to Open questions.
- Non-blocking note: `cache_env.ERL_CRASH_DUMP` names a crash-dump *file* (`erl_crash.dump`), not a directory. If loops maps `"tmp"` to a directory, set the value to a file inside it (for example `tmp/erl_crash.dump`).

### Lead fix after round 2

The lead applied the verifier's remaining fixes verbatim instead of running a third loop (WORKFLOW §4 caps rounds at 2): findings 2, 7 and 11: the recipe states the covertool path, `cache_env` keeps `REBAR_CACHE_DIR` and `REBAR_GLOBAL_CONFIG_DIR` shared, and Open question 11 explains `when_file` and `race.variants`. lint.py has no ERR and the LOOPS block passes `jq`. **Final: pass (lead-applied).**
