# Go rules (the Go pack's rule book)

Mandatory Go coding standard for every human and AI contributor to a Go backend: idiomatic, explicit, measurable, consistent. This is the rule book of the Go language pack (`specs/languages/go/pack.json`). The architecture mapping is `specs/languages/go/ARCHITECTURE.md`; universal rules are `specs/ARCHITECTURE.md`, `specs/SOLID.md` and `specs/TESTING.md`. Cite by section (`specs/languages/go/RULES.md §8`); §1-§54 are stable because other docs cite them. Targets Go 1.26 and 1.27, as of 2026-11. The appendix maps the `GO-*` rule IDs of `research/languages/go.md` to sections.

Project rules win: follow the project's own config (`.golangci.yml`, `go.mod`, framework conventions) over these defaults, and record each override with `loops run override`.

## 1. Guiding principle

Write Go that is simple, explicit, idiomatic, benchmarkable, consistent where state is authoritative, allocation-conscious in hot paths, easy to test and easy for the next engineer or agent to read. Avoid "enterprise Go" ceremony.

## 2. Formatting and static analysis

Every change passes:

```bash
gofmt -l .          # prints nothing
go vet ./...        # also compiles tests; tag variants (-tags=integration) need their own run
staticcheck ./...   # or golangci-lint v2, see below
go test -race -count=1 -shuffle=on ./...
go mod tidy -diff   # non-zero when go.mod/go.sum need changes
```

- Tests always run with the race detector, locally and in CI. `-race` needs cgo and a C compiler (§37).
- CI enforces formatting; reviewers do not. Use `goimports` or `gofumpt` only if the project standardises on one, preferably through golangci-lint v2 `formatters:`. Re-run `gofmt` after moving to 1.27 (alignment changes can reflow files).
- Linters: `staticcheck`, or golangci-lint **v2** (`version: "2"`, `linters.default: standard`, explicit `enable:`, `formatters:`, `exclusions.generated: lax`). Never mix v1 and v2 configs (`golangci-lint migrate`). Useful set: `errcheck govet staticcheck unused ineffassign errorlint revive gosec sloglint depguard`.
- Pin dev tools in `go.mod` with the `tool` directive (`go get -tool honnef.co/go/tools/cmd/staticcheck@<version>`, run as `go tool staticcheck`) instead of `tools.go` blank imports or `@latest` installs.
- "When installed" is not a gate: in CI make `govulncheck` (`govulncheck ./...`, and `-mode=binary` on the release artifact; with `-format sarif`/`openvex` it exits 0, so gate on the findings yourself) and a SAST step (`gosec -exclude-generated ./...`, or the `gosec` linter) required (§38).
- `go fix -diff ./...` runs the modernizers (1.26+); review the diff periodically.
- `loops gate` runs the pack's checks in order (`specs/WORKFLOW.md §6`); `govulncheck` is off by default (`loops gate --vuln`).

## 3. Package naming

Short, descriptive, lowercase names: `order`, `payment`, `ledger`, `clock`, `ratelimit`.

- Bad: `utils`, `helpers`, `common`, `manager`, `misc`, `core2`.
- Avoid stutter: `package order; type OrderManager` becomes `package order; type Book`.

## 4. Export only what is needed

Default to unexported. Export only when another package has an architectural reason to use the symbol. A smaller API means less coupling, less documentation, less agent context and safer refactors.

## 5. Interfaces

- Use interfaces at real behavioural boundaries; prefer small, consumer-defined ones.
- The consumer owns the interface; the concrete type is constructed only at the composition root (`specs/ARCHITECTURE.md §6`, `specs/SOLID.md §6-7`).
- Do not create a `Fooer` interface for every concrete type.

```go
// Good: the consumer lists the calls it makes.
type OrderReader interface {
    ByID(ctx context.Context, id OrderID) (Order, error)
}

// Bad: a provider-owned mirror of a whole repository.
type OrderService interface { Get(...); Save(...); Delete(...); List(...); Refund(...); Export(...) }
```

Size limits for types, interfaces and packages are advisory warnings from `loops arch` (`specs/SOLID.md §8`).

## 6. Accept interfaces, return concrete types

```go
func NewCheckout(pay PaymentCharger) *Checkout   // good
func NewCheckout(pay PaymentCharger) CheckoutAPI // avoid
```

Return an interface only when the API genuinely needs the abstraction.

## 7. Constructors and zero values

- A constructor establishes invariants (`NewQueue(capacity int) *Queue`). Do not write one that only does `return &T{}`.
- Prefer a useful zero value when natural (counters, empty buffers, optional flags). Do not make the zero value valid if that weakens an invariant.

## 8. Error handling

- Errors signal operation failures. Expected business outcomes are result types or typed values, not `errors.New`.
- Wrap infrastructure errors with context and `%w`.

```go
// Bad
return errors.New("insufficient funds")
// Good: an expected outcome is data
return ChargeResult{Status: Declined, Reason: InsufficientFunds}
// Good: infrastructure failure wraps its cause
return fmt.Errorf("load order %q: %w", id, err)
```

- Taxonomy (domain result, validation, infrastructure): `specs/ARCHITECTURE.md §13`.
- Handle an error once: return it or log it, never both.

## 9. Sentinel and typed errors

Use them only when callers must branch programmatically; a sentinel is right for a programmer-visible condition (`io.EOF`). No large hierarchies. Prefer `errors.Is` and `errors.As`, or `errors.AsType[*MyErr](err)` (1.26). Combine independent failures with `errors.Join`. Never discard an error with `_` without a comment.

## 10. Context

- `context.Context` is for I/O, cancellation, deadlines and long-running operations. It is the first parameter: `func Load(ctx context.Context, ...)`.
- Never store a context in a struct.
- Do not thread it through pure domain functions.
- Every outbound call has a deadline derived from the incoming context.
- Use `context.WithCancelCause` to say why, `context.WithoutCancel` for work that must outlive the request, `context.AfterFunc` for cleanup on cancel. In tests start work from `t.Context()` (1.24; cancelled just before `Cleanup`). `context.Value` is for request-scoped data, not optional parameters.

## 11. Concurrency

Goroutines are not free architecture.

- One owner per piece of mutable state; others get messages or immutable snapshots.
- No goroutine per request item, per order or per job without a bound; use a bounded worker pool.
- Workers compute on immutable inputs and return results; the owner commits them in deterministic order (`skills/consistency`).
- Every goroutine has an owner that waits for it or cancels it; no goroutine outlives its context.
- Use channels where ownership boundaries benefit; do not use them as a replacement for function calls.
- `go` statements appear only in a small number of places (a pool, a server, a consumer loop) and never in `domain`.
- Own goroutines with `errgroup.WithContext` plus `SetLimit`, or `sync.WaitGroup` with `wg.Go(func(){...})` (1.25), which removes the `Add`/`Done` misuse that the `waitgroup` vet analyzer reports. Never call `wg.Add` inside the goroutine.
- Loop variables are per iteration since 1.22; no `v := v` copies. `GOMAXPROCS` follows the cgroup CPU limit since 1.25; drop `automaxprocs`.
- Detect leaks in tests with `goleak` (`goleak.VerifyTestMain`) and in production with the `goroutineleak` pprof profile (GA in 1.27). Graceful shutdown: `signal.NotifyContext` -> stop accepting -> `Server.Shutdown(ctx)` with a deadline -> drain workers -> close pools.

## 12. Mutexes and atomics

- Prefer ownership over shared mutation. If a mutex is needed: keep protected state small, document lock order when there are several locks, never hold a lock across I/O, never expose internal slices or maps while a lock protects them.
- `sync/atomic` belongs at clear concurrency boundaries (snapshot pointer swap, metrics), not sprinkled through domain code. Use the typed values (`atomic.Int64`, `atomic.Pointer[T]`); 1.27's `go fix` has an `atomictypes` modernizer. Never copy a struct that holds a mutex or `WaitGroup` (`copylocks`).
- Run race tests on every concurrent adapter and worker.

## 13. Consistent Go (ordering and clocks)

Authoritative code, meaning anything whose result is stored or acted on, avoids:

```go
time.Now()                  // inject a Clock
math/rand global functions  // inject an RNG or ID source
map iteration for decisions
goroutine completion order
runtime.NumCPU() affecting results
filesystem or directory order
```

- Sort keys when order matters; add an explicit tie-break (`created_at`, then ID).
- For concurrent time-dependent tests use `testing/synctest` (GA in 1.25: `synctest.Test`, `synctest.Wait`; `synctest.Sleep` in 1.27); an injected `Clock` stays the rule for authoritative code. `testing/cryptotest.SetGlobalRandom` (1.26) fixes crypto randomness in tests.
- Full procedure: `skills/consistency`.

## 14. Maps and slices

- Maps suit sparse or random lookup. Do not use them by habit in hot loops. When iteration order matters, sort the keys (`slices.Sorted(maps.Keys(m))`, 1.23), or use an ordered structure. Writing to a nil map panics; maps are not safe for concurrent writes.
- Use the 1.21+ helpers instead of hand-written ones: `slices`, `maps`, `cmp`, `clear`, `min`/`max`, `sync.OnceValue`. `append` may alias the caller's backing array (use `s[:n:n]` or `slices.Clone`), and re-slicing a huge array retains it.
- Prefer slices for dense ordered data. Preallocate when the size is known: `items := make([]Item, 0, n)`. Reuse scratch slices in hot loops.
- When exposing a read-only view, do not leak the mutable backing array: clone, hand over immutable snapshots, or copy on publish.

## 15. Struct layout, pointer vs value

- Hot structs are compact. Mind alignment and padding; inspect sizes before micro-packing. Prefer IDs or indices over pointer graphs in hot data.
- Values for small, immutable, value-like types (IDs, money, coordinates, timestamps). Pointers when mutation identity matters, the struct is large or copying is undesirable. Avoid pointers to tiny scalar-like structs.

## 16. References between entities

Reference other aggregates by stable ID, not by pointer.

```go
type Shipment struct{ Order OrderID } // good
type Shipment struct{ Order *Order }  // avoid
```

IDs simplify deletion, persistence, replay, hashing and cross-service messages.

## 17. Generics, reflection, `any`

- Generics only where they clearly remove duplication (heap, small collection primitives). No generic "architecture frameworks". Generic type aliases (1.24), self-referential type parameters (1.26) and generic methods (1.27; interface methods still cannot be generic) exist; accept them only where they delete real duplication.
- Iterators: `iter.Seq`/`iter.Seq2` with `for range seq` is the idiomatic lazy sequence (1.23); pull iterators need `iter.Pull` and `defer stop()`. Keep indexed loops in measured hot paths (§39).
- No reflection in hot paths; acceptable in serialisation or tooling adapters when justified.
- Avoid `any` and `map[string]any` in domain and application APIs. Prefer typed commands, events and definitions.

## 18. Enums and IDs

- Typed integer enums with a meaningful zero value; `String()` for diagnostics only. No strings in hot state machines.
- Distinct ID types where mixing them is a real bug risk: `type OrderID string`, `type UserID uint64`. Do not wrap every integer if conversion noise outweighs the safety.

## 19. Constants

Typed constants for meaningful values. No magic numbers in business logic. Tunable values (limits, rates, timeouts) come from validated config (§28).

## 20. Time

- Instants and durations come from an injected `Clock` in authoritative code; `time.Now` is allowed only in `cmd`, adapters and `platform`.
- Never mix logical time (versions, sequence numbers, ticks) with wall-clock `time.Duration`; name parameters clearly (`elapsedSeq`, `retryTimeout`).
- Store UTC; convert at the edge.

## 21. Randomness

- No global RNG in authoritative code. Inject a source (`*rand.Rand` from `math/rand/v2`: `rand.New(rand.NewPCG(s1, s2))`, an ID generator, a token generator). Since 1.26 the `crypto/*` APIs ignore a `rand` parameter and use the system source.
- Make the identity of a random draw explicit and stable so unrelated code changes do not shift it.
- Security tokens use `crypto/rand` (`rand.Text()` since 1.24) in an adapter, behind a port. For identifiers use the standard `uuid` package on 1.27+ (`uuid.New`, `uuid.NewV7`, `uuid.Parse`) rather than a third-party UUID library.

## 22. Allocation, pools, defer

- Hot code strives for zero avoidable allocations. Watch for `fmt.Sprintf`, closures, temporary maps, repeated `append` growth, interface boxing, `[]byte(string)` conversions and reflection. Do not sacrifice readability without evidence.
- `sync.Pool` only for temporary reusable objects with demonstrated allocation pressure; benchmark first; never for long-lived ownership.
- Use `defer` normally for unlock, close and cleanup. In ultra-hot tiny functions benchmark before avoiding it.

## 23. Strings

Do not use human-readable string IDs in hot loops when compact indexes exist. Load-time mapping (`"plan.pro" -> PlanIndex(3)`) lets debug output map back to the string.

## 24. Logging

- Structured logging with `log/slog` (JSON handler in production). Prefer typed attributes and the `*Context` methods so handlers can add trace IDs; lint with `sloglint` (`attr-only`):

```go
logger.LogAttrs(ctx, slog.LevelWarn, "charge declined",
    slog.String("order", orderID), slog.String("reason", reason))
```

- `slog.NewMultiHandler` (1.26) fans out to several handlers; `slog.DiscardHandler` silences logs in tests. A handler wrapper (or the `otelslog` bridge) injects `trace_id` and `span_id` from `ctx`.

- Every request-scoped log line carries a correlation ID. Never log secrets, tokens or personal data.
- No per-item logging in hot paths in production; trace and debug are selectively enabled.
- Logging is observational: it never influences ordering, randomness or state, and logs are never parsed back into behaviour.
- Metrics are observational too: no behaviour switches on CPU load or latency measurements of the same code path (circuit breakers belong in an adapter and are explicit).

## 25. Panics and assertions

- Panic only for impossible programmer invariants or development-only corruption detection. Never for bad input or business outcomes. Servers recover at the edge (handler middleware), log, and return an error response.
- Go has no `assert`. Add targeted invariant checks in dev and test builds with informative messages; no generic `assert()` helpers scattered around.

## 26. Comments and doc comments

- Comments explain why, invariants, complexity, ordering and subtle edge cases. Not `// increment i`.
- Good: `// Tie-break by ID so worker scheduling cannot change the order of refunds.`
- Exported identifiers start their comment with the name (`go.dev/doc/comment`); mark retired API with a `// Deprecated:` paragraph; keep a runnable `Example` function for each public use case. Keep comments short. Architecture belongs in docs, not 100-line function comments.

## 27. Functions, parameters, options

- Prefer focused functions. A clear 40-line state transition beats eight one-line helpers. Do not split a function only to hit a line count.
- Many related parameters become a request struct (`ChargeRequest{Order, Amount, Key}`); not for trivial two-argument functions.
- Functional options sparingly: they can hide required invariants. Prefer explicit config structs for complex constructors.

## 28. Configuration and content loading

Config and content (feature flags, price tables, rule sets, templates, fixtures) are loaded at the edge and become validated, typed, immutable values before business code sees them.

- Load config in `cmd`/`platform`; never read environment variables, flags or files inside `domain` or `application`. Parse env into a typed struct once (`caarlos0/env`, `go-envconfig` or `koanf`).
- Config structs are explicit and validated at startup: fail fast with all errors listed, not the first.
- Pipeline for file-based definitions:

```text
discover -> deterministic order -> parse -> validate schema -> validate references
-> freeze -> assign compact runtime indexes
```

- Enumeration order of the filesystem never affects results; sort.
- Reject: duplicate IDs, missing references, dependency cycles, out-of-range values, unsafe paths, unsupported versions, ambiguous overrides.
- Untrusted input (uploaded files, webhooks, queue messages, persisted blobs from older versions) is bounded: max size, count, nesting depth, string length, decompression ratio, schema version (§38).
- Convert untyped maps to typed structs at load; never let `map[string]any` flow through business code.
- Namespaced stable IDs for definitions (`billing.plan_pro`); free-form strings are not hot-path keys.
- A validation command that loads and checks config without starting the service is part of the feature (`cmd/<svc>` with `-check`, or a test).
- Tests cover: deterministic ordering, duplicate IDs, missing reference, invalid path, override precedence, round-trip fixtures.

## 29. File I/O, paths, serialisation

- `domain` never touches the filesystem; adapters own files and directories.
- Open files whose names come from users or content through `os.Root` (`os.OpenRoot(dir)` then `root.Open(name)`, or `os.OpenInRoot`; 1.24): it cannot escape the directory, symlinks included. Since 1.25 `Root` also has `Symlink`, `Rename`, `Link`, `Readlink`, `Chmod`, `MkdirAll`, `RemoveAll`, `ReadFile` and `WriteFile`. Use `filepath.IsLocal` for pure validation. `filepath.Join` plus prefix checks is the old, bypassable way.
- Persisted formats are versioned; DTOs are separate from domain types so a renamed field does not silently drop data. Write files atomically: temp file, fsync where policy requires, rename. Schema evolution: §45.

## 30. Testing

Detail in `skills/testing`.

- Tests are deterministic, name behaviour, avoid sleeps and real time (inject the clock or use `synctest`, §13), use table tests when cases share structure, use fixtures for multi-component behaviour.
- Use `t.Helper()`, `t.Cleanup`, `t.TempDir()`, `t.Setenv` and `t.Context()`; per-iteration loop variables make `tc := tc` unnecessary. `T.ArtifactDir` (1.26, with `-artifacts`) stores test outputs. Prefer hand-written fakes for consumer-owned ports; generate mocks only when the call sequence is the point.
- Run with `-count=1 -shuffle=on` (print the seed); tests never depend on order. `t.Parallel()` only without shared mutable state.
- Integration tests use real dependencies (`httptest`, `testcontainers-go`), isolated per test, behind `//go:build integration` or `-short`, and run in CI with `-tags=integration` (give `go vet` the same tag).
- Names describe behaviour: `TestRefund_RejectsAlreadyRefundedOrder`, `TestRetry_StopsAfterMaxAttempts`.
- Table tests are for same behaviour over many inputs, not for rows that need unrelated setup.
- Subtests group cases and sharpen failures; parallel subtests never share mutable state.
- Fuzz where it pays: parsers, decoders, migrations, validators, quantity and money arithmetic, DTO round trips.
- A bug fix starts with a failing test.
- Hard rule: new code ships with unit tests, new-code coverage is at least 70% (`loops gate` coverage, WORKFLOW §6), and every `go test` runs with `-race`. Reading the coverage table: `skills/testing`.
- The coverage recipe passes no `-coverpkg`, so a package is credited only by its own tests. Add `-coverpkg=./...` (consistently between runs) to credit cross-package tests, at the cost of noisier per-package numbers. Binaries: `go build -cover`, run with `GOCOVERDIR`, convert with `go tool covdata textfmt` (unrecovered panics lose the data).

Other languages: the universal test and coverage policy is `specs/TESTING.md`; each language's commands and race policy live in its pack (`specs/languages/<id>/pack.json`, `loops lang show <id>`).

## 31. Benchmarks and profiling

- Hot paths have benchmarks with `b.ReportAllocs()`; do not optimise without a baseline.
- Write benchmarks as `for b.Loop() { ... }` (1.24): it keeps results alive, excludes setup before and cleanup after the loop, and since 1.26 no longer blocks inlining. No manual `ResetTimer`. Compare with `benchstat` after `-count=10`.
- Use `pprof`, execution traces, heap profiles and `benchstat`. Fix algorithmic cost before instruction-level tweaks.
- For unclear allocations: `go build -gcflags='-m=2'`, then confirm with a benchmark.
- Hot services: commit a representative CPU profile as `default.pgo` in the main package directory (PGO; `-pgo=auto` is the default). Set `GOMEMLIMIT` below the container limit, with `GOGC` as the knob. The Green Tea GC is the default in 1.26; 1.27 makes allocations under 80 bytes up to 30% cheaper, about 1% overall. The 1.25 flight recorder keeps a rolling trace window for incident analysis.

## 32. Complexity documentation

Hot or scaling code states its expected complexity in a comment or the package `AGENTS.md`.

```text
Pending-order lookup by customer: O(orders of that customer)
Batch settlement: O(n log n) in batch size
```

Any expected O(N^2) needs justification.

## 33. Data-oriented bias

For hot, large-N code prefer dense slices, stable IDs, compact records, indexes and bit flags over deep pointer graphs, inheritance-like interface towers and per-item goroutines. Go composition works well with explicit stores.

## 34. Iteration ordering

Never assume the storage order of a map, set or result set is meaningful unless the contract says so. Order-sensitive resolution uses explicit keys: priority plus ID, sequence number, `ORDER BY` with a unique tie-break column.

## 35. Avoid hidden work

Getter-named functions must not scan globally, allocate heavily, mutate or do I/O. Name expensive operations honestly: `FindOverdueInvoices`, `RebuildIndex`, `ComputeQuote`.

## 36. Caches and revisions

- A cache documents its source, its invalidation trigger, its rebuild point and its revision. No "cache until it looks stale".
- Revision counters (config version, index version, schema version) make cache validation and stale-result detection simple, including for worker results that must be rejected if the revision moved.

## 37. Build tags, unsafe, cgo, generated code

- Build tags sparingly: dev diagnostics, platform adapters, `integration`. Never fork business logic by tag.
- `-race` requires cgo and a C compiler: never set `CGO_ENABLED=0` for tests. Keep cgo out of the production image (`CGO_ENABLED=0`, static binary) but present in the test image (`golang:<ver>` has gcc; alpine needs `gcc musl-dev`).
- `unsafe` is forbidden by default. Introducing it needs an ADR, benchmark proof, correctness tests and a portability analysis.
- Avoid cgo in `domain` and `application`.
- Never hand-edit generated files; do not feed generated code to agents unless debugging the generator. Read the schema source (`.proto`, OpenAPI, SQL) instead.

## 38. Security

Treat external input as untrusted: requests, webhooks, messages, uploaded files, persisted blobs from other versions.

- Check lengths, counts, nesting, path safety, decompression limits, schema version and reference cycles.
- Authenticate and authorise at the inbound adapter; pass an authenticated principal inward as data.
- Parameterised queries only. No string-built SQL. Commands via `exec.CommandContext` with no shell; HTML via `html/template`.
- Never execute arbitrary code from input. Never log secrets.
- Cap request bodies (`http.MaxBytesReader`, `io.LimitReader`). Servers set `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`; never a bare `http.ListenAndServe` (gosec G112). Outbound calls: allow-list URLs and block private ranges in a `net.Dialer.Control` hook (SSRF).
- Paths: `os.Root` (§29). Crypto: `crypto/rand`, argon2id or bcrypt, `subtle.ConstantTimeCompare`, no MD5, SHA-1, DES or RC4; the FIPS 140-3 module where compliance asks. OWASP Go-SCP is the checklist.
- Supply chain: `go mod verify`, the checksum database (`GOPRIVATE` only for private modules), GitHub Actions pinned by SHA.
- Run `govulncheck` and `gosec` in CI as required steps, not "when installed" (§2).

## 39. Performance-friendly API design

- `func (s *Store) Get(id ID) (Record, bool)` style, not APIs that build large temporary slices on every call. Caller-provided scratch buffers only where a benchmark justifies it.
- Range-over-func iterators (`iter.Seq`) are the idiomatic iterator and the compiler often inlines them; use them freely outside hot loops. In a hot loop keep the benchmark requirement: compare against a plain indexed loop.

## 40. Memory retention

Beware of slicing huge backing arrays, caches with no eviction, maps retaining deleted keys and pooled objects keeping references. Long-running services need a heap-stability check (a soak test or a profile over time). Pre-size maps (`make(map[K]V, n)`) in initialisation-heavy code. Set `GOMEMLIMIT` in containers (§31).

## 41. API ownership and service naming

- Do not put interfaces in `pkg/common`; the consumer package defines what it needs. This keeps ports narrow.
- Avoid meaningless suffixes (`Manager`, `Helper`, `Processor`, `Handler`, `Service`) unless they match the concept. Prefer domain nouns: `Scheduler`, `Ledger`, `Registry`, `Store`, `Dispatcher`.

## 42. Domain language

One word per concept in code, docs and APIs (`Order`, not `Order`/`Purchase`/`Basket` for the same thing). Canonical names live in the architecture docs or the package `AGENTS.md`.

## 43. State transitions

Centralise important transitions: `Pending -> Authorised -> Captured -> Refunded` is changed by one transition function that checks the source state, not by five packages assigning a field. Use explicit transition functions or events (`skills/jobs-and-state-machines`).

## 44. Validation at boundaries, commands

- Validate external input once at the boundary; inside the domain rely on established invariants. Avoid repeated defensive validation in hot functions.
- Command handling: parse adapter input, build a typed command, validate application and domain constraints, apply atomically, return a typed result. Transport types (HTTP request, gRPC message, SQL row) never enter the domain.

## 45. Schema migration and compatibility

- Migrations are version-explicit; never infer a migration from "field missing" when a schema version exists. Test with historical fixtures.
- Prefer expand-then-contract: add, backfill, switch readers, remove.
- A public API, persisted schema, event payload or config key change is a compatibility decision: update all call sites in the same change, update docs, state it in the task packet. Before 1.0, internal APIs may change quickly; external contracts still need a deliberate policy.

## 46. Idempotency and stable hashing

- Operations that may be retried (webhooks, queue redelivery, client retries) carry an idempotency key and are safe to apply twice.
- Hashes and checksums over state use stable ordering, fixed numeric encoding and never pointer addresses or map iteration order.

## 47. Observability without side effects

Errors are explicit; logs, metrics and traces exist where operators need them; none of them alters behaviour (§24). Expose machine-readable diagnostics (health, version, schema version) in an inbound adapter rather than making agents infer state from many files.

## 48. Dependencies: standard library, then a library, then our own code

Take the first that fits:

1. The standard library has it ready to call. Check the newest release: `log/slog`, `ServeMux` method and wildcard routing (1.22), `slices`/`maps`/`iter`, `os.Root`, `errors.AsType`, `encoding/json/v2` (on by default in 1.27, with `encoding/json` backed by it; opt out with `GOEXPERIMENT=nojsonv2`) and `uuid` (1.27, import path `"uuid"`) all exist.
2. A well-known, maintained library has it.
3. We write it, with the reason in the package `AGENTS.md`: a hot path a library cannot serve without allocating, output that must stay reproducible, or a library that costs more than the code it replaces.

Assembling a standard format from standard-library parts does not count as step 1 (semver from a regexp; a UUID from `crypto/rand` before 1.27, when `uuid` joined the standard library). Do not add a dependency for a few obvious lines either.

Where libraries may go:

- `cmd`, `internal/adapters`, `internal/platform`: allowed.
- `internal/domain`, `internal/application`: standard library only. Avoid `pkg/` by default (`internal/` is compiler-enforced and the official layout has no `pkg/`); where a project has one, it stays standard-library only.
- A library's types stay inside its adapter and never cross a port.

Every new dependency states in the task result: capability; maintenance (not archived, recent releases); license; what it pulls in (`go mod graph`); security implications.

## 49. Dependency updates

Update deliberately: run tests (with `-race`), benchmarks for hot dependencies, `go mod tidy -diff`, `go mod verify` and `govulncheck`. Use Dependabot (`gomod`) or Renovate, grouped by minor and patch; never auto-merge a major upgrade. Multi-module repos upgrade per module. Only the architect role (and `go-engineer`, in wave 0 and language lanes) touches `go.mod`, `go.sum`, the `go`, `toolchain` and `tool` lines, and `go.work` (`ROLES.md`).

## 50. Custom lint rules

Project-specific checks should catch: `time.Now` in `domain`; global `math/rand` in authoritative packages; forbidden imports; `utils` packages; edits to generated code; `init()` registration; runtime-assigned package vars. `loops arch` implements as errors the import edges, third-party imports in core, `time.Now`, global `math/rand` and `go` statements in domain (`clock`, `rand`, `go`), `init()`, package-level mutable state and `cmd` wiring, and the size limits as warnings (`specs/SOLID.md §8`). Unchecked: `utils` packages, edits to generated code and `database/sql` in domain (review items). In golangci-lint v2 add `depguard` (deny lists per package glob), `forbidigo` and `sloglint`; `go/analysis` analyzers and `go fix` modernizers cover other project-specific rules.

## 51. Review checklist

```text
Architecture boundary
Module hygiene (tidy -diff, tool and toolchain pins)
Consistency (ordering, clock, RNG, idempotency)
State ownership
Allocation impact
Complexity
Error semantics
Concurrency / race risk
Persisted-schema and API compatibility
Config / content impact
Test quality (shuffle, race)
Security (SAST, vuln scan, timeouts)
Observability (trace propagation)
Benchmark evidence
Naming and API clarity
```

The ordered review procedure is in `skills/review-integration`.

## 52. AI contributor checklist

Before coding: search for an existing type or primitive; read the package `AGENTS.md`; read the tests; identify whether the path is hot; identify the consistency constraints.

Before finishing: `gofmt`; `go mod tidy -diff`; targeted tests with `-race`; a benchmark if a hot path changed; the claims report with evidence for every claim (`templates/CLAIMS_REPORT.md`).

## 53. Performance escalation path

Optimise in this order, and stop when the target is met:

```text
1. eliminate O(N^2) and global scans
2. reduce update or polling frequency
3. add indexes
4. add dirty invalidation
5. skip inactive state
6. batch operations
7. reduce allocations
8. improve data layout
9. cache
10. parallelise pure work
11. micro-optimise only with evidence
```

Procedure: `skills/performance`.

## 54. Final directive

Prefer boring, explicit, measurable Go. The code should make obvious who owns state, when state changes, why ordering is deterministic, what an operation costs, how it is tested and where an external technology plugs in. If a clever abstraction makes those answers harder, remove it.

## Appendix: rule ID index

`research/languages/go.md` rule IDs, mapped to the sections above. Behaviour not covered by a section (repo, data, API, ops) is in the `lang-go` skill references (`skills/lang-go/`).

| Research IDs | Section |
|---|---|
| `GO-REPO-01..03`, `GO-REPO-05` | §41, §48 (layout, `pkg/`); `skills/lang-go/repo.md` |
| `GO-REPO-04` | `skills/lang-go/repo.md` (per-module checks) |
| `GO-REPO-06..08`, `GO-REPO-11`, `GO-REPO-12` | §2, §49 |
| `GO-REPO-09`, `GO-REPO-10`, `GO-REPO-13..15` | §49; `skills/lang-go/repo.md` |
| `GO-TOOL-01..09` | §2, §49, §50 |
| `GO-ARCH-01..12` | §5, §6, §16, §41, §50; `specs/languages/go/ARCHITECTURE.md` |
| `GO-IDIOM-01..03`, `GO-IDIOM-19` | §8, §9, §10, §25 |
| `GO-IDIOM-04..08`, `GO-IDIOM-16..17` | §7, §14, §15, §17, §18 |
| `GO-IDIOM-09..15`, `GO-IDIOM-18` | §12, §22, §26, §37, §41 |
| `GO-CONC-01..10` | §10-§13 |
| `GO-TEST-01..15` | §30 |
| `GO-PERF-01..07` | §22, §31, §39, §40 |
| `GO-SEC-01..11` | §29, §38 |
| `GO-DATA-01..09` | §36, §45; `skills/lang-go/data-api.md` |
| `GO-API-01..10` | §44; `skills/lang-go/data-api.md` |
| `GO-OBS-01..07` | §24, §47; `skills/lang-go/data-api.md` |
| `GO-OPS-01..11` | §28, §37; `skills/lang-go/data-api.md` |
| `GO-DOC-01..05` | §26, §51, §52 |
| `GO-FW-01..06` | §48; `skills/lang-go/frameworks.md` |
