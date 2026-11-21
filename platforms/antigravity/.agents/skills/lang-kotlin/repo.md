# Kotlin repo and project management

Digest of `specs/languages/kotlin/RULES.md` REPO and TOOL. Universal repo practice: `specs/REPO_PRACTICES.md` when present.

## Layout (`KT-REPO-01/02`)
```
settings.gradle.kts   gradle.properties   gradlew  gradle/wrapper/*
gradle/libs.versions.toml   gradle.lockfile   build-logic/   config/detekt/detekt.yml   .editorconfig
domain/ application/ adapters/{http,persistence,messaging}/ app/    each: src/{main,test}/kotlin
```
- Standard source sets; one top-level class per `Foo.kt`; packages mirror directories by convention only.
- Multi-project by hexagonal layer, not by technical noise.

## Build logic and versions
- Shared logic in an included build `build-logic/` (convention plugins), wired by `pluginManagement { includeBuild("build-logic") }`. No `allprojects {}`, `subprojects {}` or new `buildSrc` (`KT-REPO-03`).
- Every dependency and plugin version lives once in `gradle/libs.versions.toml` (`KT-REPO-05`).
- Pin the toolchain: commit the wrapper, set the distribution checksum, `kotlin { jvmToolchain(21) }` with the foojay resolver (`KT-REPO-07`). Target Kotlin 2.4.x on JDK 21 or 25 (`KT-TOOL-01`).

## Locking and supply chain
- `dependencyLocking { lockAllConfigurations() }`, generate with `./gradlew dependencies --write-locks`, commit `gradle.lockfile`, `LockMode.STRICT` in CI (`KT-REPO-06`).
- Optional `gradle/verification-metadata.xml` for checksums (`KT-SEC-06`).

## Hygiene
- `.gitignore`: `build/`, `.gradle/`, `.kotlin/`, `out/`, `*.iml`, `*.class`; keep `gradle-wrapper.jar` and `gradle.lockfile` tracked (`KT-REPO-09`).
- `.editorconfig` with `ktlint_code_style = ktlint_official`; one formatter only (`KT-TOOL-02`).
- Conventional commits; version in `gradle.properties` or the tag (`KT-REPO-12`).
- Renovate (understands catalogs and locks); group Kotlin plugin and `kotlinx-*` bumps; `./gradlew buildHealth` for unused deps (`KT-REPO-11`).

## Toolchain
- ktlint via `org.jlleitschuh.gradle.ktlint` (`ktlintCheck`, `ktlintFormat`) (`KT-TOOL-02`).
- detekt stable `io.gitlab.arturbosch.detekt` 1.23.8 with `buildUponDefaultConfig = true`; 2.x is alpha (`KT-TOOL-03`).
- Compiler: `allWarningsAsErrors = true`, `-Xjsr305=strict`, `explicitApi()` for libraries (`KT-TOOL-04`).
- OSV on `gradle.lockfile`, CycloneDX SBOM, license allow-list (`KT-TOOL-05`).

## CI order (`KT-REPO-10`)
Wrapper validation, `ktlintCheck`, `detekt`, `build`, `test` + `koverXmlReport`, vuln scan, image build. This mirrors `loops gate`.

## Publishing (`KT-REPO-08`)
Libraries: SemVer, tag `v<x.y.z>`, `maven-publish` and signing to Central, sources and Dokka jars. Services ship a container.
