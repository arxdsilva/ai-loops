# Scala idioms digest

IDs from `specs/languages/scala/RULES.md`. Project style (`.scalafmt.conf`, `.scalafix.conf`) wins.

## Totality and errors (`SCALA-IDIOM-01`, `-02`)
- Expected failures are values: `Either[DomainError, A]` with a `sealed` error ADT. Exceptions are for bugs and Java interop. Never `throw` in domain; never `catch Throwable` (use `NonFatal`).
- No `null`: `Option(x)` at Java boundaries; `-Yexplicit-nulls` for Java-heavy modules. Never `.get` on `Option`, `Try` or `Either`.

## Immutability (`SCALA-IDIOM-03`, `-12`)
`val` over `var`, immutable collections, `final case class` plus `copy`, no `return`. Mutable state only inside `Ref`, `AtomicReference` or `TrieMap` at the edge. No `.par`; use `traverse`, `parTraverse`, `ZIO.foreachPar`.

## Scala 3 (`SCALA-IDIOM-04`, `-09`)
`enum` for ADTs, `given` and `using` instead of implicits, `extension` methods, `opaque type`. Exhaustive `match`; let exhaustivity and `-Wunused` warnings fail the build. No implicit conversions; pass `ExecutionContext` explicitly or as a named `given`.

## Resources (`SCALA-IDIOM-05`)
`Resource.make` or `Resource.fromAutoCloseable` (cats-effect), `ZIO.acquireRelease` or `ZLayer.scoped` (ZIO), `scala.util.Using` for synchronous code. No manual `close()` in `finally`.

## Equality and time (`SCALA-IDIOM-06`)
`-language:strictEquality` plus `derives CanEqual`, or `cats.Eq`. Time is `java.time` only; inject `Clock[F]` or `zio.Clock`; store `Instant` in UTC; never `java.util.Date`.

## Naming (`SCALA-IDIOM-07`)
`UpperCamelCase` types, `lowerCamelCase` values and methods, lowercase packages, no `get` prefixes, no symbolic methods outside DSLs. Pick one of braces or `significantIndentation` and stay consistent.

## Avoid (`SCALA-IDIOM-08`, `-10`, `-11`)
Runtime reflection and `scala.reflect` in services (prefer `derives`, `inline`, Mirror); `Await.result`, `Thread.sleep`, `scala.concurrent.blocking` in effect code (use `IO.sleep`, `ZIO.sleep`); `Any`-typed collections, `asInstanceOf`, `isInstanceOf`, `Try` as a domain error type; `@SuppressWarnings` without a ticket.

## Compiler flags (`SCALA-TOOL-05`, `-06`)
`-deprecation -feature -unchecked -Wunused:all -Wvalue-discard -Wnonunit-statement`, `-Werror` in CI, `-source:future` on libraries you own, `-language:strictEquality` for domain types. WartRemover in domain and application only (`SCALA-TOOL-07`).

## Codecs (`SCALA-FW-04`)
Use `semiauto` or `derives Codec.AsObject`, not `io.circe.generic.auto._`.

## Performance hot paths (`SCALA-PERF-04`)
Avoid boxing in generics, `List` indexing and `++` in loops (use `Vector` or `ArrayBuffer`), repeated `.toList`, per-call regex compile, `Seq.contains` on large sequences (use `Set`), string `+` in loops. Benchmark with JMH in a `bench` subproject (`SCALA-PERF-01`), profile with async-profiler or JFR (`SCALA-PERF-02`), G1 or ZGC with `-XX:MaxRAMPercentage=75` (`SCALA-PERF-03`).
