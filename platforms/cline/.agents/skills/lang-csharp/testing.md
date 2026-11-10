# Testing (CS-TEST)

Universal testing rules: `specs/TESTING.md` (when present) and the `testing` skill. All gate runs follow the race policy in SKILL.md section 4 (WORKFLOW §6).

## Framework and layout
- xUnit v3 for new projects (MTP-native, use the `mtp` variant), xUnit v2 + `coverlet.collector` in existing ones; NUnit/MSTest when the repo already uses them; TUnit is MTP-only (CS-TEST-01). Unit, integration and architecture tests in separate projects.
- `<Project>.Tests`, `<Project>.IntegrationTests`, files `*Tests.cs`, mirror the source tree (CS-TEST-02). Method names `Method_Scenario_Expected`.
- Arrange-act-assert; `[Theory]` with `[InlineData]`/`[MemberData]` for tables; `IClassFixture`/`ICollectionFixture` for shared setup; no logic in tests (CS-TEST-03).

## Doubles and integration
- Hand-written fakes and in-memory ports first; NSubstitute (or Moq pinned) for interaction checks on ports you own; never mock `DbContext` or the BCL (use `HttpMessageHandler` fakes and `FakeTimeProvider`). Assertion library: xUnit asserts or Shouldly; FluentAssertions v8+ is commercial (CS-TEST-04).
- Integration: `WebApplicationFactory<Program>` (`public partial class Program {}`), a real database via Testcontainers, `Respawn` or rollback between tests. Never the EF InMemory provider (CS-TEST-05).

## Coverage (CS-TEST-06..09)
- Cobertura, line and branch; the gate globs `{coverage_dir}/**/*cobertura.xml`. VSTest: `coverlet.collector`. MTP: `Microsoft.Testing.Extensions.CodeCoverage`. Never one fixed `--coverage-output` for several projects.
- Exclude generated code and migrations (`ExcludeByFile`, `[ExcludeFromCodeCoverage]`).
- New code at least 70%, intersected with the diff by `loops gate`.

## Determinism and speed (CS-TEST-10/11/12)
- Inject `TimeProvider`, seeded `Random`; no `Thread.Sleep`; no shared static state; tests sharing a resource go in one `[Collection]`. xunit.v3 4.x: `[assembly: Parallelization(...)]`; older: `[assembly: CollectionBehavior(...)]`, `xunit.runner.json`.
- Tag concurrency tests `[Trait("Category","Concurrency")]` so `concurrency-repeat` finds them.
- Property tests (FsCheck, CsCheck) for parsers and invariants; Stryker.NET on Domain nightly.
- TDD loop: `dotnet test --no-build --filter "FullyQualifiedName~OrderTests"` (VSTest), `--filter-trait`/`--filter-class` after `--` for xunit.v3, `--minimum-expected-tests N` so an empty run fails.
