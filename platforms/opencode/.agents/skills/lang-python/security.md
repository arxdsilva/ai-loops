# Python: security

Digest of RULES `PY-SEC`. Language-neutral threat handling: `skills/consistency` and `specs/ARCHITECTURE.md`. Ruff `S` rules and `bandit` automate most of this.

## Injection and execution (`PY-SEC-01/02/03`)

- SQL: bind parameters (`cursor.execute(sql, params)`, SQLAlchemy `text()` with binds, the Django ORM); never f-strings or `%` into SQL [S608].
- Processes: argv lists, never `shell=True` or `os.system` on input [S602, S605]. Jinja2 autoescape on.
- No `pickle.loads`, `marshal`, `shelve`, `yaml.load` (use `safe_load`), `torch.load` without `weights_only=True` on untrusted data [bandit B301/B506]. Use JSON or a schema (Pydantic, msgpack).
- XML through `defusedxml`; never `eval`, `exec`, `compile` on input.

## Network and files (`PY-SEC-04/05/10`)

- SSRF: allow-list outbound URLs, resolve and re-check IPs, `follow_redirects=False` unless needed.
- Path traversal: `Path.resolve()` plus `is_relative_to(base)`; `tarfile.extractall(..., filter="data")` (3.12+); no unchecked archive extraction.
- CSRF and CORS: Django keeps `CsrfViewMiddleware`; cookie-authenticated FastAPI/Flask APIs need CSRF tokens; never `allow_origins=["*"]` with credentials.
- Production: Django `DEBUG=False`, `ALLOWED_HOSTS`, `manage.py check --deploy`; Flask `debug=False`; FastAPI docs off if not public; never `verify=False` [B501].

## Secrets and crypto (`PY-SEC-06`)

Secrets from env or a vault through typed settings, never committed. `secrets` for tokens (not `random`), `argon2-cffi`/`bcrypt`/`scrypt` for passwords (not `hashlib.md5/sha1`), `hmac.compare_digest` for comparisons.

## Validation (`PY-SEC-07`)

Validate once at the adapter with Pydantic v2 (strict types, `extra="forbid"`, length bounds); the domain receives valid value objects.

## Supply chain and scanning (`PY-SEC-08/09`, `PY-TOOL-10`)

- Install from the lock (`uv sync --locked`) or with `--require-hashes`; verify names (typosquatting); prefer wheels (sdists run build code); pin GitHub Actions by SHA; publish with Trusted Publishing and attestations.
- Gate checks: `bandit -r src -ll -q` (medium and above) runs when installed; `pip-audit --locked .` is off by default and runs with `loops gate --vuln`. Semgrep (`p/python`) is an optional extra.
- A finding is fixed or recorded with its ticket; never suppress it to pass the gate (`# nosec` needs a reason).
