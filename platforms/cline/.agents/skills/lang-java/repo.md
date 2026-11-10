# Java: repository, toolchain and CI

Digest of `specs/languages/java/RULES.md` REPO and TOOL. Universal repo rules: `specs/REPO_PRACTICES.md` where present.

## Layout
- Standard Maven directories in Maven and Gradle (`JAVA-REPO-01`): `src/main/java`, `src/main/resources`, `src/test/java`, `src/test/resources`; optional `src/integrationTest/java` (Gradle `jvm-test-suite`).
- Commit on day one (`JAVA-REPO-02`): wrapper, `.gitignore`, `.gitattributes`, `.editorconfig`, CI, DB migration folder, `.sdkmanrc` or `.tool-versions`.
- Multi-module (`JAVA-REPO-11`): one Maven reactor or Gradle multi-project build, modules `domain`, `application`, `adapters-*`, `bootstrap` so the compiler enforces direction. Convention plugins in `build-logic`. Split repos by deployable, not by layer.

## Wrapper and JDK
- Invoke only `./mvnw` / `./gradlew` (`JAVA-REPO-03`); generate with `mvn wrapper:wrapper` or `gradle wrapper --gradle-version <v>`. `.gitattributes`: `mvnw text eol=lf`, `gradlew text eol=lf`, `*.jar binary` (`JAVA-REPO-09`).
- Pin the JDK with a toolchain, not `JAVA_HOME` (`JAVA-REPO-04`): Gradle `java { toolchain { languageVersion = JavaLanguageVersion.of(25) } }` plus the foojay resolver; Maven `maven-toolchains-plugin`, `maven-enforcer-plugin` `requireJavaVersion` and `<maven.compiler.release>`.
- Target an LTS (`JAVA-TOOL-01`): JDK 25 for services, 21 as the library floor. Non-LTS (26, 27, 28) only for experiments.

## Dependencies
- Central version management (`JAVA-REPO-05`): import BOMs (`spring-boot-dependencies`, `junit-bom`, `testcontainers-bom`, `quarkus-bom`) or a Gradle version catalog; no versions repeated in modules.
- Reproducible resolution for applications (`JAVA-REPO-06`): Gradle `dependencyLocking` and a committed `gradle.lockfile`; Maven exact versions plus Enforcer `requireReleaseDeps` and `dependencyConvergence`. Libraries do not lock.
- Renovate or Dependabot per BOM; `mvn dependency:analyze` or the dependency-analysis plugin for unused deps (`JAVA-REPO-07`).
- Enforcer rules (`JAVA-TOOL-08`): `banDuplicatePomDependencyVersions`, banned `commons-logging`, `log4j:log4j`, `javax.*`.

## Quality tools
- Format with Spotless wrapping google-java-format (`spotless:check` / `spotlessCheck`); `ratchetFrom 'origin/main'` for legacy repos (`JAVA-TOOL-02`). There is no portable single-file format, so `post_edit` is null; never run a whole-module `spotless:apply` from a hook.
- Error Prone and NullAway at compile time, `-Werror` in CI (`JAVA-TOOL-03`). SpotBugs with FindSecBugs; PMD and Checkstyle only with a curated committed ruleset, never overlapping Spotless or Error Prone (`JAVA-TOOL-04`).
- Vuln, SBOM: see `security.md`.

## CI and release
- One command is the gate locally and in CI (`JAVA-REPO-10`): `setup-java` (Temurin, dependency cache), `./mvnw -B -ntp verify`, then vuln scan, SBOM, image. JDK matrix (21, 25) for libraries only.
- SemVer for libraries, Central Portal publishing with sources, javadoc, signatures; services tag `vX.Y.Z` (`JAVA-REPO-08`). Conventional commits drive changelog and version (`JAVA-REPO-12`).
- `.gitignore`: `target/`, `build/`, `.gradle/`, `out/`, `*.class`, `hs_err_pid*`, `.idea/`, `*.iml`.
- Run-created files the cleanup removes are in the pack's `artifacts` (`JAVA-OPS-06`).
