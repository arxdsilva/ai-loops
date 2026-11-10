# Hexagonal Architecture

Authoritative layering, ports and repository model for a backend in any language, so that business rules are testable without infrastructure and agents can work in parallel without collisions. Extracted from rimworld-go and generalised. `loops arch` checks the mechanical parts (§25).

This file states each rule once, in neutral terms. The examples are pseudo-code. How a language spells a rule (its layout tree, its port construct, its lint mechanics) lives in its pack: `skills/lang-<id>/architecture.md` and `specs/languages/<id>/RULES.md §ARCH` (the table below; Go's `RULES.md` is numbered and maps the IDs in its appendix). Go's details are in `specs/languages/go/ARCHITECTURE.md`.

| Language (`id`) | Layout, ports and wiring | Enforcement (`loops arch` + the pack's native tool, TOOLING §6.1) |
|---|---|---|
| python | `skills/lang-python/architecture.md` | delegates to import-linter |
| typescript | `skills/lang-typescript/architecture.md` | scanner + dependency-cruiser |
| java | `skills/lang-java/architecture.md` | scanner + ArchUnit |
| cpp | `skills/lang-cpp/architecture.md` | scanner + CMake target graph |
| csharp | `skills/lang-csharp/architecture.md` | scanner on the project graph |
| php | `skills/lang-php/architecture.md` | scanner + deptrac |
| go | `skills/lang-go/architecture.md`, `specs/languages/go/ARCHITECTURE.md` | built-in Go parser |
| ruby | `skills/lang-ruby/architecture.md` | delegates to packwerk |
| swift | `skills/lang-swift/architecture.md` | build import check + scanner |
| kotlin | `skills/lang-kotlin/architecture.md` | scanner + Gradle module graph |
| rust | `skills/lang-rust/architecture.md` | crate graph + scanner |
| scala | `skills/lang-scala/architecture.md` | sbt `dependsOn` + scanner |
| elixir | `skills/lang-elixir/architecture.md` | delegates to boundary and `mix xref` |
| erlang | `skills/lang-erlang/architecture.md` | delegates to `rebar3 xref` |
| clojure | `skills/lang-clojure/architecture.md` | scanner (+ `poly check`) |

## 1. Purpose

- Business logic is independent of transport, storage, queues and vendors.
- Everything is testable headlessly: no network, database or filesystem in domain or application tests.
- Modules are small and owned, so several agents can change different layers at once.
- The architecture is explicit rather than framework-heavy. It preserves the direction of dependencies and testability, not ceremony.
- Follow the framework's conventions where the project uses one, and keep the domain framework-free (WORKFLOW §1: project rules win).

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

The domain may use only the language's standard library and pure, low-level utility modules with no business concepts.

Forbidden in `domain` (and mostly in `application`): HTTP, gRPC, SQL drivers, ORMs, queue clients, process and OS access, the filesystem, CLI frameworks, config loaders, loggers that change behaviour, vendor SDKs, web or DI framework annotations, generated transport or persistence types. Each pack lists its forbidden modules as `arch.forbidden_in_domain`.

Boundary test for every import: does the inner model need to know this technology exists? If not, keep it outside.

## 4. Repository structure

The layers, whatever the language calls its directories:

| Layer home | Holds |
|---|---|
| entry points | one wiring-only composition root per deployable (§23) |
| `domain/<concept>` | entities, rules, domain services; standard library and pure utilities only |
| `application/<usecase>` | use cases, command handlers, queries; consumer-owned ports |
| `ports/{inbound,outbound}` | shared port declarations, only when several modules use one |
| `adapters/inbound` | http, grpc, cli, consumer, scheduler |
| `adapters/outbound` | postgres, redis, kafka, `<vendor>` clients |
| `platform` | config, logging, metrics, tracing, clock, pools |
| `compose/<svc>` | composition helpers, used only by the entry points |
| shared primitives | low-level utilities with no business concepts (§15) |
| `migrations/` | versioned schema migrations |

Use the ecosystem's canonical source, test and resource layout around these homes (`specs/REPO_PRACTICES.md §1`). The tree per language is in its `skills/lang-<id>/architecture.md`; Go's is `specs/languages/go/ARCHITECTURE.md §1`.

Module detail lives in each module's `AGENTS.md` (`templates/PACKAGE_AGENTS.md`). Create a module when its first real behaviour needs it, not to match a diagram.

## 5. What each layer owns

| Layer | Owns | Must not |
|---|---|---|
| Domain | Business rules, invariants, entities, value objects, state transitions, domain services. Runnable headlessly with fakes for nothing. | Know about I/O, time sources, randomness sources, config |
| Application | Orchestration: load, call domain, persist, publish; transactions; idempotency; authorisation checks that need data; read models | Contain business rules; import adapters |
| Ports | Narrow interfaces at boundaries: repositories, clocks, publishers, clients | Mirror a concrete type |
| Adapters | Translating a technology into a port or a port into a request: handlers, repositories, clients, consumers | Hold business rules; leak library types inward |
| Platform | Cross-cutting infrastructure the process needs: config loading, logging setup, metrics, clock implementation | Be imported by domain |

## 6. Ports are consumer-owned

Mandatory: code that depends on an adapter or an external technology depends on a narrow abstraction describing the behaviour it needs (an interface, trait, protocol, behaviour or function type), never on the concrete type. The concrete adapter is constructed once at the composition root and passed around as the abstraction it satisfies. This is dependency inversion applied at the boundary: the abstraction is what both sides depend on, so real, fake and replacement implementations swap without touching consumers.

- The same applies inward: an inbound adapter does not hold the concrete application service just because that is what exists. It declares a small port listing only the operations it calls, and the concrete type satisfies it.
- Keep ports small and single-purpose (interface segregation, `specs/SOLID.md §6`). Never add an operation to an existing port "just in case".
- Declare a port wherever a module boundary is crossed (adapter and application, application and a domain-facing port), even with one implementation today. Do not declare one merely because a concrete type exists.

```text
module payment                      # the consumer declares the port
port Charger
  charge(request: ChargeRequest) -> ChargeResult | error
```

## 7. Inbound ports

Operations external actors can request. All inbound adapters (HTTP, gRPC, CLI, queue consumers, jobs) use the same application surface where practical.

```text
port Checkout
  place(command: PlaceOrder) -> OrderID | error
  get(id: OrderID) -> OrderView | error
```

## 8. Outbound ports

```text
port OrderRepository
  save(order: Order) -> ok | error
  by_id(id: OrderID) -> Order | error

port EventPublisher
  publish(event: Event) -> ok | error

port PaymentGateway
  charge(request: ChargeRequest) -> ChargeResult | error
```

Adapters implement these. Library, driver and SQL row types never cross a port. Entities reference each other by ID, not by object graph.

## 9. Domain services vs application services

- Domain service: a business rule spanning several domain objects: `PriceOrder`, `ApplyDiscount`, `AllocateStock`, `TransferFunds`.
- Application service: orchestration: `PlaceOrder`, `CancelSubscription`, `RetryFailedPayments`, `ImportCatalog`.

## 10. Commands

State changes enter as typed commands handled by the application layer.

```text
command PlaceOrder
  customer: CustomerID
  items:    list of Item
  key:      IdempotencyKey
```

- A command is a plain, immutable, serialisable value: no HTTP request, no SQL row.
- Handlers validate, load, call the domain, persist and publish; they return a typed result.
- Transport adapters parse and validate input once at the edge, map request to command and result to response; they never mutate domain state directly.
- Retried commands carry an idempotency key (`skills/consistency`).

## 11. Queries and read models

- Reads return explicit read models (DTOs), not domain internals and not storage rows.
- Query code may bypass the domain model for performance (a SQL projection) but never mutates.
- Do not hand mutable stores or entities to adapters.

```text
read model OrderView
  id:     OrderID
  status: text
  total:  Money          # never a float: decimal or integer minor units
```

## 12. One owner per piece of mutable state

Every piece of mutable state has exactly one owner: an aggregate behind a repository, an actor or process, a lock-guarded object, a table with a single writer service.

- Others communicate by commands or messages, or read immutable snapshots.
- Workers may compute (pricing, rendering, batch scoring, encoding, aggregation) but do not mutate authoritative state directly.
- Worker pattern for results that must be applied consistently:

```text
snapshot -> enqueue work with sequence ID -> workers compute -> collect
-> sort by sequence -> validate revision -> commit on the owner
```

- Concurrent work (threads, tasks, goroutines, processes) is spawned only in a bounded pool, a server loop or a consumer loop, never in `domain`. Blocking I/O never runs on an event loop or scheduler thread.

## 13. Error taxonomy

| Kind | Examples | Represented as |
|---|---|---|
| Expected domain outcome | `InsufficientStock`, `CardDeclined`, `AlreadyCancelled` | Result values, tagged values or typed domain errors |
| Validation | invalid command, malformed input, invalid config | Typed validation error, mapped to 4xx at the edge |
| Infrastructure | database down, timeout, corrupt payload | A wrapped error or exception with context, mapped to 5xx or retried |

- The inbound adapter owns the mapping from error kind to transport status (HTTP code with an RFC 9457 problem body, gRPC status, exit code).
- Ordinary business outcomes are values, not exceptions or panics; those are for faults.
- Never swallow an error. Add context when wrapping or rethrowing, handle each error once, and let callers branch on the kind, never on the message. The language's mechanics: `specs/languages/<id>/RULES.md §IDIOM`.

## 14. Cancellation and deadlines

Every I/O or long-running boundary (handlers, repositories, clients, jobs, migrations) accepts cancellation and a deadline, derived from the inbound request and propagated outward. Pure domain functions take neither. The language's mechanism (Go: `context.Context`, `specs/languages/go/ARCHITECTURE.md §2`) is in its pack.

## 15. Module boundaries, domain types, shared primitives

- A module owns one coherent concept: prefer `domain/pricing` over a catch-all `domain/shop`. Do not fragment into one module per type. A type is split inside its module first; a new module is the last step (`specs/SOLID.md §2`).
- Canonical types have one home. Search before creating `UserID`, `Money`, `Clock` or an error type; never recreate one elsewhere.
- Shared primitives are reusable, low-level utilities with no business concepts and no dependency on the application: money arithmetic, retry and backoff, ID generation, bit sets, deterministic heaps. Add one only when the standard library and the well-known libraries lack it (`specs/REPO_PRACTICES.md §5`). Go: `specs/languages/go/ARCHITECTURE.md §3`.
- Export only what callers need: the public surface is minimal and documented.

## 16. Domain events

Typed events decouple domain effects from reactions.

```text
event OrderPaid
  order: OrderID
  at:    Instant          # UTC; the zone is applied at the edge
```

- Explicit and versioned; no untyped map payloads.
- Published after the state change is committed (outbox pattern when delivery must be reliable).
- Consumers are idempotent: delivery is at least once.
- No hidden synchronous side effects; ordering guarantees are documented per stream.

## 17. State machines

Entities with lifecycles (orders, payments, subscriptions, jobs) use explicit state enums (or sum types) and one transition function that checks the source state. Prefer visible transitions over callback chains (`skills/jobs-and-state-machines`).

## 18. Persistence boundary

- Domain types are not the storage schema. Repositories translate between domain and row/document DTOs.
- Schema changes are versioned migrations in `migrations/`, tested with fixtures; zero-downtime changes expand first and contract later; rollbacks and backfills are planned, not improvised.
- Transactions are opened by the application layer through a port (unit of work), not by handlers or domain.
- Persisted and wire formats are versioned (`skills/consistency`).

## 19. Config boundary

Config and content are loaded in the composition root or `platform` from the environment and files, validated at startup (fail fast, listing every error), converted to typed immutable values and passed in. Domain and application never read the environment or files. Secrets come from the environment or a vault and are never logged. The language's loaders: `specs/languages/<id>/RULES.md §OPS`.

## 20. Time boundary

Authoritative code takes a `Clock` port, and randomness and ID generation come from injected sources too. Wall-clock time is used by logging timestamps, adapters, schedulers and profiling only. Store and compute in UTC instants; convert zones at the edge. A test must be able to control time without sleeping. Each pack lists the calls that break this as `arch.impure_calls`.

## 21. Dependency injection

Explicit constructor injection, wired by hand at one composition root. No service locators, no global registries, no field injection. A container is acceptable only where it is the framework's norm and the project uses it (WORKFLOW §1); it is then configured in the composition root alone.

```text
new_checkout(orders: OrderRepository, pay: PaymentGateway, clock: Clock, bus: EventPublisher) -> Checkout
```

## 22. Initialization order

```text
config -> logging/metrics/tracing -> load and validate content
-> construct outbound adapters -> construct application services
-> construct inbound adapters -> run (serve / consume / schedule)
-> graceful shutdown in reverse order
```

Fail fast on invalid config or unreachable required dependencies. Shutdown on SIGTERM honours a deadline and drains in-flight work.

## 23. The composition root is wiring only

Each deployable has one entry point whose only job is to parse arguments through a module, build the concrete adapters and application services, hand them over as ports, call one `run` and exit. It holds no business logic, loops, dispatch or helpers. No other module imports the composition helpers.

| Logic | Home |
|---|---|
| Flags, usage text, a tool's `run` | `adapters/inbound/cli/<tool>` |
| Composition needing many adapters | `compose/<svc>`, used only by the entry point |
| Profiling, log file, signal handling | `platform/<name>` |
| Clients a tool calls | `adapters/outbound/<client>` |

Go's mechanical version (`cmd/<tool>/main.go`, 80 lines, reported as `cmd` errors): `specs/languages/go/ARCHITECTURE.md §4`.

## 24. Testing architecture

- Domain tests: unit tests, no fakes needed beyond an injected clock and RNG.
- Application tests: fake outbound ports (in-memory repository, recording publisher, scripted gateway). Prefer real domain objects.
- Adapter tests validate the technology translation separately: handler tests through the framework's test client, repository tests against a real database in a container or a documented fixture, client contract tests against a fake server.
- Integration tests wire real adapters only where necessary; end-to-end checks hit the running service.
- Good fakes: in-memory store, deterministic clock, scripted gateway, recording publisher. Avoid mocking every call; avoid asserting call counts of private helpers.

The test rules themselves are `specs/TESTING.md`.

## 25. Architecture lint

`loops arch` checks direction and purity from the pack's `arch` data: its parser (built in, a regex scanner, or none), its resolver, `forbidden_in_domain`, `impure_calls`, `third_party_forbidden_in` and `size_warnings`. Where a pack delegates (`arch.native_check`), the native tool's check runs as an `arch` check in `loops gate`. Layer directories can be overridden in `.loops/config.json` under `layers`.

| Rule | Severity | Checks |
|---|---|---|
| `imports` | error | Domain imports no application, ports, adapters, platform or entry point, and nothing in `forbidden_in_domain`; the core imports no third-party code where the pack says so; application and ports import no adapters; outbound adapters do not import inbound; only the entry point uses the composition helpers |
| `clock` | error | No wall-clock read in domain (`impure_calls`) |
| `rand` | error | No global random draw or random ID in domain (`impure_calls`) |
| `go` | error | No spawning of concurrent work in domain (`impure_calls`) |
| `init` | error | No implicit registration at load time (`specs/SOLID.md §4`) |
| `global` | error | No runtime-assigned module-level state (`specs/SOLID.md §9`) |
| `cmd` | error | The composition root is wiring only (§23) |
| `methods`, `fields`, `iface`, `lines`, `files` | warn | Size limits (`specs/SOLID.md §8`) |

Which rules a pack can check depends on its parser; Go checks all of them (`specs/languages/go/ARCHITECTURE.md §5`). `loops gate` fails only on errors. Unchecked mechanically: `utils` modules and edits to generated code; these are review items (`skills/review-integration`).

## 26. Architecture decision records

Write an ADR (`templates/ADR.md`, `docs/adr/NNNN-<slug>.md`) only for a decision that changes a contract: a port, the persisted or wire schema, the concurrency model, the command set, the consistency model. Everything else goes in the module `AGENTS.md` or the PR body. An ADR records context, decision, consequences and alternatives, not reasoning transcripts.

## 27. Architecture smells

Flag for review:

- ports with 20+ operations; a giant `AppContext` passed everywhere;
- adapters returning raw domain internals; domain importing transport or storage types;
- generated serialisation code in `domain`;
- untyped, dynamic-heavy APIs; pervasive reflection, macros or monkey-patching outside framework edges;
- global registries; module cycles; `utils`;
- hidden concurrent mutation of shared state;
- an interface per class;
- `Manager`/`Service` naming without clear ownership;
- handlers that contain business rules; repositories that contain business rules.

## 28. Migration policy

If the architecture must change:

1. write or update the ADR;
2. update the ports;
3. update all call sites in the same change where practical;
4. update the module `AGENTS.md`;
5. update the tests.

Do not let the architecture drift through many undocumented "temporary" exceptions.

## 29. Definition of architecture compliance

A change is compliant when:

- domain holds business rules, application holds orchestration, adapters hold technology;
- dependencies point inward and `loops arch` (plus the pack's native check, if any) reports no errors;
- every mutation is a typed command or owned by one owner;
- external concerns enter through ports and every port has a fake;
- tests run headlessly;
- consistency behaviour is preserved (ordering, idempotency, injected clock and RNG);
- module ownership stays explicit and public contracts are documented.

## 30. Final directive

Use hexagonal architecture to keep dependency direction and testability, not to push every function through five interfaces. Prefer concrete domain code with small ports at real boundaries. The architecture should make the system easier to understand, test and extend, not more ceremonial.
