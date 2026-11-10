# Go Best Practices

Mandatory Go coding standard for every human and AI contributor to a Go backend: idiomatic, explicit, measurable, consistent. Cite by section (`specs/languages/go/RULES.md §8`). Extracted from rimworld-go and generalised for backend services.

## 1. Guiding principle

Write Go that is simple, explicit, idiomatic, benchmarkable, consistent where state is authoritative, allocation-conscious in hot paths, easy to test and easy for the next engineer or agent to read. Avoid "enterprise Go" ceremony.

## 2. Formatting and static analysis

Every change passes:

```bash
gofmt -l .          # prints nothing
go vet ./...
staticcheck ./...   # when installed
go test -race ./...
```

- Tests always run with the race detector, locally and in CI.
- CI enforces formatting; reviewers do not.
- Use `goimports` if the project standardises on it.
- `loops gate` runs these in order (`specs/WORKFLOW.md §6`).

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

Use them only when callers must branch programmatically. No large hierarchies. Prefer `errors.Is` and `errors.As`.

## 10. Context

- `context.Context` is for I/O, cancellation, deadlines and long-running operations. It is the first parameter: `func Load(ctx context.Context, ...)`.
- Never store a context in a struct.
- Do not thread it through pure domain functions.
- Every outbound call has a deadline derived from the incoming context.

## 11. Concurrency

Goroutines are not free architecture.

- One owner per piece of mutable state; others get messages or immutable snapshots.
- No goroutine per request item, per order or per job without a bound; use a bounded worker pool.
- Workers compute on immutable inputs and return results; the owner commits them in deterministic order (`skills/consistency`).
- Every goroutine has an owner that waits for it or cancels it; no goroutine outlives its context.
- Use channels where ownership boundaries benefit; do not use them as a replacement for function calls.
- `go` statements appear only in a small number of places (a pool, a server, a consumer loop) and never in `domain`.

## 12. Mutexes and atomics

- Prefer ownership over shared mutation. If a mutex is needed: keep protected state small, document lock order when there are several locks, never hold a lock across I/O, never expose internal slices or maps while a lock protects them.
- `sync/atomic` belongs at clear concurrency boundaries (snapshot pointer swap, metrics), not sprinkled through domain code.
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
- Full procedure: `skills/consistency`.

## 14. Maps and slices

- Maps suit sparse or random lookup. Do not use them by habit in hot loops. When iteration order matters, sort the keys, or use an ordered structure.
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

- Generics only where they clearly remove duplication (heap, small collection primitives). No generic "architecture frameworks".
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

- No global RNG in authoritative code. Inject a source (`rand.Rand`, ID generator, token generator).
- Make the identity of a random draw explicit and stable so unrelated code changes do not shift it.
- Security tokens use `crypto/rand` in an adapter, behind a port.

## 22. Allocation, pools, defer

- Hot code strives for zero avoidable allocations. Watch for `fmt.Sprintf`, closures, temporary maps, repeated `append` growth, interface boxing, `[]byte(string)` conversions and reflection. Do not sacrifice readability without evidence.
- `sync.Pool` only for temporary reusable objects with demonstrated allocation pressure; benchmark first; never for long-lived ownership.
- Use `defer` normally for unlock, close and cleanup. In ultra-hot tiny functions benchmark before avoiding it.

## 23. Strings

Do not use human-readable string IDs in hot loops when compact indexes exist. Load-time mapping (`"plan.pro" -> PlanIndex(3)`) lets debug output map back to the string.

## 24. Logging

- Structured logging (`log/slog`):

```go
logger.Warn("charge declined", "order", orderID, "user", userID, "reason", reason, "request_id", reqID)
```

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
- Exported identifiers start their comment with the name. Keep comments short. Architecture belongs in docs, not 100-line function comments.

## 27. Functions, parameters, options

- Prefer focused functions. A clear 40-line state transition beats eight one-line helpers. Do not split a function only to hit a line count.
- Many related parameters become a request struct (`ChargeRequest{Order, Amount, Key}`); not for trivial two-argument functions.
- Functional options sparingly: they can hide required invariants. Prefer explicit config structs for complex constructors.

## 28. Configuration and content loading

Config and content (feature flags, price tables, rule sets, templates, fixtures) are loaded at the edge and become validated, typed, immutable values before business code sees them.

- Load config in `cmd`/`platform`; never read environment variables, flags or files inside `domain` or `application`.
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
- Use `filepath`, normalise and validate user-supplied paths, reject traversal.
- Persisted formats are versioned; DTOs are separate from domain types so a renamed field does not silently drop data. Write files atomically: temp file, fsync where policy requires, rename. Schema evolution: §45.

## 30. Testing

Detail in `skills/testing`.

- Tests are deterministic, name behaviour, avoid sleeps and real time (inject the clock), use table tests when cases share structure, use fixtures for multi-component behaviour.
- Names describe behaviour: `TestRefund_RejectsAlreadyRefundedOrder`, `TestRetry_StopsAfterMaxAttempts`.
- Table tests are for same behaviour over many inputs, not for rows that need unrelated setup.
- Subtests group cases and sharpen failures; parallel subtests never share mutable state.
- Fuzz where it pays: parsers, decoders, migrations, validators, quantity and money arithmetic, DTO round trips.
- A bug fix starts with a failing test.
- Hard rule: new code ships with unit tests, new-code coverage is at least 70% (`loops gate` coverage, WORKFLOW §6), and every `go test` runs with `-race`. Reading the coverage table: `skills/testing`.

Other languages (conventions the agents follow; `loops gate` automates Go only):

| Language | Tests and coverage (min 70% of new code) | Race / concurrency |
|---|---|---|
| Rust | `cargo test`; `cargo-llvm-cov` | `loom`, or `RUSTFLAGS=-Zsanitizer=thread` on nightly |
| Python | `pytest` with `pytest-cov --cov-fail-under=70` | `pytest-xdist`, `pytest-repeat` to shake out races |
| TypeScript/JS | `vitest` or `jest --coverage` with thresholds of 70 | repeat runs, async leak detection |
| Java/Kotlin | JaCoCo minimum 0.70 | `jcstress` |
| C/C++ | `gcov` or `llvm-cov` | `-fsanitize=thread` |

## 31. Benchmarks and profiling

- Hot paths have benchmarks with `b.ReportAllocs()`; do not optimise without a baseline.
- Keep setup out of timed work (`b.ResetTimer()`); keep results alive so the compiler cannot eliminate the work.
- Use `pprof`, execution traces, heap profiles and `benchstat`. Fix algorithmic cost before instruction-level tweaks.
- For unclear allocations: `go build -gcflags='-m=2'`, then confirm with a benchmark.

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

- Build tags sparingly: dev diagnostics, platform adapters. Never fork business logic by tag.
- `unsafe` is forbidden by default. Introducing it needs an ADR, benchmark proof, correctness tests and a portability analysis.
- Avoid cgo in `domain` and `application`.
- Never hand-edit generated files; do not feed generated code to agents unless debugging the generator. Read the schema source (`.proto`, OpenAPI, SQL) instead.

## 38. Security

Treat external input as untrusted: requests, webhooks, messages, uploaded files, persisted blobs from other versions.

- Check lengths, counts, nesting, path safety, decompression limits, schema version and reference cycles.
- Authenticate and authorise at the inbound adapter; pass an authenticated principal inward as data.
- Parameterised queries only. No string-built SQL.
- Never execute arbitrary code from input. Never log secrets.
- Run `govulncheck` in CI.

## 39. Performance-friendly API design

- `func (s *Store) Get(id ID) (Record, bool)` style, not APIs that build large temporary slices on every call. Caller-provided scratch buffers only where a benchmark justifies it.
- Be cautious with closure-based iterators in hot loops (escape and allocation); benchmark against a plain indexed loop.

## 40. Memory retention

Beware of slicing huge backing arrays, caches with no eviction, maps retaining deleted keys and pooled objects keeping references. Long-running services need a heap-stability check (a soak test or a profile over time). Pre-size maps (`make(map[K]V, n)`) in initialisation-heavy code.

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

1. The standard library has it ready to call.
2. A well-known, maintained library has it.
3. We write it, with the reason in the package `AGENTS.md`: a hot path a library cannot serve without allocating, output that must stay reproducible, or a library that costs more than the code it replaces.

Assembling a standard format from standard-library parts does not count as step 1 (a UUID from `crypto/rand`, semver from a regexp). Do not add a dependency for a few obvious lines either.

Where libraries may go:

- `cmd`, `internal/adapters`, `internal/platform`: allowed.
- `internal/domain`, `internal/application`, `pkg`: standard library only.
- A library's types stay inside its adapter and never cross a port.

Every new dependency states in the task result: capability; maintenance (not archived, recent releases); license; what it pulls in (`go mod graph`); security implications.

## 49. Dependency updates

Update deliberately: run tests (with `-race`), benchmarks for hot dependencies and `govulncheck`. Never auto-merge a major upgrade blindly. Only the architect role touches `go.mod` and `go.sum` (`ROLES.md`).

## 50. Custom lint rules

Project-specific checks should catch: `time.Now` in `domain`; global `math/rand` in authoritative packages; forbidden imports; `utils` packages; edits to generated code; `init()` registration; runtime-assigned package vars. `loops arch` implements as errors the import edges, third-party imports in core, `time.Now`, global `math/rand` and `go` statements in domain (`clock`, `rand`, `go`), `init()`, package-level mutable state and `cmd` wiring, and the size limits as warnings (`specs/SOLID.md §8`). Unchecked: `utils` packages and edits to generated code (review items).

## 51. Review checklist

```text
Architecture boundary
Consistency (ordering, clock, RNG, idempotency)
State ownership
Allocation impact
Complexity
Error semantics
Concurrency / race risk
Persisted-schema and API compatibility
Config / content impact
Test quality
Benchmark evidence
Naming and API clarity
```

The ordered review procedure is in `skills/review-integration`.

## 52. AI contributor checklist

Before coding: search for an existing type or primitive; read the package `AGENTS.md`; read the tests; identify whether the path is hot; identify the consistency constraints.

Before finishing: `gofmt`; targeted tests with `-race`; a benchmark if a hot path changed; the claims report with evidence for every claim (`templates/CLAIMS_REPORT.md`).

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
