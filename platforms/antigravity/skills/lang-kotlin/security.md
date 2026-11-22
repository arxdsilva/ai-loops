# Kotlin security

Digest of RULES SEC. Universal checklist: `specs/SECURITY.md` when present.

- **SQL and processes (`KT-SEC-01`).** Bind parameters (jOOQ DSL, Exposed DSL, named `@Query`, `PreparedStatement`); never a Kotlin string template in SQL. `ProcessBuilder(list)`, never `Runtime.exec(string)`.
- **Deserialization (`KT-SEC-02`).** No `ObjectInputStream` on untrusted data; Jackson without `activateDefaultTyping`, explicit DTOs or `kotlinx.serialization`; disable XXE; SnakeYAML with `SafeConstructor`.
- **Web (`KT-SEC-03`).** Allow-list outbound URLs (SSRF) and redirect targets; `Path.normalize().startsWith(base)`; explicit CORS (never `anyHost()` with credentials); CSRF on for cookie sessions.
- **Secrets and crypto (`KT-SEC-04`).** Env or vault, never committed `application-*.yml` secrets; Argon2id or bcrypt; `java.security.SecureRandom`, not `kotlin.random.Random`; no MD5, SHA-1 or ECB; JCA or Tink.
- **Validation (`KT-SEC-05`).** Jakarta Bean Validation on DTOs at the adapter edge. Under the old default or `-Xannotation-default-target=first-only` write `@field:NotBlank`; with `param-property` (opt-in preview from Kotlin 2.2.0) an unqualified annotation hits param and field. The domain trusts its types.
- **Supply chain (`KT-SEC-06`).** Commit `gradle.lockfile`; dependency verification (`./gradlew --write-verification-metadata sha256 help`); `repositoriesMode = FAIL_ON_PROJECT_REPOS`; validate the wrapper; sign and attest releases.
- **SAST (`KT-SEC-07`).** Semgrep Kotlin rules and detekt; SpotBugs with FindSecBugs is noisy, start as a non-blocking report.
- **Dependency scan (`KT-TOOL-05`).** `loops gate --vuln` runs `osv-scanner scan source -L gradle.lockfile`; or `./gradlew dependencyCheckAnalyze`. Missing tool: install hint, never a silent skip when configured.
