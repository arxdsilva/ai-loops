---
name: lang-python
description: Use when writing, reviewing, testing or setting up Python backend code in a loops run (the python language pack) - toolchain, layout, race policy, coverage, idioms and footguns.
---

# Python language pack

Rules: `specs/languages/python/RULES.md` (IDs `PY-<SEC>-NN`). Pack data: `specs/languages/python/pack.json` (`loops lang show python`). Layers, ports, errors and consistency are language-neutral: `specs/ARCHITECTURE.md`, `skills/hexagonal`, `skills/consistency`. Gate, roles and the race rule: `specs/WORKFLOW.md §6`.

## 1. When it applies

`loops lang detect` reports `python` for a root (`pyproject.toml`, `uv.lock` or `requirements.txt`). Several packs can apply to one repo: gate each root with its own pack.

## 2. Project rules first

The repository's own configuration wins over the pack defaults: `pyproject.toml` (`[tool.ruff]`, `[tool.mypy]`, `[tool.pytest.ini_options]`, `[tool.coverage]`, `[tool.importlinter]`), `ruff.toml`, `mypy.ini`, `pyrightconfig.json`, `tox.ini`, `.pre-commit-config.yaml`, framework conventions (Django apps, Flask blueprints). Read them before writing code (`config_files` in the pack). Poetry, pip-tools, black, flake8, unittest or a flat layout are legitimate project choices: follow them and log each deviation with `loops run override --rule "<loops rule>" --project "<project rule, file:line>" --note "<what you did>"`.

## 3. The toolchain the gate runs

`loops gate` runs these in order (agents may run the single commands while iterating; QA scopes with `--only`/`--pkgs`; `{pkgs}` = `src`, `{pkg_dirs}` = `.`):

| Check | Command | Required |
|---|---|---|
| `sync` (deps) | `uv sync --locked` when `uv.lock` exists, else `python -m pip install -e . --group dev` | no |
| `ruff-format` | `ruff format --check .` | only when ruff config is committed (D14) |
| `ruff` | `ruff check --output-format=concise .` | yes |
| `mypy` / `pyright` | `mypy --strict src` / `pyright` (the one the project configures; each skips if not installed) | no |
| `import-linter` (arch) | `lint-imports`, when `.importlinter` exists | no |
| `test` | `pytest -p randomly -n 2 -W error --cov --cov-branch --cov-report=xml:{coverage}` with `PYTHONDEVMODE=1` | yes |
| `coverage` | new-code coverage >= 70% from the Cobertura file (loops computes it) | yes |
| `bandit` | `bandit -r src -ll -q` | no |
| `pip-audit` | `pip-audit --locked .`, only with `--vuln` | off by default |

A missing optional tool skips with its install hint (D12); missing pytest, pytest-cov, pytest-randomly or pytest-xdist fails the required `test` check. Tools must resolve on PATH: activate `.venv`, or run the gate as `uv run loops gate`. After a file edit the post-edit hook runs `ruff format <file>` and `ruff check <dir>`. See `PY-TOOL-03..10`, `PY-REPO-04/05`.

Run `uv sync --locked` (never plain `uv sync`) so a stale lock breaks the build (`PY-REPO-05`).

## 4. Race policy: `none` plus the substitute

CPython has no data-race detector, so the required `test` check is the substitute (`PY-CONC-05/06`, WORKFLOW §6): random order (`-p randomly`, printed seed), two xdist workers (`-n 2`), warnings as errors (`-W error`), dev mode (`PYTHONDEVMODE=1`: asyncio debug, `ResourceWarning`). To satisfy it:
- never pass `-p no:randomly`, a fixed order, a retry plugin or `-n 0` to the gate run;
- reproduce a failure with `--randomly-seed=<seed from the output>`; fix the shared state, never the flag;
- for changed test files, also run them under two other `--randomly-seed` values;
- threaded code on a free-threaded interpreter (3.14t/3.15t): add `PYTHON_GIL=0 pytest --parallel-threads=4 --iterations=10` (`concurrency.md`);
- if the host cannot run the substitute (plugin missing, no pytest-xdist), the gate fails with a preflight message; install it, do not skip.

## 5. Coverage recipe

Cobertura XML at `{coverage}`, branch coverage on. Project prerequisites the architect or `python-engineer` adds in wave 0 (D13, logged as an override if the project differs):
- `[dependency-groups]` with `test = [pytest, pytest-cov, pytest-randomly, pytest-xdist]` and `lint = [mypy, import-linter, bandit]`; `uv.lock` committed;
- `[tool.coverage.run] source = ["src"]`, `branch = true`; `[tool.coverage.report] exclude_also = ["if TYPE_CHECKING:", "raise NotImplementedError", "\\.\\.\\."]`;
- `[tool.pytest.ini_options]`: `testpaths = ["tests"]`, `addopts = ["--import-mode=importlib", "--strict-markers", "--strict-config"]`, `filterwarnings = ["error"]`;
- `[tool.ruff]` with explicit rule selection (`PY-TOOL-04`), `[tool.mypy] strict = true`, `.importlinter`, `.python-version`, `.gitignore` entries (`PY-REPO-13`).

Excluded from the gate's coverage: tests, `conftest.py`, `*.pyi`, migrations, `compose.py`, `main.py`, `__main__.py`, generated files. Do not add `omit` entries to hit a number (`PY-TEST-09`).

## 6. Layer map

```
src/<pkg>/
  domain/        pure: frozen dataclasses, enums, errors; no I/O, no clock, no os/requests/sqlalchemy/pydantic
  application/   use cases + ports (typing.Protocol, owned by the consumer)
  adapters/
    inbound/     http (FastAPI/Flask routers), cli, worker
    outbound/    db (SQLAlchemy, Alembic), clock, http clients
  compose.py     composition root: build_app(settings)
  main.py        create_app() only
tests/unit  tests/integration  conftest.py
```

`loops arch` does not parse Python (`arch.parser: none`): `import-linter` enforces direction, forbidden imports and cycles. Purity, clock and RNG injection stay review items (`PY-ARCH-07`). Django keeps its app layout (`frameworks.md`).

## 7. Footguns

1. Mutable default arguments: `None` plus create inside, or `default_factory`.
2. Blocking calls in `async def`: `httpx.AsyncClient` or `asyncio.to_thread`.
3. Bare `except`, swallowed `CancelledError`: narrow the type, re-raise cancellation.
4. `asyncio.create_task` without keeping a reference: `TaskGroup`.
5. `pickle`/`yaml.load` on untrusted data: JSON, `safe_load`, Pydantic.
6. Naive or `utcnow()` datetimes: `datetime.now(UTC)` via an injected clock.
7. `assert` for validation: raise a real exception.
8. Import-time side effects and cycles: wire in `compose.py`, enforce with import-linter.
9. Float money, mutable hash keys: `Decimal`, frozen dataclasses.
10. "The GIL makes it thread-safe": it does not; use a `Lock` or a queue.

## 8. Load on demand

- `repo.md`: layout, pyproject, uv, lockfile, CI order, hygiene (`PY-REPO`, `PY-TOOL`).
- `architecture.md`: hexagon in Python, Protocol ports, import-linter contracts (`PY-ARCH`).
- `idioms.md`: errors, typing, dataclasses, datetimes, money, imports (`PY-IDIOM`, `PY-PERF`).
- `testing.md`: pytest layout, fixtures, fakes, randomly/xdist, coverage (`PY-TEST`).
- `concurrency.md`: asyncio, threads, free-threading, the substitute check (`PY-CONC`).
- `security.md`: injection, deserialisation, SSRF, secrets, supply chain (`PY-SEC`).
- `data-api.md`: SQLAlchemy, migrations, HTTP, logging, metrics, config, packaging (`PY-DATA`, `PY-API`, `PY-OBS`, `PY-OPS`).
- `frameworks.md`: FastAPI, Django, Flask conventions (`PY-FW`, `PY-DOC`).
