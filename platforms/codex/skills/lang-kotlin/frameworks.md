# Kotlin frameworks

Digest of RULES FW. Follow the project's framework conventions over these defaults.

## Choosing (`KT-FW-01`)
Spring Boot dominates (ecosystem, enterprise integration, first-class coroutines). Ktor is coroutine-native and lean for small Kotlin-first services. Micronaut and Quarkus are minor.

## Spring Boot (`KT-FW-02`)
- `kotlin-spring` (all-open) so `@Configuration` and `@Transactional` classes are proxiable; `@ConfigurationProperties` on data classes; `-Xjsr305=strict`; `application.yml` in `src/main/resources`.
- Where annotations fight the hexagon, keep them in adapters and `app/` (`KT-ARCH-09`).
- Virtual threads: `spring.threads.virtual.enabled=true` (`KT-CONC-01`).
- Probes: Actuator `/actuator/health/liveness` and `/readiness` (`KT-OBS-04`).

## Ktor (`KT-FW-03`)
`Application.module()` extension functions, plugins via `install(...)`, routes in feature files, `testApplication`, HOCON or YAML config. No built-in DI: Koin, Kotlin-Inject, or manual wiring in `module()`. `StatusPages` for the error mapping (`KT-API-03`).

## Footguns (`KT-FW-04`)
- Spring proxy self-invocation bypasses `@Transactional`.
- `suspend` plus `@Transactional` needs the coroutine-aware transaction manager.
- Final classes break proxies without all-open.
- Jackson without the Kotlin module ignores non-null defaults and constructors.
- A blocking Ktor handler starves the event loop.
- `Dispatchers.setMain` is global state.

## Test hooks
Spring: `@SpringBootTest`, `WebTestClient`, `@ServiceConnection` with Testcontainers. Ktor: `testApplication { }` (`KT-TEST-01/05`).
