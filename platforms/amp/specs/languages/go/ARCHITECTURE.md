# Go architecture

The Go mechanics of `specs/ARCHITECTURE.md` and `specs/SOLID.md`. The neutral rules live there; this file only says how Go spells them and what `loops arch` checks in Go code. The Go code rules are `specs/languages/go/RULES.md`; the pack data is `specs/languages/go/pack.json` (`arch`).

## 1. Repository layout (ARCHITECTURE §4)

```text
repo/
├── cmd/<svc>/main.go          wiring only (§4)
├── internal/
│   ├── domain/<concept>/      entities, rules, domain services; stdlib + pkg only
│   ├── application/<usecase>/ use cases, command handlers, queries; consumer-owned ports
│   ├── ports/{inbound,outbound}/  shared port interfaces, when several packages use one
│   ├── adapters/
│   │   ├── inbound/           http, grpc, cli, consumer, scheduler
│   │   └── outbound/          postgres, redis, kafka, <vendor> clients
│   ├── platform/              config, logging, metrics, tracing, clock, pools
│   └── compose/<svc>/         composition root helpers; only cmd/ imports them
├── pkg/                       low-level primitives with no business concepts (§3)
├── migrations/                versioned schema migrations
└── AGENTS.md  README.md  Makefile
```

One module (`go.mod`) per repository until a separately released or deployed boundary needs its own (`specs/REPO_PRACTICES.md §2`).

## 2. `context.Context` (ARCHITECTURE §14)

Use `context.Context` at I/O and long-running boundaries: handlers, repositories, clients, jobs, migrations. First parameter, never stored in a struct. Do not thread it through pure domain functions. Deadlines and cancellation flow from the inbound adapter outward (`specs/languages/go/RULES.md §10`).

```go
type Charger interface { // owned by package payment, the consumer
    Charge(ctx context.Context, req ChargeRequest) (ChargeResult, error)
}
```

Errors (ARCHITECTURE §13): wrap with `%w`; callers branch with `errors.Is` and `errors.As`. Do not represent ordinary business outcomes as Go errors unless they are truly exceptional (`specs/languages/go/RULES.md §8–§9`).

## 3. `pkg/` (ARCHITECTURE §15)

`pkg/` is reserved for reusable low-level primitives with no business concepts and no dependency on `internal/`: money arithmetic, retry/backoff, ID generation, bit sets, deterministic heaps. Add a primitive only when the standard library lacks it (`specs/languages/go/RULES.md §48`). Do not create `pkg/` by default; most repositories need only `internal/`.

## 4. `cmd` is wiring only (ARCHITECTURE §23, SOLID §8a)

Every `cmd/<tool>` directory holds exactly one file, `main.go`, declaring only `func main`, at most 80 lines. The body parses arguments through a package, builds adapters and application types, passes them on as interfaces, calls one `Run` and exits. It has no `for`, `range`, `switch`, `select` or `go` statements, no function literals, no tests and no string literal over 40 runes. Reported as `cmd` errors.

| Logic | Home |
|---|---|
| Flags, usage text, a tool's `Run` | `internal/adapters/inbound/cli/<tool>` |
| Composition needing many adapters | `internal/compose/<svc>`, imported only by `cmd/` |
| Profiling, log file, signal handling | `internal/platform/<name>` |
| Clients a tool calls | `internal/adapters/outbound/<client>` |

```go
func NewCheckout(orders OrderRepository, pay PaymentGateway, clock Clock, bus EventPublisher) *Checkout
```

No `func init()` registration: registries are filled explicitly in the root or the owning constructor (SOLID §4).

## 5. `loops arch` in Go (ARCHITECTURE §25)

`loops arch` parses the module with the built-in Go parser. Layer directories default to the tree in §1 and can be overridden in `.loops/config.json` under `layers`: an object with keys `domain`, `application`, `ports`, `adapters`, `inbound`, `outbound`, `platform`, `compose`, `cmd`, `pkg`, each a list of module-relative prefixes; omitted keys keep the defaults.

| Rule | Severity | Go check |
|---|---|---|
| `imports` | error | Domain imports no application, ports, adapters, platform or cmd, and no `os/*` or `net/http/*`; domain, application and `pkg` import only the standard library (third-party is an error); application and ports import no adapters; outbound adapters do not import inbound; only `cmd/` imports `internal/compose`. A sub-package importing its parent is an error in domain and application, a warning elsewhere |
| `clock` | error | No `time.Now` in domain |
| `rand` | error | No global `math/rand` or `math/rand/v2` draw in domain (`New`, `NewSource`, `NewPCG`... are allowed) |
| `go` | error | No `go` statement in domain; `go` statements appear only in a pool, a server loop or a consumer loop |
| `init` | error | No `func init()` |
| `global` | error | No runtime-assigned package-level `var` (§8) |
| `cmd` | error | §4 |
| `methods`, `fields`, `iface`, `lines`, `files` | warn | §7 |
| `decls` | warn | Package-level `var`/`const` after the first `func` of a non-test file (RULES §55) |

## 6. Go vocabulary for SOLID (SOLID §2)

- **type** is a named type with methods: methods on data, not a class. No hierarchy, no base type.
- **port** is an interface. Interfaces are satisfied implicitly, so the consumer defines them; "accept interfaces, return concrete types" (`specs/languages/go/RULES.md §5–§6`). Constructor returns, including the unexported `New<X>` case, follow RULES §56.
- **module** is the package: the unit of cohesion and visibility.
- Embedding composes; it does not inherit. Substitutability exists only through interfaces and their documented contracts.

## 7. Limits in Go units (SOLID §3, §8)

| Rule id | Go measure |
|---|---|
| `methods` | Methods declared on a type across all files of its package, <= 40 |
| `fields` | Fields of a struct with >= 5 methods, <= 25 |
| `iface` | Methods written in an interface, <= 8; an embedded interface counts 1 |
| `lines` | Non-test Go lines per package, <= 12,000 |
| `files` | Non-test Go files per package, <= 60 |

```go
// Bad: Server has 60 fields, one per endpoint's state.
// Good: a feature struct owns its state, constructed by the shell.
type refundHandler struct {
    refunds RefundUseCase // consumer-owned, <=8 methods
    limits  limits
}

func newRefundHandler(r RefundUseCase, l limits) *refundHandler { return &refundHandler{refunds: r, limits: l} }
```

- An optional capability is a type assertion (`x.(interface{ Flush() error })`) and follows SOLID §5.
- Splitting a god type: the new type's consumer-owned interface goes in its own `deps.go` (SOLID §10).
- In hot paths, interface methods take values rather than pointers to the caller's locals, because an interface call hides the callee from escape analysis; benchmark with `-benchmem` after adding a host method (SOLID §7).

## 8. Package-level state (SOLID §9)

Preference and placement of package-level declarations are RULES §55. The `global` check allows only these variables:

- `sync.Once*`, `sync.Pool`, `sync.Map`, atomic values, and a cache whose mutex or `Once` is declared in the same `var (...)` group;
- immutable lookup tables, never assigned after declaration;
- interface assertions: `var _ I = (*T)(nil)`;
- sentinel errors (`var ErrNotFound = errors.New(...)`) and compiled regexps assigned at declaration.

Not allowed: any other variable assigned at runtime. Reported as `global` errors.
