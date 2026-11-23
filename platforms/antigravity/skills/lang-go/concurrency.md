# Go concurrency and race digest

Rules: `specs/languages/go/RULES.md §10-§13`. Race policy: `specs/WORKFLOW.md §6`. Research IDs `GO-CONC-01..10`.

## The race policy for Go
Mode `flag`: every `go test` has `-race` (gate: `go test -race -count=1 ...`). Never `CGO_ENABLED=0` for tests; `-race` needs cgo and a C compiler (`golang:<ver>` has gcc, alpine needs `gcc musl-dev`). The detector finds only races on executed paths and costs 5-10x memory, 2-20x time: give generous timeouts, run packages in parallel (`-p`), do not drop `-race`. For a long-lived service also run a `go build -race` binary under load. Options through `GORACE` (for example `GORACE="halt_on_error=1"`).

## Race triage
1. Read the two stacks: the access, the previous access, and where each goroutine was created.
2. Find the owner of the state. Fix by moving ownership (channel, single goroutine, immutable snapshot) before reaching for a mutex.
3. If a mutex: small protected state, documented lock order, never held across I/O, never expose the protected slice or map.
4. Re-run the failing test with `-race -count=20` and then the package.
Never add `time.Sleep` to hide it, never mark the test flaky.

## Ownership (GO-CONC-02, 03, 07, 08)
- One owner per piece of mutable state; share by communicating, or guard with a small `Mutex`/`RWMutex`. Typed atomics (`atomic.Int64`, `atomic.Pointer[T]`) at boundaries.
- Every goroutine has an owner that waits or cancels: `errgroup.WithContext` with `SetLimit`, or `sync.WaitGroup` with `wg.Go(func(){...})` (1.25). Never `wg.Add` inside the goroutine; vet's `waitgroup` analyzer reports it.
- Pass `ctx` to every blocking call; `select` on `ctx.Done()` when sending or receiving; bound channels and pools; close channels only from the sender.
- Loop variables are per iteration since 1.22. Never copy a lock (`copylocks`). No `go` in domain; workers compute on immutable inputs and the owner commits results in deterministic order (`skills/consistency`).

## Runtime (GO-CONC-01)
Goroutines run on OS threads (GMP). `GOMAXPROCS` follows the cgroup CPU limit and updates dynamically since 1.25: drop `uber-go/automaxprocs`. `runtime.NumCPU()` must not change results.

## Shutdown (GO-CONC-09, GO-OPS-04)
`signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)` -> readiness flips to failing -> stop accepting -> `http.Server.Shutdown(ctx)` with a timeout below `terminationGracePeriodSeconds` -> drain workers -> close pools -> exit 0.

## Leaks (GO-CONC-06)
Tests: `goleak.VerifyTestMain(m)`. Production: `/debug/pprof/goroutineleak` (1.27, GA). A leak is not a data race; neither tool covers the other.

## Time-dependent tests (GO-CONC-10)
`testing/synctest`: `synctest.Test(t, func(t *testing.T){ ... })` runs in a bubble with a fake clock; `synctest.Wait()` blocks until all bubble goroutines are idle; `synctest.Sleep` (1.27). Authoritative code still takes an injected `Clock`.

## Channels vs mutex
Channels where an ownership boundary benefits (pipelines, work distribution); a mutex for a small shared structure; never a channel as a function-call replacement.
