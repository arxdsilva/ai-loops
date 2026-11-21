# Python: concurrency and the race substitute

Digest of RULES `PY-CONC`. Job and state-machine design: `skills/jobs-and-state-machines`.

## Race policy: `none`

There is no flag-enabled race detector for CPython. The pack's required `test` check is the substitute (WORKFLOW §6, `PY-CONC-05/06`):

| Tool | Finds | Does not find |
|---|---|---|
| `pytest-randomly` | order dependence, shared module state | races |
| `pytest-xdist -n 2` | hidden shared files, DBs, ports | races |
| `-W error`, `PYTHONDEVMODE=1` | unawaited coroutines, `ResourceWarning`, slow callbacks, thread misuse | races |
| `pytest-run-parallel` on a free-threaded build | thread races in the code under test | races in C extensions that re-enable the GIL |
| TSan CPython build (`--disable-gil --with-thread-sanitizer`, no xdist) | data races | needs a custom interpreter; advisory |

Optional free-threaded run, only when the project uses threads and has a 3.14t/3.15t interpreter: `PYTHON_GIL=0 pytest --parallel-threads=4 --iterations=10` (`--skip-thread-unsafe` as needed). Check the build with `sysconfig.get_config_var("Py_GIL_DISABLED")` and `sys._is_gil_enabled()`; an unsupported C extension re-enables the GIL unless `PYTHON_GIL=0` (`PY-CONC-02`). Run threaded tests with a timeout (`pytest-timeout`, `faulthandler_timeout`) and a small `sys.setswitchinterval` (`PY-CONC-12`). Language-specific stress tests are review items, not gate checks.

## Choosing a model (`PY-CONC-01`)

`asyncio` for I/O fan-out; `ThreadPoolExecutor` for blocking libraries; `ProcessPoolExecutor` for CPU-bound work; `concurrent.interpreters` (3.14) is the new option. The GIL serialises bytecode but does not make `x += 1`, check-then-act or lazy init atomic: guard shared state with `threading.Lock` or confine it and pass messages through `queue.Queue`/`asyncio.Queue` (`PY-CONC-03`). Prefer immutability; `contextvars` over globals for request context, `threading.local` only for thread-affine resources (`PY-CONC-04`).

## Asyncio rules

- Never block inside `async def` (`requests`, `time.sleep`, sync DB drivers, file I/O, `subprocess.run`): async equivalents or `await asyncio.to_thread(...)` [ASYNC] (`PY-CONC-07`).
- `asyncio.TaskGroup` for structured concurrency, `except*` for `ExceptionGroup`; keep a reference to every `create_task` [RUF006]; `gather` on long-lived work needs `return_exceptions` (`PY-CONC-08`).
- Wrap every outbound await in `async with asyncio.timeout(s)`; catch `TimeoutError` outside the block; re-raise `CancelledError`; do not call `uncancel()` in application code (`PY-CONC-09`).
- Bound concurrency with `Semaphore` or `Queue(maxsize=n)`; never one task per unbounded input (`PY-CONC-10`).
- FastAPI: `async def` only for awaitable code, plain `def` for blocking (runs in the threadpool) (`PY-FW-02`).

## Background work

Use a job framework (Celery, Dramatiq, arq, DB queue); idempotent handlers, ack after success, drain on `SIGTERM` (`PY-CONC-11`).
