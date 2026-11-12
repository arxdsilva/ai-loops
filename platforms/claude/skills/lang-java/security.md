# Java: security

Digest of `specs/languages/java/RULES.md` SEC and TOOL-05/06. Universal rules: `specs/SECURITY.md` where present, OWASP ASVS.

## Injection and deserialization
- Parameterised `PreparedStatement`, JPA named parameters, Spring Data derived queries, jOOQ, `JdbcClient .param()`. Never concatenate input into JPQL, HQL or SQL, including `ORDER BY` (allow-list the columns). `ProcessBuilder` takes an argv list, never a shell string. Same for LDAP filters and template engines (`JAVA-SEC-01`).
- Never deserialize untrusted data with `ObjectInputStream`; set `-Djdk.serialFilter`; no Jackson default typing or `@JsonTypeInfo(use = Id.CLASS)`; safe YAML constructors; XML parsers with secure-processing and external entities off (`JAVA-SEC-02`).

## Web
- SSRF: allow-list outbound URLs (Spring Framework 7 / Boot 4.1 have an `InetAddressFilter`); path traversal: `Path.normalize().startsWith(base)`; validate redirect targets; keep CSRF on for browser sessions (`JAVA-SEC-03`).
- Validate at the adapter boundary with Jakarta Bean Validation on request records, and construct domain value objects with invariant checks; bind to dedicated request records, never entities (mass assignment). `@Valid` is not authorization (`JAVA-SEC-05`).
- Actuator: expose only `health`, `info`, `prometheus`, on a separate secured management port; deny by default with `authorizeHttpRequests` and method security; `spring.jpa.show-sql` off in prod; never log credentials (`JAVA-SEC-08`).

## Secrets and crypto
- Secrets from the environment, Vault or K8s secrets, never in `application.yaml`, source or images; `PasswordEncoder` (Argon2, bcrypt, scrypt); `SecureRandom` for tokens, never `Random`/`Math.random` (`JAVA-SEC-04`). `new Random` is also an impure call in the domain.

## Supply chain and SAST
- Central or an internal mirror with checksum policy `fail`; Gradle dependency verification (`gradle/verification-metadata.xml`); check new coordinates for typosquatting; pinned plugins; SBOM (`JAVA-SEC-06`, `JAVA-TOOL-06`).
- Pack checks, all opt-in (`default: off`) except SpotBugs when its plugin is declared:
  - `osv-scanner scan source -r .`: reads `pom.xml` and `gradle.lockfile`, needs no key; the PR gate. Install hint: `go install github.com/google/osv-scanner/v2/cmd/osv-scanner@latest`.
  - dependency-check (`-DfailBuildOnCVSS=7`): needs an NVD API key and a warm database; use nightly.
  - `sbom`: CycloneDX (`makeAggregateBom`, `cyclonedxBom`).
- SAST (`JAVA-SEC-07`): SpotBugs with FindSecBugs in the build; Semgrep or CodeQL in CI; triage with a baseline file; no `@SuppressFBWarnings` without a justification. A configured vuln check whose tool is missing fails, it does not skip.
