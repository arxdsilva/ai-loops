---
name: hexagonal
description: Use when designing or reviewing package or module boundaries in any language pack - ports, adapters, dependency direction, cross-layer interfaces, wiring at the composition root, or SOLID size limits and splitting a god type.
---

# Hexagonal architecture

## Load

- `specs/ARCHITECTURE.md §3` dependency rule, `§6` consumer-owned ports, `§15` package boundaries, `§27` smells, `§29` compliance.
- `specs/SOLID.md §6-7` interfaces, `§8` limits, `§9` package state, `§10` splitting.
- Your language's layout tree and its domain bans (`arch.forbidden_in_domain`, `impure_calls`): `skills/lang-<id>/architecture.md` (load `loops:lang-<id>`).

## Boundary test

For every dependency ask: does the inner model need to know this technology exists? If not, keep it outside.

Forbidden in `domain`: HTTP/gRPC, SQL drivers and ORMs, queue clients, OS and filesystem access, generated transport or persistence types, YAML/JSON parsers, CLI frameworks, config loaders, vendor SDKs, behaviour-changing logging.

## Ports

- Create a port only at a real boundary. Consumer-owned, narrow, at most 8 methods. The language's form of a port (interface, protocol, trait, behaviour, abstract class) is in its language skill.
- Consumers depend on a small interface for the behaviour they call, never the concrete type; construct concrete types only at the composition root.
- Do not create an interface merely because a concrete type exists, but always create one at an adapter/application or application/domain boundary.

Go example:

```go
type OrderWriter interface { Save(ctx context.Context, o order.Order) error }
```

## Cross-package design

Before adding a shared interface or type: search for an existing one; find the consumer; define the minimum behaviour; keep technology types out; document ownership and ordering assumptions.

## Domain vs application

- Domain: `PriceOrder`, `ApplyDiscount`, `AllocateStock`, `TransferFunds`.
- Application: `PlaceOrder`, `CancelSubscription`, `ImportCatalog`, `RetryFailedPayments`.

## Transport and persistence rules

- Handlers receive requests, build typed commands, call an inbound port, map the result and errors to the transport. They hold no business rules.
- Domain types are not the storage schema; repositories translate (`specs/ARCHITECTURE.md §18`).
- Reads return read models, never mutable entities or rows (`§11`).

## Adding to a type, interface or package

When a change adds a method or field to an existing type, declares an interface, adds module-level mutable state or import-time registration (Go: a package var or `init()`), or creates or splits a package or module:

1. Read `specs/SOLID.md §8` before adding to a type or package already over a limit.
2. Put new state and behaviour on its own type with a consumer-owned interface and a constructor called from the composition root.
3. Register tables at the composition root, never at import time (Go: `init()`); a test hook is a field, not a global.
4. Run `loops arch` (and the pack's native arch check if it has one: `loops lang show <id>`); fix errors, and treat warnings on code you touched as a prompt to split.

Skip this for a task that touches no type, interface, module-level state or layout.

## Architecture review checklist

Reject or request redesign when:

- domain imports adapter or platform code;
- a "service" owns unrelated concerns;
- an interface has broad CRUD merely for abstraction;
- mutable stores or entities cross the adapter boundary;
- a package becomes a dumping ground;
- a change adds import-time registration (Go `init()`) or grows a type already over its limit instead of splitting it;
- shared primitives are duplicated;
- technology dictates domain modelling;
- a new abstraction adds indirection without boundary value.

## ADR trigger

Write or update an ADR (`templates/ADR.md`) for changes to layers, state ownership, canonical primitives, the command protocol, the persistence or wire schema strategy, the concurrency model or the consistency model (`specs/ARCHITECTURE.md §26`).
