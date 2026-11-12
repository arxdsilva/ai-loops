# SOLID

SOLID stated per unit, in any language: responsibility, extension and size limits that keep types and modules small enough for a person (or an agent) to hold in context. Extracted from rimworld-go and generalised. `loops arch` checks the limits a pack can measure (§8, §8a, §9). The Go vocabulary, limits and allowed-lists are in `specs/languages/go/ARCHITECTURE.md §6–§8`; other languages map the units in `skills/lang-<id>/architecture.md`.

## 1. Why this document exists

Dependency rules (`specs/ARCHITECTURE.md`) are easy to hold. Size and shape rules drift unnoticed: god classes with dozens of fields, interfaces mirroring whole services, registration on load, mutable globals. `loops arch` makes the drift visible. This document gives the neutral units and the limits; it restates no rule from the other specs.

## 2. The units

Every limit below is stated in four neutral units. Each pack says what they mean in its language.

| Unit | Means | Examples |
|---|---|---|
| **type** | a named thing with state and/or behaviour | class, struct with methods, record, data class, object, GenServer module, Clojure record |
| **port** | a consumer-declared contract | interface, trait, protocol, behaviour, abstract base class, function type |
| **module** | the unit of cohesion and visibility | package, namespace, crate module, Python package, Elixir context, OTP application |
| **function** | one callable | method, function, closure, handler |

- Responsibility is measured by what state a type owns and which transitions it performs, not by inheritance.
- Composition beats inheritance. Substitutability comes through ports and their documented contracts.
- The module, not the type, is the unit of cohesion (`specs/ARCHITECTURE.md §15`). Split a type inside its module first; a new module is the last step.

## 3. S: single responsibility

One type owns one family of state transitions. One module owns one concept. One file holds one feature of that concept.

- A type stays within the §8 limits for operations and, if it has behaviour, fields. A pure data DTO is exempt.
- A module stays within the §8 limits for lines and files.
- A function is focused: a clear 40-line state transition beats eight one-line helpers. Many related parameters become a request or command type.
- When a feature needs new state on a shell type (`Server`, `App`, `Service`), give it its own type; do not add a field to the shell.

```text
# Bad: Server has 60 fields, one per endpoint's state.
# Good: a feature type owns its state, constructed by the shell.
type RefundHandler
  refunds: RefundUseCase      # consumer-owned port, <= 8 operations
  limits:  Limits
```

The shell keeps one field `refund: RefundHandler` and forwards the entry points.

## 4. O: open/closed

Extend by adding a value to a table or a type that satisfies a port; do not edit a central switch for every feature.

- Registries are filled explicitly at the composition root or in the owning constructor, never by side effects at load time (static initialisers, import-time decorators, `init` functions, auto-discovery the domain depends on).
- A dispatch over string prefixes or message types exists once per boundary. A new feature registers a handler; it does not add a `case`.

```text
new_router(routes: list of Route) -> Router   # each feature exports a Route; the root lists them in one place
```

## 5. L: Liskov

Anything that satisfies a port honours its documented contract: same preconditions, same results, same errors. A runtime check for an optional capability (a type test, a `respond_to?`, a downcast) is a second port and needs its own contract. Allowed only when all hold:

- the feature degrades gracefully without the capability;
- the capability has at most 3 operations and is documented as optional;
- it is not in a hot loop;
- it is not used to reach an operation every real implementation has (put that operation on the right small port instead).

## 6. I: interface segregation

A port is a consumer's list of calls.

- At most 8 explicit operations. Compose larger needs from smaller ports; an embedded or extended port counts as 1.
- Add an operation to the port whose consumer calls it, never to the widest port nearby.
- Bad: a `Store` with 30 operations consumed alike by handlers, jobs and reports; every fake implements all 30. Good: `OrderReader`, `OrderWriter`, `Clock`, `Publisher`, each owned by its consumer.

## 7. D: dependency inversion

The mandatory rule is `specs/ARCHITECTURE.md §6`: ports are consumer-owned and concrete types are built only at the composition root (`specs/ARCHITECTURE.md §21`).

Feature sub-modules are consumers: they declare what they need from the root, the root constructs them, and a sub-module never imports its parent. In hot paths, measure the cost of the indirection (the language's profiler or benchmark tool) after adding a port.

## 8. Measurable limits

`loops arch` reports these as **warnings** where the pack can measure them (its `arch.size_warnings`; a pack may tighten a number, never loosen it). They never fail `loops gate`. There is no budget file and no ratchet: the numbers are advisory targets that keep reviews honest, and a warning on code you touched is a signal to split (§10) or to say why not in the claims report.

| Rule id | Unit | Limit | Measures |
|---|---|---|---|
| `methods` | type | <= 40 | Operations declared on a type, across all its files |
| `fields` | type | <= 25, for a type with >= 5 operations | Fields of a type that has behaviour; DTOs exempt |
| `iface` | port | <= 8 explicit operations | Operations written in a port; an embedded or extended port counts 1 |
| `lines` | module | <= 12,000 non-test lines | Non-test source lines |
| `files` | module | <= 60 | Non-test source files |

Review heuristics, checked by the pack's linter where it is configured (`specs/languages/<id>/RULES.md §ARCH`), never by `loops arch`:

| Unit | Target |
|---|---|
| file | about 400 lines; one feature per file |
| function | about 40 lines; at most 5 parameters (more become a request type) |

Error-severity rules, enforced by `loops arch` and `loops gate` where the pack's parser detects them (`specs/ARCHITECTURE.md §25`):

| Rule id | Limit | Measures |
|---|---|---|
| `init` | 0 | Implicit registration at load time (§4) |
| `global` | 0 | Module-level state assigned at runtime (§9) |
| `imports` | forbidden edges | Dependency direction, a dependency-free core, composition helpers used only by the entry point (`specs/ARCHITECTURE.md §25`) |
| `clock` | 0 | Wall-clock reads in domain |
| `rand` | 0 | Global random draws in domain |
| `go` | 0 | Spawning concurrent work in domain |
| `cmd` | wiring only | §8a |

Do not add to a type or module that is already over a limit; split it. Run `loops arch` before handing work over if you touched a type, a port or an import.

## 8a. The composition root is wiring

The entry point of each deployable parses arguments through a module, builds adapters and application types, passes them on as ports, calls one `run` and exits (`specs/ARCHITECTURE.md §23`). It has no loops, dispatch, spawned work, closures or long literals. Go's measured form (one `main.go`, at most 80 lines) is `specs/languages/go/ARCHITECTURE.md §4`; it is reported as `cmd` errors.

## 9. Mutable module-level state

No module-level variable is assigned at runtime. Move the state onto the type that owns it, or make it a constructor option. A test hook is a field on its owner, nil or absent in production, never a global a test swaps and restores. Reported as `global` errors where the pack detects them.

Allowed everywhere: immutable lookup tables and constants set at declaration; compiled patterns; sentinel error values; lazily initialised caches guarded by the language's once-only or lock primitive declared beside them. Each language's exact allowed-list (Go: `specs/languages/go/ARCHITECTURE.md §8`) is in its pack. Process-owned state in an actor runtime (an OTP process, an agent, an atom held by its owner) is ownership, not a global (`specs/ARCHITECTURE.md §12`).

## 10. How to split a god type

1. Pick one feature cluster. File names such as `*_handler.*` or `*_billing.*` mark the seams.
2. List the fields and operations only that cluster touches. A field used by two clusters stays on the shell.
3. Declare a new type holding those fields.
4. Write the consumer-owned port the new type needs from the shell, at most 8 operations, in the new type's own file.
5. Add its constructor and call it from the root constructor.
6. The root keeps one field for the new type and one forwarding operation per entry point; delete each forwarder as callers are repointed.
7. Move the cluster's tests with it in the same change. Integration tests that drive the whole shell stay at the root.

A cluster may become a sub-module when it owns state no other cluster writes and everything it needs from the root fits behind consumer-owned ports (each at most 8 operations, composed). The root builds it and implements the ports with small adapter types; the sub-module never imports the root. Keep the sub-module to one concept, not one module per type, and no `utils`.

## 11. What not to do

- No interface per class (`specs/ARCHITECTURE.md §27`).
- No `utils`, `helpers` or `common` modules.
- No module per type.
- No provider-owned "full" interface that mirrors a concrete type.
- Do not split a function only to meet a line count.
- No forwarding wrapper that outlives the change that repoints its callers.
- No abstraction for a single implementation that crosses no module boundary.
