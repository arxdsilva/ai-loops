---
name: lang-java
description: Use when writing, reviewing, testing or setting up Java backend code in a loops run (the java language pack) - toolchain, layout, race policy, coverage, idioms and footguns.
---

# Java backend (the `java` pack)

JDK 25 LTS (27 current, 21 floor for libraries), Maven 3.9/3.10 base with a Gradle variant, JUnit 6, Spring Boot 4.1 as the dominant framework. As of 2026-11. Rule book: `specs/languages/java/RULES.md` (IDs `JAVA-<SEC>-NN`). Pack data: `specs/languages/java/pack.json` (`loops lang show java`).

## When it applies

`loops lang detect` reports `java`: a `pom.xml`, `build.gradle.kts` or `build.gradle` at the root, and no `src/main/kotlin` (that goes to the `kotlin` pack). The Gradle variant is picked when a `build.gradle*` file exists. Multi-module reactors and multi-project builds are one pack root; the tool aggregates modules, loops does not iterate them.

## Project rules first

The repo's own config beats the pack defaults: its Checkstyle/PMD/SpotBugs rulesets, `.editorconfig`, Spotless or formatter settings, `.mvn/maven.config`, `gradle.properties`, the framework it already uses (Spring, Quarkus or Micronaut: `JAVA-FW-07`), its test conventions and JDK pin. Read `config_files` from the pack before coding. Every time you follow a project rule over a loops rule, log it: `loops run override --rule "<loops rule>" --project "<project rule, file:line>" --note "<what you did>"` (ROLES.md "Project rules first").

## Toolchain the gate runs

Agents run `loops gate` (all checks) or `loops gate --only <check>,...` while iterating. The checks, in order (Maven form; the Gradle variant uses `./gradlew --no-daemon <task>`):

| Check | Command | Required |
|---|---|---|
| build | `./mvnw -B -ntp -DskipTests verify` (falls back to `mvn` without a wrapper) | yes |
| spotless | `./mvnw -B -ntp spotless:check` (skipped when the plugin is not in the pom) | no |
| checkstyle, pmd | `./mvnw -B -ntp checkstyle:check` / `pmd:check`, only when the ruleset file is committed | no |
| spotbugs | `./mvnw -B -ntp spotbugs:check` (FindSecBugs as its plugin) | no |
| arch | `loops arch` (import scanner, `package-dir` resolver) | no |
| test | `./mvnw -B -ntp verify` plus the JUnit parallel and random-order keys, run twice | yes |
| coverage | loops parses JaCoCo XML for new-code coverage (WORKFLOW §6) | yes |
| osv-scanner, dependency-check, sbom | opt-in (`--vuln`, or enable in config); OSV needs no key, dependency-check needs an NVD key | no |

Gradle equivalents: `assemble`, `spotlessCheck`, `checkstyleMain checkstyleTest`, `spotbugsMain`, `pmdMain`, `test --rerun jacocoTestReport`, `dependencyCheckAnalyze`, `cyclonedxBom`.

Direct commands for a fast loop (the test-guard accepts `./mvnw test|verify`, `mvn test|verify`, `./gradlew test`, `gradle test`):

- one test: `./mvnw -B -ntp -pl <module> -Dtest=OrderServiceTest#placesOrder -Dsurefire.failIfNoSpecifiedTests=false test`
- Gradle: `./gradlew :module:test --tests 'com.acme.OrderServiceTest.placesOrder'`

Always use the wrapper (`JAVA-REPO-03`). Do not set `JAVA_HOME`, `JAVA_TOOL_OPTIONS`, `MAVEN_ARGS` or Gradle cache dirs yourself; the run environment scopes tmp and the JaCoCo exec, and `~/.m2` and `GRADLE_USER_HOME` stay shared (`JAVA-OPS-05`). Never relocate `-Dmaven.repo.local` per run.

## Race policy

The JVM has no general data-race detector, so the pack's race mode is `none` with a required substitute (WORKFLOW §6, GENERIC §5.2). The `test` check carries it:

- JUnit Jupiter parallel execution on (`junit.jupiter.execution.parallel.enabled=true`, `mode.default=concurrent`, `mode.classes.default=concurrent`);
- random method order (`junit.jupiter.testmethod.order.default=org.junit.jupiter.api.MethodOrderer$Random`);
- the whole check runs **twice**, with a different `junit.jupiter.execution.order.random.seed` each run; the gate records the seeds. A failing seed is the reproduction: rerun it with that seed.

Do not defeat it: no `--rerun-failed`/retry plugins that hide a flake (`rerunFailingTestsCount`), no fixed order, no `-DforkCount=0` serial mode, no `@Execution(SAME_THREAD)` to silence a failure. A test that only passes serially has a race or shared state: fix the code or the test isolation (`JAVA-TEST-09`, `JAVA-CONC-05`).

On Gradle the `-D` flags do not reach the test JVM. The project needs a committed `src/test/resources/junit-platform.properties` with the same keys (architect or java-engineer, wave 0). Without it the gate's race substitute does not run: report it as a missing prerequisite, do not skip it. Where the project has jcstress or Lincheck suites, run them for lock-free or memory-model-sensitive code (`JAVA-CONC-06`) and report the result. If the substitute cannot run on the host, the gate fails with "race policy unavailable"; never skip it silently.

## Coverage recipe

The gate parses `**/target/site/jacoco/jacoco.xml` (Gradle: `**/build/reports/jacoco/test/jacocoTestReport.xml`), one file per module, merged. Source files resolve against `src/main/java`. Coverage is line-based; new-code coverage ≥ 70% is computed by loops against the diff (`JAVA-TEST-06`, `-07`).

Wave-0 prerequisites the architect or `java-engineer` adds if missing (log each as an override when the project already differs):

- Maven: `jacoco-maven-plugin` with executions `prepare-agent` and `report` (bound to `verify`); any Surefire `<argLine>` must include `@{argLine}` or the agent is dropped and the report is empty.
- Gradle: the `jacoco` plugin with `tasks.jacocoTestReport { reports { xml.required = true } }`; the test task must `finalizedBy` or be followed by `jacocoTestReport`.
- `src/test/resources/junit-platform.properties` with the parallel and random-order keys (needed for Gradle, harmless for Maven).
- Optional: `spotless-maven-plugin` (google-java-format), SpotBugs with FindSecBugs, an ArchUnit test (`JAVA-ARCH-03`).

The exec file is relocated per run with both `-Djacoco.destFile` and `-Djacoco.dataFile` (done by the run environment). Setting only `destFile` makes the report read stale data. Gradle needs `test --rerun`, or `:test UP-TO-DATE` gives a stale report. Exclude generated code and composition roots (`*Application`, `config/**`); the pack's globs already do.

## Layers on this language

```
src/main/java/com/acme/orders/
  domain/            pure: records, sealed types, value objects, Clock and IdGenerator ports; no Spring, JPA, Jackson, java.io, java.sql
  application/       use cases (@Transactional here), consumer-owned ports (OrderRepository, PaymentGateway)
  adapter/in/web/    controllers, request records + Bean Validation, ProblemDetail mapping
  adapter/out/persistence/   JPA entities + mappers, Flyway migrations in resources/db/migration
  config/            composition root: @Configuration wiring adapters to ports, @SpringBootApplication
src/test/java/...    mirrors main; *Test unit, *IT integration (Failsafe / jvm-test-suite)
```

Direction is enforced by `loops arch` (imports resolved by package and directory) and, where present, ArchUnit tests in the normal test run. See `specs/ARCHITECTURE.md`, `JAVA-ARCH-01..10`.

## Top 10 footguns

1. Open-session-in-view left on: `spring.jpa.open-in-view=false`.
2. N+1 from lazy collections: `JOIN FETCH`, `@EntityGraph`, DTO projections.
3. `@Transactional` self-invocation or on private methods: move to another bean.
4. `@Data`/naive `equals` on JPA entities: id-based equals, constant `hashCode`.
5. `Optional` as a field or parameter, or `.get()`: `orElseThrow`.
6. Mutable state in singleton beans: stateless or `java.util.concurrent`.
7. Swallowed `InterruptedException`: restore the interrupt flag and propagate.
8. Preview APIs (`StructuredTaskScope`) on an LTS: executors or `CompletableFuture`.
9. `double` money, `new BigDecimal(0.1)`, `SimpleDateFormat`: `BigDecimal("0.10")`, `java.time`.
10. Native deserialization, XXE, SSRF defaults: closed-type JSON, secure-processing features.

## Load on demand

- `repo.md`: layout, wrapper, JDK toolchain, BOMs, CI, `.gitignore` (REPO, TOOL).
- `architecture.md`: layering, access control, ArchUnit, constructor injection, modules (ARCH).
- `idioms.md`: exceptions, Optional, records, sealed types, equals, time and money (IDIOM).
- `testing.md`: JUnit, AssertJ, Mockito, Testcontainers, coverage, flakiness (TEST, PERF).
- `concurrency.md`: virtual threads, scoped values, race substitute, async and background work (CONC).
- `security.md`: injection, deserialization, secrets, supply chain, SAST (SEC).
- `data-api.md`: JPA, transactions, Flyway, HTTP and messaging, observability, packaging (DATA, API, OBS, OPS, DOC).
- `frameworks.md`: Spring Boot, Quarkus, Micronaut, Modulith (FW).
