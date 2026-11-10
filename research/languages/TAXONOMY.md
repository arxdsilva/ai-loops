# Gap-analysis taxonomy

Every language file (`<lang>.md`, built from [_TEMPLATE.md](_TEMPLATE.md)) answers **every item** below, in this order. An item that doesn't apply is marked `n/a` with a one-line reason; nothing is left out.

The item IDs (`REPO-01`, …) are stable. Language rules point back to them, and the future per-language rules (`specs/languages/<lang>/RULES.md`) will be cited as `JAVA §REPO-01`.

## Rule rows

Every rule in a language file is one table row:

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `JAVA-TEST-04` | Do or avoid, in one imperative sentence | One sentence | `MUST` / `SHOULD` / `AVOID` | `[title](url)`, plus the version or date | `covered` / `go-only` / `missing` / `n/a` | `gate` / `hook` / `arch` / `review` |

- **ID:** `<LANG>-<SECTION>-<NN>`, numbered within the section. Put the taxonomy item(s) it answers in brackets at the end of the Rule cell, for example `[TEST-06]`.
- **Sev:**
  - `MUST` means breaking it is a defect;
  - `SHOULD` is the default unless a project rule says otherwise;
  - `AVOID` is an anti-pattern, and the Rule cell names the alternative.
- **loops today:** compare against [LOOPS_BASELINE.md](LOOPS_BASELINE.md).
  - `covered`: a language-neutral loops rule already says this.
  - `go-only`: loops says it, but only for Go, in Go terms.
  - `missing`: loops says nothing about it.
  - `n/a`: this ecosystem doesn't need it.
- **Enforce:** the cheapest place loops could check the rule mechanically:
  - `gate`: a command in `loops gate`;
  - `hook`: a tool-call hook;
  - `arch`: the dependency checker;
  - `review`: only a reviewing agent can judge it.
- **Priority** for every rule that isn't `covered`, recorded in the file's "Gaps vs loops" table:
  - **P0** affects gate correctness, security or the hard test gate;
  - **P1** affects architecture or code quality;
  - **P2** is guidance only.

**Sources:**
- Prefer the official documentation and the official style guide.
- Then canonical references: PEPs, Effective Java and Effective Kotlin, the Rust API Guidelines, the C++ Core Guidelines, the .NET design guidelines, PSR/PER, the Rails guides, the Swift API Design Guidelines, the Elixir library guidelines.
- Then OWASP and each tool's own documentation.
- Blogs only as corroboration.
- Every rule cites a source, and anything that changes over time records a version or date.

## Minimum depth

The minimum number of rule rows per section:

| Sections | Minimum rules |
|---|---|
| REPO, ARCH, IDIOM, TEST | 8 each |
| TOOL, CONC, SEC, DATA, API, OPS | 5 each |
| PERF, OBS, DOC, FW | 4 each |

Plus a **Top 10 footguns** list, the **LOOPS** json block, and the "Gaps vs loops" table.

---

## 1. REPO: repository and project management

- **REPO-01 Canonical layout** for a service: single module vs. multi-module; where sources, tests, resources, configs and scripts live; the official or de-facto layout (for example Maven's standard directory layout, the Python src layout, Cargo's package layout).
- **REPO-02 Workspaces and monorepos:** the native mechanism (Maven reactor, Gradle multi-project or composite builds, uv/Poetry workspaces, pnpm/npm/yarn workspaces, .NET solutions, Cargo workspaces, Mix umbrellas, `go.work`, SwiftPM multi-target, sbt multi-project, …). When to split a repo, and polyrepo vs. monorepo tooling (Nx, Turborepo, Bazel, Pants).
- **REPO-03 Bootstrapping:** the official starter or template (Spring Initializr, `dotnet new`, `cargo new`, `uv init`, `mix phx.new`, `rails new`, `composer create-project`, `swift package init`, …), and what a fresh repo must contain on day one.
- **REPO-04 Dependency management:**
  - the manifest and its lockfile, and whether to commit the lockfile (application vs. library);
  - the version-pinning policy, private registries and mirrors, vendoring;
  - dependency-update automation (Renovate, Dependabot);
  - removing unused dependencies.
- **REPO-05 Versioning and release:** SemVer in this ecosystem, changelogs, tags, publishing to the registry (Maven Central, PyPI, npm, NuGet, Packagist, RubyGems, crates.io, Hex, …), reproducible builds, signing and provenance.
- **REPO-06 Toolchain pinning:** `.tool-versions`, `rust-toolchain.toml`, `global.json`, `.python-version`, `.nvmrc` or `engines`, the Gradle and Maven wrappers, `go` and `toolchain` lines, and so on.
- **REPO-07 Hygiene files:**
  - the `.gitignore` essentials, `.gitattributes`, `.editorconfig`;
  - the pre-commit framework or hooks;
  - the license file and the `CODEOWNERS` and PR template conventions.
- **REPO-08 CI shape:**
  - the canonical stages (restore → format check → lint → typecheck → build → test with coverage → vuln scan → package);
  - dependency and build caching, matrix builds across versions and OSes;
  - what must be checked on every PR.
- **REPO-09 Branching and commits:** any ecosystem-specific conventions, for example generated changelogs driven by conventional commits. Usually `covered`.

## 2. TOOL: toolchain

- **TOOL-01 Versions:** the current stable release, the LTS releases and their support windows, the release cadence, and the recommended version manager.
- **TOOL-02 Formatter:** the one obvious formatter, its config file, and the check-only argv.
- **TOOL-03 Linters and static analysis:** the de-facto linter set, the strictness levels, the config file, and the argv.
- **TOOL-04 Type checking:** compiler strictness flags or an external checker (mypy/pyright, `tsc --noEmit`, PHPStan/Psalm, Sorbet, Dialyzer, nullable reference types, …), and the recommended level.
- **TOOL-05 Vulnerability and supply chain:** the vulnerability scanner (OSV-Scanner, pip-audit, `npm audit`, `dotnet list package --vulnerable`, `composer audit`, bundler-audit, cargo-audit/cargo-deny, mix_audit, OWASP dependency-check, …), SBOM generation, and license policy.
- **TOOL-06 Editor and LSP baseline:** the language server, plus any settings that change behaviour (for example format-on-save conflicts).

## 3. ARCH: architecture

- **ARCH-01 Styles in this ecosystem.** For each style below, say how it is expressed, which frameworks favour or fight it, and when to choose it:
  - hexagonal, clean or onion;
  - layered and MVC;
  - DDD tactical patterns: entities, value objects, aggregates, repositories, domain events;
  - CQRS and event sourcing;
  - event-driven;
  - modular monolith vs. microservices;
  - serverless;
  - the actor model.
- **ARCH-02 Mapping to hexagonal.** How the loops layers (domain, application with ports, inbound and outbound adapters, composition root) map onto idiomatic packages or modules in this language and its main framework. Give a concrete directory tree.
- **ARCH-03 Module system semantics:**
  - visibility and access modifiers, and whether cycles are allowed;
  - how a module or package name maps to a directory (needed for the loops arch checker);
  - how imports are written (the exact statement forms).
- **ARCH-04 Enforcing dependency direction:** the native or de-facto tool (ArchUnit, Konsist, import-linter, dependency-cruiser, deptrac, packwerk, NetArchTest/ArchUnitNET, the `boundary` library, Cargo crate boundaries, Go `internal/`, …), and what a rule looks like in it.
- **ARCH-05 Dependency injection:** framework containers vs. manual wiring, where the composition root lives, and the anti-patterns (service locator, field injection).
- **ARCH-06 Purity of the domain:** how to keep the domain free of IO, clock, randomness and framework annotations in this language, and which imports or calls betray impurity.
- **ARCH-07 Interfaces and ports:** idiomatic abstraction (interfaces, traits, protocols, behaviours, duck typing, structural types), where the consumer defines them, and keeping them small.
- **ARCH-08 SOLID in this language:** what each principle looks like in practice. Size heuristics: file, function and type size, parameter counts.

## 4. IDIOM: what to do and what to avoid

- **IDIOM-01 Error handling:** exceptions vs. result types, checked vs. unchecked, wrapping and context, panics and fatal errors, error hierarchies, and never swallowing errors.
- **IDIOM-02 Null and absence:** null-safety features, Option or Maybe, and the pitfalls of each.
- **IDIOM-03 Immutability and value types:** records, data classes, `readonly`, frozen objects, and value semantics.
- **IDIOM-04 Type system:** generics, sum types and enums, pattern matching, type inference limits, and type-level footguns.
- **IDIOM-05 Resources:** RAII, `using` and `await using`, try-with-resources, context managers, `defer`, `ensure` and `after`; leaks of connections, files and goroutine-like tasks.
- **IDIOM-06 Collections, strings, equality:** equality and hashing contracts, string building, encoding, and time zones and date handling.
- **IDIOM-07 Naming and API design:** the official style guide, public API surface rules, and documentation comments.
- **IDIOM-08 Metaprogramming:** reflection, macros, decorators, annotations and monkey-patching. When they are acceptable.
- **IDIOM-09 Footguns:** the language's best-known traps, each with the bad pattern and the fix. They feed the "Top 10 footguns" list.

## 5. CONC: concurrency and the `-race` equivalent

- **CONC-01 Model:** OS threads, virtual threads, async/await, coroutines, actors, an event loop, processes; and the GIL or free-threading.
- **CONC-02 Shared state:** the idiomatic synchronisation primitives, immutability and message passing, and how to avoid data races by design.
- **CONC-03 Race detection.** What exists in this ecosystem, for example:
  - TSan;
  - Go `-race`;
  - Rust's compile-time `Send`/`Sync` plus loom and miri;
  - Swift 6 strict concurrency;
  - jcstress and Lincheck;
  - pytest-run-parallel and free-threaded CPython;
  - process isolation on the BEAM.
- **CONC-04 Loops race policy (required).** The exact way to "run tests with race" here. It is one of:
  - `flag`: an enforceable flag or env var on the normal test command;
  - `build`: a separate sanitizer build or test mode;
  - `none`: give the reason, plus the substitute check that loops should enforce instead, such as parallel execution, randomised order, or a stress or repeat count.
- **CONC-05 Async pitfalls:** blocking inside async code, cancellation, timeouts, backpressure, and unhandled rejections or exceptions in tasks.
- **CONC-06 Background work:** thread pools and executors, job frameworks, graceful shutdown of workers.

## 6. TEST: testing

- **TEST-01 Frameworks:** the unit test framework, and the integration and end-to-end options.
- **TEST-02 Layout and recognition:** where tests live, naming conventions, and **the globs that identify test files**.
- **TEST-03 Style:** table-driven or parametrised tests, fixtures, the arrange-act-assert style, and the conventions for test names.
- **TEST-04 Test doubles:** mocks, fakes and stubs; the idiomatic library; and when *not* to mock.
- **TEST-05 Integration with real dependencies:** Testcontainers or the equivalent, in-memory substitutes, and database isolation per test.
- **TEST-06 Coverage:** the tool, **the output format** (lcov, Cobertura, JaCoCo, Clover, Go profile, …), **the exact argv**, line vs. branch coverage, and how to exclude generated code.
- **TEST-07 Coverage of new code:** how to measure coverage of changed lines against a git base at ≥70%, which native or third-party tools do it (diff-cover, and so on), and what loops has to compute itself.
- **TEST-08 Property, fuzz and mutation testing:** the libraries (Hypothesis, jqwik, fast-check, proptest, StreamData, …) and when each is worth it.
- **TEST-09 Flakiness:** isolation, randomised order, controlled clock and randomness, parallel test execution, and timeouts.
- **TEST-10 TDD loop speed:** watch mode, running a single test, and incremental test selection.

## 7. PERF: performance and profiling

- **PERF-01 Benchmarks:** the harness (JMH, BenchmarkDotNet, criterion, pytest-benchmark, Benchee, vitest bench, …) and how to compare results statistically.
- **PERF-02 Profiling:** CPU, memory and allocation profilers; continuous profiling.
- **PERF-03 Memory and runtime tuning:** GC settings, pooling, and startup (JIT warm-up, AOT).
- **PERF-04 Hot-path mistakes:** the common performance anti-patterns in this language.

## 8. SEC: security

- **SEC-01 Injection:** SQL, command, template and LDAP injection, plus the safe APIs.
- **SEC-02 Deserialization and parsing:** unsafe deserializers (pickle, Java serialization, YAML loaders, `unserialize`, Marshal, …), and XXE.
- **SEC-03 Web risks:** SSRF, path traversal, open redirects, CSRF and CORS defaults in the main frameworks.
- **SEC-04 Secrets and crypto:** secret handling and config, which crypto APIs to use and which to avoid, password hashing, and randomness for security.
- **SEC-05 Input validation:** the idiomatic validation libraries, and where validation belongs (the adapter boundary).
- **SEC-06 Supply chain:** lockfile integrity, typosquatting, install scripts (npm `postinstall`, setup.py), and provenance and signing.
- **SEC-07 Static security analysis:** the SAST tool (bandit, brakeman, sobelow, gosec, Semgrep rules, SpotBugs FindSecBugs, …).

## 9. DATA: persistence

- **DATA-01 Access style:** ORM vs. query builder vs. raw SQL in this ecosystem, and when to pick each.
- **DATA-02 Migrations:** the tool (Flyway, Liquibase, Alembic, EF migrations, Doctrine or Laravel migrations, Active Record migrations, Ecto, sqlx or diesel, …), forward-only vs. reversible migrations, and zero-downtime patterns.
- **DATA-03 Transactions:** boundaries (where in the hexagon), isolation levels, and the unit of work.
- **DATA-04 ORM traps:** N+1 queries, lazy loading, open-session-in-view, and identity-map surprises.
- **DATA-05 Connections:** pooling, timeouts, and async drivers.

## 10. API: APIs and integration

- **API-01 HTTP:** the dominant frameworks; routing, middleware and the request lifecycle.
- **API-02 Contracts:** OpenAPI-first vs. code-first, gRPC/protobuf, GraphQL, and API versioning.
- **API-03 Validation and errors at the edge:** problem details (RFC 9457), and status code mapping.
- **API-04 Resilience:** timeouts, retries with backoff, circuit breakers, idempotency keys, rate limiting.
- **API-05 Messaging:** Kafka, RabbitMQ, SQS and NATS clients; the outbox pattern; at-least-once handling.
- **API-06 HTTP clients:** the idiomatic client, connection reuse, and context or deadline propagation.

## 11. OBS: observability

- **OBS-01 Logging:** the structured logging library, levels, correlation IDs, and keeping personal data out of logs.
- **OBS-02 Metrics:** the library and naming conventions.
- **OBS-03 Tracing:** OpenTelemetry SDK maturity, auto-instrumentation, and context propagation.
- **OBS-04 Health:** liveness and readiness, and the endpoints the dominant frameworks provide.

## 12. OPS: configuration, packaging and deployment

- **OPS-01 Config:** 12-factor, typed config binding, secrets from the environment or a vault.
- **OPS-02 Packaging:** the artifact form (fat jar, wheel, container, single binary, release), multi-stage Dockerfiles, distroless or slim base images.
- **OPS-03 Native and AOT:** GraalVM native image, .NET NativeAOT, PyInstaller and similar, plus their trade-offs.
- **OPS-04 Lifecycle:** graceful shutdown, signal handling, and draining.
- **OPS-05 Build caches (for the loops runspace):** the cache and temp locations a build or test writes, and **which env vars can safely point them into a per-run directory** without re-downloading the world. Examples: `CARGO_TARGET_DIR`, `npm_config_cache`, `PIP_CACHE_DIR`, `PYTHONPYCACHEPREFIX`, `GRADLE_USER_HOME` (it holds the downloads too), `NUGET_PACKAGES`, `MIX_BUILD_PATH`, `GOCACHE`.
- **OPS-06 Artifacts left behind:** what a build or test leaves in the tree (`target/`, `build/`, `out/`, `bin/` + `obj/`, `__pycache__`, `.pytest_cache`, `.coverage`, `coverage/`, `node_modules/.cache`, `_build/`, `.build/`, `*.class`, …). This list drives the loops cleanup.

## 13. DOC: documentation and review

- **DOC-01 API docs:** the doc tool (Javadoc, KDoc/Dokka, Sphinx or MkDocs, TypeDoc, DocFX, rustdoc, ExDoc, YARD, DocC, Haddock, POD, …) and the comment conventions.
- **DOC-02 Repo docs:** README norms, ADRs, and changelogs in this ecosystem.
- **DOC-03 Review focus:** what experienced reviewers in this ecosystem look for first.

## 14. FW: frameworks

- **FW-01 Dominant frameworks:** the 2–4 most used backend frameworks, with a usage source.
- **FW-02 Conventions per framework:** the conventions each one imposes on REPO, ARCH, TEST and DATA, and where they conflict with hexagonal. Loops must follow the framework where the project uses it; project rules win.
- **FW-03 Framework footguns.**

## 15. LOOPS: machine-readable tooling data

One fenced `json` block per language, matching the draft pack schema below. Commands come from official docs, and each argv has a doc URL in `sources`. Use `null` for anything that doesn't exist, never an invented value.

```json
{
  "id": "go",
  "name": "Go",
  "versions": {"stable": "1.27", "lts": null, "as_of": "2026-11"},
  "detect": [{"file": "go.mod"}],
  "source_globs": ["**/*.go"],
  "test_globs": ["**/*_test.go"],
  "generated": {"globs": ["**/*.pb.go", "**/*_gen.go", "**/zz_generated*"], "header_regex": "^// Code generated .* DO NOT EDIT\\.$"},
  "checks": [
    {"name": "build", "kind": "build", "argv": ["go", "build", "./..."], "required": true, "tool": "go"},
    {"name": "gofmt", "kind": "format", "argv": ["gofmt", "-l", "."], "required": false, "tool": "gofmt", "fails_on": "output"},
    {"name": "vet", "kind": "lint", "argv": ["go", "vet", "./..."], "required": true, "tool": "go"},
    {"name": "staticcheck", "kind": "lint", "argv": ["staticcheck", "./..."], "required": false, "tool": "staticcheck", "install": "go install honnef.co/go/tools/cmd/staticcheck@latest"},
    {"name": "test", "kind": "test", "argv": ["go", "test", "-race", "-count=1", "-covermode=atomic", "-coverprofile={coverage}", "./..."], "required": true, "tool": "go"},
    {"name": "govulncheck", "kind": "vuln", "argv": ["govulncheck", "./..."], "required": false, "tool": "govulncheck", "install": "go install golang.org/x/vuln/cmd/govulncheck@latest"}
  ],
  "test_commands": [{"argv_prefix": ["go", "test"]}],
  "race": {"mode": "flag", "flags": ["-race"], "env": {"GOFLAGS": "-race"}, "reason": "Go's built-in race detector", "substitute": null},
  "coverage": {"format": "gocover", "path": "{coverage}", "granularity": "statement"},
  "post_edit": {"format_file": ["gofmt", "-w", "{file}"], "lint_dir": ["go", "vet", "./{dir}"]},
  "cache_env": {"GOCACHE": "cache", "GOTMPDIR": "tmp"},
  "artifacts": ["*.test", "*.test.exe", "*.out", "*.prof", "*.pprof", "*.cov"],
  "arch": {
    "import_regex": "^\\s*(?:import\\s+)?(?:[A-Za-z_.]+\\s+)?\"([^\"]+)\"",
    "resolver": "module-path",
    "native_tool": null,
    "forbidden_in_domain": ["os", "net/http", "database/sql"],
    "impure_calls": ["time.Now", "math/rand", "go "]
  },
  "test_jobs": "-p=2",
  "config_files": [".golangci.yml", ".golangci.yaml", ".editorconfig"],
  "sources": {"checks": "https://pkg.go.dev/cmd/go", "race": "https://go.dev/doc/articles/race_detector"}
}
```

**Field notes:**
- `{coverage}`, `{file}` and `{dir}` are placeholders that loops fills in.
- `fails_on: "output"` means the check fails when it prints anything, as `gofmt -l` does.
- `resolver` says how an import string maps to a directory: `module-path`, `package-dir` (Java/Kotlin/Scala), `dotted-module` (Python), `relative-path` (TS/JS), `namespace-psr4` (PHP), `crate-path` (Rust), `include-path` (C++), and so on. `native` means delegate to `native_tool`.
- `config_files` lists the project rule and config files that the practices-researcher reads.
- `arch` is the check `loops arch` runs, and `coverage` is the new-code check `loops gate` computes from the coverage file. Neither is an argv in `checks`.
- `race.mode` is `flag`, `build` or `none` (CONC-04). With `none`, `substitute` names the check loops enforces instead.
- `test_commands` lists the command prefixes the test guard recognises as running tests.
