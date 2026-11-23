# Rust architecture (digest)

Cites RULES.md. Layering, ports, SOLID and the dependency rule are in `specs/ARCHITECTURE.md` and `specs/SOLID.md`; this file only maps them onto Rust.

## Mapping
| Layer | Rust unit | May depend on |
|---|---|---|
| domain | crate `domain` (or `src/domain`) | std and plain value types only |
| application | crate `application`: ports (traits) + use cases | domain |
| inbound adapter | crate `adapter-http`, `adapter-grpc` | application, domain, axum/tonic |
| outbound adapter | crate `adapter-postgres`, `adapter-*` | application, domain, sqlx/reqwest |
| composition root | crate `bootstrap` (the `[[bin]]`) | everything |

- Default style: modular monolith, one binary, hexagonal crates (RUST-ARCH-01, -02). Crates cannot be cyclic and cannot use what `Cargo.toml` does not list, so the compiler enforces the direction.
- Inside a crate: items are private by default, `pub(crate)` for internals, `pub` only for the API; modules may be cyclic, crates may not (RUST-ARCH-03).
- Hyphens in package names become underscores in paths (`adapter_http`); the `loops arch` resolver (`crate-path`) maps `use` first segments to workspace crates accordingly.

## What `loops arch` checks
The pack uses the regex parser over `use` and `extern crate` lines with the `crate-path` resolver:
- `forbidden_in_domain`: `std::fs`, `std::net`, `std::env`, `std::process`, `std::thread`, `tokio`, `sqlx`, `diesel`, `axum`, `actix_web`, `reqwest`, `hyper`, `tracing` (RUST-ARCH-07);
- `impure_calls`: `SystemTime::now`, `Instant::now`, `Utc::now`, `OffsetDateTime::now_utc`, `Uuid::new_v4`, `rand::`, `thread_rng`, `tokio::spawn`, `thread::spawn`.
It is a line scanner: it does not see glob re-exports or macro-generated `use`. For the crate graph run `cargo tree -p domain --edges normal` or use `cargo metadata`, and `cargo-deny` `[bans]` with `wrappers` to restrict who may depend on a crate (RUST-ARCH-04). Inside one crate, clippy `disallowed-types`, `disallowed-methods` and `disallowed-macros` in `clippy.toml` ban cross-layer items (RUST-ARCH-05).

## Rules of thumb
- Ports are small traits owned by the consumer (`application`); `async fn` in traits for async ports, with a `Send` bound (`trait_variant::make(Send)` or `impl Future + Send`) so handlers stay spawnable; `Arc<dyn Trait>` (with `async_trait`) only when dynamic dispatch is needed (RUST-ARCH-08).
- Wire by hand in `bootstrap/main.rs` with `Arc<dyn Port>` or generics; no DI container, no `OnceLock` service locators (RUST-ARCH-06).
- Domain: newtypes, enums for state machines, private fields with validating constructors, events as plain enums returned by aggregate methods, no `sqlx::FromRow` or `Serialize` on entities (RUST-ARCH-09, RUST-DATA-06).
- Inject `Clock`, `IdGenerator` and `Rng` ports; pass time as a parameter (RUST-ARCH-07, RUST-IDIOM-10).
- Size limits are heuristics: functions about 50 lines, `clippy::too_many_arguments`, `cognitive_complexity` in `clippy.toml`, one module per about 400 lines; `#[non_exhaustive]` on public enums that will grow (RUST-ARCH-10).
- Event-driven work: a `tokio::sync::mpsc` channel plus one owning task before any actor framework; durable events through an outbox (RUST-ARCH-11). Serverless: `cargo-lambda`, handler as an inbound adapter (RUST-ARCH-12).

## Review checklist
Layer direction in `Cargo.toml`; ports in the consumer crate; no IO crate reachable from `domain`; the composition root is the only place that sees all crates (RUST-DOC-03).
