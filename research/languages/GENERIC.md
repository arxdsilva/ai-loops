# Language-neutral practices

As of **2026-11-10**. This file collects what the 15 language files agree on. Stage C turns it into language-neutral specs, so that each per-language pack only adds its own mechanics: a pack cites the neutral rule (`TESTING §3`) instead of restating it.

## 1. Method

- A practice counts as **universal** when at least 12 of the 15 files state it as a rule row.
- The script `scripts/universal.py` found the rows. Its input is `rules.json`, which the MATRIX script writes.
  - A candidate practice is a set of taxonomy tags (the `[TEST-06]` at the end of each Rule cell) plus a regex over the rule text.
  - A language "states" the practice when one of its rule rows carries a listed tag and its text matches the regex.
- **The ID column shows one representative row per language:** the strongest severity first, then file order. The files hold the full wording.
- **"loops today"** is the majority status of the matching rows. Where it is `neutral`, the table cites the neutral home (LOOPS_BASELINE).
- Every taxonomy item is answered by at least 13 files, because the template requires it. So the test here is agreement on the *practice*, not on the topic.

## 2. Universal practices

### REPO: repository and project management

| Practice | Rule (one line) | n | Rule IDs | loops today |
|---|---|---|---|---|
| Standard layout | Use the ecosystem's canonical source/test/resource layout; don't invent one | 15 | PY-REPO-01, TS-REPO-01, JAVA-REPO-01, CPP-REPO-01, CS-REPO-01, PHP-REPO-01, GO-REPO-02, RUBY-REPO-01, SWIFT-REPO-01, KT-REPO-01, RUST-REPO-01, SCALA-REPO-01, ELIXIR-REPO-01, ERL-REPO-01, CLJ-REPO-01 | missing (HX §4 is the Go layout) |
| One repo, one build, until a deployable splits | Default to one module/solution/workspace; split only at a separately released or deployed boundary | 14 | PY-REPO-10, JAVA-REPO-11, CPP-REPO-06, CS-REPO-02, PHP-REPO-01, GO-REPO-03, RUBY-REPO-03, SWIFT-REPO-03, KT-REPO-02, RUST-REPO-04, SCALA-REPO-03, ELIXIR-REPO-01, ERL-REPO-02, CLJ-REPO-16 | missing |
| Commit the lockfile | Applications commit the lockfile; libraries follow the ecosystem norm | 14 (Clojure pins exact versions instead, CLJ-REPO-05/06) | PY-REPO-04, TS-REPO-06, JAVA-REPO-06, CPP-REPO-09, CS-REPO-07, PHP-REPO-04, GO-REPO-07, RUBY-REPO-04, SWIFT-REPO-05, KT-REPO-06, RUST-REPO-06, SCALA-REPO-06, ELIXIR-REPO-04, ERL-REPO-04 | missing (GO_BEST §49 is Go-only) |
| Frozen install in CI | CI and the gate install from the lockfile and fail if it is stale | 13 | PY-REPO-05, TS-REPO-05, JAVA-REPO-06, CS-REPO-07, PHP-REPO-04, GO-REPO-08, RUBY-REPO-05, SWIFT-REPO-05, KT-REPO-06, RUST-REPO-06, ELIXIR-REPO-05, CLJ-SEC-09, CPP-SEC-08 | missing |
| Pin the toolchain in a committed file | `.tool-versions`, `rust-toolchain.toml`, `global.json`, `.python-version`, wrappers, the `go`/`toolchain` lines | 15 | PY-REPO-09, TS-REPO-10, JAVA-REPO-03, CPP-REPO-12, CS-REPO-11, PHP-REPO-10, GO-REPO-11, RUBY-REPO-08, SWIFT-REPO-09, KT-REPO-07, RUST-REPO-11, SCALA-REPO-11, ELIXIR-REPO-08, ERL-REPO-09, CLJ-REPO-11 | missing |
| Automated dependency updates | Renovate or Dependabot (or Scala Steward, antq), grouped, never auto-merging majors | 15 (Erlang notes that neither bot supports rebar3, ERL-REPO-07) | PY-REPO-11, TS-REPO-07, JAVA-REPO-07, CPP-REPO-10, CS-REPO-09, PHP-REPO-08, GO-REPO-09, RUBY-REPO-07, SWIFT-REPO-07, KT-REPO-11, RUST-REPO-08, SCALA-REPO-07, ELIXIR-REPO-06, ERL-REPO-07, CLJ-REPO-07 | missing (GO_BEST §49 is Go-only) |
| SemVer and tag releases | SemVer, `v<x.y.z>` tags, registry publishing with provenance where the registry supports it | 15 | PY-REPO-12, TS-REPO-09, JAVA-REPO-08, CPP-REPO-11, CS-REPO-10, PHP-REPO-09, GO-REPO-10, RUBY-REPO-10, SWIFT-REPO-08, KT-REPO-08, RUST-REPO-09, SCALA-REPO-10, ELIXIR-REPO-07, ERL-REPO-08, CLJ-REPO-10 | missing |
| Ignore build outputs | `.gitignore` lists the ecosystem's outputs and caches (the same list as OPS-06) | 15 | PY-REPO-13, TS-REPO-11, JAVA-REPO-09, CPP-REPO-13, CS-REPO-04, PHP-REPO-11, GO-REPO-13, RUBY-REPO-11, SWIFT-REPO-10, KT-REPO-09, RUST-REPO-13, SCALA-REPO-12, ELIXIR-REPO-09, ERL-REPO-10, CLJ-REPO-12 | missing |
| CI stage order | restore → format check → lint → typecheck → build → test with coverage → vuln scan → package, on every PR | 15 | PY-REPO-15, TS-REPO-12, JAVA-REPO-10, CPP-REPO-14, CS-REPO-14, PHP-REPO-12, GO-REPO-14, RUBY-REPO-13, SWIFT-REPO-11, KT-REPO-10, RUST-REPO-14, SCALA-REPO-14, ELIXIR-REPO-10, ERL-REPO-11, CLJ-REPO-13 | go-only (WORKFLOW §6 lists the Go gate) |
| Conventional commits | Conventional commits drive the changelog and the release level | 14 | PY-REPO-16, TS-REPO-13, JAVA-REPO-12, CPP-REPO-15, CS-REPO-15, PHP-REPO-14, GO-REPO-15, SWIFT-REPO-12, KT-REPO-12, RUST-REPO-15, SCALA-REPO-15, ELIXIR-REPO-11, ERL-REPO-12, CLJ-REPO-14 | neutral in 8 files, missing in 6 (no spec, skill or role states it; CPP-REPO-15, RUST-REPO-15) |

### TOOL: toolchain

| Practice | Rule (one line) | n | Rule IDs | loops today |
|---|---|---|---|---|
| One formatter, check-only in the gate | One formatter with a committed config; the gate runs its check mode | 15 | PY-TOOL-03, TS-TOOL-02, JAVA-TOOL-02, CPP-TOOL-03, CS-TOOL-02, PHP-TOOL-02, GO-TOOL-02, RUBY-TOOL-02, SWIFT-TOOL-02, KT-TOOL-02, RUST-TOOL-02, SCALA-TOOL-03, ELIXIR-TOOL-03, ERL-TOOL-02, CLJ-TOOL-02 | go-only (gate `gofmt`) |
| Strictest practical type and compile checks | Compiler warnings as errors, or the external type checker at a stated level | 15 | PY-TOOL-07, TS-TOOL-06, JAVA-TOOL-03, CPP-TOOL-04, CS-TOOL-05, PHP-TOOL-04, GO-TOOL-05, RUBY-TOOL-05, SWIFT-TOOL-04, KT-TOOL-04, RUST-TOOL-06, SCALA-TOOL-05, ELIXIR-TOOL-06, ERL-TOOL-06, CLJ-TOOL-06 | missing |
| Vulnerability scan of the lockfile | Run the ecosystem's scanner (or OSV-Scanner) in CI and the gate | 15 (Swift has no tool, SWIFT-TOOL-05) | PY-TOOL-10, TS-TOOL-08, JAVA-TOOL-05, CPP-TOOL-09, CS-TOOL-06, PHP-TOOL-08, GO-TOOL-06, RUBY-TOOL-06, SWIFT-TOOL-05, KT-TOOL-05, RUST-TOOL-07, SCALA-TOOL-08, ELIXIR-TOOL-08, ERL-TOOL-08, CLJ-TOOL-07 | go-only (`govulncheck`, optional) |
| SBOM | Generate a CycloneDX or SPDX SBOM for deployables | 14 | PY-TOOL-10, TS-TOOL-08, JAVA-TOOL-06, CPP-TOOL-09, CS-TOOL-07, PHP-TOOL-09, GO-TOOL-07, SWIFT-TOOL-05, KT-TOOL-05, RUST-TOOL-07, SCALA-TOOL-08, ELIXIR-TOOL-09, ERL-TOOL-08, CLJ-TOOL-08 | missing |
| *Near-universal:* strict linter | The de-facto linter, warnings fail | 11 (below threshold) | TS-TOOL-03, JAVA-TOOL-03, CPP-TOOL-05, CS-TOOL-03, GO-TOOL-04, SWIFT-TOOL-03, RUST-TOOL-03, SCALA-TOOL-07, ELIXIR-TOOL-05, ERL-TOOL-03, CLJ-TOOL-03 | go-only |

### ARCH: architecture

| Practice | Rule (one line) | n | Rule IDs | loops today |
|---|---|---|---|---|
| Pure domain | The domain imports no IO, framework, persistence or transport packages | 15 | PY-ARCH-07, TS-ARCH-07, JAVA-ARCH-05, CPP-ARCH-08, CS-ARCH-07, PHP-ARCH-07, GO-ARCH-02, RUBY-ARCH-09, SWIFT-ARCH-09, KT-ARCH-06, RUST-ARCH-07, SCALA-ARCH-02, ELIXIR-ARCH-07, ERL-ARCH-07, CLJ-ARCH-06 | neutral concept (HX §3, §5); the checker is go-only |
| Clock, RNG and IDs injected | No wall clock, global RNG or random UUID in the domain; inject them | 15 | same rows as Pure domain | neutral (`skills/consistency`, HX §20) |
| Consumer-owned, small ports | The consumer declares the interface, trait, protocol or behaviour it calls; keep it small | 15 | PY-ARCH-08, TS-ARCH-08, JAVA-ARCH-07, CPP-ARCH-09, CS-ARCH-08, PHP-ARCH-08, GO-ARCH-07, RUBY-ARCH-10, SWIFT-ARCH-10, KT-ARCH-07, RUST-ARCH-08, SCALA-ARCH-03, ELIXIR-ARCH-08, ERL-ARCH-05, CLJ-ARCH-07 | neutral (HX §6–§8) |
| Constructor injection at one composition root | Wire by hand or by container at `main`; no service locator or field injection | 14 | TS-ARCH-06, JAVA-ARCH-04, CPP-ARCH-07, CS-ARCH-06, PHP-ARCH-06, GO-ARCH-06, RUBY-ARCH-08, SWIFT-ARCH-08, KT-ARCH-05, RUST-ARCH-06, SCALA-ARCH-05, ELIXIR-ARCH-06, ERL-ARCH-05, CLJ-ARCH-04 | neutral (HX §21) |
| Enforce direction with a tool | A mechanical dependency check runs in CI (ArchUnit, deptrac, import-linter, …) | 15 | PY-ARCH-05, TS-ARCH-04, JAVA-ARCH-03, CPP-ARCH-06, CS-ARCH-04, PHP-ARCH-05, GO-ARCH-05, RUBY-ARCH-07, SWIFT-ARCH-07, KT-ARCH-04, RUST-ARCH-04, SCALA-ARCH-09, ELIXIR-ARCH-04, ERL-ARCH-04, CLJ-ARCH-03 | go-only (`loops arch`) |
| Size heuristics | File, type, function and parameter limits, as warnings | 15 | PY-ARCH-09, TS-ARCH-09, JAVA-ARCH-08, CPP-ARCH-11, CS-ARCH-09, PHP-ARCH-09, GO-ARCH-08, SWIFT-ARCH-11, KT-ARCH-08, RUST-ARCH-10, SCALA-ARCH-12, ELIXIR-ARCH-10, ERL-ARCH-03, CLJ-ARCH-09, RUBY-ARCH-02 | go-only (SOLID §8 in Go units) |

### IDIOM: what to do and what to avoid

| Practice | Rule (one line) | n | Rule IDs | loops today |
|---|---|---|---|---|
| Never swallow errors | No empty catch or bare rescue; add context when rethrowing; handle once | 14 | PY-IDIOM-02, JAVA-IDIOM-02, CPP-IDIOM-02, CS-IDIOM-01, PHP-IDIOM-01, GO-IDIOM-01, RUBY-IDIOM-01, SWIFT-IDIOM-01, KT-IDIOM-01, RUST-IDIOM-01, SCALA-IDIOM-01, ELIXIR-IDIOM-02, ERL-IDIOM-01, CLJ-IDIOM-01 | go-only (GO_BEST §8); the taxonomy is neutral (HX §13) |
| Expected outcomes are values | Business outcomes are result or tagged values; exceptions or panics are for faults | 13 | TS-IDIOM-01, JAVA-IDIOM-06, CPP-IDIOM-01, CS-IDIOM-01, GO-IDIOM-01, RUBY-IDIOM-05, SWIFT-IDIOM-05, KT-IDIOM-01, RUST-IDIOM-01, SCALA-IDIOM-01, ELIXIR-IDIOM-01, ERL-IDIOM-01, CLJ-IDIOM-01 | neutral (HX §13) plus go-only (GO_BEST §8) |
| Immutable value types | Records, data classes, `readonly`, frozen values for domain values | 15 | PY-IDIOM-05, TS-IDIOM-05, JAVA-IDIOM-05, CPP-IDIOM-07, CS-IDIOM-04, PHP-IDIOM-04, GO-IDIOM-05, RUBY-IDIOM-04, SWIFT-IDIOM-04, KT-IDIOM-05, RUST-IDIOM-06, SCALA-IDIOM-03, ELIXIR-IDIOM-05, ERL-IDIOM-05, CLJ-IDIOM-04 | missing |
| Scoped resources | Release files, connections and tasks deterministically (RAII, `using`, `with`, `defer`, `after`) | 15 | PY-IDIOM-08, TS-IDIOM-08, JAVA-IDIOM-07, CPP-IDIOM-05, CS-IDIOM-06, PHP-IDIOM-09, GO-IDIOM-09, RUBY-IDIOM-06, SWIFT-IDIOM-06, KT-IDIOM-07, RUST-IDIOM-08, SCALA-IDIOM-05, ELIXIR-IDIOM-08, ERL-IDIOM-03, CLJ-IDIOM-07 | go-only / missing |
| UTC instants, zone at the edge | Store and compute in UTC instants; convert at the boundary | 15 | PY-IDIOM-09, TS-IDIOM-10, JAVA-IDIOM-09, CPP-IDIOM-10, CS-IDIOM-08, PHP-IDIOM-08, GO-IDIOM-10, RUBY-IDIOM-07, SWIFT-IDIOM-07, KT-IDIOM-08, RUST-IDIOM-10, SCALA-IDIOM-06, ELIXIR-IDIOM-09, ERL-IDIOM-09, CLJ-IDIOM-10 | neutral in 10 rows (`skills/consistency`) |
| Metaprogramming sparingly | Reflection, macros and monkey-patching only at framework edges, never in the domain | 15 | PY-IDIOM-13, TS-IDIOM-12, JAVA-IDIOM-11, CPP-IDIOM-13, CS-IDIOM-10, PHP-IDIOM-11, GO-IDIOM-14, RUBY-IDIOM-09, SWIFT-IDIOM-09, KT-IDIOM-10, RUST-IDIOM-12, SCALA-IDIOM-08, ELIXIR-IDIOM-11, ERL-IDIOM-11, CLJ-IDIOM-12 | missing |
| *Near-universal:* money is never a float | Decimal or integer minor units | 10 (below threshold) | PY-IDIOM-10, TS-IDIOM-10, JAVA-IDIOM-09, PHP-IDIOM-08, GO-IDIOM-05, RUBY-IDIOM-11, SWIFT-IDIOM-07, RUST-IDIOM-13, ELIXIR-IDIOM-09, CLJ-IDIOM-10 | missing |

### CONC: concurrency

| Practice | Rule (one line) | n | Rule IDs | loops today |
|---|---|---|---|---|
| Ownership or message passing over shared mutation | One owner per mutable state; immutable snapshots or messages for others | 15 | PY-CONC-03, TS-CONC-02, JAVA-CONC-05, CPP-CONC-02, CS-CONC-02, PHP-CONC-02, GO-CONC-02, RUBY-CONC-02, SWIFT-CONC-02, KT-CONC-03, RUST-CONC-02, SCALA-CONC-02, ELIXIR-CONC-02, ERL-CONC-02, CLJ-CONC-01 | neutral core (HX §12) but worded in GO_BEST §11–§12 |
| Never block the event loop or scheduler | Blocking IO goes to a dedicated pool | 15 | PY-CONC-01, TS-CONC-06, JAVA-CONC-05, CPP-CONC-07, CS-CONC-05, PHP-CONC-05, GO-CONC-07, RUBY-CONC-01, SWIFT-CONC-07, KT-CONC-03, RUST-CONC-01, SCALA-CONC-05, ELIXIR-CONC-05, ERL-CONC-06, CLJ-CONC-04 | missing |
| Cancellation and deadlines propagate | Every outbound call has a deadline derived from the inbound request | 15 | PY-CONC-09, TS-CONC-08, JAVA-CONC-07, CPP-CONC-07, CS-CONC-07, PHP-CONC-05, GO-CONC-03, RUBY-CONC-08, SWIFT-CONC-06, KT-CONC-02, RUST-CONC-06, SCALA-CONC-05, ELIXIR-CONC-05, ERL-CONC-06, CLJ-CONC-03 | go-only (GO_BEST §10; HX §14 uses `context.Context`) |
| Bounded concurrency | No unbounded fan-out; bounded pools and backpressure | 15 | PY-CONC-09, TS-CONC-07, JAVA-CONC-07, CPP-CONC-07, CS-CONC-05, PHP-CONC-05, GO-CONC-02, RUBY-CONC-02, SWIFT-CONC-07, KT-CONC-03, RUST-CONC-02, SCALA-CONC-05, ELIXIR-CONC-06, ERL-CONC-07, CLJ-CONC-04 | go-only (GO_BEST §11) |
| Graceful worker shutdown | Workers drain on SIGTERM with a deadline | 15 | PY-CONC-11, TS-OPS-04, JAVA-CONC-08, CPP-CONC-08, CS-CONC-08, PHP-CONC-06, GO-CONC-09, RUBY-OPS-04, SWIFT-CONC-09, KT-CONC-02, RUST-CONC-10, SCALA-CONC-07, ELIXIR-OPS-04, ERL-CONC-08, CLJ-CONC-06 | neutral (`skills/jobs-and-state-machines`, HX §22) |
| A race policy on every test run | Run the language's race detector, or its stated substitute (§5) | 15 | PY-CONC-06, TS-CONC-05, JAVA-CONC-06, CPP-CONC-04, CS-CONC-04, PHP-CONC-04, GO-CONC-04, RUBY-CONC-05, SWIFT-CONC-03, KT-CONC-04, RUST-CONC-04, SCALA-CONC-04, ELIXIR-CONC-04, ERL-CONC-04, CLJ-CONC-08 | go-only (race-guard) |

### TEST: testing

| Practice | Rule (one line) | n | Rule IDs | loops today |
|---|---|---|---|---|
| Recognisable test files | Tests live where the ecosystem expects, matched by stated globs | 15 | PY-TEST-02, TS-TEST-02, JAVA-TEST-02, CPP-TEST-02, CS-TEST-02, PHP-TEST-02, GO-TEST-02, RUBY-TEST-02, SWIFT-TEST-02, KT-TEST-02, RUST-TEST-02, SCALA-TEST-02, ELIXIR-TEST-02, ERL-TEST-02, CLJ-TEST-02 | go-only (`*_test.go`) |
| Parametrised tests | Table, parametrised or data-driven tests when cases share structure | 15 | PY-TEST-04, TS-TEST-03, JAVA-TEST-03, CPP-TEST-03, CS-TEST-03, PHP-TEST-03, GO-TEST-03, RUBY-TEST-03, SWIFT-TEST-03, KT-TEST-03, RUST-TEST-03, SCALA-TEST-03, ELIXIR-TEST-03, ERL-TEST-03, CLJ-TEST-03 | go-only (`skills/testing`) |
| Fakes at ports, not mocks everywhere | In-memory fakes for outbound ports; don't mock what you own | 15 | PY-TEST-06, TS-TEST-04, JAVA-TEST-04, CPP-TEST-04, CS-TEST-04, PHP-TEST-04, GO-TEST-05, RUBY-TEST-04, SWIFT-TEST-04, KT-TEST-04, RUST-TEST-04, SCALA-TEST-04, ELIXIR-TEST-04, ERL-TEST-04, CLJ-TEST-04 | neutral (HX §24) plus go-only (skill) |
| Real dependencies in integration tests | Testcontainers or the equivalent, with isolation per test | 14 | PY-TEST-07, TS-TEST-05, JAVA-TEST-05, CPP-TEST-05, CS-TEST-05, PHP-TEST-05, GO-TEST-12, RUBY-TEST-05, SWIFT-TEST-05, KT-TEST-05, RUST-TEST-05, SCALA-TEST-05, ERL-TEST-05, CLJ-TEST-05 | missing |
| Coverage in a machine format | The coverage tool writes lcov, Cobertura, JaCoCo, … at a known path | 15 | PY-TEST-08, TS-TEST-06, JAVA-TEST-06, CPP-TEST-06, CS-TEST-06, PHP-TEST-06, GO-TEST-06, RUBY-TEST-06, SWIFT-TEST-06, KT-TEST-06, RUST-TEST-06, SCALA-TEST-06, ELIXIR-TEST-06, ERL-TEST-06, CLJ-TEST-06 | go-only |
| ≥70% coverage of new code | Changed lines against the git base, computed by loops | 15 | PY-TEST-10, TS-TEST-08, JAVA-TEST-07, CPP-TEST-07, CS-TEST-09, PHP-TEST-07, GO-TEST-07, RUBY-TEST-07, SWIFT-TEST-07, KT-TEST-07, RUST-TEST-07, SCALA-TEST-07, ELIXIR-TEST-07, ERL-TEST-07, CLJ-TEST-07 | go-only (rule neutral in ROLES.md, enforced for Go) |
| Property and fuzz tests where they pay | Parsers, codecs, money, state machines | 15 | PY-TEST-11, TS-TEST-09, JAVA-TEST-08, CPP-TEST-08, CS-TEST-10, PHP-TEST-08, GO-TEST-09, RUBY-TEST-08, SWIFT-TEST-08, KT-TEST-08, RUST-TEST-09, SCALA-TEST-08, ELIXIR-TEST-08, ERL-TEST-08, CLJ-TEST-08 | go-only |
| Deterministic, isolated, random-order tests | Random order with a printed seed, a controlled clock, no sleeps, parallel-safe | 15 | PY-TEST-13, TS-TEST-10, JAVA-TEST-09, CPP-TEST-09, CS-TEST-11, PHP-TEST-09, GO-TEST-10, RUBY-TEST-09, SWIFT-TEST-09, KT-TEST-09, RUST-TEST-11, SCALA-TEST-09, ELIXIR-TEST-09, ERL-TEST-09, CLJ-TEST-10 | neutral for the clock and RNG (`skills/consistency`); go-only for the order and seed |

### SEC: security

| Practice | Rule (one line) | n | Rule IDs | loops today |
|---|---|---|---|---|
| Parameterised queries | Never build SQL or shell commands from strings | 15 | PY-SEC-01, TS-SEC-01, JAVA-SEC-01, CPP-SEC-03, CS-SEC-01, PHP-SEC-01, GO-SEC-01, RUBY-SEC-01, SWIFT-SEC-01, KT-SEC-01, RUST-SEC-01, SCALA-SEC-01, ELIXIR-SEC-01, ERL-SEC-01, CLJ-SEC-01 | go-only (GO_BEST §38) |
| No unsafe deserialisation | No native object deserialisers on untrusted input; typed, size-limited decoding | 15 | PY-SEC-02, TS-SEC-02, JAVA-SEC-02, CPP-SEC-04, CS-SEC-02, PHP-SEC-02, GO-SEC-02, RUBY-SEC-02, SWIFT-SEC-02, KT-SEC-02, RUST-SEC-02, SCALA-SEC-02, ELIXIR-SEC-02, ERL-SEC-03, CLJ-SEC-03 | missing |
| Secrets from env or vault, never logged | No secrets in code, config files or logs | 15 | PY-SEC-06, TS-SEC-04, JAVA-SEC-04, CPP-SEC-06, CS-SEC-04, PHP-SEC-05, GO-SEC-06, RUBY-SEC-05, SWIFT-SEC-04, KT-SEC-04, RUST-SEC-04, SCALA-SEC-04, ELIXIR-SEC-05, ERL-SEC-06, CLJ-SEC-06 | go-only (GO_BEST §24, §38) |
| Modern password hashing and a CSPRNG | Argon2, bcrypt or scrypt for passwords; the platform CSPRNG for tokens | 15 | PY-SEC-06, TS-SEC-04, JAVA-SEC-04, CPP-SEC-06, CS-SEC-05, PHP-SEC-04, GO-SEC-07, RUBY-SEC-05, SWIFT-SEC-04, KT-SEC-04, RUST-SEC-04, SCALA-SEC-04, ELIXIR-SEC-05, ERL-SEC-07, CLJ-SEC-07 | go-only (GO_BEST §21) |
| Validate at the inbound adapter | Parse, then validate once at the edge, then use typed commands | 15 | PY-SEC-07, TS-SEC-05, JAVA-SEC-05, CPP-API-04, CS-SEC-06, PHP-SEC-08, GO-SEC-08, RUBY-SEC-06, SWIFT-SEC-05, KT-SEC-05, RUST-SEC-05, SCALA-SEC-05, ELIXIR-SEC-06, ERL-SEC-09, CLJ-SEC-08 | neutral (HX §7, §10) |
| Supply-chain hygiene | Lockfile integrity, scoped registries, install scripts off | 15 | PY-SEC-08, TS-SEC-06, JAVA-SEC-06, CPP-SEC-08, CS-SEC-07, PHP-SEC-09, GO-SEC-09, RUBY-SEC-07, SWIFT-SEC-06, KT-SEC-06, RUST-SEC-06, SCALA-SEC-06, ELIXIR-SEC-07, ERL-SEC-10, CLJ-SEC-09 | missing |
| SAST in CI | Run the ecosystem's static security analyser | 15 | PY-SEC-09, TS-SEC-08, JAVA-SEC-07, CPP-SEC-09, CS-SEC-08, PHP-SEC-10, GO-SEC-10, RUBY-SEC-08, SWIFT-SEC-07, KT-SEC-07, RUST-SEC-07, SCALA-SEC-07, ELIXIR-SEC-08, ERL-SEC-11, CLJ-SEC-10 | missing |

### DATA, API, OBS, OPS, DOC, FW

| Practice | Rule (one line) | n | Rule IDs (representative) | loops today |
|---|---|---|---|---|
| Versioned, forward migrations, expand then contract | Migrations are versioned; zero-downtime changes expand first | 15 | PY-DATA-04, TS-DATA-02, JAVA-DATA-02, GO-DATA-02, RUBY-DATA-02, ELIXIR-DATA-03, … | neutral (`skills/consistency`) |
| Transactions in the application layer | The use case owns the unit of work | 15 | PY-DATA-05, JAVA-DATA-03, CS-DATA-03, GO-DATA-03, RUST-DATA-03, … | neutral (HX §18) |
| Avoid N+1 queries | Preload or batch | 14 | PY-DATA-06, JAVA-DATA-04, CS-DATA-04, PHP-DATA-04, RUBY-PERF-04, … | missing |
| Pools with timeouts | Bounded connection pools with acquire and statement timeouts | 15 | PY-DATA-07, TS-DATA-05, JAVA-DATA-07, GO-DATA-04, ELIXIR-DATA-06, … | missing |
| Problem details at the edge | RFC 9457 errors; status mapping in the inbound adapter | 15 | PY-API-04, TS-API-03, JAVA-API-03, GO-API-04, KT-API-03, … | neutral (HX §13) |
| Timeouts, retries with backoff, idempotency keys | Outbound resilience lives in adapters | 15 | PY-API-05, TS-API-04, JAVA-API-04, PHP-API-04, GO-API-06, … | neutral (`skills/consistency`, `skills/jobs-and-state-machines`) |
| Transactional outbox | Publish events through an outbox | 14 | PY-API-07, JAVA-API-05, CS-DATA-06, GO-API-08, ELIXIR-API-05, … | neutral (HX §16, jobs skill) |
| Contract-first APIs | OpenAPI or protobuf as the contract; versioned | 15 | PY-API-02, TS-API-02, JAVA-API-02, GO-API-03, RUST-API-03, … | missing |
| Reuse HTTP clients, propagate deadlines | One configured client per dependency | 15 | PY-API-08, TS-API-06, JAVA-API-06, GO-API-07, RUBY-API-04, … | go-only (GO_BEST §10) |
| Structured logs with correlation IDs, no personal data | Machine-parsable logs; IDs on every request | 15 | PY-OBS-01, TS-OBS-01, JAVA-OBS-01, GO-OBS-01, ERL-OBS-01, … | go-only (GO_BEST §24) |
| OpenTelemetry | OTel SDK for traces and metrics; propagate context | 15 | PY-OBS-03, TS-OBS-02, JAVA-OBS-03, GO-OBS-02, RUST-OBS-03, … | missing |
| Separate liveness and readiness | Distinct health endpoints | 15 | PY-OBS-05, TS-OBS-04, JAVA-OBS-04, GO-OBS-04, ELIXIR-OBS-04, … | missing |
| Typed config from the environment, validated at startup | 12-factor; fail fast with every error listed | 15 | PY-OPS-01, TS-OPS-01, JAVA-OPS-01, GO-OPS-01, CLJ-OPS-01, … | neutral core (HX §19) plus go-only (GO_BEST §28) |
| Multi-stage, minimal, non-root images | Slim, distroless or chiselled runtime images | 15 | PY-OPS-02, TS-OPS-02, JAVA-OPS-02, GO-OPS-02, RUST-OPS-02, … | missing |
| Graceful shutdown on SIGTERM | Drain in reverse init order with a deadline | 15 | same rows as Graceful worker shutdown | neutral (HX §22) |
| Run-scoped build caches | Share download caches; scope build outputs and tmp per run | 15 | PY-OPS-06, TS-OPS-05, JAVA-OPS-05, GO-OPS-05, RUST-OPS-05, … | go-only (`go-env`) |
| Known artifact list | Every build and test output is listed for cleanup | 15 | PY-OPS-07, TS-OPS-06, JAVA-OPS-06, GO-OPS-06, ERL-OPS-06, … | go-only (`workspace`) |
| Doc comments on the public API | Use the ecosystem's doc tool and comment form | 15 | PY-DOC-01, TS-DOC-01, JAVA-DOC-01, GO-DOC-01, RUST-DOC-01, … | go-only (GO_BEST §26) |
| README, ADRs, changelog | Repo docs norms | 15 | PY-DOC-02, JAVA-DOC-02, GO-DOC-02, … | neutral (HX §26, `skills/documentation`) |
| A language-specific review focus | Reviewers check the language's top traps first | 15 | PY-DOC-03, TS-DOC-03, … CLJ-DOC-03 | go-only (GO_BEST §51) |
| Follow the framework; project rules win | Use the framework's conventions where the project uses it; keep the domain framework-free | 15 | PY-FW-02, JAVA-FW-02, GO-FW-02, RUBY-FW-02, KT-ARCH-09, … | neutral (WORKFLOW §1) |

## 3. Proposed Stage C specs

Each spec states the rule once; the packs cite it (`REPO §3`) and add only mechanics. The section lists come from the practices above.

| Spec | Replaces or extends | Sections |
|---|---|---|
| `specs/REPO_PRACTICES.md` | new; the Go parts of GO_BEST §48–§49 move here | 1 Layout per ecosystem (points to the pack) · 2 One repo/module until a deployable splits · 3 Manifest, lockfile and frozen install · 4 Toolchain pinning · 5 Dependency policy (stdlib → library → own code; adding and removing deps) · 6 Update bots and cooldowns · 7 Versioning, tags, release, provenance · 8 Hygiene files (`.gitignore`, `.editorconfig`, CODEOWNERS, licence) · 9 CI stage order and caching · 10 Conventional commits |
| `specs/ARCHITECTURE.md` | the neutral core of `HEXAGONAL_ARCHITECTURE.md` (§1–§3, §5–§13, §16–§22, §24, §26–§30) | 1 Model · 2 Dependency rule · 3 Layers and what each owns · 4 Ports (consumer-owned, inbound, outbound) · 5 Commands and queries · 6 One owner per state · 7 Error taxonomy · 8 Domain events, state machines · 9 Persistence, config and time boundaries · 10 DI and composition root · 11 Initialisation and shutdown order · 12 Testing architecture · 13 Mechanical enforcement (`loops arch` plus the pack's `native_tool`) · 14 ADRs, smells, migration policy, compliance. Per-language layout trees go in `specs/languages/<lang>/ARCH.md` |
| `specs/SOLID.md` | the neutral core of `SOLID_GO.md` (§1, §3–§7, §10–§11) | 1 Why · 2 S, O, L, I, D, stated per unit (type/module/package) · 3 Limits as a table in *neutral units* (methods per type, fields per stateful type, port size, lines and files per module), with the per-language unit mapping in the pack (`arch.size_units`) · 4 Global mutable state · 5 How to split a god type · 6 What not to do |
| `specs/TESTING.md` | `skills/testing` core plus GO_BEST §30 | 1 Test pyramid and ownership · 2 Recognition (pack `test_globs`) · 3 Style: behaviour names, parametrised, AAA · 4 Doubles: fakes at ports · 5 Integration with real dependencies · 6 Determinism: clock, RNG, order, seed · 7 Property and fuzz tests · 8 The hard gate: ≥70% of new code, and the race policy (§5 below) · 9 Flakiness: no retries in the gate · 10 TDD loop speed |
| `specs/CONCURRENCY.md` | the neutral core of GO_BEST §10–§13 | 1 Models (threads, async, actors, BEAM) · 2 Ownership and message passing · 3 Cancellation and deadlines · 4 Bounded concurrency and backpressure · 5 Never block the loop · 6 Background work and shutdown · 7 Race policy (cites TESTING §8) |
| `specs/SECURITY.md` | GO_BEST §38, §44 | 1 Untrusted input and validation at the edge · 2 Injection (SQL, command, template) · 3 Deserialisation and parsing limits · 4 Web risks (SSRF, path traversal, CSRF/CORS) · 5 Secrets, crypto, password hashing, CSPRNG · 6 Supply chain (lockfile, registries, install scripts, SBOM) · 7 Vulnerability scan and SAST in the gate |
| `specs/PERSISTENCE_AND_INTEGRATION.md` | parts of `skills/consistency` and `skills/jobs-and-state-machines`, GO_BEST §45–§46 | 1 Access style · 2 Migrations: versioned, expand/contract · 3 Transactions in the application layer · 4 ORM traps (N+1, lazy loading) · 5 Pools and timeouts · 6 HTTP edge: problem details · 7 Outbound resilience: timeouts, retries, idempotency · 8 Messaging and outbox · 9 Contract-first APIs |
| `specs/OPERATIONS.md` | GO_BEST §24, §28, §47; HX §19, §22 | 1 Config: typed, validated, from the environment · 2 Structured logging, no personal data · 3 Metrics and OpenTelemetry · 4 Health: liveness and readiness · 5 Packaging and images · 6 Graceful shutdown · 7 Run-scoped caches and artifacts (the loops runspace contract; cites TOOLING) |
| `specs/GO_BEST_PRACTICES.md` | kept, trimmed to Go | the Go-only rows of §4 below, each citing the neutral spec for the rule |

## 4. Audit of the Go specs: universal or Go-only?

**Inputs:**
- reading the specs section by section;
- LOOPS_BASELINE;
- go.md's "Proposed updates to GO_BEST_PRACTICES" table (PU).

**Status values:**
- **U:** the rule is universal; lift it into a neutral spec and keep a Go example.
- **G:** Go-only.
- **U+G:** a neutral rule worded with Go mechanics; split it.

### GO_BEST_PRACTICES.md

| § | Topic | Class | Neutral home | Notes, including PU |
|---|---|---|---|---|
| 1 | Guiding principle | U+G | ARCHITECTURE intro | "boring, explicit, measurable" holds everywhere |
| 2 | Formatting and static analysis | U+G | TESTING §8, REPO §9 | PU P1: `tidy -diff`, `-shuffle`, golangci-lint v2, `go tool`; PU P0: tools from PATH leave the gate green |
| 3 | Package naming | G | — | |
| 4 | Export only what is needed | U | ARCHITECTURE §4 | minimal public API (IDIOM-07 in every file) |
| 5–6 | Interfaces; accept interfaces, return concrete | U+G | ARCHITECTURE §4, SOLID | consumer-owned small ports are universal; "return concrete types" is Go |
| 7 | Constructors and zero values | G | — | "constructors establish invariants" is U |
| 8 | Error handling | U+G | ARCHITECTURE §7 | expected outcomes as values, wrap with context, handle once (14 files); PU P2: `errors.AsType`, `errors.Join` |
| 9 | Sentinel and typed errors | G | — | |
| 10 | Context | U+G | CONCURRENCY §3 | deadlines from the inbound request; PU P2 |
| 11–12 | Concurrency; mutexes and atomics | U+G | CONCURRENCY §2, §4 | PU P1: `WaitGroup.Go`, goroutineleak, GOMAXPROCS |
| 13 | Consistent ordering and clocks | U | `skills/consistency` (already neutral) | PU P1: `testing/synctest` |
| 14–15 | Maps and slices; struct layout | G | — | PU P2 |
| 16 | Reference entities by ID | U | ARCHITECTURE §8 | |
| 17 | Generics, reflection, `any` | U+G | IDIOM in packs | "metaprogramming sparingly" is universal (15 files); PU P1 |
| 18–19 | Enums and IDs; constants | U+G | ARCHITECTURE | typed IDs, no magic numbers |
| 20–21 | Time; randomness | U | CONCURRENCY/`skills/consistency` | UTC (15 files), injected RNG; PU P2: `math/rand/v2` |
| 22–23 | Allocation, pools, `defer`; strings | G | `skills/performance` | |
| 24 | Logging | U+G | OPERATIONS §2 | PU P1: no OpenTelemetry guidance |
| 25 | Panics and assertions | U+G | ARCHITECTURE §7 | "faults only, recover at the edge" is U |
| 26 | Doc comments | U+G | `skills/documentation` | PU P2 |
| 27 | Functions and parameters | U | SOLID §3 | |
| 28 | Configuration and content loading | U | OPERATIONS §1 | PU P2 |
| 29 | File I/O, paths, serialisation | U+G | SECURITY §4 | PU **P0**: `os.Root`/`OpenInRoot` replaces manual traversal checks |
| 30 | Testing | U+G | TESTING | PU P1. The "Other languages" table contradicts the packs: Python `--cov-fail-under=70` is whole-project, not new-code; Rust nightly TSan vs. the pack's `none` + miri/loom; Java/Kotlin `jcstress` vs. the packs' parallel + random order. **Replace it with a pointer to the packs** (PU says the same) |
| 31 | Benchmarks and profiling | U+G | `skills/performance` | PU P1: `b.Loop`, PGO |
| 32–36 | Complexity notes; data-oriented bias; ordering; hidden work; caches | U | `skills/performance`, `skills/consistency` | |
| 37 | Build tags, unsafe, cgo, generated code | U+G | REPO, SECURITY | "never hand-edit generated code" is U; PU P1: `-race` needs cgo |
| 38 | Security | U+G | SECURITY | PU **P0**: thin; add OWASP, `gosec`, server timeouts |
| 39–40 | API design for performance; memory retention | G | `skills/performance` | PU P1/P2 |
| 41 | API ownership, naming | U | ARCHITECTURE §4 | PU P2: avoid `pkg/` by default |
| 42–47 | Domain language, transitions, boundary validation, migrations, idempotency, observability | U | ARCHITECTURE, PERSISTENCE, OPERATIONS | already neutral in substance (LOOPS_BASELINE) |
| 48 | Dependencies: stdlib → library → own | U+G | REPO §5 | PU **P0**: the stdlib list is stale (1.27 `uuid`, json/v2) |
| 49 | Dependency updates | U+G | REPO §6 | PU P1: bots, `tidy -diff`, per-module upgrades |
| 50 | Custom lint rules | U+G | ARCHITECTURE §13 | PU P1 |
| 51–52 | Review and AI-contributor checklists | U+G | `skills/review-integration` | PU P2; each pack adds its DOC-03 focus |
| 53 | Performance escalation path | U | `skills/performance` | |
| new | Repo and ops sections | — | REPO_PRACTICES, OPERATIONS | PU **P0**: layout, toolchain, releases, CI, containers, persistence, API, OTel |

### HEXAGONAL_ARCHITECTURE.md

| § | Class | Notes |
|---|---|---|
| 1–3, 5–13, 16–21, 24, 26–30 | U | Model, the dependency rule, ports, commands, queries, ownership, error taxonomy, events, state machines, the boundaries, DI, testing architecture, ADRs, smells, migration, compliance. §24 mentions `httptest`; give per-language examples. |
| 4 Repository structure | G | The `cmd/`, `internal/`, `pkg/` tree. Replace it with a neutral layer list plus per-language trees from each file's ARCH-02. |
| 14 Context | G | `context.Context`. The neutral rule (deadlines at IO boundaries, not in the domain) moves to CONCURRENCY §3. |
| 15 Package boundaries, `pkg/` | U+G | "One concept per module; canonical types have one home" is U. `pkg/` is G (PU P2). |
| 22 Initialisation order | U | Already neutral text. |
| 23 `cmd` is wiring only | U+G | "The composition root is wiring only" is U. The 80-line `main.go` rule and the homes table are G. |
| 25 Architecture lint | U+G | The rule table is U as concepts. The Go mechanics (`os/*`, `net/http/*`, `time.Now`, `math/rand`, `go`, `init`) become the pack's `arch` data. |

### SOLID_GO.md

| § | Class | Notes |
|---|---|---|
| 1, 3–7, 10–11 | U+G | The principles are U. The examples, "40 methods" and "8 methods" are Go units; restate them in neutral units with a pack mapping. |
| 2 Go vocabulary | G | Each pack supplies its own vocabulary: class, trait, protocol, behaviour, namespace. |
| 8 Measurable limits | U+G | The `methods`, `fields`, `iface`, `lines` and `files` warnings are U if the pack defines the unit. `init`, `global`, `clock`, `rand`, `go` and `cmd` are Go detections of U rules: no implicit registration, no global mutable state, a pure domain, a wiring-only root. |
| 8a `cmd` is wiring | G | |
| 9 Package-level state | U+G | "No runtime-assigned global state" is U (Ruby, Elixir, Clojure and Erlang state it in their own terms). The allowed-list is Go. |

## 5. The hard test gate, generalised

The rule (ROLES.md, WORKFLOW §6 and §8 item 3) has two parts. Both stay hard for every pack.

### 5.1 ≥70% coverage of new code

The diff and the evaluation in `internal/domain/coverage` are already language-neutral. Generalising needs:

1. **The parser per format.** Each produces the same `Profile` (file → covered/uncovered line ranges). See TOOLING §3.
2. **Candidates.** Changed lines in files that:
   - match the pack's `source_globs`;
   - are not in its `test_globs`;
   - are not generated (`generated.globs` or `generated.header_regex`);
   - are outside the pack's `coverage.exclude` (replacing Go's `DefaultExclude`: `cmd/**`, `internal/compose/**`).

   The config only adds exclusions.
3. **Path normalisation** to repo-relative paths: absolute paths in Clover and excoveralls, `./` in simplecov-lcov, Cobertura `<sources>` roots, JaCoCo package plus source-file names (TOOLING §3).
4. **Multi-file merge.** Erlang writes one file per app; C#, Java and Kotlin write one per project or module.
5. **Granularity** is normalised to lines. Go and Scala report statements, Python branches, Clojure forms.
6. **"A changed unit has no tests"** is Go's package rule.
   - It generalises only where the pack can map a source unit to its tests: a Go package, a Java/Kotlin/Scala source set, Rust inline modules.
   - Elsewhere the per-file "all new lines untested" finding carries it.
   - Which units a pack supports is a Stage C detail (TOOLING decision D7).
7. **The floor** stays `coverage.DefaultMin = 70`. Config may only raise it.

### 5.2 The race rule

What the 15 packs declare (`race.mode` in each LOOPS block):

| Mode | Languages | Count |
|---|---|---|
| `flag` | Go (`-race`), Swift (`--sanitize=thread`, plus Swift 6 mode) | 2 |
| `build` | C++ (a TSan preset; it can't share a build with ASan) | 1 |
| `none` + substitute | Python, TypeScript, Java, C#, PHP, Ruby, Kotlin, Rust (plus an optional `race.build` tier), Scala, Elixir, Erlang, Clojure | **12** |

The `none` substitutes, as each pack's `race.substitute` states them, mix four ingredients:
- **random order with a printed seed** (8): Python, TS, Java, PHP, Ruby, Kotlin, Elixir, Clojure;
- **parallel execution** (9): Python `-n 2`, TS `--sequence.concurrent`, Java/Kotlin JUnit concurrent, PHP paratest, Ruby parallel_tests, Scala parallel suites, Elixir `async: true`, C# xUnit collections;
- **repeat runs** (10):
  - TS `--repeats=3`, Scala 3×, Clojure 3 seeds, Elixir `--repeat-until-failure 5`;
  - Java and PHP run twice with two seeds;
  - Ruby re-runs the changed specs; Kotlin uses `@RepeatedTest` on shared-state tests;
  - C# repeats its `Category=Concurrency` tests;
  - Erlang runs once under each of `+S 1:1` and `+S 8:8`, plus `ct --repeat`;
- **static rules instead of runs** (2):
  - Rust: `unsafe_code=forbid`, clippy `-D warnings`, and miri/loom for crates with `unsafe` or atomics;
  - C#: the CA2007/CA2012/CA2016 and VSTHRD analyzers as errors.

  Clojure adds clj-kondo and reflection warnings on top of its runs.

Rust is the one `none` pack whose substitute has no repeated or randomised run, so its gate check is static. Scala and Erlang repeat but don't randomise.

**Proposed wording** to replace "every test run uses `-race`".

For WORKFLOW §6:

> **Race policy.** Every test run follows the pack's race policy (TAXONOMY CONC-04):
> - `flag`: every test command carries the race flag, and the race-guard denies a test command without it.
> - `build`: the full gate runs the pack's race build (its `race: true` test check), and it must pass.
> - `none`: the full gate runs the pack's substitute checks (its `race: true` checks), and they must pass. The pack states, and the gate records:
>   - the seed of every randomised run;
>   - the parallelism used;
>   - the repeat count applied to the changed tests.
>
>   A pack whose substitute is static (Rust) names the lint checks that carry it.
>
> A required race or substitute check that is skipped fails the full gate, as a skipped required check does today. There is no opt-out: the fix is a cheaper test, never a weaker policy.

For ROLES.md (shared implementer rules):

> every test run uses the race detector or the pack's race substitute (WORKFLOW §6).

For WORKFLOW §8 item 3:

> all tests run under the pack's race policy (§6).

**Why the gate and not the hook.**
- In `none` mode there is no flag to check per command.
- Gradle can't pass JUnit properties from the command line; only `junit-platform.properties` works (KT-CONC-04; java.md Open question 4). Maven can, through Surefire. Erlang's variants are env maps (ERL-CONC-04). A per-command guard would therefore deny legitimate commands.
- The gate is the one place every policy can be checked mechanically. TOOLING §4 shows what race-guard keeps doing per mode.
