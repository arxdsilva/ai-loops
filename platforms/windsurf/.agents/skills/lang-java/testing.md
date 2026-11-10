# Java: testing and performance

Digest of `specs/languages/java/RULES.md` TEST and PERF. Universal testing rules: `specs/TESTING.md` where present, and WORKFLOW §6.

## Stack and layout
- JUnit Jupiter (5.x/6.x via `junit-bom`) + AssertJ + Mockito + Testcontainers; no JUnit 4 or TestNG in new code (`JAVA-TEST-01`).
- `src/test/java` mirrors main; `*Test` runs under Surefire, `*IT` under Failsafe in `verify` or a Gradle `integrationTest` suite (`JAVA-TEST-02`). These match the pack's `test_globs`, which also feed the test guard and ownership.
- Arrange-act-assert, one behaviour per test, `@DisplayName` or `method_condition_result` names, `@ParameterizedTest`, `@Nested`; fresh instance per test (`JAVA-TEST-03`).

## Doubles and integration
- Fakes and in-memory port implementations for application tests; Mockito (`MockitoExtension`, strict stubs) only at the boundary; never mock types you do not own, value objects or the class under test (`JAVA-TEST-04`).
- Real dependencies via Testcontainers (`@ServiceConnection` in Spring Boot), pinned to the production image tag, shared per class, isolated by rollback or reset (`JAVA-TEST-05`).
- Spring slices (`@WebMvcTest`, `@DataJpaTest`, `@JsonTest`) for adapters, plain tests for domain and application, `@SpringBootTest` for a few end-to-end cases (`JAVA-TEST-11`). `@DataJpaTest` rolls back and can hide flush issues.
- Contract tests with REST-assured, `MockMvcTester`, Pact or spring-cloud-contract; ArchUnit tests are ordinary JUnit tests (`JAVA-TEST-12`).

## Coverage
- JaCoCo in every build (`JAVA-TEST-06`); loops computes new-code coverage from `jacoco.xml` against the diff, ≥ 70% of changed executable lines (`JAVA-TEST-07`). Recipe and prerequisites: `SKILL.md`.
- Property tests with jqwik and mutation tests with PIT on changed classes, nightly (`JAVA-TEST-08`).

## Flakiness and the race substitute
- No `Thread.sleep` (use Awaitility), injected `Clock`/`Random` seeds, no shared static state, no order dependence, `@Timeout`, bounded Spring context cache; quarantine rather than retry (`JAVA-TEST-09`).
- The gate runs the suite twice with parallel execution and `MethodOrderer$Random` under different seeds; a failure is a real isolation bug. See `concurrency.md`.
- Fast loop (`JAVA-TEST-10`): `./mvnw -pl <module> -Dtest=Class#method test`, `./gradlew :module:test --tests '...'`, Gradle `--continuous`.

## Performance
- JMH only for micro-benchmarks, with `@Fork`, warmup, `Blackhole`; keep results outside the tree's tracked files (`JAVA-PERF-01`).
- Profile with JFR, async-profiler, heap dumps and MAT; measure before tuning (`JAVA-PERF-02`). Container memory: `-XX:MaxRAMPercentage=75`, G1 default, ZGC for low pause (`JAVA-PERF-03`).
- Avoid hot-path mistakes: `+` in loops, boxing, per-call `Pattern`, exceptions for control flow, unbounded caches (`JAVA-PERF-04`). Database throughput first: N+1, batch size, indexes, pool size (`JAVA-PERF-05`).
