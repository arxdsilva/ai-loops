---
name: lang-csharp
description: Use when writing, reviewing, testing or setting up C# (.NET) backend code in a loops run (the csharp language pack) - toolchain, layout, race policy, coverage, idioms and footguns.
---

# C# (.NET) backend

Rules: `specs/languages/csharp/RULES.md` (cite by ID, `CS-CONC-05`). Pack data: `specs/languages/csharp/pack.json` (`loops lang show csharp`). Universal rules: `specs/ARCHITECTURE.md`, `specs/SOLID.md`. Written for .NET 10 LTS and C# 14, as of 2026-11.

## 1. When it applies

`loops lang detect` reports `csharp` (a `global.json`, `*.slnx`, `*.sln`, `*.csproj` or `Directory.Build.props` at the root). If `global.json` names `Microsoft.Testing.Platform` the **`mtp`** variant is active; check which with `loops lang detect`.

## 2. Project rules first

The repo's own config wins over the pack and over this skill: `.editorconfig` and analyzer severities, `Directory.Build.props`, `global.json`, the test framework in use (xUnit, NUnit, MSTest, TUnit), `coverlet.runsettings`, `nuget.config`. Read them first (`pack.json` `config_files`). Every deviation: `loops run override --rule "<loops rule>" --project "<project rule, file:line>" --note "<what you did>"`.

## 3. Toolchain the gate runs

`loops gate` runs these in order; agents run single checks with `loops gate --only <check>` while iterating, never the whole suite (WORKFLOW §6).

| Check | Command | Notes |
|---|---|---|
| `restore` | `dotnet restore --locked-mode` | only when `packages.lock.json` is committed (CS-REPO-07) |
| `build` | `dotnet build -warnaserror -c Release` | analyzers and nullable run here; warnings are errors (CS-TOOL-03, CS-TOOL-05) |
| `format` | `dotnet format --verify-no-changes --no-restore` | required when `.editorconfig` is committed |
| `arch` | `loops arch` | `using` lines plus the `<ProjectReference>` graph (CS-ARCH-02) |
| `test` | `dotnet test --no-build -c Release` + coverage args | VSTest base; `mtp` variant adds `--minimum-expected-tests 1` |
| `concurrency-repeat` | `dotnet test --no-build -c Release --filter "Category=Concurrency"` x3 | race substitute (section 4) |
| `coverage` | internal | Cobertura, new code at least 70% (CS-TEST-09) |
| `vuln` | `dotnet package list --vulnerable --include-transitive --format json` | off by default; `--vuln`; exits 0 even when vulnerable, so the gate matches the JSON (CS-TOOL-06) |

Per-file after an edit: `dotnet format --no-restore --include <file>`. The SDK is pinned in `global.json` (CS-REPO-11); never bump it inside a feature lane. Use `-maxcpucount:2` (`test_jobs`) when several agents build at once.

Environment: pack `env` sets `MSBUILDDISABLENODEREUSE=1`, `DOTNET_NOLOGO=1`, `DOTNET_CLI_TELEMETRY_OPTOUT=1`. The run scopes `DOTNET_CLI_HOME` and `TMPDIR` only. `NUGET_PACKAGES` and the NuGet HTTP cache stay shared (CS-OPS-05). Never set them yourself. `bin/`, `obj/`, `TestResults/`, `*.trx`, `*.binlog` and Cobertura files are cleanup artifacts, not deliverables (CS-OPS-06).

## 4. Race policy

.NET has no race detector. The pack is `race.mode = none` with required substitutes (WORKFLOW §6, CS-CONC-04):
1. `test` runs with xUnit parallel collections **on**. Never set `parallelizeTestCollections=false` or serialise the assembly to hide a failure.
2. `concurrency-repeat` runs tests tagged `[Trait("Category","Concurrency")]` three times. Tag every test that exercises shared state, `Task.WhenAll`, `Channel<T>`, locks or background services, and write them to loop (`Parallel.For`, many tasks).
3. `build` treats the concurrency analyzers as errors: set `dotnet_diagnostic.CA2007/CA2012/CA2016.severity = error` (CA2007 only in library projects), `VSTHRD*` if the repo uses `Microsoft.VisualStudio.Threading.Analyzers`.
Optional: Microsoft Coyote for systematic interleavings. A retry attribute or `--blame-hang` loop that hides a flaky test is not a fix. Zero tagged tests passes the check, so concurrent code without a tagged test is a review finding.

## 5. Coverage recipe

Cobertura, line and branch. The gate reads `{coverage_dir}/**/*cobertura.xml` (`**` matches zero directories, so both layouts work):
- **VSTest (base, xUnit v2, NUnit, MSTest):** needs `coverlet.collector` in every test project. The gate adds `--collect:"XPlat Code Coverage" --results-directory {coverage_dir}`; coverlet writes `<guid>/coverage.cobertura.xml`.
- **`mtp` variant (xunit.v3, TUnit, `global.json` runner):** needs `Microsoft.Testing.Extensions.CodeCoverage` in every test project. The gate adds `--results-directory {coverage_dir} -- --coverage --coverage-output-format cobertura`; each project writes `<guid>.cobertura.xml`. Never pass a fixed `--coverage-output` to several projects (CS-TEST-07). `--collect:"XPlat Code Coverage"` fails under MTP.

Wave 0 prerequisites, owned by the architect and `csharp-engineer` (D13, logged as an override):
- the coverage package above in each test project (via `Directory.Packages.props`, CS-REPO-05);
- `global.json` with the SDK pin and, for MTP, `"test": {"runner": "Microsoft.Testing.Platform"}`;
- `Directory.Build.props` with `Nullable`, `TreatWarningsAsErrors`, `AnalysisLevel`, `EnforceCodeStyleInBuild`;
- `.editorconfig` (it also turns the `format` check on), `Directory.Packages.props`, `nuget.config` with source mapping;
- `ExcludeByFile` for `**/Migrations/*.cs` (coverlet) when migrations exist (CS-TEST-08);
- a test project per layer plus `Program` visible to tests when `WebApplicationFactory` is used (CS-TEST-05).

## 6. Loops layers in .NET

Assembly references are the boundary (CS-ARCH-01). One project per layer:

```
src/Shop.Domain/          pure rules, no PackageReference, no ProjectReference
src/Shop.Application/     use cases + ports (interfaces)        -> Domain
src/Shop.Infrastructure/  EF Core, HTTP clients (adapters)      -> Application
src/Shop.Api/             composition root (Program.cs), inbound -> Application, Infrastructure
tests/Shop.<Layer>.Tests/ one test project per layer (+ IntegrationTests, Architecture.Tests)
```

Map: domain-engineer owns `Shop.Domain`, application-engineer `Shop.Application`, persistence/integration engineers `Shop.Infrastructure`, api-engineer `Shop.Api`. `loops arch` fails on a project reference or `using` that points outward, and on impure calls (`DateTime.UtcNow`, `Guid.NewGuid`, `Task.Run`) in the domain: inject `TimeProvider` and an id generator (CS-ARCH-07).

## 7. Footguns

1. `.Result` / `.Wait()` / `async void`: await all the way up (CS-CONC-05).
2. `new HttpClient()` per call: `IHttpClientFactory` (CS-API-06).
3. EF N+1, tracked reads: `Include`/projection, `AsNoTracking()` (CS-DATA-04).
4. `DbContext` in a singleton or shared across tasks: scoped, or `IDbContextFactory<T>` (CS-DATA-05).
5. Captive dependency: `ValidateScopes`/`ValidateOnBuild` (CS-ARCH-05).
6. `!` and suppressed nullable warnings: fix the annotation (CS-IDIOM-03).
7. `throw ex;` / blanket `catch`: `throw;`, specific types (CS-IDIOM-01).
8. `DateTime.Now`, culture-sensitive strings: `TimeProvider`, `StringComparison.Ordinal` (CS-IDIOM-07, CS-IDIOM-08).
9. `FromSqlRaw` with concatenation, `BinaryFormatter`: `FromSql`, `System.Text.Json` (CS-SEC-01, CS-SEC-02).
10. Missing `CancellationToken` and unbounded `Task.Run` fire-and-forget: pass the token, use `BackgroundService` (CS-CONC-07, CS-CONC-08).

## 8. Load on demand

- `repo.md`: solution layout, `global.json`, central package management, lock files, CI stages (CS-REPO, CS-TOOL).
- `architecture.md`: hexagon as projects, ports, DI composition, arch tests (CS-ARCH).
- `idioms.md`: nullability, records, exceptions, disposal, strings, time, LINQ (CS-IDIOM).
- `testing.md`: xUnit, fakes, integration tests, coverage, determinism (CS-TEST).
- `concurrency.md`: async rules, locking, cancellation, background work (CS-CONC).
- `security.md`: injection, deserialisation, secrets, crypto, auth, supply chain (CS-SEC).
- `data-api.md`: EF Core, migrations, minimal APIs, resilience, logging, health, config, packaging (CS-DATA, CS-API, CS-OBS, CS-OPS).
- `frameworks.md`: ASP.NET Core, EF Core, generic host, Aspire, framework footguns (CS-FW, CS-PERF, CS-DOC).
