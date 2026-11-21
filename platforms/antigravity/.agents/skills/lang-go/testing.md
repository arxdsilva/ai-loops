# Go testing digest

Rules: `specs/languages/go/RULES.md §30, §31, §13, §20`. Universal: `specs/TESTING.md`, `skills/testing`. Research IDs `GO-TEST-01..15`, `GO-PERF-01..07`.

## Layout and style (GO-TEST-01..03, 13)
- Tests beside the code in `x_test.go`, named `TestType_Behaviour`; `package x_test` for API-level tests; fixtures in `testdata/`.
- Standard `testing` with `if got != want { t.Errorf(...) }` and `go-cmp` for diffs; `testify/require` only if the project already uses it.
- Table tests with named subtests (`t.Run(tc.name, ...)`) when rows share structure; not for rows needing unrelated setup.
- `t.Helper()`, `t.Cleanup`, `t.TempDir()`, `t.Setenv`. Loop variables are per iteration (1.22): no `tc := tc`.
- A bug fix starts with a failing test. Assert behaviour, not log text.

## Context, time, randomness (GO-TEST-04, 11, GO-CONC-10)
- Start goroutines and calls from `t.Context()` (cancelled just before `Cleanup`).
- Inject the clock and RNG; no `time.Sleep` for synchronisation. For timers and goroutines use `testing/synctest` (1.25: `synctest.Test`, `synctest.Wait`; 1.27: `synctest.Sleep`). `sync.Mutex` and real I/O are not bubble-aware. `testing/cryptotest.SetGlobalRandom` (1.26) for crypto randomness.

## Doubles (GO-TEST-05)
Hand-written fakes for consumer-owned ports (in `_test.go` or a `<pkg>test` package); prefer an in-memory fake repository. Generate mocks (`mockery`, `gomock`) only when the call sequence is the point.

## Run flags (GO-TEST-10, 14)
Gate: `go test -race -count=1 ...`. Add `-shuffle=on` locally and in CI (print the seed; tests must not depend on order). Iterating: `go test -race -run 'TestName' ./pkg`, `-failfast`, `-skip`, `-short`. `go test -json` for tooling; `gotestsum` for readable CI output (still `-race`). `-artifacts` plus `t.ArtifactDir()` (1.26) keeps test outputs. `t.Parallel()` only without shared mutable state.

## Coverage (GO-TEST-06..08)
- Gate recipe: `-covermode=atomic -coverprofile=cover.out`; inspect with `go tool cover -func cover.out`. New-code coverage is computed by loops against `git diff` (min 70%); `diff-cover` does not read Go profiles.
- No `-coverpkg` in the recipe, so cross-package tests do not credit a package. Adding `-coverpkg=./...` credits them but changes the numbers; keep it consistent between runs.
- Integration binaries: `go build -cover`, run with `GOCOVERDIR=dir`, `go tool covdata textfmt -i=dir -o=cover.txt`; an unrecovered panic loses the data; use `-coverpkg=<module>` rather than `main`.
- Excluded from the gate: `cmd/**`, `internal/compose/**`, `**/testdata/**`, generated code.

## Integration (GO-TEST-12)
Real dependencies: `httptest.NewServer` and `NewRecorder` for handlers, `testcontainers-go` for Postgres or Kafka, transaction-per-test or schema-per-test isolation. Gate behind `//go:build integration` or `-short`; CI runs `-tags=integration`, and `go vet -tags=integration ./...` compiles those files (plain `go build` and `go vet` do not).

## Fuzz and property tests (GO-TEST-09)
`func FuzzX(f *testing.F)`; run `go test -fuzz=FuzzX -fuzztime=30s ./pkg`; commit minimised failures under `testdata/fuzz/`. Targets: parsers, decoders, validators, money math, DTO round trips. Properties: `pgregory.net/rapid` (`testing/quick` is frozen). Mutation testing (`go-mutesting`, `gremlins`) is optional and nightly only (GO-TEST-15).

## Benchmarks and profiling (RULES §31)
```go
func BenchmarkQuote(b *testing.B) {
    b.ReportAllocs()
    in := setup()          // before the loop: not timed
    for b.Loop() {         // 1.24; keeps results alive; no ResetTimer
        _ = Quote(in)
    }
}
```
`-count=10` then `benchstat old.txt new.txt`. Profile: `go test -cpuprofile cpu.out -memprofile mem.out -bench .`, `go tool pprof -http=:0 cpu.out`, `runtime/trace`. PGO: commit `default.pgo` next to the main package (2-14%, refresh per release). `GOMEMLIMIT` below the container limit. Green Tea GC is the default in 1.26.

## Race and leaks
`concurrency.md`. Goroutine leaks: `goleak.VerifyTestMain(m)`; production `goroutineleak` pprof profile (1.27).
