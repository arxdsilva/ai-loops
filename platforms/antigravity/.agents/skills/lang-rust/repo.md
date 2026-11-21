# Rust repo practices (digest)

Cites `specs/languages/rust/RULES.md`. Follow the project's own layout and config first; log overrides with `loops run override`.

## Layout
- Cargo's standard package layout: `src/main.rs`, `src/lib.rs`, `src/bin/`, `tests/`, `benches/`, `examples/`; thin `main.rs` over a library (RUST-REPO-01).
- Workspace with a virtual root manifest, `members = ["crates/*"]`, explicit `resolver = "3"`, flat `crates/` directory, crates split along architecture boundaries, not features (RUST-REPO-02, -04).
- Shared dependencies once in `[workspace.dependencies]`, inherited with `dep.workspace = true`; shared `version`, `edition`, `rust-version` in `[workspace.package]` (RUST-REPO-03).
- Optional `xtask/` for automation in Rust; mark it in `default-members` exclusions. Internal crates carry `publish = false` (RUST-REPO-10).

## Day-one files (architect or rust-engineer, wave 0)
| File | Purpose | Rule |
|---|---|---|
| `Cargo.toml` | workspace, `[workspace.lints]`, `[profile.release]` | RUST-REPO-02, RUST-TOOL-04 |
| `Cargo.lock` | committed for services and, by decision D22, for libraries too; CI uses `--locked` | RUST-REPO-06 |
| `rust-toolchain.toml` | `channel` pin (1.99.x as of 2026-11), components `rustfmt`, `clippy`, `llvm-tools-preview` | RUST-REPO-11 |
| `rustfmt.toml`, `clippy.toml` | only to deviate from defaults; clippy thresholds, `disallowed-*`, `allow-unwrap-in-tests` | RUST-TOOL-02, -05 |
| `deny.toml` | advisories, licenses, bans, sources | RUST-TOOL-07, RUST-SEC-06 |
| `.config/nextest.toml` | only if nextest is wanted; selects the gate's nextest variant | RUST-TEST-08 |
| `.gitignore` | `/target`, `lcov.info`, `*.profraw`, `*.profdata`, `.env`; do not ignore `Cargo.lock`, `.sqlx/` | RUST-REPO-13 |

## Toolchain and editions
- Install and switch only with rustup; the six-week train has no LTS, so bump the pin in a dedicated PR every release or two (RUST-TOOL-01).
- Set `rust-version` (MSRV) in `[workspace.package]`; a service may track latest stable (RUST-REPO-11).
- New code uses `edition = "2024"`; migrate with `cargo fix --edition`. The post-edit hook reads the edition from the nearest `Cargo.toml`; a member that inherits it (`edition.workspace = true`) is not matched, so the placeholder is unresolved, the hook fails open, and `cargo fmt --check` in the gate catches formatting (RUST-REPO-12).

## Dependencies
- Caret requirements, never `*` or git dependencies on published crates; add with `cargo add` (RUST-REPO-07).
- Renovate or Dependabot with grouped minor and patch updates; `cargo deny check` and tests on every update PR; prune with `cargo machete` (RUST-REPO-08).

## CI and releases
- Stage order: toolchain, cache (`Swatinem/rust-cache`), `cargo fmt --all --check`, clippy `-D warnings`, `cargo build --locked`, tests with coverage, `cargo test --doc`, `cargo deny check` and `cargo audit`, package (RUST-REPO-14). `loops gate` runs the same checks locally.
- SemVer as Cargo defines it; libraries run `cargo semver-checks`; services may use `release-plz` or `cargo release` (RUST-REPO-09). Conventional commits only if a tool consumes them (RUST-REPO-15).
- Release binaries with `cargo-dist` or a CI matrix; publish libraries with `cargo publish` (RUST-REPO-10).
- Reproducible builds: `--locked`, pinned toolchain, `trim-paths`, `cargo vendor` only for offline builds (RUST-OPS-07).
