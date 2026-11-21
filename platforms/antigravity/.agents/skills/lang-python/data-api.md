# Python: data, API, observability, operations

Digest of RULES `PY-DATA`, `PY-API`, `PY-OBS`, `PY-OPS`. Consistency, outbox and idempotency design: `skills/consistency`, `skills/jobs-and-state-machines`.

## Persistence

- SQLAlchemy 2.x (typed `Mapped[...]`, `select()`) behind a repository port; Django ORM in Django projects (`PY-DATA-01`). ORM entities stay in the outbound adapter and map to domain dataclasses; `Base` never reaches `domain/` (`PY-DATA-02`).
- Alembic (`revision --autogenerate`, then review by hand) or Django migrations; committed, immutable once released, applied by a release step, not at app start (`PY-DATA-03`). Expand then contract across two releases; indexes `CONCURRENTLY` on Postgres (`PY-DATA-04`).
- One transaction per command, opened in the application layer (unit-of-work context manager); repositories never commit; `transaction.atomic` in Django (`PY-DATA-05`).
- Kill N+1: `selectinload`/`joinedload`, `select_related`/`prefetch_related`; `lazy="raise"` in async code; assert query counts in tests (`PY-DATA-06`).
- Pool connections (`pool_size`, `pool_pre_ping=True`, `pool_recycle`; `CONN_MAX_AGE`); async drivers with `AsyncSession` only in async stacks; statement and connect timeouts (`PY-DATA-07`). `expire_on_commit=False` for async sessions; never share a session across tasks or threads (`PY-DATA-08`).

## API

- Default stack: FastAPI on Starlette/Uvicorn (or Gunicorn with Uvicorn workers); Django with DRF or Ninja; Flask for small services. Routers are inbound adapters (`PY-API-01`).
- Code-first OpenAPI; export it in CI and diff it for breaking changes (`PY-API-02`). Version in URL or header; gRPC via `grpcio` + `buf`; GraphQL via Strawberry (`PY-API-03`).
- RFC 9457 `application/problem+json` from one exception handler per framework; map domain errors to status codes at the adapter; keep the 422 shape consistent (`PY-API-04`).
- Every outbound call: explicit connect and read timeouts, retries with backoff and jitter only for idempotent requests (`tenacity`), circuit breaker for hot dependencies, `Idempotency-Key` on unsafe endpoints [S113] (`PY-API-05`). Rate limiting at the gateway or `slowapi` (`PY-API-06`).
- Messaging: outbox table in the same transaction, relay publishes, consumers deduplicate (`PY-API-07`).
- One long-lived `httpx.Client`/`AsyncClient` built in the composition root and closed on shutdown; propagate deadlines and correlation IDs (`PY-API-08`).

## Observability

- Stdlib `logging` configured once at startup, JSON via `structlog` or `python-json-logger`; `getLogger(__name__)`; no `print`; lazy args; no secrets or PII [T20, G] (`PY-OBS-01`). Correlation IDs through `contextvars` (`PY-OBS-02`).
- `prometheus-client` or OpenTelemetry metrics: `snake_case`, base units, `_total` counters, low-cardinality labels (`PY-OBS-03`). OpenTelemetry tracing (traces and metrics stable, logs still maturing as of 2026-11), OTLP export (`PY-OBS-04`).
- `/healthz` (liveness, no dependencies) and `/readyz` (checks DB and broker) (`PY-OBS-05`).

## Operations

- Config from the environment into one typed `pydantic-settings` object built in the composition root (`PY-OPS-01`).
- Libraries: `uv build`. Services: multi-stage image, builder runs `uv sync --locked --no-install-project --no-editable` then `uv sync --locked --no-editable`, final stage copies only `.venv` onto `python:3.x-slim` or distroless, non-root (`PY-OPS-02`). Container env: `PYTHONDONTWRITEBYTECODE=1`, `PYTHONUNBUFFERED=1`, `UV_LINK_MODE=copy`, `UV_PYTHON_DOWNLOADS=0`; `.venv` in `.dockerignore` (`PY-OPS-03`).
- PyInstaller, Nuitka, zipapp only for CLIs and desktop; do not expect AOT speed-ups (`PY-OPS-04`).
- Handle `SIGTERM`: ASGI servers drain in-flight requests, FastAPI `lifespan` opens and closes pools, PID 1 via exec-form `CMD` or `tini` (`PY-OPS-05`).
- Run caches and artifacts: `PY-OPS-06/07` and SKILL §3/§5. Download caches (`UV_CACHE_DIR`, `PIP_CACHE_DIR`) are shared, never scoped.
