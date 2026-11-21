# Scala frameworks digest

Respect the project's stack; do not mix stacks (`SCALA-FW-01`). Project rules win; log overrides. IDs from `specs/languages/scala/RULES.md`.

## Dominant stacks (as of 2026-11)
- Typelevel: http4s plus Tapir plus doobie or skunk on cats-effect, fs2 for streams.
- ZIO: ZIO HTTP or Tapir, ZIO Test, zio-jdbc or Quill.
- Play: legacy and CRUD, full-stack.
- Pekko (Apache fork of Akka 2.6): actors, clustering, streams, Pekko HTTP.

## Fitting the hexagon (`SCALA-FW-02`)
- http4s: `F[_]: Async`, servers as `Resource`. Keep routes in `adapters-http` and make them thin.
- ZIO: layers and `ZIOAppDefault`; one `ZLayer` per adapter project, composed in `app`.
- Play: wants `app/`, `conf/routes`, controllers and compile-time DI. Keep controllers thin adapters in an `adapters-http` project calling application ports; otherwise it fights the subproject graph.
- Pekko: typed actors as adapters or as an application service for per-entity state; never share mutable state across actors.

## Akka licence (`SCALA-ARCH-11`, `SCALA-FW-03`)
Akka 2.7+ is BSL 1.1 and not free for production (Change Date 2029 for 2.10.x). New projects use Apache Pekko: `org.apache.pekko`, imports renamed from `akka.*`. Play users get a grant for akka-streams only through Play; anything else needs legal review. Flag, do not migrate silently.

## Framework footguns (`SCALA-FW-04`)
- Play's Guice DI hides wiring errors until runtime: prefer compile-time DI.
- http4s `HttpRoutes` return `OptionT`: a missed case is a 404, not a compile error. Test every route.
- ZIO layer wiring errors surface at startup as long type errors; keep layers small.
- circe or Tapir auto-derivation bloats implicit scope and compile time: use `semiauto` or `derives`.
- Future-based Slick on a small pool starves; bound and size pools (`SCALA-DATA-05`).

## Picking nothing new
When the task does not name a framework, read `build.sbt` and `project/Dependencies.scala` first and follow what is there. Introducing a second HTTP or effect library needs the architect's approval and a plan entry.
