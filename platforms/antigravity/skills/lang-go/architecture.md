# Go architecture digest

Universal rules: `specs/ARCHITECTURE.md`, `specs/SOLID.md`. Go mapping: `specs/languages/go/ARCHITECTURE.md`. Rules: `specs/languages/go/RULES.md §5, §6, §16, §41-§44, §50`.

## Layers and imports (GO-ARCH-02, 03, 04)
- domain imports only the standard library and other domain packages; no `os`, `net/http`, `database/sql`, no `time.Now`, global `math/rand`, no `go` statements.
- Direction: domain <- application <- adapters <- compose <- cmd. Outbound adapters never import inbound ones. `internal/` makes outside reach a compile error.
- Package name equals the directory name, lowercase, no underscores. The `loops arch` resolver maps imports by module path plus directory. `loops arch` does not flag `database/sql` in domain: review for it.

## Ports and wiring (GO-ARCH-06, 07, RULES §5, §6)
- Interfaces are small and defined by the consumer next to the code that calls them; accept interfaces, return concrete types; no `Mock...` interface per type.
- Wire by hand in `compose`: constructors take interfaces and return structs; no globals, no `init()` registration, no service locator, no reflection DI (`fx`, `dig`). `wire` only for a huge graph.
- `cmd/<bin>/main.go` is wiring only: at most 80 lines, no string literal over 40 runes (errors from `loops arch`).

## Frameworks stay in adapters (GO-ARCH-12)
chi routers, gRPC servers, sqlc `Queries`, GORM models and generated code live in the adapter that owns them and never cross a port. Transport types (`*http.Request`, `*gin.Context`, `pgx.Tx`, SQL rows) never enter application or domain (RULES §44).

## Size limits (GO-ARCH-08)
Warnings from `loops arch` (`specs/SOLID.md §8`): type 40 methods, struct 25 fields, interface 8 methods, package 12000 lines or 60 files.

## DDD tactics (GO-ARCH-09)
Aggregates are struct types behind one package; reference other aggregates by ID (RULES §16); value objects are comparable value types; repositories are consumer-owned ports (`Get`/`Save` by aggregate); domain events are plain structs published through a port.

## Messaging and serverless (GO-ARCH-10, 11)
- Consumers are inbound adapters, publishers outbound; the application sees `Publisher.Publish(ctx, Event)`. Use the outbox pattern for DB plus broker atomicity; handlers are idempotent (RULES §46).
- Lambda or Cloud Functions: the handler is an inbound adapter; small static binary, no `init()` work, lazy clients.

## Style choice (GO-ARCH-01)
Modular monolith with hexagonal packages is the default; split services only along a team or scaling boundary; CQRS or event sourcing only where read and write models diverge. The actor model is goroutines owning state, communicating over channels (RULES §11).

## Enforcement (GO-ARCH-05)
`loops arch` checks import edges, third-party imports in core, `time.Now`, global `math/rand`, `go` statements in domain, `init()`, package-level mutable state and `cmd` wiring. A project without loops uses `go-arch-lint`, `arch-go` or golangci-lint `depguard`.

## Project layout of a new service
Use the tree in `SKILL.md`. Add package `AGENTS.md` for domain language and complexity notes (RULES §32, §42).
