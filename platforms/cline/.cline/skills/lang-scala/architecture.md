# Scala architecture digest

Neutral rules: `specs/ARCHITECTURE.md`, `specs/SOLID.md`. Scala mapping below; IDs from `specs/languages/scala/RULES.md`.

## Shape (`SCALA-ARCH-01`)
Modular monolith: one deployable, one sbt subproject per layer (and per bounded context when large). Split to services only for independent scaling or ownership. CQRS or event sourcing only where audit or replay is required.

## Layers (`SCALA-REPO-02`, `SCALA-ARCH-02`, `-03`)
- `domain`: plain Scala. `final case class`, `enum` or `sealed trait` ADTs, `opaque type` IDs, pure functions returning `Either` or `Option`. No `Future`, `IO`, `ZIO`, `Ref`, `Instant.now()`, `Random`, `sys.env`.
- `application`: use cases and ports. Ports are small traits owned by the consumer: `trait OrderRepo[F[_]] { def find(id: OrderId): F[Option[Order]] }`, or an `IO` or `ZIO` variant with a `ZLayer`.
- Adapters implement ports and hold framework code (http4s, Tapir, doobie, Kafka). `app` is the composition root.
- Direction: the `dependsOn` graph is the first guard; `loops arch` reads `import` lines and `package` clauses (`SCALA-ARCH-08`) and flags forbidden imports and impure calls in `domain`.

## One effect style per service (`SCALA-ARCH-04`)
Tagless final (`F[_]: Async`), concrete `IO`, or ZIO: pick one. Do not mix `Future`, `IO` and `ZIO` in application code; convert at the edge (`IO.fromFuture`, `ZIO.fromFuture`).

## Wiring (`SCALA-ARCH-05`, `-06`)
Manual wiring in `Main` with `Resource[IO, A]` or `ZLayer`; no runtime DI container. One `ZLayer` (or `make[F[_]: Async](cfg): Resource[F, Port[F]]`) per adapter project, composed only in `app`. Keep live and fake layers in different projects or configs. Play's Guice only if the framework forces it; prefer compile-time DI.

## Visibility and cycles (`SCALA-ARCH-07`)
`private`, `private[pkg]`, `final` on non-extension classes, `sealed` ADTs for exhaustive `match`. Scala allows package cycles inside one project, so acyclicity is enforced by subprojects, not packages.

## Value objects and entities (`SCALA-ARCH-10`)
Smart constructors return `Either[DomainError, A]` (or `iron` or `refined` types). Entities carry an ID and are immutable with `copy`; domain events are `sealed trait` or `enum`.

## Actors (`SCALA-ARCH-11`)
Pekko Typed (`org.apache.pekko`) for per-entity state, clustering and streams; never Akka 2.7+ (BSL). Otherwise cats-effect with fs2, or ZIO. Never share mutable state across actors.

## Size limits (`SCALA-ARCH-12`)
File at most 400 lines, function at most 40, at most 5 parameters (use a case class), traits at most 7 methods, one public top-level type per file (companions together).

## Package-to-directory (`SCALA-ARCH-08`)
`package a.b` conventionally lives in `a/b/` but is not enforced; chained `package a; package b` clauses and relative imports exist. The scanner is partial, which is why the sbt graph is primary. Import forms: `import a.b.C`, `import a.b.{C, D as E}`, `import a.b.*` (Scala 3), `import a.b._` (Scala 2), `import a.b.{given, *}`. Optional extra: ArchUnit through JUnit for package rules inside one project (`SCALA-ARCH-09`).

## Review hooks
Tagless-final leaks into domain, effect types in port signatures that the domain should not know, adapters importing each other, `implicit` bloat.
