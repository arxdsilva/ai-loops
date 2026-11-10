# Hexagonal Architecture

Authoritative layering, ports and repository model for a Go backend, so that business rules are testable without infrastructure and agents can work in parallel without collisions. Extracted from rimworld-go and generalised. `loops arch` checks the mechanical parts (§25).

## 1. Purpose

- Business logic is independent of transport, storage, queues and vendors.
- Everything is testable headlessly: no network, database or filesystem in domain or application tests.
- Modules are small and owned, so several agents can change different layers at once.
- The architecture is explicit rather than framework-heavy; it preserves direction of dependencies and testability, not ceremony.

## 2. Model

```text
 Inbound adapters            HTTP / gRPC / CLI / queue consumers / schedulers
        |  call
        v
 Application layer           use cases, orchestration, transactions, consumer-owned ports
        |  call
        v
 Domain core                 entities, value objects, rules, domain services (pure)
        ^
        |  implemented by
 Outbound adapters           SQL repositories, caches, queue producers, external API clients
```

The consumer owns the abstraction it needs; adapters implement it.

## 3. Dependency rule

Dependencies point inward: `adapters -> application -> domain`.

Domain may use only the standard library and pure utility packages in `pkg/`.

Forbidden in `domain` (and mostly in `application`): HTTP, gRPC, SQL drivers, ORMs, queue clients, `os`, filesystem, CLI frameworks, config loaders, loggers that change behaviour, vendor SDKs, generated transport or persistence types.

Boundary test for every import: does the inner model need to know this technology exists? If not, keep it outside.

## 4. Repository structure

```text
repo/
├── cmd/<svc>/main.go          wiring only (§23)
├── internal/
│   ├── domain/<concept>/      entities, rules, domain services; stdlib + pkg only
│   ├── application/<usecase>/ use cases, command handlers, queries; consumer-owned ports
│   ├── ports/{inbound,outbound}/  shared port interfaces, when several packages use one
│   ├── adapters/
│   │   ├── inbound/           http, grpc, cli, consumer, scheduler
│   │   └── outbound/          postgres, redis, kafka, <vendor> clients
│   ├── platform/              config, logging, metrics, tracing, clock, pools
│   └── compose/<svc>/         composition root helpers; only cmd/ imports them
├── pkg/                       low-level primitives with no business concepts
├── migrations/                versioned schema migrations
└── AGENTS.md  README.md  Makefile
```

Package detail lives in each package's `AGENTS.md` (`templates/PACKAGE_AGENTS.md`). Create a package when its first real behaviour needs it, not to match a diagram.

## 5. What each layer owns

| Layer | Owns | Must not |
|---|---|---|
| Domain | Business rules, invariants, entities, value objects, state transitions, domain services. Runnable headlessly with fakes for nothing. | Know about I/O, time sources, randomness sources, config |
| Application | Orchestration: load, call domain, persist, publish; transactions; idempotency; authorisation checks that need data; read models | Contain business rules; import adapters |
| Ports | Narrow interfaces at boundaries: repositories, clocks, publishers, clients | Mirror a concrete type |
| Adapters | Translating a technology into a port or a port into a request: handlers, repositories, clients, consumers | Hold business rules; leak library types inward |
| Platform | Cross-cutting infrastructure the process needs: config loading, logging setup, metrics, clock implementation | Be imported by domain |

## 6. Ports are consumer-owned

Mandatory: code that depends on an adapter or an external technology depends on a narrow interface describing the behaviour it needs, never on the concrete type. The concrete adapter is constructed once at the composition root and passed around as the interface it satisfies. This is dependency inversion applied at the boundary: the interface is the abstraction both sides depend on, so real, fake and replacement implementations swap without touching consumers.

- The same applies inward: an inbound adapter does not hold a concrete `*application.Checkout` just because that is what exists. It defines a small interface listing only the methods it calls; the concrete type satisfies it implicitly.
- Keep interfaces small and single-purpose (interface segregation, `specs/SOLID.md §6`). Never add a method to an existing port "just in case".
- Create an interface wherever a package boundary is crossed (adapter and application, application and domain-facing port), even with one implementation today. Do not create one merely because a concrete type exists.

```go
package payment // the consumer

type Charger interface {
    Charge(ctx context.Context, req ChargeRequest) (ChargeResult, error)
}
```

## 7. Inbound ports

Operations external actors can request. All inbound adapters (HTTP, gRPC, CLI, queue consumers, jobs) use the same application surface where practical.

```go
type Checkout interface {
    Place(ctx context.Context, cmd PlaceOrder) (OrderID, error)
    Get(ctx context.Context, id OrderID) (OrderView, error)
}
```

## 8. Outbound ports

```go
type OrderRepository interface {
    Save(ctx context.Context, o order.Order) error
    ByID(ctx context.Context, id order.ID) (order.Order, error)
}

type EventPublisher interface {
    Publish(ctx context.Context, e Event) error
}

type PaymentGateway interface {
    Charge(ctx context.Context, req ChargeRequest) (ChargeResult, error)
}
```

Adapters implement these. Library, driver and SQL row types never cross a port.

## 9. Domain services vs application services

- Domain service: a business rule spanning several domain objects: `PriceOrder`, `ApplyDiscount`, `AllocateStock`, `TransferFunds`.
- Application service: orchestration: `PlaceOrder`, `CancelSubscription`, `RetryFailedPayments`, `ImportCatalog`.

## 10. Commands

State changes enter as typed commands handled by the application layer.

```go
type PlaceOrder struct {
    CustomerID customer.ID
    Items      []Item
    Key        IdempotencyKey
}
```

- A command is a plain, serialisable value: no HTTP request, no SQL row.
- Handlers validate, load, call the domain, persist and publish; they return a typed result.
- Transport adapters map request to command and result to response; they never mutate domain state directly.
- Retried commands carry an idempotency key (`skills/consistency`).

## 11. Queries and read models

- Reads return explicit read models (DTOs), not domain internals and not storage rows.
- Query code may bypass the domain model for performance (a SQL projection) but never mutates.
- Do not hand mutable stores or entities to adapters.

```go
type OrderView struct {
    ID     order.ID
    Status string
    Total  Money
}
```

## 12. One owner per piece of mutable state

Every piece of mutable state has exactly one owner: an aggregate behind a repository, a goroutine, a lock-guarded struct, a table with a single writer service.

- Others communicate by commands or messages, or read immutable snapshots.
- Workers may compute (pricing, rendering, batch scoring, encoding, aggregation) but do not mutate authoritative state directly.
- Worker pattern for results that must be applied consistently:

```text
snapshot -> enqueue work with sequence ID -> workers compute -> collect
-> sort by sequence -> validate revision -> commit on the owner
```

- `go` statements appear only in a pool, a server loop or a consumer loop, not in `domain`.

## 13. Error taxonomy

| Kind | Examples | Represented as |
|---|---|---|
| Expected domain outcome | `InsufficientStock`, `CardDeclined`, `AlreadyCancelled` | Result values or typed domain errors |
| Validation | invalid command, malformed input, invalid config | Typed validation error, mapped to 4xx at the edge |
| Infrastructure | database down, timeout, corrupt payload | Wrapped `error`, mapped to 5xx or retried |

- The inbound adapter owns the mapping from error kind to transport status (HTTP code, gRPC status, exit code).
- Do not represent ordinary business outcomes as Go errors unless they are truly exceptional.
- Wrap with `%w`; callers branch with `errors.Is` and `errors.As`.

## 14. Context

Use `context.Context` at I/O and long-running boundaries: handlers, repositories, clients, jobs, migrations. First parameter, never stored. Do not thread it through pure domain functions. Deadlines and cancellation flow from the inbound adapter outward.

## 15. Package boundaries, domain types, `pkg/`

- A package owns one coherent concept: prefer `internal/domain/pricing` over a catch-all `internal/domain/shop`. Do not fragment into one package per struct. A type is split inside its package first; a new package is the last step (`specs/SOLID.md §2`).
- Canonical types have one home. Search before creating `UserID`, `Money`, `Clock` or an error type; never recreate one elsewhere.
- `pkg/` is reserved for reusable low-level primitives with no business concepts and no dependency on `internal/`: money arithmetic, retry/backoff, ID generation, bit sets, deterministic heaps. Add a primitive only when the standard library lacks it (`specs/languages/go/RULES.md §48`).

## 16. Domain events

Typed events decouple domain effects from reactions.

```go
type OrderPaid struct {
    Order order.ID
    At    time.Time
}
```

- Explicit and versioned; no `map[string]any` payloads.
- Published after the state change is committed (outbox pattern when delivery must be reliable).
- Consumers are idempotent: delivery is at least once.
- No hidden synchronous side effects; ordering guarantees are documented per stream.

## 17. State machines

Entities with lifecycles (orders, payments, subscriptions, jobs) use explicit state enums and one transition function that checks the source state. Prefer visible transitions over callback chains (`skills/jobs-and-state-machines`).

## 18. Persistence boundary

- Domain types are not the storage schema. Repositories translate between domain and row/document DTOs.
- Schema changes are versioned migrations in `migrations/`, tested with fixtures; rollbacks and backfills are planned, not improvised.
- Transactions are opened by the application layer through a port (unit of work), not by handlers or domain.
- Persisted and wire formats are versioned (`skills/consistency`).

## 19. Config boundary

Config and content are loaded in `cmd`/`platform`, validated, converted to typed immutable structs and passed in. Domain and application never read the environment or files (`specs/languages/go/RULES.md §28`).

## 20. Time boundary

Authoritative code takes a `Clock` port. Wall-clock time is used by logging timestamps, adapters, schedulers and profiling only. A test must be able to control time without sleeping.

## 21. Dependency injection

Explicit constructors; no DI frameworks, no service locators, no global registries.

```go
func NewCheckout(orders OrderRepository, pay PaymentGateway, clock Clock, bus EventPublisher) *Checkout
```

## 22. Initialization order

```text
config -> logging/metrics/tracing -> load and validate content
-> construct outbound adapters -> construct application services
-> construct inbound adapters -> run (serve / consume / schedule)
-> graceful shutdown in reverse order
```

Fail fast on invalid config or unreachable required dependencies. Shutdown honours a deadline and drains in-flight work.

## 23. `cmd` is wiring only

Every `cmd/<tool>` holds exactly one `main.go` declaring only `func main`. It parses arguments through a package, builds concrete adapters and application types, hands them over as interfaces, calls one `Run` and exits. No loops, switches, closures, helper functions, tests or long strings. `loops arch` reports violations as errors (`specs/SOLID.md §8a`).

| Logic | Home |
|---|---|
| Flags, usage text, a tool's `Run` | `internal/adapters/inbound/cli/<tool>` |
| Composition needing many adapters | `internal/compose/<svc>`, imported only by `cmd/` |
| Profiling, log file, signal handling | `internal/platform/<name>` |
| Clients a tool calls | `internal/adapters/outbound/<client>` |

## 24. Testing architecture

- Domain tests: unit tests, no fakes needed beyond injected clock/RNG.
- Application tests: fake outbound ports (in-memory repository, recording publisher, scripted gateway). Prefer real domain objects.
- Adapter tests: validate the technology translation separately (handler tests with `httptest`, repository tests against a real database in a container or a documented fixture, client contract tests against a fake server).
- Integration tests wire real adapters only where necessary; end-to-end checks hit the running service (`skills/testing`).
- Good fakes: in-memory store, deterministic clock, scripted gateway, recording publisher. Avoid mocking every call; avoid asserting call counts of private helpers.

## 25. Architecture lint

`loops arch` parses the module and reports findings. Layer directories default to the tree in §4 and can be overridden in `.loops/config.json` under `layers`: an object with keys `domain`, `application`, `ports`, `adapters`, `inbound`, `outbound`, `platform`, `compose`, `cmd`, `pkg`, each a list of module-relative prefixes; omitted keys keep the defaults.

| Rule | Severity | Checks |
|---|---|---|
| `imports` | error | Domain imports no application, ports, adapters, platform or cmd, and no `os/*` or `net/http/*`; domain, application and `pkg` import only the standard library (third-party is an error); application and ports import no adapters; outbound adapters do not import inbound; only `cmd/` imports `internal/compose`. A sub-package importing its parent is an error in domain and application, a warning elsewhere |
| `clock` | error | No `time.Now` in domain |
| `rand` | error | No global `math/rand` or `math/rand/v2` draw in domain (`New`, `NewSource`, `NewPCG`... are allowed) |
| `go` | error | No `go` statement in domain |
| `init` | error | No `func init()` registration (`specs/SOLID.md §4`) |
| `global` | error | No runtime-assigned package-level state (`specs/SOLID.md §9`) |
| `cmd` | error | `cmd/<tool>` is wiring only (§23) |
| `methods`, `fields`, `iface`, `lines`, `files` | warn | Size limits (`specs/SOLID.md §8`) |

`loops gate` fails only on errors. Unchecked mechanically: `utils` packages and edits to generated code; these are review items (`skills/review-integration`).

## 26. Architecture decision records

Write an ADR (`templates/ADR.md`, `docs/adr/NNNN-<slug>.md`) only for a decision that changes a contract: a port, the persisted or wire schema, the concurrency model, the command set, the consistency model. Everything else goes in the package `AGENTS.md` or the PR body. An ADR records context, decision, consequences and alternatives, not reasoning transcripts.

## 27. Architecture smells

Flag for review:

- interfaces with 20+ methods; a giant `AppContext` passed everywhere;
- adapters returning raw domain internals; domain importing transport or storage types;
- generated serialisation code in `domain`;
- `any`-heavy APIs; pervasive reflection;
- global registries; package cycles; `utils`;
- hidden goroutine mutation of shared state;
- an interface per struct;
- `Manager`/`Service` naming without clear ownership;
- handlers that contain business rules; repositories that contain business rules.

## 28. Migration policy

If the architecture must change:

1. write or update the ADR;
2. update the ports;
3. update all call sites in the same change where practical;
4. update the package `AGENTS.md`;
5. update the tests.

Do not let the architecture drift through many undocumented "temporary" exceptions.

## 29. Definition of architecture compliance

A change is compliant when:

- domain holds business rules, application holds orchestration, adapters hold technology;
- dependencies point inward and `loops arch` reports no errors;
- every mutation is a typed command or owned by one owner;
- external concerns enter through ports and every port has a fake;
- tests run headlessly;
- consistency behaviour is preserved (ordering, idempotency, injected clock and RNG);
- package ownership stays explicit and public contracts are documented.

## 30. Final directive

Use hexagonal architecture to keep dependency direction and testability, not to push every function through five interfaces. Prefer concrete domain code with small ports at real boundaries. The architecture should make the system easier to understand, test and extend, not more ceremonial.
