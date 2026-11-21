---
name: lang-swift
description: Use when writing, reviewing, testing or setting up Swift backend code in a loops run (the swift language pack) - toolchain, layout, race policy, coverage, idioms and footguns.
---

# Swift (server-side) language pack

Rules: `specs/languages/swift/RULES.md` (IDs `SWIFT-<SEC>-NN`). Pack data: `loops lang show swift`. Universal practice: `specs/ARCHITECTURE.md`, `specs/SOLID.md`, `specs/WORKFLOW.md`.

## When it applies
`loops lang detect` reports `swift` (a `Package.swift` at the root). Targets: SwiftPM only, Swift 6.4.x, SwiftNIO runtime, Hummingbird 2 or Vapor 4. No `.xcodeproj` for a server.

## Project rules first
Follow the project's own config over the pack defaults: an existing `.swift-format`, `.swiftlint.yml`, `.editorconfig`, framework layout (Vapor `Sources/App`, Hummingbird `buildApplication`), tools version, target names. Log every override with `loops run override` (ROLES.md "Project rules first"). `loops lang show swift` lists `config_files` to read first.

## The toolchain the gate runs
Agents run `loops gate`, or single checks while iterating (`loops gate --only <check>,…`). The checks, from `pack.json`:

| Check | Command | Notes |
|---|---|---|
| `build` (required) | `swift build --force-resolved-versions --explicit-target-dependency-import-check error` | a stale `Package.resolved` or an undeclared import fails |
| `swift-format` (required once `.swift-format` exists) | `swift format lint --strict --recursive Sources Tests` | `--strict` makes warnings fail; fix with `swift format format --in-place --recursive …` |
| `swiftlint` (optional, needs `.swiftlint.yml`) | `swiftlint lint --strict` | install: `brew install swiftlint`; Linux via its Docker image |
| `arch` | `loops arch` | regex parser, module = target name (SWIFT-ARCH-06/07) |
| `test` (required) | `swift test --sanitize=thread --enable-code-coverage` | see race policy |
| `coverage` (required) | lcov from `llvm-cov export` | see coverage recipe |
| `sbom` (optional, off) | `swift package generate-sbom` | emits SPDX/CycloneDX only; there is no first-party vuln check (SWIFT-TOOL-05) |

Single test while iterating: `swift test --filter 'DomainTests.OrderTests/testPlace'`, `--skip-build` after a build, `swift test list` to list (SWIFT-TEST-10). There is no `-race`; do not invent one.

Caches: SwiftPM has no cache env vars. The gate and the env hook add `--scratch-path <run>/swift/build` after the subcommand; the shared `--cache-path`, `--config-path`, `--security-path` stay global (SWIFT-OPS-05). Never hardcode `.build/<triple>/debug`: ask `swift build --show-bin-path` (SWIFT-TOOL-07).

## Race policy (WORKFLOW §6, mode `flag`)
1. Every target compiles in Swift 6 language mode: a 6.x `swift-tools-version`, never `.swiftLanguageMode(.v5)` without a logged override (SWIFT-CONC-02, -04).
2. Tests run with `swift test --sanitize=thread` (the pack's `race.flags`). The test guard denies a `swift test` without it.
3. Shared mutable state is an `actor`, a `Mutex` or an immutable `Sendable` value. `@unchecked Sendable` and `nonisolated(unsafe)` need a lock and a comment.
4. TSan cannot combine with `--sanitize=address`; run ASan/UBSan as a separate optional job. If TSan cannot run on the host, the gate fails with "race policy unavailable"; it never skips.

## Coverage recipe
`swift test --enable-code-coverage` writes `default.profdata` beside the path printed by `--show-codecov-path`. The pack then runs `llvm-cov export -format=lcov` (via `xcrun` on macOS) with one `-object` per test binary: macOS `<T>.xctest/Contents/MacOS/<T>`, Linux `<T>.so` under `.build/out/Products/Debug-linux-<arch>` (the `<T>-test-runner` binary holds only generated entry-point code, no package sources). Extra positionals are source filters and drop targets silently, so never pass objects without `-object`. `Tests/`, `.build/`, `checkouts` and `DerivedSources` are filtered out. New-code coverage must be at least 70% (WORKFLOW §6). Details: `testing.md`.

### Prerequisites the architect or `swift-engineer` adds in wave 0 (D13, logged overrides)
- `.swift-format` committed, e.g. `{"version": 1, "indentation": {"spaces": 4}}` to match `swift package init` (the stock template fails `lint --strict` against the default 2 spaces), or run `swift format format -i -r Sources Tests` once. Its presence makes the format check required.
- `Package.resolved` committed for services; `.swift-version` pinning the toolchain, mirrored in the Docker base image and CI.
- A `.gitignore` with `/.build`, `.swiftpm/xcode/`, `.netrc`, `*.profraw`.
- Layer targets in `Package.swift` with `dependencies:` pointing inward.

## How the loops layers map
```
Package.swift                 tools-version 6.x; targets are the boundaries
Sources/
  Domain/                     target, dependencies: []  (pure: no NIO, Vapor, Foundation networking)
  Application/                dependencies: ["Domain"]  use cases + ports (protocols)
  <X>Adapter/                 inbound (HTTPAdapter) / outbound (PostgresAdapter); depend on Application
  App/                        executableTarget: @main, config, wiring
Tests/<Target>Tests/          mirror the targets; Swift Testing
```
Direction is enforced twice: SwiftPM `dependencies:` plus `--explicit-target-dependency-import-check error`, and `loops arch` for purity (`Date()`, `UUID()`, `Task {` in Domain).

## Top 10 footguns
1. `try!`, `!` or an out-of-range index traps and kills the process: `guard let`, `.first`, `try`.
2. Blocking the cooperative pool (`DispatchSemaphore`, sync I/O in `async`): async clients, `Task.sleep(for:)`.
3. `@unchecked Sendable` or `nonisolated(unsafe)` as a silencer: `actor` or `Mutex`.
4. Dropping a target to `.v5` to get green: migrate with `swift package migrate`, log it.
5. `Task {}` or `Task.detached` for request work: structured concurrency.
6. Retain cycles in escaping closures: `[weak self]`.
7. Shipping a debug build: `-c release`.
8. `branch:` dependencies or an uncommitted `Package.resolved`: `from:`, commit, `--force-resolved-versions`.
9. A shared `DateFormatter`, non-UTC dates, `Double` money: `FormatStyle`, `Decimal`.
10. `import Vapor`/`Fluent`/NIO in Domain: target `dependencies: []`, Fluent models in the adapter.

## Load on demand
- `repo.md`: layout, manifest, lockfile, toolchain pin, CI, releases.
- `architecture.md`: targets as layers, access levels, ports, DI, purity.
- `idioms.md`: errors, optionals, value types, enums, resources, time, API design.
- `concurrency.md`: Swift 6 mode, actors, Sendable, TSan, cancellation, lifecycle.
- `testing.md`: Swift Testing, fakes, coverage export, flakiness, TDD loop.
- `security.md`: injection, deserialization, crypto, supply chain, SAST.
- `data-api.md`: persistence, HTTP and messaging, observability, deployment.
- `frameworks.md`: Hummingbird, Vapor, SSWG packages and their footguns.
