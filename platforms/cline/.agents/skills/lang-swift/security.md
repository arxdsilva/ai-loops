# Swift: security digest

Rules in RULES.md SEC. Universal rules: `specs/ARCHITECTURE.md` (validation at boundaries).

- SQL: never concatenate. PostgresNIO `PostgresQuery` interpolation binds values; SQLKit `\(bind: x)` and `\(ident: x)`; Fluent's typed builder. Processes: `Subprocess` with an argv array, never `sh -c` with user input (SWIFT-SEC-01).
- Untrusted input: decode `Codable` into typed DTOs with size limits (`collectBody(upTo:)`); avoid `NSKeyedUnarchiver` and non-secure `NSCoding`; disable external entities in `XMLParser` (SWIFT-SEC-02).
- Web: allowlist redirects; resolve paths with `URL.standardizedFileURL` and check the prefix; block SSRF to private ranges in AsyncHTTPClient callers; CORS with explicit origins; CSRF for cookie sessions. Neither framework enables CORS or CSRF by default (SWIFT-SEC-03).
- Crypto: `swift-crypto` for hashing, AEAD and HMAC; passwords with Bcrypt or Argon2, never raw SHA; `SystemRandomNumberGenerator` or `SymmetricKey(size:)`; constant-time MAC compares; no secrets in source, a committed `.env` or logs (SWIFT-SEC-04).
- Validate at the adapter (`Validatable`, custom `init(from:)`), then build domain value objects whose `init throws` (SWIFT-SEC-05).
- Supply chain: commit `Package.resolved`; keep fingerprint checking strict and signing-entity checking at warn; pin critical deps by `exact` or revision; review new `Package.swift` files and build plugins; never pass `--disable-sandbox` or `--allow-writing-to-package-directory` for untrusted plugins (SWIFT-SEC-06).
- SAST: no Swift-first tool dominates. Use Semgrep, CodeQL (macOS runners), SwiftLint security-adjacent rules; the Swift 6 compiler with warnings as errors is the main check (SWIFT-SEC-07). Vuln scanning: Dependabot alerts plus `swift package generate-sbom`; no gate vuln check exists (SWIFT-TOOL-05).
