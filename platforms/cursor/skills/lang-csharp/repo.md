# Repo and toolchain (CS-REPO, CS-TOOL)

Digest of `specs/languages/csharp/RULES.md` REPO and TOOL. The architect and `csharp-engineer` own these files in wave 0; implementers do not edit them.

## Day-one files (CS-REPO-04)
`global.json`, `Directory.Build.props`, `Directory.Packages.props`, `.editorconfig`, `.gitignore` (`dotnet new gitignore`), `nuget.config`, LICENSE, a CI workflow running restore, format, build, test, vuln.

```
.
├── global.json  Directory.Build.props  Directory.Build.targets  Directory.Packages.props
├── nuget.config  .editorconfig  .gitattributes  Shop.slnx
├── src/    Shop.Domain  Shop.Application  Shop.Infrastructure  Shop.Api
├── tests/  Shop.Domain.Tests  Shop.Api.IntegrationTests  Shop.Architecture.Tests
└── benchmarks/  docs/adr/
```

- **Layout (CS-REPO-01/02/03):** `src/`, `tests/`, one project per layer, one `.slnx` (or `.sln`) at the root; `dotnet build/test` fail on a folder with several solutions. Bootstrap with `dotnet new` templates. For xUnit v3 run `dotnet new install xunit.v3.templates` first; `xunit3` projects are MTP-native.
- **SDK pin (CS-REPO-11):** `{"sdk":{"version":"10.0.100","rollForward":"latestFeature"}}`, no wildcards; `rollForward: disable` when lock files are used (CS-REPO-07). MTP opt-in: `"test": {"runner": "Microsoft.Testing.Platform"}` (SDK 10).
- **Build policy (CS-REPO-12):** `TargetFramework`, `Nullable`, `ImplicitUsings`, `AnalysisLevel`, `TreatWarningsAsErrors`, `EnforceCodeStyleInBuild`, `RestorePackagesWithLockFile` live in the root `Directory.Build.props`. Only the nearest file is imported; nested ones must import the parent.
- **Central Package Management (CS-REPO-05/06):** versions only in `Directory.Packages.props` (`ManagePackageVersionsCentrally`); `VersionOverride` needs a comment. Transitive pinning for applications, not libraries.
- **Lock files (CS-REPO-07):** commit `packages.lock.json` for applications only; the gate's `restore` check then runs `--locked-mode`.
- **Sources (CS-REPO-08):** `packageSourceMapping` in `nuget.config`, so each package id maps to one feed.
- **Updates and release (CS-REPO-09/10/15):** Renovate or Dependabot (`nuget`), grouped by .NET major; SemVer in the project; `dotnet pack -c Release`, SourceLink, `.snupkg`; conventional commits.
- **Style (CS-REPO-13, CS-TOOL-02):** `.editorconfig` with `root = true`; `dotnet format --verify-no-changes` is the CI check; `.gitattributes` `* text=auto eol=lf`.
- **Analyzers (CS-TOOL-03/04/05):** `AnalysisLevel` `latest-recommended`, nullable on, `-warnaserror` in CI; third-party analyzers (StyleCop, Meziantou, Roslynator) as `GlobalPackageReference` with `PrivateAssets=all`.
- **Versions (CS-TOOL-01):** new services target `net10.0` LTS (support to 2028-11-14); .NET 8 and 9 end 2026-11-10.
- **Supply chain (CS-TOOL-06/07):** NuGetAudit on, `WarningsAsErrors` `NU1901;NU1902;NU1903;NU1904`; SBOM with CycloneDX in releases; local tools in `dotnet-tools.json`.
- **CI order (CS-REPO-14):** restore --locked-mode, format, build -warnaserror, test + Cobertura, `dotnet package list --vulnerable`, publish. Cache `~/.nuget/packages` keyed on lock files or `Directory.Packages.props`.
