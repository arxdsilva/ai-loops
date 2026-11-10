# Clojure (JVM backend): backend practices and loops gap analysis

| | |
|---|---|
| Pack id | `clojure` |
| Current stable / LTS | `1.12.6` (2026-09-02) / none (Clojure has no LTS; the 1.12.x line gets only bug-fix point releases). JVM: run on a Java LTS (21 or 25). `1.13.0-alpha11` is the dev line. |
| Dominant runtimes / frameworks | JVM (HotSpot); Ring + Reitit (or Pedestal, http-kit/Jetty adapters); Integrant / Component / Mount for lifecycle; next.jdbc + HoneySQL; Polylith for monorepos |
| As of | 2026-11 |
| Researcher | `B1-clojure` |
| Verifier | (filled in by B2) |

> **Summary:** A strong Clojure backend team uses `deps.edn` + tools.build, a REPL-connected workflow, plain data (maps) at every boundary, Ring/Reitit handlers as thin inbound adapters, Integrant/Component as the composition root, and Kaocha + clj-kondo + cljfmt in CI. There is no lockfile and no race detector, so the two key loops substitutes are a pinned-SHA/version policy and a randomised-order, repeat-run test gate. Biggest gaps in loops today: (1) the gate, coverage parser (cloverage lcov/Cobertura), arch parser (`ns` forms), and hooks are Go-only; (2) there is no race-substitute concept; (3) no language detection or repo-management rules for `deps.edn`/`project.clj`/`workspace.edn`.

Rules follow [TAXONOMY.md](TAXONOMY.md), using the row format `ID | Rule | Why | Sev | Source | loops today | Enforce`. Gaps are judged against [LOOPS_BASELINE.md](LOOPS_BASELINE.md).

Source shorthand used below (all opened or found via search on 2026-11-10 unless noted in Open questions): Clojure CLI reference https://clojure.org/reference/clojure_cli ; deps.edn reference https://clojure.org/reference/deps_edn ; tools.build guide https://clojure.org/guides/tools_build ; Clojure Style Guide https://guide.clojure.style .

## 1. REPO: repository and project management

Canonical tree for a single-service repo (deps.edn + tools.build):

```
my-service/
  deps.edn                 # :paths ["src" "resources"], aliases :test :build :dev :lint :cov
  build.clj                # tools.build tasks (clean, uber), run with: clojure -T:build uber
  tests.edn                # Kaocha config (#kaocha/v1)
  bb.edn                   # optional: Babashka task runner for dev/CI shortcuts
  .clj-kondo/config.edn    # lint config (commit config + imported lib configs; ignore .cache)
  .cljfmt.edn              # format config
  .tool-versions           # java temurin-21.x ; clojure 1.12.x ; babashka ; clj-kondo
  .gitignore  .editorconfig  .gitattributes  CODEOWNERS  LICENSE  README.md  CHANGELOG.md
  Dockerfile  .github/workflows/ci.yml  renovate.json
  src/my_service/          # namespaces my-service.* (dashes in ns, underscores in dirs)
    domain/  app/  adapters/  system.clj   core.clj (-main)
  test/my_service/         # mirrors src; files end in _test.clj
  dev/user.clj             # :dev alias only (extra-paths ["dev"]): REPL helpers, (go)/(reset)
  resources/               # config.edn, db/migrations/*.sql, logback.xml
  target/                  # build output (ignored)
```

Polylith workspace (monorepo; see REPO-02): `workspace.edn`, `deps.edn` (dev project), `components/<name>/{src,test}/<top-ns>/<name>/{interface.clj,core.clj}`, `bases/<name>/…`, `projects/<name>/deps.edn`, `development/src/dev.clj`.

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `CLJ-REPO-01` | Use the standard source layout: `src/<ns-path>/…`, `test/…`, `resources/`, one top-level namespace prefix per service (`my-service.*`); put REPL/dev-only code under a `dev/` path added only by the `:dev` alias. [REPO-01] | `deps.edn` `:paths` is the only layout contract; keeping dev code off the default path stops it shipping in the uberjar. | MUST | [deps.edn reference](https://clojure.org/reference/deps_edn) (`:paths`, `:extra-paths`); 1.12.6 | missing | review |
| `CLJ-REPO-02` | Name namespaces with dashes and files with underscores (`my-service.http.handler` is `src/my_service/http/handler.clj`); one namespace per file; the ns name must equal the file path. [REPO-01] | The loader maps `-` to `_`; a mismatch fails at require time or breaks AOT. loops arch must apply this mapping. | MUST | [Clojure Style Guide, namespaces](https://guide.clojure.style) ; [Clojure reference: namespaces](https://clojure.org/reference/namespaces) | missing | arch |
| `CLJ-REPO-03` | Choose `deps.edn` + tools.build for new services; keep Leiningen (`project.clj`) only for existing projects that depend on its plugins. Never mix both as sources of truth for dependencies in one project. [REPO-03] | `deps.edn` is the official, data-only, tool-agnostic CLI; Leiningen is mature but plugin-driven. Two manifests drift. | SHOULD | [Deps and CLI guide](https://clojure.org/guides/deps_and_cli) ; [Leiningen](https://leiningen.org/) | missing | review |
| `CLJ-REPO-04` | Bootstrap with `neil new app` / `clojure -Tnew create` (deps-new) or a team template; day-one repo must contain `deps.edn`, `build.clj`, `tests.edn`, a passing test, `.clj-kondo/`, `.cljfmt.edn`, CI workflow, README, `.gitignore`. [REPO-03] | A repo that cannot run lint+format+test+build on day one never grows those gates later. | SHOULD | [deps-new](https://github.com/seancorfield/deps-new) ; [neil](https://github.com/babashka/neil) | missing | gate |
| `CLJ-REPO-05` | Pin every dependency to an exact version: Maven coordinates as exact `{:mvn/version "x.y.z"}` (never `RELEASE`/`LATEST`/ranges), git deps as full `:git/sha` (plus `:git/tag`), tools via `-T` tool installs with pinned versions. There is no lockfile by default; the pinned `deps.edn` is the lock. [REPO-04] | `deps.edn` resolves transitive deps at runtime and resolution is not locked; exact top-level pins plus `clojure -Stree` snapshots are what teams use. | MUST | [deps.edn reference](https://clojure.org/reference/deps_edn) (`:mvn/version`, `:git/sha`) | missing | review |
| `CLJ-REPO-06` | Commit a derived dependency snapshot only if reproducibility matters: `clojure -Stree > deps-tree.txt` in CI as an artifact/diff check, or use `clojure -X:deps mvn-pom` to generate and commit `pom.xml` for scanners. [REPO-04] | Gives reviewers and scanners a resolved graph without a native lockfile. | SHOULD | [Clojure CLI reference](https://clojure.org/reference/clojure_cli) (`-Stree`, `-Spath`) | missing | gate |
| `CLJ-REPO-07` | Automate bumps with Renovate (manager matches `deps.edn` and `bb.edn`) or `antq` in CI (`clojure -Tantq outdated`); review each bump with the tests green. [REPO-04] | Pins without automation rot; Renovate's deps.edn manager updates Maven coords. | SHOULD | [Renovate deps-edn manager](https://docs.renovatebot.com/modules/manager/deps-edn/) ; [antq](https://github.com/liquidz/antq) | missing | review |
| `CLJ-REPO-08` | Remove unused dependencies: run `clojure -Tantq` plus `clj-kondo` `:unused-namespace` and `:unused-referred-var`; check that every `:deps` entry is required somewhere. [REPO-04] | Transitive JVM jars carry CVEs; unused ones add attack surface. | SHOULD | [clj-kondo linters](https://github.com/clj-kondo/clj-kondo/blob/master/doc/linters.md) | missing | gate |
| `CLJ-REPO-09` | Ship deployables with tools.build: `build.clj` has `clean`, `copy-dir`, `compile-clj`, `uber` into `target/classes` and `target/<lib>-<version>-standalone.jar`; the main ns declares `(:gen-class)` and `-main`. Run `clojure -T:build uber`. [REPO-05] | Official recipe; AOT only the main ns transitively; deterministic jar names. | MUST | [tools.build guide](https://clojure.org/guides/tools_build) | missing | gate |
| `CLJ-REPO-10` | Version with SemVer-like coordinates for libraries; derive the version from git (`b/git-count-revs`) or a tag; publish libraries to Clojars/Maven Central with `deps-deploy`, signed (`:sign-releases? true`). Services are not published; the container image tag is the release. [REPO-05] | Clojure libs keep compatibility by accretion ("never break"); signing is required for Central. | SHOULD | [deps-deploy](https://github.com/slipset/deps-deploy) ; [Spec-ulation (Rich Hickey)](https://www.youtube.com/watch?v=oyLBGkS5ICk) | missing | review |
| `CLJ-REPO-11` | Pin the toolchain: `.tool-versions` (asdf/mise) with `java`, `clojure`, `babashka`, `clj-kondo`; pin the Clojure CLI install version in CI; pin the `org.clojure/clojure` version in `deps.edn`. [REPO-06] | The CLI, JDK and Clojure all vary independently; none is recorded by `deps.edn` except the Clojure jar. | MUST | [Clojure install](https://clojure.org/guides/install_clojure) ; [mise](https://mise.jdx.dev/) | missing | review |
| `CLJ-REPO-12` | `.gitignore` must include `.cpcache/`, `target/`, `.clj-kondo/.cache`, `.lsp/.cache`, `.nrepl-port`, `.lein-*`, `classes/`, `*.class`, `.calva/output-window/`, `hs_err_pid*.log`; commit `.clj-kondo/config.edn` and `.clj-kondo/<lib>/` imported configs. [REPO-07] | Caches are machine-local; configs are project rules. The CLI docs say add `.cpcache/` to `.gitignore`. | MUST | [Clojure CLI reference](https://clojure.org/reference/clojure_cli) (cache) ; [clj-kondo config](https://github.com/clj-kondo/clj-kondo/blob/master/doc/config.md) | missing | hook |
| `CLJ-REPO-13` | CI stages in order: setup JDK+Clojure → `clojure -P -M:test` (prefetch, cached on `deps.edn` hash) → `cljfmt check` → `clj-kondo --lint` → tests with coverage → vulnerability scan → `clojure -T:build uber` → container build. Cache `~/.m2`, `~/.gitlibs`, `.cpcache`. [REPO-08] | Cheap checks first; deps resolution dominates cold CI time. | SHOULD | [Clojure CLI reference](https://clojure.org/reference/clojure_cli) (`-P`) ; [clj-kondo CLI](https://github.com/clj-kondo/clj-kondo#readme) | missing | gate |
| `CLJ-REPO-14` | Use Conventional Commits; they drive changelog and release level; keep one `CHANGELOG.md` (Keep a Changelog). [REPO-09] | Neutral; nothing Clojure-specific. | SHOULD | [Conventional Commits](https://www.conventionalcommits.org/) | missing | review |

### REPO-02 Workspaces and monorepos (rules)

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `CLJ-REPO-15` | For several deployables sharing code, use Polylith (`poly` CLI): `components/` (interface + implementation), `bases/` (entry points: HTTP/CLI), `projects/` (deployable assemblies with their own `deps.edn`), `development/` (REPL project). Run `poly check` and `poly test` in CI. [REPO-02] | Polylith is the de-facto Clojure monorepo architecture and enforces the component interface rule; it is not a build tool, so keep tools.build for artifacts. | SHOULD | [Polylith docs](https://cljdoc.org/d/polylith/clj-poly/CURRENT/doc/readme) ; [polylith repo](https://github.com/polyfy/polylith) | missing | gate |
| `CLJ-REPO-16` | Without Polylith, use `deps.edn` `:local/root` deps between sibling directories, or one repo with several `:paths`; never copy namespaces between services. [REPO-02] | `:local/root` is the native multi-module mechanism (no reactor). | SHOULD | [deps.edn reference](https://clojure.org/reference/deps_edn) (`:local/root`) | missing | review |
| `CLJ-REPO-17` | Prefer a monorepo + Polylith for one team with shared domain code; split repos only when release cadences or ownership differ. [REPO-02] | Cross-service refactoring is atomic in a monorepo; Polylith gives incremental test/build by changed bricks. | SHOULD | [Polylith docs](https://cljdoc.org/d/polylith/clj-poly/CURRENT/doc/readme) | missing | review |

## 2. TOOL: toolchain

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `CLJ-TOOL-01` | Target Clojure `1.12.x` (stable 1.12.6, 2026-09-02) on a Java LTS (21 or 25). Do not run production on `1.13.0-alpha*`. Clojure has no LTS; point releases are bug fixes only. [TOOL-01] | 1.12 brought method values, qualified method syntax, array class symbols, `Supplier`/functional-interface conversion; 1.13 alphas are unstable (new ASM, Java 17 bytecode). | MUST | [Clojure releases](https://clojure.org/releases/downloads) ; [dev changelog](https://clojure.org/releases/devchangelog) | missing | review |
| `CLJ-TOOL-02` | Format with cljfmt (`.cljfmt.edn`): check `clojure -Tcljfmt check` (or `cljfmt check`), fix `cljfmt fix`; run the same argv locally and in CI. zprint is the alternative only if the team picks it as the single formatter. [TOOL-02] | Style-guide formatting is machine-enforced; two formatters fight each other. | MUST | [cljfmt](https://github.com/weavejester/cljfmt) ; [zprint](https://github.com/kkinnear/zprint) | go-only (gofmt in gate) | gate |
| `CLJ-TOOL-03` | Lint with clj-kondo: `clj-kondo --lint src test --parallel`; exit 0 clean, 2 warnings, 3 errors (default `--fail-level warning`). Commit `.clj-kondo/config.edn`; run `--lint "$(clojure -Spath)" --dependencies --parallel --copy-configs` once to import library hooks and warm the cache. [TOOL-03] | Fast, no-compile, REPL-free; library configs (`--copy-configs`) teach it macros like `defroutes`/`deftest`. | MUST | [clj-kondo README](https://github.com/clj-kondo/clj-kondo#readme) ; [config](https://github.com/clj-kondo/clj-kondo/blob/master/doc/config.md) | go-only (vet/staticcheck) | gate |
| `CLJ-TOOL-04` | Treat these clj-kondo findings as errors in CI: `:unresolved-symbol`, `:unresolved-namespace`, `:unused-binding`, `:unused-namespace`, `:redefined-var`, `:inline-def`, `:invalid-arity`, `:type-mismatch`, `:missing-test-assertion`, `:discouraged-var` (for impure fns in the domain). Set via `:linters {:xxx {:level :error}}`. [TOOL-03] | Level lifting makes lint a hard gate without exit-code parsing. | SHOULD | [clj-kondo linters](https://github.com/clj-kondo/clj-kondo/blob/master/doc/linters.md) | missing | gate |
| `CLJ-TOOL-05` | Optionally run Eastwood (`clojure -M:eastwood`) on CI nightly or pre-release; it loads (evaluates) namespaces, so run it only in an environment where loading code is safe. [TOOL-03] | Catches arity/reflection/deprecation/wrong-tag issues clj-kondo can miss, at a cost: slow, evals code. | SHOULD | [Eastwood](https://github.com/jonase/eastwood) | missing | gate |
| `CLJ-TOOL-06` | Clojure is dynamically typed: the "type checker" is clj-kondo (arity, `:type-mismatch`), plus `clojure.spec`/malli at boundaries and `:pre`/`:post` or `assert` for invariants. Instrument with `malli.dev` / `clojure.spec.test.alpha/instrument` in dev and tests only. [TOOL-04] | No compile-time types; instrumentation is dev-time only and costs runtime. | SHOULD | [Spec guide](https://clojure.org/guides/spec) ; [malli](https://github.com/metosin/malli) | missing | review |
| `CLJ-TOOL-07` | Scan dependencies in CI with `clj-watson` (deps.edn) or nvd-clojure; generate `pom.xml` and feed it to OSV-Scanner/Trivy/Dependency-Track as a second opinion. OSV-Scanner does not natively read `deps.edn`. nvd-clojure needs an NVD API key (`NVD_API_TOKEN`) and runs from a separate helper project. [TOOL-05] | Native scanners expect lockfiles/`pom.xml`; Clojure's manifest is Clojure code. | MUST | [nvd-clojure](https://github.com/rm-hull/nvd-clojure) ; [clj-watson](https://github.com/clj-holmes/clj-watson) ; [OSV supported lockfiles](https://google.github.io/osv-scanner/supported-languages-and-lockfiles) | go-only (govulncheck) | gate |
| `CLJ-TOOL-08` | Generate an SBOM from the resolved classpath: `clojure -X:deps mvn-pom` then CycloneDX Maven plugin or `syft` against the uberjar; record licenses and fail on GPL/AGPL deps unless approved. [TOOL-05] | Supply-chain evidence and license policy; no first-party Clojure SBOM tool. | SHOULD | [Clojure CLI reference](https://clojure.org/reference/clojure_cli) ; [CycloneDX](https://cyclonedx.org/) | missing | gate |
| `CLJ-TOOL-09` | Editor: clojure-lsp (powered by clj-kondo analysis) + nREPL/Calva/CIDER/Cursive; set format-on-save to cljfmt via clojure-lsp (`:cljfmt` config) so editor and CI agree; commit `.lsp/config.edn`, ignore `.lsp/.cache`. [TOOL-06] | Two formatters (LSP built-in vs CLI) with different configs are the classic format-war. | SHOULD | [clojure-lsp](https://clojure-lsp.io/) | missing | review |

## 3. ARCH: architecture

Clojure favours data over types: the domain is plain maps/records plus pure functions; ports are protocols (or plain function maps); adapters are namespaces that reify/extend them; the system is a data map assembled by Integrant/Component. Maps `->` namespaces.

ARCH-01 summary: **hexagonal** is natural (pure core + protocols at the edge). **Layered/MVC** appears with Ring+Selmer but is discouraged for backends. **DDD tactical**: entities are maps validated by malli/spec, value objects are maps/records, aggregates are map trees with a single namespace of behaviour, repositories are protocols, domain events are plain maps (`{:event/type ...}`). **CQRS/event sourcing**: idiomatic with immutable event streams and `reduce`; frameworks: Datomic, Kafka Streams via interop. **Event-driven**: core.async inside a process, Kafka/NATS across. **Modular monolith**: the Polylith default. **Microservices**: Polylith projects deployed separately. **Serverless**: Babashka/native-image on Lambda (cold start) or JVM with SnapStart. **Actor model**: not native; agents are the closest (asynchronous, single-writer), plus core.async `go` loops as CSP processes.

ARCH-02 tree (single service, hexagonal):

```
src/my_service/
  domain/            # pure: maps, malli schemas, pure fns. Requires clojure.core/set/string/walk only
    order.clj  pricing.clj  events.clj
  app/               # use cases + port protocols (consumer-owned)
    ports.clj        # (defprotocol OrderRepo (find-order [_ id]) (save-order! [_ o]))  (defprotocol Clock (now [_]))
    place_order.clj  # (defn place-order [{:keys [repo clock]} cmd] ...) takes ports as a map arg
  adapters/
    inbound/http/    # reitit routes + handlers, ring middleware, coercion (malli), problem+json
      routes.clj  handlers.clj  middleware.clj
    inbound/cli/     inbound/consumer/    # kafka/nats consumers
    outbound/db/     # next.jdbc + HoneySQL implementing OrderRepo
    outbound/http/   # clj-http / hato clients
    outbound/clock.clj
  system.clj         # composition root: Integrant config (data) + init-key/halt-key! methods
  core.clj           # -main: (ig/init config) + shutdown hook
```

Polylith mapping: component = port interface (`interface.clj`, public) + `core` (private implementation); base = inbound adapter (HTTP); project = composition (picks components); domain components depend only on other components' interfaces.

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `CLJ-ARCH-01` | Apply the dependency rule by namespace: `domain` requires nothing outside `domain` and `clojure.*`; `app` requires `domain` (and port protocols); `adapters.*` require `app`/`domain`; only `system`/`core` require adapters. [ARCH-01, ARCH-02] | Same rule as hexagonal; Clojure only has namespaces, so directories are the layers. | MUST | [WORKFLOW/HEXAGONAL §1](../../specs/HEXAGONAL_ARCHITECTURE.md) ; [Polylith docs](https://cljdoc.org/d/polylith/clj-poly/CURRENT/doc/readme) | go-only | arch |
| `CLJ-ARCH-02` | Parse `(ns … (:require [a.b.c :as x] [d.e :refer [f]]) (:import …))` and `(require '[…])` forms to build the import graph; resolve `a.b.c` to `src/a/b/c.clj(c)` with `-` to `_`. Prefer clj-kondo analysis output (`--config '{:output {:analysis true :format :edn}}'`) over regex. [ARCH-03] | Namespaces are the only module unit; the ns form is the import list; `:as-alias` and prefix lists (`[a.b [c] [d]]`) must be handled. | MUST | [clj-kondo analysis](https://github.com/clj-kondo/clj-kondo/blob/master/analysis/README.md) ; [Clojure ns reference](https://clojure.org/reference/namespaces) | missing | arch |
| `CLJ-ARCH-03` | Enforce boundaries mechanically: (a) clj-kondo `:discouraged-namespace` + `:config-in-ns` per layer (e.g. `my-service.domain.*` forbids `clojure.java.io`, `next.jdbc`, `ring.*`); (b) Polylith `poly check` for interface-only access between components; (c) loops `arch`. [ARCH-04] | There is no ArchUnit; clj-kondo config-in-ns is the cheapest static boundary tool. | SHOULD | [clj-kondo config](https://github.com/clj-kondo/clj-kondo/blob/master/doc/config.md) ; [Polylith docs](https://cljdoc.org/d/polylith/clj-poly/CURRENT/doc/readme) | go-only | arch |
| `CLJ-ARCH-04` | Wire manually in `system.clj` with a lifecycle library: Integrant (system as data, `init-key`/`halt-key!`, `#ig/ref`) is the default recommendation; Component for teams already on it (records, explicit `Lifecycle`); Mount only for small apps (global state through var side effects). No service locator and no global atoms of dependencies. [ARCH-05] | Integrant allows anything to depend on anything and the config is edn; Component restricts deps to records/maps; Mount's implicit globals hamper tests and the REPL reset. | MUST | [Integrant](https://github.com/weavejester/integrant) ; [Component](https://github.com/stuartsierra/component) ; [Mount](https://github.com/tolitius/mount) | covered (HEXAGONAL §DI) | review |
| `CLJ-ARCH-05` | Pass ports and dependencies as an explicit first argument (a map `{:keys [repo clock]}`) to use-case functions; never `(def conn ...)` at namespace level or `(atom ...)` globals holding collaborators. [ARCH-05] | Plain functions with a deps map are trivially testable with fakes; `def`'d state breaks REPL reload and parallel tests. | MUST | [Component README](https://github.com/stuartsierra/component) ; [Style Guide](https://guide.clojure.style) | go-only | review |
| `CLJ-ARCH-06` | Keep the domain pure: no `System/currentTimeMillis`, `Instant/now`, `rand`, `random-uuid`, `slurp`, `spit`, `future`, `swap!` on shared atoms, or `java.io`/`java.sql`/`java.net`/`ring`/`next.jdbc` requires; pass time and ids in as data or via a port. [ARCH-06] | Immutable data and pure functions are Clojure's core property; one `Instant/now` in the domain makes tests time-dependent. | MUST | [Rationale](https://clojure.org/about/rationale) ; [clj-kondo `:discouraged-var`](https://github.com/clj-kondo/clj-kondo/blob/master/doc/config.md) | covered (HEXAGONAL, `skills/consistency`) | arch |
| `CLJ-ARCH-07` | Use protocols for polymorphism on the type of the first argument, defined by the consumer (`app/ports.clj`) and kept to 1-5 methods; use multimethods only for open dispatch on data (e.g. event type, `:type` keyword); use a plain map of functions for simple strategy seams. [ARCH-07] | Protocols compile to fast interface dispatch; multimethods are slower but dispatch on arbitrary values and are open. | SHOULD | [Protocols](https://clojure.org/reference/protocols) ; [Multimethods](https://clojure.org/reference/multimethods) | covered (HEXAGONAL consumer-owned ports) | review |
| `CLJ-ARCH-08` | Model data with maps and validate at boundaries with malli or clojure.spec; do not wrap every entity in `defrecord` unless you need protocol dispatch or performance. Keep one schema namespace per bounded context. [ARCH-01] | Maps compose and serialize for free; records add type-based dispatch and interop but lose some map ergonomics. | SHOULD | [malli](https://github.com/metosin/malli) ; [Spec guide](https://clojure.org/guides/spec) | missing | review |
| `CLJ-ARCH-09` | SOLID sizes: function under ~15 lines, namespace under ~400 lines, at most 4 positional params (use a map beyond that), no more than ~8 public vars per port protocol, no circular requires (the compiler rejects cycles). [ARCH-08] | Clojure style: small composable fns; cycles are a compile error, so cycles reveal hidden layering mistakes. | SHOULD | [Style Guide](https://guide.clojure.style) ; [Clojure ns reference](https://clojure.org/reference/namespaces) | go-only (SOLID_GO §8) | arch |
| `CLJ-ARCH-10` | Prefer a modular monolith (Polylith components in one project) first; split into projects (deployables) only on a scaling, ownership or release-cadence need. Use CQRS/event sourcing only where audit or temporal queries are a requirement. [ARCH-01] | Polylith lets the same components be assembled into one or many deployables without rewriting. | SHOULD | [Polylith docs](https://cljdoc.org/d/polylith/clj-poly/CURRENT/doc/readme) | covered (neutral hex rules) | review |

## 4. IDIOM: what to do and what to avoid

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `CLJ-IDIOM-01` | Return data for expected failures (`{:error/type :not-found ...}` or anomaly maps) and throw `ex-info` with a data map (`:type`, cause) for exceptional ones; catch at the adapter boundary and map to HTTP status; never swallow with an empty `catch` or a bare `(catch Exception _ nil)`. [IDIOM-01] | Clojure has no checked exceptions or Result type; `ex-info` carries structured data and `ex-data` reads it. | MUST | [Style Guide, exceptions](https://guide.clojure.style) ; [ex-info](https://clojuredocs.org/clojure.core/ex-info) | go-only (GO_BEST §8-9) | review |
| `CLJ-IDIOM-02` | Wrap causes: `(throw (ex-info "msg" {:order-id id} cause))`; log once at the outermost boundary; use `try`/`catch` with the most specific class; use `finally` only for cleanup. [IDIOM-01] | Preserves the stack and data without double logging. | MUST | [ex-data](https://clojuredocs.org/clojure.core/ex-data) ; [Style Guide](https://guide.clojure.style) | go-only | review |
| `CLJ-IDIOM-03` | `nil` is falsey, `()`/`[]`/`{}`/`""` are truthy, and `(seq coll)` is the emptiness idiom (not `(not (empty? ...))`); use `some?`/`nil?` when `false` is a legal value; use `(some-> ...)`/`(when-let ...)`/`(if-some ...)` to thread nil safely. [IDIOM-02] | `nil` punning is idiomatic but `false` vs `nil` bugs are the top trap (`(or false 1)`, `(get {:a false} :a :default)`). | MUST | [Style Guide](https://guide.clojure.style) ; [Clojure ref: special forms](https://clojure.org/reference/special_forms) | missing | review |
| `CLJ-IDIOM-04` | Use immutable persistent collections everywhere; use `assoc`/`update`/`assoc-in`/`update-in`; use transients (`transient`, `persistent!`) only locally inside a function for measured hot loops; never mutate Java collections that cross a function boundary. [IDIOM-03] | Persistent structures give structural sharing; transients are single-thread, local-only. | MUST | [Transients](https://clojure.org/reference/transients) ; [Data structures](https://clojure.org/reference/data_structures) | missing | review |
| `CLJ-IDIOM-05` | Use `defrecord` for domain entities only when protocol dispatch is needed; otherwise maps with namespaced keywords (`:order/id`); use `clojure.spec`/malli to describe shapes; prefer qualified keywords for inter-ns data. [IDIOM-03] | Qualified keys avoid collisions when maps merge across contexts. | SHOULD | [Spec guide](https://clojure.org/guides/spec) ; [Style Guide](https://guide.clojure.style) | missing | review |
| `CLJ-IDIOM-06` | Prefer spec **or** malli, not both: malli (schemas as data, coercion/transformers, Reitit integration, OpenAPI generation) for new HTTP services; clojure.spec when targeting library-level contracts without extra deps. Do not call spec `s/valid?` on hot paths in production. [IDIOM-04] | Both validate data; malli is data-first and ties to Reitit/coercion; spec macros need compile-time registration and have no built-in coercion. Unsettled; see Open questions. | SHOULD | [malli](https://github.com/metosin/malli) ; [Spec guide](https://clojure.org/guides/spec) | missing | review |
| `CLJ-IDIOM-07` | Release resources with `with-open` (any `java.io.Closeable`), or a lifecycle `halt-key!`; never leave `ResultSet`, `Connection`, streams or `ExecutorService` unclosed; realize lazy seqs inside the `with-open` scope (`doall`, `into`, `reduce`). [IDIOM-05] | Lazy sequences escaping `with-open` read a closed resource. | MUST | [Clojure ref: java interop](https://clojure.org/reference/java_interop) ; [next.jdbc getting started](https://github.com/seancorfield/next-jdbc/blob/develop/doc/getting-started.md) | go-only (defer) | review |
| `CLJ-IDIOM-08` | Never let laziness escape resource, dynamic-binding or transaction scope; avoid `lazy-seq` over I/O; use `reduce`/`transduce`/`run!`/`doseq` for side effects, never `map`/`for` for side effects. [IDIOM-05] | `(map println xs)` does nothing until realised and may run twice or in chunks of 32. | MUST | [Laziness](https://clojure.org/reference/lazy) ; [Style Guide](https://guide.clojure.style) | missing | review |
| `CLJ-IDIOM-09` | Equality is value-based (`=`); `==` is numeric only; `identical?` only for reference identity; mind that `(= 1 1N)` and `(= 1 1.0)` differ across number categories; keys in maps are compared with `=`/`hash` consistency; never use mutable objects as keys. [IDIOM-06] | Mixed-numeric keys silently miss in maps. | MUST | [Equality guide](https://clojure.org/guides/equality) | missing | review |
| `CLJ-IDIOM-10` | Use `java.time` through `clojure.java-time` or direct interop; always carry an explicit zone (`ZoneOffset/UTC`, `Instant`); never use `java.util.Date`/`SimpleDateFormat`; use `BigDecimal`/`bigdec` for money; arithmetic with `+`/`*` throws on long overflow, `+'` auto-promotes. [IDIOM-06] | Long overflow raises `ArithmeticException`; Doubles lose cents. | MUST | [Clojure ref: numbers](https://clojure.org/reference/data_structures#Numbers) ; [java.time](https://docs.oracle.com/en/java/javase/21/docs/api/java.base/java/time/package-summary.html) | go-only | review |
| `CLJ-IDIOM-11` | Naming: `kebab-case` for vars, namespaces and keywords; predicates end in `?`; unsafe/side-effecting end in `!`; conversions `x->y`; private helpers `defn-`; docstrings on public vars, placed before the arg vector. [IDIOM-07] | Community style, checked by clj-kondo/cljfmt where possible. | SHOULD | [Clojure Style Guide](https://guide.clojure.style) | go-only (GO_BEST §26) | review |
| `CLJ-IDIOM-12` | Use macros only when a function cannot do the job (control-flow/binding forms, DSLs); write the function first and a thin macro over it; never use `eval`, `read-string` on untrusted input, or `load-string` in production paths. [IDIOM-08] | Macros are not first-class and defeat tooling; `eval`/`read-string` are code-execution vectors. | MUST | [Macros](https://clojure.org/reference/macros) ; [clojure.edn](https://clojure.github.io/clojure/clojure.edn-api.html) | missing | review |
| `CLJ-IDIOM-13` | Avoid reflection: put `(set! *warn-on-reflection* true)` at the top of every namespace (or in a CI task) and eliminate every warning with a type hint (`^String s`, return hints before the arg vector). Add hints only where a warning appears. [IDIOM-08] | Reflective interop is 10-100x slower and fails at runtime if hints are wrong; the default is `false`. | SHOULD | [Java interop reference](https://clojure.org/reference/java_interop) (type hints) | missing | gate |
| `CLJ-IDIOM-14` | Prefer function composition and `->`, `->>`, `as->`, `cond->`; avoid deep nesting; use destructuring (`{:keys [a b] :or {...}}`) in params; avoid `declare` forward references unless needed; no `def` inside functions (`:inline-def`). [IDIOM-09] | Idiomatic readability; `def` inside fns mutates the namespace at runtime. | SHOULD | [Style Guide](https://guide.clojure.style) | missing | gate |

## 5. CONC: concurrency and the `-race` equivalent

Model: JVM OS threads (virtual threads on Java 21+, usable through `Thread/ofVirtual` and executors; Clojure's `future`/`agent` pools are platform threads); immutability and managed references; core.async CSP (`go` blocks run on a fixed dispatch pool, the `thread` macro and `<!!` for blocking). No GIL.

**Loops race policy: `mode = none`.** Clojure/JVM has no general data-race detector; immutability makes most races impossible by construction (atoms are "always free of race conditions" for their own value). The substitute loops should enforce: (1) tests run in Kaocha's randomised order with a printed seed (`--seed`), and the gate runs the suite 3 times with different seeds (`clojure -M:test --seed N`); (2) clj-kondo `:inline-def` lint plus `*warn-on-reflection*` clean; (3) `test.check` property tests for concurrent code paths and a stress loop (run the concurrency tests N times); (4) for Java-interop shared mutable state, review-only rule: forbid raw `java.util.HashMap` shared across threads, `volatile!` across threads, and `set!` of thread-bound vars; Lincheck/jcstress are the JVM-level tools for the rare case. Not an enforceable flag.

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `CLJ-CONC-01` | Hold shared state in exactly one managed reference: `atom` for independent state, `ref` + `dosync` for coordinated multi-ref transactions, `agent` for asynchronous single-writer state; use `swap!` with a pure function; do side effects only outside transaction/`swap!` functions (they may retry). [CONC-02] | `swap!` retries in a spin loop; STM retries a transaction. A side effect inside runs multiple times. | MUST | [Atoms](https://clojure.org/reference/atoms) ; [Refs](https://clojure.org/reference/refs) ; [Agents](https://clojure.org/reference/agents) | go-only (GO_BEST §11-12) | review |
| `CLJ-CONC-02` | Prefer one `atom` holding one immutable map over several atoms; use `swap-vals!`/`compare-and-set!` when you need the old value; use `io!` in functions that must not run inside `dosync`; avoid `ref` unless you truly need cross-ref consistency. [CONC-02] | Fewer references means fewer ordering hazards; STM is rarely needed. | SHOULD | [Atoms](https://clojure.org/reference/atoms) ; [Refs](https://clojure.org/reference/refs) | missing | review |
| `CLJ-CONC-03` | Use `delay`/`promise`/`future` deliberately: `deref` with a timeout `(deref f 5000 :timeout)`; a `future` swallows exceptions until deref; never `deref` in a tight loop; call `shutdown-agents` on exit or the JVM may hang 60 s. [CONC-05] | Futures and agents use non-daemon pools; unhandled exceptions vanish silently. | MUST | [Futures/pmap](https://clojure.org/reference/refs#_future) ; [Agents](https://clojure.org/reference/agents) | missing | review |
| `CLJ-CONC-04` | In core.async: never block (`<!!`, `Thread/sleep`, JDBC, HTTP) inside a `go` block; use `thread` or `a/io-thread` for blocking work; use bounded buffers (`(chan 100)`, `sliding-buffer`, `dropping-buffer`) for backpressure; close channels you own; always include a timeout (`alts!` + `timeout`) when waiting on external input. [CONC-05] | `go` blocks share a small fixed pool (8 threads); one blocking call can starve all. Unbounded channels are not allowed (max 1024 pending puts). | MUST | [core.async](https://github.com/clojure/core.async) ; [core.async Rationale](https://clojure.org/news/2013/06/28/clojure-clore-async-channels) | missing | review |
| `CLJ-CONC-05` | Catch exceptions inside every `go` block and every `future`/agent action (`set-error-handler!`, `:error-mode :continue`); an uncaught exception in a `go` block is lost to the default uncaught-exception handler. [CONC-05] | Lost errors turn into hung pipelines. | MUST | [core.async](https://github.com/clojure/core.async) ; [Agents](https://clojure.org/reference/agents) | missing | review |
| `CLJ-CONC-06` | Run background work on explicit `ExecutorService`s created in the lifecycle (Integrant `init-key`) and shut down in `halt-key!` with `.shutdown` + `.awaitTermination`; name threads; stop consuming new work first, then drain. [CONC-06] | Graceful shutdown needs an owner; unowned pools leak across REPL resets. | MUST | [Integrant](https://github.com/weavejester/integrant) ; [Java ExecutorService](https://docs.oracle.com/en/java/javase/21/docs/api/java.base/java/util/concurrent/ExecutorService.html) | covered (GO_BEST §11; `skills/jobs-and-state-machines`) | review |
| `CLJ-CONC-07` | Avoid `def`/`alter-var-root`/`set!` for state, and avoid dynamic vars (`binding`) crossing threads: binding conveyance applies to `future`/`pmap`/`agent` sends but not to raw `Thread` or core.async `go` blocks reliably. [CONC-02] | Thread-local bindings are a hidden-parameter race source. | AVOID | [Vars](https://clojure.org/reference/vars) | missing | review |
| `CLJ-CONC-08` | Make the loops race-substitute concrete: run the test suite with Kaocha randomisation and a fixed printed seed, repeated at least 3 times in the gate, and run any test tagged `^:concurrency` or in `*concurrent*` namespaces 20 times; fail the gate on the first flake. [CONC-04] | Without a detector, repetition and shuffling are the only machine checks. | SHOULD | [Kaocha](https://github.com/lambdaisland/kaocha) ; [test.check](https://github.com/clojure/test.check) | missing | gate |

## 6. TEST: testing

**Coverage recipe:** tool = cloverage (line/form coverage; no branch metric), format = lcov (`--lcov` writes `lcov.info`) and/or Cobertura (`--cobertura`) under the default dir `target/coverage`; argv `clojure -M:cov` with alias `{:cov {:extra-paths ["test"] :extra-deps {cloverage/cloverage {:mvn/version "<pinned>"}} :main-opts ["-m" "cloverage.coverage" "-p" "src" "-s" "test" "--lcov" "--no-html"]}}` run as `clojure -M:cov -o target/coverage` (the alias has no `-o`). With Kaocha: `clojure -M:test --plugin cloverage` with `:cloverage/opts {:lcov? true :html? false :output "target/coverage"}` in `tests.edn`. New-code coverage: loops computes it itself by intersecting the git diff added lines with the lcov `DA:<line>,<hits>` records (diff-cover also accepts lcov/Cobertura XML).

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `CLJ-TEST-01` | Use `clojure.test` (`deftest`, `is`, `testing`, `are`) run by Kaocha (`clojure -M:test`); use `matcher-combinators` for structural assertions; use `lambdaisland/kaocha` over `lein test` for focus, watch, plugins and reporters. [TEST-01] | Kaocha is the de-facto runner with a config file (`tests.edn`) and plugins. | MUST | [Kaocha](https://github.com/lambdaisland/kaocha) ; [clojure.test](https://clojure.github.io/clojure/clojure.test-api.html) | go-only (`go test`) | gate |
| `CLJ-TEST-02` | Tests live under `test/` mirroring `src/`; the namespace ends in `-test` (file `*_test.clj`/`*_test.cljc`); Kaocha `:ns-patterns ["-test$"]`; unit and integration split into two Kaocha suites (`:unit`, `:integration`) selectable by `--focus`/`--skip-meta`. Test-file globs: `test/**/*_test.clj`, `test/**/*_test.cljc`. [TEST-02] | The loops test guard and coverage exclusion need the glob. | MUST | [Kaocha](https://github.com/lambdaisland/kaocha) (tests.edn) | go-only (`*_test.go`) | gate |
| `CLJ-TEST-03` | Use `are` for table-style cases and `testing` strings that read as a sentence; one logical assertion per `testing`; arrange data as plain maps built by small factory fns in a `test/<ns>/fixtures.clj`; use `use-fixtures :each` for state reset. [TEST-03] | `are` is clojure.test's table form; fixtures isolate state. | SHOULD | [clojure.test](https://clojure.github.io/clojure/clojure.test-api.html) | go-only (GO_BEST §30 table tests) | review |
| `CLJ-TEST-04` | Test with fakes (a record or `reify` implementing the port protocol, or `{:find-order (fn ...)}` maps), not mocking libraries; use `with-redefs` only in rare unit tests and never in parallel tests; use `mockery`/`clj-fakes` only for third-party seams. [TEST-04] | `with-redefs` mutates global vars (thread-unsafe); fakes keep tests deterministic and let the domain be tested pure. | SHOULD | [Clojure test docs: with-redefs](https://clojure.github.io/clojure/clojure.core-api.html#clojure.core/with-redefs) | go-only | review |
| `CLJ-TEST-05` | Integration tests use real Postgres via Testcontainers (`clj-test-containers`) or an embedded DB started in a Kaocha/Integrant fixture; isolate each test in a transaction rolled back, or a fresh schema; reset the system with Integrant `halt!`/`init`. [TEST-05] | Mocked SQL hides dialect and migration errors; a data-as-system makes per-test restarts cheap. | SHOULD | [clj-test-containers](https://github.com/javahippie/clj-test-containers) ; [Testcontainers](https://testcontainers.com/) | missing | review |
| `CLJ-TEST-06` | Measure coverage with cloverage: `clojure -M:cov` (see recipe above), output `target/coverage/lcov.info`; exclude generated namespaces with `--ns-exclude-regex`; read lcov `DA:` lines. Cloverage reports line/form coverage only. Kaocha-cloverage uses `:cloverage/opts` and the `:kaocha.plugin/cloverage` plugin. [TEST-06] | Only cloverage targets Clojure forms; JaCoCo works on bytecode but is noisy with Clojure's generated classes. | MUST | [cloverage](https://github.com/cloverage/cloverage) ; [kaocha-cloverage](https://cljdoc.org/d/lambdaisland/kaocha-cloverage/CURRENT) | go-only (go cover profile) | gate |
| `CLJ-TEST-07` | Compute new-code coverage (≥70%) in loops: parse `lcov.info` (`SF:`, `DA:line,hits`), intersect with `git diff --unified=0 <base>` added lines; or run `diff-cover target/coverage/lcov.info --compare-branch=origin/main --fail-under=70`. Cloverage does not know about the diff. [TEST-07] | Neutral diff logic exists in loops; only the lcov parser and `.clj/.cljc` candidates are missing. | MUST | [diff-cover](https://github.com/Bachmann1234/diff_cover) ; [LOOPS_BASELINE](LOOPS_BASELINE.md) | go-only | gate |
| `CLJ-TEST-08` | Add `test.check` property tests (`clojure.test.check.clojure-test/defspec`, `prop/for-all`) for parsers, pure domain rules and idempotency; use `malli.generator`/`s/gen` to derive generators from schemas; run ≥100 iterations in CI. [TEST-08] | Property tests find edge cases tables miss. | SHOULD | [test.check](https://github.com/clojure/test.check) ; [malli generators](https://github.com/metosin/malli#value-generation) | go-only (fuzz) | review |
| `CLJ-TEST-09` | Mutation testing is optional (no mainstream Clojure mutation tool was identified; Jepsen is for distributed-system fault testing); do not gate on it. [TEST-08] | No established tooling found; unsettled. | SHOULD | [Jepsen](https://github.com/jepsen-io/jepsen) | missing | review |
| `CLJ-TEST-10` | Keep tests deterministic: inject clock/rand/uuid via ports; seed `test.check` (`:seed`) when reproducing; Kaocha randomises order and prints the seed (`--seed N`, `--no-randomize` to debug); add timeouts to futures/promises in tests; never share atoms across tests. [TEST-09] | Order dependence and clock reads are the main flake sources. | MUST | [Kaocha](https://github.com/lambdaisland/kaocha) | covered (`skills/consistency`) | gate |
| `CLJ-TEST-11` | Fast TDD loop in the REPL: `(clojure.test/run-tests 'my.ns-test)`, Kaocha `--focus my.ns-test/my-test`, `--watch`, `--fail-fast`; use `clojure.tools.namespace.repl/refresh` or Integrant `reset` to reload; keep a `user.clj` with `(go)` / `(reset)`. [TEST-10] | Sub-second feedback needs a running REPL; cold JVM start is the cost to avoid. | SHOULD | [REPL guide](https://clojure.org/guides/repl/introduction) ; [Kaocha](https://github.com/lambdaisland/kaocha) | missing | review |

## 7. PERF: performance and profiling

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `CLJ-PERF-01` | Benchmark with criterium (`quick-bench`, `bench`) in the REPL or JMH via `jmh-clojure`/`criterium` in a `:bench` alias; compare runs statistically, never with `time`. [PERF-01] | `time` includes JIT warm-up and GC noise. | SHOULD | [criterium](https://github.com/hugoduncan/criterium) ; [JMH](https://github.com/openjdk/jmh) | go-only (benchstat) | review |
| `CLJ-PERF-02` | Profile with clj-async-profiler (flamegraphs, CPU/alloc) or JFR (`-XX:+FlightRecorder` / `jcmd <pid> JFR.start`), view in JMC/VisualVM; use continuous profiling (Pyroscope/async-profiler agent) in prod. [PERF-02] | JVM profilers see through Clojure fn names (`ns$fn`). | SHOULD | [clj-async-profiler](https://github.com/clojure-goes-fast/clj-async-profiler) ; [JFR](https://docs.oracle.com/en/java/javase/21/jfr/) | go-only (pprof) | review |
| `CLJ-PERF-03` | Tune the JVM, not Clojure: set `-Xmx`/`-XX:MaxRAMPercentage=75` for containers, pick G1 (default) or ZGC, use `-XX:+UseContainerSupport` (default), enable direct linking for AOT builds (`-Dclojure.compiler.direct-linking=true`) at the cost of REPL redefinition. [PERF-03] | Container-aware heap sizing and direct linking speed fn calls but break `with-redefs`. | SHOULD | [Clojure compilation docs](https://clojure.org/reference/compilation) ; [JVM options](https://docs.oracle.com/en/java/javase/21/docs/specs/man/java.html) | missing | review |
| `CLJ-PERF-04` | Avoid: reflection (type-hint), boxed math in hot loops (`^long`/`^double` hints, `unchecked-*` only after profiling), `apply str` in loops (use `StringBuilder`/`str` with `reduce`), `(count lazy-seq)`, `concat` chains, `into []` from huge lazy seqs without transducers, `keys`/`vals` recomputed per iteration, `memoize` on unbounded keys. [PERF-04] | These are the standard Clojure hot-path traps. | AVOID | [Java interop reference](https://clojure.org/reference/java_interop) ; [Transducers](https://clojure.org/reference/transducers) | missing | review |
| `CLJ-PERF-05` | Use transducers (`(into [] (comp (map f) (filter p)) xs)`) and `reduce` for pipelines on large collections instead of chained lazy `map`/`filter`; use `pmap` only for CPU-heavy pure functions with coarse chunks. [PERF-04] | No intermediate lazy seqs; `pmap` overhead dominates on small work. | SHOULD | [Transducers](https://clojure.org/reference/transducers) | missing | review |

## 8. SEC: security

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `CLJ-SEC-01` | Build SQL with HoneySQL maps and `?` params (`(sql/format {:select [:*] :from [:users] :where [:= :id id]})`) executed by next.jdbc; never concatenate user input into SQL strings; whitelist dynamic identifiers/column names; never use `:raw`/`[:raw ...]` with user input. [SEC-01] | Parameter binding is the only safe path; HoneySQL's `:raw` re-opens injection. | MUST | [OWASP SQL Injection Prevention](https://cheatsheetseries.owasp.org/cheatsheets/SQL_Injection_Prevention_Cheat_Sheet.html) ; [HoneySQL](https://github.com/seancorfield/honeysql) | go-only (GO_BEST §38) | review |
| `CLJ-SEC-02` | Shell out with `clojure.java.shell/sh` or `ProcessBuilder` with an argument vector, never through `sh -c` and a concatenated string; validate against an allow-list. [SEC-01] | Argument vectors avoid shell interpretation. | MUST | [OWASP OS Command Injection Defense](https://cheatsheetseries.owasp.org/cheatsheets/OS_Command_Injection_Defense_Cheat_Sheet.html) | go-only | review |
| `CLJ-SEC-03` | Parse untrusted data with `clojure.edn/read-string` (with `:readers` restricted) or JSON (jsonista/cheshire); NEVER `clojure.core/read-string`, `load-string` or `eval` on input, and never set `*read-eval*` true; avoid Java `ObjectInputStream`; keep YAML parsers in safe mode. [SEC-02] | `read-string` with `#=` executes code; Java serialization gadget chains are a known RCE class. | MUST | [OWASP Deserialization](https://cheatsheetseries.owasp.org/cheatsheets/Deserialization_Cheat_Sheet.html) ; [clojure.edn](https://clojure.github.io/clojure/clojure.edn-api.html) | missing | review |
| `CLJ-SEC-04` | Use `ring-defaults` (`secure-site-defaults`/`secure-api-defaults`) for security headers, CSRF (`ring.middleware.anti-forgery`), HTTPS redirect, and same-site cookies; use `ring-cors` with an explicit origin allow-list, never `*` with credentials. [SEC-03] | `site-defaults` add CSRF and cookie protection; APIs must opt in per route. | MUST | [ring-defaults](https://github.com/ring-clojure/ring-defaults) ; [OWASP CSRF](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html) | missing | review |
| `CLJ-SEC-05` | Defend SSRF and traversal: allow-list outbound hosts, resolve and block internal IPs before `hato`/`clj-http` calls; serve files with `ring.util.response/file-response` with a `:root` and reject `..`; use `ring.middleware.resource` rather than hand-built paths. [SEC-03] | Ring's file helpers confine paths when given a root. | MUST | [OWASP SSRF](https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html) ; [Ring wiki](https://github.com/ring-clojure/ring/wiki) | missing | review |
| `CLJ-SEC-06` | Read secrets from env vars or a vault (aero `#env`, Integrant config, Vault); never commit them to `config.edn` or print config maps/errors that contain them; redact with a `:secret` key convention in logging. [SEC-04] | Config maps with passwords are routinely logged by REPL helpers and `ex-data`. | MUST | [OWASP Secrets Management](https://cheatsheetseries.owasp.org/cheatsheets/Secrets_Management_Cheat_Sheet.html) ; [aero](https://github.com/juxt/aero) | missing | hook |
| `CLJ-SEC-07` | Hash passwords with `buddy-hashers` using argon2id or bcrypt; use JCA via `buddy-core` for crypto; use `java.security.SecureRandom` (never `rand`/`rand-int`) for tokens; JWT via `buddy-sign` with algorithm pinned. [SEC-04] | `rand` is not cryptographically secure. | MUST | [OWASP Password Storage](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html) ; [buddy](https://github.com/funcool/buddy) | missing | review |
| `CLJ-SEC-08` | Validate and coerce all input at the HTTP adapter with malli/spec coercion (Reitit `:coercion`), return 400/422 with problem details; do not validate deep in the domain. [SEC-05] | Validation at the boundary keeps the domain trusted. | MUST | [Reitit coercion](https://cljdoc.org/d/metosin/reitit/CURRENT/doc/coercion/coercion) ; [OWASP Input Validation](https://cheatsheetseries.owasp.org/cheatsheets/Input_Validation_Cheat_Sheet.html) | go-only | review |
| `CLJ-SEC-09` | Supply chain: pin versions/SHAs (REPO-05); prefer Clojars/Central artifacts from known groups; verify git deps by `:git/sha`, not a branch; run the dependency scan on every PR; do not set `CLOJURE_CLI_ALLOW_HTTP_REPO`. [SEC-06] | There is no lockfile hash check; an `http://` repo allows MITM. | MUST | [Clojure CLI reference](https://clojure.org/reference/clojure_cli) (`CLOJURE_CLI_ALLOW_HTTP_REPO`) | missing | gate |
| `CLJ-SEC-10` | Run SAST with `clj-kondo` security hooks plus Semgrep (Clojure supported) and, for interop-heavy code, SpotBugs + FindSecBugs on the uberjar; `clj-holmes` (rules over source) is available. [SEC-07] | No bandit/brakeman equivalent exists; Semgrep and clj-holmes fill the gap. | SHOULD | [Semgrep languages](https://semgrep.dev/docs/supported-languages) ; [clj-holmes](https://github.com/clj-holmes/clj-holmes) | missing | gate |

## 9. DATA: persistence

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `CLJ-DATA-01` | Use next.jdbc (`execute!`, `execute-one!`, `plan`) with HoneySQL for queries; no full ORM; map rows to domain maps in the outbound adapter; use the default table-qualified keys; pass `:builder-fn rs/as-unqualified-kebab-maps` only if you set it once through `jdbc/with-options`. [DATA-01] | SQL as data composes; ORMs fight immutable data. | SHOULD | [next.jdbc](https://github.com/seancorfield/next-jdbc/blob/develop/doc/getting-started.md) ; [HoneySQL](https://github.com/seancorfield/honeysql) | missing | review |
| `CLJ-DATA-02` | Use Migratus or Ragtime with versioned `.sql` files (`resources/migrations/20261110-add-orders.up.sql`) run at startup by a separate job/command, not by every replica; forward-only in production, expand-then-contract for zero-downtime. [DATA-02] | Migration tools differ in reversible support; multi-replica startup migrations race. | MUST | [Migratus](https://github.com/yogthos/migratus) ; [Ragtime](https://github.com/weavejester/ragtime) | covered (`skills/consistency` migrations; GO_BEST §44) | review |
| `CLJ-DATA-03` | Open transactions in the application layer with `jdbc/with-transaction [tx ds]` (commit on success, rollback on exception) and pass `tx` to repository functions; never start transactions in HTTP handlers or domain. [DATA-03] | Transaction boundary = use case boundary. | MUST | [next.jdbc transactions](https://github.com/seancorfield/next-jdbc/blob/develop/doc/transactions.md) | covered (HEXAGONAL persistence) | review |
| `CLJ-DATA-04` | N+1: next.jdbc has no lazy loading, but repeated queries in `map` are the Clojure N+1; batch with `WHERE id IN (...)` (HoneySQL `[:in :id ids]`) and group in memory; use `jdbc/plan` + `reduce` to stream large results inside the connection scope. [DATA-04] | `plan` is a reducible; consuming outside the connection reads a closed ResultSet. | MUST | [next.jdbc plan](https://github.com/seancorfield/next-jdbc/blob/develop/doc/getting-started.md) | missing | review |
| `CLJ-DATA-05` | Pool with HikariCP via `next.jdbc.connection/->pool` (`:username` not `:user`); set `:maximumPoolSize`, `:connectionTimeout`, `:maxLifetime`; call `get-connection` at startup to fail fast; close the pool in `halt-key!`. [DATA-05] | Pool validation is deferred; unbounded pools exhaust the DB. | MUST | [next.jdbc getting started](https://github.com/seancorfield/next-jdbc/blob/develop/doc/getting-started.md) ; [HikariCP](https://github.com/brettwooldridge/HikariCP) | missing | review |

## 10. API: APIs and integration

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `CLJ-API-01` | Build HTTP on Ring (handler = fn of request map to response map, middleware = fn wrapping a handler) with Reitit as router (data-driven routes, per-route middleware/interceptors, coercion); use Jetty or http-kit adapters; Pedestal when you want interceptors and a full service framework. [API-01] | Ring is the standard abstraction; Reitit is the dominant router. | SHOULD | [Ring wiki](https://github.com/ring-clojure/ring/wiki) ; [Reitit](https://cljdoc.org/d/metosin/reitit/CURRENT) ; [Pedestal](https://pedestal.io/) | missing | review |
| `CLJ-API-02` | Describe the API as data in Reitit routes and generate OpenAPI via `reitit-swagger`; for contract-first teams validate responses against the OpenAPI file in tests; use malli for request/response schemas. [API-02] | One source of truth for validation and docs. | SHOULD | [Reitit swagger](https://cljdoc.org/d/metosin/reitit/CURRENT/doc/ring/swagger) | missing | review |
| `CLJ-API-03` | Return RFC 9457 problem details (`application/problem+json`) from an exception-handling middleware (`reitit.ring.middleware.exception`) mapping `ex-data` `:type` to status codes; never leak stack traces. [API-03] | Uniform errors; the middleware turns `ex-info` into responses. | MUST | [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) ; [Reitit exception middleware](https://cljdoc.org/d/metosin/reitit/CURRENT/doc/ring/exception_handling_with_ring) | covered (error taxonomy HEXAGONAL) | review |
| `CLJ-API-04` | Set timeouts on every outbound call (connect + read), retry only idempotent requests with exponential backoff and jitter (`diehard`/`again`), add circuit breakers (`diehard`), rate-limit at the edge, and require `Idempotency-Key` for POST. [API-04] | Clients without timeouts hang threads. | MUST | [diehard](https://github.com/sunng87/diehard) | covered (`skills/consistency`, jobs) | review |
| `CLJ-API-05` | Use `hato` (java.net.http) or `clj-http` with a shared client, explicit timeouts, and connection reuse; do not create a client per request; propagate deadlines and correlation IDs as headers. [API-06] | A client per call defeats keep-alive. | SHOULD | [hato](https://github.com/gnarroway/hato) ; [clj-http](https://github.com/dakrone/clj-http) | go-only (GO_BEST §10) | review |
| `CLJ-API-06` | Messaging: use the Java clients via interop wrappers (`jackdaw` for Kafka, `langohr`, `amazonica`/Cognitect aws-api for SQS, `nats.clj`); implement the outbox pattern in the same DB transaction; make consumers idempotent; commit offsets after processing. [API-05] | At-least-once delivery means duplicates. | MUST | [jackdaw](https://github.com/FundingCircle/jackdaw) ; [aws-api](https://github.com/cognitect-labs/aws-api) | covered (`skills/consistency`, jobs) | review |

## 11. OBS: observability

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `CLJ-OBS-01` | Log through `clojure.tools.logging` backed by Logback/SLF4J with JSON encoding, or use mulog (events as data); log maps with `:correlation-id`; levels by meaning; never log request bodies, tokens or PII. [OBS-01] | Structured logs and the logging facade avoid binding to one backend. | MUST | [tools.logging](https://github.com/clojure/tools.logging) ; [mulog](https://github.com/BrunoBonacci/mulog) | go-only (GO_BEST §24) | review |
| `CLJ-OBS-02` | Export metrics with iapetos (Prometheus) or Micrometer; name `snake_case` with unit suffix, labels bounded in cardinality. [OBS-02] | High-cardinality labels break Prometheus. | SHOULD | [iapetos](https://github.com/clj-commons/iapetos) ; [Prometheus naming](https://prometheus.io/docs/practices/naming/) | missing | review |
| `CLJ-OBS-03` | Trace with the OpenTelemetry Java agent (`-javaagent:opentelemetry-javaagent.jar`, zero-code) for Ring/JDBC/Kafka, and manual spans through the Java API; propagate W3C `traceparent` across core.async/executor boundaries explicitly (context is thread-local). [OBS-03] | Agent works at JVM level; Clojure's own threading is not auto-propagated. | SHOULD | [OTel Java agent](https://opentelemetry.io/docs/zero-code/java/agent/) | missing | review |
| `CLJ-OBS-04` | Expose `/healthz` (liveness, no deps) and `/readyz` (readiness: DB reachable) as Reitit routes; readiness flips to failing on shutdown start. [OBS-04] | Orchestrators use these for routing and restarts. | MUST | [Kubernetes probes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/) | missing | review |

## 12. OPS: configuration, packaging and deployment

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `CLJ-OPS-01` | Load config with aero (`config.edn` with `#env`, `#profile`) or Integrant edn; validate the resulting map with malli at startup and fail fast; secrets only from env/vault. [OPS-01] | 12-factor config as data. | MUST | [aero](https://github.com/juxt/aero) ; [12factor](https://12factor.net/config) | go-only | review |
| `CLJ-OPS-02` | Package as an AOT uberjar (`clojure -T:build uber`) in a multi-stage Dockerfile: build stage with the Clojure CLI image, run stage `eclipse-temurin:21-jre` (or distroless java) as non-root, `ENTRYPOINT ["java","-XX:MaxRAMPercentage=75","-jar","app.jar"]`. [OPS-02] | Small runtime image; JVM container-aware flags. | SHOULD | [tools.build guide](https://clojure.org/guides/tools_build) ; [Eclipse Temurin images](https://hub.docker.com/_/eclipse-temurin) | missing | review |
| `CLJ-OPS-03` | GraalVM native-image is possible but needs reflection config, `--initialize-at-build-time` for Clojure runtime classes, and avoiding `eval`; use only for CLIs/functions where start-up time dominates (Babashka is the scripting alternative). [OPS-03] | Clojure's dynamic features fight closed-world AOT. | AVOID | [clj-easy graal-docs](https://github.com/clj-easy/graalvm-clojure) | missing | review |
| `CLJ-OPS-04` | Handle SIGTERM with `(.addShutdownHook (Runtime/getRuntime) (Thread. #(ig/halt! system)))`; on shutdown stop readiness, stop consumers, drain in-flight requests (Jetty `:stop-timeout`), close pools; call `shutdown-agents`. [OPS-04] | JVM runs shutdown hooks on SIGTERM; unowned pools keep it alive. | MUST | [Integrant](https://github.com/weavejester/integrant) | go-only | review |
| `CLJ-OPS-05` | Keep `~/.m2/repository` and `~/.gitlibs` shared across runs (content-addressed; a per-run `GITLIBS` re-clones every git dep). Point only temp at a per-run dir via `JAVA_TOOL_OPTIONS=-Djava.io.tmpdir=<dir>`. `CLJ_CACHE` only moves the user cache and has no effect when a project `deps.edn` exists (`./.cpcache` is used), so clean `./.cpcache` instead. There is no env var for `~/.m2` (use `:mvn/local-repo` in `-Sdeps`). [OPS-05] | Re-downloading the Maven world or re-cloning git deps per run is slow; `./.cpcache` is in the tree. | SHOULD | [Clojure CLI reference](https://clojure.org/reference/clojure_cli) | go-only (`GOCACHE`) | hook |
| `CLJ-OPS-06` | Clean build artifacts after a run: `target/`, `.cpcache/`, `.clj-kondo/.cache`, `.lsp/.cache`, `.nrepl-port`, `classes/`, `*.class`, `*.hprof`, `hs_err_pid*.log`, `.lein-failures`, `.lein-repl-history`. [OPS-06] | Drives the loops cleanup list. | MUST | [Clojure CLI reference](https://clojure.org/reference/clojure_cli) ; [nvd-clojure](https://github.com/rm-hull/nvd-clojure) (target/nvd) | go-only | hook |

## 13. DOC: documentation and review

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `CLJ-DOC-01` | Docstring every public var (before the arg vector), use markdown, document args/return as data shape; generate API docs with cljdoc (libraries) or Codox; check `clj-kondo` `:missing-docstring` (opt-in) on public fns. [DOC-01] | cljdoc renders docstrings and articles. | SHOULD | [cljdoc](https://cljdoc.org/) ; [Style Guide](https://guide.clojure.style) | go-only (GO_BEST §26) | gate |
| `CLJ-DOC-02` | README, ADRs under `doc/adr/`, and `CHANGELOG.md`; put runnable REPL examples in `(comment ...)` rich-comment blocks next to the code. [DOC-02] | Rich comments are the REPL-driven, living examples. | SHOULD | [Rich comments](https://betweentwoparens.com/rich-comment-blocks) ; [ADR](https://adr.github.io/) | covered (HEXAGONAL ADRs) | review |
| `CLJ-DOC-03` | Review first for: reflection warnings, `def` state, laziness leaking out of scope, side effects in `swap!`/`dosync`, `read-string`/`eval`, dependencies pinned, protocol sizes, domain purity, and the `false`/`nil` traps. [DOC-03] | The ecosystem's repeating review findings. | SHOULD | [Style Guide](https://guide.clojure.style) | go-only | review |
| `CLJ-DOC-04` | Delete leftover `(comment ...)` scratch code and `tap>`/`println` debug lines before merging; keep `(comment ...)` only for documented examples. [DOC-03] | REPL-driven development leaves debris. | SHOULD | [REPL guide](https://clojure.org/guides/repl/introduction) | missing | hook |

## 14. FW: frameworks

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `CLJ-FW-01` | Choose from the dominant stack: Ring (+ Jetty/http-kit) with Reitit (library-style, data-driven), Pedestal (interceptor-based service framework), or Compojure/Ring for older code. No authoritative usage figures were found; ranking is by ecosystem prominence (see Open questions). [FW-01] | Clojure favours libraries over frameworks. | SHOULD | [Ring](https://github.com/ring-clojure/ring) ; [http-kit](https://github.com/http-kit/http-kit) ; [Reitit](https://cljdoc.org/d/metosin/reitit/CURRENT) | missing | review |
| `CLJ-FW-02` | Follow the stack's conventions where used: Reitit data routes and coercion at the adapter; Pedestal interceptors are adapters and must not hold domain logic; Component/Integrant for the lifecycle; keep the handler a thin map-to-map function calling an app-layer function. Project rules win. [FW-02] | These conventions map directly onto inbound adapters. | SHOULD | [Pedestal](https://pedestal.io/) ; [Reitit](https://cljdoc.org/d/metosin/reitit/CURRENT) | covered (project rules win; `WORKFLOW` §1) | review |
| `CLJ-FW-03` | Do not put the Component/Integrant system into Ring request maps wholesale; inject only the needed collaborators through closures or middleware (`wrap-deps`); do not let handlers reach into `system` via a global atom/var. [FW-02] | Passing the whole system hides dependencies (service-locator smell). | AVOID | [Component README](https://github.com/stuartsierra/component) | covered (HEXAGONAL smells) | review |
| `CLJ-FW-04` | Footguns: Mount's namespace-level state disables independent test systems; `wrap-params` ordering (it must precede `wrap-keyword-params` and body parsers); `ring-json` keywordizing untrusted keys (`keyword` interning); Reitit middleware order is per-route data, not global; Pedestal interceptor `:enter`/`:leave` order is reversed on leave. [FW-03] | Documented ordering mistakes. | SHOULD | [Ring wiki](https://github.com/ring-clojure/ring/wiki) ; [Pedestal](https://pedestal.io/) | missing | review |

## Top 10 footguns

1. **Reflection.** Bad: `(defn len [s] (.length s))` with no hint. Fix: `(set! *warn-on-reflection* true)` per ns and `^String s`. ([Java interop](https://clojure.org/reference/java_interop))
2. **Laziness escaping scope.** Bad: `(with-open [r (reader f)] (line-seq r))` returned. Fix: `(doall ...)`/`(into [] ...)` or `reduce` inside the scope. ([Laziness](https://clojure.org/reference/lazy))
3. **Side effects in `swap!`/`dosync`.** Bad: `(swap! a (fn [x] (send-email!) ...))`. Fix: pure fn; do the effect after the commit. ([Atoms](https://clojure.org/reference/atoms))
4. **`map` for side effects.** Bad: `(map save! items)`. Fix: `(run! save! items)` or `doseq`. ([Style Guide](https://guide.clojure.style))
5. **Blocking inside core.async `go`.** Bad: `(go (jdbc/execute! ...))`. Fix: `(a/thread ...)` and bounded channels. ([core.async](https://github.com/clojure/core.async))
6. **`read-string`/`eval` on input.** Bad: `(read-string body)`. Fix: `clojure.edn/read-string` or JSON. ([OWASP Deserialization](https://cheatsheetseries.owasp.org/cheatsheets/Deserialization_Cheat_Sheet.html))
7. **`nil` vs `false`.** Bad: `(or (:enabled cfg) true)` (turns `false` into `true`). Fix: `(get cfg :enabled true)`. ([Style Guide](https://guide.clojure.style))
8. **Unpinned/`RELEASE` dependencies and no lockfile.** Bad: `{:mvn/version "RELEASE"}`. Fix: exact versions/SHAs and `-Stree` diffing. ([deps.edn](https://clojure.org/reference/deps_edn))
9. **Namespace-level `def` state and Mount globals.** Bad: `(def conn (connect!))`. Fix: lifecycle map via Integrant `init-key`. ([Integrant](https://github.com/weavejester/integrant))
10. **Overflow and mixed numerics.** Bad: `(+ Long/MAX_VALUE 1)` throws; `(= 1 1.0)` is false; doubles for money. Fix: `+'`, `bigdec`, normalise key types. ([Equality](https://clojure.org/guides/equality))

## 15. LOOPS: tooling data

```json
{
  "id": "clojure",
  "name": "Clojure (JVM)",
  "versions": {"stable": "1.12.6", "lts": null, "as_of": "2026-11"},
  "detect": [{"file": "deps.edn"}, {"file": "project.clj"}, {"file": "workspace.edn"}],
  "source_globs": ["**/*.clj", "**/*.cljc"],
  "test_globs": ["test/**/*_test.clj", "test/**/*_test.cljc", "**/test/**/*_test.clj", "**/test/**/*_test.cljc"],
  "generated": {"globs": ["**/target/**", "**/classes/**"], "header_regex": null},
  "checks": [
    {"name": "deps", "kind": "build", "argv": ["clojure", "-P", "-M:test"], "required": true, "tool": "clojure"},
    {"name": "cljfmt", "kind": "format", "argv": ["clojure", "-Tcljfmt", "check"], "required": false, "tool": "cljfmt", "install": "clojure -Ttools install-latest :lib io.github.weavejester/cljfmt :as cljfmt"},
    {"name": "clj-kondo", "kind": "lint", "argv": ["clj-kondo", "--lint", "src", "test", "--parallel"], "required": true, "tool": "clj-kondo"},
    {"name": "test", "kind": "test", "argv": ["clojure", "-M:cov", "-o", "target/coverage"], "required": true, "tool": "clojure"},
    {"name": "test-repeat", "kind": "test", "argv": ["clojure", "-M:test", "--randomize", "--seed", "{seed}"], "required": true, "tool": "clojure", "repeat": 3, "note": "race substitute: Kaocha randomised order, 3 runs with different seeds (tests.edn must define the :test alias suite)"},
    {"name": "nvd", "kind": "vuln", "argv": ["clojure", "-J-Dclojure.main.report=stderr", "-Tnvd", "nvd.task/check", ":classpath", "\"{classpath}\"", ":config-filename", "\"nvd-clojure.edn\""], "required": false, "tool": "nvd-clojure", "install": "clojure -Ttools install nvd-clojure/nvd-clojure '{:mvn/version \"RELEASE\"}' :as nvd", "env_required": ["NVD_API_TOKEN"]}
  ],
  "test_commands": [{"argv_prefix": ["clojure", "-M:test"]}, {"argv_prefix": ["clojure", "-M:cov"]}, {"argv_prefix": ["clojure", "-X:test"]}, {"argv_prefix": ["lein", "test"]}, {"argv_prefix": ["bin/kaocha"]}],
  "race": {"mode": "none", "flags": [], "env": {}, "reason": "The JVM and Clojure have no general data-race detector; immutable data and managed references (atom/ref/agent) remove most races by construction", "substitute": "check test-repeat: Kaocha randomised order, 3 runs with different seeds (--seed N); concurrency-tagged tests repeated 20 times; clj-kondo and reflection warnings clean"},
  "coverage": {"format": "lcov", "path": "target/coverage/lcov.info", "granularity": "form"},
  "post_edit": {"format_file": ["cljfmt", "fix", "{file}"], "lint_dir": ["clj-kondo", "--lint", "{dir}"]},
  "cache_env": {"JAVA_TOOL_OPTIONS": "-Djava.io.tmpdir={tmp}"},
  "artifacts": ["target/", ".cpcache/", ".clj-kondo/.cache/", ".lsp/.cache/", ".nrepl-port", "classes/", "*.class", "*.hprof", "hs_err_pid*.log", ".lein-failures", ".lein-repl-history"],
  "arch": {
    "import_regex": "\\[\\s*([a-z][A-Za-z0-9_.\\-]*(?:\\.[A-Za-z0-9_\\-]+)+)(?:\\s|\\])",
    "resolver": "dotted-module-underscore",
    "native_tool": "poly check",
    "forbidden_in_domain": ["clojure.java.io", "clojure.java.shell", "clojure.java.jdbc", "next.jdbc", "honey.sql", "ring.", "reitit.", "org.httpkit", "clj-http", "hato", "clojure.core.async"],
    "impure_calls": ["System/currentTimeMillis", "Instant/now", "LocalDate/now", "(rand", "random-uuid", "UUID/randomUUID", "(slurp", "(spit", "(future", "(swap!", "(reset!"]
  },
  "test_jobs": null,
  "config_files": [".clj-kondo/config.edn", ".cljfmt.edn", "cljfmt.edn", "tests.edn", ".editorconfig", ".lsp/config.edn", "deps.edn"],
  "sources": {
    "checks": "https://clojure.org/reference/clojure_cli",
    "cljfmt": "https://github.com/weavejester/cljfmt",
    "clj-kondo": "https://github.com/clj-kondo/clj-kondo#readme",
    "coverage": "https://github.com/cloverage/cloverage",
    "kaocha": "https://github.com/lambdaisland/kaocha",
    "vuln": "https://github.com/rm-hull/nvd-clojure",
    "race": "https://clojure.org/reference/atoms",
    "caches": "https://clojure.org/reference/clojure_cli",
    "arch": "https://cljdoc.org/d/polylith/clj-poly/CURRENT/doc/readme"
  }
}
```

Notes on the block: the `cov` alias is `{:cov {:extra-paths ["test"] :extra-deps {cloverage/cloverage {:mvn/version "<pin>"}} :main-opts ["-m" "cloverage.coverage" "-p" "src" "-s" "test" "--lcov" "--no-html"]}}`, and loops appends `-o target/coverage` (the dir is fixed, so `coverage.path` is a plain file path, not the `{coverage}` placeholder). `test-repeat` is a proposed `repeat` field and `{seed}` a new placeholder; `nvd` needs `NVD_API_TOKEN` and the `-Ttools install` first (`env_required` is a proposed field). `{classpath}` and `{tmp}` are new placeholders (classpath from `clojure -Spath`); `resolver` `dotted-module-underscore` is a proposed new value (dash to underscore). Leiningen projects need alternates (`lein cljfmt check`, `lein kaocha`, `lein cloverage`), not included.

## Gaps vs loops

| Rule ID | Gap | loops today | Priority | Where loops should fix it |
|---|---|---|---|---|
| `CLJ-CONC-04/08` race policy | No `none`+substitute concept in the gate; no repeat/seed runs | go-only (race-guard) | P0 | gate: add `race.mode=none` substitute runs; `WORKFLOW` §6, `ROLES.md` hard rule wording |
| `CLJ-TEST-06/07` | Gate coverage parser is Go-only; needs lcov `DA:` parser, `.clj/.cljc` candidates, and `_test.clj` exclusion | go-only | P0 | gate/coverage adapter (new lcov parser), `domain/coverage` |
| `CLJ-TOOL-02/03` | Gate runs gofmt/vet only; needs cljfmt/clj-kondo checks, exit-code mapping (kondo 2/3) | go-only | P0 | gate (`runCheck` driven by pack `checks`) |
| `CLJ-ARCH-02/03` | No `ns`-form parser, no `-` to `_` resolver, no `poly check` delegation | go-only | P1 | arch (`archcheck.Parser` port + adapter), pack `arch` |
| `CLJ-TOOL-07`, `CLJ-SEC-09` | No vuln step for `deps.edn`; OSV lacks Clojure; NVD key needed | go-only | P0 | gate vuln check (nvd/clj-watson), pack docs |
| `CLJ-OPS-05/06` | Cache env and cleanup list Go-only (`GOCACHE`); tmp dir; keep ~/.m2 and ~/.gitlibs shared; clean ./.cpcache | go-only | P1 | hook go-env generalisation, `workspace`/cleanup artifacts |
| `CLJ-REPO-01..17` | No repo-management rules at all (deps pinning, tools.build, Polylith, Renovate, CI shape, .gitignore) | missing | P1 | new `specs/languages/clojure` + `skills/repo-management`; architect role |
| `CLJ-REPO-12` | `.gitignore` essentials missing | missing | P1 | hook on edit + gate `hygiene` check |
| `CLJ-IDIOM-13` / `CLJ-PERF-04` | Reflection check absent | missing | P1 | gate (`*warn-on-reflection*` task), skill `performance` |
| `CLJ-SEC-01..10` | No OWASP mapping or SAST | missing | P0 (SEC-01,03,09) P1 rest | skill `security` + reviewer checklist; gate SAST |
| `CLJ-TEST-02` | Test-file globs and `test_commands` for the test guard | go-only | P0 | pack `test_globs`, hook `race-guard`/stop-gate dirty detection (`.clj`) |
| `CLJ-IDIOM-01..14`, `CLJ-CONC-01..07` | No Clojure language rules | go-only | P1 | `specs/languages/clojure/RULES.md`, engineer role |
| `CLJ-ARCH-04..10` | Layout, DI, ports are Go-worded | go-only | P1 | HEXAGONAL layout appendix per language |
| `CLJ-DATA-*`, `CLJ-API-*`, `CLJ-OBS-*`, `CLJ-OPS-01..04`, `CLJ-DOC-*`, `CLJ-FW-*`, `CLJ-PERF-*` | Guidance missing or Go-worded | missing | P2 | skills `performance`, `documentation`, new `observability`, `deploy` |
| Language detection | `init` needs `go.mod`; must detect `deps.edn`/`project.clj`/`workspace.edn` | missing | P0 | CLI `init`, pack `detect` |

## Open questions

- **Confirmed by the B2 spot run (2026-11-10):** cloverage `--lcov` writes `<dir>/lcov.info` (default `target/coverage`); Kaocha `--seed`/`--[no-]randomize`; cljfmt `install-latest`. kaocha-cloverage has no CLI output flag, so the standalone `:cov` alias is used.
- **Project `.cpcache`:** it cannot be redirected per run (`CLJ_CACHE` is ignored with a project `deps.edn`); loops should clean it instead.
- **FW-01 usage data:** no authoritative framework usage numbers were found (State of Clojure 2025 has none); Clojars download stats could supply them.
- **Schema extensions proposed:** `repeat`, `{seed}`, `env_required`, `{classpath}`, `{tmp}`.
- **Race substitute:** is repeated randomised runs enough to satisfy the hard rule "tests always run with the race detector or the language's equivalent"? The maintainer must decide whether `mode=none` + substitute is accepted for managed-reference languages.
- **clj-watson vs nvd-clojure:** both scan `deps.edn` classpaths; flag syntax for clj-watson varies by version (v5/v6) and false positives are common; pick one. OSV-Scanner has no documented Clojure/`deps.edn` support.
- **Reflection check as gate:** no official flag turns on `*warn-on-reflection*` globally; proposed: a `:reflection` alias that binds it and `require`s all namespaces. Unverified.
- **spec vs malli:** the ecosystem is split; the rule recommends malli for HTTP services, spec for library contracts, but surveys were not consulted (State of Clojure 2025 library percentages not opened).
- **Polylith and loops ownership globs:** components share one tree; loops ownership globs (`internal/...`) need a per-pack mapping to `components/<x>/**`.

## Verification

Verifier: `B2-beam`, 2026-11-10. The file had no Verification heading, so this section was appended.

- **Verdict: `fail`, round 1.** One link fails, two rules rest on a weak source, the LOOPS block has three tooling errors, and several `loops today` values are wrong. The argv the researcher built from scratch all checked out in the spot run.
- **Completeness:** all 14 sections are present, plus Top 10, LOOPS, Gaps and Open questions. The Verification heading was missing, which was a lint ERR; this section fixes it. Rule counts: REPO 17, TOOL 9, ARCH 10, IDIOM 14, CONC 8, TEST 11, PERF 5, SEC 10, DATA 5, API 6, OBS 4, OPS 6, DOC 4, FW 4 (113 in total; every minimum is met). The LOOPS JSON parses. Apart from the missing heading, there are no malformed rows (current `lint.py`).
- **Links:** `links.sh` reported 2 FAILs.
  - `https://github.com/polylith/polylith` returns 404. It is a real failure (finding 1).
  - `https://github.com/seancorfield/next-jdbc/blob/develop/doc/transactions.md` returned 503, then 200 on a retry, so it is transient and not a finding.

**Sources opened**
1. https://clojure.org/releases/downloads: yes. Stable is 1.12.6 (Sep 2, 2026), and the dev release is 1.13.0-alpha11.
2. https://clojure.org/reference/clojure_cli: yes for `-P`, `-Stree`, `GITLIBS` (default `~/.gitlibs`), `CLOJURE_CLI_ALLOW_HTTP_REPO` (MITM warning) and ".cpcache/ in .gitignore". `CLJ_CACHE` is the *user* cache; with a writable project `deps.edn` the CLI uses `./.cpcache` (finding 4). `:mvn/local-repo` is documented in the deps.edn reference.
3. https://github.com/cloverage/cloverage: yes. `--lcov`, `--cobertura`, `-o/--output` (default `target/coverage`), `--[no-]html`, `-p`, `-s`, `-e/--ns-exclude-regex`. As the researcher said, the README has no deps.edn alias, only `clj -Sdeps … -m cloverage.coverage -p src -s test`.
4. https://github.com/rm-hull/nvd-clojure: partly. The helper-project form `clojure -J-Dclojure.main.report=stderr -M -m nvd.task.check "nvd-clojure.edn" "$(cd <proj>; clojure -Spath …)"` matches, the `NVD_API_TOKEN` key is "MANDATORY", and reports go to `target/nvd/`. The form only works from inside the helper project (finding 6).
5. https://github.com/jonase/eastwood: yes. Eastwood evaluates code and causes "any and all side effects that loading the file would cause". Usage is `clojure -M:test:eastwood`, version 1.4.3. (The researcher hadn't opened it.)
6. https://github.com/FundingCircle/jackdaw: yes. A Clojure Kafka library covering the admin, producer, consumer and Streams APIs, last pushed 2026-09-28. (The researcher hadn't opened it.)
7. https://github.com/gnarroway/hato: yes. It wraps JDK 11 `HttpClient`, recommends a reusable `build-http-client`, and has `:connect-timeout` and `:timeout`. (The researcher hadn't opened it.)
8. https://github.com/sunng87/diehard: yes. Retry, circuit breaker, rate limiter and bulkhead, built on Failsafe. (The researcher hadn't opened it.)
9. https://docs.renovatebot.com/modules/manager/deps-edn/: yes. It matches `(deps|bb)\.edn`.
10. https://clojure.org/news/2026/02/18/state-of-clojure-2025: the page exists, **but** it has no framework or library usage figures and doesn't mention mutation testing. It **does not support** FW-01 or TEST-09. Counted as 1 weak source (finding 2).
11. https://github.com/lambdaisland/kaocha: the README doesn't document the CLI flags. They are confirmed through `--help` in the spot run.
12. https://github.com/clojure/core.async (README changelog): CONC-04's go-pool claim still holds. Virtual threads for `go` (ASYNC-262) were reverted in 1.9.847-alpha3, `io-thread` uses virtual threads, and `pool-size` was replaced by `executor-factory` (1.8). Mention `io-thread` virtual threads as the preferred home for blocking work.

**Tooling truth.** All of these were confirmed by the spot run.
- `clojure -P -M:test`: exit 0.
- cljfmt install: `clojure -Ttools install-latest :lib io.github.weavejester/cljfmt :as cljfmt` installed 0.16.6, exit 0.
- `clojure -Tcljfmt check`: exit 0 when clean; exit 1 with "1 file(s) formatted incorrectly".
- `clj-kondo --lint src test --parallel`, run through the clj-kondo 2026.10.26 jar: exit 0 when clean and **exit 2** on a warning, as TOOL-03 says.
- Kaocha: `clojure -M:test --seed 42`, `--no-randomize` and `--randomize` all exit 0. `--help` lists `--[no-]randomize`, `--seed SEED`, `--focus`, `--[no-]watch` and `--[no-]fail-fast`, which settles that Open question.
- The `:cov` alias as written in the LOOPS notes, run as `clojure -M:cov -o /tmp/covout`: exit 0, "Writing LCOV report to: /tmp/covout/lcov.info". `lcov.info` uses `SF:src/hello/core.clj` (relative) and `DA:` records.
- `clojure -M:cov` with no `-o`: writes `target/coverage/lcov.info`. The lcov filename and the default directory are confirmed (cloverage 1.2.4).
- `CLJ_CACHE=/tmp/cljcache`: the directory was **never created** during `-P`, the test, cov or `-T` runs; `./.cpcache` was used instead (finding 4).
- Not run: `nvd`, because it needs an NVD API key.

**Spot run**
- Image: `clojure:temurin-21-tools-deps` (CLI 1.12.6.1673, OpenJDK 21.0.13). The project was under `/tmp/hello` in the container, with `src/hello/core.clj` and one `deftest`, and `deps.edn` aliases `:test` (Kaocha) and `:cov` (cloverage). There was no repo mount.
- Exit codes:
  - `-P -M:test` → 0;
  - `-P -M:cov` → 0;
  - cljfmt install → 0;
  - `-Tcljfmt check` → 0, then 1 on a bad file;
  - `-M:test --seed 42` → 0;
  - `--no-randomize` → 0;
  - `-M:cov -o /tmp/covout` → 0;
  - `-M:cov` → 0;
  - clj-kondo → 0, then 2 with an unused binding.

**Findings**
1. **Link FAIL, REPO-15:** `https://github.com/polylith/polylith` returns 404. The repository is now `https://github.com/polyfy/polylith` (200, pushed 2026-11-01).
2. **Weak source, FW-01 and TEST-09:** the State of Clojure 2025 page has no framework or library usage figures and no mention of mutation testing. FW-01 needs a real usage source (for example Clojars download counts for ring, reitit, pedestal and compojure). TEST-09 should cite the tools themselves (`mutant`, `jepsen`) or drop the "not mainstream" claim.
3. **LOOPS `{coverage}` placeholder misuse:** the schema defines `{coverage}` as the coverage *file* (Go: `-coverprofile={coverage}`), but the `test` check uses it as a directory (`-o {coverage}`) with `coverage.path = "{coverage}/lcov.info"`. Fix: either use a fixed path (`"path": "target/coverage/lcov.info"` with no `-o`), or add a new placeholder such as `{coverage_dir}` and explain it in Open questions.

   Related: the coverage recipe line puts `"-o" "target/coverage"` inside the alias, while the LOOPS note says the alias has no `-o`. Make them agree.
4. **LOOPS `cache_env.CLJ_CACHE` has no effect** on a project with a `deps.edn`. The CLI uses `./.cpcache`, and the spot run never created `CLJ_CACHE`. A per-run `GITLIBS` re-clones every git dependency, which goes against TAXONOMY OPS-05 ("without re-downloading the world") and the file's own "keep ~/.m2 shared" logic. Fix: drop `CLJ_CACHE` (or keep it only for `-T` tools with no project), keep `GITLIBS` shared, and rely on the cleanup list for `./.cpcache`. Update OPS-05.
5. **Race substitute not wired into `checks`:** `race.substitute` says "Kaocha randomised order … suite run 3 times with different seeds", but the only test check is `clojure -M:cov`, which runs through cloverage's own `clojure.test` runner, not Kaocha, so nothing is randomised. Fix: add an explicit check, for example `{"name": "test-repeat", "argv": ["clojure", "-M:test", "--randomize"], …}` run 3 times (or one with `--seed {seed}`), or switch coverage to kaocha-cloverage. Document it as Elixir does with `test-repeat`. Also, `race.flags` should be `[]`, not `null`, to match the schema type.
6. **LOOPS `nvd` argv is not runnable as given.** `-M -m nvd.task.check` needs nvd-clojure on the classpath, which the README gets from a separate helper project run in that directory, plus `NVD_API_TOKEN`. `install` is `null`, and there is no field for the working directory. Fix: use the README's tool form (`clojure -J-Dclojure.main.report=stderr -Tnvd nvd.task/check :classpath "{classpath}"`) with `install` set to the `-Ttools install` line, document the token requirement, or mark the check as needing setup in Open questions.
7. **`loops today` values that break the rules:**
   - `CLJ-SEC-08` (`covered (GO_BEST §43 neutral core)`), `CLJ-OPS-01` (`covered (GO_BEST §28 neutral core)`), `CLJ-OPS-04` (`covered (GO_BEST §11-12)`) and `CLJ-DOC-03` (which cites GO_BEST §51) all point to GO_BEST, which the baseline calls **go-only**. They should be `go-only`.
   - `CLJ-TEST-04` is `covered (consistency/testing; Go-worded)`, but `skills/testing` is go-only per the baseline.
   - `CLJ-ARCH-05` is `covered` with no citation.
   - `CLJ-ARCH-01` is `covered …; layout go-only`, a hedge.
   - In the Gaps table, `CLJ-SEC-01..10` and `CLJ-IDIOM-01..14` are `go-only/missing`.
   - `CLJ-REPO-14` is `covered (AGENTS.md releases; WORKFLOW §1)`. AGENTS.md governs the loops repo itself, and nothing in specs, skills or agents mentions conventional commits, so it should be `missing`.

   Fix: give each row one valid value and move the nuance to the Gap column.
8. **CLJ-REPO-06 severity:** `AVOID` is used on a rule that recommends committing a dependency snapshot, and it names no alternative. Fix: `SHOULD`.
9. **Open questions to update:** the cloverage argv and `lcov.info` filename, the Kaocha `--seed`/`--[no-]randomize` spelling, the cljfmt install line and the "Several sources … B2 should open them" item are all settled by the spot run and the sources above. Remove them, or record the confirmed values.

### Round 2

Verifier: `B2-beam`, 2026-11-10.

- **Verdict: `pass`, round 2.**
  - `lint.py` reports no ERR (113 rules).
  - `links.sh` reports no FAIL.
  - The LOOPS JSON parses.
  - All 9 round-1 findings are resolved.
- **Re-spot-run** (`clojure:temurin-21-tools-deps`, the same hello-world):
  - `clojure -M:cov -o target/coverage` → 0, and writes `target/coverage/lcov.info` (`SF:src/hello/core.clj`), which matches `coverage.path`;
  - `clojure -M:test --randomize --seed {11,22,33}` → 0, 0, 0;
  - the same with a failing test → 1, so `test-repeat` does gate.
- The `nvd` install line matches the nvd-clojure README (`clojure -Ttools install nvd-clojure/nvd-clojure '{:mvn/version "RELEASE"}' :as nvd`). The `-Tnvd` argv quotes the EDN strings correctly.
- Findings:
  1. Resolved. REPO-15 links `github.com/polyfy/polylith`.
  2. Resolved. FW-01 drops the survey claim and says no authoritative usage figures were found. TEST-09 cites Jepsen for what it is and makes no mutation-tool claim.
  3. Resolved. `coverage.path` is the fixed `target/coverage/lcov.info`, `-o target/coverage` is explicit, and the recipe, the notes and the alias now agree (the alias has no `-o`).
  4. Resolved. `cache_env` keeps only `JAVA_TOOL_OPTIONS`; `CLJ_CACHE` and `GITLIBS` are gone; Open questions say `.cpcache` is handled by cleanup.
  5. Resolved. A `test-repeat` check (`repeat: 3`, `{seed}`) is in `checks`, `race.flags` is `[]`, and the extensions are listed in Open questions.
  6. Resolved. `nvd` uses the README's tool form, with an install line and `env_required: ["NVD_API_TOKEN"]` (explained).
  7. Resolved. SEC-08, OPS-01, OPS-04, DOC-03, TEST-04, ARCH-05 and ARCH-01 are `go-only`, REPO-14 is `missing`, and the Gaps rows have single values.
  8. Resolved. REPO-06 is now `SHOULD`.
  9. Resolved. Open questions record the confirmed values, and the "B2 should open them" item is gone.
