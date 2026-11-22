# Kotlin architecture

Digest of RULES ARCH. The layering rules are `specs/ARCHITECTURE.md`; this file only maps them onto Gradle and Kotlin.

## Modules are the layers (`KT-ARCH-01/02`)
- Modular monolith first; each layer is a Gradle module listing only inward dependencies: `:domain` has none, `:application` -> `:domain`, adapters -> `:application`, `:app` -> all.
- Use `implementation` by default, `api` deliberately. A forbidden import then fails at compile time; module cycles are illegal in Gradle.
- DDD tactical: entities as classes, value objects as `@JvmInline value class`, one root per aggregate, events as sealed hierarchies.

## Inside a module (`KT-ARCH-03/04`)
- Default `internal` for module-private code (Kotlin's default is `public`), `private` otherwise.
- Package cycles are legal, so ban them with a test. Konsist: `Konsist.scopeFromProduction().assertArchitecture { domain.dependsOnNothing(); application.dependsOn(domain) }` as an ordinary test in `app/src/test`. ArchUnit through JUnit works too.
- The directory is not enforced to match the package, so `loops arch` reads the `package` header. Imports: `import a.b.C`, `a.b.*`, `a.b.C as D`.

## Wiring (`KT-ARCH-05`)
Manual constructor wiring in `app/` or Spring `@Configuration`; constructor injection only. No `@Autowired lateinit var`, no `ApplicationContext.getBean`, no Koin `get()` in domain code. Koin or Kotlin-Inject are fine for Ktor.

## Pure domain (`KT-ARCH-06`)
Forbidden in domain (also the pack's `forbidden_in_domain`): `java.io`, `java.net`, `java.sql`, `javax/jakarta.persistence`, `org.springframework`, `io.ktor`, Jackson annotations. Impure calls to inject instead: `Instant.now()`, `LocalDate.now()`, `System.currentTimeMillis()`, `UUID.randomUUID()`, `Random`, `GlobalScope`, `Thread`. Inject a `Clock` and an id generator port.

## Ports (`KT-ARCH-07`)
Small interfaces (often `fun interface`) defined in `:application` by the consumer. `suspend` is fine on ports; never leak a `Flow` of framework types.

## Size signals (`KT-ARCH-08`)
File under about 400 lines, function under 30, at most 5-6 parameters. Enable detekt `LongMethod`, `LargeClass`, `LongParameterList`, `ComplexCondition`, `TooManyFunctions`.

## Spring (`KT-ARCH-09`, `KT-FW-02`)
Apply `kotlin("plugin.spring")` and `kotlin("plugin.jpa")` only in adapter modules. No `@Entity` on domain classes; keep a separate persistence model mapped to the domain.
