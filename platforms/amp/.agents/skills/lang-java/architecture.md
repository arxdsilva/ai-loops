# Java: architecture

Digest of `specs/languages/java/RULES.md` ARCH. The universal rules are `specs/ARCHITECTURE.md` and `specs/SOLID.md`; this only adds the Java forms.

## Layers as packages
- `domain` (pure), `application` (use cases, ports), `adapter.in.*` / `adapter.out.*`, `config` or `bootstrap` (composition root). Feature first, layer second (`JAVA-ARCH-01`).
- Domain is pure (`JAVA-ARCH-05`): no `org.springframework`, `jakarta.persistence`, `jakarta.validation`, Jackson, `java.io`, `java.nio.file`, `java.net`, `java.sql`; no `Instant.now()`, `LocalDate.now()`, `System.currentTimeMillis()`, `UUID.randomUUID()`, `new Random()`, `new Thread`, `Executors.`. Inject `java.time.Clock` and an `IdGenerator` port. These are the pack's `forbidden_in_domain` and `impure_calls`, checked by `loops arch`.
- Keep JPA entities in `adapter.out.persistence` with a mapper (MapStruct or by hand); annotating domain classes `@Entity` needs an ADR (`JAVA-ARCH-06`).

## Ports
- Small, consumer-owned interfaces in `domain` or `application`, named for the capability (`OrderRepository`, `PaymentGateway`), no `IFoo`/`FooImpl` (`JAVA-ARCH-07`). A Spring Data repository sits behind a port, it is not the port.
- Constructor injection only: one constructor, `final` fields, no field or setter injection, no `getBean` or static locators. Adapters are wired in `@Configuration` classes in the composition root (`JAVA-ARCH-04`).

## Access and enforcement
- Package-private by default; `public` only for the module API; sealed interfaces to close hierarchies; `module-info.java` for libraries (`JAVA-ARCH-02`).
- Package cycles compile, so forbid them with ArchUnit (`slices().matching("..(*)..").should().beFreeOfCycles()`).
- Enforce direction in the normal test run with ArchUnit `onionArchitecture()` or `layeredArchitecture()` plus a rule that `..domain..` does not depend on frameworks (`JAVA-ARCH-03`). `loops arch` scans `import` lines with the `package-dir` resolver; ArchUnit covers what imports cannot see (types, annotations).
- Multi-module builds make direction compiler-enforced: `domain` has no dependency on `adapters-*`.

## Size heuristics (warnings)
Class ≤ 300 lines, method ≤ 30, ≤ 4 constructor parameters (more is an SRP signal: parameter object or split), cyclomatic complexity ≤ 10, interface ≤ 5 methods (`JAVA-ARCH-08`).

## Modular monolith
Prefer Spring Modulith before microservices (`JAVA-ARCH-09`, `-10`): modules are sub-packages of the application package, cross-module calls go through the module API or published events (`@ApplicationModuleListener`). Details in `frameworks.md`.
