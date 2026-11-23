# TypeScript frameworks digest

Rules: RULES.md FW and API-01. Pick on team size and runtime, and check current usage (State of JS, npm trends) before choosing (TS-FW-01). Follow the repo's existing framework.

| Framework | Use for | Hexagonal fit |
|---|---|---|
| Fastify | performance, schema-first | plugins as composition root |
| NestJS | large teams, DI | needs decorator-free domain and application |
| Hono | edge, multi-runtime (Node, Bun, Deno, Workers) | thin handlers, web-standard `Request` |
| Express | legacy | add structure yourself |

## NestJS (TS-FW-02, TS-ARCH-06, 10, TS-FW-06)
- Modules, DI, decorators. Keep `domain/` and `application/` plain TS with zero `@nestjs/*` imports. Each adapter folder holds a `*.module.ts` that provides the port token: `{provide: ORDER_REPO, useClass: PgOrderRepo}`; `app.module.ts` plus `main.ts` are the composition root.
- Constructor injection with `@Inject(TOKEN)`. No `moduleRef.get`, no `@Global()` overuse, no field injection in domain or application.
- Entities and DTO decorators (`@Entity`, `@Column`, `@ApiProperty`) stay in adapters; map to domain objects.
- Footguns: circular providers (`forwardRef`), request-scoped providers cascading and slowing everything, `synchronize: true`, missing `app.enableShutdownHooks()`.
- Unit tests via `Test.createTestingModule`; validation with `ValidationPipe({whitelist: true, forbidNonWhitelisted: true})`. Jest is the default runner (the pack falls back to jest when no vitest or vite config exists).

## Fastify (TS-FW-03)
Encapsulated plugins; `fastify-plugin` only for shared decorators; JSON Schema per route; `app.inject()` for tests (no port); `fastify.register` is the composition root; set `bodyLimit`; pino logging is built in.

## Hono (TS-FW-04)
Web-standard `Request` and `Response`; `app.request()` for tests; `zValidator` for validation; keep handlers thin.

## Express (TS-FW-05)
v5 handles rejected promises in async handlers; v4 swallowed them. Still add a final error middleware, mind `app.use` ordering, never read `req.body` untyped (parse with zod), set `express.json({limit})`.

## Runtimes beyond Node (TS-OPS-08)
Bun: `bun.lock`, `bun test`, `bun run`; the bun variant of the pack covers install, build, test and audit. Deno: `deno.json`, permission flags, no gate variant (`testing.md`). Test, coverage and install commands differ per runtime, so choose deliberately.
