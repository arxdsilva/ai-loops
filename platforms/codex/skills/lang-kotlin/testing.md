# Kotlin testing

Digest of RULES TEST. Universal testing practice: `specs/TESTING.md` when present; coverage and race gates: `specs/WORKFLOW.md §6`.

## Frameworks and layout (`KT-TEST-01/02`)
- JUnit 5 via `useJUnitPlatform()`; Kotest assertions and property modules on top. One runner style per module.
- Unit tests in `src/test/kotlin`; integration in a separate `integrationTest` source set. Files `*Test.kt`, `*Tests.kt`, `*IT.kt`, `*Spec.kt`; backtick names are fine.
- E2E: Spring `@SpringBootTest`/`WebTestClient`, Ktor `testApplication { }`.

## Style and doubles (`KT-TEST-03/04`)
- Arrange-act-assert; `@ParameterizedTest` with `@MethodSource`/`@CsvSource` (or Kotest `withData`); `@Nested` for grouping; no shared mutable fixtures.
- Hand-written fakes for ports. MockK (`coEvery`/`coVerify` for suspend) only at boundaries you do not own; never mock value objects or the class under test; `unmockkAll` after `mockkStatic`/`mockkObject`.

## Integration (`KT-TEST-05`)
Testcontainers (`PostgreSQLContainer`, `KafkaContainer`), one container per class, truncate or roll back per test, Flyway against the real engine, never H2 as a Postgres stand-in.

## Coverage (`KT-TEST-06/07`)
- `./gradlew test --rerun koverXmlReport` writes `build/reports/kover/report.xml` (JaCoCo schema); HTML is `build/reports/kover/html`.
- Exclude generated code in the `kover { reports { filters { excludes { ... } } } }` block. An `annotatedBy` filter needs BINARY or RUNTIME retention.
- Kover has no diff mode: loops intersects `git diff -U0` lines with the report's per-line counters; the bar is 70% on new code.

## Coroutines (`KT-TEST-09`)
`runTest` (virtual time, 60 s default timeout) with `StandardTestDispatcher`, `advanceUntilIdle`, `advanceTimeBy`, `backgroundScope` for long-lived collectors. Inject dispatchers: `delay` on a real dispatcher is not virtualised. `Dispatchers.setMain` needs `resetMain()` in teardown. Inject `Clock`.

## Flakiness and the race substitute (`KT-TEST-10`)
- No `Thread.sleep` (use Awaitility or virtual time), unique data per test, `Random(seed)`, `@Timeout`.
- `junit-platform.properties` carries parallel execution and `MethodOrderer$Random`. The value must be the fully qualified class name.

## Fast loop (`KT-TEST-11`)
`./gradlew :m:test --tests 'com.acme.FooTest'`, `--continuous` for watch mode, `org.gradle.caching=true` and `org.gradle.configuration-cache=true`. Add `--rerun` after `test` whenever tests must actually execute.

## Property and mutation (`KT-TEST-08`)
Kotest `checkAll` or jqwik for domain invariants. PIT mutation only on the domain module, on a schedule.
