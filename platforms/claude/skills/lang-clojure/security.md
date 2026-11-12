# Clojure security

Digest of CLJ-SEC. Universal rules: OWASP and `specs/ARCHITECTURE.md`. Rules: `specs/languages/clojure/RULES.md`.

## Injection and parsing (CLJ-SEC-01/02/03)
- SQL: HoneySQL maps with `?` params through next.jdbc; never concatenate input.
- Shell: `clojure.java.shell/sh` or `ProcessBuilder` with an argv vector, never `sh -c` with a string; allow-list arguments.
- Untrusted data: `clojure.edn/read-string` (restricted `:readers`) or JSON. Never `read-string`, `load-string` or `eval`. Disable keywordizing of untrusted JSON keys (keyword interning).

## HTTP and input (CLJ-SEC-04/05/08)
- `ring-defaults` (`secure-api-defaults`, `secure-site-defaults`): headers, CSRF, HTTPS, same-site cookies.
- SSRF and traversal: allow-list outbound hosts, block internal IPs before `hato`/`clj-http`; `file-response` with a root.
- Validate and coerce at the HTTP adapter (Reitit `:coercion`); 400/422 with problem details.

## Secrets and crypto (CLJ-SEC-06/07)
- Secrets from env or a vault (aero `#env`); never in `config.edn`, logs or exception data. Passwords with `buddy-hashers` (argon2id or bcrypt); tokens with `SecureRandom`, never `rand`.

## Supply chain and SAST (CLJ-SEC-09/10, CLJ-TOOL-07/08)
- Exact pins, git deps by SHA. The `nvd` check (`default: off`) needs `NVD_API_TOKEN`; OSV-Scanner does not read `deps.edn`, so feed it a generated `pom.xml` as a second opinion.
- SAST: clj-kondo hooks, Semgrep, SpotBugs + FindSecBugs on the uberjar for interop-heavy code. Eastwood evaluates code: run it only where loading is safe.
