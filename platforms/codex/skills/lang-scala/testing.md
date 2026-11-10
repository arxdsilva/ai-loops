# Scala testing digest

Neutral test rules: `specs/TESTING.md` when present, WORKFLOW §6. IDs from `specs/languages/scala/RULES.md`.

## Frameworks (`SCALA-TEST-01`)
MUnit by default (`CatsEffectSuite`, `ScalaCheckSuite`); ScalaTest for existing codebases; weaver-test for cats-effect-native parallel suites; ZIO Test for ZIO. One per project.

## Layout (`SCALA-TEST-02`)
`src/test/scala` mirrors main packages; files `*Spec.scala`, `*Suite.scala` or `*Test.scala`. Integration tests live in an `it` subproject (`it/src/test/scala`) with `dependsOn(app % "test->test")`; the `IntegrationTest` configuration is deprecated.

## Style and doubles (`SCALA-TEST-03`, `-04`)
Arrange-act-assert; names as sentences (`test("rejects an expired token")`). Hand-written fakes of the port trait or `ZLayer` (in-memory `Ref`-backed repos); Mockito only for third-party Java types; never mock the domain.

## Integration (`SCALA-TEST-05`)
`testcontainers-scala` with a real Postgres or Kafka, fresh schema per suite via Flyway, truncate or roll back per test. Needs Docker: tag these tests so they can be skipped where Docker is absent, and say so in the claim.

## Coverage (`SCALA-TEST-06`, `-07`)
- `sbt -batch clean coverage test coverageReport` (the gate's `test` check). Always `clean` first. Add `coverageAggregate` for multi-project.
- Cobertura at `**/target/scala-*/coverage-report/cobertura.xml`. Class `filename` resolves against the `<sources>` entries; skip entries that are not directories (a bogus `--source` appears).
- New code at least 70%: loops intersects `<line number hits>` with the diff. Exclude generated code (`coverageExcludedPackages`, `coverageExcludedFiles` on Scala 3.3.4+; `$COVERAGE-OFF$` markers work on Scala 2 only). Scala.js and Native are unsupported on Scala 3.
- Prerequisites (`sbt-scoverage` plugin, thresholds): wave 0, see SKILL.md.

## Flakiness and time (`SCALA-TEST-09`)
Control time with `TestControl` (cats-effect) or `TestClock` (ZIO); no `Thread.sleep`; a timeout on every test (`munitTimeout`, `TestAspect.timeout`); no shared mutable global state across suites. sbt runs suites in parallel by default; forked tests run sequentially unless `Test / testForkedParallel := true`.

## Properties and mutation (`SCALA-TEST-08`)
ScalaCheck (`munit-scalacheck`, ZIO Test generators) for parsers, codecs, state machines and domain invariants. Stryker4s on the domain is an occasional job, not a gate.

## TDD loop (`SCALA-TEST-10`, `-11`)
`sbt ~testQuick`; `sbt "testOnly com.acme.OrderSpec"`; MUnit filter `testOnly *OrderSpec -- --tests=*rejects*`; `sbtn` or `sbt --client` for a warm server. The test guard recognises `sbt test|testOnly|testQuick|coverage`, `sbtn test`, `sbt --client test`, `mill __.test` and `scala-cli test` (with or without `-batch`).
Check your scope with `sbt -batch "<project>/test"` only; the gate owns the full repeated run. Never weaken a test to get green.
