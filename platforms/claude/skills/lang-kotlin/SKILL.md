---
name: lang-kotlin
description: Use when writing, reviewing, testing or setting up Kotlin backend code in a loops run (the kotlin language pack) - toolchain, layout, race policy, coverage, idioms and footguns.
---

# Kotlin backend (JVM, Gradle)

Applies when `loops lang detect` reports `kotlin` (`src/main/kotlin`, or a Gradle build applying the Kotlin plugin). Rules: `specs/languages/kotlin/RULES.md` (IDs `KT-<SEC>-NN`). Pack data: `specs/languages/kotlin/pack.json` (`loops lang show kotlin`). Layering and SOLID are language-neutral: `specs/ARCHITECTURE.md`, `specs/SOLID.md`.

## Project rules first

The project's own config wins over this skill: `.editorconfig`, `config/detekt/detekt.yml`, the version catalog, framework conventions (Spring or Ktor), existing `build-logic/` conventions. When you follow one over a loops rule, record it: `loops run override --rule "<loops rule>" --project "<project rule, file:line>" --note "<what you did>"` (ROLES.md "Project rules first"). The practices-researcher reads `config_files` from the pack.

## Toolchain the gate runs

`loops gate` runs these in order; while iterating run a single one. All use the wrapper (`./gradlew`, never a machine Gradle) and `--console=plain`; the gate adds `--project-cache-dir {run}/.gradle` (`cache_argv`).

| Check | Command | Notes |
|---|---|---|
| build (required) | `./gradlew assemble` | compiles main; warnings are errors by project config (`KT-TOOL-04`) |
| ktlint (optional) | `./gradlew ktlintCheck` | skipped with a hint if the plugin is absent |
| detekt (optional) | `./gradlew detekt` | stable plugin `io.gitlab.arturbosch.detekt` 1.23.8 (`KT-TOOL-03`) |
| arch (optional) | `loops arch` | reads each file's `package` header, not its directory |
| test (required) | `./gradlew test --rerun koverXmlReport` | `--rerun` goes after `test`, or Gradle reports UP-TO-DATE and skips the tests |
| test-repeat (required) | `./gradlew test --rerun`, 2 times | race substitute, see below |
| coverage (required) | loops parses `build/reports/kover/report.xml` | JaCoCo schema, new-code at least 70% |
| osv (off by default) | `osv-scanner scan source -L gradle.lockfile` | `--vuln` |

Narrow scope while working: `./gradlew :module:test --tests 'com.acme.FooTest'` (`KT-TEST-11`). Never run the full gate as an implementer; the lead and QA do (ROLES.md).
Gradle is a shared daemon host: keep `GRADLE_USER_HOME` shared (it holds downloads), scope only the project cache dir by argv (`KT-OPS-05`). Never set `ORG_GRADLE_PROJECT_*` for it; it does nothing.

## Race policy (WORKFLOW §6)

The JVM has no race detector, so the pack's mode is `none`. The gate still requires the substitute and never skips it:
- `junit-platform.properties` turns on parallel execution (`junit.jupiter.execution.parallel.enabled=true`, `...mode.default=concurrent`) and random method order (`junit.jupiter.testmethod.order.default=org.junit.jupiter.api.MethodOrderer$Random`, a fully qualified name). Gradle `-D` does not reach the test JVM, so the file is the only route.
- `test` runs once, `test-repeat` twice more; each run picks a new random seed. A test that fails in any run fails the gate; never add retries.
- Tests touching shared state use `@RepeatedTest`; concurrent data structures get a Lincheck test (`KT-CONC-04`); coroutine code uses `runTest` virtual time with injected dispatchers (`KT-TEST-09`).
- If the properties file is missing the gate fails with the exact snippet; see the next section.

## Coverage recipe and wave-0 prerequisites (D13)

The architect or `kotlin-engineer` adds these in wave 0 and logs each as an override if the project already differs:
- Kover `org.jetbrains.kotlinx.kover` 0.9.11 in the root build; `koverXmlReport` aggregates modules into `build/reports/kover/report.xml` (`KT-TEST-06`).
- `src/test/resources/junit-platform.properties` with the three race keys above; `tasks.test { useJUnitPlatform() }`.
- `kotlin { compilerOptions { allWarningsAsErrors = true } }` and `jvmToolchain(21)`.
- Dependency locking with a committed `gradle.lockfile` (`KT-REPO-06`).
- Exclude generated code in `kover { reports { filters { excludes { ... } } } }`.

## How the loops layers map

```
settings.gradle.kts           include(":domain", ":application", ":adapters:http", ":app")
domain/        no dependencies; no framework, java.io/net/sql, Instant.now, UUID.randomUUID
application/   -> domain; ports are small interfaces defined here (consumer-owned)
adapters/*/    -> application; Spring/Ktor/jOOQ/Exposed live only here
app/           composition root: manual wiring or @Configuration
```
Module dependencies are the compiler-enforced direction (`KT-ARCH-02`); `loops arch` and Konsist cover package-level rules inside a module (`KT-ARCH-03/04`). Pure domain: inject `Clock` and an id generator (`KT-ARCH-06`).

## Ten footguns

1. Swallowed `CancellationException` (`catch (e: Exception)`, `runCatching` on suspend code): rethrow it.
2. `GlobalScope.launch`: own a `CoroutineScope(SupervisorJob() + dispatcher)` and cancel it on shutdown.
3. Blocking IO on `Dispatchers.Default`: `withContext(Dispatchers.IO)` or an injected dispatcher.
4. Platform types (`String!`) leaking from Java: declare the Kotlin type; `-Xjsr305=strict`.
5. `!!` and `lateinit` in domain code: `?:`, `requireNotNull`, constructor injection.
6. `data class` as a JPA entity: plain class with the `kotlin-jpa` and all-open plugins, mapped to a domain type.
7. Bean Validation annotation on a constructor property without `@field:` on an older default target: silently ignored.
8. `./gradlew test` is UP-TO-DATE and runs nothing: `--rerun` after `test`.
9. `delay` under `runTest` on a hard-coded `Dispatchers.IO` waits real time: inject the dispatcher.
10. MDC, transaction or OTel context bound to a thread while coroutines hop: `MDCContext()`, coroutine-aware transaction managers.

## Load on demand

- `repo.md`: Gradle layout, catalog, locking, wrapper, CI, publishing.
- `architecture.md`: modules as layers, visibility, Konsist, DI, purity.
- `idioms.md`: errors, null-safety, immutability, sealed types, resources.
- `testing.md`: JUnit 5, MockK, Testcontainers, `runTest`, Kover, flakiness.
- `concurrency.md`: coroutines, Flow, cancellation, Lincheck, workers.
- `security.md`: injection, deserialization, secrets, supply chain, SAST.
- `data-api.md`: persistence, migrations, HTTP, messaging, observability, ops.
- `frameworks.md`: Spring Boot, Ktor, footguns, Spring/Ktor conventions.
