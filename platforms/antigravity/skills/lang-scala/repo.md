# Scala repo and toolchain digest

Cites `specs/languages/scala/RULES.md`. Project config wins; log overrides with `loops run override`.

## Layout (`SCALA-REPO-01..03`)
- Per subproject: `src/main/{scala,resources}`, `src/test/{scala,resources}`; packages mirror directories (`com/acme/domain/Order.scala` is `package com.acme.domain`).
- One root `build.sbt` with `lazy val` projects; the root `aggregate`s all with `publish / skip := true`. Shared settings go in `commonSettings` or an `AutoPlugin` under `project/`.
- Hexagonal layers are sbt subprojects with explicit `dependsOn` (`SCALA-REPO-02`); see `architecture.md`.

## Build tool (`SCALA-REPO-04`, `SCALA-REPO-05`)
- sbt by default: widest plugin set (scoverage, scalafix, native-packager). Mill when the team already uses it (variant `mill`). scala-cli only for scripts and spikes. Never two build tools in one module graph.
- Bootstrap: `sbt new scala/scala3.g8`, then add `.scalafmt.conf`, `project/build.properties`, scalafix, tpolecat flags, CI, `.gitignore`, licence on day one.

## Pins and dependencies (`SCALA-REPO-06`, `-07`, `-09`, `-11`)
- Pin `sbt.version` in `project/build.properties`, `scalaVersion` in the build, the JDK in `.sdkmanrc` or `.tool-versions` (JDK 21 or 25 LTS, `SCALA-TOOL-02`). Pin plugin versions in `project/plugins.sbt`.
- Every dependency uses `%%` (`%%%` for Scala.js or Native), exact versions, centralised in `project/Dependencies.scala`. sbt has no lockfile by default: treat `dependencyTree` output as the review artifact.
- Scala Steward or Renovate for updates (not plain Dependabot); `sbt-explicit-dependencies` for unused or undeclared ones; submit the graph with `sbt-dependency-submission` so alerts work (`SCALA-REPO-08`).

## Versions (`SCALA-TOOL-01`)
- Greenfield services: Scala 3.9.x LTS (3.3.x LTS if a dependency lags). Libraries cross-build 3.3 LTS plus 2.13. Scala 3.10.x is opt-in. Migrate 2.13 to 3 with `-Xsource:3` and scalafix migration rewrites, then switch.
- sbt 1.x is the verified line. sbt 2 moves `target/` and some plugins lag: check before moving, and re-verify the coverage glob.

## `.gitignore` and hygiene (`SCALA-REPO-12`, `-13`)
`target/`, `project/target/`, `project/project/`, `.bsp/`, `.bloop/`, `.metals/`, `.idea/`, `.vscode/`, `.scala-build/`, `.ammonite/`, `metals.sbt`, `*.class`, `*.log`. Commit `.scalafmt.conf` (pinned `version`, `runner.dialect`) and `.editorconfig`.

## CI shape (`SCALA-REPO-14`)
Restore Coursier and sbt caches, then `scalafmtCheckAll scalafmtSbtCheck`, `scalafixAll --check`, `Test/compile` with `-Werror`, `clean coverage test coverageReport`, a dependency scan, `Docker/publishLocal`. Matrix over JDK 21 and 25, and Scala 3 LTS plus latest. This is what `loops gate` mirrors.

## Releases (`SCALA-REPO-10`, `-15`)
Libraries: SemVer, `sbt-ci-release` from tags, PGP signing, `sbt-mima-plugin` (`mimaPreviousArtifacts`) because binary incompatibility is the main release hazard. Services ship a Docker image. Conventional commits drive the changelog.

## Toolchain (`SCALA-TOOL-03..09`)
- scalafmt via `sbt-scalafmt`: check in the gate, fix with `scalafmtAll scalafmtSbt`.
- scalafix (`OrganizeImports`, `RemoveUnused`, `DisableSyntax`) with `.scalafix.conf`; semantic rules need SemanticDB (automatic on Scala 3).
- Compiler: `sbt-tpolecat` (CI mode adds `-Werror`) or at least `-deprecation -feature -unchecked -Wunused:all -Wvalue-discard -Wnonunit-statement`.
- WartRemover (`Null`, `Var`, `Return`, `AsInstanceOf`, `TryPartial`, `OptionPartial`, `Throw`) as errors in domain and application only; optional until the project opts in (Scala 3 support is partial).
- Vulnerabilities: `sbt-dependency-check` or OSV-Scanner (both opt in); SBOM with `sbt-sbom`. Editor: Metals writes `.bloop/`, `.metals/`, `.bsp/`.
