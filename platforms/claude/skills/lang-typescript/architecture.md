# TypeScript architecture digest

Rules: RULES.md ARCH. Principles: `specs/ARCHITECTURE.md` (dependency rule §3, consumer-owned ports §6, package boundaries §15) and `specs/SOLID.md`.

## Layer map (TS-ARCH-02)
- `src/domain`: entities, value objects, domain events, errors. No `node:*`, no framework, no ORM, no `process.env`, no clock.
- `src/application`: use cases and the port interfaces they consume (`OrderRepo`, `Clock`, `IdGen`, `UnitOfWork`).
- `src/adapters/inbound/{http,queue,cli}` and `src/adapters/outbound/{postgres,http-client,kafka}`.
- `src/compose/container.ts`: the composition root. `src/main.ts`: reads `process.env`, starts the server, installs signal handlers.
- Dependencies point inward only. A thin CRUD service may stay layered; write the choice as an ADR (TS-ARCH-01).

## Ports and wiring (TS-ARCH-06, 08)
- A port is a small `interface` in `application/` next to its consumer. Interfaces are structural, so adapters satisfy them without `implements`.
- Branded ids: `type OrderId = string & {readonly __brand: 'OrderId'}`.
- Wire by constructor or factory argument in `compose/`. No service locators (`moduleRef.get`, global container lookups), no property injection in domain or application, no `@Global()` modules.
- Inject `Clock` and `IdGen`; the domain never calls `Date.now()`, `new Date()`, `Math.random()` or `crypto.randomUUID()` (TS-ARCH-07).

## What `loops arch` checks
- The regex parser reads `import x from`, `import type`, `export * from`, `import()` and `require()`; relative specifiers resolve against the importing file (and carry the extension under `nodenext`).
- It reports domain imports of `node:fs`, `node:net`, `node:http`, `node:https`, `node:child_process`, `node:crypto`, `fs`, `http`, `child_process`, `@nestjs/*`, `express`, `fastify`, `hono`, `pg`, `@prisma/client`, `typeorm`, and impure calls (`Date.now`, `new Date(`, `Math.random`, `crypto.randomUUID`, `process.env`, `fetch(`, `setTimeout(`, `setInterval(`).
- Path aliases (`tsconfig` `paths`, `@/app/z`) are not resolved: use relative imports across layers, or enforce with dependency-cruiser.

## dependency-cruiser (TS-ARCH-04, 05)
There is no package-private visibility in ES modules, so the language cannot stop `domain` importing `adapters`. When the repo commits `.dependency-cruiser.cjs`, `loops gate` runs `depcruise src --config .dependency-cruiser.cjs` as a required `arch` check. Rule shape:
```js
{name: 'domain-pure', severity: 'error', from: {path: '^src/domain'},
 to: {path: '^src/(application|adapters|compose)|^node_modules/(?!zod)'}}
```
Also forbid cycles (`circular: true`): they are legal at runtime but break ESM init order. eslint-plugin-boundaries or `no-restricted-imports` are acceptable alternatives.

## Size and shape (TS-ARCH-09, 10, 11)
- File under about 300 lines, function under about 40, complexity under 15, at most 4 parameters (`complexity`, `max-lines-per-function`, `max-params`).
- Decorated ORM or Swagger entities (`@Entity`, `@Column`, `@ApiProperty`) live in adapters; map to plain domain objects at the boundary.
- Barrel `index.ts` files only at a module's public API, never across layers.

## Styles in TS
Hexagonal is the default for services with business rules. Layered/NestJS becomes hexagonal when services depend on port tokens. DDD tactical: classes with private state, branded value objects, aggregates returning events, framework-free. CQRS and event sourcing only with real read/write asymmetry. Event-driven handlers (BullMQ, Kafka, NATS, SQS) are inbound adapters. Start as a modular monolith. Actors are not idiomatic: use queues and `worker_threads`.
