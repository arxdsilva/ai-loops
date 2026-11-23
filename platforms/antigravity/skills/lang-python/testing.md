# Python: testing

Digest of RULES `PY-TEST`. Pyramid, determinism and the 70% rule: `skills/testing` concepts, `specs/WORKFLOW.md §6`; Python specifics here.

## Layout and discovery

- pytest is the only runner; it also runs `unittest.TestCase` (`PY-TEST-01`).
- Tests live in top-level `tests/{unit,integration}`; files `test_*.py` or `*_test.py`, functions `test_*`, classes `Test*` without `__init__`. Set `testpaths = ["tests"]` and `--import-mode=importlib` (`PY-TEST-02`). Pack test globs: `**/test_*.py`, `**/*_test.py`, `**/conftest.py`, `tests/**` (`PY-TEST-03`).
- `--strict-markers --strict-config` (or `strict = true` on pytest 9), register every marker, `filterwarnings = ["error"]` (`PY-TEST-15`).

## Style

- `@pytest.mark.parametrize` with `pytest.param(..., id="case")` in place of table loops (`PY-TEST-04`).
- Fixtures: narrowest scope that works (function default), `yield` for teardown, shared ones in `conftest.py`; arrange-act-assert; `test_<unit>_<behaviour>` (`PY-TEST-05`).
- Fakes over mocks: in-memory adapters implementing the port (`specs/ARCHITECTURE.md` testing section). `unittest.mock`/`pytest-mock` only at process boundaries, with `autospec=True`; do not patch what you own (`PY-TEST-06`).
- Integration tests use real dependencies via Testcontainers or compose, one DB per session, per-test rollback or truncation (`PY-TEST-07`).
- Property tests with Hypothesis for parsers, serialisers, state machines, money; `settings(derandomize=True)` in CI or a pinned `--hypothesis-seed` (`PY-TEST-11`). Mutation testing (`mutmut`) is occasional, never in the per-change gate (`PY-TEST-12`).

## Determinism and parallelism

- `pytest-randomly` shuffles order and reseeds `random`, Faker, NumPy; `--randomly-seed=N` reproduces. Inject clock and RNG ports instead of freezing globals (`PY-TEST-13`).
- `pytest-xdist` `-n 2` is the gate default (`-n auto` on a big box); give each worker its own DB, schema or tmp dir via `worker_id` (`PY-TEST-14`).
- Order dependence or shared state that only appears under the gate flags is a real bug; fix the test (`PY-CONC-06`).

## Coverage

- Gate command: `pytest -p randomly -n 2 -W error --cov --cov-branch --cov-report=xml:{coverage}` with `PYTHONDEVMODE=1`; Cobertura XML, branch rate (`PY-TEST-08`). `[tool.coverage.run] source = ["src"]` is required, else tests count as source.
- `[tool.coverage.report] exclude_also = ["if TYPE_CHECKING:", "raise NotImplementedError", "\\.\\.\\."]`; omit generated code only; never exclude to hit a number (`PY-TEST-09`).
- New code needs at least 70% coverage: `loops gate` intersects the Cobertura file with the diff against `--base` (`PY-TEST-10`).
- Coverage data goes to `{tmp}/.coverage` in a run (`COVERAGE_FILE`); xdist writes `.coverage.*` shards that pytest-cov combines.

## Fast loop

`pytest -x --lf`, `pytest path::test -k expr`, `pytest-testmon` for selection (`PY-TEST-16`). QA scopes the gate: `loops gate --only test,coverage`. Use the single check commands while iterating; the full suite belongs to the lead and QA (`ROLES.md`).

## Django

`pytest-django` with `@pytest.mark.django_db`, `factory_boy` factories, `manage.py makemigrations --check` and `check --deploy` in CI (`PY-FW-05`).
