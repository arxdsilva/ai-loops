# Rust security (digest)

Cites RULES.md. Review means `loops:review-integration` plus this list; findings are claims with evidence.

- **Injection:** bind parameters only (`sqlx::query!`, diesel DSL, sea-orm values); never `format!` into SQL; allow-list dynamic identifiers. `Command::new(bin).arg(x)` without a shell (RUST-SEC-01).
- **Deserialisation:** `serde` into strict typed structs, `deny_unknown_fields` for config, body-size limits (`DefaultBodyLimit`, `RequestBodyLimitLayer`); length-limit `bincode`/`postcard` on untrusted input; avoid the archived `serde_yaml` for new work (RUST-SEC-02).
- **Web:** SSRF allow-list and private-range blocking in `reqwest`; path traversal via `canonicalize` + `starts_with` or `ServeDir`; explicit CORS origins, never `Any` with credentials; CSRF for cookie sessions (RUST-SEC-03).
- **Secrets and crypto:** `secrecy::SecretString`, no `Debug` on raw secrets, no secrets in `.cargo/config.toml` or Docker layers; `rustls`, `ring`/`aws-lc-rs`, RustCrypto; `argon2` for passwords; `subtle::ConstantTimeEq` for tokens; `OsRng`/`getrandom` for keys (RUST-SEC-04).
- **Validation at the edge:** DTOs parsed into domain types with `TryFrom` (`validator` or `garde` for rules); extractors reject bad input with 400/422 before use cases run (RUST-SEC-05).
- **Supply chain:** `Cargo.lock` committed, `--locked`, `cargo deny check` with `unknown-registry = "deny"`, `unknown-git = "deny"`, allow-listed licenses; review `build.rs` and proc-macro crates (they run at build time); optional `cargo vet` (RUST-SEC-06). In the pack, `deny` (when `deny.toml` exists) and `audit` are `default: off`: run `loops gate --vuln` or enable them in config; CI should require them.
- **Static analysis:** clippy (`undocumented_unsafe_blocks`, `indexing_slicing`, `arithmetic_side_effects` for sensitive crates), `cargo audit` for RustSec, `cargo geiger` for `unsafe` in dependencies (RUST-SEC-07).
- **Panics are DoS:** handlers never panic on external input; add `CatchPanicLayer` as a last line and alert on it (RUST-SEC-08, RUST-IDIOM-03).
- **Edge hardening:** non-root minimal image, security headers, `rustls`, request and handshake timeouts, body and header caps (RUST-SEC-09).
- **Unsafe:** every block carries `// SAFETY:`; see [concurrency.md](concurrency.md) for the miri tier (RUST-IDIOM-04).
