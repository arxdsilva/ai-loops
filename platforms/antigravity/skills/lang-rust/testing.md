# Rust testing (digest)

Cites RULES.md. TDD, claims and the 70% new-code rule are neutral (`WORKFLOW §6`, `ROLES.md`). Never weaken a test to make it pass.

## Layout and naming
- Unit tests in `#[cfg(test)] mod tests { use super::*; }` next to the code; integration tests in `tests/*.rs` (each file its own binary, public API only); doctests in `///` examples; `#[tokio::test]` for async (RUST-TEST-01).
- Test files by glob: `**/tests/**/*.rs`, `**/benches/**/*.rs`, `**/tests.rs`, `**/*_test.rs`, `**/*_tests.rs`. Unit tests also live inside `src/**`, so file-based detection is incomplete. Shared helpers in `tests/common/mod.rs` (RUST-TEST-02).
- Name tests `behaviour_when_condition`, one behaviour each, arrange-act-assert; `rstest` or `test_case` or a table loop for parametrised cases; prefer returning `Result<(), E>` to `#[should_panic]` (RUST-TEST-03).

## Doubles and integration
- Hand-written fakes that implement the port trait (in-memory repositories) over mocks; `mockall` only to assert boundary interactions; `wiremock` for outbound HTTP; never mock the type under test (RUST-TEST-04).
- Real dependencies: `testcontainers`, `#[sqlx::test]` (fresh database per test), `tower::ServiceExt::oneshot` on an axum `Router` without a socket (RUST-TEST-05).
- Property tests with `proptest` (commit `proptest-regressions/`), fuzzing with `cargo fuzz` for decoders, `cargo-mutants` on critical logic only; snapshots with `insta` (RUST-TEST-09, -10).

## Running and coverage
- Gate test: `cargo llvm-cov {pkgs} --locked --lcov --output-path {coverage}`; with `.config/nextest.toml` present the pack switches to `cargo llvm-cov nextest ...`. Doctests always run in the separate `doctest` check (RUST-TEST-06, -08).
- Coverage is judged on new lines (lcov `DA:` records against the git diff) at 70%; inline `#[cfg(test)]` lines count as code, and `tests/`, `benches/`, `examples/`, `build.rs`, `main.rs`, `src/bin/` are excluded (RUST-TEST-07).
- Never leave a test `#[ignore]`d without a written reason (RUST-TEST-13).

## Fast loop for agents
Use only your scope: `cargo check -p <crate>`, `cargo nextest run -p <crate> <filter>` or `cargo test -p <crate> <name> -- --exact`, with `CARGO_BUILD_JOBS=2` (RUST-TEST-12). Do not run the whole workspace; the lead and QA run `loops gate`. Never set `CARGO_TARGET_DIR`; loops scopes it per run. Compile time dominates: a fast linker (`mold`, `lld`) and `[profile.dev] debug = "line-tables-only"` help.

## Flakiness
No shared global state or fixed ports (bind port 0, read `local_addr`); inject the clock; `tokio::time::pause()` (feature `test-util`) for time; never `sleep` to wait. `cargo test` runs tests as threads of one process, nextest one process per test (RUST-TEST-11). No retry flags to hide flakes.
