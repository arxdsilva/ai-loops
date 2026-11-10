---
name: lang-scala
description: Use when writing, reviewing, testing or setting up Scala backend code in a loops run (the scala language pack) - toolchain, layout, race policy, coverage, idioms and footguns.
---

# Scala backend (the `scala` pack)

Rule book: `specs/languages/scala/RULES.md` (IDs `SCALA-<SEC>-NN`). Gate data: `specs/languages/scala/pack.json`. Layering, design limits and the race rule are neutral: `specs/ARCHITECTURE.md`, `specs/SOLID.md`, WORKFLOW §6. This skill only maps them onto Scala.

## When it applies
`loops lang detect` reports `scala` (a `build.sbt`, or a `build.mill` for the `mill` variant). `loops lang show scala` prints the effective pack, with the variant applied.

## Project rules first
Follow the project's own config over the defaults here: `build.sbt`, `project/plugins.sbt`, `.scalafmt.conf`, `.scalafix.conf`, `.editorconfig`, `project/build.properties`, the chosen effect style (cats-effect, ZIO, Play, Pekko) and framework conventions (`SCALA-FW-01/02`). When you follow a project rule over a loops rule, log it: `loops run override --rule "<loops rule>" --project "<project rule, file:line>" --note "<what you did>"`. Never mix effect systems or build tools to match this skill.

## Toolchain the gate runs
Agents run `loops gate`, or one check's command while iterating. The base build tool is sbt; commands below are sbt 1.x (the verified line; sbt 2 moves `target/`, see Coverage).

| Check | Command | Notes |
|---|---|---|
| `compile` (required) | `sbt -batch Test/compile` | warnings fail only if the project sets `-Werror` (`SCALA-TOOL-05`) |
| `scalafmt` (required when `.scalafmt.conf` exists) | `sbt -batch scalafmtCheckAll scalafmtSbtCheck` | check only; fix with `sbt scalafmtAll scalafmtSbt` |
| `scalafix` (optional, needs `.scalafix.conf`) | `sbt -batch "scalafixAll --check"` | one quoted argv element |
| `arch` | `loops arch` | regex scanner, `package-dir` resolver |
| `test` (required, repeated 3x) | `sbt -batch clean coverage test coverageReport` | race substitute and coverage in one |
| `coverage` (required) | internal | parses Cobertura, new code at least 70% |
| `dependency-check`, `osv` | `sbt -batch dependencyCheck`, `osv-scanner scan source -r .` | opt in (`--vuln`) |

Iteration loop, which is not the gate: `sbt ~testQuick`, `sbt "testOnly com.acme.OrderSpec"`, `sbtn` or `sbt --client` to keep the server warm (`SCALA-TEST-10`). Never run bare `sbt` interactively: always `-batch` in scripts. Missing plugin: an optional check skips with its `install` hint, a required one fails (D12). Post-edit hook: none for Scala (sbt has no single-file formatter), so run `scalafmtCheckAll` before reporting.

Mill variant (`build.mill`): `mill __.test.compile`, `mill __.test`, formatter `mill mill.scalalib.scalafmt.ScalafmtModule/checkFormatAll __.sources`. It has not been spot-run, so confirm with the project's own Mill setup. scala-cli (`project.scala`) is not gated: single-module scripts and spikes only.

## Race policy (WORKFLOW §6)
`race.mode` is `none`: the JVM has no general data-race detector, and ThreadSanitizer does not apply. The substitute is a required gate check, never skipped silently:
- The `test` check carries `repeat: 3`, so the gate runs the whole test argv three times.
- Suites run in parallel: `Test / parallelExecution` is true by default. If the build sets `Test / fork := true`, it must also set `Test / testForkedParallel := true`, or forked tests run sequentially and the substitute proves nothing. This is a wave 0 prerequisite (below).
- Any `Ref`, `STM`, `AtomicCell` or fiber code gets a concurrent test (`parTraverse` or `ZIO.foreachPar`, at least 100 iterations) and virtual time, never `Thread.sleep` (`SCALA-CONC-04`, `SCALA-TEST-09`).
- Do not weaken it: no `testOnly` shortcut in place of the gate, no retries to hide flakiness, no `Test / parallelExecution := false`. jcstress or Lincheck for lock-free code are review items, not gate checks. Details: `concurrency.md`.

## Coverage recipe
`sbt-scoverage`; always `clean` first, since stale instrumentation is the top failure (`SCALA-TEST-06`). The Cobertura file is `target/scala-<full ver>/coverage-report/cobertura.xml` (glob `target/scala-*/coverage-report/cobertura.xml`); `scoverage-report/` holds only `scoverage.xml` and HTML. Cobertura `<class filename>` is relative to a `<sources>` entry, and `<sources>` can hold a bogus `--source` entry that is not a directory. Multi-project builds: add `coverageAggregate`, and the gate merges every match. sbt 2 moves `target/` to `target/out/...`: re-verify the glob (`show coverageDataDir`) and log an override if the project is on sbt 2.

Project prerequisites the architect or `scala-engineer` adds in wave 0 (D13, logged as an override), with the exact snippet if the gate preflight fails:
- `project/plugins.sbt`: `sbt-scoverage`, `sbt-scalafmt`, and optionally `sbt-scalafix`, `sbt-tpolecat`, `sbt-dependency-check`, each pinned.
- `.scalafmt.conf` with `version = ...` and `runner.dialect`, committed (this is what makes `scalafmt` required).
- `build.sbt`: `Test / testForkedParallel := true` when `Test / fork := true`; `-Werror` in CI mode; `coverageFailOnMinimum := true` with `coverageMinimumStmtTotal := 70`; `coverageExcludedPackages` for generated code.
- `project/build.properties` (`sbt.version=`), JDK pin (`.sdkmanrc` or `.tool-versions`), `.gitignore` per `SCALA-REPO-12`.

## Run-scoped environment (D15)
Download caches stay shared: `COURSIER_CACHE` and the sbt boot directory (`shared_env`); relocating them re-fetches the world. Only temp is scoped: `JAVA_TOOL_OPTIONS=-Djava.io.tmpdir={tmp}`. `target/`, `project/target/`, `.bsp/`, `.bloop/`, `.metals/` cannot be redirected cheaply in sbt 1, so cleanup removes them when they did not exist at the baseline.

## Layers on Scala
```
domain/        sbt project, no dependsOn      case classes, enum ADTs, opaque types; no IO, no effect system
application/   dependsOn domain               use cases + consumer-owned port traits (F[_] or ZLayer)
adapters-http/ dependsOn application          http4s/Tapir/ZIO HTTP routes, codecs, DTOs
adapters-db/   dependsOn application          doobie/skunk repos, Flyway migrations
app/           dependsOn all                  composition root: Resource or ZLayer wiring, config
it/            dependsOn app % "test->test"   Testcontainers integration tests
```
The sbt `dependsOn` graph is the first enforcement (the compiler rejects a wrong-direction import); `loops arch` adds purity (no `cats.effect`, `zio`, `java.io`, `Instant.now`, `Await.result` in domain) and catches package cycles inside one project (`SCALA-ARCH-02/07/08/09`).

## Footguns
1. Blocking the compute pool (`IO(jdbc())`, `Await.result`): use `IO.blocking`, `ZIO.attemptBlocking`.
2. Dropped effects (a statement returning `IO`): `-Wnonunit-statement` and `-Wvalue-discard` as errors.
3. `ref.get.flatMap(v => ref.set(v + 1))`: use `update` or `modify`.
4. Fire-and-forget `.start` or `fork`: use `Supervisor`, `background`, `Scope`.
5. `Option.get`, `null`, `asInstanceOf`: match or `fold`; WartRemover in domain.
6. `%` instead of `%%`: the Scala binary suffix is missing.
7. Eager `Future` at class init: use lazy, cancelable `IO` or `ZIO`.
8. `sbt coverage test` without `clean`: stale instrumentation.
9. `io.circe.generic.auto._` on a big module: `derives Codec.AsObject` or `semiauto`.
10. Akka for a new project (BSL licence): use Apache Pekko.

## Load on demand
- `repo.md`: layout, sbt multi-project, pins, `.gitignore`, CI, releases.
- `architecture.md`: layers as sbt projects, effect style, DI, `loops arch` mapping.
- `idioms.md`: totality, immutability, Scala 3 features, errors, resources, compiler flags.
- `testing.md`: frameworks, TDD loop, coverage, doubles, Testcontainers, flakiness.
- `concurrency.md`: fiber models, shared state, the race substitute, async pitfalls.
- `security.md`: SQL, deserialisation, web, secrets, supply chain, SAST.
- `data-api.md`: DATA, API, OBS and OPS digests.
- `frameworks.md`: http4s/Tapir, ZIO, Play, Pekko, Akka licence.
