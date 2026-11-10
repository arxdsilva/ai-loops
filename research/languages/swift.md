# Swift (server-side): backend practices and loops gap analysis

| | |
|---|---|
| Pack id | `swift` |
| Current stable / LTS | `6.4.2` (6.4 released 2026-09-15) / none (Swift has no LTS; support window = latest release, SSWG packages CI the two latest) |
| Dominant runtimes / frameworks | SwiftNIO runtime; Hummingbird 2 (2.28.x), Vapor 4 (4.122.x; Vapor 5 is Beta 4, not stable); PostgresNIO, Fluent, SQLKit |
| As of | 2026-11 |
| Researcher | `B1-swift` |
| Verifier | (filled in by B2) |

> **Summary:** A strong Swift backend team uses SwiftPM only (no Xcode project), Swift 6 language mode so data races are compile errors, `swift format`/SwiftLint, Swift Testing, and hexagonal layers expressed as SwiftPM targets whose `dependencies:` are the boundary. Biggest loops gaps: (1) the gate, race guard and coverage parser are Go-only, and Swift's race policy is "Swift 6 mode + `--sanitize=thread`", a build-mode flag, not `-race`; (2) coverage needs a two-step `swift test --enable-code-coverage` then `llvm-cov export -format=lcov` and its paths differ by build system and OS; (3) language detection, arch import parsing (module = target name) and cache/artifact handling (`.build/`, `.swiftpm/`) do not exist.

Rules follow [TAXONOMY.md](TAXONOMY.md), using the row format `ID | Rule | Why | Sev | Source | loops today | Enforce`. Gaps are judged against [LOOPS_BASELINE.md](LOOPS_BASELINE.md). Facts marked "verified locally" were run on Swift 6.4.2 / macOS 26 (arm64) on 2026-11-10; Linux paths not verified locally are flagged.

## 1. REPO: repository and project management

Canonical service tree (one SwiftPM package, hexagonal layers as targets, tests mirror targets):

```
my-service/
├── Package.swift                  # swift-tools-version: 6.4; targets are the boundaries
├── Package.resolved               # COMMITTED for executables/services
├── .swift-version                 # swiftly toolchain pin (e.g. 6.4.2)
├── .swift-format                  # JSON config
├── .swiftlint.yml                 # optional
├── .editorconfig  .gitignore  .gitattributes  LICENSE  README.md  CODEOWNERS
├── Sources/
│   ├── Domain/                    # pure: no NIO, no framework, no Foundation networking
│   ├── Application/               # use cases + ports (protocols), depends on Domain
│   ├── Adapters/                  # (optionally split: HTTPAdapter, PostgresAdapter, ...)
│   └── App/                       # executableTarget: composition root, @main
├── Tests/
│   ├── DomainTests/  ApplicationTests/  AdaptersTests/  AppTests/
├── Dockerfile  .dockerignore
└── .github/workflows/ci.yml
```

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `SWIFT-REPO-01` | Use one SwiftPM package with `Sources/<Target>` and `Tests/<Target>Tests`; do not commit an `.xcodeproj` for a server. Executables go in `executableTarget`, libraries in `target`. [REPO-01] | SwiftPM infers target dirs from `Sources/`, `Tests/`; the folder name is the module name used by `import`. | MUST | [SwiftPM Package Manager docs](https://docs.swift.org/latest/documentation/packagemanagerdocs/), [`swift package init`](https://www.swift.org/documentation/server/guides/building.html) | missing | review |
| `SWIFT-REPO-02` | Add `// swift-tools-version: 6.4` (or the lowest version you support, at least 6.0) on line 1 of `Package.swift`. A 6.x tools version makes Swift 6 language mode the default for targets. [REPO-01] | Language mode follows the tools version; verified locally: `swift package init` writes 6.4 and no `swiftLanguageModes`. | MUST | [Swift migration guide](https://www.swift.org/migration/documentation/migrationguide/) | missing | gate |
| `SWIFT-REPO-03` | Prefer one package with multiple targets over multi-package until a part has a separate release cadence; use local `.package(path:)` dependencies for a multi-package monorepo. [REPO-02] | SwiftPM has no workspace concept like Cargo/Go; targets give the boundary, packages give versioning. | SHOULD | [SwiftPM docs: Package.swift manifest](https://docs.swift.org/latest/documentation/packagemanagerdocs/) | missing | review |
| `SWIFT-REPO-04` | Bootstrap with `swift package init --type executable --name <Svc>` or the Hummingbird/Vapor templates (`hummingbird-project/template`, `vapor new`); then delete sample code, commit a `.swift-format` (or format the template once; the stock template fails `swift format lint --strict`), and create the four layer targets. [REPO-03] | Official starters carry the right tools version and `.gitignore`. | SHOULD | [Building for production (swift.org)](https://www.swift.org/documentation/server/guides/building.html), [Hummingbird template](https://github.com/hummingbird-project/template) | missing | review |
| `SWIFT-REPO-05` | Commit `Package.resolved` for services and executables; for libraries do not rely on it (consumers ignore it). In CI run `swift build --force-resolved-versions` (aliases `--disable-automatic-resolution`, `--only-use-versions-from-resolved-file`) so a stale lockfile fails the build. [REPO-04] | The flag "fail[s] resolution if it is out-of-date" (verified in `swift package resolve --help`). | MUST | [SwiftPM docs: resolving versions](https://docs.swift.org/latest/documentation/packagemanagerdocs/) | missing | gate |
| `SWIFT-REPO-06` | Declare dependencies with `from:` (semver up-to-next-major) for SSWG/Apple packages; use `exact:` or a revision only for forks or security pins and comment why. Never use `branch:` on a release. [REPO-04] | SwiftPM forbids unstable (branch) dependencies in tagged releases consumed by others and they break reproducibility. | MUST | [SwiftPM docs: Package.swift](https://docs.swift.org/latest/documentation/packagemanagerdocs/) | missing | review |
| `SWIFT-REPO-07` | Automate dependency updates with Dependabot (`package-ecosystem: "swift"`) or Renovate (`swift` manager); review `Package.resolved` diffs like code. Remove unused `.product(...)` entries; `--explicit-target-dependency-import-check=error` catches undeclared imports, not unused ones. [REPO-04] | Both bots read `Package.swift`/`Package.resolved`. | SHOULD | [Dependabot ecosystems](https://docs.github.com/en/code-security/dependabot/ecosystems-supported-by-dependabot/supported-ecosystems-and-repositories), [Renovate swift manager](https://docs.renovatebot.com/modules/manager/swift/) | missing | review |
| `SWIFT-REPO-08` | Release a library by pushing a SemVer git tag `X.Y.Z` (no `v` is conventional but `vX.Y.Z` also resolves); SwiftPM resolves from tags, no registry upload needed. Services release a container image tagged by git SHA + semver. Keep a CHANGELOG. [REPO-05] | Swift Package Index and SwiftPM read git tags; there is no central publish step (the Swift Package Registry spec, SE-0292, is optional). | SHOULD | [SE-0292 Package Registry Service](https://github.com/swiftlang/swift-evolution/blob/main/proposals/0292-package-registry-service.md), [Swift Package Index](https://swiftpackageindex.com/) | missing | review |
| `SWIFT-REPO-09` | Pin the toolchain with a `.swift-version` file (read by swiftly) and mirror it in the Docker base image tag and the CI matrix. [REPO-06] | swiftly supports `.swift-version` as a shared project setting. | MUST | [swiftly README](https://github.com/swiftlang/swiftly), [Install on Linux](https://www.swift.org/install/linux/) | missing | gate |
| `SWIFT-REPO-10` | Keep `.gitignore` with `/.build`, `/Packages`, `.swiftpm/xcode/`, `.swiftpm/configuration/registries.json`, `xcuserdata/`, `DerivedData/`, `.netrc`, `.DS_Store` (what `swift package init` emits); also ignore coverage output and `*.profraw`. Commit `.swift-format`, `.editorconfig`, `CODEOWNERS`. [REPO-07] | `.build/` can be GBs; `.netrc` holds credentials. Verified locally: init writes exactly this list. | MUST | [swift package init template (verified locally)](https://docs.swift.org/latest/documentation/packagemanagerdocs/) | missing | hook |
| `SWIFT-REPO-11` | CI stages: restore cache of `.build` keyed on `Package.resolved` + tools version, `swift format lint --strict -r Sources Tests`, SwiftLint, `swift build --force-resolved-versions -c release`, `swift test --sanitize=thread --enable-code-coverage`, export lcov, SBOM/audit, container build. Matrix: latest and previous Swift minor on Linux (Ubuntu), plus macOS only if you ship there. [REPO-08] | SSWG graduation requires CI on the two latest recommended Swift versions; a server targets Linux. | SHOULD | [SSWG incubation process](https://www.swift.org/sswg/incubation-process.html) | go-only (`WORKFLOW` §6 gates) | gate |
| `SWIFT-REPO-12` | Use conventional commits; they drive `loops release`. No Swift-specific commit rules. [REPO-09] | Ecosystem has no mandated changelog tool. | SHOULD | [Conventional Commits](https://www.conventionalcommits.org/) | missing | review |

## 2. TOOL: toolchain

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `SWIFT-TOOL-01` | Target Swift 6.4.x (latest stable, as of 2026-11); Swift releases roughly twice a year (6.2 Sep 2025, 6.4 2026-09-15) with no LTS; Hummingbird 2.27+ dropped 6.1 and needs Swift >= 6.2. Install via swiftly on Linux/macOS (`swiftly install latest`, `swiftly use`), or the official `swift:<ver>` Docker images. [TOOL-01] | Frameworks track the last two or three minors; no LTS means pin and upgrade deliberately. | MUST | [Swift 6.4 release](https://www.swift.org/blog/swift-6.4-released/), [swiftly](https://github.com/swiftlang/swiftly), [Hummingbird releases](https://github.com/hummingbird-project/hummingbird/releases) | missing | review |
| `SWIFT-TOOL-02` | Format with the toolchain-bundled `swift format` (swift-format, since Swift 6). Check argv: `swift format lint --strict --recursive Sources Tests`; fix: `swift format format --in-place --recursive Sources Tests`. Config: `.swift-format` (JSON, found by walking up); COMMIT a `.swift-format` (e.g. `{"version":1,"indentation":{"spaces":4}}` to match the `swift package init` template, or run `swift format format -i -r Sources Tests` once at bootstrap), because the fresh template (4-space indent) fails `lint --strict` against the default 2 spaces (verified by B2 on swift:6.4.2/Linux); dump defaults with `swift format dump-configuration`. [TOOL-02] | `--strict` turns warnings into a non-zero exit (lint prints to stderr, so `fails_on` must be exit code, not output). | MUST | [swift-format README](https://github.com/swiftlang/swift-format) | missing | gate |
| `SWIFT-TOOL-03` | Choose ONE style/lint stack: swift-format for layout (official, no SourceKit), plus SwiftLint for semantic rules (`swiftlint lint --strict`, `.swiftlint.yml`, `opt_in_rules`, `analyzer_rules` need a compiler log and are slow). Do not run both formatters in autofix mode; disable overlapping rules. [TOOL-03] | They overlap (line length, trailing commas) and fight each other when both rewrite. | SHOULD | [SwiftLint](https://github.com/realm/SwiftLint), [swift-format](https://github.com/swiftlang/swift-format) | missing | gate |
| `SWIFT-TOOL-04` | Type checking is the compiler: Swift 6 language mode (complete strict concurrency) and `-warnings-as-errors` in CI (`swiftSettings: [.treatAllWarnings(as: .error)]` or `swift build -Xswiftc -warnings-as-errors`). Enable upcoming features deliberately (`.enableUpcomingFeature("ExistentialAny")`, `"InternalImportsByDefault"`, `"MemberImportVisibility"`). [TOOL-04] | Swift 6 mode makes data-race diagnostics errors; warnings left as warnings rot. | MUST | [Swift migration guide](https://www.swift.org/migration/documentation/migrationguide/), [SwiftPM docs](https://docs.swift.org/latest/documentation/packagemanagerdocs/) | missing | gate |
| `SWIFT-TOOL-05` | Vulnerability/supply chain: there is NO first-party `swift audit`. Use GitHub Dependabot security alerts (Swift supported) and, in CI, `swift package generate-sbom` (SE-0509, Swift 6.4, SPDX or CycloneDX). OSV-Scanner does NOT support `Package.resolved` (verified against its supported-lockfiles page); Dependabot does support Swift. [TOOL-05] | Honest gap: loops cannot assume a vuln command exists; `vuln` check is `null`. | SHOULD | [Swift 6.4 release](https://www.swift.org/blog/swift-6.4-released/), [OSV-Scanner lockfiles](https://google.github.io/osv-scanner/supported-languages-and-lockfiles/) | missing | gate |
| `SWIFT-TOOL-06` | LSP is `sourcekit-lsp` (in the toolchain); use the Swift VS Code extension (it integrates with swiftly). Do not let editor format-on-save use a different formatter than CI (`swift format` vs SwiftLint `--fix`). Prebuilt `swift-syntax` for macros is on by default in 6.4 (`--enable-experimental-prebuilts`). [TOOL-06] | Editor/CI drift causes churn diffs. | SHOULD | [Swift 6.4 release](https://www.swift.org/blog/swift-6.4-released/), [sourcekit-lsp](https://github.com/swiftlang/sourcekit-lsp) | missing | review |
| `SWIFT-TOOL-07` | Swift Build is the default SwiftPM build system in 6.4 (`--build-system swiftbuild`; `native` is deprecated). Output lands under `.build/out/Products/Debug` on macOS and `.build/out/Products/Debug-linux-<arch>` on Linux (verified), not the old `.build/<triple>/debug` layout (only with `--build-system native`); always query paths with `swift build --show-bin-path` and `swift test --show-codecov-path`, never hardcode. [TOOL-01] | Layout changed between build systems and OSes. | MUST | [Swift 6.4 release](https://www.swift.org/blog/swift-6.4-released/), `swift test --help` (verified locally) | missing | gate |

## 3. ARCH: architecture

ARCH-02 mapping. SwiftPM target dependencies are the enforcement: a target can only `import` modules it declares (made a hard error by `--explicit-target-dependency-import-check=error`, default `none`), and cycles between targets are rejected by SwiftPM.

```
Package.swift
Sources/
  Domain/              # target "Domain": dependencies: []
    Order.swift  Money.swift  OrderError.swift  Clock.swift (protocol)
  Application/         # target "Application": dependencies: ["Domain"]
    PlaceOrder.swift  Ports/OrderRepository.swift  Ports/EventPublisher.swift
  PostgresAdapter/     # outbound: dependencies: ["Application","Domain", .product(name:"PostgresNIO")]
  HTTPAdapter/         # inbound: dependencies: ["Application","Domain", .product(name:"Hummingbird")]
  App/                 # executableTarget: dependencies: all of the above; @main, config, wiring
Tests/{Domain,Application,PostgresAdapter,HTTPAdapter,App}Tests/
```

Package.swift sketch: `.target(name:"Domain", swiftSettings:[.enableUpcomingFeature("ExistentialAny")])`, `.target(name:"Application", dependencies:["Domain"])`, `.target(name:"HTTPAdapter", dependencies:["Application", .product(name:"Hummingbird", package:"hummingbird")])`.

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `SWIFT-ARCH-01` | Choose hexagonal as the default: layers as SwiftPM targets (Domain, Application, adapters, App). Use a modular monolith (one package, many targets) first; split into services only on a deploy/ownership boundary. Hummingbird (composition via generics, explicit) suits it better than Vapor's global `Application` object, but both work. [ARCH-01] | Targets give compile-time boundaries for free. | SHOULD | [Hummingbird docs](https://docs.hummingbird.codes/2.0/documentation/hummingbird/), [SwiftPM](https://docs.swift.org/latest/documentation/packagemanagerdocs/) | covered (`HEXAGONAL_ARCHITECTURE` §1-§4) | review |
| `SWIFT-ARCH-02` | Model DDD tactical patterns with value types: entities as `struct` with an `id`, value objects as `struct` with validating `init throws`, aggregates as one root `struct`/`actor`-owned state, repositories as protocols in Application, domain events as an `enum`. [ARCH-01] | Value semantics make aggregates copy-safe and `Sendable` by default. | SHOULD | [Swift API Design Guidelines](https://www.swift.org/documentation/api-design-guidelines/), [The Swift Programming Language: Structures and Classes](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/classesandstructures/) | covered (`HEXAGONAL_ARCHITECTURE` domain events) | review |
| `SWIFT-ARCH-03` | CQRS/event sourcing and actors: use an `actor` for in-process single-owner state (a projection, a cache, an aggregate registry) and message-passing via `AsyncStream`; do not build a custom mailbox. For cross-process events use Kafka/NATS/SQS clients behind a port. [ARCH-01] | Swift actors give serialized access with compile-time checks; Swift has no OTP-style supervision tree. | SHOULD | [Swift Concurrency: actors](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/concurrency/) | covered (`HEXAGONAL_ARCHITECTURE` single owner of state) | review |
| `SWIFT-ARCH-04` | Serverless: Swift on AWS Lambda uses `swift-aws-lambda-runtime` with a static Linux SDK binary; keep handlers thin and reuse the same Application target. [ARCH-01] | Same hexagon, different inbound adapter. | SHOULD | [swift-aws-lambda-runtime](https://github.com/swift-server/swift-aws-lambda-runtime) | missing | review |
| `SWIFT-ARCH-05` | Module = target name = `Sources/<Target>` directory; `import Domain`. Access levels: `internal` (default) is module-private; use `public` only on the port/use-case API; `package` (SE-0386) shares between targets of one package without exposing it to consumers; `open` is for classes meant for subclassing (rare). Cycles between modules are impossible; cycles inside one module are allowed. [ARCH-03] | Access level is the second boundary after target dependencies. | MUST | [SE-0386 package access modifier](https://github.com/swiftlang/swift-evolution/blob/main/proposals/0386-package-access-modifier.md), [Access Control](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/accesscontrol/) | missing | arch |
| `SWIFT-ARCH-06` | Imports are `import Module`, `@testable import Module`, `internal import Module` (SE-0409, with `InternalImportsByDefault`), or `import struct Module.Type`; loops' arch parser must read the first token after `import` (skipping the kind keyword) and map it to a target by name. `@_exported import` is an AVOID. [ARCH-03] | Parsing needs the exact forms; `@_exported` leaks a dependency through the boundary. | MUST | [SE-0409 access-level modifiers on import](https://github.com/swiftlang/swift-evolution/blob/main/proposals/0409-access-level-on-imports.md) | missing | arch |
| `SWIFT-ARCH-07` | Enforce direction in `Package.swift` plus `swift build --explicit-target-dependency-import-check=error`; for rules beyond that (domain must not import Foundation networking, NIO, Vapor, Hummingbird, Fluent, PostgresNIO) use `loops arch` with a Swift parser, or a SwiftLint `custom_rules` regex on `Sources/Domain/**` as a stopgap. No ArchUnit equivalent is established. [ARCH-04] | Verified locally that the flag exists (values none/warn/error, default none). | MUST | `swift build --help` (verified locally), [SwiftLint custom_rules](https://github.com/realm/SwiftLint) | missing | arch |
| `SWIFT-ARCH-08` | Wire manually in the App target (`@main` creates adapters, injects them into use cases, builds the router). Avoid global singletons and service locators; DI containers (swift-dependencies, Factory) are optional and never inside Domain. Hummingbird passes dependencies via the `RequestContext`/router builder; Vapor via `app.storage`/`Request.application` (a locator; keep access to the adapter layer). [ARCH-05] | Explicit init injection is checked by the compiler. | SHOULD | [Hummingbird docs](https://docs.hummingbird.codes/2.0/documentation/hummingbird/), [Vapor docs](https://docs.vapor.codes/) | covered (`HEXAGONAL_ARCHITECTURE` DI) | review |
| `SWIFT-ARCH-09` | Keep Domain free of I/O, clock, randomness: inject a `Clock` (stdlib `Clock` protocol, `ContinuousClock`; `TestClock` from swift-clocks in tests), a `RandomNumberGenerator`, and ID generation; flag `Date()`, `Date.now`, `UUID()`, `.random(in:)`, `Task {`, `URLSession`, `FileManager`, `ProcessInfo` in Domain. [ARCH-06] | These make tests non-deterministic and hide dependencies. | MUST | [Clock (Swift stdlib)](https://developer.apple.com/documentation/swift/clock), [swift-clocks](https://github.com/pointfreeco/swift-clocks) | covered (`skills/consistency`, GO_BEST §13/§20) | arch |
| `SWIFT-ARCH-10` | Ports are `protocol`s defined in Application (consumer-owned), small (1-3 requirements), with `Sendable` and async throws methods; use `any Port` (with `ExistentialAny`) at wiring and generics (`some Port` / `<R: OrderRepository>`) on hot paths. [ARCH-07] | Protocol-oriented design; `any` costs a box and dynamic dispatch. | SHOULD | [Protocols](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/protocols/), [SE-0335 `any`](https://github.com/swiftlang/swift-evolution/blob/main/proposals/0335-existential-any.md) | covered (consumer-owned ports) | review |
| `SWIFT-ARCH-11` | SOLID limits: keep files under about 400 lines, functions under 50, a type under about 10 public members, and init/function parameters under 5 (use a parameter struct). Prefer composition over inheritance; classes only when identity or reference semantics are needed (`final class` by default). [ARCH-08] | Swift favours structs/protocols; deep class hierarchies are a smell. | SHOULD | [Swift API Design Guidelines](https://www.swift.org/documentation/api-design-guidelines/), SwiftLint `file_length`/`function_body_length` defaults | go-only (`SOLID_GO` §8) | arch |

## 4. IDIOM: what to do and what to avoid

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `SWIFT-IDIOM-01` | Use `throws` with typed domain error `enum`s conforming to `Error` (and `Sendable`); use typed throws (`throws(OrderError)`, SE-0413) at domain boundaries where the error set is closed. Wrap lower-level errors with context at the adapter; never `catch {}` silently and never `try?` where failure matters. [IDIOM-01] | Swift errors are unchecked; typed throws restore exhaustiveness. | MUST | [Error Handling](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/errorhandling/), [SE-0413 typed throws](https://github.com/swiftlang/swift-evolution/blob/main/proposals/0413-typed-throws.md) | covered (`HEXAGONAL_ARCHITECTURE` error taxonomy; GO_BEST §8-9) | review |
| `SWIFT-IDIOM-02` | Never use `try!`, `fatalError`, or `!` force-unwrap in server code paths driven by input; use `guard let`, `if let`, `??`, `try`. Reserve `precondition`/`fatalError` for programmer errors at startup. Swift crashes the whole process on a trap. [IDIOM-01, IDIOM-02] | A trap kills every in-flight request; there is no per-request recover. | MUST | [The Basics: Optionals](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/thebasics/), SwiftLint `force_unwrapping`, `force_try` | missing | gate |
| `SWIFT-IDIOM-03` | Model absence with `Optional`; do not use sentinel values (`-1`, `""`). Prefer `guard let` early exit; avoid implicitly unwrapped optionals (`T!`) except for framework-injected outlets. [IDIOM-02] | Optionals are compiler-checked. | MUST | [The Basics](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/thebasics/) | missing | review |
| `SWIFT-IDIOM-04` | Default to `struct`/`enum` and `let`; use `var` only when mutated; make types `Sendable` by value composition. `final class` with immutable stored `let` properties is also `Sendable`. [IDIOM-03] | Value semantics remove shared mutable state. | SHOULD | [Structures and Classes](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/classesandstructures/), [Sendable](https://developer.apple.com/documentation/swift/sendable) | missing | review |
| `SWIFT-IDIOM-05` | Model closed sets as `enum` with associated values and `switch` exhaustively (no `default:` on domain enums; use `@unknown default` only for non-frozen external enums); use `if case`/`switch` pattern matching. [IDIOM-04] | Adding a case becomes a compile error at every use. | MUST | [Enumerations](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/enumerations/) | missing | review |
| `SWIFT-IDIOM-06` | Resources: Swift has ARC, not GC pauses, but no `finally` for async; use `defer` (async calls inside `defer` run to completion in Swift 6.4, SE-0493), `withTaskCancellationShield` (SE-0504) for cleanup that must survive cancellation, and `~Copyable` types or `with...` closures for scoped resources (`withThrowingTaskGroup`, `PostgresClient.withConnection`). Break retain cycles with `[weak self]`/`unowned` in escaping closures. [IDIOM-05] | Leaked connections and retain cycles are the common leaks. | MUST | [Swift 6.4 release](https://www.swift.org/blog/swift-6.4-released/), [ARC](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/automaticreferencecounting/) | missing | review |
| `SWIFT-IDIOM-07` | Time: use `Date` only at the edge and `Duration`/`ContinuousClock` for intervals; format dates with `Date.FormatStyle`/`ISO8601FormatStyle` (explicit `TimeZone`/UTC), never `DateFormatter` shared across threads (not `Sendable`). Money is `Decimal`, never `Double`. Strings are Unicode: `count` is O(n), index via `String.Index`. [IDIOM-06] | Classic date/time and Unicode footguns. | MUST | [Date.FormatStyle](https://developer.apple.com/documentation/foundation/date/formatstyle), [Strings and Characters](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/stringsandcharacters/) | go-only (GO_BEST §13) | review |
| `SWIFT-IDIOM-08` | Follow the Swift API Design Guidelines: clarity at the point of use, `UpperCamelCase` types, `lowerCamelCase` members, argument labels that read as a phrase, mutating/non-mutating pairs (`sort()` / `sorted()`), no abbreviations; document public API with `///` DocC comments. [IDIOM-07] | The official guideline; SwiftLint/ swift-format enforce parts. | MUST | [API Design Guidelines](https://www.swift.org/documentation/api-design-guidelines/) | go-only (GO_BEST §26) | review |
| `SWIFT-IDIOM-09` | Avoid macros/property wrappers/result builders in the domain; they are fine at edges (Codable, Swift Testing `@Test`, Hummingbird/Vapor DSL, `@Observable` n/a on server). Avoid `Mirror`/`Any`/`AnyObject` type erasure and `@objc` dynamism on Linux. [IDIOM-08] | Macros hide behaviour and slow builds (swift-syntax). | AVOID | [Macros](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/macros/) | missing | review |
| `SWIFT-IDIOM-10` | Equality/hash: synthesize `Equatable`/`Hashable` from stored properties; do not hand-write `==` without `hash(into:)`; do not use `Float`/`Double` as dictionary keys; value-type `Codable` DTOs live in adapters, not Domain. [IDIOM-06] | Hand-written conformances drift apart. | SHOULD | [Equatable](https://developer.apple.com/documentation/swift/equatable), [Hashable](https://developer.apple.com/documentation/swift/hashable) | missing | review |
| `SWIFT-IDIOM-11` | Footgun avoidance: `try!`/`!` crashes; `Array` index out of range traps (use `.first`, `indices.contains`); integer overflow traps (use `addingReportingOverflow` or `&+` deliberately); `Dictionary` iteration order is random; capture lists in escaping closures; `Task {}` inherits actor context (use `Task.detached` knowingly). [IDIOM-09] | Traps terminate the server. | MUST | [Swift book: Advanced Operators (overflow)](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/advancedoperators/) | missing | review |

## 5. CONC: concurrency and the `-race` equivalent

**Loops race policy:** `mode = flag`. Compile-time: Swift 6 language mode (tools-version >= 6.0 or `swiftLanguageModes: [.v6]`, i.e. complete strict concurrency, data races are errors) AND at test time `swift test --sanitize=thread` (also `--sanitize thread`). Substitute: none needed. Do not combine with `address` (mutually exclusive); run ASan/UBSan as a separate optional job. Verified locally: `swift test --enable-code-coverage --sanitize=thread` ran together and produced coverage.

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `SWIFT-CONC-01` | Model: async/await on cooperative thread pool + structured concurrency (`async let`, `TaskGroup`) + actors; SwiftNIO `EventLoop`s underneath (Hummingbird/Vapor run handlers as `async`). No GIL. [CONC-01] | Swift concurrency is the default model; NIO `EventLoopFuture` is legacy. | MUST | [Concurrency (Swift book)](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/concurrency/), [SwiftNIO](https://github.com/apple/swift-nio) | missing | review |
| `SWIFT-CONC-02` | Compile every target in Swift 6 language mode: shared mutable state must be an `actor`, `Sendable`, `Mutex` (`Synchronization.Mutex`, Swift 6) or `nonisolated(unsafe)` with a written justification; passing non-`Sendable` values across isolation domains is a compile error. Never silence with `@unchecked Sendable` without a lock and a comment. [CONC-02] | The compiler proves data-race freedom for safe code. | MUST | [Migration guide](https://www.swift.org/migration/documentation/migrationguide/), [Sendable](https://developer.apple.com/documentation/swift/sendable), [Mutex](https://developer.apple.com/documentation/synchronization/mutex) | missing | gate |
| `SWIFT-CONC-03` | Run tests with `swift test --sanitize=thread` in CI and in the loops hard gate; it catches what the compiler cannot (`@unchecked Sendable`, `nonisolated(unsafe)`, C/ObjC interop, `Mutex` misuse). Linux needs the toolchain's TSan runtime (bundled in official images). Expect 5-15x slowdown, so scope to unit/integration tests. [CONC-03, CONC-04] | The Swift equivalent of `go test -race` is sanitizer build mode. | MUST | [swift test --sanitize (verified locally: `address, thread, undefined, scudo, fuzzer`)](https://docs.swift.org/latest/documentation/packagemanagerdocs/), [Thread Sanitizer](https://clang.llvm.org/docs/ThreadSanitizer.html) | go-only (`race-guard`, GO_BEST §11-12) | gate |
| `SWIFT-CONC-04` | Migrate incrementally: `swift package migrate` applies upcoming-feature fix-its per target; adopt `-strict-concurrency=complete` per target (`.enableUpcomingFeature("StrictConcurrency")`) before flipping to `.swiftLanguageMode(.v6)`. Never downgrade a target to `.v5` to pass a gate without logging an override. [CONC-03] | Prevents regressions during migration; matches the loops "every override is logged" rule. | SHOULD | [Migration guide](https://www.swift.org/migration/documentation/migrationguide/), `swift package migrate --help` (verified locally; the version that introduced it was not confirmed) | covered (overrides logged: `WORKFLOW` §1) | review |
| `SWIFT-CONC-05` | `@MainActor` is for UI; on the server avoid it. With "approachable concurrency" (`.enableUpcomingFeature("ApproachableConcurrency")`, written into `swift package init` output in 6.4) keep default isolation `nonisolated` for libraries/servers and do not set `defaultIsolation(MainActor.self)` in a server target. [CONC-01] | Main-actor default isolation would serialize handlers on one thread. | MUST | [Swift migration guide](https://www.swift.org/migration/documentation/migrationguide/), [SE-0466 default isolation](https://github.com/swiftlang/swift-evolution/blob/main/proposals/0466-control-default-actor-isolation.md) | missing | review |
| `SWIFT-CONC-06` | Cancellation is cooperative: call `try Task.checkCancellation()` or `Task.isCancelled` in loops, pass `async` work through structured tasks so cancelling a request cancels children; use `withTaskCancellationHandler` to wire cancellation to I/O; never `Task.detached` for request work. [CONC-05] | Unstructured tasks leak past the request and ignore cancellation. | MUST | [Concurrency: Tasks](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/concurrency/#Tasks-and-Task-Groups), [Swift 6.4 release](https://www.swift.org/blog/swift-6.4-released/) | go-only (GO_BEST §10) | review |
| `SWIFT-CONC-07` | Do not block the cooperative pool: no `sleep()`, `DispatchSemaphore.wait`, synchronous file/DB/`URLSession` data calls or heavy CPU in `async` code; use `try await Task.sleep(for:)`, async clients, and for CPU work use a dedicated `NIOThreadPool`/`NIOSingletons`. Do not mix `DispatchSemaphore` and `await`. [CONC-05] | The pool has about one thread per core; blocking starves all requests. | MUST | [SwiftNIO guidance](https://github.com/apple/swift-nio), [WWDC Swift concurrency behind the scenes](https://developer.apple.com/videos/play/wwdc2021/10254/) | missing | review |
| `SWIFT-CONC-08` | Bound concurrency and backpressure: `withThrowingTaskGroup` with a sliding window (add a task as one finishes), `AsyncStream` with `bufferingPolicy: .bufferingNewest(n)`, `swift-async-algorithms` for merge/debounce; set timeouts on every outbound call. [CONC-05] | Unbounded `addTask` loops exhaust memory/connections. | SHOULD | [swift-async-algorithms](https://github.com/apple/swift-async-algorithms), [AsyncStream](https://developer.apple.com/documentation/swift/asyncstream) | missing | review |
| `SWIFT-CONC-09` | Run background work and graceful shutdown with `swift-service-lifecycle` (`ServiceGroup`, `Service`, signal handling `SIGTERM`/`SIGINT`, `gracefulShutdown()`); register server, DB client, OTel and workers in one `ServiceGroup`. [CONC-06, OPS-04] | The SSWG standard for lifecycle; Hummingbird embeds it. | MUST | [swift-service-lifecycle](https://github.com/swift-server/swift-service-lifecycle) | missing | review |
| `SWIFT-CONC-10` | Swift Testing runs tests in parallel by default and `swift test --parallel` parallelizes XCTest; mark shared-resource suites `@Suite(.serialized)`; never depend on test order. [CONC-03] | Parallel runs surface hidden shared state. | SHOULD | [swift-testing](https://github.com/swiftlang/swift-testing) | missing | review |

## 6. TEST: testing

**Coverage recipe:** `swift test --enable-code-coverage --sanitize=thread` writes `default.profdata` plus a JSON summary; get the JSON path with `swift test --show-codecov-path` (the `codecov/` dir beside it holds `default.profdata`). Get `<bin>` from `swift build --show-bin-path`. Test binaries are OS-aware and must be DISCOVERED, never hardcoded: with the default Swift Build, macOS `<bin>` = `.build/out/Products/Debug` and the object is `<TestTarget>.xctest/Contents/MacOS/<TestTarget>` (verified locally); Linux `<bin>` = `.build/out/Products/Debug-linux-<arch>` and the objects are one `<TestTarget>.so` (or `<TestTarget>-test-runner`) per test target; there is NO `<Pkg>PackageTests.xctest` (verified by B2 on swift:6.4.2 aarch64; it exists only with the deprecated `--build-system native`, under `.build/<triple>/debug/`). Export: `llvm-cov export -format=lcov -instr-profile=<codecov dir>/default.profdata -ignore-filename-regex='(\.build\|Tests)/' <obj1> -object <obj2> ... > coverage.lcov` (macOS: `xcrun llvm-cov`; Linux: `llvm-cov` from the toolchain). New-code coverage: loops parses lcov `SF:`/`DA:` against the git base (line granularity; no branch data by default); `diff-cover` also accepts lcov.

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `SWIFT-TEST-01` | Use Swift Testing (`import Testing`, `@Test`, `@Suite`, `#expect`, `#require`) for all new tests; keep XCTest only for performance tests (`measure`), UI/Obj-C interop and frameworks that still ship XCTest helpers (Vapor's `XCTVapor`/`VaporTesting`, Hummingbird `HummingbirdTesting` works with both). Swift 6.4 lets `XCTAssert` run inside Swift Testing and `#expect` inside XCTest. [TEST-01] | swift-testing is bundled in the toolchain, parallel, and macro-based; mixing is supported. | SHOULD | [swift-testing](https://github.com/swiftlang/swift-testing), [Swift 6.4 release](https://www.swift.org/blog/swift-6.4-released/) | missing | review |
| `SWIFT-TEST-02` | Layout: `Tests/<Target>Tests/*.swift`; file `FooTests.swift` tests `Foo`; Swift Testing suites are `struct`s with `@Suite`; XCTest classes end in `Tests` and methods begin `test`. Test recognition globs: `Tests/**/*.swift`, `**/*Tests.swift`, `**/*Test.swift`. [TEST-02] | `testTarget` defaults to `Tests/<name>`. | MUST | [SwiftPM: test targets](https://docs.swift.org/latest/documentation/packagemanagerdocs/) | go-only (`*_test.go`) | gate |
| `SWIFT-TEST-03` | Write parameterized tests with `@Test(arguments: [...])` (the Swift Testing equivalent of table-driven tests); name with `@Test("does X when Y")`; arrange-act-assert; group with `.tags(...)` and run subsets with `swift test --filter`/`--skip`. [TEST-03] | Each argument is its own case in reports. | SHOULD | [swift-testing: parameterized tests](https://github.com/swiftlang/swift-testing) | go-only (table tests, `skills/testing`) | review |
| `SWIFT-TEST-04` | Test doubles: hand-written fakes/stubs implementing the port protocols (`InMemoryOrderRepository`, spy actors); no mocking library is standard (swift has no runtime reflection mocking; avoid macro mock generators unless already adopted). Do not mock the domain or value types. [TEST-04] | Protocol ports make fakes trivial. | SHOULD | [Protocols](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/protocols/) | go-only (`skills/testing`) | review |
| `SWIFT-TEST-05` | Integration: use real Postgres via Testcontainers (`testcontainers-swift` is community, immature) or docker-compose in CI; create an isolated schema/DB per test or suite; HTTP tests use `HummingbirdTesting` (`app.test(.router)` runs without sockets) or `VaporTesting`/XCTVapor `app.test(.GET, ...)`. [TEST-05] | In-process app testing avoids port races. | SHOULD | [Hummingbird testing](https://docs.hummingbird.codes/2.0/documentation/hummingbird/), [Vapor testing](https://docs.vapor.codes/) | missing | review |
| `SWIFT-TEST-06` | Coverage argv: `swift test --enable-code-coverage` (default is disabled), then `llvm-cov export -format=lcov` as above (see recipe). `--show-codecov-path` prints the JSON; derive profdata/binary locations rather than hardcoding. Exclude tests and dependencies with `-ignore-filename-regex='(\.build\|Tests\|Package\.swift)'` or lcov post-filter; exclude generated code by `*.pb.swift`. [TEST-06] | Paths differ per build system and OS; line-level only. | MUST | `swift test --help` (verified locally), [llvm-cov export](https://llvm.org/docs/CommandGuide/llvm-cov.html) | go-only (gocover adapter) | gate |
| `SWIFT-TEST-07` | New-code coverage >= 70%: loops parses the lcov file (`SF:<path>`, `DA:<line>,<hits>`, `end_of_record`), intersects with `git diff -U0 <base>` added lines in `Sources/**/*.swift` (excluding generated), and fails below 70%; `diff-cover coverage.lcov --compare-branch=<base> --fail-under=70` is the third-party equivalent. [TEST-07] | Same hard gate as Go, different parser. | MUST | [diff-cover](https://github.com/Bachmann1234/diff_cover), `llvm-cov export -format=lcov` (verified locally emits `SF:`, `FN:`, `DA:`) | go-only (`gate/coverage.go`, gocover) | gate |
| `SWIFT-TEST-08` | Property tests: `SwiftCheck` (community, stagnant) or parameterized `@Test` with seeded random inputs; Swift Testing has no built-in property mode. Fuzz: `--sanitize=fuzzer` exists for libFuzzer targets. Mutation: no mature server-side tool (Muter exists, slow); skip by default. [TEST-08] | Be honest about the ecosystem gap. | AVOID | [SwiftCheck](https://github.com/typelift/SwiftCheck), [Muter](https://github.com/muter-mutation-testing/muter) | go-only (fuzz in `skills/testing`) | review |
| `SWIFT-TEST-09` | Flakiness: inject `Clock`; avoid `Task.sleep` in tests (use `confirmation(expectedCount:)` or `AsyncStream` expectations); run `swift test --repeat-until fail --maximum-repetitions 20` on a suspect Swift Testing test (Swift 6.4, verified in `--help`); set `.timeLimit(.minutes(1))` traits. [TEST-09] | Swift 6.4 can repeat tests (ST-0024). | SHOULD | [Swift 6.4 release](https://www.swift.org/blog/swift-6.4-released/), `swift test --help` (verified locally) | go-only (`skills/testing`) | review |
| `SWIFT-TEST-10` | TDD loop: `swift test --filter 'DomainTests.OrderTests/testPlace'` (regex on `<target>.<case>/<test>`), `swift test --skip-build` after a build, `swift test list`/`-l` to list; no built-in watch mode (use `watchexec -e swift swift test --filter ...`). Incremental builds reuse `.build/`. [TEST-10] | Single-test selection is regex based. | SHOULD | [SwiftPM docs](https://docs.swift.org/latest/documentation/packagemanagerdocs/), `swift test --help` (verified locally) | missing | review |

## 7. PERF: performance and profiling

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `SWIFT-PERF-01` | Benchmark with `ordo-one/package-benchmark` (`swift package benchmark`, thresholds + baselines, mallocs/ARC counters); XCTest `measure {}` is a weak fallback; never time with `Date()` loops. Always benchmark `-c release`. [PERF-01] | package-benchmark is the de-facto statistical harness. | SHOULD | [package-benchmark](https://github.com/ordo-one/package-benchmark) | go-only (benchstat) | review |
| `SWIFT-PERF-02` | Profile with `perf` on Linux (`perf record -g .build/release/App`, flame graphs), Valgrind/heaptrack for memory, Instruments on macOS, and `swift-backtrace` for crashes (Linux has no default backtrace on older toolchains; Swift 5.9+ ships backtracing configurable via `SWIFT_BACKTRACE`). [PERF-02] | No pprof; Linux perf is standard. | SHOULD | [Server guide: building](https://www.swift.org/documentation/server/guides/building.html), [Swift Backtrace](https://www.swift.org/blog/swift-5.9-released/) | go-only (pprof) | review |
| `SWIFT-PERF-03` | Ship `-c release`; consider `-Xswiftc -cross-module-optimization` (verify with benchmarks) and `--static-swift-stdlib`; avoid existential `any` on hot paths, large struct copies in actors, repeated `String` concatenation, and `Array` copy-on-write surprises (`isKnownUniquelyReferenced`). ARC traffic dominates: look for retain/release in profiles. [PERF-03, PERF-04] | Debug builds are "significantly slower". | MUST | [Building for production](https://www.swift.org/documentation/server/guides/building.html) | missing | review |
| `SWIFT-PERF-04` | Use `Foundation` sparingly on Linux (`swift-foundation` is the new fast core); prefer `NIOCore.ByteBuffer`, `Span`/`InlineArray` (Swift 6.2+), and `JSONEncoder` reuse; avoid `String(format:)` and regex on hot paths; pool nothing by hand. [PERF-04] | Foundation on Linux historically lagged Darwin. | SHOULD | [swift-foundation](https://github.com/swiftlang/swift-foundation), [SwiftNIO ByteBuffer](https://github.com/apple/swift-nio) | missing | review |

## 8. SEC: security

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `SWIFT-SEC-01` | Never build SQL by string concatenation. PostgresNIO `PostgresQuery` supports string interpolation where interpolated values become bind parameters; in SQLKit use `\(bind: x)` and `\(ident: x)`; in Fluent use the typed query builder. Process execution: `Subprocess` (1.0 in Swift 6.4) with an argv array, never `sh -c` with user input. [SEC-01] | Interpolation is safe only through the library's `ExpressibleByStringInterpolation` type. | MUST | [PostgresNIO](https://github.com/vapor/postgres-nio), [SQLKit](https://github.com/vapor/sql-kit), [OWASP SQL Injection Prevention](https://cheatsheetseries.owasp.org/cheatsheets/SQL_Injection_Prevention_Cheat_Sheet.html) | go-only (GO_BEST §38) | review |
| `SWIFT-SEC-02` | Decode untrusted input with `Codable` into typed DTOs with size limits (`maxUploadSize`/`collectBody(upTo:)`); avoid `NSKeyedUnarchiver` and `NSCoding` with `requiresSecureCoding = false` (Darwin legacy); XML via `XMLParser` has external-entity risk: disable `shouldResolveExternalEntities`. [SEC-02] | Codable is schema-bound; keyed unarchive can instantiate arbitrary classes. | MUST | [OWASP Deserialization](https://cheatsheetseries.owasp.org/cheatsheets/Deserialization_Cheat_Sheet.html), [Hummingbird](https://docs.hummingbird.codes/2.0/documentation/hummingbird/) | missing | review |
| `SWIFT-SEC-03` | Web risks: validate redirect targets against an allowlist; resolve file paths with `URL.standardizedFileURL` and check prefix to stop traversal (Vapor/Hummingbird `FileMiddleware` rejects `..`); block SSRF to link-local/private ranges in AsyncHTTPClient callers; add CORS middleware with explicit origins (`CORSMiddleware`/Hummingbird `CORSMiddleware`) and CSRF protection for cookie sessions. [SEC-03] | Neither framework enables CORS/CSRF by default. | MUST | [OWASP SSRF](https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html), [Vapor docs](https://docs.vapor.codes/) | go-only (GO_BEST §38) | review |
| `SWIFT-SEC-04` | Crypto: use `swift-crypto` (`import Crypto`, CryptoKit API on Linux) for hashing/AEAD/HMAC; passwords via `Bcrypt` (Vapor) or Argon2 library, never raw SHA; randomness via `SystemRandomNumberGenerator` (cryptographically secure) or `SymmetricKey(size:)`; compare MACs with constant-time APIs; no secrets in source, `.env` committed, or logs. [SEC-04] | `Insecure.MD5/SHA1` exist for legacy only. | MUST | [swift-crypto](https://github.com/apple/swift-crypto), [OWASP Password Storage](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html) | missing | review |
| `SWIFT-SEC-05` | Validate at the adapter boundary: Vapor `Validatable` (`validations`), Hummingbird `Codable` + custom `init(from:)` or a `Validator`; convert to domain value objects whose `init throws` enforces invariants. [SEC-05] | Domain stays trusted. | MUST | [Vapor Validation](https://docs.vapor.codes/), [OWASP Input Validation](https://cheatsheetseries.owasp.org/cheatsheets/Input_Validation_Cheat_Sheet.html) | covered (`HEXAGONAL_ARCHITECTURE` validation at boundaries) | review |
| `SWIFT-SEC-06` | Supply chain: commit `Package.resolved`; SwiftPM checks fingerprints (`--resolver-fingerprint-checking strict` default) and warns on signing entity changes (`--resolver-signing-entity-checking warn`); pin by `exact`/revision for critical deps; review `Package.swift` of new deps and build plugins (they run at build time; SwiftPM sandboxes them, do not pass `--disable-sandbox`); never `swift package --allow-writing-to-package-directory` for untrusted plugins. [SEC-06] | Plugins and macros execute code at build time. | MUST | `swift test --help` (verified locally), [SwiftPM docs](https://docs.swift.org/latest/documentation/packagemanagerdocs/) | missing | review |
| `SWIFT-SEC-07` | SAST: no Swift-first tool is dominant. Use Semgrep (Swift supported), CodeQL (Swift on macOS runners), and SwiftLint security-adjacent rules; the compiler with Swift 6 mode and strict warnings is the main check. [SEC-07] | Open gap. | SHOULD | [Semgrep Swift](https://semgrep.dev/docs/supported-languages), [CodeQL Swift](https://codeql.github.com/docs/codeql-language-guides/codeql-for-swift/) | missing | gate |

## 9. DATA: persistence

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `SWIFT-DATA-01` | Choose by framework: Fluent ORM if you use Vapor and accept model-in-domain coupling; PostgresNIO (`PostgresClient`, async, pooled) or SQLKit for Hexagonal services where the repository adapter maps rows to domain types. Keep Fluent `Model` classes in the adapter, never in Domain. [DATA-01] | Fluent models are `final class` + property wrappers (non-`Sendable`-friendly). | SHOULD | [PostgresNIO](https://github.com/vapor/postgres-nio), [Fluent](https://github.com/vapor/fluent), [SQLKit](https://github.com/vapor/sql-kit) | covered (repository adapters: `HEXAGONAL_ARCHITECTURE` persistence) | review |
| `SWIFT-DATA-02` | Migrations: Fluent `AsyncMigration` (`prepare`/`revert`, registered with `app.migrations.add`; run `swift run App migrate`) or plain SQL migration files applied by Flyway/golang-migrate/`sqitch` in CI. Keep forward-only in production, expand-contract for zero downtime. [DATA-02] | Fluent migrations are Swift code tied to the service; SQL tools are language-neutral. | MUST | [Fluent migrations](https://docs.vapor.codes/), [PostgresNIO](https://github.com/vapor/postgres-nio) | covered (`skills/consistency` migrations) | review |
| `SWIFT-DATA-03` | Transactions at the application use-case boundary: `PostgresClient.withTransaction { }` / Fluent `database.transaction { }`; pass the transactional connection to repository ports via a unit-of-work port, not a global. Default isolation is `READ COMMITTED`; use `SERIALIZABLE` + retry for invariants. [DATA-03] | Transactions are closures, which suits scoped resources. | MUST | [PostgresNIO](https://github.com/vapor/postgres-nio), [Fluent transactions](https://docs.vapor.codes/) | covered (`HEXAGONAL_ARCHITECTURE` transactions) | review |
| `SWIFT-DATA-04` | Fluent traps: relations are lazy; use `.with(\.$children)` (eager load) to avoid N+1; `@Parent`/`@Children` fields not loaded throw at access; `Model` classes are not `Sendable`, so map to domain structs before crossing an actor. [DATA-04] | Fluent has no session/identity map; the N+1 risk is explicit loading. | MUST | [Fluent relations](https://docs.vapor.codes/) | missing | review |
| `SWIFT-DATA-05` | Connections: one shared `PostgresClient` (pool) per process, started inside the `ServiceGroup` (`await client.run()`), with `Configuration.options.connectTimeout`, pool min/max sized to core count, and statement timeouts; no per-request connection. [DATA-05] | PostgresNIO pools and applies backpressure. | MUST | [PostgresNIO](https://github.com/vapor/postgres-nio) | missing | review |

## 10. API: APIs and integration

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `SWIFT-API-01` | HTTP framework: Hummingbird 2 (small, async/await, modular, `Router`, `RouterMiddleware`, typed `RequestContext`) or Vapor 4 (batteries included, Fluent, Leaf). Vapor 5 is in beta (Beta 4, 2026-10) so build production on Vapor 4 or Hummingbird. Request lifecycle: NIO channel -> middleware chain -> route handler -> response. [API-01] | Check release status before choosing; Vapor 5 is not GA. | MUST | [Hummingbird releases](https://github.com/hummingbird-project/hummingbird/releases), [Vapor releases](https://github.com/vapor/vapor/releases) | missing | review |
| `SWIFT-API-02` | Contracts: OpenAPI-first with `swift-openapi-generator` (build plugin or committed generated code) plus `swift-openapi-hummingbird`/`-vapor` transports; gRPC with `grpc-swift-2` (async) and swift-protobuf; version via path prefix or content negotiation. Generated sources are not hand-edited. [API-02] | Typed server stubs from the spec. | SHOULD | [swift-openapi-generator](https://github.com/apple/swift-openapi-generator), [grpc-swift-2](https://github.com/grpc/grpc-swift-2) | missing | review |
| `SWIFT-API-03` | Map domain errors to RFC 9457 problem+json in one error middleware (Hummingbird `HTTPError`/`MiddlewareProtocol`; Vapor `AbortError`/`ErrorMiddleware`); never leak internal error text. [API-03] | Single mapping point at the adapter edge. | MUST | [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) | covered (`HEXAGONAL_ARCHITECTURE` error taxonomy) | review |
| `SWIFT-API-04` | Resilience: every outbound call has a timeout (AsyncHTTPClient `HTTPClient.Configuration.timeout`, `deadline:`), bounded retries with jittered backoff, idempotency keys on mutating endpoints, rate-limit middleware. No standard circuit-breaker library; write one as an actor. [API-04] | Defaults are lenient. | MUST | [async-http-client](https://github.com/swift-server/async-http-client) | covered (`skills/consistency` idempotency) | review |
| `SWIFT-API-05` | Messaging: Kafka via `swift-kafka-client`, NATS via `nats.swift`, AWS SQS/SNS via `aws-sdk-swift` or Soto; RabbitMQ via `rabbitmq-nio`; use the outbox pattern and idempotent consumers (at-least-once). [API-05] | Ecosystem is thin; keep clients behind ports. | SHOULD | [swift-kafka-client](https://github.com/swift-server/swift-kafka-client), [Soto](https://github.com/soto-project/soto) | covered (`skills/jobs-and-state-machines` outbox) | review |
| `SWIFT-API-06` | HTTP client: one long-lived `HTTPClient` (AsyncHTTPClient) or `URLSession` for the process, shut down via `shutdown()`/ServiceGroup; propagate cancellation by structured tasks and set deadlines. On Linux `URLSession` needs `import FoundationNetworking`. [API-06] | Creating a client per request leaks event loops; an un-shutdown `HTTPClient` crashes in debug. | MUST | [async-http-client](https://github.com/swift-server/async-http-client) | go-only (GO_BEST §10) | review |

## 11. OBS: observability

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `SWIFT-OBS-01` | Log with `swift-log` (`Logger`, `LoggingSystem.bootstrap` once in `main`), structured metadata (`logger[metadataKey: "request_id"]`, `logger.info("msg", metadata: [...])`), levels `trace...critical`; never log secrets or PII; do not use `print`. Hummingbird `LogRequestsMiddleware`. [OBS-01] | SSWG graduated API; every framework logs through it. | MUST | [swift-log](https://github.com/apple/swift-log) | go-only (GO_BEST §24) | review |
| `SWIFT-OBS-02` | Metrics via `swift-metrics` (`Counter`, `Timer`, `Gauge`; backends: swift-prometheus, OTel); name `snake_case` with unit suffix, low-cardinality labels; Hummingbird `MetricsMiddleware`. [OBS-02] | Same API-vs-backend split as swift-log. | SHOULD | [swift-metrics](https://github.com/apple/swift-metrics), [swift-prometheus](https://github.com/swift-server/swift-prometheus) | missing | review |
| `SWIFT-OBS-03` | Tracing via `swift-distributed-tracing` (`withSpan`, `ServiceContext` task-local propagation) with `swift-otel` as OTLP backend (`OTel.bootstrap()` returns a `Service`, run it in the `ServiceGroup`); use `TracingMiddleware`. swift-otel is a backend for the three Swift APIs, not a general OTel SDK; check its current stability. [OBS-03] | Task-local `ServiceContext` carries trace context across `await`. | SHOULD | [swift-distributed-tracing](https://github.com/apple/swift-distributed-tracing), [swift-otel](https://github.com/swift-otel/swift-otel) | missing | review |
| `SWIFT-OBS-04` | Expose `/health/live` (process up) and `/health/ready` (DB reachable) routes; Hummingbird has no built-in endpoint (add a route), Vapor likewise; wire to Kubernetes probes. [OBS-04] | Not provided by default. | SHOULD | [Hummingbird docs](https://docs.hummingbird.codes/2.0/documentation/hummingbird/) | missing | review |

## 12. OPS: configuration, packaging and deployment

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `SWIFT-OPS-01` | Config: 12-factor env vars into a typed `Sendable` struct at startup using `swift-configuration` (Apple, providers for env/JSON/reloading) or `swift-argument-parser` + `ProcessInfo`; Hummingbird has `Environment`. Fail fast on missing config; secrets from the environment/vault only. [OPS-01] | Typed config avoids stringly-typed lookups. | MUST | [swift-configuration](https://github.com/apple/swift-configuration), [swift-argument-parser](https://github.com/apple/swift-argument-parser) | covered (GO_BEST §28 config; hexagonal config boundary) | review |
| `SWIFT-OPS-02` | Package as a container: multi-stage Dockerfile, build stage `swift:6.4-noble` running `swift build -c release --static-swift-stdlib` (or `-Xswiftc -static-stdlib`), runtime stage `ubuntu:noble` or `swift:6.4-slim`; for fully static use the Static Linux SDK (`--swift-sdk x86_64-swift-linux-musl`). Copy `.build/release/<App>` found by `swift build -c release --show-bin-path`. Run as non-root. [OPS-02] | Official images on Docker Hub; Vapor's template Dockerfile uses this pattern. | SHOULD | [Docker Hub swift](https://hub.docker.com/_/swift), [Building for production](https://www.swift.org/documentation/server/guides/building.html) | missing | review |
| `SWIFT-OPS-03` | Native/AOT: Swift compiles ahead of time to a native binary already; no JIT/GraalVM question. Cross-compile with Swift SDKs (`swift sdk install ...`, Static Linux SDK). Embedded Swift is not for servers. [OPS-03] | n/a mostly. | SHOULD | [Static Linux SDK](https://www.swift.org/documentation/articles/static-linux-getting-started.html) | n/a | review |
| `SWIFT-OPS-04` | Graceful shutdown: `ServiceGroup` with `.sigterm`/`.sigint` handling, stop accepting new requests, drain in-flight tasks, close DB pool, flush OTel; Docker `STOPSIGNAL SIGTERM`, Kubernetes `terminationGracePeriodSeconds` > drain time. [OPS-04] | Task cancellation propagates only if structured. | MUST | [swift-service-lifecycle](https://github.com/swift-server/swift-service-lifecycle) | missing | review |
| `SWIFT-OPS-05` | Run-scoped caches: SwiftPM has no env var; use flags `--scratch-path <dir>` (default `.build`), `--cache-path`, `--config-path`, `--security-path` on every `swift build/test/package` call to relocate. Shared defaults live in `~/.swiftpm/{cache,configuration,security}` and `~/Library/Caches/org.swift.swiftpm` (macOS, verified locally), so keep `--cache-path` shared to avoid re-cloning dependencies and relocate only `--scratch-path`. [OPS-05] | `.build/` holds compiled output and `checkouts/`; the global cache holds git clones. | MUST | [SwiftPM docs](https://docs.swift.org/latest/documentation/packagemanagerdocs/), `swift test --help` (verified locally) | go-only (`GOCACHE`) | hook |
| `SWIFT-OPS-06` | Artifacts left behind: `.build/` (incl. `.build/out`, `checkouts`, `repositories`, `codecov/`), `.swiftpm/` (Xcode workspace state), `*.profraw`, `*.profdata`, `*.lcov`, `coverage.json`, `Package.resolved` is source (keep), `.netrc`. Clean with `swift package clean` (build artifacts) or `swift package reset` (also caches); `rm -rf .build` as last resort. [OPS-06] | Drives loops cleanup. | MUST | [SwiftPM docs](https://docs.swift.org/latest/documentation/packagemanagerdocs/), `swift package --help` (verified locally) | go-only (artifact globs) | hook |

## 13. DOC: documentation and review

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `SWIFT-DOC-01` | Document public API with `///` DocC markup (summary line, `- Parameter x:`, `- Returns:`, `- Throws:`); build with `swift package generate-documentation` (swift-docc-plugin) and host the archive. [DOC-01] | DocC is the official doc tool. | SHOULD | [DocC](https://www.swift.org/documentation/docc/), [swift-docc-plugin](https://github.com/swiftlang/swift-docc-plugin) | go-only (GO_BEST §26) | review |
| `SWIFT-DOC-02` | README with build/run/test commands (`swift build`, `swift test`), required Swift version (= `.swift-version`), ADRs in `docs/adr/`, CHANGELOG. [DOC-02] | Day-one clarity. | SHOULD | [Swift Evolution process as model](https://github.com/swiftlang/swift-evolution) | covered (`skills/documentation`, ADRs in hexagonal §) | review |
| `SWIFT-DOC-03` | Review focus: Sendable/isolation correctness and any `@unchecked Sendable`/`nonisolated(unsafe)`, force unwraps/`try!`, blocking calls in async, Task lifetime and cancellation, `[weak self]`, access control, target dependencies, public API naming. [DOC-03] | The most common Swift 6 defects. | MUST | [Migration guide](https://www.swift.org/migration/documentation/migrationguide/) | missing | review |
| `SWIFT-DOC-04` | Publish API docs/changelog per tag; keep generated OpenAPI spec committed or built in CI; update docs in the same PR as behaviour. [DOC-01] | Docs drift. | SHOULD | [swift-openapi-generator](https://github.com/apple/swift-openapi-generator) | covered (`skills/documentation`: update docs in the same change) | review |

## 14. FW: frameworks

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `SWIFT-FW-01` | Dominant frameworks: Vapor (largest community; 4.122.x stable, 5.0 beta as of 2026-10), Hummingbird 2.28.x (SSWG-adopted, lighter, requires Swift >= 6.2), plus raw SwiftNIO/`swift-openapi-*` for gateways. SSWG maturity: Sandbox (>=2 sponsors) -> Incubating (>=2 production users, supermajority vote) -> Graduated (>=3 production users, stable API with a major release, support for new Swift within 30 days, CI on two latest Swift). Use SwiftNIO, swift-log, swift-metrics, swift-distributed-tracing, swift-service-lifecycle, AsyncHTTPClient, swift-crypto first. [FW-01] | Prefer graduated packages. | SHOULD | [SSWG incubation](https://www.swift.org/sswg/incubation-process.html), [Hummingbird](https://github.com/hummingbird-project/hummingbird), [Vapor](https://github.com/vapor/vapor) | missing | review |
| `SWIFT-FW-02` | Vapor conventions: `Sources/App` + `Tests/AppTests`, `configure.swift` / `routes.swift`, Fluent `Model` classes, `Content` DTOs, `Application` singleton and `req.application` (service-locator style); put use cases in a separate target and keep controllers thin so Vapor stays an adapter. Hummingbird conventions: `buildApplication(...)` function returning an `ApplicationProtocol`, `Router` + `RequestContext` generic, test with `app.test(.router)`; this is closer to hexagonal. [FW-02] | Follow the framework where adopted; project rules win. | SHOULD | [Vapor docs](https://docs.vapor.codes/), [Hummingbird docs](https://docs.hummingbird.codes/2.0/documentation/hummingbird/) | covered (project rules win: `WORKFLOW` §1) | review |
| `SWIFT-FW-03` | Vapor 4 predates Swift 6 strict concurrency; enable Swift 6 mode for your own targets and expect `Sendable` friction at Fluent `Model` boundaries (map to DTOs). Hummingbird 2 is `Sendable`-first. [FW-03] | Real migration cost. | SHOULD | [Vapor releases](https://github.com/vapor/vapor/releases), [Hummingbird](https://github.com/hummingbird-project/hummingbird) | missing | review |
| `SWIFT-FW-04` | Framework footguns: calling blocking APIs on a NIO event loop; forgetting `app.shutdown()`/`asyncShutdown()` in tests (Vapor asserts); mutating `app.storage` after boot; Fluent model fields accessed before load; using `req.application.db` across requests with caches; mixing `EventLoopFuture` and `async` APIs. [FW-03] | Frequent forum issues. | AVOID | [Vapor docs](https://docs.vapor.codes/), [SwiftNIO](https://github.com/apple/swift-nio) | missing | review |

## Top 10 footguns

1. **Force unwrap / `try!` / array trap kills the process.** Bad: `let u = users[0]!`, `try! decode(...)`. Fix: `guard let`, `.first`, `try` with a mapped error. ([Swift book: The Basics](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/thebasics/))
2. **Blocking the cooperative thread pool.** Bad: `semaphore.wait()`, sync DB/file I/O in `async`. Fix: async clients, `Task.sleep(for:)`, `NIOThreadPool`. ([SwiftNIO](https://github.com/apple/swift-nio))
3. **Silencing Sendable errors.** Bad: `@unchecked Sendable`, `nonisolated(unsafe)` without a lock. Fix: `actor` or `Mutex`; TSan in CI. ([Migration guide](https://www.swift.org/migration/documentation/migrationguide/))
4. **Staying in Swift 5 mode.** Bad: `.swiftLanguageMode(.v5)` to get green. Fix: migrate per target with `swift package migrate`. ([Swift 6.4 release](https://www.swift.org/blog/swift-6.4-released/))
5. **Unstructured `Task {}` / `Task.detached` for request work.** Bad: fire-and-forget, loses cancellation, errors vanish. Fix: `async let`/`TaskGroup`, ServiceGroup for background. ([Concurrency](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/concurrency/))
6. **Retain cycles.** Bad: escaping closure capturing `self` strongly. Fix: `[weak self]`, struct/actor designs. ([ARC](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/automaticreferencecounting/))
7. **Debug build in production.** Bad: shipping `swift build`. Fix: `-c release`. ([Building for production](https://www.swift.org/documentation/server/guides/building.html))
8. **Stale `Package.resolved`/branch deps.** Bad: `branch: "main"`, uncommitted lockfile. Fix: `from:`, commit lockfile, `--force-resolved-versions`. ([SwiftPM docs](https://docs.swift.org/latest/documentation/packagemanagerdocs/))
9. **Shared `DateFormatter`/`JSONDecoder` config and non-UTC dates.** Bad: global `DateFormatter`. Fix: `FormatStyle`, explicit UTC. ([Date.FormatStyle](https://developer.apple.com/documentation/foundation/date/formatstyle))
10. **Domain importing framework.** Bad: `import Vapor`/`Fluent` in Domain target. Fix: target deps `[]`, `--explicit-target-dependency-import-check=error`, Fluent models in adapter. ([SwiftPM docs](https://docs.swift.org/latest/documentation/packagemanagerdocs/))

## 15. LOOPS: tooling data

Notes: schema extended with `coverage.produce` (post-test export step) and `cache_flags` because SwiftPM has no cache env vars; no `vuln` check exists (no first-party tool; the `sbom` check only emits an SPDX/CycloneDX document and never fails on vulnerabilities). `swift format lint --strict` signals failure by exit code and needs a committed `.swift-format` (the stock template fails the default config). All argv are from `swift test --help` / `swift package --help` / `swift build --help` (6.4.2, verified locally) or the swift-format and llvm-cov docs.

```json
{
  "id": "swift",
  "name": "Swift (server-side)",
  "versions": {"stable": "6.4.2", "lts": null, "as_of": "2026-11"},
  "detect": [{"file": "Package.swift"}],
  "source_globs": ["Sources/**/*.swift"],
  "test_globs": ["Tests/**/*.swift", "**/*Tests.swift"],
  "generated": {"globs": ["**/*.pb.swift", "**/*.grpc.swift", "**/GeneratedSources/**", ".build/**"], "header_regex": "^// DO NOT EDIT\\.$"},
  "checks": [
    {"name": "build", "kind": "build", "argv": ["swift", "build", "--force-resolved-versions", "--explicit-target-dependency-import-check", "error"], "required": true, "tool": "swift"},
    {"name": "swift-format", "kind": "format", "argv": ["swift", "format", "lint", "--strict", "--recursive", "Sources", "Tests"], "required": false, "tool": "swift-format", "fails_on": "exit"},
    {"name": "swiftlint", "kind": "lint", "argv": ["swiftlint", "lint", "--strict"], "required": false, "tool": "swiftlint", "install": "brew install swiftlint  (Linux: docker run ghcr.io/realm/swiftlint:latest or build from source)"},
    {"name": "test", "kind": "test", "argv": ["swift", "test", "--sanitize=thread", "--enable-code-coverage"], "required": true, "tool": "swift"},
    {"name": "sbom", "kind": "sbom", "argv": ["swift", "package", "generate-sbom"], "required": false, "tool": "swift"}
  ],
  "test_commands": [{"argv_prefix": ["swift", "test"]}],
  "race": {"mode": "flag", "flags": ["--sanitize=thread"], "env": null, "reason": "Swift 6 language mode makes data races compile errors (tools-version >= 6.0 or swiftLanguageModes [.v6]); ThreadSanitizer via SwiftPM catches the rest", "substitute": null},
  "coverage": {
    "format": "lcov",
    "path": "{coverage}",
    "granularity": "line",
    "produce": [
      {"argv": ["swift", "test", "--show-codecov-path"], "note": "prints the JSON path; default.profdata is in the same codecov/ directory"},
      {"argv": ["llvm-cov", "export", "-format=lcov", "-instr-profile={codecov_dir}/default.profdata", "-ignore-filename-regex=(\\.build|Tests)/", "{test_binary}"], "stdout_to": "{coverage}", "note": "macOS: prefix with xcrun. Discover test objects under <bin> = swift build --show-bin-path: Linux (default Swift Build) one <TestTarget>.so (or <TestTarget>-test-runner) per test target in .build/out/Products/Debug-linux-<arch>; macOS <TestTarget>.xctest/Contents/MacOS/<TestTarget>. Pass the first as {test_binary} and each further one as -object <path>. <Pkg>PackageTests.xctest exists only with --build-system native"}
    ]
  },
  "post_edit": {"format_file": ["swift", "format", "format", "--in-place", "{file}"], "lint_dir": ["swift", "format", "lint", "--strict", "--recursive", "{dir}"]},
  "cache_env": null,
  "cache_flags": {"--scratch-path": "build", "--cache-path": "shared-keep", "--config-path": "shared-keep", "--security-path": "shared-keep"},
  "artifacts": [".build/", ".swiftpm/", "*.profraw", "*.profdata", "*.lcov"],
  "arch": {
    "import_regex": "^\\s*(?:@\\w+\\s+)*(?:(?:public|package|internal|fileprivate|private)\\s+)?import\\s+(?:(?:typealias|struct|class|enum|protocol|let|var|func)\\s+)?([A-Za-z_][A-Za-z0-9_]*)",
    "resolver": "target-name",
    "native_tool": "swift build --explicit-target-dependency-import-check=error",
    "forbidden_in_domain": ["NIO", "NIOCore", "NIOHTTP1", "Vapor", "Fluent", "Hummingbird", "PostgresNIO", "SQLKit", "AsyncHTTPClient", "FoundationNetworking"],
    "impure_calls": ["Date()", "Date.now", "UUID()", ".random(", "Task {", "Task.detached", "URLSession", "FileManager", "ProcessInfo"]
  },
  "test_jobs": null,
  "config_files": [".swift-format", ".swiftlint.yml", ".swiftlint.yaml", ".editorconfig", ".swift-version"],
  "sources": {
    "checks": "https://docs.swift.org/latest/documentation/packagemanagerdocs/",
    "race": "https://clang.llvm.org/docs/ThreadSanitizer.html",
    "format": "https://github.com/swiftlang/swift-format",
    "lint": "https://github.com/realm/SwiftLint",
    "coverage": "https://llvm.org/docs/CommandGuide/llvm-cov.html",
    "sbom": "https://www.swift.org/blog/swift-6.4-released/",
    "cache_flags": "swift test --help (6.4.2)"
  }
}
```

## Gaps vs loops

| Rule ID | Gap | loops today | Priority | Where loops should fix it |
|---|---|---|---|---|
| `SWIFT-TEST-06` and `SWIFT-TEST-07` | lcov export is a two-step command; test objects differ per OS and must be discovered; no lcov parser | go-only | P0 | gate (lcov coverage adapter, discovery via `--show-codecov-path` and `--show-bin-path`) |
| `SWIFT-CONC-03` | Race policy is `--sanitize=thread` plus Swift 6 mode; `race-guard` knows only `go test -race` | go-only | P0 | hook `race-guard` (per-pack race flags), gate |
| `SWIFT-REPO-05` | No lockfile-freshness check (`--force-resolved-versions`) | missing | P0 | gate (build check argv) |
| `SWIFT-TOOL-05` | No first-party vuln scanner (OSV-Scanner lacks `Package.resolved`); only Dependabot and SBOM | missing | P0 | gate (`vuln` null), security skill |
| `SWIFT-TOOL-02` | `swift format lint --strict` fails by exit code and needs a committed `.swift-format` | missing | P0 | gate `fails_on`, `loops init` scaffolding |
| `SWIFT-TOOL-04` | Swift 6 mode and warnings-as-errors not enforced | missing | P1 | gate |
| `SWIFT-TOOL-07` | Build output paths vary by build system and OS | missing | P1 | gate (path discovery) |
| `SWIFT-TOOL-01` | Toolchain version and swiftly guidance | missing | P2 | pack reference |
| `SWIFT-TOOL-03` | swift-format plus SwiftLint stack; `.swift-format`/`.swiftlint.yml` unknown to practices-researcher | missing | P1 | `agents/practices-researcher.md`, gate |
| `SWIFT-TOOL-06` | Editor and LSP baseline | missing | P2 | pack reference |
| `SWIFT-ARCH-04` | Serverless guidance | missing | P2 | pack reference |
| `SWIFT-ARCH-05`, `SWIFT-ARCH-06` and `SWIFT-ARCH-07` | Arch parser for `import` forms; target-name resolver; native `--explicit-target-dependency-import-check` | missing | P1 | arch (new Swift parser adapter) |
| `SWIFT-ARCH-11` | Size and SOLID limits in Swift terms | go-only | P1 | arch limits per pack |
| `SWIFT-ARCH-09` | Domain impurity calls (`Date()`, `UUID()`, `Task {`) | covered | P1 | arch `impure_calls` |
| `SWIFT-OPS-05` and `SWIFT-OPS-06` | Cache via flags not env; `.build/` and `.swiftpm/` cleanup | go-only | P1 | runspace adapter, cleanup |
| `SWIFT-REPO-09` | Language detection (`Package.swift`) and `.swift-version` pin | missing | P1 | `loops init`, pack detect |
| `SWIFT-CONC-01` | Concurrency model guidance | missing | P2 | pack skill |
| `SWIFT-CONC-02`, `SWIFT-CONC-04` and `SWIFT-CONC-05` | Strict concurrency, `@unchecked Sendable` review, no `@MainActor` default | missing | P1 | pack skill, review checklist |
| `SWIFT-CONC-06` and `SWIFT-CONC-07` | Cancellation and blocking the cooperative pool | go-only | P1 | pack skill, review checklist |
| `SWIFT-CONC-08`, `SWIFT-CONC-09` and `SWIFT-CONC-10` | Backpressure, ServiceLifecycle, parallel tests | missing | P2 | pack skill |
| `SWIFT-IDIOM-02` and `SWIFT-IDIOM-11` | Trap-based failures (`!`, `try!`, overflow) | missing | P1 | gate (SwiftLint), skill |
| `SWIFT-IDIOM-03`, `SWIFT-IDIOM-04`, `SWIFT-IDIOM-05` and `SWIFT-IDIOM-06` | Optionals, value types, exhaustive enums, resources | missing | P2 | pack skill |
| `SWIFT-IDIOM-07` | Time, Decimal, Unicode | go-only | P2 | pack skill |
| `SWIFT-IDIOM-08`, `SWIFT-IDIOM-09` and `SWIFT-IDIOM-10` | API naming, metaprogramming, equality | missing | P2 | pack skill |
| `SWIFT-TEST-01`, `SWIFT-TEST-02`, `SWIFT-TEST-03`, `SWIFT-TEST-05`, `SWIFT-TEST-08` and `SWIFT-TEST-10` | Swift Testing vs XCTest, globs, parameterized, integration, property, loop speed | missing | P1 | skill `testing` per-language reference, gate globs |
| `SWIFT-TEST-04` and `SWIFT-TEST-09` | Fakes and determinism in Swift terms | go-only | P2 | skill `testing` |
| `SWIFT-DOC-01` and `SWIFT-DOC-03` | DocC and Swift review checklist | missing | P2 | review role, pack |
| `SWIFT-REPO-01`, `SWIFT-REPO-02`, `SWIFT-REPO-03`, `SWIFT-REPO-04`, `SWIFT-REPO-06`, `SWIFT-REPO-07`, `SWIFT-REPO-08`, `SWIFT-REPO-10`, `SWIFT-REPO-11` and `SWIFT-REPO-12` | Layout, tools version, bootstrap, pinning, bots, tags, hygiene, CI shape, commits | missing | P2 | spec REPO, skill |
| `SWIFT-SEC-*`, `SWIFT-DATA-04`, `SWIFT-DATA-05`, `SWIFT-API-*`, `SWIFT-OBS-*`, `SWIFT-OPS-02`, `SWIFT-OPS-04`, `SWIFT-PERF-*`, `SWIFT-FW-*` | Framework and ecosystem guidance | missing | P2 | pack reference files |

## Open questions

- **Coverage paths on Linux with Swift Build:** verified only on macOS 6.4.2 (`.build/out/Products/Debug/...`, test bundle `DemoTests.xctest`). Linux layout (`<Pkg>PackageTests.xctest` vs `.xctest` per target) needs a Linux spot run; loops should derive via `--show-bin-path`/`--show-codecov-path`.
- **Does `--show-codecov-path` return a bin path usable for lcov?** It only prints the JSON path; finding the test binary is loops' job. Maintainer decision: parse SwiftPM's JSON export instead of lcov (it contains per-file line segments but loops would need a new parser) vs lcov via `llvm-cov`.
- **Vulnerability scanning:** settled: OSV-Scanner does not support `Package.resolved`; Dependabot does. Decide whether loops adds a GitHub-advisory check or leaves `vuln` null.
- **SwiftLint on Linux** needs SourceKit (`LINUX_SOURCEKIT_LIB_PATH`) or the Docker image; whether loops should require it or treat it as optional.
- **Vapor 5** is in beta; `SWIFT-FW-*` must be re-checked when it ships.
- **TSan + Swift Testing:** works locally with `--sanitize=thread` and coverage; large suites may be slow or hit false positives with `@unchecked Sendable` C interop; unsettled how strict to be (suppression file via `TSAN_OPTIONS=suppressions=...`).
- **swift-otel stability and Foundation-on-Linux perf claims** were not confirmed from primary pages (README did not state a version); verify before codifying.
- Some rule sources are API-doc roots whose deep pages were not fetched (marked by root URL); B2 should spot-check them.

## Verification

Verifier: `B2-systems`, 2026-11-10.

- **Verdict:** `fail`, round 1. There are lint ERRs, 1 dead link (cited about 15 times), a wrong Linux coverage path in LOOPS, and a vuln-check contradiction. The source check passes (10 opened, 1 weak).
- **Completeness:** all 14 sections are present, plus the Top 10, the LOOPS block, Gaps, Open questions and Verification. Rule counts are REPO 12, TOOL 7, ARCH 11, IDIOM 11, CONC 10, TEST 10, PERF 4, SEC 7, DATA 5, API 6, OBS 4, OPS 6, DOC 4 and FW 4. Every minimum is met. The LOOPS JSON parses, with `race.mode=flag`. Lint reports "8 malformed rule rows" (finding 1).
- **Sources opened (10, 1 weak):**
  1. https://www.swift.org/blog/swift-6.4-released/: **yes**, with one exception. Confirmed: released 2026-09-15; Swift Build is the default; SBOM generation in SPDX or CycloneDX; async `defer`; `withTaskCancellationShield`; XCTAssert/`#expect` interop; Subprocess 1.0; test repetition. **Weak:** the page does not mention prebuilt swift-syntax (SWIFT-TOOL-06) or `swift package migrate`, though `migrate` exists in the 6.4.2 help.
  2. https://google.github.io/osv-scanner/supported-languages-and-lockfiles/: **yes**. Swift and `Package.resolved` are not listed, which settles that open question: OSV-Scanner does not support Swift.
  3. Dependabot supported ecosystems (docs.github.com): **yes**. `swift` covers v5 and v6 with security updates, and only git registries are supported.
  4. https://docs.renovatebot.com/modules/manager/swift/: **yes**, the page exists (200).
  5. https://github.com/swiftlang/swift-format: **yes**. `--strict` makes warnings exit non-zero; `-r` and `-i` exist; `.swift-format` is found by walking up the tree; it ships in the toolchain since Swift 6.
  6. https://github.com/swiftlang/swiftly: **yes**. It shares "the preferred toolchain as a project setting with a .swift-version file".
  7. https://www.swift.org/sswg/incubation-process.html: **yes**. Graduation needs CI on the two latest Swift versions, new GA support within 30 days, and 2/2/3 production users for the three levels.
  8. Hummingbird and Vapor releases (GitHub API): **yes**. Hummingbird 2.28.1 is from 2026-10-31. Vapor 4.122.3 and 5.0.0-beta.4 are from 2026-10-13.
  9. SE-0493, SE-0504 and SE-0466 proposal URLs: **yes**, all return 200. SE-0509 exists as `0509-swift-sboms-via-swiftpm.md`.
  10. https://www.swift.org/migration/documentation/migrationguide/: the page exists (200) but is a DocC single-page app, so its text can't be fetched. The tools-version-6 default language mode matches the spot run.
- **Tooling truth:**
  - `swift build --force-resolved-versions --explicit-target-dependency-import-check error`: both flags appear in `swift build --help`; exit 0.
  - `swift format lint --strict --recursive Sources Tests`: argv confirmed. It fails on the stock `swift package init` template (finding 6).
  - `swift test --sanitize=thread --enable-code-coverage`: confirmed on Linux aarch64, exit 0. Race `mode=flag` is correct.
  - `--show-codecov-path`: confirmed.
  - `coverage.produce[1]` `{test_binary}`: wrong on Linux (finding 3).
  - `swift package generate-sbom`: exists (`swift package --help`), but `kind: vuln` is wrong (finding 4).
  - `cache_flags` (`--scratch-path`, `--cache-path`, `--config-path`, `--security-path`): all appear in `swift test --help`.
  - Also confirmed in the help output: `--repeat-until`/`--maximum-repetitions`, `--filter`/`--skip`/`--skip-build`, `--parallel`/`--num-workers`, and `swift package clean`/`reset`/`migrate`.
  - `post_edit`: argv shape is valid.
- **Spot run (`docker run --rm swift:6.4.2`, Swift 6.4.2, aarch64-unknown-linux-gnu):**
  - `swift package init --type library` exits 0 and writes tools-version 6.4.
  - The build argv exits 0.
  - `swift format lint --strict --recursive Sources Tests` exits 1 with "[Indentation] unindent by 2 spaces" in the template's `Tests/HelloTests/HelloTests.swift`.
  - `swift test --sanitize=thread --enable-code-coverage` exits 0.
  - `swift test --show-codecov-path` prints `/tmp/Hello/.build/out/Products/Debug-linux-aarch64/codecov/Hello.json`, and `default.profdata` sits beside it.
  - `swift build --show-bin-path` prints `.build/out/Products/Debug-linux-aarch64`. That directory holds `HelloTests.so` and `HelloTests-test-runner`, and **no** `HelloPackageTests.xctest`.
  - `llvm-cov export -format=lcov ... HelloPackageTests.xctest` exits 1 (no such file). The same export against `HelloTests.so` or `HelloTests-test-runner` exits 0 and writes `SF:`/`DA:` lcov.
  - With `--build-system native` (which prints a deprecation warning), the path is `.build/aarch64-unknown-linux-gnu/debug/` and `HelloPackageTests.xctest` exists.
- **Findings:**
  1. **Lint.**
     - SWIFT-TEST-06 (line 143) has an unescaped `|` in `(\.build|Tests|Package\.swift)`. Fix: escape it as `\|`.
     - SWIFT-TEST-10, SWIFT-OPS-05 and SWIFT-OPS-06 cite only "`swift test --help` (verified locally)". Every rule needs a URL. Fix: add the SwiftPM docs URL from finding 2.
     - The other 4 ERR rows are Gaps rows (303, 304, 305, 309) whose first cell is a lone ID, which lint takes for a rule row. Fix: add text after the ID in that cell.
  2. **Link.** `https://docs.swift.org/swiftpm/documentation/packagemanagerdocs/` is 404: it 301-redirects to a broken `.../packagemanagerdocs/packagemanagerdocs/`. It is cited in REPO-01/03/05/06/10, ARCH-01, TOOL-04, CONC-03, TEST-02, SEC-06, the Top 10 (8 and 10) and LOOPS `sources.checks`. Fix: use https://docs.swift.org/latest/documentation/packagemanagerdocs/ (200), or https://www.swift.org/documentation/package-manager/.
  3. **Coverage recipe, LOOPS `coverage.produce[1]` and SWIFT-TOOL-07: Linux paths are wrong.**
     - With the default Swift Build on Linux, there is no `<Pkg>PackageTests.xctest`. The test binaries are `<bin-path>/<TestTarget>.so` or `<TestTarget>-test-runner`, one per test target.
     - The bin path is `.build/out/Products/Debug-linux-<arch>`, not `Debug`.
     - `<Pkg>PackageTests.xctest` exists only with the deprecated `--build-system native`.
     - Fix: on Linux, pass every `<TestTarget>.so` to `llvm-cov export` (the first as the object, the rest with `-object`). Note the arch-suffixed directory, and drop "(not verified locally)".
  4. **LOOPS `checks` sbom vs the notes.** The notes say "`vuln` check is omitted because no first-party tool exists", yet the `sbom` check has `kind: "vuln"`. `generate-sbom` only emits an SBOM and never fails on a vulnerability, so a gate would show a vuln pass that means nothing. Fix: give it a non-vuln kind (for example `sbom`) or drop it, and keep vuln `null`.
  5. **`loops today` values that disagree with the baseline.**
     - SWIFT-REPO-08 cites "`WORKFLOW` §release via tag", but WORKFLOW has no release section. Use `missing`.
     - SWIFT-REPO-12 cites "AGENTS.md releases", which is the plugin's contributor guide, not shipped guidance. Use `missing`.
     - SWIFT-DOC-04 cites "`WORKFLOW` docs in step", which is not in WORKFLOW. Cite `skills/documentation` instead ("update it in the same change").
     - SWIFT-TEST-04 and SWIFT-TEST-09 cite `skills/testing`, which the baseline marks go-only. Use `go-only`.
     - SWIFT-IDIOM-07 cites only GO_BEST §13, which is go-only. Use `go-only`.
  6. **SWIFT-TOOL-02 / SWIFT-REPO-04.** Code fresh from `swift package init` fails the format check: the template indents 4 spaces and swift-format's default is 2, so the check exits 1. Fix: require a committed `.swift-format` (or run `swift format format -i -r` once at bootstrap) and say so in the rule.
  7. **SWIFT-ARCH-04, Sev.** It is `AVOID`, but the rule is a positive instruction ("Swift on AWS Lambda uses swift-aws-lambda-runtime"). Fix: `SHOULD`.
  8. **Gaps table.** About 20 rules that aren't `covered` are missing from it: ARCH-04, ARCH-11, CONC-01, CONC-06 to CONC-10, DOC-01, DOC-03, IDIOM-03 to IDIOM-06, IDIOM-08 to IDIOM-10, TOOL-01, TOOL-04, TOOL-06 and TOOL-07. Fix: list each one with a priority. TOOL-04 (warnings as errors) and CONC-06/07 are P1.
  9. **SWIFT-CONC-04** says `swift package migrate` is "(Swift 6.4)". The 6.4 release post does not mention it; I believe it shipped in Swift 6.2, but I couldn't confirm that from a primary source. Fix: cite a source that names the version, or drop the version.

### Round 2

Verifier: `B2-systems`, 2026-11-10.

- **Verdict:** `pass`, round 2. Lint has no ERR (101 rules), links.sh reports no FAIL, the LOOPS block passes `jq`, and all 9 round-1 findings are resolved.
- **Round-1 findings:**
  1. **Resolved.**
     - The SWIFT-TEST-06 pipe is escaped.
     - SWIFT-TEST-10, SWIFT-OPS-05 and SWIFT-OPS-06 now cite the SwiftPM docs URL.
     - The Gaps rows no longer trip lint.
  2. **Resolved.** Every source now uses `https://docs.swift.org/latest/documentation/packagemanagerdocs/` (200), including LOOPS `sources.checks`. The old URL survives only in the round-1 verification text.
  3. **Resolved.**
     - The coverage recipe and the `coverage.produce[1]` note now say to discover test objects.
     - On Linux, that means one `<TestTarget>.so` or `-test-runner` per test target, in `.build/out/Products/Debug-linux-<arch>`.
     - On macOS, it means `<TestTarget>.xctest/Contents/MacOS/<TestTarget>`.
     - The first object is passed as the binary and the rest with `-object`.
     - `<Pkg>PackageTests.xctest` is described as native-only.
     - SWIFT-TOOL-07 gives both bin paths.
     - All of this matches the round-1 spot run.
  4. **Resolved.** The `sbom` check is now `kind: "sbom"`, and vuln stays `null`.
  5. **Resolved.**
     - SWIFT-REPO-08 and SWIFT-REPO-12 are `missing`.
     - SWIFT-DOC-04 cites `skills/documentation` ("update docs in the same change"), which that skill does say.
     - SWIFT-TEST-04 and SWIFT-TEST-09 are `go-only (skills/testing)`.
     - SWIFT-IDIOM-07 is `go-only (GO_BEST §13)`.
  6. **Resolved.** SWIFT-TOOL-02 and SWIFT-REPO-04 now require a committed `.swift-format` (4-space example) or a one-time format at bootstrap.
  7. **Resolved.** SWIFT-ARCH-04 is `SHOULD`.
  8. **Resolved.** The Gaps table now lists ARCH-04, ARCH-11, CONC-01, CONC-06 to CONC-10, DOC-01, DOC-03, IDIOM-03 to IDIOM-06, IDIOM-08 to IDIOM-10, TOOL-01, TOOL-04, TOOL-06 and TOOL-07.
  9. **Resolved.** SWIFT-CONC-04 no longer names a Swift version for `swift package migrate`; the source cell says the introducing version was not confirmed.
