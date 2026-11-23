# Swift: architecture digest

Universal rules: `specs/ARCHITECTURE.md`, `specs/SOLID.md`. Swift rules: RULES.md ARCH.

## Layers are targets
- Hexagonal by default, as a modular monolith: Domain, Application, adapters, App (SWIFT-ARCH-01).
- A target can only `import` what its `dependencies:` declare; `--explicit-target-dependency-import-check error` makes an undeclared import fail the build; SwiftPM rejects target cycles (SWIFT-ARCH-07).
- Domain: `dependencies: []`. Application: `["Domain"]`. Adapters: Application, Domain and one framework product. App wires everything.

## Access levels are the second boundary
- `internal` is module-private (default); `public` only on port and use-case API; `package` (SE-0386) shares inside one package without exposing it; `open` is rare (SWIFT-ARCH-05).
- Import forms the arch parser reads: `import M`, `@testable import M`, `internal import M`, `import struct M.T`. `@_exported import` leaks a dependency: avoid (SWIFT-ARCH-06).

## Domain and ports
- Entities are `struct`s with an `id`; value objects are structs with `init throws`; events are enums; repositories are protocols in Application (SWIFT-ARCH-02).
- Ports: `protocol`s in Application, 1-3 requirements, `Sendable`, `async throws`. `any Port` at wiring, generics on hot paths (SWIFT-ARCH-10).
- Purity: inject `Clock`, a `RandomNumberGenerator` and ID generation. Domain must not use `Date()`, `UUID()`, `.random(`, `Task {`, `URLSession`, `FileManager`, `ProcessInfo` (SWIFT-ARCH-09). The arch scanner flags those and imports of NIO, Vapor, Fluent, Hummingbird, PostgresNIO, SQLKit, AsyncHTTPClient.

## State and wiring
- Single-owner in-process state is an `actor` plus `AsyncStream`, not a hand-built mailbox (SWIFT-ARCH-03).
- Wire manually in the App target; no singletons or service locators; DI containers never inside Domain (SWIFT-ARCH-08).
- Serverless: swift-aws-lambda-runtime as another inbound adapter over the same Application target (SWIFT-ARCH-04).

## Size limits
Files under about 400 lines, functions under 50, a type under about 10 public members, fewer than 5 parameters; composition over inheritance, `final class` by default (SWIFT-ARCH-11; `specs/SOLID.md` limits).
