# Scala security digest

Neutral rules: `specs/REPO_PRACTICES.md` when present, OWASP cheat sheets. IDs from `specs/languages/scala/RULES.md`.

## Injection (`SCALA-SEC-01`)
No SQL by `s"..."` or `+`. doobie `fr"..."` and `sql"..."` bind parameters (`Fragment.const` only for trusted identifiers); skunk `sql"..."` with codecs; Slick `sql"..."` and `#$` only for trusted identifiers; Quill quotations. Commands: `ProcessBuilder(Seq(...))`, never one shell string.

## Deserialisation (`SCALA-SEC-02`)
No Java serialization (`ObjectInputStream`), Kryo with open registration, or Jackson default typing. Use circe, jsoniter-scala, zio-json or Tapir codecs over sealed ADTs. Disable doctype declarations in XML parsers (XXE).

## Web (`SCALA-SEC-03`)
Allow-list outbound URLs (SSRF); normalise paths then check the prefix; explicit CORS origins; CSRF tokens for cookie auth (Play filter, http4s `CSRF`); a security-headers middleware.

## Secrets and crypto (`SCALA-SEC-04`)
Secrets from env or a vault, loaded with Ciris or PureConfig into a case class with a redacting `Secret` wrapper; none in `application.conf`. Argon2id or bcrypt for passwords, `SecureRandom`, JDK TLS defaults. Not MD5, SHA1, `java.util.Random`, DES.

## Boundary validation (`SCALA-SEC-05`)
Decode DTOs with circe, zio-json or Tapir, then convert to domain types through smart constructors (`Either`) or refined and iron types; reject unknown fields where it matters (`Configuration.default.withStrictDecoding`). Parse, do not validate.

## Supply chain (`SCALA-SEC-06`, `SCALA-TOOL-08`)
Maven Central or an internal mirror only; Coursier verifies checksums. sbt plugins run code at build time, so pin them in `project/plugins.sbt` and review Scala Steward PRs. Dependency scans (`sbt dependencyCheck`, `osv-scanner`) are opt-in gate checks (`--vuln`); an installed-but-failing scanner is a failure, a missing one prints its `install` hint.

## SAST (`SCALA-SEC-07`)
Semgrep (Scala rules) plus WartRemover; find-sec-bugs works on bytecode with limited Scala support. No dominant Scala SAST exists, so treat results as advisory.

## Licence (`SCALA-ARCH-11`, `SCALA-FW-03`)
Akka 2.7+ is BSL (Change Date 2029 for 2.10.x). Use Apache Pekko for new work; flag any `com.typesafe.akka` dependency for legal review.

## Review checklist
Interpolated SQL, `ObjectInputStream`, secrets in `.conf` or logs (`println`, `toString` of config), unvalidated DTOs reaching domain, unpinned sbt plugins.
