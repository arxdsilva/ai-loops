# Swift: repo and toolchain digest

Rules in `specs/languages/swift/RULES.md` (REPO, TOOL).

## Layout
- One SwiftPM package: `Sources/<Target>`, `Tests/<Target>Tests`; executables in `executableTarget`, libraries in `target`. No `.xcodeproj` (SWIFT-REPO-01).
- Prefer one package with many targets; use local `.package(path:)` only for a part with its own release cadence (SWIFT-REPO-03).
- Bootstrap: `swift package init --type executable --name <Svc>` or the Hummingbird or `vapor new` templates, then delete the samples, commit a `.swift-format`, add the layer targets (SWIFT-REPO-04).

## Manifest and lockfile
- Line 1 `// swift-tools-version: 6.x` (at least 6.0): it makes Swift 6 language mode the default (SWIFT-REPO-02).
- Commit `Package.resolved` for services; CI builds with `--force-resolved-versions` so a stale lock fails (SWIFT-REPO-05).
- Dependencies use `from:`; `exact:` or a revision only for forks and security pins, with a comment; never `branch:` on a release (SWIFT-REPO-06).
- Dependabot (`swift` ecosystem) or Renovate keeps them current; review `Package.resolved` diffs; remove unused `.product(...)` entries (SWIFT-REPO-07).

## Toolchain
- Target Swift 6.4.x; there is no LTS, so pin and upgrade deliberately. Pin with `.swift-version` (swiftly) and mirror it in the Docker base image and CI matrix (SWIFT-TOOL-01, SWIFT-REPO-09).
- Swift Build is the default build system in 6.4; `--build-system native` is deprecated. Query paths, never hardcode: `swift build --show-bin-path`, `swift test --show-codecov-path` (SWIFT-TOOL-07).
- Format: toolchain `swift format`, config `.swift-format` (JSON, found by walking up). Commit one: the stock template fails `lint --strict` against the 2-space default (SWIFT-TOOL-02).
- One lint stack: swift-format for layout plus SwiftLint for semantic rules; do not run two formatters in autofix mode (SWIFT-TOOL-03).
- Type checking is the compiler: Swift 6 mode, `treatAllWarnings(as: .error)` or `-Xswiftc -warnings-as-errors` in CI (SWIFT-TOOL-04).
- No first-party vuln tool and OSV-Scanner does not read `Package.resolved`: use Dependabot alerts and `swift package generate-sbom` (SWIFT-TOOL-05).

## Hygiene and CI
- `.gitignore`: `/.build`, `/Packages`, `.swiftpm/xcode/`, `xcuserdata/`, `DerivedData/`, `.netrc`, `.DS_Store`, `*.profraw` (SWIFT-REPO-10).
- CI order: cache `.build` keyed on `Package.resolved` plus tools version, format lint, SwiftLint, `swift build --force-resolved-versions -c release`, `swift test --sanitize=thread --enable-code-coverage`, lcov export, SBOM, container build. Matrix: latest and previous Swift minor on Linux (SWIFT-REPO-11).
- Release a library by a SemVer git tag; services ship a container tagged by git SHA and semver (SWIFT-REPO-08). Conventional commits (SWIFT-REPO-12).
- Cleanup: `swift package clean`; `swift package reset` also drops caches (SWIFT-OPS-06).
