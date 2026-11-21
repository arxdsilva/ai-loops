# Python: frameworks and documentation

Digest of RULES `PY-FW` and `PY-DOC`. The project's own framework layout wins over loops layouts; log deviations with `loops run override`.

## Choosing (`PY-FW-01`)

FastAPI for typed APIs, Django for batteries-included, admin and ORM, Flask for small or legacy services. Follow what the repo already uses.

## FastAPI (`PY-FW-02`)

- Routers in `adapters/inbound/http`; `Depends` only for adapters and auth.
- `create_app()` with a `lifespan` that wires `compose.py`; no module-level connections.
- `async def` only for awaitable code; plain `def` for blocking work (threadpool).
- `response_model` must not leak ORM fields; schemas are Pydantic at the edge only.

## Django (`PY-FW-03`, `PY-FW-05`)

- Apps, `models.py`, `manage.py` win over loops layouts. Domain logic in a plain-Python `domain/` or `services/` package per app; models and views are adapters; apps import each other only through a service API; enforce with import-linter.
- Tests: `pytest-django`, `@pytest.mark.django_db`, `factory_boy`; CI runs `manage.py makemigrations --check` and `manage.py check --deploy`.
- `ATOMIC_REQUESTS` is a smell for non-trivial flows; use `transaction.atomic` in the use case.

## Flask (`PY-FW-04`)

App factory `create_app`, blueprints as inbound adapters, extensions initialised inside the factory, config from env; no module-level `app` singleton; no `g`/`current_app` outside a context.

## Framework footguns (`PY-FW-06`)

FastAPI: blocking call in `async def`, sync dependency with global state. Django: N+1 in templates and serializers, fat signals, `DEBUG=True` in production, circular app imports. Flask: global state outside the app context.

## Documentation and review

- Docstrings on every public module, class and function (PEP 257, Google or NumPy style; ruff `D`); MkDocs with mkdocstrings, or Sphinx for libraries; doctest only for tiny examples (`PY-DOC-01`).
- README with run, test and deploy commands, ADRs in `docs/adr/`, `CHANGELOG.md` (`PY-DOC-02`); see `skills/documentation`.
- Review focus: `Any` and `# type: ignore`, blocking calls in async code, mutable defaults, naive datetimes, swallowed exceptions, N+1, import-time side effects, untested branches (`PY-DOC-03`).
- Public API changes carry a deprecation first: `warnings.warn(..., DeprecationWarning, stacklevel=2)` or `typing_extensions.deprecated` (PEP 702) (`PY-DOC-04`).
