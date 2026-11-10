# Swift: testing digest

Universal testing rules: `specs/TESTING.md` when present, else `loops:testing`. Rules in RULES.md TEST.

## Frameworks and layout
- Swift Testing (`import Testing`, `@Test`, `@Suite`, `#expect`, `#require`) for new tests. XCTest only for `measure` performance tests, Obj-C interop and framework helpers (Vapor `VaporTesting`); Swift 6.4 lets `XCTAssert` and `#expect` interoperate (SWIFT-TEST-01).
- `Tests/<Target>Tests/*.swift`; `FooTests.swift` tests `Foo`; suites are `struct`s (SWIFT-TEST-02).
- Parameterized `@Test(arguments: [...])` replaces table tests; name with `@Test("does X when Y")`; group with `.tags` (SWIFT-TEST-03).
- Doubles are hand-written fakes and spy actors behind the port protocols; no mock library by default; never mock Domain (SWIFT-TEST-04).
- Integration: real Postgres via docker-compose or Testcontainers, an isolated schema per suite; HTTP via `HummingbirdTesting` `app.test(.router)` or `VaporTesting`, no sockets (SWIFT-TEST-05).

## Coverage (gate: new code at least 70%)
1. `swift test --sanitize=thread --enable-code-coverage` (the gate's `test` check; both flags work together).
2. `swift test --show-codecov-path` prints `<bin>/codecov/<Pkg>.json`; `default.profdata` sits beside it.
3. `swift build --show-bin-path` gives `<bin>`: `.build/out/Products/Debug` (macOS) or `.build/out/Products/Debug-linux-<arch>` (Linux, default Swift Build).
4. Test objects, one per test target: macOS `<T>.xctest/Contents/MacOS/<T>`; Linux `<T>-test-runner` or `<T>.so`. There is no `<Pkg>PackageTests.xctest` except with `--build-system native`.
5. `llvm-cov export -format=lcov -instr-profile=<codecov>/default.profdata -ignore-filename-regex='(/checkouts/|\.build/|/Tests/|/DerivedSources/)' -object <o1> -object <o2> …`, with `xcrun` on macOS. A second positional object is read as a source filter and silently drops that target (SWIFT-TEST-06).
6. loops parses lcov `SF:`/`DA:` against the git base and fails under 70% on added `Sources/**/*.swift` lines (SWIFT-TEST-07). Line granularity only; no branch data by default.

The gate does steps 2-5 through the pack's placeholders. Run them by hand only to debug.

## Flakiness and TDD loop
- Inject `Clock`; avoid `Task.sleep` in tests (use `confirmation(expectedCount:)`); `.timeLimit(.minutes(1))` traits; `swift test --repeat-until fail --maximum-repetitions 20` on a suspect test (SWIFT-TEST-09).
- `swift test --filter 'DomainTests.OrderTests/testPlace'`, `--skip-build`, `swift test list`; no built-in watch (`watchexec -e swift swift test --filter …`) (SWIFT-TEST-10).
- Property tests: seeded parameterized `@Test` (SwiftCheck is stagnant); mutation testing is skipped by default (SWIFT-TEST-08). Benchmarks: package-benchmark, release builds only (SWIFT-PERF-01).
