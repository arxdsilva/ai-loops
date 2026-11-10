# Swift: idioms digest

Rules in RULES.md IDIOM.

## Errors and absence
- Typed domain error enums conforming to `Error` and `Sendable`; typed throws (`throws(OrderError)`) where the error set is closed. Wrap lower errors with context at the adapter. Never `catch {}` silently or `try?` where failure matters (SWIFT-IDIOM-01).
- No `try!`, `fatalError` or `!` on input-driven paths: a trap kills every in-flight request. Reserve `precondition` for startup programmer errors (SWIFT-IDIOM-02).
- Absence is `Optional`, never a sentinel; `guard let` early exit; avoid `T!` (SWIFT-IDIOM-03).
- Array indexing and integer overflow trap: use `.first`, `indices.contains`, `addingReportingOverflow` (SWIFT-IDIOM-11).

## Types
- Default to `struct`, `enum` and `let`; `final class` with immutable `let` properties is `Sendable` too (SWIFT-IDIOM-04).
- Closed sets are enums with associated values, switched exhaustively with no `default:`; `@unknown default` only for external non-frozen enums (SWIFT-IDIOM-05).
- Synthesize `Equatable`/`Hashable`; never write `==` without `hash(into:)`; no floats as dictionary keys; `Codable` DTOs live in adapters (SWIFT-IDIOM-10).
- Keep macros, property wrappers and result builders out of Domain; avoid `Mirror`, `Any`, `@objc` on Linux (SWIFT-IDIOM-09).

## Resources, time, text
- `defer` (async calls allowed in 6.4, SE-0493), `withTaskCancellationShield` for cleanup that must survive cancellation, `~Copyable` or `with…` closures for scoped resources; break cycles with `[weak self]` (SWIFT-IDIOM-06).
- `Date` only at the edge, `Duration`/`ContinuousClock` for intervals, `Date.FormatStyle` with explicit UTC, never a shared `DateFormatter`. Money is `Decimal`. `String.count` is O(n) (SWIFT-IDIOM-07).

## Style
Swift API Design Guidelines: clarity at the call site, `UpperCamelCase` types, labels that read as phrases, `sort()`/`sorted()` pairs, `///` DocC on public API (SWIFT-IDIOM-08, SWIFT-DOC-01).
