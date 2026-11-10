# SOLID in Go

The Go reading of SOLID: responsibility, extension and size limits that keep types and packages small enough for a person (or an agent) to hold in context. Extracted from rimworld-go. `loops arch` checks them (§8, §8a, §9).

## 1. Why this document exists

Dependency rules (`specs/ARCHITECTURE.md`) are easy to hold. Size and shape rules drift unnoticed: god structs with dozens of fields, interfaces mirroring whole services, `init()` registration, mutable package globals. `loops arch` makes the drift visible. This document gives the Go vocabulary and the limits; it restates no rule from the other specs.

## 2. Go vocabulary for SOLID

- A type is methods on data, not a class: no hierarchy, no base type. Responsibility is measured by what state a type owns and which transitions it performs.
- Interfaces are satisfied implicitly, so the consumer defines them. An interface is the consumer's list of calls, sized by that consumer.
- Embedding composes; it does not inherit. Substitutability exists only through interfaces and their documented contracts.
- The package, not the type, is the unit of cohesion and visibility (`specs/ARCHITECTURE.md §15`). Split a type inside its package first; a new package is the last step.

## 3. S: single responsibility

One type owns one family of state transitions. One package owns one concept. One file holds one feature of that concept.

- A type has at most 40 methods and, if it has behaviour, at most 25 fields. A pure data DTO is exempt.
- A package has at most 12,000 non-test lines and 60 files.
- When a feature needs new state on a shell type (`Server`, `App`, `Service`), give it its own type; do not add a field to the shell.

```go
// Bad: Server has 60 fields, one per endpoint's state.
// Good: a feature struct owns its state, constructed by the shell.
type refundHandler struct {
    refunds RefundUseCase // consumer-owned, <=8 methods
    limits  limits
}

func newRefundHandler(r RefundUseCase, l limits) *refundHandler { return &refundHandler{refunds: r, limits: l} }
```

The shell keeps one field `refund *refundHandler` and forwards the entry points.

## 4. O: open/closed

Extend by adding a value to a table or a type that satisfies an interface; do not edit a central switch for every feature.

- Registries are filled explicitly at the composition root or in the owning constructor, never from `init()`.
- A dispatch over string prefixes or message types exists once per boundary. A new feature registers a handler; it does not add a `case`.

```go
func NewRouter(routes []Route) *Router // each feature exports a Route; the root lists them in one place
```

## 5. L: Liskov

Anything that satisfies an interface honours its documented contract: same preconditions, same results, same errors. A type assertion to an optional capability (`x.(interface{ Flush() error })`) is a second interface and needs its own contract. Allowed only when all hold:

- the feature degrades gracefully without the capability;
- the capability has at most 3 methods and is documented as optional;
- it is not in a hot loop;
- it is not used to reach a method every real implementation has (put that method on the right small interface instead).

## 6. I: interface segregation

An interface is a consumer's list of calls.

- At most 8 explicit methods. Compose larger needs by embedding; an embedded interface counts as 1.
- Add a method to the interface whose consumer calls it, never to the widest interface nearby.
- Bad: a `Store` with 30 methods consumed alike by handlers, jobs and reports; every fake implements all 30. Good: `OrderReader`, `OrderWriter`, `Clock`, `Publisher`, each owned by its consumer.

## 7. D: dependency inversion

The mandatory rule is `specs/ARCHITECTURE.md §6` with `specs/languages/go/RULES.md §5-6, §41`: ports are consumer-owned and concrete types are built only at the composition root.

Feature sub-packages are consumers: they define what they need from the root, the root constructs them, and a sub-package never imports its parent. In hot paths, interface methods take values rather than pointers to the caller's locals, because an interface call hides the callee from escape analysis; benchmark with `-benchmem` after adding a host method.

## 8. Measurable limits

`loops arch` reports these as **warnings**. They never fail `loops gate`. There is no budget file and no ratchet: the numbers are advisory targets that keep reviews honest, and a warning on code you touched is a signal to split (§10) or to say why not in the claims report.

| Rule id | Limit | Measures |
|---|---|---|
| `methods` | <= 40 per type | Methods declared on a type across all files of its package |
| `fields` | <= 25 per struct with >= 5 methods | Fields of a type that has behaviour; DTOs exempt |
| `iface` | <= 8 explicit methods | Methods written in an interface; an embedded interface counts 1 |
| `lines` | <= 12,000 non-test lines per package | Non-test Go lines |
| `files` | <= 60 per package | Non-test Go files |

Error-severity rules, enforced by `loops arch` and `loops gate`:

| Rule id | Limit | Measures |
|---|---|---|
| `init` | 0 | `func init()` declarations (registration is explicit, §4) |
| `global` | 0 | Package-level `var` assigned at runtime (§9) |
| `imports` | forbidden edges | Dependency direction, stdlib-only core, compose only from cmd (`specs/ARCHITECTURE.md §25`) |
| `clock` | 0 | `time.Now` in domain |
| `rand` | 0 | Global `math/rand` draw in domain |
| `go` | 0 | `go` statements in domain |
| `cmd` | wiring only | §8a |

Do not add to a type or package that is already over a limit; split it. Run `loops arch` before handing work over if you touched a type, an interface or an import.

## 8a. `cmd` is wiring

Every `cmd/<tool>` directory holds exactly one file, `main.go`, declaring only `func main`, at most 80 lines. The body parses arguments through a package, builds adapters and application types, passes them on as interfaces, calls one `Run` and exits. It has no `for`, `range`, `switch`, `select` or `go` statements, no function literals and no string literal over 40 runes. Logic goes to the homes in `specs/ARCHITECTURE.md §23`. Reported as `cmd` errors.

## 9. Mutable package-level state

Allowed:

- `sync.Once*`, `sync.Pool`, `sync.Map`, atomic values, and a cache whose mutex or `Once` is declared in the same `var (...)` group;
- immutable lookup tables, never assigned after declaration;
- interface assertions: `var _ I = (*T)(nil)`;
- sentinel errors (`var ErrNotFound = errors.New(...)`) and compiled regexps assigned at declaration.

Not allowed: any other variable assigned at runtime. Move it onto the type that owns it, or make it a constructor option. A test hook is a field on its owner, nil in production, never a global a test swaps and restores. Reported as `global` errors.

## 10. How to split a god type

1. Pick one feature cluster. File names such as `*_handler.go` or `*_billing.go` mark the seams.
2. List the fields and methods only that cluster touches. A field used by two clusters stays on the shell.
3. Declare `type X struct` holding those fields.
4. Write the consumer-owned interface X needs from the shell, at most 8 methods, in X's own `deps.go`.
5. Add `NewX(deps)` and call it from the root constructor.
6. The root keeps one field `x *X` and one forwarding method per entry point; delete each forwarder as callers are repointed to `x`.
7. Move the cluster's tests with it in the same change. Integration tests that drive the whole shell stay at the root.

A cluster may become a sub-package when it owns state no other cluster writes and everything it needs from the root fits behind consumer-owned roles in its `deps.go` (each at most 8 methods, composed by embedding). The root builds it and implements the roles with small adapter types; the sub-package never imports the root. Keep the sub-package to one concept, not one package per type, and no `utils`.

## 11. What not to do

- No interface per struct (`specs/languages/go/RULES.md §5`, `specs/ARCHITECTURE.md §27`).
- No `utils`, `helpers` or `common` packages (`specs/languages/go/RULES.md §3`).
- No package per struct.
- No provider-owned "full" interface that mirrors a concrete type.
- Do not split a function only to meet a line count (`specs/languages/go/RULES.md §27`).
- No forwarding wrapper that outlives the change that repoints its callers.
- No abstraction for a single implementation that crosses no package boundary.
