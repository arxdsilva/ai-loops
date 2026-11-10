---
name: lang-rust
description: Use when writing, reviewing, testing or setting up Rust backend code in a loops run (the rust language pack) - toolchain, layout, race policy, coverage, idioms and footguns.
---
# lang-rust

The Rust language pack: `specs/languages/rust/pack.json` (what the gate runs) and `specs/languages/rust/RULES.md` (the rule book, IDs `RUST-<SEC>-NN`). Universal rules are not repeated here: layering is `specs/ARCHITECTURE.md`, SOLID is `specs/SOLID.md`, gates and the race rule are `WORKFLOW §6`, test and repo practices are `specs/TESTING.md` and `specs/REPO_PRACTICES.md`.

## When it applies
`loops lang detect` reports `rust` (a `Cargo.toml` at the root). Check with `loops lang show rust` for the resolved pack (the `nextest` variant is picked when `.config/nextest.toml` exists).

## Project rules first
Follow the project's own config over this pack: `rust-toolchain.toml`, `rustfmt.toml`, `clippy.toml`, `[workspace.lints]`, `deny.toml`, `.cargo/config.toml`, `.config/nextest.toml`, and the framework already in use (axum, actix-web, tonic). Read them before coding (`config_files` in the pack). Every time you follow a project rule over a pack rule, run `loops run override --rule "<pack rule>" --project "<project rule, file:line>" --note "<what you did>"`.

## Toolchain the gate runs
Agents run `loops gate` (or `loops gate --only <checks> --pkgs -p,<crate>` while iterating); the checks, in order:

| Check | Command | Required |
|---|---|---|
| deps | `cargo fetch --locked` | no |
| build | `cargo build {pkgs} --all-targets --locked` | yes |
| fmt | `cargo fmt --all --check` | no (treat as required when `rustfmt.toml` is committed) |
| clippy | `cargo clippy {pkgs} --all-targets --all-features --locked -- -D warnings -D unsafe_code -D clippy::await_holding_lock -D clippy::await_holding_refcell_ref` | yes (also the race substitute) |
| arch | `loops arch` | no |
| test | `cargo llvm-cov {pkgs} --locked --lcov --output-path {coverage}` (nextest variant: `cargo llvm-cov nextest ...`) | yes |
| doctest | `cargo test --doc {pkgs} --locked` | yes |
| coverage | new-code coverage from the lcov file, at least 70% | yes |
| miri, loom, tsan, doc, deny, audit | see pack | off by default |

`{pkgs}` is `--workspace` by default. `--pkgs -p,<crate>` replaces it for a scoped run (comma-separated, so `-p` and the crate name become two arguments).

The pack sets `CARGO_INCREMENTAL=0`. `CARGO_TARGET_DIR` and `TMPDIR` are scoped per run by loops; `CARGO_HOME`, `RUSTUP_HOME` and `SCCACHE_DIR` are shared (RUST-OPS-05). Never set `CARGO_TARGET_DIR` yourself. A cold per-run target dir recompiles everything: `RUSTC_WRAPPER=sccache` helps when installed. Cap parallelism with `CARGO_BUILD_JOBS=2` and `cargo nextest -j2`. After each edit the post-edit hook runs `rustfmt --edition {edition} <file>` on that one file only; never run `cargo fmt` (it rewrites files other agents own, see `specs/PARALLEL_WORK.md`).

Required tools: the rustup components `rustfmt`, `clippy`, `llvm-tools-preview` and `cargo-llvm-cov`. A missing coverage tool fails the gate; the install hint is in the check note (`rustup component add llvm-tools-preview && cargo install cargo-llvm-cov --locked`). The official `rust:1` Docker image has neither rustfmt nor clippy: add them through `rust-toolchain.toml` (`components = ["rustfmt","clippy","llvm-tools-preview"]`).

## Race policy
Safe Rust is data-race free by construction, so the pack uses `race.mode = none` with a required substitute (WORKFLOW §6, RUST-CONC-04): the `clippy` check denies `unsafe_code`, `await_holding_lock` and `await_holding_refcell_ref`. To satisfy it:
- keep `unsafe` out; a crate that really needs it adds `#[allow(unsafe_code)]` on the smallest module, a `// SAFETY:` comment on every block, and the optional checks `miri` (UB and data races, nightly), `loom` (lock-free models, `--cfg loom`) and, for FFI, `tsan` (nightly, `-Zbuild-std`). They are off by default: enable them in the project config or with `loops gate --only miri`;
- never hold a `std::sync::Mutex` guard or a `RefCell` borrow across `.await`; share by `Arc` and channels, not by cloning locks (RUST-CONC-02, -08);
- never silence a lint with a blanket `#![allow]` to get green; fix the code or record an override.
Details in [concurrency.md](concurrency.md).

## Coverage recipe
`cargo llvm-cov` writes lcov to `{coverage}`; loops creates the parent directory. `tests/`, `benches/`, `examples/`, `build.rs`, `src/main.rs`, `src/bin/` and generated code are not judged. Inline `#[cfg(test)]` lines count as code (an accepted bias, D8). Doctests are run separately and not measured.
Prerequisites the architect or `rust-engineer` adds in wave 0 (an override when the project lacks them):
- `rust-toolchain.toml` with the components above;
- `[workspace.lints]` plus `[lints] workspace = true` in every crate (RUST-TOOL-04);
- `Cargo.lock` committed (RUST-REPO-06) and `.gitignore` with `/target`, `lcov.info`, `*.profraw`;
- `.config/nextest.toml` only if the project wants nextest (it switches the variant);
- `.sqlx/` committed when sqlx macros are used (RUST-DATA-02).

## Layers on Rust
```
crates/
  domain/           lib   std + plain value types only (no tokio, sqlx, axum, reqwest, tracing)
  application/      lib   ports (traits) + use cases; depends on domain
  adapter-http/     lib   axum handlers: inbound adapter, DTOs, error mapping
  adapter-postgres/ lib   sqlx repositories: outbound adapter, implements ports
  bootstrap/        bin   composition root: config, wiring, shutdown (thin main.rs)
```
Single-crate service: modules `domain`, `application`, `adapters` with `pub(crate)` visibility, thin `main.rs` over a library. The crate graph in `Cargo.toml` is the first line of defence; `loops arch` regex-scans `use` lines for the rest. See [architecture.md](architecture.md).

## Top 10 footguns
1. Blocking the async runtime (`std::fs`, `thread::sleep`, CPU loops): use `tokio::fs`, `spawn_blocking`.
2. `std::sync::Mutex` guard or `RefCell` borrow held across `.await`: scope it before awaiting.
3. `unwrap`/`expect`/`v[i]` on external input: `?`, `.get(i)`.
4. `select!` or timeouts over cancellation-unsafe futures: use cancel-safe methods, make writes transactional.
5. Detached `tokio::spawn` that swallows panics: keep the `JoinHandle`/`JoinSet`, log `JoinError`.
6. Release-mode integer overflow and `as` casts: `checked_*`, `try_from`, `overflow-checks = true`.
7. `.clone()` to please the borrow checker, `unbounded_channel`: borrow, `Arc`, bounded channels.
8. Byte-slicing UTF-8 (`&s[..10]`): `s.get(..10)`, `char_indices`.
9. A `reqwest::Client` per request, no timeouts: one shared client with `timeout`.
10. Unlocked dependency graph, unvetted `build.rs`: commit `Cargo.lock`, `--locked`, `cargo deny check`.

## Load on demand
- [repo.md](repo.md): workspace layout, lockfile, toolchain pin, MSRV, editions, CI shape, releases.
- [architecture.md](architecture.md): crates as layers, ports as traits, the domain purity list, `loops arch` limits.
- [idioms.md](idioms.md): errors, `unwrap` policy, ownership, enums, naming, overflow, time, strings.
- [testing.md](testing.md): test layout, nextest, doubles, integration tests, coverage, flakiness.
- [concurrency.md](concurrency.md): tokio rules, cancellation, shutdown, the race substitute and the optional tiers.
- [security.md](security.md): injection, secrets, crypto, SSRF, supply chain (cargo-deny, cargo-audit), panics as DoS.
- [data-api.md](data-api.md): sqlx/diesel, migrations, transactions, axum/tower, resilience, tracing, config, Docker.
- [frameworks.md](frameworks.md): axum and actix-web conventions and framework footguns.
