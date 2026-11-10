# Rust frameworks (digest)

Cites RULES.md. Detect the framework from `Cargo.toml` and follow it; do not introduce a second one (project rules win).

## Choosing (RUST-FW-01)
`axum` is the default for new work (tokio, tower, hyper); `actix-web` is the mature alternative with its own runtime wrapper and middleware API; `tonic` for gRPC; `poem` and `rocket` have a smaller share. `tokio` is the runtime in the large majority of async code. Confirm the current ranking at selection time.

## axum (RUST-FW-02)
- Handlers are async functions returning `Result<impl IntoResponse, ApiError>`; they are the inbound adapter and call one use case.
- Dependencies through `State<AppState>` (`Clone + Send + Sync + 'static`, `Arc`s inside); body-consuming extractor last.
- `Router::merge`/`nest` per bounded context; `ServiceBuilder` layers, outermost first (trace, request id, timeout, body limit, CORS, catch-panic).
- Graceful shutdown with `axum::serve(...).with_graceful_shutdown(...)` (RUST-CONC-10).
- Test with `ServiceExt::oneshot` (RUST-TEST-05).

## actix-web (RUST-FW-03)
- Shared state with `App::new().app_data(web::Data::new(...))`; build the pool once outside the `HttpServer` factory (the factory runs per worker).
- Middleware via `wrap`; `web::block` for blocking work; workers are single-threaded runtimes.
- A mismatched `app_data` type silently yields a 500.

## tower first (RUST-FW-05)
Prefer `tower` services and layers over framework-specific middleware so timeouts, limits and tracing are shared across axum, tonic and client stacks; write a custom layer only when `tower-http` has none.

## Framework footguns (RUST-FW-04)
- axum: extractor order (body last), `Json` rejection returns plain 422/400 unless handled, `State` needs `Clone`, `Mutex` cloned instead of `Arc<Mutex>`, missing timeout layer.
- hyper keep-alive connections leak without timeouts.
- sqlx macros fail offline without `.sqlx/` (RUST-DATA-02).
- None of these is a compile error; they appear as runtime or CI failures, so cover them with integration tests.
