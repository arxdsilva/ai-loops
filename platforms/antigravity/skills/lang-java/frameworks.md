# Java: frameworks

Digest of `specs/languages/java/RULES.md` FW. The pack follows whichever framework the repo already uses (`JAVA-FW-07`); detect it from `pom.xml` / `build.gradle*` dependencies (`spring-boot`, `quarkus`, `micronaut`) and never mix a second one in.

## Landscape (as of 2026-11)
- Spring Boot 4.1 on Spring Framework 7, Jakarta EE 11, Java 17 minimum and 25 supported: the dominant choice (`JAVA-FW-01`).
- Quarkus: build-time CDI, native-first, Dev Services. Micronaut: compile-time DI, no reflection, GraalVM-friendly.
- Choose Spring for ecosystem and hiring, Quarkus for Kubernetes-native and fast restarts, Micronaut for compile-time DI and serverless or CLI work (`JAVA-FW-07`).

## Spring Boot (`JAVA-FW-02`, `-05`)
- Starter parent or BOM import; `@SpringBootApplication` in the root package above all modules; `application.yaml` plus profiles; Actuator; `@ConfigurationProperties`.
- Footguns: `@Transactional` and `@Async` self-invocation or private methods (proxy bypass), `@Autowired` field injection, `@Scheduled` on every pod, circular dependencies, `@Value` strings instead of typed properties.
- Hexagonal fit: domain stays free of Spring; the composition root wires ports (`architecture.md`).

## Quarkus (`JAVA-FW-03`)
- ArC CDI (a subset), constructor injection, `@QuarkusTest`, Dev Services (auto Testcontainers), `./mvnw quarkus:dev`. Panache active record conflicts with hexagonal: use `PanacheRepository` inside the persistence adapter.

## Micronaut (`JAVA-FW-04`)
- Compile-time DI and AOP via annotation processors, `@Singleton`, `@Requires`, Micronaut Data, `@MicronautTest`. Beans are plain classes, so hexagonal maps directly.

## Spring Modulith (`JAVA-FW-06`, `JAVA-ARCH-09`, `-10`)
- A module is a direct sub-package of the application package; `package-info.java` with `@ApplicationModule(allowedDependencies=...)`.
- Add `ApplicationModules.of(Application.class).verify()` to a test so the boundary runs in the gate; `@ApplicationModuleTest` for slices; events across modules.

## Detection for the pack
Framework detection is a heuristic (dependency names): treat it as a hint and read the repo's own conventions first. Record any override with `loops run override`.
