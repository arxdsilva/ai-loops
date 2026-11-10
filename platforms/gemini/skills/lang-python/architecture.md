# Python: architecture

Digest of RULES `PY-ARCH`, with `PY-DATA-02` and `PY-OPS-01`. Layer concepts live in `specs/ARCHITECTURE.md`; this file only maps them.

## Mapping

- `domain` -> `application` -> `adapters`; dependencies point inward (`PY-ARCH-01`). Modules are dotted paths under `src/` (`orders.domain.order` = `src/orders/domain/order.py`) (`PY-ARCH-02`).
- Ports are `typing.Protocol` in the application layer, owned by the consumer; adapters satisfy them structurally. `abc.ABC` only for shared behaviour (`PY-ARCH-08`).
- Keep ports to 1-5 methods; one use case per module; functions about 40 lines, up to 5 parameters (keyword-only after `*`), modules under about 400 lines (`PY-ARCH-09`, `specs/SOLID.md`).
- Domain objects are `@dataclass(frozen=True, slots=True)`. Domain events are frozen dataclasses raised by aggregates, dispatched after commit; outbox across services (`PY-ARCH-11`).
- Transport schemas (Pydantic, DRF serializers) and ORM models stay in adapters; map at the edge (`PY-ARCH-10`, `PY-DATA-02`).
- Wire by hand in `compose.py` (`build_app(settings)`), or `Depends` at the inbound edge only; no service locator, no connections opened at import (`PY-ARCH-06`).
- Config is read once into a typed `pydantic-settings` object in the composition root; the domain never reads `os.environ` (`PY-OPS-01`).
- Start as a modular monolith with one package per bounded context and `independence` contracts (`PY-ARCH-12`).

## Purity

No `os`, `requests`, `httpx`, `sqlalchemy`, `django`, `fastapi`, `flask`, `boto3`, `pydantic`, `subprocess`, `socket` in the domain, and no `datetime.now()`, `time.time()`, `random.*`, `uuid.uuid4()`, `os.environ` (inject `Clock` and `IdGenerator`) (`PY-ARCH-07`). These lists are in the pack's `arch` block, but `loops arch` does not scan Python: enforce them with the import-linter `forbidden` contract and in review.

## import-linter

Python has no visibility modifiers (underscore = private, `__all__` = public API) and cycles are legal but fail at import, so forbid them (`PY-ARCH-04`). `loops arch` reports nothing for Python (`parser: none`); the gate's `import-linter` check runs `lint-imports` when `.importlinter` exists (`PY-ARCH-03/05`):

```ini
[importlinter]
root_package = orders

[importlinter:contract:layers]
name = Hexagon layers
type = layers
layers =
    orders.adapters
    orders.application
    orders.domain

[importlinter:contract:pure-domain]
name = Domain is pure
type = forbidden
source_modules = orders.domain
forbidden_modules = sqlalchemy, fastapi, django, requests, httpx, pydantic
```

Add `independence` contracts between bounded contexts and an `acyclic_siblings` contract where useful. A project that already configures import-linter in `pyproject.toml` or `setup.cfg` keeps it; then enable the check through config, since the pack gates on `.importlinter`.

## Django

Django apps own models, ORM and admin, so a pure hexagon fights the framework: follow the project layout, keep domain logic in a plain-Python `domain/` or `services/` package per app, treat models and views as adapters, let apps import each other only through a service API, and enforce with import-linter (`PY-FW-03`, `frameworks.md`).
