# Python: repository and toolchain

Digest of `specs/languages/python/RULES.md` REPO and TOOL. Project config wins; log overrides (SKILL §2).

## Canonical tree (`PY-REPO-01`)

```
service/
  pyproject.toml   uv.lock   .python-version   .gitignore   .editorconfig   .pre-commit-config.yaml
  README.md  CHANGELOG.md  Dockerfile  .dockerignore  .importlinter
  src/<pkg>/   tests/{unit,integration}/conftest.py   migrations/   scripts/
  .github/workflows/ci.yml   .github/dependabot.yml
```

## Project and dependencies

- Metadata only in `pyproject.toml` (PEP 621); no `setup.py`/`setup.cfg` for new code (`PY-REPO-02`).
- Dev, test and lint dependencies in PEP 735 `[dependency-groups]`, not extras (`PY-REPO-03`). The pack's non-uv fallback is `python -m pip install -e . --group dev` (pip 25.1+).
- uv: `uv init`, `uv add`, `uv lock`, `uv sync --locked`, `uv run`; commit `uv.lock`, never `.venv` (`PY-REPO-04`). CI uses `--locked`, never plain `uv sync` (`PY-REPO-05`).
- Without uv: pip-tools `--generate-hashes` or Poetry, one tool per repo, `--require-hashes` (`PY-REPO-06`). `uv export --format pylock.toml` yields the PEP 751 interchange file; never hand-edit it (`PY-REPO-07`).
- Libraries declare ranges, applications get exact versions from the lock (`PY-REPO-08`).
- Pin the interpreter in `.python-version` and `requires-python`; test the supported matrix (3.12 to 3.15; 3.10 is EOL, `PY-TOOL-01`, `PY-REPO-09`). `uv python install` provisions it (`PY-TOOL-02`).
- uv workspaces for interdependent packages (`PY-REPO-10`); Dependabot or Renovate weekly (`PY-REPO-11`).

## Gate tools

- Format: `ruff format`; drop black and isort (`ruff check --select I`) (`PY-TOOL-03`). The gate's `ruff-format` is required once a ruff config is committed (variants `ruff-toml`, `dot-ruff-toml`, `ruff-pyproject`).
- Lint: `ruff check` with an explicit rule set (`E, F, W, I, B, UP, SIM, C4, PT, RUF, S, ASYNC, PERF, TID, ARG`); defaults miss bugbear and async rules. `--fix` only locally, safe fixes only (`PY-TOOL-04/05`). Config in `[tool.ruff]`; nearest config wins, use `extend` (`PY-TOOL-06`).
- Types: strict. `mypy --strict` or pyright `typeCheckingMode = "strict"`; one checker is the gate. Legacy code: `strict = true` per package via `[[tool.mypy.overrides]]`, `ignore[code]` only, `warn_unused_ignores` (`PY-TOOL-07/08`). Modern syntax: `X | None`, builtin generics, PEP 695 (`PY-TOOL-09`).
- Vulnerabilities: `pip-audit --locked .` (opt in with `loops gate --vuln`; `-f cyclonedx-json` for SBOMs) (`PY-TOOL-10`). Licences with `pip-licenses` when policy asks (`PY-TOOL-11`).

## Hygiene

- `.gitignore`: `__pycache__/`, `*.py[cod]`, `.venv/`, `.pytest_cache/`, `.mypy_cache/`, `.ruff_cache/`, `.coverage*`, `coverage.xml`, `htmlcov/`, `dist/`, `*.egg-info/` (`PY-REPO-13`). Cleanup after a run removes only new, untracked artifacts (`PY-OPS-07`).
- Pre-commit and CI run the same commands (`PY-REPO-14`); CI order: sync, format, lint, types, import-linter, tests with coverage, pip-audit, build (`PY-REPO-15`).
- Release with a `v<version>` tag, `uv build`, PyPI Trusted Publishing; conventional commits (`PY-REPO-12/16`).
- Editor: Pylance/pyright or basedpyright, `ruff server`; one formatter on save (`PY-TOOL-12`).

## Wave 0 checklist (python-engineer)

Manifest and groups, lock, `.python-version`, ruff/mypy/pytest/coverage config, `.importlinter`, `.gitignore`, CI file: see SKILL §5. Never edit these outside wave 0 or a language lane.
