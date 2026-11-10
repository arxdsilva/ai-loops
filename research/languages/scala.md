# Scala: backend practices and loops gap analysis

| | |
|---|---|
| Pack id | `scala` |
| Current stable / LTS | Scala 3.10.0 (2026-10-12) / **3.9.x LTS** (3.9.0, 2026-09-03) and the older **3.3.x LTS** (3.3.9, 2026-10-28). Scala 2.13.18 (2025-11-24) is the last 2.x line. Support window: Scala LTS lines get long-term maintenance, 3.3 LTS is the conservative floor, 3.9 LTS is the new default for greenfield. |
| Dominant runtimes / frameworks | JVM (JDK 21 or 25 LTS); cats-effect + http4s + Tapir + doobie/skunk (Typelevel); ZIO + ZIO HTTP/Tapir + Quill/zio-jdbc; Play Framework (legacy/CRUD); Pekko (Apache fork of Akka 2.6) for actors and streams; Akka itself is BSL-licensed |
| As of | 2026-11 |
| Researcher | `B1-scala` |
| Verifier | (filled in by B2) |

> **Summary:** A strong Scala backend team builds with sbt (or Mill), compiles on Scala 3 with fatal warnings, formats with scalafmt, rewrites with scalafix, and keeps the domain pure by writing it as plain case classes and ADTs with effects only at the edges (tagless final or ZIO layers). There is no JVM race detector, so race safety comes from the effect system (immutable data, `Ref`, STM, fibers) plus a stress/repeat substitute. Coverage is scoverage (instrumenting compiler plugin, Cobertura XML). Biggest loops gaps: (1) the gate, coverage parser, test guard and post-edit hook are Go-only, so Scala needs a pack with sbt argv and a Cobertura parser; (2) `race.mode = none` needs a loops-enforced substitute, because `-race` does not exist; (3) the arch checker needs a Scala import parser (`package-dir` resolver, `import a.b.{C, D}`, `import a.*`, `import a._`) and the run cache needs `COURSIER_CACHE`/`SBT_*`/`target/` handling.

Rules follow [TAXONOMY.md](TAXONOMY.md), using the row format `ID | Rule | Why | Sev | Source | loops today | Enforce`. Gaps are judged against [LOOPS_BASELINE.md](LOOPS_BASELINE.md).

Terminology: **sbt** = default build tool; **Mill** = alternative; **scala-cli** = scripts and single-file tools only. Scala 3 vs 2.13: Scala 3 is the default for new code. 2.13 stays for libraries with unported macros (check with the Scala 3 migration guide). Scala 3 and 2.13 can interoperate (a Scala 3 project can depend on 2.13 artifacts via `.cross(CrossVersion.for3Use2_13)`).

## 1. REPO: repository and project management

Canonical tree for a service (sbt multi-project, hexagonal):

```
my-service/
├── build.sbt                    # root + subprojects (domain, application, adapters, app)
├── project/
│   ├── build.properties         # sbt.version=...   (toolchain pin)
│   ├── plugins.sbt              # sbt-scalafmt, sbt-scalafix, sbt-scoverage, sbt-tpolecat, sbt-native-packager, sbt-dependency-check
│   └── Dependencies.scala       # versions + modules in one place (or use a version catalog-like object)
├── .scalafmt.conf
├── .scalafix.conf
├── .jvmopts / .sbtopts          # -Xmx, -Xss for sbt itself
├── .java-version | .tool-versions   # JDK pin (sdkman .sdkmanrc, asdf, or mise)
├── domain/src/{main,test}/scala/com/acme/domain/...
├── application/src/{main,test}/scala/com/acme/application/...
├── adapters-http/src/...        # inbound
├── adapters-db/src/...          # outbound
├── app/src/{main,test}/scala/com/acme/app/Main.scala   # composition root
├── app/src/main/resources/{application.conf,logback.xml,db/migration/}
├── it/src/test/scala/...        # integration tests (Testcontainers), own subproject
├── .github/workflows/ci.yml
├── .gitignore  .gitattributes  .editorconfig  CODEOWNERS  LICENSE  README.md  CHANGELOG.md
```

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `SCALA-REPO-01` | Use the standard Maven-style layout per subproject: `src/main/scala`, `src/main/resources`, `src/test/scala`, `src/test/resources`; package names mirror directories (`com/acme/domain/Order.scala` has `package com.acme.domain`). [REPO-01] | sbt, Metals, scoverage and the loops arch resolver all assume it | MUST | [sbt: Directory structure](https://www.scala-sbt.org/1.x/docs/Directories.html) (sbt 1.x docs) | missing | review |
| `SCALA-REPO-02` | Model hexagonal layers as sbt subprojects with explicit `dependsOn` edges (`application dependsOn domain`, adapters `dependsOn application`, `app` aggregates everything); never let `domain` depend on an adapter project. [REPO-02] | The compiler then rejects a wrong-direction import because the classes are not on the classpath | MUST | [sbt: Multi-project builds](https://www.scala-sbt.org/1.x/docs/Multi-Project.html) | missing | arch |
| `SCALA-REPO-03` | Keep one root `build.sbt` with `lazy val` projects and a root project that `aggregate`s all (`publish / skip := true`); share settings through a `commonSettings` sequence or a custom `AutoPlugin` under `project/`. [REPO-02] | Avoids settings drift between subprojects | SHOULD | [sbt: Multi-project builds](https://www.scala-sbt.org/1.x/docs/Multi-Project.html) | missing | review |
| `SCALA-REPO-04` | Choose the build tool per repo: sbt by default (broadest plugin ecosystem, scoverage/scalafix/native-packager); Mill when the team wants a typed, fast build with `mill.main.build`; scala-cli only for scripts, spikes and single-module tools. Do not mix two build tools in one module graph. [REPO-02, REPO-03] | Tool sprawl breaks CI and Metals import | SHOULD | [Mill](https://mill-build.org/), [scala-cli](https://scala-cli.virtuslab.org/) | missing | review |
| `SCALA-REPO-05` | Bootstrap with `sbt new scala/scala3.g8` (or the company template) and add on day one: `.scalafmt.conf`, `project/build.properties`, `scalafix`, tpolecat flags, CI, `.gitignore`, license. [REPO-03] | Starter gives a Scala 3 compile target and sbt layout | SHOULD | [Scala 3: getting started](https://docs.scala-lang.org/scala3/getting-started.html) | missing | review |
| `SCALA-REPO-06` | Declare every dependency with `%%` (or `%%%` for Scala.js/Native), pin exact versions, centralise in `project/Dependencies.scala`, and commit no build output. sbt has no lockfile by default; for reproducibility use `sbt-lock`/`sbt-dependency-lock`-style tooling or a pinned Coursier mirror, and treat `dependencyTree` output as the review artifact. [REPO-04] | Scala artifacts carry the binary version suffix; `%` silently resolves the wrong artifact | MUST | [sbt: Library dependencies](https://www.scala-sbt.org/1.x/docs/Library-Dependencies.html) | missing | review |
| `SCALA-REPO-07` | Automate dependency updates with Scala Steward (the Scala-aware updater) or Renovate; give it `.scala-steward.conf` pins for LTS Scala and JDK. [REPO-04] | Generic Dependabot misreads `build.sbt` and Scala suffixes | SHOULD | [Scala Steward](https://github.com/scala-steward-org/scala-steward) | missing | review |
| `SCALA-REPO-08` | Submit the dependency graph to GitHub (`sbt-github-dependency-graph` / `scalacenter/sbt-dependency-submission`) so Dependabot alerts work on sbt builds. [REPO-04, TOOL-05] | sbt manifests are not parsed natively | SHOULD | [sbt-dependency-submission](https://github.com/scalacenter/sbt-dependency-submission) | missing | gate |
| `SCALA-REPO-09` | Remove unused dependencies with `sbt-explicit-dependencies` (`undeclaredCompileDependenciesTest`, `unusedCompileDependenciesTest`) or `-Wunused` plus review. [REPO-04] | Transitive dependency leaks break at upgrade | SHOULD | [sbt-explicit-dependencies](https://github.com/cb372/sbt-explicit-dependencies) | missing | gate |
| `SCALA-REPO-10` | Libraries: SemVer, `sbt-ci-release` for Maven Central (Sonatype Central) publishing from tags, PGP signing, `sbt-mima-plugin` (`mimaPreviousArtifacts`) for binary compatibility, publish for Scala 3 and 2.13 where needed. Services: release a Docker image, not a library. [REPO-05] | Binary incompatibility is the Scala ecosystem's biggest release hazard | MUST (libs) | [sbt-ci-release](https://github.com/sbt/sbt-ci-release), [MiMa](https://github.com/lightbend/mima) | missing | gate |
| `SCALA-REPO-11` | Pin sbt in `project/build.properties` (`sbt.version=2.0.12` or the 1.x line for plugins not yet ported), Scala in `scalaVersion`, and JDK in `.sdkmanrc`/`.tool-versions`; use `sbt` launcher via `sbtn`/coursier `cs setup`. [REPO-06] | sbt resolves the pinned version per project | MUST | [sbt releases](https://github.com/sbt/sbt/releases) (2.0.12 stable, 1.13.1 and 2.1.0-RC3 as of 2026-11) | missing | gate |
| `SCALA-REPO-12` | `.gitignore` must contain `target/`, `project/target/`, `project/project/`, `.bsp/`, `.bloop/`, `.metals/`, `.idea/`, `*.class`, `*.log`, `metals.sbt`, `.vscode/` (Metals-generated parts), `.scala-build/`, `.ammonite/`. [REPO-07] | Build, BSP and IDE output must never be committed | MUST | [Scala .gitignore template](https://github.com/github/gitignore/blob/main/Scala.gitignore) | missing | gate |
| `SCALA-REPO-13` | Commit `.scalafmt.conf` with a pinned `version = ...`, `runner.dialect`, and `.editorconfig` for charset/newline; run `scalafmtCheckAll` in pre-commit or CI. [REPO-07] | Formatter versions change output | MUST | [scalafmt configuration](https://scalameta.org/scalafmt/docs/configuration.html) | missing | gate |
| `SCALA-REPO-14` | CI stages: restore cache (Coursier + sbt + ivy) → `scalafmtCheckAll scalafmtSbtCheck` → `scalafixAll --check` → `compile` with `-Werror` → `coverage test coverageReport` → dependency scan → `Docker/publishLocal`; matrix over JDK 21/25 and Scala 3 LTS + latest. [REPO-08] | One pipeline shape per repo | SHOULD | [sbt: Testing](https://www.scala-sbt.org/1.x/docs/Testing.html), [setup-java cache sbt](https://github.com/actions/setup-java) | missing | review |
| `SCALA-REPO-15` | Use conventional commits; generate the changelog from tags (release-drafter or `sbt-ci-release`); sign commits where policy asks. [REPO-09] | Conventional commits drive loops release levels | SHOULD | [Conventional Commits](https://www.conventionalcommits.org/) | covered (AGENTS.md releases) | review |

## 2. TOOL: toolchain

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `SCALA-TOOL-01` | Greenfield services: Scala 3.9.x LTS (3.3.x LTS if a dependency lags); libraries cross-build 3.3 LTS plus 2.13. Treat Scala 3.10.x "Next" as opt-in. Migrate 2.13 to 3 with `-Xsource:3`, scalafix `MigrationRewrites` (Scala 3 migration plugin: `scala3-migrate`), then switch. [TOOL-01] | LTS gets long backports and binary stability | SHOULD | [Scala downloads](https://www.scala-lang.org/download/all.html) (3.10.0, 3.9.0 LTS, 3.3.9 LTS, 2.13.18; as of 2026-11), [Scala 3 migration guide](https://docs.scala-lang.org/scala3/guides/migration/compatibility-intro.html) | missing | review |
| `SCALA-TOOL-02` | Install toolchains with Coursier (`cs setup`) or sdkman/mise; use JDK 21 or 25 LTS (Temurin). Do not run on JDK < 17. [TOOL-01] | Virtual threads and modern GC need 21+ | SHOULD | [Coursier install](https://get-coursier.io/docs/cli-installation), [Adoptium](https://adoptium.net/) | missing | review |
| `SCALA-TOOL-03` | Format with scalafmt via `sbt-scalafmt`: check argv `sbt scalafmtCheckAll scalafmtSbtCheck`; fix `sbt scalafmtAll scalafmtSbt`; standalone `scalafmt --check`. [TOOL-02] | One formatter, check-only in the gate | MUST | [sbt-scalafmt](https://github.com/scalameta/sbt-scalafmt), [scalafmt](https://scalameta.org/scalafmt/) | go-only (gofmt in gate/hook) | gate |
| `SCALA-TOOL-04` | Lint and refactor with scalafix (`sbt "scalafixAll --check"`) using `OrganizeImports`, `RemoveUnused`, `DisableSyntax` rules; keep `.scalafix.conf` in repo. scalafix semantic rules need `semanticdbEnabled := true` (automatic on Scala 3). [TOOL-03] | Semantic rewrites beat regex | SHOULD | [scalafix](https://scalacenter.github.io/scalafix/) | missing | gate |
| `SCALA-TOOL-05` | Use `sbt-tpolecat` (or `typelevel/sbt-tpolecat` `tpolecatDevModeOptions`/`tpolecatCiModeOptions`) so CI mode adds `-Werror`; at minimum set `-deprecation -feature -unchecked -Wunused:all -Wvalue-discard -Wnonunit-statement -Wconf:...`. [TOOL-04] | Warnings must fail CI, not rot | MUST | [scalac options](https://docs.scala-lang.org/overviews/compiler-options/index.html) (`-Werror`/`-Xfatal-warnings`, `-Wunused:*`, `-Wvalue-discard`), [sbt-tpolecat](https://github.com/typelevel/sbt-tpolecat) | missing | gate |
| `SCALA-TOOL-06` | Scala 3 extra strictness: `-Wunused:all`, `-Wvalue-discard`, `-Wnonunit-statement`, `-source:future` on libraries you own, `-Yexplicit-nulls` only at Java boundaries, and `-language:strictEquality` for domain types. [TOOL-04] | Opts into the strict dialect | SHOULD | [Scala 3 compiler options](https://docs.scala-lang.org/scala3/guides/migration/options-intro.html), [strictEquality](https://docs.scala-lang.org/scala3/reference/contextual/multiversal-equality.html) | missing | gate |
| `SCALA-TOOL-07` | Add WartRemover (`Wart.Null`, `Wart.Var`, `Wart.Return`, `Wart.AsInstanceOf`, `Wart.TryPartial`, `Wart.OptionPartial`, `Wart.Throw`) as errors in domain/application projects only; on Scala 3 use the versions that support it. [TOOL-03] | Bans the partial functions that destroy totality | SHOULD | [WartRemover](https://www.wartremover.org/doc/warts.html) | missing | gate |
| `SCALA-TOOL-08` | Scan dependencies with OWASP `sbt-dependency-check` (`sbt dependencyCheck`; set `dependencyCheckFailBuildOnCVSS`) or OSV-Scanner against the dependency graph; generate an SBOM with `sbt-sbom` (CycloneDX: `sbt makeBom`). [TOOL-05] | CVEs in transitive jars | MUST | [sbt-dependency-check](https://github.com/albuch/sbt-dependency-check), [OSV-Scanner](https://google.github.io/osv-scanner/), [sbt-sbom](https://github.com/sbt/sbt-sbom) | go-only (govulncheck) | gate |
| `SCALA-TOOL-09` | Editor baseline: Metals (LSP, uses Bloop/BSP) with `metals.sbt` ignored; format-on-save must call the same `.scalafmt.conf`; disable Metals' "import build" prompt in CI. [TOOL-06] | Metals writes `.bloop/`, `.metals/`, `.bsp/` | SHOULD | [Metals](https://scalameta.org/metals/) | missing | review |

## 3. ARCH: architecture

Concrete tree mapping loops layers (sbt subprojects, package-per-layer):

```
domain/src/main/scala/com/acme/domain/            # case classes, ADTs, opaque types; no IO, no cats-effect/zio
application/src/main/scala/com/acme/application/  # use cases + ports (trait Orders[F[_]] / ZLayer service traits)
adapters-http/src/main/scala/com/acme/http/       # inbound: http4s / Tapir / ZIO HTTP routes, codecs, DTOs
adapters-db/src/main/scala/com/acme/db/           # outbound: doobie/skunk repositories, Flyway migrations
adapters-kafka/src/main/scala/com/acme/kafka/     # inbound/outbound messaging
app/src/main/scala/com/acme/app/Main.scala        # composition root: Resource / ZLayer wiring, config load
```

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `SCALA-ARCH-01` | Default to a modular monolith: one deployable, sbt subprojects per bounded context, hexagonal inside each. Split to services only on independent scaling/ownership. Use CQRS/event sourcing (Pekko Persistence, Akka-like, or Postgres event log with fs2) only where audit/replay is a requirement. [ARCH-01] | Scala's type system makes module seams cheap; distributed systems are not | SHOULD | [HEXAGONAL_ARCHITECTURE §1](../../specs/HEXAGONAL_ARCHITECTURE.md), [Pekko docs](https://pekko.apache.org/docs/pekko/current/) | covered (HEXAGONAL §1-§30) | review |
| `SCALA-ARCH-02` | The domain is plain Scala: `final case class`, `enum`/`sealed trait` ADTs, `opaque type` IDs, pure functions returning `Either`/`Option`. No `Future`, `IO`, `ZIO`, `Ref`, `Instant.now()`, `Random`, `scala.sys.env` in domain. [ARCH-06] | Purity makes domain tests trivial and race-free | MUST | [Scala 3 opaque types](https://docs.scala-lang.org/scala3/reference/other-new-features/opaques.html), [HEXAGONAL §3-§4](../../specs/HEXAGONAL_ARCHITECTURE.md) | go-only (arch forbids os/net-http/time.Now in Go) | arch |
| `SCALA-ARCH-03` | Ports are small traits owned by the consumer (application): `trait OrderRepo[F[_]] { def find(id: OrderId): F[Option[Order]] }` (tagless final) or `trait OrderRepo { def find(id: OrderId): IO[...] }` / `ZIO[Any, DbError, Option[Order]]` plus `ZLayer`. Adapters implement them. [ARCH-07] | Consumer-owned ports, same as the loops rule | MUST | [cats-effect: tagless final](https://typelevel.org/cats-effect/docs/typeclasses), [ZIO: service pattern](https://zio.dev/reference/service-pattern/) | covered (HEXAGONAL consumer-owned ports) | arch |
| `SCALA-ARCH-04` | Choose ONE effect style per service. Tagless final (`F[_]: Async`) gives testability and abstraction at the cost of verbosity; concrete `IO` is simpler and usually enough; ZIO uses its environment/error channel. Do not mix `Future`, `IO` and `ZIO` in application code; convert at the edge (`IO.fromFuture`, `ZIO.fromFuture`). [ARCH-05, CONC-01] | Mixed effects defeat cancellation and tests | MUST | [cats-effect docs](https://typelevel.org/cats-effect/docs/getting-started), [ZIO docs](https://zio.dev/overview/getting-started) | missing | review |
| `SCALA-ARCH-05` | Dependency injection: manual wiring in `Main` using `Resource[IO, A]` (cats-effect) or `ZLayer` (ZIO); no runtime DI container, no reflection DI (Guice/MacWire only if Play forces it; prefer compile-time DI with `MacWire` or Play compile-time DI). [ARCH-05] | Compile-time wiring is checked by scalac | MUST | [Resource](https://typelevel.org/cats-effect/docs/std/resource), [ZLayer](https://zio.dev/reference/di/) | covered (HEXAGONAL DI) | review |
| `SCALA-ARCH-06` | ZLayer vs sbt: one `ZLayer` per adapter project (`DbLive.layer`), assembled with `++`/`>>>` only in `app`; keep `*Live` (real) and `*Test` (fake) layers in different projects/configs. Tagless final equivalent: adapters expose `def make[F[_]: Async](cfg): Resource[F, OrderRepo[F]]`. [ARCH-02] | The sbt graph then equals the layer graph | SHOULD | [ZIO: ZLayer](https://zio.dev/reference/contextual/zlayer) | missing | arch |
| `SCALA-ARCH-07` | Visibility: use `private`, `private[pkg]`, `protected`, and `export`/`package-private` in Scala 3 to hide internals; mark all non-extension classes `final`; `sealed` for ADTs so `match` is exhaustive. Cycles between packages compile in one project, so enforce acyclicity with sbt subprojects, not packages. [ARCH-03, ARCH-04] | Scala does not forbid package cycles | MUST | [Scala style: access modifiers](https://docs.scala-lang.org/style/declarations.html) | missing | arch |
| `SCALA-ARCH-08` | Package-to-directory mapping and imports: `package a.b` lives in `a/b` by convention (not enforced); imports are `import a.b.C`, `import a.b.{C, D as E}`, `import a.b.*` (Scala 3) / `import a.b._` (Scala 2), `import a.b.{given, *}`. The arch checker must parse these forms and map them to subprojects via the `package-dir` resolver. [ARCH-03] | Scala packages need not match dirs, so the checker must read `package` clauses too | MUST | [Scala 3 syntax: imports](https://docs.scala-lang.org/scala3/reference/contextual/given-imports.html) | go-only (goparse) | arch |
| `SCALA-ARCH-09` | Enforce direction with (a) sbt `dependsOn` (compiler-enforced), (b) `sbt-sbtarch`/ArchUnit via `archunit` through JUnit for package rules, (c) scalafix `DisableSyntax`/`NoImports` or custom rule, (d) `loops arch`. Example ArchUnit: `noClasses().that().resideInAPackage("..domain..").should().dependOnClassesThat().resideInAnyPackage("..adapters..","..http4s..","..doobie..","cats.effect..","zio..")`. [ARCH-04] | Package rules inside one project need a tool | SHOULD | [ArchUnit](https://www.archunit.org/userguide/html/000_Index.html), [scalafix rules](https://scalacenter.github.io/scalafix/docs/rules/overview.html) | go-only | arch |
| `SCALA-ARCH-10` | Value objects are `final case class`/`opaque type` with smart constructors returning `Either[DomainError, A]` (or refined types via `iron`/`refined`); entities carry an ID and do not rely on `equals`; aggregates are immutable with `copy`; domain events are `sealed trait`/`enum`. [ARCH-01] | Illegal states become unrepresentable | SHOULD | [Iron](https://github.com/Iltotore/iron), [refined](https://github.com/fthomas/refined) | covered (HEXAGONAL domain/model, events) | review |
| `SCALA-ARCH-11` | Actor model: use Pekko Typed (`org.apache.pekko`), not Akka (BSL since 2.7 and later, Change Date 2029 for 2.10.x, commercial licence for production), for stateful per-entity concurrency, clustering and streams; otherwise prefer cats-effect/fs2 or ZIO. Never share mutable state across actors. [ARCH-01, CONC-01] | Akka's licence forbids free production use outside the Play grant | MUST | [Pekko](https://pekko.apache.org/), [Akka licence](https://doc.akka.io/libraries/akka-core/current/project/licenses.html) (2.10.24, BSL 1.1, change date 2029-10-09 as of 2026-11) | missing | review |
| `SCALA-ARCH-12` | Size heuristics: file ≤ 400 lines, function ≤ 40 lines, ≤ 5 parameters (use a case class), traits ≤ 7 methods, one top-level public type per file (companions together). [ARCH-08] | Matches loops SOLID limits in Scala units | SHOULD | [SOLID_GO §8](../../specs/SOLID_GO.md), [Scala style](https://docs.scala-lang.org/style/) | go-only | arch |

## 4. IDIOM: what to do and what to avoid

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `SCALA-IDIOM-01` | Model expected failures as values: `Either[DomainError, A]` with a `sealed` error ADT; reserve exceptions for bugs and Java interop. Effect error channel: `IO` with `MonadError`, ZIO's `E`. Never `throw` in domain, never `catch Throwable` (use `NonFatal`). [IDIOM-01] | Total functions, exhaustive matches | MUST | [Scala style](https://docs.scala-lang.org/style/), [GO_BEST §8-9](../../specs/GO_BEST_PRACTICES.md) | go-only | review |
| `SCALA-IDIOM-02` | Never use `null`; use `Option`. Wrap Java returns at the boundary with `Option(x)`. Use `-Yexplicit-nulls` for Java-heavy modules. Never call `.get` on `Option`/`Try`/`Either`. [IDIOM-02] | NPEs and partial functions | MUST | [Explicit nulls](https://docs.scala-lang.org/scala3/reference/experimental/explicit-nulls.html), [WartRemover](https://www.wartremover.org/doc/warts.html) | missing | gate |
| `SCALA-IDIOM-03` | Immutable by default: `val` over `var`, immutable collections (`scala.collection.immutable`), `final case class` + `copy`; no `var` or `return`. Mutable state only inside `Ref`/`AtomicReference`/`TrieMap` at the edge. [IDIOM-03] | Removes shared-mutable-state races | MUST | [Scala style: immutability](https://docs.scala-lang.org/style/) | missing | gate |
| `SCALA-IDIOM-04` | Scala 3: `enum` for ADTs, `given`/`using` instead of implicits, `extension` methods, `opaque type`, union/intersection types with restraint; Scala 2 code: avoid implicit conversions. Use exhaustive pattern matching and let `-Wunused`/exhaustivity warnings fail the build. [IDIOM-04] | Idiomatic and safer | SHOULD | [Scala 3 reference](https://docs.scala-lang.org/scala3/reference/), [contextual abstractions](https://docs.scala-lang.org/scala3/reference/contextual/index.html) | missing | review |
| `SCALA-IDIOM-05` | Acquire resources with `Resource.make`/`Resource.fromAutoCloseable` (cats-effect), `ZIO.acquireRelease`/`ZLayer.scoped` (ZIO), or `scala.util.Using` for synchronous code. Never call `close()` manually in finally outside those. [IDIOM-05] | Leaked connections/fibers | MUST | [Resource](https://typelevel.org/cats-effect/docs/std/resource), [ZIO Scope](https://zio.dev/reference/resource/scope), [scala.util.Using](https://scala-lang.org/api/3.x/scala/util/Using$.html) | missing | review |
| `SCALA-IDIOM-06` | Equality: `case class` auto-generates `equals`; do not override on mutable fields; enable `-language:strictEquality` and `derives CanEqual`; use `cats.Eq` for typed comparison. Time: `java.time` only; inject `Clock[F]`/`zio.Clock`; store `Instant` UTC; never `java.util.Date`. [IDIOM-06] | Multiversal equality catches `Int == String` | SHOULD | [Multiversal equality](https://docs.scala-lang.org/scala3/reference/contextual/multiversal-equality.html), [Clock](https://typelevel.org/cats-effect/docs/typeclasses/clock) | covered (consistency skill: clock injection) | review |
| `SCALA-IDIOM-07` | Naming: `UpperCamelCase` types, `lowerCamelCase` values, `UPPER_SNAKE` only for Java-style constants, packages lowercase; omit `get` prefixes; use parameterless methods without parens for accessors; no symbolic method names except DSL operators. Format with scalafmt; two-space indent (Scala 3 braces optional: pick `significantIndentation` consistently). [IDIOM-07] | Official style guide | SHOULD | [Scala Style Guide](https://docs.scala-lang.org/style/naming-conventions.html) | missing | gate |
| `SCALA-IDIOM-08` | Metaprogramming: prefer `derives`, `inline` and Mirror-based derivation over reflection; macros only in libraries; no runtime reflection and no `scala.reflect` in services. Avoid annotation-heavy frameworks. [IDIOM-08] | Compile-time derivation is checked and AOT-friendly | AVOID | [Scala 3 macros](https://docs.scala-lang.org/scala3/guides/macros/index.html) | missing | review |
| `SCALA-IDIOM-09` | Avoid implicit conversions and `implicit` parameters with default types such as `ExecutionContext` in public APIs; pass them explicitly or via `given` with a name. [IDIOM-09] | Implicit resolution surprises | AVOID | [Scala 3: conversions](https://docs.scala-lang.org/scala3/reference/contextual/conversions.html) | missing | gate |
| `SCALA-IDIOM-10` | Avoid `Await.result`, `Future.apply` in hot paths, `Thread.sleep`, and `scala.concurrent.blocking` in effect code. Use `IO.sleep`/`ZIO.sleep`. [IDIOM-09, CONC-05] | Blocks the compute pool | AVOID | [cats-effect: thread model](https://typelevel.org/cats-effect/docs/thread-model) | missing | gate |
| `SCALA-IDIOM-11` | Avoid `Any`/`AnyRef`-typed collections, `asInstanceOf`, `isInstanceOf`, `Try` as a domain error type, and `@SuppressWarnings` without a ticket. [IDIOM-09] | Type erasure and warnings suppression | AVOID | [WartRemover warts](https://www.wartremover.org/doc/warts.html) | missing | gate |
| `SCALA-IDIOM-12` | Avoid parallel collections and deep `for`-comprehension nests in application logic; use `traverse`/`parTraverse`/`ZIO.foreachPar`. Do not use `.par`. [IDIOM-06] | Hidden thread pools | AVOID | [cats traverse](https://typelevel.org/cats/typeclasses/traverse.html) | missing | review |

## 5. CONC: concurrency and the `-race` equivalent

**Loops race policy: `mode = none`. There is no general data-race detector for the JVM.** Substitute enforced by loops: run the test task with suites in parallel (`Test / parallelExecution` is already true by default; if `Test / fork := true`, also set `Test / testForkedParallel := true`, since forked tests run sequentially otherwise) and have loops repeat the whole test command 3 times itself (three sequential invocations of the same argv, e.g. `sbt -batch 'set Test / testForkedParallel := true' test`) (`-repeat` in ZIO Test via `TestAspect.nonFlaky`/`repeats`; cats-effect via `TestControl`), with randomised order where the framework allows it. jcstress and Lincheck are optional per-project tools, not gate checks.

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `SCALA-CONC-01` | Model: JVM threads (platform and Loom virtual threads on JDK 21+), cats-effect fibers on a work-stealing `IORuntime`, ZIO fibers on ZIO's executor, Pekko actors; Scala `Future` is eager and uncancellable and is the legacy option. [CONC-01] | Each model has its own cancellation and blocking rules | MUST (know) | [cats-effect concurrency](https://typelevel.org/cats-effect/docs/concepts), [ZIO fibers](https://zio.dev/reference/fiber/) | missing | review |
| `SCALA-CONC-02` | Share state only via `Ref`/`Ref.Make`, `AtomicCell`, `Semaphore`, `Queue`, `Deferred` (cats-effect), `Ref`/`Queue`/`Promise`/`STM`/`TRef` (ZIO), or message passing (Pekko). Never mutate `var`/`mutable.Map` from fibers; do not read-modify-write with `get` then `set` (use `update`/`modify`). [CONC-02] | Fibers run on multiple threads | MUST | [Ref](https://typelevel.org/cats-effect/docs/std/ref), [ZIO STM](https://zio.dev/reference/stm/) | go-only (GO_BEST §11-12) | review |
| `SCALA-CONC-03` | Race detection: none for JVM. Static aids: `-Wnonunit-statement` (finds dropped effects), scalafix `DisableSyntax`; dynamic aids: jcstress (OpenJDK) for lock-free code, Lincheck for linearizability, ThreadSanitizer is not for the JVM; effect-system aids: cats-effect `TestControl` (virtual-time mock runtime, not deterministic: it picks among pending fibers at random), ZIO `TestClock`. [CONC-03] | State the limitation honestly | MUST | [jcstress](https://github.com/openjdk/jcstress), [Lincheck](https://github.com/JetBrains/lincheck), [TestControl](https://typelevel.org/cats-effect/docs/core/test-runtime) | go-only | review |
| `SCALA-CONC-04` | Loops substitute for race: (1) tests run with suite parallelism (default; with `fork := true` add `Test / testForkedParallel := true`), (2) the gate itself invokes the test command 3 times (loops loop, not an sbt feature), (3) any `Ref`/`STM`/fiber code has a concurrent test using `parTraverse`/`ZIO.foreachPar` with ≥100 iterations, (4) tests use virtual time not `sleep`. [CONC-04] | Probabilistic but cheap | MUST | [ZIO TestAspect](https://zio.dev/reference/test/aspects/) (`nonFlaky`, `repeats`, `timeout`) | go-only (race-guard) | gate |
| `SCALA-CONC-05` | Async pitfalls: never block a compute thread (wrap in `IO.blocking`/`ZIO.attemptBlocking`), always set timeouts (`.timeout`), make cancellation safe (`uncancelable`/`bracket`), bound queues for backpressure, and never fire-and-forget `.start`/`fork` without supervision (use `Supervisor`/`Scope`). Unhandled errors in forked fibers are silent, so join or log them. [CONC-05] | Fiber leaks | MUST | [cats-effect: Dispatcher/Supervisor](https://typelevel.org/cats-effect/docs/std/supervisor), [ZIO: fork](https://zio.dev/reference/fiber/#fork) | go-only (GO_BEST §10) | review |
| `SCALA-CONC-06` | Streams: use fs2 or ZIO Streams or Pekko Streams with bounded buffers; `parEvalMap(n)`, `mapAsync(n)`; handle backpressure by design. [CONC-05] | Unbounded memory | SHOULD | [fs2](https://fs2.io/), [ZIO Streams](https://zio.dev/reference/stream/) | missing | review |
| `SCALA-CONC-07` | Background work: dedicated `Resource`-managed worker fibers, `Supervisor`, schedule with `fs2.Stream.awakeEvery` or `Schedule`; graceful shutdown on SIGTERM by running `Main` as `IOApp`/`ZIOAppDefault`, which cancels the main fiber and runs finalizers. Use Quartz/Pekko Scheduler only if persistence of schedules is needed. [CONC-06] | Finalizers run on cancel | MUST | [IOApp](https://typelevel.org/cats-effect/docs/core/io-runtime-config), [ZIOApp](https://zio.dev/reference/core/zioapp) | covered (jobs-and-state-machines) | review |

## 6. TEST: testing

**Coverage recipe:** scoverage via `sbt-scoverage`; format Cobertura XML (plus `scoverage.xml`); argv `sbt -batch clean coverage test coverageReport` (add `coverageAggregate` for multi-project); output `target/scala-<full scala version>/coverage-report/cobertura.xml` per subproject (aggregate: root `target/scala-<ver>/coverage-report/`; `scoverage.xml` and HTML go to `scoverage-report/`); statement + branch coverage; new-code coverage: loops computes it by intersecting changed lines (git diff against base) with `<class filename=... ><line number hits>` entries; `diff-cover` also reads Cobertura XML. On sbt 2 the target dir moves, so resolve by `show coverageDataDir`/glob.

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `SCALA-TEST-01` | Unit framework: MUnit by default (simple, JUnit-compatible, `CatsEffectSuite` via munit-cats-effect, ScalaCheck via `ScalaCheckSuite`); ScalaTest for existing codebases or BDD style; weaver-test for cats-effect-native, parallel-by-default suites; ZIO Test for ZIO. Pick one per project. [TEST-01] | One style, one runner | SHOULD | [MUnit](https://scalameta.org/munit/), [ScalaTest](https://www.scalatest.org/), [weaver-test](https://disneystreaming.github.io/weaver-test/), [ZIO Test](https://zio.dev/reference/test/) | go-only (skills/testing) | review |
| `SCALA-TEST-02` | Layout: `src/test/scala` mirrors main packages; unit tests `*Spec.scala`, `*Suite.scala` or `*Test.scala`; integration tests in an `it` subproject (`it/src/test/scala`, `IntegrationTest` config is deprecated, so use a separate project with `dependsOn(app % "test->test")`). Test globs: `**/src/test/scala/**/*.scala`, `**/*Spec.scala`, `**/*Suite.scala`, `**/*Test.scala`, `**/*Spec.sc`. [TEST-02] | Loops needs to know what is a test | MUST | [sbt: Testing](https://www.scala-sbt.org/1.x/docs/Testing.html) | go-only (`*_test.go`) | gate |
| `SCALA-TEST-03` | Style: arrange-act-assert, names as sentences (`test("rejects an expired token")`), parametrise with loops/`List.foreach` in MUnit, `forAll`/`TableDrivenPropertyChecks` in ScalaTest, `Gen` in ZIO Test. [TEST-03] | Readable failures | SHOULD | [MUnit tests](https://scalameta.org/munit/docs/tests.html) | go-only (table-driven) | review |
| `SCALA-TEST-04` | Test doubles: hand-written fakes of the port trait/`ZLayer` (in-memory `Ref`-backed repos); Mockito-scala/MockitoSugar only for third-party Java types; never mock the domain. [TEST-04] | Fakes survive refactors | SHOULD | [HEXAGONAL testing](../../specs/HEXAGONAL_ARCHITECTURE.md), [ZIO: testing effects](https://zio.dev/reference/test/) | covered (HEXAGONAL, testing skill) | review |
| `SCALA-TEST-05` | Integration tests use `testcontainers-scala` (`TestContainerForAll` / `munit-testcontainers`) with a real Postgres/Kafka and a fresh schema per suite via Flyway; clean per test with transactions or truncation. Requires Docker: gate marks these tests so they can be skipped when Docker is unavailable. [TEST-05] | Real SQL dialects | SHOULD | [testcontainers-scala](https://github.com/testcontainers/testcontainers-scala) | missing | gate |
| `SCALA-TEST-06` | Coverage: `sbt -batch clean coverage test coverageReport`; set `coverageFailOnMinimum := true`, `coverageMinimumStmtTotal := 70`, `coverageMinimumBranchTotal := 60`; exclude generated code with `coverageExcludedPackages := "<empty>;.*\\.generated\\..*;.*BuildInfo.*"` and `coverageExcludedFiles` (Scala 3.3.4+ / 3.4.2+; comment markers `$COVERAGE-OFF$` work only on Scala 2). Always `clean` first, because stale instrumentation is the top failure. Scala.js/Native: not supported on Scala 3. [TEST-06] | Instrumented classes are slow and must not ship | MUST | [sbt-scoverage](https://github.com/scoverage/sbt-scoverage) (v2.4.4 as of 2026-11, Scala 3.2+) | go-only (gate coverage profile) | gate |
| `SCALA-TEST-07` | Coverage of new code ≥70%: loops parses `cobertura.xml` (`<class filename>`, `<line number hits>`) and intersects with `git diff -U0 <base>...HEAD`; `diff-cover cobertura.xml --compare-branch=<base> --fail-under=70` is an equivalent external check. Map `filename` against the `<sources>` element. [TEST-07] | Format differs from Go profile | MUST | [diff-cover](https://github.com/Bachmann1234/diff_cover), [Cobertura format](https://github.com/cobertura/web/blob/master/htdocs/xml/coverage-04.dtd) | go-only (gate/coverage.go) | gate |
| `SCALA-TEST-08` | Property tests with ScalaCheck (`munit-scalacheck`, `scalatestplus-scalacheck`, `zio-test` generators) for parsers, codecs, state machines and domain invariants; mutation testing with Stryker4s (`sbt stryker`) on the domain as an occasional job, not a gate. [TEST-08] | Finds boundary bugs | SHOULD | [ScalaCheck](https://scalacheck.org/), [Stryker4s](https://stryker-mutator.io/docs/stryker4s/getting-started/) | go-only (fuzz) | review |
| `SCALA-TEST-09` | Flakiness: control time with `TestControl` (cats-effect) or `TestClock` (ZIO); avoid `Thread.sleep`; give every test a timeout (`munitTimeout`, `TestAspect.timeout`); share no mutable global state between suites; know that sbt runs suites in parallel by default (`Test / parallelExecution` is true); forked tests (`Test / fork := true`) run sequentially unless `Test / testForkedParallel := true`. [TEST-09] | Deterministic tests | MUST | [TestControl](https://typelevel.org/cats-effect/docs/core/test-runtime), [sbt: Testing](https://www.scala-sbt.org/1.x/docs/Testing.html) | covered (consistency) | gate |
| `SCALA-TEST-10` | TDD loop: `sbt ~testQuick` (watch + failed/changed only), `sbt "testOnly com.acme.OrderSpec"`, `sbt "testOnly *OrderSpec -- --tests=*rejects*"` (MUnit filter `-- --tests=`), keep the sbt server running with `sbtn`/`sbt --client` for 10x faster startup. [TEST-10] | Fast feedback | SHOULD | [sbt: testQuick/watch](https://www.scala-sbt.org/1.x/docs/Testing.html) | missing | review |
| `SCALA-TEST-11` | The test guard must recognise as test commands: `sbt test`, `sbt "testOnly ..."`, `sbt testQuick`, `sbt coverage test`, `sbtn test`, `sbt --client test`, `mill <mod>.test`, `scala-cli test`. [TEST-10] | `race-guard` needs prefixes | MUST | [Mill testing](https://mill-build.org/mill/scalalib/testing.html), [scala-cli test](https://scala-cli.virtuslab.org/docs/commands/test) | go-only | hook |

## 7. PERF: performance and profiling

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `SCALA-PERF-01` | Benchmark with JMH via `sbt-jmh` (`sbt "jmh:run -i 5 -wi 5 -f 1 .*Bench.*"`), in a separate `bench` subproject; compare runs with the same JDK and flags; never time with `System.nanoTime` loops. [PERF-01] | JIT warm-up | SHOULD | [sbt-jmh](https://github.com/sbt/sbt-jmh), [JMH](https://github.com/openjdk/jmh) | go-only (benchstat) | review |
| `SCALA-PERF-02` | Profile with async-profiler (CPU/alloc flame graphs), JFR (`-XX:StartFlightRecording`), VisualVM; cats-effect has fiber dumps (`IORuntime` tracing, `-Dcats.effect.tracing.mode=full`) and ZIO has fiber dump/`Supervisor` tooling. [PERF-02] | Find hot paths and fiber leaks | SHOULD | [async-profiler](https://github.com/async-profiler/async-profiler), [cats-effect tracing](https://typelevel.org/cats-effect/docs/tracing) | go-only (pprof) | review |
| `SCALA-PERF-03` | Tuning: use G1 or ZGC on JDK 21+, `-XX:MaxRAMPercentage=75`, size `IORuntime` compute pool to cores, run blocking IO on the blocking pool, consider virtual threads for blocking JDBC; GraalVM native image only with reflection configs (see OPS). [PERF-03] | Container-aware memory | SHOULD | [JDK GC tuning](https://docs.oracle.com/en/java/javase/21/gctuning/) | missing | review |
| `SCALA-PERF-04` | Hot-path mistakes: boxing in generic code (use `@specialized` or arrays), `List` indexing and `++` in loops (use `Vector`/`ArrayBuffer`), repeated `.toList`, intermediate collections (use `.view` or `iterator`), regex compile per call, `Seq.contains` on large `Seq` (use `Set`), `Future` flatMap chains allocating per step, string `+` in loops (use `StringBuilder`). [PERF-04] | Common Scala slowdowns | SHOULD | [Scala collections performance](https://docs.scala-lang.org/overviews/collections-2.13/performance-characteristics.html) | go-only | review |

## 8. SEC: security

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `SCALA-SEC-01` | SQL: never build SQL with string interpolation `s"..."` or `+`; use doobie `fr"..."`/`sql"..."` interpolators (parameters become bind variables, `Fragment.const` only for trusted identifiers), skunk `sql"..."` with codecs, Slick `sql"..."`/`#$` only for trusted identifiers, Quill quotations. Commands: use `ProcessBuilder(Seq(...))`, never a single shell string. [SEC-01] | Typed interpolators parameterise | MUST | [OWASP SQL Injection Prevention](https://cheatsheetseries.owasp.org/cheatsheets/SQL_Injection_Prevention_Cheat_Sheet.html), [doobie fragments](https://typelevel.org/doobie/docs/05-Parameterized.html) | go-only (GO_BEST §38) | gate |
| `SCALA-SEC-02` | Deserialisation: never use Java serialization (`ObjectInputStream`), Kryo with open class registration, or Jackson default typing; use circe/jsoniter-scala/zio-json/Tapir codecs with sealed ADTs; configure XML parsers against XXE (`disallow-doctype-decl`). [SEC-02] | JVM deserialisation gadgets | MUST | [OWASP Deserialization](https://cheatsheetseries.owasp.org/cheatsheets/Deserialization_Cheat_Sheet.html), [XXE](https://cheatsheetseries.owasp.org/cheatsheets/XML_External_Entity_Prevention_Cheat_Sheet.html) | go-only | gate |
| `SCALA-SEC-03` | Web: SSRF allow-list outbound URLs, normalise paths (`os.Path`/`java.nio.file.Path.normalize` then check prefix), CORS explicit origins (http4s `CORS` middleware, Play filters), CSRF tokens for cookie auth (Play CSRF filter, http4s `CSRF`), security headers middleware. [SEC-03] | OWASP A01/A10 | MUST | [OWASP SSRF](https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html), [http4s CORS](https://http4s.org/v0.23/docs/cors.html) | missing | review |
| `SCALA-SEC-04` | Secrets from env or vault, loaded with Ciris/PureConfig into a case class with a `Secret` wrapper (redacting `toString`); no secrets in `application.conf`; passwords hashed with Argon2id/bcrypt (BouncyCastle or Spring Security Crypto), randomness via `SecureRandom`, TLS via JDK defaults; do not use `MD5`/`SHA1`/`java.util.Random`/DES. [SEC-04] | Standard crypto hygiene | MUST | [OWASP Password Storage](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html), [Ciris](https://cir.is/) | go-only | gate |
| `SCALA-SEC-05` | Validate at the boundary: decode DTOs with circe/zio-json/Tapir, then convert into domain types via smart constructors (`Either`) or refined/iron types; reject unknown fields where appropriate (`Configuration.default.withStrictDecoding`). [SEC-05] | Parse, don't validate | MUST | [Tapir validation](https://tapir.softwaremill.com/en/latest/endpoint/validation.html), [circe](https://circe.github.io/circe/) | go-only (GO_BEST §43) | review |
| `SCALA-SEC-06` | Supply chain: trust only Maven Central/Sonatype and an internal mirror; Coursier verifies checksums; enable sbt `updateOptions` and `sbt-dependency-check`; pin sbt plugin versions in `project/plugins.sbt` since plugins run arbitrary code at build time; review Scala Steward PRs. [SEC-06] | sbt plugins execute code | MUST | [sbt: Resolvers](https://www.scala-sbt.org/1.x/docs/Resolvers.html), [OWASP Dependency-Check](https://owasp.org/www-project-dependency-check/) | go-only (GO_BEST §48-49) | gate |
| `SCALA-SEC-07` | SAST: Semgrep with the Scala ruleset; scalac security via WartRemover; `sbt-scala-security`/find-sec-bugs works on bytecode (`sbt-findbugs` scala support is limited), so combine with CodeQL (Scala via Java/Kotlin extractor limited) and Semgrep. [SEC-07] | No dominant Scala SAST | SHOULD | [Semgrep Scala](https://semgrep.dev/docs/supported-languages/), [FindSecBugs](https://find-sec-bugs.github.io/) | missing | gate |

## 9. DATA: persistence

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `SCALA-DATA-01` | Access style: doobie (JDBC, pure SQL with typechecking, cats-effect), skunk (native Postgres, non-blocking, Postgres only), Slick (FRM, mature, Future-based, optional effect wrappers), Quill (compile-time SQL, ZIO/Scala 3 support via quill-jdbc-zio), ZIO-JDBC/zio-sql. Prefer SQL-first (doobie/skunk); avoid heavy ORMs. [DATA-01] | SQL stays visible and checked | SHOULD | [doobie](https://typelevel.org/doobie/), [skunk](https://typelevel.org/skunk/), [Slick](https://scala-slick.org/), [Quill](https://zio.dev/zio-quill/) | missing | review |
| `SCALA-DATA-02` | Migrations: Flyway (`sbt-flyway` or `flyway-core` at startup on a separate migrate job) with versioned SQL `V<n>__name.sql`; forward-only; expand-contract for zero downtime; never auto-migrate on every replica without a lock. [DATA-02] | Safe deploys | MUST | [Flyway](https://documentation.red-gate.com/fd/migrations-271585107.html), [Liquibase](https://docs.liquibase.com/) | covered (consistency migrations) | review |
| `SCALA-DATA-03` | Transactions at the use-case boundary: doobie `.transact(xa)` on a whole `ConnectionIO`; ZIO `ZIO.serviceWithZIO`+`transaction`; do not open transactions in adapters per call; the port exposes a unit-of-work or uses `ConnectionIO` inside the adapter only. Choose isolation per query; retry on serialization failure. [DATA-03] | Atomicity in one place | MUST | [doobie transactions](https://typelevel.org/doobie/docs/14-Managing-Connections.html) | covered (HEXAGONAL persistence) | review |
| `SCALA-DATA-04` | Avoid N+1 (Slick `join`/`groupBy`; doobie one query with `IN`/`unnest`); no lazy loading exists in doobie/skunk, which is an advantage; Hibernate/JPA from Scala is an anti-pattern. [DATA-04] | Predictable query counts | SHOULD | [doobie](https://typelevel.org/doobie/docs/07-Updating.html) | missing | review |
| `SCALA-DATA-05` | Pool with HikariCP (`HikariTransactor`), size the pool, set `connectionTimeout`/`maxLifetime`, bound the connect/blocking EC (`ExecutionContexts.fixedThreadPool`), use skunk `Session.pooled`. [DATA-05] | Pool exhaustion | MUST | [doobie HikariTransactor](https://typelevel.org/doobie/docs/14-Managing-Connections.html), [HikariCP](https://github.com/brettwooldridge/HikariCP) | missing | review |

## 10. API: APIs and integration

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `SCALA-API-01` | HTTP: http4s (cats-effect, middleware, Ember server) with Tapir endpoints is the Typelevel default; ZIO HTTP for ZIO; Play for full-stack/legacy; Pekko HTTP for actor stacks. Define routes in inbound adapters only. [API-01] | Frameworks are adapters | SHOULD | [http4s](https://http4s.org/), [ZIO HTTP](https://zio.dev/zio-http/), [Play](https://www.playframework.com/), [Pekko HTTP](https://pekko.apache.org/docs/pekko-http/current/) | missing | arch |
| `SCALA-API-02` | Contracts: Tapir endpoints as the single source for server, client and OpenAPI (`SwaggerInterpreter`); or OpenAPI-first with sbt-openapi-generator / `sbt-guardrail`; gRPC via ScalaPB / fs2-grpc / zio-grpc; GraphQL via Caliban; version in the path or media type. [API-02] | One definition, no drift | SHOULD | [Tapir](https://tapir.softwaremill.com/), [ScalaPB](https://scalapb.github.io/), [Caliban](https://ghostdogpr.github.io/caliban/) | missing | review |
| `SCALA-API-03` | Map domain errors to HTTP in the adapter with an RFC 9457 `application/problem+json` body (Tapir `errorOut` oneOf); never leak stack traces; 400 for decode failures, 404/409/422 per taxonomy. [API-03] | Stable error contract | MUST | [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457), [Tapir errors](https://tapir.softwaremill.com/en/latest/server/errors.html) | covered (HEXAGONAL error taxonomy) | review |
| `SCALA-API-04` | Resilience: timeouts on every call; retries with backoff (cats-retry, `ZIO.retry(Schedule.exponential)`, fs2 retry); circuit breaker (Pekko `CircuitBreaker`, resilience4j, cats-effect `Hotswap`-based); idempotency keys for POST; rate limit with `http4s-throttle` or a gateway. [API-04] | Failure containment | MUST | [cats-retry](https://cb372.github.io/cats-retry/), [ZIO Schedule](https://zio.dev/reference/schedule/) | covered (consistency idempotency) | review |
| `SCALA-API-05` | Messaging: fs2-kafka, zio-kafka, Pekko Connectors Kafka, or `fs2-rabbit`; commit offsets after processing (at-least-once) and make handlers idempotent; use the outbox pattern in the same DB transaction. [API-05] | Duplicates are normal | MUST | [fs2-kafka](https://fd4s.github.io/fs2-kafka/), [zio-kafka](https://zio.dev/zio-kafka/) | covered (jobs skill: outbox, leases) | review |
| `SCALA-API-06` | HTTP clients: http4s Ember/JDK client, sttp client4 (backend-agnostic), ZIO HTTP client; one shared client `Resource` per process with a bounded pool; propagate deadlines via `.timeout` and trace headers. [API-06] | Connection reuse | SHOULD | [sttp](https://sttp.softwaremill.com/), [http4s client](https://http4s.org/v0.23/docs/client.html) | go-only (GO_BEST §10) | review |

## 11. OBS: observability

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `SCALA-OBS-01` | Logging: log4cats (cats-effect, `SelfAwareStructuredLogger`) or `zio-logging` over SLF4J + Logback with `logstash-logback-encoder` JSON; put request/trace IDs in MDC or `StructuredLogger` context; never log secrets or PII. Never use `println`. [OBS-01] | Structured, correlated logs | MUST | [log4cats](https://typelevel.org/log4cats/), [zio-logging](https://zio.dev/zio-logging/) | go-only (GO_BEST §24) | gate |
| `SCALA-OBS-02` | Metrics: Micrometer or Prometheus client via http4s-prometheus-metrics/`zio-metrics-connectors`; names `snake_case`, base units, bounded label cardinality. [OBS-02] | Prometheus conventions | SHOULD | [Prometheus naming](https://prometheus.io/docs/practices/naming/), [http4s metrics](https://github.com/http4s/http4s-prometheus-metrics) | missing | review |
| `SCALA-OBS-03` | Tracing: otel4s (OpenTelemetry for cats-effect, Local-based propagation) or the OpenTelemetry Java agent (`-javaagent`) for auto-instrumentation; zio-opentelemetry for ZIO; Scala `Future`/fiber context propagation needs the library, not the plain agent alone. [OBS-03] | Fibers move across threads | SHOULD | [otel4s](https://typelevel.org/otel4s/), [OTel Java](https://opentelemetry.io/docs/languages/java/) | missing | review |
| `SCALA-OBS-04` | Health: expose `/healthz` (liveness, no dependencies) and `/readyz` (DB ping with timeout) from http4s/Tapir/ZIO HTTP routes; Play/Pekko have management modules (`pekko-management` health checks). [OBS-04] | Kubernetes probes | MUST | [Pekko Management](https://pekko.apache.org/docs/pekko-management/current/healthchecks.html) | missing | review |

## 12. OPS: configuration, packaging and deployment

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `SCALA-OPS-01` | Config: Typesafe Config (HOCON) + PureConfig (`derives ConfigReader`) or Ciris/zio-config into typed case classes at startup; env overrides via `${?ENV}`; fail fast on missing values; secrets never committed. [OPS-01] | 12-factor | MUST | [PureConfig](https://pureconfig.github.io/), [Ciris](https://cir.is/), [Typesafe Config](https://github.com/lightbend/config) | go-only (GO_BEST §28) | review |
| `SCALA-OPS-02` | Package as a container with `sbt-native-packager` (`sbt Docker/publishLocal`, `JavaAppPackaging`) or `sbt-assembly` fat jar; multi-stage Dockerfile on `eclipse-temurin:<jdk>-jre` (distroless Java also fine), non-root user, `-XX:MaxRAMPercentage`, `-XX:+ExitOnOutOfMemoryError`. [OPS-02] | Small, safe images | SHOULD | [sbt-native-packager](https://www.scala-sbt.org/sbt-native-packager/), [sbt-assembly](https://github.com/sbt/sbt-assembly) | missing | review |
| `SCALA-OPS-03` | Native image: GraalVM `native-image` via `sbt-native-image`; works with http4s/Tapir/doobie only with reflection and resource configs; test with the tracing agent; use only when startup/RAM dominates; Scala 3 enums and `derives` are generally friendlier than Scala 2 macros. [OPS-03] | Closed-world assumption | AVOID (default) | [sbt-native-image](https://github.com/scalameta/sbt-native-image), [GraalVM native image](https://www.graalvm.org/latest/reference-manual/native-image/) | missing | review |
| `SCALA-OPS-04` | Lifecycle: `IOApp`/`ZIOAppDefault` handles SIGTERM by cancelling the main fiber; drain the HTTP server (Ember `shutdownTimeout`), stop consumers before closing pools; make `Resource` release order the reverse of acquisition. [OPS-04] | Zero dropped requests | MUST | [IOApp](https://typelevel.org/cats-effect/docs/core/io-runtime-config), [Ember](https://http4s.org/v0.23/docs/service.html) | covered (HEXAGONAL init order, jobs) | review |
| `SCALA-OPS-05` | Run-scoped build caches: point `COURSIER_CACHE` (jars), `-Djava.io.tmpdir=<dir>/tmp` (and `sbt.global.base` if needed) into a per-run directory; keep `COURSIER_CACHE` and `sbt.boot.directory` shared (they hold downloads; per-run copies re-fetch the world, like `GRADLE_USER_HOME`); `target/` stays per project and cannot be redirected cheaply in sbt 1 (set `ThisBuild / target` only if needed; sbt 2 uses `target/out`). Mill: `out/`; scala-cli: `.scala-build/`. [OPS-05] | Isolates parallel runs | SHOULD | [Coursier cache](https://get-coursier.io/docs/cache), [sbt: Global settings](https://www.scala-sbt.org/1.x/docs/Global-Settings.html), [sbt: .sbtopts / -D](https://www.scala-sbt.org/1.x/docs/Command-Line-Reference.html) | go-only (GOCACHE) | hook |
| `SCALA-OPS-06` | Artifacts left behind for cleanup: `target/`, `project/target/`, `project/project/`, `.bsp/`, `.bloop/`, `.metals/`, `.scala-build/`, `out/` (Mill), `*.class`, `*.log`, `hs_err_pid*.log`, `*.hprof`, `*.jfr`, `scoverage-data/`, `scoverage-report/`, `coverage-report/`, `cobertura.xml`, `scoverage.xml`, `.ammonite/`, `metals.sbt`. [OPS-06] | Drives cleanup | MUST | [sbt: clean](https://www.scala-sbt.org/1.x/docs/Command-Line-Reference.html) | go-only (`*.test`, `*.prof`) | hook |

## 13. DOC: documentation and review

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `SCALA-DOC-01` | API docs: Scaladoc (`sbt doc`), Scala 3 `scaladoc` with `/** ... */` and markdown (`@param`, `@return`, `@throws`); document public traits/ADTs and their laws; use `-siteroot`/MDoc for checked docs (`sbt-mdoc`). [DOC-01] | Compiled docs do not rot | SHOULD | [Scaladoc](https://docs.scala-lang.org/scala3/guides/scaladoc/index.html), [mdoc](https://scalameta.org/mdoc/) | go-only (GO_BEST §26) | gate |
| `SCALA-DOC-02` | Repo docs: README (build, run, test, config), ADRs under `docs/adr/`, CHANGELOG from conventional commits, architecture diagram of subprojects. [DOC-02] | Onboarding | SHOULD | [ADR](https://adr.github.io/), [WORKFLOW ADRs](../../specs/WORKFLOW.md) | covered (HEXAGONAL ADRs) | review |
| `SCALA-DOC-03` | Review focus: effect-system hygiene (no blocking, no unmanaged fibers, resources in `Resource`/`Scope`), total matches (sealed + exhaustivity), no `null`/`var`/`.get`/`asInstanceOf`, domain purity, binary compatibility on public libs, tagless-final leaks, `implicit` hygiene, test determinism. [DOC-03] | Highest-yield checks | MUST | [Scala style](https://docs.scala-lang.org/style/), [cats-effect best practices](https://typelevel.org/cats-effect/docs/faq) | go-only (GO_BEST §51-52) | review |
| `SCALA-DOC-04` | Public API surface: use `@deprecated("msg", "since")`, `@experimental`, MiMa filters in `project/` with justification, and `private[acme]` to restrict. [DOC-01] | Controlled evolution | SHOULD | [MiMa](https://github.com/lightbend/mima) | missing | gate |

## 14. FW: frameworks

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `SCALA-FW-01` | Dominant stacks (as of 2026-11, per Scala community surveys, check the latest): http4s/Tapir/doobie on cats-effect; ZIO + ZIO HTTP; Play; Pekko HTTP. Respect the project's choice; do not mix stacks. [FW-01] | Follow the framework | MUST | [State of Scala survey](https://scalasurvey2026.virtuslab.com/) (check current year), [Typelevel](https://typelevel.org/) | missing | review |
| `SCALA-FW-02` | Conventions: http4s requires `F[_]: Async` and `Resource` for servers; ZIO uses layers and `ZIOAppDefault`; Play wants `app/`, `conf/routes`, controllers and compile-time DI (conflicts with strict subproject hexagon unless controllers are thin adapters in an `adapters-http` project). Project rules win. [FW-02] | Fit hexagon to framework | SHOULD | [Play: project layout](https://www.playframework.com/documentation/latest/Anatomy) | missing | review |
| `SCALA-FW-03` | Akka vs Pekko: the Akka licence (BSL 1.1 since 2.7) is not free for production; use Apache Pekko (Apache-2.0, fork of Akka 2.6) with `org.apache.pekko` and rename imports from `akka.*`. Akka-HTTP/Play users must check the licence grant. [FW-02] | Legal risk | MUST | [Pekko](https://pekko.apache.org/), [Akka licenses](https://doc.akka.io/libraries/akka-core/current/project/licenses.html) | missing | review |
| `SCALA-FW-04` | Framework footguns: Play's reflection-based Guice DI hides wiring errors; http4s `HttpRoutes` return `OptionT`, so a missed case is a 404; ZIO layers fail at startup with long type errors; implicit scope bloat in Tapir/circe auto-derivation (use `semiauto`/`derives`); `Future`-based Slick on small pools. [FW-03] | Known traps | SHOULD | [circe: semiauto vs auto](https://circe.github.io/circe/codecs/auto-derivation.html) | missing | review |

## Top 10 footguns

1. **Blocking the compute pool.** Bad: `IO(blockingJdbc())` / `Await.result`. Fix: `IO.blocking(...)`, `ZIO.attemptBlocking`, or a dedicated pool. ([cats-effect thread model](https://typelevel.org/cats-effect/docs/thread-model))
2. **Dropped effects.** Bad: a statement `logger.info("x")` returning `IO` that is never run. Fix: `-Wnonunit-statement` plus `-Wvalue-discard` as errors. ([scalac options](https://docs.scala-lang.org/overviews/compiler-options/index.html))
3. **Read-modify-write on `Ref`.** Bad: `ref.get.flatMap(v => ref.set(v + 1))`. Fix: `ref.update(_ + 1)` or `modify`. ([Ref](https://typelevel.org/cats-effect/docs/std/ref))
4. **Fire-and-forget fibers.** Bad: `io.start` never joined or cancelled. Fix: `Supervisor`, `background`, `Scope`. ([Supervisor](https://typelevel.org/cats-effect/docs/std/supervisor))
5. **`Option.get` / `null` / `asInstanceOf`.** Fix: pattern match, `fold`, WartRemover `Wart.OptionPartial`, `Wart.Null`. ([WartRemover](https://www.wartremover.org/doc/warts.html))
6. **Wrong `%` vs `%%`.** Bad: `"org.typelevel" % "cats-core" % "2.13.0"`. Fix: `%%`, so the Scala binary suffix is added. ([sbt library deps](https://www.scala-sbt.org/1.x/docs/Library-Dependencies.html))
7. **Eager `Future`.** Bad: `val f = Future(sideEffect())` at class init runs immediately and cannot be cancelled. Fix: `IO`/`ZIO` (lazy, cancelable) or a `def`. ([cats-effect](https://typelevel.org/cats-effect/docs/getting-started))
8. **Stale scoverage instrumentation.** Bad: `sbt coverage test` without `clean`. Fix: `sbt clean coverage test coverageReport`, then `coverageOff`. ([sbt-scoverage](https://github.com/scoverage/sbt-scoverage))
9. **Implicit ambiguity and auto-derivation.** Bad: `io.circe.generic.auto._` in a large module (slow compile, wrong codecs). Fix: `derives Codec.AsObject` / `semiauto`. ([circe](https://circe.github.io/circe/))
10. **Using Akka for a new project.** Bad: `akka-actor` 2.7+ (BSL). Fix: Apache Pekko. ([Akka licences](https://doc.akka.io/libraries/akka-core/current/project/licenses.html))

## 15. LOOPS: tooling data

```json
{
  "id": "scala",
  "name": "Scala",
  "versions": {"stable": "3.10.0", "lts": "3.9.x (also 3.3.x LTS)", "as_of": "2026-11"},
  "detect": [{"file": "build.sbt"}, {"file": "build.sc"}, {"file": "build.mill"}, {"file": "project.scala"}],
  "source_globs": ["**/src/main/scala/**/*.scala", "**/src/main/scala-3/**/*.scala", "**/src/main/scala-2.13/**/*.scala", "**/*.sc"],
  "test_globs": ["**/src/test/scala/**/*.scala", "**/src/it/scala/**/*.scala", "**/*Spec.scala", "**/*Suite.scala", "**/*Test.scala"],
  "generated": {"globs": ["**/target/**", "**/src_managed/**", "**/*.pb.scala", "**/BuildInfo.scala", "**/generated/**"], "header_regex": "^// (Generated|Code generated|AUTO-GENERATED)"},
  "checks": [
    {"name": "compile", "kind": "build", "argv": ["sbt", "-batch", "Test/compile"], "required": true, "tool": "sbt"},
    {"name": "scalafmt", "kind": "format", "argv": ["sbt", "-batch", "scalafmtCheckAll", "scalafmtSbtCheck"], "required": true, "tool": "sbt-scalafmt"},
    {"name": "scalafix", "kind": "lint", "argv": ["sbt", "-batch", "scalafixAll --check"], "required": false, "tool": "sbt-scalafix"},
    {"name": "test", "kind": "test", "argv": ["sbt", "-batch", "clean", "coverage", "test", "coverageReport"], "required": true, "tool": "sbt-scoverage"},
    {"name": "dependency-check", "kind": "vuln", "argv": ["sbt", "-batch", "dependencyCheck"], "required": false, "tool": "sbt-dependency-check"},
    {"name": "osv", "kind": "vuln", "argv": ["osv-scanner", "scan", "source", "-r", "."], "required": false, "tool": "osv-scanner"}
  ],
  "test_commands": [
    {"argv_prefix": ["sbt"]}, {"argv_prefix": ["sbtn"]}, {"argv_prefix": ["mill"]}, {"argv_prefix": ["scala-cli", "test"]}
  ],
  "race": {"mode": "none", "flags": [], "env": {}, "reason": "The JVM has no general data-race detector; ThreadSanitizer does not apply to JVM code.", "substitute": "repeat-parallel: loops invokes the test argv 3 times with suites in parallel (parallelExecution is default true; with fork add Test / testForkedParallel := true); concurrent fiber tests (parTraverse/foreachPar, nonFlaky) with virtual time; optional jcstress/Lincheck for lock-free code"},
  "coverage": {"format": "cobertura", "path": "target/scala-*/coverage-report/cobertura.xml", "granularity": "statement"},
  "post_edit": {"format_file": null, "lint_dir": null},
  "cache_env": {"COURSIER_CACHE": "shared", "SBT_BOOT": "shared", "JAVA_TOOL_OPTIONS": "-Djava.io.tmpdir=<dir>/tmp"},
  "artifacts": ["target/", "project/target/", "project/project/", ".bsp/", ".bloop/", ".metals/", ".scala-build/", "out/", "*.class", "hs_err_pid*.log", "*.hprof", "*.jfr", "scoverage-data/", "scoverage-report/", "coverage-report/"],
  "arch": {
    "import_regex": "^\\s*import\\s+([A-Za-z_][A-Za-z0-9_.]*)",
    "resolver": "package-dir",
    "native_tool": "sbt subproject dependsOn (compiler-enforced); ArchUnit optional",
    "forbidden_in_domain": ["java.io", "java.net", "java.sql", "java.nio.file", "scala.concurrent", "cats.effect", "zio", "fs2", "org.http4s", "doobie", "skunk", "slick", "play", "org.apache.pekko", "akka"],
    "impure_calls": ["Instant.now", "LocalDateTime.now", "System.currentTimeMillis", "System.nanoTime", "scala.util.Random", "java.util.UUID.randomUUID", "Thread.sleep", "Await.result", "sys.env", "null", "var "]
  },
  "test_jobs": "set Global / concurrentRestrictions += Tags.limit(Tags.Test, 2)",
  "config_files": [".scalafmt.conf", ".scalafix.conf", ".editorconfig", "build.sbt", "project/plugins.sbt", "project/build.properties", ".sbtopts", ".jvmopts"],
  "sources": {
    "checks": "https://www.scala-sbt.org/1.x/docs/Command-Line-Reference.html",
    "scalafmt": "https://github.com/scalameta/sbt-scalafmt",
    "scalafix": "https://scalacenter.github.io/scalafix/docs/users/installation.html",
    "coverage": "https://github.com/scoverage/sbt-scoverage",
    "race": "https://typelevel.org/cats-effect/docs/core/test-runtime",
    "vuln": "https://github.com/albuch/sbt-dependency-check",
    "osv": "https://google.github.io/osv-scanner/",
    "caches": "https://get-coursier.io/docs/cache"
  }
}
```

## Gaps vs loops

| Rule ID | Gap | loops today | Priority | Where loops should fix it |
|---|---|---|---|---|
| `SCALA-TEST-06/07` | Cobertura XML parser, `.scala` candidate files, `src/test/scala` globs, generated-code markers | go-only | P0 | `gate/coverage.go` + new `coverage` adapter; pack data |
| `SCALA-CONC-04` / `TEST-11` | `race.mode = none` with substitute (3x repeat, parallel, fork); test guard must accept `sbt`/`mill`/`scala-cli` prefixes | go-only | P0 | `guard/race.go`, gate runner, WORKFLOW §6 |
| `SCALA-TOOL-03/04/05` | Gate runs gofmt/vet/staticcheck only; needs scalafmt, scalafix, `-Werror` compile checks from pack | go-only | P0 | `gate.runCheck` driven by pack `checks` |
| `SCALA-TOOL-08`, `SEC-06` | Vulnerability scan is govulncheck only; need dependency-check/OSV | go-only | P0 | gate, pack checks |
| `SCALA-ARCH-08/09`, `ARCH-02` | Arch parser is Go-only; need Scala `import` parser (`package-dir`), forbidden imports and impure calls | go-only | P1 | `archcheck.Parser` port + `scalaparse` adapter |
| `SCALA-SEC-05`, `SCALA-OPS-01` | Boundary validation and typed config rules exist only in GO_BEST | go-only | P1 | pack RULES.md |
| `SCALA-OPS-05/06` | Run-scoped tmpdir (`COURSIER_CACHE` and sbt boot stay shared) and cleanup list (`target/`, `.bsp/`, `.bloop/`...) | go-only | P1 | `workspace`, `runspace`, `go-env` hook generalised |
| `SCALA-TOOL-01`, `REPO-*` | No repo-creation/lockfile/pinning/CI/release rules | missing | P1 | new `repo` skill + pack |
| `SCALA-ARCH-04/05/06` | Effect-system choice, layers and tagless final mapping | missing | P1 | pack skill reference + architect role |
| `SCALA-IDIOM-*` | Null/partial functions/immutability/resources rules | missing | P1 | pack RULES.md; WartRemover gate |
| `SCALA-ARCH-11`, `FW-03` | Akka licence trap | missing | P1 | pack rules |
| `SCALA-SEC-*` | No OWASP mapping or SAST step | missing | P1 | security skill + gate Semgrep |
| `SCALA-REPO-12`, `OPS-06` | `.gitignore`/hygiene checks | missing | P2 | pack data, review |
| `SCALA-PERF-*`, `OBS-*`, `DOC-*`, `API-*` | JMH, async-profiler, otel4s, Tapir, etc. | missing/go-only | P2 | pack reference files |
| `SCALA-TEST-05` | Testcontainers needs Docker | missing | P2 | gate skip logic |
| post-edit hook | No format/lint on `.scala` edit (scalafmt is slow in sbt; prefer standalone `scalafmt` binary via Coursier) | go-only | P1 | `go-post-edit` generalised; pack `post_edit` |

## Open questions

- **sbt 1 vs 2.** sbt 2.0.12 is stable (2.1.0-RC3 and 1.13.1 also shipping, as of 2026-11); many plugins (WartRemover, scalafix, native-packager) may still lag on sbt 2. Maintainer should decide which line the pack assumes. sbt 2 moves `target` to `target/out/jvm/<scala>/<project>/`, which changes the coverage path glob; verify on a real project.
- **Exact Cobertura path (resolved in round 2).** sbt-scoverage docs say reports are in `target/scala-<ver>/scoverage-report`; the Cobertura file name (`cobertura.xml`) and the key `coverageOutputCobertura` come from plugin source and secondary sources, so confirm by running `sbt coverageReport` once. The README fetched did not name the file.
- **sbt cannot take multiple tasks in `-batch` quoting reliably across shells**: the argv above passes each task as a separate arg, which sbt accepts; `scalafixAll --check` must be one quoted arg (as shown).
- **Mill and scala-cli versions** were not re-verified in this pass (Mill 1.x assumed); also Mill's own coverage (`mill.contrib.scoverage`) differs from sbt-scoverage paths.
- **http4s, cats-effect, ZIO versions** were not re-verified for 2026-11; the survey URL for 2025 needs checking against the newest year.
- **Race substitute is weaker than `-race`.** Maintainer should accept `none + repeat/parallel` or require per-project jcstress/Lincheck tests for lock-free code.
- **WartRemover and scalafix on Scala 3** have partial rule support; the gate should treat them as `required: false` until a project opts in.
- **Akka licence nuance**: Play users receive an additional-use grant for akka-streams only through Play; treat anything else as needing legal review.

## Verification

- **Verdict: `fail`, round 1.** It has 1 lint ERR row, 8 `FAIL` links, a wrong coverage path, a flawed race substitute and 1 forbidden status value.
- **Completeness:** all 14 sections, Top 10, LOOPS, Gaps, Open questions and Verification are present. Rule counts are REPO 15, TOOL 9, ARCH 12, IDIOM 12, CONC 7, TEST 11, PERF 4, SEC 7, DATA 5, API 6, OBS 4, OPS 6, DOC 4, FW 4 (106 in all, every minimum met). The LOOPS JSON parses and has every required key. Lint reports `ERR 1 malformed rule rows` (finding 1).
- **Links:** `links.sh` checked 160 URLs and found 8 `FAIL` (finding 2). The URLs cited from memory were checked by `links.sh` and the ones below were opened.
- **Sources opened (10, 3 unsupporting plus the dead links):**
  - https://www.scala-lang.org/download/all.html: yes. 3.10.0 (2026-10-12), 3.9.0 LTS (2026-09-03), 3.3.9 LTS (2026-10-28), 2.13.18 (2025-11-24).
  - https://github.com/sbt/sbt/releases (via the GitHub API): yes. v2.0.12 and v1.13.1 (2026-10-28), v2.1.0-RC3 (2026-11-05) pre-release (REPO-11).
  - https://github.com/scoverage/sbt-scoverage: **partial**. It confirms `clean coverage test`, `coverageReport`, `coverageAggregate`, `coverageExcludedFiles` (Scala 3.3.4+/3.4.2+), comment markers on Scala 2 only, and Scala 3 support from 3.2. It does not name the Cobertura file or its directory, and the spot run shows the file's path is wrong. Maven Central confirms v2.4.4.
  - https://www.scala-sbt.org/1.x/docs/Testing.html: **no** for TEST-09 and the race substitute. sbt runs tests in parallel by default, and "tests executed in a forked JVM are executed *sequentially*" unless `Test / testForkedParallel := true` (finding 4).
  - https://typelevel.org/cats-effect/docs/core/test-runtime: **no** for CONC-03's "(deterministic scheduler)". The page says "This does not mean it is a *deterministic* runtime" (finding 6).
  - https://doc.akka.io/libraries/akka-core/current/project/licenses.html: yes. Akka 2.10.24, BSL 1.1, Change Date 2029-10-09, and a Play-only grant for akka-streams (ARCH-11, FW-03).
  - https://zio.dev/reference/test/aspects/: yes, but brief. It shows `nonFlaky`, `repeats` and `timeout` in examples (CONC-04).
  - https://get-coursier.io/docs/cache: yes. `COURSIER_CACHE` > `coursier.cache` > default `~/.cache/coursier/v1`; sbt uses `-Dsbt.coursier.home` (OPS-05).
  - https://www.wartremover.org/doc/warts.html: yes. All 7 warts are listed (TOOL-07). Scala 3 support is not mentioned, which matches Open questions.
  - Dead links: see finding 3.
- **Tooling truth:**
  - `sbt -batch scalafmtCheckAll scalafmtSbtCheck`: confirmed (exit 0; sbt-scalafmt 2.6.2).
  - `sbt -batch clean coverage test coverageReport`: confirmed (exit 0).
  - `coverage.path` `target/scala-*/scoverage-report/cobertura.xml`: **wrong**. The plugin writes `target/scala-<full ver>/coverage-report/cobertura.xml`; only `scoverage.xml` and HTML go to `scoverage-report/` (finding 3).
  - Format: Cobertura (`coverage-04.dtd`; `<class filename="com/acme/Hello.scala">` relative to a `<sources>` entry). Note that `<sources>` also contains a bogus `<source>--source</source>`, so the parser must skip entries that are not directories.
  - `coverageOutputCobertura`: it exists and defaults to `true` (`show coverageOutputCobertura` → true). That resolves the open question, and the "set … := true to force" step is unnecessary.
  - `Test/parallelExecution` defaults to true and `Test/fork` to false (`show`).
  - `scalafixAll --check`, `dependencyCheck` and the osv argv were not run (the plugins were not installed in the hello-world); `scalafixAll --check` as a single argv element is the correct sbt form.
- **Spot run:** image `sbtscala/scala-sbt:eclipse-temurin-alpine-25.0.5_8_1.13.1_3.9.0` (sbt 1.13.1, Scala 3.9.0, JDK 25), sbt-scoverage 2.4.4, sbt-scalafmt 2.6.2, MUnit 1.2.1, one `FunSuite` test.
  - `sbt -batch scalafmtCheckAll scalafmtSbtCheck` → exit 0.
  - `sbt -batch clean coverage test coverageReport` → exit 0. It printed `Passed: Total 1`, `Written Cobertura report [/tmp/s/target/scala-3.9.0/coverage-report/cobertura.xml]` and `Written XML coverage report [/tmp/s/target/scala-3.9.0/scoverage-report/scoverage.xml]`, with stmt and branch at 100%.
  - `sbt -batch 'show coverageOutputCobertura' 'show Test/parallelExecution' 'show Test/fork' 'show coverageDataDir'` → `true`, `true`, `false`, `/tmp/s/target/scala-3.9.0`.
  - sbt 2 paths were not tested.
- **Findings:**
  1. **Lint ERR, `SCALA-CONC-05`.** `loops today` = `partial (GO_BEST §10)`, and `partial` is not allowed. Fix: `go-only (GO_BEST §10)`.
  2. **Dead links (8).**
     - DATA-02: https://documentation.red-gate.com/fd/migrations-184127470.html → https://documentation.red-gate.com/fd/migrations-271585107.html
     - OBS-02: https://http4s.org/v0.23/docs/metrics.html → https://github.com/http4s/http4s-prometheus-metrics
     - TEST-11: https://mill-build.org/mill/scalalib/test-frameworks.html → https://mill-build.org/mill/scalalib/testing.html
     - FW-01: https://scalasurvey2025.virtuslab.com/ (000) → https://scalasurvey2026.virtuslab.com/
     - ARCH-04 and Top 10 #7: https://typelevel.org/cats-effect/docs/ → https://typelevel.org/cats-effect/docs/getting-started
     - IDIOM-06: https://typelevel.org/cats-effect/docs/std/clock → https://typelevel.org/cats-effect/docs/typeclasses/clock
     - DATA-03: https://typelevel.org/doobie/docs/10-Connection.html → https://typelevel.org/doobie/docs/14-Managing-Connections.html (or `03-Connecting.html`)
     - ARCH-06: https://zio.dev/reference/di/zlayer → https://zio.dev/reference/contextual/zlayer
  3. **Tooling, `coverage.path` and the TEST coverage recipe.** The Cobertura file is `target/scala-*/coverage-report/cobertura.xml`, not `…/scoverage-report/cobertura.xml`, so loops would find no coverage file and the P0 gate would fail. The aggregate goes to the root `target/scala-*/coverage-report/`. Evidence: spot run output. Fix the JSON path, the recipe paragraph and OPS-06/`artifacts` (add `coverage-report/`).
  4. **Race substitute, CONC policy line, `race.substitute`, `SCALA-TEST-09`.** "`Test / parallelExecution := true` and `Test / fork := true`" does not run suites in parallel: forked tests run sequentially unless `Test / testForkedParallel := true`, and `parallelExecution` is already true by default (sbt Testing docs; `show` in the spot run). TEST-09's "parallel by default only if … := true" is wrong. The "3 repeats" are not encoded in any argv either. Fix: specify `Test / fork := true` plus `Test / testForkedParallel := true` (or no fork), say how loops sets them (`set` commands in argv, for example `sbt -batch 'set Test / testForkedParallel := true' …`), and state how loops repeats the test check 3 times.
  5. **Contradiction, `cache_env` vs `SCALA-OPS-05`.** The row says to keep `COURSIER_CACHE` shared, but the JSON maps `COURSIER_CACHE` to `"cache"` (the per-run value, as with `GOCACHE`). `SBT_OPTS` also scopes `sbt.boot.directory` and `sbt.ivy.home` per run, which makes every run re-fetch the sbt launcher and boot jars. Fix: mark `COURSIER_CACHE` and the boot directory as shared, as the Java and Kotlin files do for `GRADLE_USER_HOME`, and scope only `java.io.tmpdir` (plus `sbt.global.base` if needed).
  6. **`SCALA-CONC-03`, factual.** `TestControl` is described as a "deterministic scheduler", but its docs say the opposite (it picks pending fibers at random; virtual time is externally driven). Fix: "virtual-time mock runtime (not deterministic)".
  7. **`loops today` vs baseline (`SCALA-SEC-05`, `SCALA-OPS-01`).** These are `covered` by GO_BEST §43 and §28 alone, and the baseline says GO_BEST is go-only as written. Fix: `go-only` and add them to Gaps.
  8. **Minor, `test_jobs`.** `-Dsbt.task.forcegc=false` has nothing to do with job limits. Fix: give only `Global / concurrentRestrictions += Tags.limit(Tags.Test, 2)` (as a `set` argv) and drop forcegc.

### Round 2

- **Verdict: `pass`, round 2.** No lint ERR and no `FAIL` links; all 8 findings are resolved.
- **Lint:** `scala.md: 372 lines, 106 rules`, with no ERR.
- **Links:** `links.sh` reported 1 × 503 (github.com `Scala.gitignore`), which returned 200 on a retry, so it is transient.
- **LOOPS JSON:** valid (`jq`).
- **Re-spot-run:** `sbtscala/scala-sbt:eclipse-temurin-alpine-25.0.5_8_1.13.1_3.9.0`, with two suites that each sleep for 2 s.
  - `sbt -batch 'set Test / fork := true' test` → the suites ran one after the other on `pool-1-thread-1` (B ended before A started), in 7 s.
  - Adding `'set Test / testForkedParallel := true'` → both suites started together on `pool-1-thread-2` and `pool-1-thread-3`, in 3 s.
  - `sbt -batch clean coverage test coverageReport` → `Written Cobertura report [/tmp/s/target/scala-3.9.0/coverage-report/cobertura.xml]`, and the `target/scala-*/coverage-report/cobertura.xml` glob matches.
- **Round 1 findings:**
  1. Resolved. SCALA-CONC-05 is `go-only (GO_BEST §10)`.
  2. Resolved. None of the 8 dead URLs remain outside this section, and the link check is clean.
  3. Resolved. `coverage.path` is `target/scala-*/coverage-report/cobertura.xml`, and the recipe, OPS-06 and `artifacts` now include `coverage-report/`. Confirmed by the re-spot-run.
  4. Resolved. The policy line, CONC-04, TEST-09 and `race.substitute` say `parallelExecution` defaults to true, that `Test / testForkedParallel := true` is needed with fork, and that loops (not sbt) repeats the test argv 3 times. The fork behaviour is confirmed by the re-spot-run.
  5. Resolved. `cache_env` marks `COURSIER_CACHE` and `SBT_BOOT` as `shared` and scopes only `java.io.tmpdir`.
     - Note: `SBT_BOOT` is a label, not an env var that sbt reads (sbt uses `-Dsbt.boot.directory`). That is the same label style as Java's `maven.repo.local`, so it is left for the synthesizer's schema pass.
  6. Resolved. SCALA-CONC-03 calls `TestControl` a "virtual-time mock runtime" and says it is not deterministic.
  7. Resolved. SCALA-SEC-05 and SCALA-OPS-01 are `go-only (GO_BEST §43/§28)` and appear in the Gaps table.
  8. Resolved. `test_jobs` is `set Global / concurrentRestrictions += Tags.limit(Tags.Test, 2)`.
