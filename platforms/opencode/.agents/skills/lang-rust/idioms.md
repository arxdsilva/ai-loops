# Rust idioms (digest)

Cites RULES.md. Enforcement is mostly clippy `-D warnings` plus the workspace lint table (RUST-TOOL-04, -05).

## Errors
- `Result<T, E>` for every fallible operation, `?` to propagate; `panic!` only for broken invariants; no `let _ =` on a `Result` (RUST-IDIOM-01).
- Typed errors with `thiserror` in domain, application and adapters; `anyhow::Result` with `.context(...)` only in `bootstrap`, CLIs and tests; never `anyhow::Error` or `Box<dyn Error>` in a public library API (RUST-IDIOM-02).
- `unwrap`/`expect` are denied outside tests (`clippy::unwrap_used`, `expect_used`; `allow-unwrap-in-tests = true`); `expect("invariant: ...")` only for provable invariants; `v[i]` on external input becomes `.get(i)` (RUST-IDIOM-03).

## Safety
- `unsafe_code = "forbid"` in `[workspace.lints.rust]`; the gate also passes `-D unsafe_code`. An opt-out crate isolates `unsafe` in a small module, documents each block with `// SAFETY:` (`undocumented_unsafe_blocks`) and runs miri (RUST-IDIOM-04).
- No `static mut`, no `std::env::set_var`, no `lazy_static`: use `LazyLock`/`OnceLock`, read config once (RUST-IDIOM-15).

## Ownership and types
- Immutable by default; take `&T`, `&str`, `&[T]`, return owned; `Arc<T>` instead of deep clones; no `.clone()` just to silence the borrow checker (RUST-IDIOM-06).
- `Option` for absence, `ok_or_else`, `let ... else`, let chains (edition 2024) (RUST-IDIOM-05).
- Enums and exhaustive `match`; avoid `_ =>` on your own enums; `#[non_exhaustive]` on growing public types (RUST-IDIOM-07).
- RAII: release in `Drop`, but no `.await` in `drop`: give async resources an explicit `async fn close(self)`; no `mem::forget` or `Box::leak` on request paths (RUST-IDIOM-08).
- Generics and traits before macros; `macro_rules!` only when a function cannot work; proc macros in their own crate, never in domain code (RUST-IDIOM-12).

## Data details
- Strings are UTF-8: never byte-slice unless ASCII; `char_indices`, `str::get`, `unicode-segmentation`; `Path`/`OsString` for file names (RUST-IDIOM-09).
- Time: `time` or `chrono` in UTC, `Duration` for spans, `Instant` for elapsed time, a `Clock` port; derive `PartialEq`+`Eq`+`Hash` together; no floats as map keys; `BTreeMap` or sorted output for determinism (RUST-IDIOM-10).
- Integer overflow wraps silently in release: `checked_*`/`saturating_*`, `overflow-checks = true` for services, `try_from` instead of narrowing `as` (RUST-IDIOM-13).
- Money in `rust_decimal` or integer minor units, never `f64` (RUST-DATA-07).

## Naming and API shape
RFC 430 casing; `as_`/`to_`/`into_`; getters without `get_`; `iter`/`iter_mut`/`into_iter`; constructors `new`/`with_*`/`from_*`; a `Builder` for many optional fields; public types implement `Debug`; fields private (RUST-IDIOM-11).

## Async footguns
Mutex or `RefCell` guard across `.await` is the most common async bug (RUST-IDIOM-14); see [concurrency.md](concurrency.md).
