# TypeScript / JavaScript (server-side): backend practices and loops gap analysis

| | |
|---|---|
| Pack id | `typescript` |
| Current stable / LTS | Node 26 "Lithium" (released 2026-05-05, Active LTS since 2026-10-28) / Node 24 "Krypton" (Maintenance since 2026-10-20, EOL 2028-04-30) and Node 22 "Jod" (Maintenance until 2027-04-30); TypeScript 7.0.2 is latest on npm, 5.8+ needed for Node type stripping; typescript-eslint supports TS >=4.8.4 <6.1.0 only (as of 2026-11) |
| Dominant runtimes / frameworks | Node.js (default), Bun, Deno; NestJS, Fastify, Express, Hono |
| As of | 2026-11 |
| Researcher | `B1-typescript` |
| Verifier | (filled in by B2) |

> **Summary:** A strong TS backend team runs Node LTS pinned in `engines` and `.nvmrc`, one package manager with a committed lockfile and `npm ci`/`pnpm install --frozen-lockfile` in CI, `tsc --noEmit` under `strict` plus `noUncheckedIndexedAccess`, ESLint flat config with typescript-eslint type-checked rules (or Biome) and Prettier, vitest with v8 coverage to lcov, and zod/valibot at every edge.
> There is no data-race detector: the honest policy is that JS races are logical (interleaving across `await`, or `SharedArrayBuffer`), so loops enforces shuffled, repeated, concurrent test runs instead of a flag.
> Biggest gaps: loops has no language detection, no `package.json`-driven gate (`build`/`tsc`/`eslint`/`vitest`), no lcov parser for new-code coverage, no `npm audit`/OSV step, and no repo-management rules (lockfile, toolchain, supply chain, provenance).

Rules follow [TAXONOMY.md](TAXONOMY.md), using the row format `ID | Rule | Why | Sev | Source | loops today | Enforce`. Gaps are judged against [LOOPS_BASELINE.md](LOOPS_BASELINE.md). "Runtime" below means Node unless stated; Bun and Deno deltas are called out.

## 1. REPO: repository and project management

Canonical tree for a single-package service (workspace layout in REPO-02):

```
.
├── package.json            # "type": "module", engines, packageManager, scripts
├── package-lock.json | pnpm-lock.yaml | yarn.lock | bun.lock   # exactly one, committed
├── .nvmrc  (or .node-version / mise.toml)
├── .npmrc                  # save-exact, ignore-scripts, min-release-age
├── tsconfig.json  tsconfig.build.json
├── eslint.config.mjs | biome.json   .prettierrc.json   .editorconfig
├── vitest.config.ts
├── .dependency-cruiser.cjs
├── .github/ (workflows, dependabot.yml | renovate.json, CODEOWNERS, PULL_REQUEST_TEMPLATE.md)
├── src/        # domain/ application/ adapters/ main.ts (composition root)
├── test/       # integration + e2e; unit tests colocated as src/**/*.test.ts
├── migrations/
├── Dockerfile  .dockerignore
└── dist/ coverage/ node_modules/   # git-ignored build output
```

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `TS-REPO-01` | Put sources in `src/`, tests colocated as `*.test.ts` (unit) or under `test/` (integration/e2e), emit to `dist/`, and set `"files": ["dist"]` or never publish the service at all [REPO-01] | `package.json` has no standard layout, so a convention keeps globs and Docker copies stable | SHOULD | [Node packages](https://nodejs.org/api/packages.html), [TypeScript tsconfig](https://www.typescriptlang.org/tsconfig/) (rootDir/outDir) | missing | review |
| `TS-REPO-02` | Use native workspaces (`"workspaces"` in npm/yarn/bun, `pnpm-workspace.yaml` in pnpm) for multi-package repos, with TS project references (`composite`, `tsc -b`) between packages [REPO-02] | One lockfile, hoisted installs and incremental builds | SHOULD | [npm workspaces](https://docs.npmjs.com/cli/v11/using-npm/workspaces), [pnpm workspaces](https://pnpm.io/workspaces) | missing | review |
| `TS-REPO-03` | Add Turborepo or Nx only when task caching/graph ordering across 3+ packages is needed; otherwise plain workspace scripts [REPO-02] | Task runners add config and cache-poisoning surface | AVOID | [Turborepo docs](https://turborepo.com/docs), [Nx docs](https://nx.dev/docs) | missing | review |
| `TS-REPO-04` | Bootstrap with `npm init -y` + `tsc --init`, or `npm create`/`pnpm create` of the framework (`nest new`, `npm create hono@latest`, `npm init fastify`); day one has lockfile, tsconfig, lint, test, CI, `.gitignore`, `engines` [REPO-03] | A repo without a gate on commit one never gets one | MUST | [NestJS first steps](https://docs.nestjs.com/first-steps), [Hono getting started](https://hono.dev/docs/), [Fastify getting started](https://fastify.dev/docs/latest/Guides/Getting-Started/) | missing | review |
| `TS-REPO-05` | Pick one package manager per repo, declare it in `"packageManager": "pnpm@x.y.z"`, commit exactly one lockfile, and install in CI with `npm ci` / `pnpm install --frozen-lockfile` / `yarn install --immutable` / `bun install --frozen-lockfile` [REPO-04] | Two lockfiles drift; `npm install` mutates the lock and hides drift | MUST | [npm ci](https://docs.npmjs.com/cli/v11/commands/npm-ci), [pnpm install](https://pnpm.io/cli/install) | missing | gate |
| `TS-REPO-06` | Commit the lockfile for applications and for libraries' own CI; pin direct deps exactly (`save-exact=true` in `.npmrc`) or rely on the lock plus Renovate [REPO-04] | Reproducible installs; the lock pins transitive deps | MUST | [Node security best practices](https://nodejs.org/en/learn/getting-started/security-best-practices) | missing | gate |
| `TS-REPO-07` | Automate updates with Renovate or Dependabot, grouped, with a release cooldown (`minimumReleaseAge` in Renovate, pnpm `minimumReleaseAge`, npm `min-release-age=1`, v11.10+) [REPO-04] | Fresh malicious releases are usually yanked within hours | SHOULD | [Renovate docs](https://docs.renovatebot.com/), [pnpm supply chain](https://pnpm.io/supply-chain-security), [Node security](https://nodejs.org/en/learn/getting-started/security-best-practices) | missing | review |
| `TS-REPO-08` | Remove unused deps and exports with `knip` in CI [REPO-04] | Unused deps widen the supply-chain surface | SHOULD | [knip](https://knip.dev/) | missing | gate |
| `TS-REPO-09` | For published packages use SemVer, Changesets for changelogs/versions, `npm publish --provenance` (or trusted publishing, no token) from GitHub Actions with `id-token: write`, and check contents with `npm publish --dry-run` plus a `files` allowlist [REPO-05] | Provenance links the tarball to the source commit | SHOULD | [npm provenance](https://docs.npmjs.com/generating-provenance-statements), [Changesets](https://github.com/changesets/changesets) | missing | review |
| `TS-REPO-10` | Pin the toolchain three ways: `"engines": {"node": ">=24 <27"}` in `package.json` (set `engine-strict=true` in `.npmrc`), `.nvmrc` (`24`) or `.node-version`, and the Docker base tag; use `volta`/`mise`/`fnm` as the manager [REPO-06] | `engines` alone only warns unless strict; CI and prod must match dev | MUST | [npm package.json engines](https://docs.npmjs.com/cli/v11/configuring-npm/package-json#engines), [Node releases](https://nodejs.org/en/about/previous-releases) | missing | gate |
| `TS-REPO-11` | Ignore `node_modules/`, `dist/`, `coverage/`, `*.tsbuildinfo`, `.turbo/`, `.env*` (keep `.env.example`), and add `.editorconfig`, `.gitattributes` (`* text=auto eol=lf`), and a pre-commit runner (lefthook or husky + lint-staged) [REPO-07] | Keeps artifacts and secrets out of git | MUST | [GitHub gitignore Node template](https://github.com/github/gitignore), [EditorConfig](https://editorconfig.org/) | missing | hook |
| `TS-REPO-12` | CI stages in order: install frozen, format check, lint, `tsc --noEmit`, build, test with coverage, `npm audit`/OSV, package; cache the package-manager store (not `node_modules`) keyed on the lockfile hash; matrix over supported Node majors [REPO-08] | Fail fast and cheap first | MUST | [actions/setup-node caching](https://github.com/actions/setup-node#caching-global-packages-data) | missing | gate |
| `TS-REPO-13` | Use Conventional Commits and let Changesets/release-please generate changelogs and tags [REPO-09] | Same flow as loops releases | SHOULD | [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) | covered (AGENTS.md releases) | review |

## 2. TOOL: toolchain

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `TS-TOOL-01` | Run production on an Active or Maintenance LTS only. As of 2026-11: Node 26 is Active LTS (released 2026-05-05, LTS since 2026-10-28); 24 (Maintenance since 2026-10-20) and 22 are Maintenance LTS; 20 and 25 are EOL; 27 is alpha. From Node 27 the cadence becomes annual [TOOL-01] | EOL lines get no security fixes | MUST | [Node releases](https://nodejs.org/en/about/previous-releases) (as of 2026-11) | missing | gate |
| `TS-TOOL-02` | Format with Prettier (`prettier --check .`; write with `--write`), config `.prettierrc.json`, and `.prettierignore`; or Biome's formatter if Biome is the linter. Never run both on the same files [TOOL-02] | One formatter ends style debates; two fight | MUST | [Prettier CLI](https://prettier.io/docs/cli), [Biome CLI](https://biomejs.dev/reference/cli/) | missing | gate |
| `TS-TOOL-03` | Lint with ESLint flat config (`eslint.config.mjs`, `defineConfig`) plus `typescript-eslint` `strictTypeChecked` with `parserOptions.projectService: true`, run with `--max-warnings=0`; typescript-eslint supports TypeScript `>=4.8.4 <6.1.0` and does not list TS 7 (the native port, 7.0.2 latest on npm), so pin `typescript` to a supported 5.x/6.0 for linting or run TS 7 only for `tsc` until support lands [TOOL-03] | Type-aware rules catch floating promises and unsafe `any` | MUST | [typescript-eslint typed linting](https://typescript-eslint.io/getting-started/typed-linting/), [typescript-eslint dependency versions](https://typescript-eslint.io/users/dependency-versions/), [ESLint config files](https://eslint.org/docs/latest/use/configure/configuration-files) | missing | gate |
| `TS-TOOL-04` | Acceptable alternative: Biome (`biome ci --reporter=github`) for speed; the CLI docs do not document type-aware rules, so treat it as unverified whether Biome replaces typescript-eslint rules such as `no-floating-promises`; keep `tsc` and a floating-promise check somewhere [TOOL-03] | Biome is one fast binary; its type-aware coverage is unverified | SHOULD | [Biome CLI](https://biomejs.dev/reference/cli/), [Biome linter](https://biomejs.dev/linter/) | missing | gate |
| `TS-TOOL-05` | Enable at least `@typescript-eslint/no-floating-promises`, `no-misused-promises`, `await-thenable`, `no-explicit-any`, `consistent-type-imports`, and `eqeqeq` [TOOL-03] | These are the async and typing footguns | MUST | [no-floating-promises](https://typescript-eslint.io/rules/no-floating-promises/) | missing | gate |
| `TS-TOOL-06` | Typecheck with `tsc --noEmit` (or `tsc -b` in project references) as its own gate step even if the bundler/test runner strips types [TOOL-04] | esbuild, swc, vitest and Node strip types without checking them | MUST | [Node TypeScript](https://nodejs.org/api/typescript.html) | missing | gate |
| `TS-TOOL-07` | Set `strict: true`, `noUncheckedIndexedAccess: true`, `exactOptionalPropertyTypes: true`, `noImplicitOverride: true`, `verbatimModuleSyntax: true`, `noFallthroughCasesInSwitch: true`, `skipLibCheck: true` [TOOL-04] | `strict` alone leaves `arr[i]` typed as `T`, not `T \| undefined` | MUST | [tsconfig reference](https://www.typescriptlang.org/tsconfig/) | missing | gate |
| `TS-TOOL-08` | Audit with `npm audit --audit-level=high --omit=dev` (pnpm: `pnpm audit --prod`), plus OSV-Scanner `osv-scanner scan source -r .` (v2 syntax); generate an SBOM with `npm sbom --sbom-format cyclonedx` [TOOL-05] | Advisory DBs differ; OSV reads lockfiles directly | MUST | [npm audit](https://docs.npmjs.com/cli/v11/commands/npm-audit), [OSV-Scanner](https://google.github.io/osv-scanner/), [npm sbom](https://docs.npmjs.com/cli/v11/commands/npm-sbom) | missing | gate |
| `TS-TOOL-09` | Use the TypeScript language server (tsserver) via the editor with the workspace TypeScript version, and make format-on-save use only the repo's formatter [TOOL-06] | Editor TS version mismatch gives phantom errors | SHOULD | [VS Code TypeScript](https://code.visualstudio.com/docs/languages/typescript) | missing | review |
| `TS-TOOL-10` | Node can run `.ts` directly via type stripping (stable in v25.2 and v24.12; default-on since v22.18/v23.6): use it for scripts and tests only if code is `erasableSyntaxOnly` (no `enum`, runtime `namespace`, parameter properties, decorators) and imports carry `.ts` extensions; keep a real build for services [TOOL-04] | Stripping is not type-checking; unsupported syntax throws `ERR_UNSUPPORTED_TYPESCRIPT_SYNTAX` | SHOULD | [Node TypeScript](https://nodejs.org/api/typescript.html) | missing | gate |

## 3. ARCH: architecture

How each style looks in TS:
- **Hexagonal/clean/onion:** folder-per-layer plus an enforced import rule (ARCH-04). TS interfaces are structural, so ports are plain `interface`/`type` declarations that adapters satisfy without `implements`.
- **Layered/MVC:** NestJS controller/service/repository is layered by default; it becomes hexagonal when services depend on port tokens, not concrete repositories.
- **DDD tactical:** classes with private state, branded value-object types, aggregates as classes returning domain events; keep them framework-free.
- **CQRS/event sourcing:** `@nestjs/cqrs` or hand-rolled command/query handlers; choose only with a real read/write asymmetry.
- **Event-driven:** BullMQ, Kafka (kafkajs/confluent), NATS, SQS; handlers are inbound adapters.
- **Modular monolith vs microservices:** start as modular monolith (one deployable, module boundaries enforced by dependency-cruiser or Nx tags), split on team or scaling pressure.
- **Serverless:** Hono, Fastify (`@fastify/aws-lambda`) on Lambda/Workers; cold start and per-invocation DI cost favour manual wiring.
- **Actor model:** not idiomatic; use queues and `worker_threads` for isolation.

ARCH-02 tree (framework-agnostic; NestJS variant under it):

```
src/
├── domain/                 # pure: entities, value objects, domain events, errors. No node:*, no framework
│   └── order/{order.ts,money.ts,errors.ts}
├── application/            # use cases + consumer-owned ports
│   └── order/{place-order.ts,ports.ts}   # ports.ts: interface OrderRepo, Clock, IdGen
├── adapters/
│   ├── inbound/{http/(routes.ts,schemas.ts),queue/,cli/}
│   └── outbound/{postgres/,http-client/,kafka/}
├── compose/container.ts    # composition root: new X(deps) wiring
└── main.ts                 # the only file that reads process.env, starts the server, installs signal handlers
```

NestJS variant: `domain/` and `application/` stay plain TS with zero `@nestjs/*` imports; each adapter folder holds a `*.module.ts` that provides the port token (`{provide: ORDER_REPO, useClass: PgOrderRepo}`); `app.module.ts` plus `main.ts` form the composition root.

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `TS-ARCH-01` | Choose hexagonal for services with real business rules and several IO edges; a thin CRUD service may stay layered, but write the choice as an ADR [ARCH-01] | Avoids ceremony where there is no domain | SHOULD | [Hexagonal architecture (Cockburn)](https://alistair.cockburn.us/hexagonal-architecture/) | covered (`HEXAGONAL_ARCHITECTURE` §1-§30) | review |
| `TS-ARCH-02` | Map loops layers to `src/domain`, `src/application`, `src/adapters/{inbound,outbound}`, `src/compose`, `src/main.ts` as in the tree above [ARCH-02] | Gives the arch checker fixed prefixes | MUST | [dependency-cruiser rules](https://github.com/sverweij/dependency-cruiser/blob/main/doc/rules-reference.md), [HEXAGONAL_ARCHITECTURE §4](../../specs/HEXAGONAL_ARCHITECTURE.md) | go-only (§4 `internal/`) | arch |
| `TS-ARCH-03` | Import statements are `import x from './a.js'`, `import {y} from '@/app/z'`, `import type {T} from '...'`, `export * from`, `import('...')`, and CJS `require('...')`; relative specifiers resolve against the importing file and must carry the extension under `nodenext` [ARCH-03] | The arch parser needs every form including dynamic and type-only | MUST | [Node ESM](https://nodejs.org/api/esm.html), [tsconfig module](https://www.typescriptlang.org/tsconfig/#module) | missing | arch |
| `TS-ARCH-04` | There is no package-private visibility: ES modules export or not; `private` is TS-only and `#field` is runtime-private. Enforce layers with dependency-cruiser (`forbidden` rules by path regex, `severity: error`) or eslint-plugin-boundaries/`no-restricted-imports`; cycles are legal at runtime but break ESM init order, so forbid them (`circular: true`) [ARCH-03, ARCH-04] | The language will not stop `domain` importing `adapters` | MUST | [dependency-cruiser rules](https://github.com/sverweij/dependency-cruiser/blob/main/doc/rules-reference.md), [eslint-plugin-boundaries](https://github.com/javierbrea/eslint-plugin-boundaries) | missing | arch |
| `TS-ARCH-05` | dependency-cruiser rule shape: `{name:"domain-pure", severity:"error", from:{path:"^src/domain"}, to:{path:"^src/(application\|adapters\|compose)\|^node_modules/(?!zod)"}}`; run `depcruise src --config .dependency-cruiser.cjs` [ARCH-04] | Concrete, mechanical, exit-code driven | MUST | [dependency-cruiser](https://github.com/sverweij/dependency-cruiser) | missing | arch |
| `TS-ARCH-06` | Wire dependencies by constructor or factory argument in `compose/`; with NestJS use constructor injection with `@Inject(TOKEN)` for ports. Avoid service locators (`moduleRef.get`, global container lookups), property/field injection in domain or application, and `@Global()` modules [ARCH-05] | Hidden dependencies break unit tests and the dependency rule | MUST | [NestJS providers](https://docs.nestjs.com/providers), [NestJS custom providers](https://docs.nestjs.com/fundamentals/custom-providers) | covered (`HEXAGONAL` DI section) | arch |
| `TS-ARCH-07` | Keep `domain/` free of `node:fs`, `node:net`, `node:http`, `node:child_process`, `process.env`, `Date.now()`, `new Date()`, `Math.random()`, `crypto.randomUUID()`, `fetch`, framework decorators, ORM entities; inject `Clock` and `IdGen` ports [ARCH-06] | Determinism and testability | MUST | [Node process.env](https://nodejs.org/api/process.html#processenv), [HEXAGONAL_ARCHITECTURE](../../specs/HEXAGONAL_ARCHITECTURE.md), [`skills/consistency`](../../skills/consistency/SKILL.md) | go-only (forbidden `os`, `time.Now`) | arch |
| `TS-ARCH-08` | Define ports as small `interface`s in `application/` next to the consumer; adapters satisfy them structurally; use `type` branded ids (`type OrderId = string & {readonly __brand: 'OrderId'}`) [ARCH-07] | Structural typing makes tiny ports free | SHOULD | [TS handbook interfaces](https://www.typescriptlang.org/docs/handbook/2/objects.html) | go-only (consumer-owned interface text) | review |
| `TS-ARCH-09` | Size heuristics: file under ~300 lines, function under ~40 lines and cognitive complexity under 15 (`complexity`, `max-lines-per-function`, `max-params: 4` in ESLint), classes with one reason to change [ARCH-08] | Mirrors loops SOLID warnings | SHOULD | [ESLint max-params](https://eslint.org/docs/latest/rules/max-params), [ESLint complexity](https://eslint.org/docs/latest/rules/complexity) | go-only (`SOLID_GO` §8 in Go units) | arch |
| `TS-ARCH-10` | Decorator-heavy NestJS entities (`@Entity`, `@Column`, `@ApiProperty`) belong in outbound/inbound adapters; map to plain domain objects at the boundary [ARCH-02] | Decorators couple the domain to TypeORM/Swagger | MUST | [NestJS techniques: database](https://docs.nestjs.com/techniques/database) | missing | arch |
| `TS-ARCH-11` | Barrel files (`index.ts` re-exports) are allowed only at module public-API level, never across layers, and not inside hot modules [ARCH-03] | Barrels create accidental cycles and defeat tree-shaking and test startup | AVOID | [Vite performance: avoid barrel files](https://vite.dev/guide/performance#avoid-barrel-files) | missing | arch |

## 4. IDIOM: what to do and what to avoid

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `TS-IDIOM-01` | Throw `Error` subclasses only (never strings or plain objects), set `cause` (`new Error('x', {cause: e})`), model expected failures as a discriminated-union `Result` in domain/application, and map to HTTP at the edge [IDIOM-01] | `catch (e)` is `unknown`; context is lost without `cause` | MUST | [MDN Error cause](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Error/cause), [typescript-eslint only-throw-error](https://typescript-eslint.io/rules/only-throw-error/) | covered (`GO_BEST` §8-9 neutral core) | gate |
| `TS-IDIOM-02` | Never `catch {}` silently; narrow with `instanceof` or a type guard before use (`useUnknownInCatchVariables` is on under `strict`) [IDIOM-01] | Swallowed errors hide faults | MUST | [tsconfig useUnknownInCatchVariables](https://www.typescriptlang.org/tsconfig/#useUnknownInCatchVariables) | covered (`GO_BEST` §8) | gate |
| `TS-IDIOM-03` | Handle absence with `T \| undefined` and strict null checks; use `??` and `?.` rather than `\|\|` for defaults; avoid `null` in new domain types, and treat `null` only as a wire/DB value at the adapter [IDIOM-02] | `\|\|` swallows `0` and `''` | MUST | [TS handbook narrowing](https://www.typescriptlang.org/docs/handbook/2/narrowing.html) | missing | gate |
| `TS-IDIOM-04` | Avoid non-null assertions (`!`) and `as` casts on external data; validate with zod/valibot instead [IDIOM-02, IDIOM-04] | Casts are unchecked lies | AVOID | [typescript-eslint no-non-null-assertion](https://typescript-eslint.io/rules/no-non-null-assertion/) | missing | gate |
| `TS-IDIOM-05` | Prefer `readonly` fields, `ReadonlyArray`, `as const`, `Object.freeze` at boundaries, and immutable updates (`structuredClone`, spread); mutating shared module state is a bug [IDIOM-03] | Shared mutable singletons are the JS version of a data race | SHOULD | [TS handbook readonly](https://www.typescriptlang.org/docs/handbook/2/objects.html#readonly-properties) | covered (`HEXAGONAL` single owner of state) | review |
| `TS-IDIOM-06` | Model states as discriminated unions with an exhaustive `switch` ending in a `never` assertion; avoid `enum` (runtime code, breaks type stripping): use string-literal unions or `as const` objects [IDIOM-04] | Exhaustiveness is checked by the compiler | MUST | [TS handbook narrowing: exhaustiveness](https://www.typescriptlang.org/docs/handbook/2/narrowing.html#exhaustiveness-checking), [Node TypeScript](https://nodejs.org/api/typescript.html) | covered (state machines §) | gate |
| `TS-IDIOM-07` | Ban `any`; use `unknown` at untrusted boundaries and narrow it; `@ts-expect-error` with a reason beats `@ts-ignore`, which is banned [IDIOM-04] | `any` turns the checker off transitively | MUST | [typescript-eslint ban-ts-comment](https://typescript-eslint.io/rules/ban-ts-comment/), [no-explicit-any](https://typescript-eslint.io/rules/no-explicit-any/) | missing | gate |
| `TS-IDIOM-08` | Release resources with `using`/`await using` (TS 5.2, Node 24+ for `Symbol.dispose` on some types) or `try/finally`; always close DB clients, streams and servers, and use `stream.pipeline` (promises) instead of `.pipe` [IDIOM-05] | Unclosed handles keep the process alive and leak | MUST | [TS 5.2 using](https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-2.html), [Node stream.pipeline](https://nodejs.org/api/stream.html#streampipelinestreams-callback) | covered (`GO_BEST` defer/close) | review |
| `TS-IDIOM-09` | Use `===`/`!==`; compare objects structurally with `node:util` `isDeepStrictEqual`, never `==` or `JSON.stringify`; use `Map`/`Set` for non-string keys [IDIOM-06] | Reference equality surprises | MUST | [ESLint eqeqeq](https://eslint.org/docs/latest/rules/eqeqeq) | missing | gate |
| `TS-IDIOM-10` | Store instants as UTC ISO-8601 or epoch ms, never local time strings; use `Temporal` (Node flag-gated/polyfill as of 2026-11, check status) or `date-fns`/`luxon`, and never do money arithmetic in `number` (use integer minor units or `decimal.js`/`bigint`) [IDIOM-06] | `Date` is mutable, `0.1+0.2!==0.3` | MUST | [MDN Temporal](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Temporal), [MDN BigInt](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/BigInt) | covered (`skills/consistency` clock) | review |
| `TS-IDIOM-11` | Follow the Google TypeScript Style Guide naming: `UpperCamelCase` types, `lowerCamelCase` values, `CONSTANT_CASE` constants, no `I` prefix, named exports over default exports; document public API with TSDoc [IDIOM-07] | Default exports defeat rename and grep | SHOULD | [Google TS style guide](https://google.github.io/styleguide/tsguide.html), [TSDoc](https://tsdoc.org/) | go-only (`GO_BEST` §26 doc comments) | gate |
| `TS-IDIOM-12` | Decorators, `Proxy`, `eval`, `new Function` and monkey-patching built-ins are allowed only inside framework adapters (NestJS decorators); never in domain code [IDIOM-08] | Magic hides dependencies and breaks type stripping | AVOID | [typescript-eslint no-implied-eval](https://typescript-eslint.io/rules/no-implied-eval/) | missing | arch |
| `TS-IDIOM-13` | Footguns: `parseInt` without radix on user input, `Array.sort()` without comparator (string sort), `for..in` over arrays, `JSON.parse` result typed as `any`, `Number` precision above 2^53, `forEach(async ...)` (does not await) [IDIOM-09] | Each is a known production bug | MUST | [MDN Array.sort](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Array/sort), [typescript-eslint no-misused-promises](https://typescript-eslint.io/rules/no-misused-promises/) | missing | gate |
| `TS-IDIOM-14` | Use ESM: `"type": "module"`, `"module": "nodenext"`, explicit file extensions in relative imports, `import.meta.dirname` instead of `__dirname`; dual CJS/ESM publishing only for libraries, via the `exports` map (Node supports `require(esm)` from 22.12) [IDIOM-04] | Mixed module systems cause `ERR_REQUIRE_ESM` and double-instance bugs | SHOULD | [Node packages exports](https://nodejs.org/api/packages.html), [Node ESM](https://nodejs.org/api/esm.html) | missing | gate |

## 5. CONC: concurrency and the `-race` equivalent

Loops race policy: `mode = none`. Substitute (enforced instead): run the suite with shuffled files and tests plus concurrent execution and a repeat count, `vitest run --sequence.shuffle.files --sequence.shuffle.tests --sequence.seed=<n> --sequence.concurrent` once per gate, and `--repeats=3` (or a 3x loop) for changed test files. Honest basis: JS has no memory-model data races in single-threaded code; races are logical interleavings across `await`, plus real races only through `SharedArrayBuffer`/`Atomics` in `worker_threads`, which no detector covers. Print the seed so failures replay.

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `TS-CONC-01` | Know the model: one JS thread runs an event loop; I/O is async via libuv; CPU work uses `worker_threads`; `cluster`/multiple processes scale across cores [CONC-01] | Choice of scaling unit follows from this | MUST | [Node event loop](https://nodejs.org/en/learn/asynchronous-work/event-loop-timers-and-nexttick), [worker_threads](https://nodejs.org/api/worker_threads.html) | missing | review |
| `TS-CONC-02` | Treat module-level mutable state, in-memory caches, and check-then-act across an `await` as shared state: do not read, `await`, then write back; use a per-key mutex/queue, DB constraints, or atomic ops [CONC-02] | Another request interleaves at every `await` | MUST | [Node: Don't block the event loop](https://nodejs.org/en/learn/asynchronous-work/dont-block-the-event-loop) | covered (`HEXAGONAL` single owner of state) | review |
| `TS-CONC-03` | Between workers use `postMessage` with structured clone; use `SharedArrayBuffer` plus `Atomics` only with a documented protocol; never share mutable objects [CONC-02] | The only true memory races in JS live here | SHOULD | [worker_threads](https://nodejs.org/api/worker_threads.html), [MDN Atomics](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Atomics) | missing | review |
| `TS-CONC-04` | No JS race detector exists: record `race.mode = none` and enforce shuffled + concurrent + repeated test runs (vitest `--sequence.shuffle.files --sequence.shuffle.tests --sequence.seed=N --sequence.concurrent`, `--repeats=N`; node:test `--test-randomize --test-concurrency` (`--test-randomize` needs Node 26.1+, early development, absent on Node 24/22); jest 30 `--randomize`) [CONC-03, CONC-04] | Surfaces order and shared-state dependence, the realistic race class | MUST | [Vitest CLI](https://vitest.dev/guide/cli.html), [node:test](https://nodejs.org/api/test.html) | go-only (`race-guard`) | gate |
| `TS-CONC-05` | Do not use test retries (`--retry.count`, default 0) to hide flaky tests in the gate; keep retry count 0 [CONC-04] | Retries mask races | AVOID | [Vitest CLI](https://vitest.dev/guide/cli.html) | missing | gate |
| `TS-CONC-06` | Never block the event loop: no `*Sync` fs/crypto calls in request paths, no big `JSON.parse`/`JSON.stringify`, no catastrophic-backtracking regex on user input; offload CPU to `worker_threads` or a queue (e.g., piscina) [CONC-05] | One blocked tick stalls every request | MUST | [Don't block the event loop](https://nodejs.org/en/learn/asynchronous-work/dont-block-the-event-loop) | missing | gate |
| `TS-CONC-07` | Every promise is awaited, returned, or explicitly `void`-ed with a catch (`no-floating-promises`); `Promise.all` fails fast, use `Promise.allSettled` when partial results are fine; cap parallelism (p-limit) [CONC-05] | Node's default `--unhandled-rejections=throw` crashes the process on an unhandled rejection | MUST | [Node process unhandledRejection](https://nodejs.org/api/process.html#event-unhandledrejection), [no-floating-promises](https://typescript-eslint.io/rules/no-floating-promises/) | covered (`GO_BEST` §10-12 neutral) | gate |
| `TS-CONC-08` | Propagate cancellation with `AbortSignal` (`AbortSignal.timeout(ms)`, `AbortSignal.any([...])`); pass `signal` to `fetch`, `fs.promises`, `events.once`, streams, DB drivers that accept it; abort on client disconnect and shutdown [CONC-05] | JS has no implicit context; cancellation is opt-in | MUST | [Node AbortSignal](https://nodejs.org/api/globals.html#class-abortsignal), [MDN AbortSignal](https://developer.mozilla.org/en-US/docs/Web/API/AbortSignal) | go-only (`GO_BEST` §10 context) | review |
| `TS-CONC-09` | Set server timeouts (`headersTimeout`, `requestTimeout`, `keepAliveTimeout`) and client timeouts on every outbound call [CONC-05] | Slowloris and hung upstreams | MUST | [Node security best practices](https://nodejs.org/en/learn/getting-started/security-best-practices) | covered (resilience in skills) | review |
| `TS-CONC-10` | Background work uses a durable queue (BullMQ, pg-boss, SQS), idempotent handlers, and shutdown that stops accepting jobs, drains in-flight ones, then exits [CONC-06] | In-process timers die with the pod | SHOULD | [BullMQ](https://docs.bullmq.io/), [`skills/jobs-and-state-machines`](../../skills/jobs-and-state-machines/SKILL.md) | covered (`skills/jobs-and-state-machines`) | review |

## 6. TEST: testing

Coverage recipe: vitest with the v8 provider, `npx --no-install vitest run --coverage --coverage.provider=v8 --coverage.reporter=lcov --coverage.reportsDirectory=coverage`, output `coverage/lcov.info` (lcov, line + branch). Passing `--coverage.reporter` replaces the default reporter list. New-code coverage: loops parses `coverage/lcov.info` (`SF:`, `DA:line,hits`, `BRDA:`) and intersects with `git diff -U0 <base>`; vitest's `coverage.changed` only limits which files are measured, it does not compute a percentage.

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `TS-TEST-01` | Default to vitest (ESM/TS native, fast, Jest-compatible API); jest 30 is acceptable for existing suites; `node:test` for zero-dependency libraries; Bun users use `bun test`, Deno users `deno test`; end-to-end via supertest or `app.inject()` (Fastify) or Playwright for browsers [TEST-01] | Jest's ESM/TS support still needs transforms | SHOULD | [Vitest](https://vitest.dev/guide/), [Jest](https://jestjs.io/docs/getting-started), [node:test](https://nodejs.org/api/test.html), [Bun test](https://bun.sh/docs/cli/test), [Deno testing](https://docs.deno.com/runtime/fundamentals/testing/) | go-only (`skills/testing`) | review |
| `TS-TEST-02` | Test file globs: `**/*.test.ts`, `**/*.spec.ts`, `**/*.test.mts`, `**/*.e2e-spec.ts`, `**/__tests__/**`, `test/**`; JS twins `*.test.js`/`*.test.mjs` [TEST-02] | The test guard and ownership globs need them | MUST | [Vitest include](https://vitest.dev/config/include), [Jest testMatch](https://jestjs.io/docs/configuration#testmatch-arraystring) | go-only (`*_test.go`) | gate |
| `TS-TEST-03` | Use `describe`/`it`, `it.each` / `test.each` tables, arrange-act-assert, and names that read `it('rejects an expired token')` [TEST-03] | Same table-driven discipline as Go | SHOULD | [Vitest test API](https://vitest.dev/api/) | go-only (table-driven §30) | review |
| `TS-TEST-04` | Prefer fakes of ports (in-memory repo, fake clock) over module mocking; use `vi.fn`/`vi.spyOn` for interaction checks, `vi.useFakeTimers()` for time, and `msw` or `undici` MockAgent for HTTP; avoid `vi.mock` of internal modules [TEST-04] | Module mocking couples tests to file layout | SHOULD | [Vitest mocking](https://vitest.dev/guide/mocking), [MSW](https://mswjs.io/docs/) | go-only (fakes, `httptest`) | review |
| `TS-TEST-05` | Integration tests use Testcontainers for Node with a real Postgres/Redis/Kafka, one schema or transaction per test (or `TRUNCATE`), and migrations applied once in a global setup [TEST-05] | Mocks of the DB hide SQL and driver bugs | SHOULD | [Testcontainers for Node](https://node.testcontainers.org/) | missing | review |
| `TS-TEST-06` | Coverage: install `@vitest/coverage-v8`; command `vitest run --coverage --coverage.reporter=lcov` writes `coverage/lcov.info`; `coverage.include: ['src/**/*.ts']` so untested files count; v8 provider uses AST remapping since vitest 3.2 and matches Istanbul accuracy. Alternatives: jest `--coverage --coverageReporters=lcov` (`coverage/lcov.info`), `node --test --experimental-test-coverage --test-reporter=lcov --test-reporter-destination=lcov.info`, Bun `bun test --coverage --coverage-reporter=lcov` (`coverage/lcov.info`), Deno `deno test --coverage=cov && deno coverage cov --lcov` [TEST-06] | lcov is the common denominator and carries line+branch | MUST | [Vitest coverage](https://vitest.dev/guide/coverage.html), [Vitest coverage config](https://vitest.dev/config/coverage), [node:test coverage](https://nodejs.org/api/test.html), [Bun coverage](https://bun.sh/docs/test/coverage), [Deno coverage](https://docs.deno.com/runtime/fundamentals/testing/#coverage) | go-only (`gocover` adapter) | gate |
| `TS-TEST-07` | Exclude generated code and types-only files via `coverage.exclude` (`**/*.d.ts`, `**/generated/**`, `**/*.config.*`, `**/migrations/**`); the generated header marker is `@generated` or `// This file was automatically generated` [TEST-06] | Generated code distorts the percentage | SHOULD | [Vitest coverage config](https://vitest.dev/config/coverage) | go-only (`DefaultExclude`) | gate |
| `TS-TEST-08` | Compute coverage of new code in loops: parse lcov, intersect `DA:` lines with the `git diff -U0` added ranges of non-test source files, require at least 70%; do not rely on `coverage.thresholds`, which is repo-wide. `diff-cover` (Python) can also read coverage reports, but its lcov support is unverified here [TEST-07] | Hard gate is about changed lines | MUST | [Vitest coverage config thresholds/changed](https://vitest.dev/config/coverage), [diff-cover](https://github.com/Bachmann1234/diff_cover) | go-only (`gate/coverage.go` Go-profile parser) | gate |
| `TS-TEST-09` | Use fast-check for property tests on parsers, money math, state machines, and codecs (`fc.assert(fc.property(...))`, seed via `fc.configureGlobal({seed})`); add Stryker mutation tests on the domain package only, nightly [TEST-08] | Properties find edge cases tables miss; mutation is expensive | SHOULD | [fast-check docs](https://fast-check.dev/docs/introduction/), [Stryker](https://stryker-mutator.io/docs/) | go-only (fuzz in `skills/testing`) | review |
| `TS-TEST-10` | Test isolation: no shared module state between tests; keep vitest `isolate: true` (default) and the `forks` pool; reset mocks (`mockReset`, `restoreMocks`) and timers; control `Date` and randomness via injected ports [TEST-09] | Leaks cause order-dependent failures | MUST | [Vitest CLI](https://vitest.dev/guide/cli.html), [Vitest config](https://vitest.dev/config/) | covered (`skills/consistency`) | gate |
| `TS-TEST-11` | Set `testTimeout`/`hookTimeout` explicitly and fail on open handles (`--detectOpenHandles` for jest, `why-is-node-running` for debugging) [TEST-09] | Hung tests stall the gate | SHOULD | [Vitest testTimeout](https://vitest.dev/config/testtimeout), [Jest CLI](https://jestjs.io/docs/cli) | missing | gate |
| `TS-TEST-12` | Fast loop: `vitest` (watch by default in a TTY), `vitest run path/to/file.test.ts -t "name"`, `vitest related <file>` or `vitest run --changed <base>` for impacted tests; CI uses `vitest run` (no watch) and `--shard=i/n` across runners [TEST-10] | Tight TDD loop; CI must not hang in watch mode | SHOULD | [Vitest CLI](https://vitest.dev/guide/cli.html) | missing | review |

## 7. PERF: performance and profiling

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `TS-PERF-01` | Benchmark with Vitest 5 benchmarking: put benchmarks in `*.bench.ts` files and run them with `vitest bench` (they do not run from `*.test.ts` under `vitest run`; `import {bench} from "vitest"` in a test file throws "bench is not a function" in 5.1) or tinybench/mitata directly, never ad-hoc `Date.now()` loops; compare against a saved baseline and treat results as noisy [PERF-01] | JIT warm-up and GC distort single runs | SHOULD | [Vitest benchmarking](https://vitest.dev/guide/benchmarking) | go-only (`skills/performance` benchstat) | review |
| `TS-PERF-02` | Profile CPU with `node --cpu-prof` or `--inspect` + Chrome DevTools, heap with `--heap-prof`/heap snapshots, and use `clinic.js` or `0x` flamegraphs; continuous profiling via Pyroscope or Datadog if available [PERF-02] | Measure before tuning | SHOULD | [Node diagnostics](https://nodejs.org/en/learn/diagnostics/user-journey), [Clinic.js](https://clinicjs.org/) | go-only (pprof) | review |
| `TS-PERF-03` | Tune memory with `--max-old-space-size` sized under the container limit, use `NODE_ENV=production`, avoid per-request closures in hot paths, and prefer streams over buffering large bodies [PERF-03] | Node's default heap may exceed the cgroup limit and cause OOM kills | SHOULD | [Node CLI options](https://nodejs.org/api/cli.html#--max-old-space-sizesize-in-mib), [Express performance](https://expressjs.com/en/advanced/best-practice-performance.html) | missing | review |
| `TS-PERF-04` | Avoid hot-path mistakes: sync I/O, `JSON.stringify` of large graphs per request, N+1 awaits in a loop (batch with `Promise.all` plus a limit), spreading large arrays in reduce (O(n^2)), unbounded in-memory caches, and regex on untrusted input [PERF-04] | Common Node latency killers | AVOID | [Don't block the event loop](https://nodejs.org/en/learn/asynchronous-work/dont-block-the-event-loop) | missing | review |
| `TS-PERF-05` | Startup: build to JS with `tsc`/swc/esbuild for production images; do not run `ts-node` or `tsx` in production; consider Node's compile cache (`module.enableCompileCache()`) for CLIs [PERF-03] | Runtime transpile adds seconds to cold start | SHOULD | [Node module compile cache](https://nodejs.org/api/module.html#module-compile-cache) | missing | review |

## 8. SEC: security

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `TS-SEC-01` | Use parameterised queries only (`pg` `$1`, Prisma/Drizzle/Kysely builders); never build SQL with template strings from input; use `execFile`/`spawn` with an argv array and never `exec` or `shell: true` for untrusted data [SEC-01] | SQL and command injection | MUST | [OWASP SQL Injection Prevention](https://cheatsheetseries.owasp.org/cheatsheets/SQL_Injection_Prevention_Cheat_Sheet.html), [OWASP Node.js Security](https://cheatsheetseries.owasp.org/cheatsheets/Nodejs_Security_Cheat_Sheet.html) | go-only (`GO_BEST` §38) | review |
| `TS-SEC-02` | Parse untrusted JSON with `JSON.parse` then a schema; reject `__proto__`/`constructor` keys, avoid recursive merges and `Object.assign` of input, use `Object.create(null)` maps or `Map`; avoid `yaml.load` unsafe tags, `eval`, `vm` as a sandbox, and `node-serialize`; disable XML external entities in XML parsers [SEC-02] | Prototype pollution and deserialization RCE | MUST | [Node security best practices](https://nodejs.org/en/learn/getting-started/security-best-practices), [OWASP Deserialization](https://cheatsheetseries.owasp.org/cheatsheets/Deserialization_Cheat_Sheet.html) | missing | review |
| `TS-SEC-03` | Add `helmet` (Express/Fastify `@fastify/helmet`/Hono `secureHeaders`), configure CORS with an explicit origin allowlist, use CSRF tokens or SameSite cookies for cookie auth, block SSRF with an allowlist and no redirects to private ranges, and canonicalise paths (`path.resolve` then prefix check) to stop traversal [SEC-03] | Defaults are permissive | MUST | [OWASP Node.js Security](https://cheatsheetseries.owasp.org/cheatsheets/Nodejs_Security_Cheat_Sheet.html), [OWASP SSRF](https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html), [Helmet](https://helmetjs.github.io/) | missing | review |
| `TS-SEC-04` | Read secrets from env or a vault only in `main.ts`/config adapter; never commit `.env`; hash passwords with argon2id (`argon2`) or `crypto.scrypt`; use `crypto.randomBytes`/`randomUUID` for tokens (never `Math.random`); compare secrets with `crypto.timingSafeEqual` [SEC-04] | Wrong primitives are silent vulnerabilities | MUST | [OWASP Password Storage](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html), [Node crypto](https://nodejs.org/api/crypto.html) | go-only (`GO_BEST` §38) | review |
| `TS-SEC-05` | Validate every inbound payload, env var and queue message at the adapter with zod or valibot (`schema.parse`/`safeParse`), derive types with `z.infer`, and pass only parsed values inward; fastify uses JSON Schema (ajv), NestJS uses `class-validator` with `ValidationPipe({whitelist:true, forbidNonWhitelisted:true})` [SEC-05] | TS types vanish at runtime | MUST | [Zod](https://zod.dev/), [Valibot](https://valibot.dev/), [Fastify validation](https://fastify.dev/docs/latest/Reference/Validation-and-Serialization/), [NestJS validation](https://docs.nestjs.com/techniques/validation) | covered (`GO_BEST` §43 validation at boundaries) | review |
| `TS-SEC-06` | Treat install scripts as untrusted: use `npm ci --ignore-scripts` (or `ignore-scripts=true` in `.npmrc`) and allow builds per package; pnpm blocks dependency build scripts by default and uses `allowBuilds` with `strictDepBuilds`; also `blockExoticSubdeps`, `minimumReleaseAge` (default 1 day) and `trustPolicy: no-downgrade`; verify with `npm audit signatures` [SEC-06] | `postinstall` runs arbitrary code at install (Shai-Hulud, event-stream class attacks) | MUST | [pnpm supply chain security](https://pnpm.io/supply-chain-security), [npm audit signatures](https://docs.npmjs.com/cli/v11/commands/npm-audit#audit-signatures), [Node security](https://nodejs.org/en/learn/getting-started/security-best-practices) | missing | gate |
| `TS-SEC-07` | Review lockfile diffs in PRs, watch for typosquats and new maintainers, use scoped registries/mirrors, and never `npm install` a package an agent suggests without checking it exists and its age [SEC-06] | LLM-hallucinated package names are squatted | MUST | [OWASP Node.js Security](https://cheatsheetseries.owasp.org/cheatsheets/Nodejs_Security_Cheat_Sheet.html) | missing | hook |
| `TS-SEC-08` | Run SAST with Semgrep (`semgrep scan --config p/typescript --config p/nodejs`) and/or CodeQL; ESLint `eslint-plugin-security` is a weak supplement [SEC-07] | Pattern rules catch injection sinks | SHOULD | [Semgrep docs](https://semgrep.dev/docs/), [CodeQL JS/TS](https://codeql.github.com/docs/codeql-language-guides/codeql-for-javascript/) | missing | gate |
| `TS-SEC-09` | Consider Node's permission model (`--permission --allow-fs-read=... `) for hardening; flags change between releases, so check the docs for the pinned version [SEC-06] | Limits the damage from malicious dependencies | SHOULD | [Node permissions](https://nodejs.org/api/permissions.html) | missing | review |
| `TS-SEC-10` | Never log secrets, tokens, or full request bodies; use pino `redact` paths [SEC-04] | Logs leak credentials | MUST | [pino redaction](https://getpino.io/#/docs/redaction) | missing | review |

## 9. DATA: persistence

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `TS-DATA-01` | Prefer a typed query builder or thin ORM: Kysely or Drizzle for SQL-first teams, Prisma for schema-first with generated client, TypeORM/MikroORM only where already used; raw SQL via `pg`/`postgres.js` for hot paths. Keep ORM entities in adapters [DATA-01] | Types from schema beat hand-written casts | SHOULD | [Kysely](https://kysely.dev/), [Drizzle](https://orm.drizzle.team/docs/overview), [Prisma](https://www.prisma.io/docs) | missing | review |
| `TS-DATA-02` | Migrations are versioned files in `migrations/`, applied by a tool (Prisma Migrate, drizzle-kit, node-pg-migrate, Knex, Flyway) in CI/deploy, never `synchronize: true` in prod; use expand-contract for zero downtime [DATA-02] | Auto-sync drops data | MUST | [Prisma Migrate](https://www.prisma.io/docs/orm/prisma-migrate), [Drizzle migrations](https://orm.drizzle.team/docs/migrations) | covered (`skills/consistency` migrations) | review |
| `TS-DATA-03` | Open transactions in the application layer through a `UnitOfWork` port (`runInTransaction(fn)`), pass the tx handle explicitly (or via `AsyncLocalStorage`), and keep them short with no network calls inside [DATA-03] | Hidden ambient transactions leak across `await`s | SHOULD | [Node AsyncLocalStorage](https://nodejs.org/api/async_context.html), [HEXAGONAL_ARCHITECTURE](../../specs/HEXAGONAL_ARCHITECTURE.md) | covered (persistence section) | review |
| `TS-DATA-04` | Avoid N+1: use joins, `include`/`with`, DataLoader batching (GraphQL), and `inArray` batches; log query counts in tests; avoid lazy-loading proxies (TypeORM `lazy`) [DATA-04] | N+1 is the top ORM defect | AVOID | [DataLoader](https://github.com/graphql/dataloader), [Prisma relation queries](https://www.prisma.io/docs/orm/prisma-client/queries/relation-queries) | missing | review |
| `TS-DATA-05` | Use one shared pool per process (`pg.Pool` with `max`, `idleTimeoutMillis`, `connectionTimeoutMillis`, `statement_timeout`), handle `pool.on('error')`, release clients in `finally`, and size `max` against `processes × replicas` vs DB limits [DATA-05] | Leaked clients exhaust the pool | MUST | [node-postgres pool](https://node-postgres.com/apis/pool) | missing | review |
| `TS-DATA-06` | Use the outbox pattern for events published with a DB write and make consumers idempotent [DATA-03] | Dual writes lose messages | SHOULD | [Transactional outbox](https://microservices.io/patterns/data/transactional-outbox.html), [`skills/consistency`](../../skills/consistency/SKILL.md) | covered (`skills/consistency`) | review |

## 10. API: APIs and integration

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `TS-API-01` | Choose per FW section: Fastify for performance and schema-first, NestJS for large teams, Hono for edge/multi-runtime, Express for legacy; routes live in inbound adapters, middleware handles cross-cutting concerns only [API-01] | Frameworks differ in lifecycle hooks | SHOULD | [Fastify lifecycle](https://fastify.dev/docs/latest/Reference/Lifecycle/), [NestJS request lifecycle](https://docs.nestjs.com/faq/request-lifecycle), [Hono](https://hono.dev/docs/) | missing | review |
| `TS-API-02` | Generate the contract rather than hand-writing: code-first OpenAPI from zod/typebox (`@fastify/swagger`, `@asteasolutions/zod-to-openapi`, `@nestjs/swagger`) or OpenAPI-first with `openapi-typescript`; gRPC via `@grpc/grpc-js` + `ts-proto`; version via path or header [API-02] | Drift between docs and code | SHOULD | [OpenAPI spec](https://spec.openapis.org/oas/latest.html), [openapi-typescript](https://openapi-ts.dev/) | missing | review |
| `TS-API-03` | Return RFC 9457 `application/problem+json` for errors from one central error handler; map domain errors to status codes at the adapter; never leak stack traces [API-03] | Uniform, safe errors | MUST | [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) | covered (`HEXAGONAL` error taxonomy) | review |
| `TS-API-04` | Outbound calls get timeout (`AbortSignal.timeout`), bounded retries with jittered backoff only for idempotent operations, circuit breaker (`opossum`/`cockatiel`), and idempotency keys on mutating endpoints; inbound rate-limit with `@fastify/rate-limit`/`express-rate-limit` [API-04] | Cascading failures | MUST | [cockatiel](https://github.com/connor4312/cockatiel), [opossum](https://nodeshift.dev/opossum/) | covered (`skills/consistency` idempotency) | review |
| `TS-API-05` | Messaging: kafkajs/Confluent client with manual commit after processing, BullMQ/NATS/SQS clients with DLQ; handlers are idempotent because delivery is at-least-once [API-05] | Redelivery is normal | MUST | [KafkaJS](https://kafka.js.org/), [BullMQ](https://docs.bullmq.io/) | covered (`skills/jobs-and-state-machines`) | review |
| `TS-API-06` | Use built-in `fetch` (undici) with a shared `Agent` for keep-alive and pass `signal`; use `undici` directly for pooling control; avoid `axios` defaults without timeouts [API-06] | Default `fetch` has no timeout | MUST | [Node fetch/undici](https://nodejs.org/api/globals.html#fetch), [undici](https://undici.nodejs.org/) | go-only (`GO_BEST` §10 context) | review |
| `TS-API-07` | Limit request body size and parse only expected content types (`bodyLimit` in Fastify, `express.json({limit})`) [API-03] | Memory DoS | MUST | [Fastify server options](https://fastify.dev/docs/latest/Reference/Server/#bodylimit) | missing | review |

## 11. OBS: observability

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `TS-OBS-01` | Log JSON with pino (Fastify's default; `nestjs-pino`), levels `fatal..trace`, a request-id per request propagated via `AsyncLocalStorage`, and no PII; never `console.log` in services [OBS-01] | Structured logs are queryable; pino is async and fast | MUST | [pino](https://getpino.io/), [Fastify logging](https://fastify.dev/docs/latest/Reference/Logging/) | go-only (`GO_BEST` §24) | gate |
| `TS-OBS-02` | Expose Prometheus metrics via `prom-client` (or the OpenTelemetry metrics SDK), with `snake_case` names, base units and bounded label cardinality [OBS-02] | Label explosion costs money | SHOULD | [prom-client](https://github.com/siimon/prom-client), [Prometheus naming](https://prometheus.io/docs/practices/naming/) | missing | review |
| `TS-OBS-03` | Trace with OpenTelemetry JS: `@opentelemetry/sdk-node` plus auto-instrumentations, loaded before app code (`node --import ./instrumentation.js`), W3C trace context propagated to logs; tracing and metrics are stable, logs are maturing (check as of 2026-11) [OBS-03] | Late-loaded SDK misses patched modules (ESM needs the import hook) | SHOULD | [OpenTelemetry JS](https://opentelemetry.io/docs/languages/js/) | missing | review |
| `TS-OBS-04` | Provide `/healthz` (liveness, no dependencies) and `/readyz` (readiness, checks DB), via `@nestjs/terminus` or a plain route; flip readiness to false at shutdown start [OBS-04] | Orchestrators route on these | MUST | [NestJS Terminus](https://docs.nestjs.com/recipes/terminus) | missing | review |
| `TS-OBS-05` | Report errors with Sentry or OTel exception events, and register `process.on('unhandledRejection')`/`'uncaughtException'` handlers that log then exit non-zero, never continue [OBS-01] | Continuing after uncaught exceptions leaves undefined state | MUST | [Node process events](https://nodejs.org/api/process.html#event-uncaughtexception) | missing | review |

## 12. OPS: configuration, packaging and deployment

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `TS-OPS-01` | Parse `process.env` once in a config module with a zod schema (or `@nestjs/config` + validation, `env-schema`), fail fast on boot, inject typed config, never read `process.env` elsewhere; Node `--env-file=.env` is for dev only [OPS-01] | Typed, fail-fast config | MUST | [Twelve-Factor config](https://12factor.net/config), [Node --env-file](https://nodejs.org/api/cli.html#--env-fileconfig) | covered (`HEXAGONAL` config boundary) | review |
| `TS-OPS-02` | Package as a multi-stage Docker image: build stage `npm ci` + `tsc`, runtime stage `npm ci --omit=dev`, copy `dist/`, pinned LTS base tag or digest (slim or distroless), `USER node`, `NODE_ENV=production`, `.dockerignore` for `node_modules`, `.git`, `.env` [OPS-02] | Small, reproducible, non-root | MUST | [OWASP Node.js Docker](https://cheatsheetseries.owasp.org/cheatsheets/NodeJS_Docker_Cheat_Sheet.html), [Docker best practices](https://docs.docker.com/build/building/best-practices/) | missing | review |
| `TS-OPS-03` | Single-binary options (Node SEA, `bun build --compile`, `deno compile`) are viable for CLIs and simple services but have limits (SEA is CJS-first, native addons); default to container images [OPS-03] | Trade-offs: size, addon support | SHOULD | [Node SEA](https://nodejs.org/api/single-executable-applications.html), [Bun compile](https://bun.com/docs/compiler/executables), [Deno compile](https://docs.deno.com/runtime/reference/cli/compile/) | missing | review |
| `TS-OPS-04` | Handle `SIGTERM`/`SIGINT`: flip readiness, `server.close()` (plus `closeIdleConnections()`), abort in-flight with an `AbortController`, close DB and queue clients, force-exit after a deadline; run `node dist/main.js` via exec form, not `npm start`, with `--init`/tini [OPS-04] | npm does not forward signals; Kubernetes sends SIGTERM | MUST | [Node http server.close](https://nodejs.org/api/http.html#serverclosecallback), [OWASP Node.js Docker](https://cheatsheetseries.owasp.org/cheatsheets/NodeJS_Docker_Cheat_Sheet.html), [NestJS lifecycle events](https://docs.nestjs.com/fundamentals/lifecycle-events) | missing | review |
| `TS-OPS-05` | Run-scoped caches: point `npm_config_cache` (npm), `PNPM_HOME`/`npm_config_store_dir` (pnpm store), `YARN_CACHE_FOLDER`, `BUN_INSTALL_CACHE_DIR`, `DENO_DIR` at the per-run dir; `TMPDIR` for temp; `node_modules/.cache` (eslint, babel, vite) lives in the tree [OPS-05] | Avoid polluting the user's global cache; re-downloads are costly | SHOULD | [npm config cache](https://docs.npmjs.com/cli/v11/using-npm/config#cache), [pnpm settings](https://pnpm.io/settings), [Bun install cache](https://bun.sh/docs/install/cache), [Deno DENO_DIR](https://docs.deno.com/runtime/reference/env_variables/) | go-only (`GOCACHE`) | hook |
| `TS-OPS-06` | Artifacts left behind: `dist/`, `build/`, `coverage/`, `*.tsbuildinfo`, `.turbo/`, `.vitest/`/`node_modules/.vite`, `node_modules/.cache/`, `.eslintcache`, `*.cpuprofile`, `*.heapprofile`, `*.heapsnapshot`, `.nyc_output/`, `junit.xml`, `lcov.info`; `node_modules/` itself only if created by the run [OPS-06] | Drives cleanup | MUST | [tsconfig tsBuildInfoFile](https://www.typescriptlang.org/tsconfig/#tsBuildInfoFile), [ESLint cache](https://eslint.org/docs/latest/use/command-line-interface#--cache) | go-only (`*.test`, `*.prof`) | hook |
| `TS-OPS-07` | Do not use `npm install -g` in CI; invoke tools via `npx --no-install`, `pnpm exec` or `package.json` scripts so versions come from the lockfile [OPS-05] | Global tools drift | MUST | [npx](https://docs.npmjs.com/cli/v11/commands/npx) | missing | gate |
| `TS-OPS-08` | Default to Node for new services; choose Bun (`bun.lock`, `bun test`, `bun run`) or Deno (`deno.json`, permissions by default) only deliberately, since test, coverage and install tooling differ per runtime [OPS-02] | Runtime choice changes every command in the pack | SHOULD | [Bun docs](https://bun.sh/docs), [Deno docs](https://docs.deno.com/) | missing | review |

## 13. DOC: documentation and review

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `TS-DOC-01` | Document exported API with TSDoc (`/** ... @param ... */`) and generate with TypeDoc where a library is published; skip type annotations in comments [DOC-01] | Types already say the shape | SHOULD | [TSDoc](https://tsdoc.org/), [TypeDoc](https://typedoc.org/) | go-only (`GO_BEST` §26) | review |
| `TS-DOC-02` | README states Node version, package manager, scripts (`dev`, `build`, `test`, `lint`), env vars; keep ADRs in `docs/adr/` and a Changesets-generated `CHANGELOG.md` [DOC-02] | Onboarding in one page | SHOULD | [Changesets](https://github.com/changesets/changesets), [ADR](https://adr.github.io/) | covered (`skills/documentation`, ADR §) | review |
| `TS-DOC-03` | Reviewers check first: `any`/`as`/`!` introductions, floating promises, missing validation at the edge, unbounded queries, blocking calls, new dependencies and install scripts, lockfile diff, secrets [DOC-03] | The highest-yield TS review items | MUST | [Google TS style guide](https://google.github.io/styleguide/tsguide.html), [typescript-eslint](https://typescript-eslint.io/rules/) | go-only (`GO_BEST` §51-52 checklists) | review |
| `TS-DOC-04` | Export an OpenAPI document and publish it (Swagger UI/Scalar) from CI; treat contract diffs (`oasdiff`) as review artifacts [DOC-01] | API changes become visible | SHOULD | [oasdiff](https://github.com/oasdiff/oasdiff) | missing | gate |

## 14. FW: frameworks

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `TS-FW-01` | Dominant frameworks: Express (largest installed base, maintenance-mode feel), NestJS (enterprise), Fastify (performance), Hono (edge, multi-runtime); check current usage in the State of JS / npm trends before choosing [FW-01] | Pick on team size and runtime | SHOULD | [State of JS](https://stateofjs.com/), [npm trends](https://npmtrends.com/express-vs-fastify-vs-hono-vs-@nestjs/core) | missing | review |
| `TS-FW-02` | NestJS: modules, DI, decorators; follow its folder-per-feature layout only if domain and application stay decorator-free; unit tests via `Test.createTestingModule`; conflict with hexagonal is entities/DTO decorators and global providers [FW-02] | Nest's default is layered | SHOULD | [NestJS modules](https://docs.nestjs.com/modules), [NestJS testing](https://docs.nestjs.com/fundamentals/testing) | missing | review |
| `TS-FW-03` | Fastify: encapsulated plugins with `fastify-plugin` only for shared decorators, JSON Schema per route, `app.inject()` for tests (no port), `fastify.register` as the composition root [FW-02] | Plugin scope is the DI mechanism | SHOULD | [Fastify plugins](https://fastify.dev/docs/latest/Reference/Plugins/), [Fastify testing](https://fastify.dev/docs/latest/Guides/Testing/) | missing | review |
| `TS-FW-04` | Hono: web-standard `Request`/`Response`, `app.request()` for tests, `zValidator` for validation, runs on Node/Bun/Deno/Workers; keep handlers thin [FW-02] | Portability | SHOULD | [Hono docs](https://hono.dev/docs/), [Hono testing](https://hono.dev/docs/guides/testing) | missing | review |
| `TS-FW-05` | Express: v5 handles rejected promises in async handlers; still add a final error middleware, and avoid `app.use` order bugs and `req.body` untyped access (parse with zod) [FW-03] | v4 swallowed async errors | MUST | [Express 5 migration](https://expressjs.com/en/guide/migrating-5.html), [Express error handling](https://expressjs.com/en/guide/error-handling.html) | missing | review |
| `TS-FW-06` | NestJS footguns: circular module/provider dependencies (`forwardRef`), request-scoped providers cascading and slowing everything, `@Global()` overuse, `synchronize: true`, missing `app.enableShutdownHooks()`; Express/Fastify footguns: unhandled async errors, missing `bodyLimit` [FW-03] | Known framework traps | MUST | [NestJS injection scopes](https://docs.nestjs.com/fundamentals/injection-scopes), [NestJS lifecycle events](https://docs.nestjs.com/fundamentals/lifecycle-events) | missing | review |

## Top 10 footguns

1. **Floating promises.** Bad: `saveAudit(x);` (rejection becomes `unhandledRejection`, which crashes Node by default). Fix: `await saveAudit(x)` or `void saveAudit(x).catch(log)`; lint `no-floating-promises`. ([source](https://typescript-eslint.io/rules/no-floating-promises/))
2. **`forEach(async)`.** Bad: `items.forEach(async i => await save(i))`. Fix: `await Promise.all(items.map(save))` with a concurrency cap, or a `for...of` loop. ([source](https://typescript-eslint.io/rules/no-misused-promises/))
3. **Blocking the event loop.** Bad: `fs.readFileSync`, large `JSON.parse`, nested-quantifier regex in a handler. Fix: async APIs, `worker_threads`, input limits. ([source](https://nodejs.org/en/learn/asynchronous-work/dont-block-the-event-loop))
4. **Types are not validation.** Bad: `const body = req.body as CreateUser`. Fix: `CreateUser.parse(req.body)` with zod/valibot. ([source](https://zod.dev/))
5. **`strict` without `noUncheckedIndexedAccess`.** Bad: `const x = arr[i]; x.id` types as safe. Fix: enable the flag and handle `undefined`. ([source](https://www.typescriptlang.org/tsconfig/#noUncheckedIndexedAccess))
6. **Install scripts and floating versions.** Bad: `npm install` in CI, `postinstall` allowed. Fix: `npm ci --ignore-scripts`, lockfile review, release cooldown. ([source](https://nodejs.org/en/learn/getting-started/security-best-practices))
7. **Prototype pollution.** Bad: `Object.assign(target, JSON.parse(input))` or recursive merge. Fix: schema validation, `Map`, `Object.create(null)`. ([source](https://nodejs.org/en/learn/getting-started/security-best-practices))
8. **Check-then-act across `await`.** Bad: `if (!(await exists(id))) await create(id)`. Fix: DB unique constraint with upsert, or a per-key lock. ([source](https://nodejs.org/en/learn/asynchronous-work/dont-block-the-event-loop))
9. **PID 1 and signals.** Bad: `CMD npm start`, no shutdown handler (SIGTERM lost, requests dropped). Fix: `CMD ["node","dist/main.js"]` with an init process and graceful `server.close()`. ([source](https://cheatsheetseries.owasp.org/cheatsheets/NodeJS_Docker_Cheat_Sheet.html))
10. **Type stripping is not type checking, and `enum` breaks it.** Bad: shipping on `node file.ts` with no `tsc`, using `enum`/parameter properties. Fix: `tsc --noEmit` in the gate and `erasableSyntaxOnly`. ([source](https://nodejs.org/api/typescript.html))

## 15. LOOPS: tooling data

The runner is `npx --no-install` so it works for npm, pnpm, yarn (classic) and bun installs; a pnpm/yarn/bun project should substitute `pnpm exec`, `yarn`, `bunx` (pack should derive it from the lockfile). The `typecheck` and `install` kinds are additions to the schema.

```json
{
  "id": "typescript",
  "name": "TypeScript / JavaScript (server-side)",
  "versions": {"stable": "26", "lts": "26", "as_of": "2026-11"},
  "detect": [{"file": "package.json"}, {"file": "tsconfig.json"}, {"file": "deno.json"}],
  "source_globs": ["**/*.ts", "**/*.mts", "**/*.cts", "**/*.js", "**/*.mjs", "**/*.cjs"],
  "test_globs": ["**/*.test.ts", "**/*.spec.ts", "**/*.test.mts", "**/*.e2e-spec.ts", "**/*.test.js", "**/*.test.mjs", "**/__tests__/**", "test/**"],
  "generated": {"globs": ["**/*.d.ts", "**/generated/**", "**/dist/**", "**/coverage/**"], "header_regex": "(@generated|This file was automatically generated)"},
  "checks": [
    {"name": "install", "kind": "install", "argv": ["npm", "ci", "--ignore-scripts"], "required": true, "tool": "npm"},
    {"name": "typecheck", "kind": "typecheck", "argv": ["npx", "--no-install", "tsc", "--noEmit"], "required": true, "tool": "tsc"},
    {"name": "prettier", "kind": "format", "argv": ["npx", "--no-install", "prettier", "--check", "."], "required": false, "tool": "prettier"},
    {"name": "eslint", "kind": "lint", "argv": ["npx", "--no-install", "eslint", "--max-warnings=0", "."], "required": false, "tool": "eslint"},
    {"name": "build", "kind": "build", "argv": ["npm", "run", "build", "--if-present"], "required": true, "tool": "npm"},
    {"name": "test", "kind": "test", "argv": ["npx", "--no-install", "vitest", "run", "--coverage", "--coverage.provider=v8", "--coverage.reporter=lcov", "--coverage.reportsDirectory={coverage_dir}", "--sequence.shuffle.files", "--sequence.shuffle.tests", "--sequence.concurrent"], "required": true, "tool": "vitest"},
    {"name": "npm-audit", "kind": "vuln", "argv": ["npm", "audit", "--audit-level=high", "--omit=dev"], "required": false, "tool": "npm"},
    {"name": "osv-scanner", "kind": "vuln", "argv": ["osv-scanner", "scan", "source", "-r", "."], "required": false, "tool": "osv-scanner", "install": "go install github.com/google/osv-scanner/v2/cmd/osv-scanner@latest"}
  ],
  "test_commands": [
    {"argv_prefix": ["npx", "vitest"]}, {"argv_prefix": ["vitest"]}, {"argv_prefix": ["npx", "jest"]}, {"argv_prefix": ["jest"]},
    {"argv_prefix": ["node", "--test"]}, {"argv_prefix": ["bun", "test"]}, {"argv_prefix": ["deno", "test"]},
    {"argv_prefix": ["npm", "test"]}, {"argv_prefix": ["npm", "run", "test"]}, {"argv_prefix": ["pnpm", "test"]}, {"argv_prefix": ["yarn", "test"]}
  ],
  "race": {"mode": "none", "flags": [], "env": {}, "reason": "JavaScript has no data-race detector; races are logical interleavings across await or SharedArrayBuffer use in worker_threads", "substitute": "vitest run --sequence.shuffle.files --sequence.shuffle.tests --sequence.concurrent with a printed seed (--sequence.seed=N) and --repeats=3 on changed test files; node:test equivalent --test-randomize --test-concurrency (Node 26.1+ only); jest 30 --randomize; no --retry.count above 0"},
  "coverage": {"format": "lcov", "path": "{coverage_dir}/lcov.info", "granularity": "line+branch"},
  "post_edit": {"format_file": ["npx", "--no-install", "prettier", "--write", "{file}"], "lint_dir": ["npx", "--no-install", "eslint", "{dir}"]},
  "cache_env": {"npm_config_cache": "cache", "YARN_CACHE_FOLDER": "cache", "npm_config_store_dir": "cache", "BUN_INSTALL_CACHE_DIR": "cache", "DENO_DIR": "cache", "TMPDIR": "tmp"},
  "artifacts": ["dist/", "build/", "coverage/", "*.tsbuildinfo", ".turbo/", "node_modules/.cache/", "node_modules/.vite/", ".eslintcache", ".nyc_output/", "*.cpuprofile", "*.heapprofile", "*.heapsnapshot", "lcov.info", "junit.xml"],
  "arch": {
    "import_regex": "(?:import|export)\\s+(?:type\\s+)?(?:[^'\";]*?\\s+from\\s+)?['\"]([^'\"]+)['\"]|import\\(\\s*['\"]([^'\"]+)['\"]\\s*\\)|require\\(\\s*['\"]([^'\"]+)['\"]\\s*\\)",
    "resolver": "relative-path",
    "native_tool": "dependency-cruiser",
    "forbidden_in_domain": ["node:fs", "node:net", "node:http", "node:https", "node:child_process", "node:crypto", "fs", "http", "child_process", "@nestjs/*", "express", "fastify", "hono", "pg", "@prisma/client", "typeorm"],
    "impure_calls": ["Date.now", "new Date(", "Math.random", "crypto.randomUUID", "process.env", "fetch(", "setTimeout(", "setInterval("]
  },
  "test_jobs": "--maxWorkers=2",
  "config_files": ["eslint.config.mjs", "eslint.config.js", "eslint.config.ts", "biome.json", "biome.jsonc", ".prettierrc", ".prettierrc.json", ".prettierrc.yaml", ".editorconfig", "tsconfig.json", ".dependency-cruiser.cjs", "vitest.config.ts", "package.json", ".nvmrc", ".npmrc"],
  "sources": {
    "checks": "https://www.typescriptlang.org/tsconfig/ ; https://typescript-eslint.io/getting-started/typed-linting/ ; https://prettier.io/docs/cli ; https://docs.npmjs.com/cli/v11/commands/npm-ci ; https://docs.npmjs.com/cli/v11/commands/npm-audit ; https://google.github.io/osv-scanner/",
    "race": "https://vitest.dev/guide/cli.html ; https://nodejs.org/api/test.html",
    "coverage": "https://vitest.dev/guide/coverage.html ; https://vitest.dev/config/coverage",
    "arch": "https://github.com/sverweij/dependency-cruiser/blob/main/doc/rules-reference.md",
    "versions": "https://nodejs.org/en/about/previous-releases"
  }
}
```

## Gaps vs loops

| Rule ID | Gap | loops today | Priority | Where loops should fix it |
|---|---|---|---|---|
| `TS-TEST-06/08` | No lcov parser or `DA:`-line new-code coverage; the gate only reads Go profiles | go-only | P0 | gate (`gate/coverage.go`) plus a new `lcov` outbound adapter |
| `TS-CONC-04` | `race-guard` is Go-only; for `mode=none` loops needs a substitute check (shuffle/concurrent/repeat, ban `--retry`) | go-only | P0 | hook `race-guard`, pack `race` field, gate |
| `TS-TOOL-06/07`, `TS-TOOL-03` | Gate switch has no tsc, eslint/biome, prettier, vitest checks | missing | P0 | gate: data-driven `checks` from the pack |
| `TS-SEC-06`, `TS-TOOL-08` | No `npm audit`/OSV step, no install-script policy | missing | P0 | gate vuln check, pack, hook for `npm install` without `--ignore-scripts` |
| `TS-REPO-05/06/10` | No lockfile/`engines`/`.nvmrc` rules; `init` requires `go.mod` | missing | P0 | language detection (`package.json`), spec WORKFLOW, pack skill |
| `TS-ARCH-04/05` | No import parser for TS or dependency-cruiser integration | missing | P1 | arch: `relative-path` resolver, `native_tool` delegation |
| `TS-ARCH-07` | Purity list is Go (`time.Now`, `os`); needs TS equivalents | go-only | P1 | arch checker config per pack |
| `TS-OPS-05/06` | Run-scoped cache env and artifact cleanup are Go-only (`GOCACHE`, `*.test`) | go-only | P1 | `loops run env`, workspace cleanup, hook |
| `TS-CONC-06/07/08`, `TS-IDIOM-13` | Event-loop, floating promise, AbortSignal rules not in any skill | missing / go-only | P1 | pack skill, lint config (no-floating-promises) in gate |
| `TS-SEC-01..05` | No OWASP mapping; validation is Go-worded | missing | P1 | pack skill reference, SAST step (Semgrep) |
| `TS-ARCH-02/03/06/10`, `TS-FW-*` | Hexagonal mapping for TS/NestJS absent | missing | P1 | pack ARCH reference and role |
| `TS-REPO-01..04,07..09,11..12` | No repo bootstrapping, CI shape, Renovate, provenance rules | missing | P2 | pack skill |
| `TS-TEST-01..05,09..12` | Testing skill cites Go; vitest idioms, Testcontainers, fast-check absent | go-only / missing | P2 | pack testing skill |
| `TS-PERF-*`, `TS-OBS-*`, `TS-API-01/02/06/07`, `TS-DATA-01/04/05`, `TS-DOC-*` | Perf and observability are Go-worded; no OTel | go-only / missing | P2 | pack skills |
| `TS-OPS-01..04,07,08` | No packaging or shutdown guidance | missing | P2 | pack skill |

## Open questions

- **Package manager and runner:** `npx --no-install` works for npm and most others, but pnpm/yarn Berry/bun projects should use their own exec; should the pack derive the runner from the lockfile? Corepack is not bundled with Node 26 (verifier confirmed), so `packageManager` is only honoured where Corepack or the package manager itself (pnpm, npm 11) enforces it.
- **Coverage output path:** vitest accepts `--coverage.reportsDirectory`, but loops' schema only has `{coverage}` (a file placeholder). The block uses an invented `{coverage_dir}` placeholder and `coverage.path` is `{coverage_dir}/lcov.info`, consistent with the argv; the maintainer must add the directory placeholder (default `coverage`).
- **Schema additions:** kinds `install` and `typecheck` are not in the Go example schema. Whether `install` belongs in the gate (it mutates `node_modules`) needs a decision.
- **Race policy:** `mode=none` plus shuffle/concurrent is my judgment, not a source. `vitest --sequence.concurrent` can change semantics for suites that rely on file-level `beforeEach` state; the gate might use it only in a second pass. Confirm the maintainers accept `none` as satisfying the hard "race or equivalent" rule.
- **Runtimes:** the block targets Node + vitest. Bun and Deno need alternate test argv and lcov recipes (listed in TS-TEST-06) and a separate detect rule (`deno.json`, `bun.lock`).
- **New-code coverage tools:** `diff-cover` lcov support was not verified; loops should compute it itself.
- **Node versions:** per the verifier, 26 is Active LTS (2026-10-28), 24 and 22 Maintenance; `engines` examples (`>=24 <27`) should be rechecked at pack time. Node 27 is alpha and moves to an annual cadence.
- **TypeScript 7:** typescript-eslint lists support only for `<6.1.0`; whether it supports TS 7 is unsettled.
- **Biome vs typescript-eslint:** Biome's type-aware lint coverage is undocumented on its CLI page; unsettled as of 2026-11.
- **Temporal:** support status in Node was not verified.
- Several `docs.*` and `github.com` URLs were taken from known canonical locations without a fetch (Vitest bench anchor, Jest, Bun, Deno, Prettier, Fastify, NestJS pages); the verifier should open them.

## Verification

- **Verdict: `fail`, round 1.** The file has 4 lint ERR rows, 3 FAIL links, wrong Node LTS data in the header, in TOOL-01 and in the LOOPS block, and one LOOPS path inconsistency. The core tooling (vitest argv, lcov path, race substitute flags) is correct, and the spot run confirmed it.
- **Completeness:** all 14 sections, Top 10, LOOPS, Gaps, and Open questions are present. Rule counts: REPO 13, TOOL 10, ARCH 11, IDIOM 14, CONC 10, TEST 12, PERF 5, SEC 10, DATA 6, API 7, OBS 5, OPS 8, DOC 4, FW 6, so every minimum is met. The LOOPS block parses and has every required key. Lint result with the current `lint.py`: `ERR 4 malformed rule rows` (TS-ARCH-02, TS-ARCH-05, TS-ARCH-07, TS-DATA-06).
- **Links** (`links.sh`, run twice): `FAIL 404 https://bun.com/docs/compiler/executables`, `FAIL 404 https://docs.deno.com/runtime/reference/env_variables/`, `FAIL 404 https://vite.dev/guide/performance#avoid-barrel-files`. npmtrends returned 500 on the first run only and passed on the recheck, so it is not a finding.
- **Sources opened (12):**
  - https://vitest.dev/guide/cli.html: yes. It lists `--sequence.shuffle.files`, `.tests`, `--sequence.seed`, `--sequence.concurrent`, `--repeats`, `--coverage.provider`, `--coverage.reporter`, `--coverage.reportsDirectory` (default `./coverage`), `--changed`, `--shard`, `--maxWorkers` and `vitest related`. Plain `--retry` is not listed; only `--retry.count`, `.delay` and `.condition` are.
  - https://nodejs.org/api/test.html: yes. `--test-randomize` exists, "Added in: v26.1.0, Stability 1.0 - Early development", with `--test-random-seed`. `--test-concurrency` and the lcov reporter with `--test-reporter-destination` are documented.
  - https://jestjs.io/docs/cli: yes. Jest 30.5 has `--randomize`, `--seed` and `--detectOpenHandles`.
  - https://vitest.dev/guide/coverage.html: yes. V8 has used AST-based remapping since v3.2.0. Untested files are excluded unless `coverage.include` is set. The current major is Vitest 5 (5.1.0).
  - https://nodejs.org/api/typescript.html: yes. Type stripping is on by default from v23.6.0/v22.18.0 and stable from v25.2.0/v24.12.0. `enum`, runtime `namespace` and parameter properties are unsupported, and `.ts` extensions are mandatory.
  - https://typescript-eslint.io/getting-started/typed-linting/: yes. It uses `defineConfig` with `projectService: true`. Note that it recommends `recommendedTypeChecked`; `strictTypeChecked` appears under shared configs.
  - https://pnpm.io/supply-chain-security: yes. It confirms `allowBuilds`, `strictDepBuilds` (on by default), `blockExoticSubdeps` (on by default), `minimumReleaseAge` (default 1440 minutes) and `trustPolicy: no-downgrade`.
  - https://nodejs.org/en/about/previous-releases: partial. Node 26 was first released 2026-05-05 and is listed as "Latest LTS"; 27 is alpha with an annual cadence from 27 on; 25 and 20 are EOL. The page does not support "LTS since 2026-05" (see finding 7).
  - https://biomejs.dev/reference/cli/: weak. It supports `biome ci --reporter=github` but says nothing about type-aware rules, so it does not back TS-TOOL-04's claim.
  - https://typescript-eslint.io/troubleshooting/typed-linting/performance/ (the moved page behind the 404 in TS-ARCH-11): no. It does not mention barrel files.
  - https://vitest.dev/guide/features#benchmarking: partial. In Vitest 5, benchmarking uses the `bench` fixture from the test context (see finding 9).
  - Prettier CLI, Fastify Server, NestJS injection scopes: fetched with curl and grepped. Each contains the cited option (`--check`, `bodyLimit`, request scope), so the researcher's unfetched URLs are now checked except the Bun and Deno ones, which are dead (finding 4).
- **Tooling truth** (spot run, node:26 = v26.13.0, npm 11.20.0, vitest 5.1.0):
  - `npm ci --ignore-scripts`: confirmed (exit 0).
  - `npx --no-install tsc --noEmit`: confirmed (exit 0).
  - `npx --no-install prettier --check .`: confirmed. It exits 1 when files need formatting, so `fails_on: exit` is the right reading.
  - `npx --no-install vitest run --coverage --coverage.provider=v8 --coverage.reporter=lcov --coverage.reportsDirectory=coverage --sequence.shuffle.files --sequence.shuffle.tests --sequence.concurrent`: confirmed (exit 0). It prints "Running tests with seed …" and writes `coverage/lcov.info` (lcov: `SF:`, `FN:`, `DA:`) plus `coverage/lcov-report/`.
  - `--sequence.seed=42 --repeats=3`: confirmed (exit 0).
  - `node --test --test-randomize --test-concurrency=2`: confirmed on Node 26 (exit 0).
  - `node --test --experimental-test-coverage --test-reporter=lcov --test-reporter-destination=…`: confirmed (exit 0, writes lcov).
  - `npm audit --audit-level=high --omit=dev` and `npm sbom --sbom-format cyclonedx`: confirmed (exit 0).
  - Not run: osv-scanner, eslint and the Jest argv. Their syntax matches the docs.
- **Spot run:** image `node:26` (v26.13.0). The commands and exit codes are listed above. `command -v corepack` returns nothing, so Corepack is not bundled with Node 26, which settles that Open question. `npm config ls -l` shows `min-release-age` exists in npm 11.20.
- **Findings:**
  1. **TS-ARCH-05: broken table row (lint ERR).** The unescaped `|` in `^src/(application|adapters|compose)|^node_modules/(?!zod)` splits the row into 10 cells, and GFM renders it broken. Fix: escape each pipe as `\|`, or move the regex into a code block under the table.
  2. **TS-TOOL-07, TS-IDIOM-03: resolved.** These rows use escaped `\|`, which GFM renders correctly. The updated `lint.py` now honours `\|` and no longer flags them. No action needed.
  3. **TS-ARCH-02, TS-ARCH-07, TS-DATA-06: no external URL (lint ERR).** These rows cite only loops specs. Fix: add a primary source, for example dependency-cruiser rules for ARCH-02, the TS handbook or Node `process.env` docs for ARCH-07, and https://microservices.io/patterns/data/transactional-outbox.html for DATA-06.
  4. **Dead links.**
     - TS-OPS-03 `https://bun.com/docs/compiler/executables` returns 404; replace it with https://bun.com/docs/compiler/executables (200).
     - TS-OPS-05 `https://docs.deno.com/runtime/reference/env_variables/` returns 404; replace it with https://docs.deno.com/runtime/reference/env_variables/ (200, documents `DENO_DIR`).
  5. **TS-ARCH-11: dead and unsupporting source.** The URL returns 404. Its successor (typed-linting/performance) does not mention barrel files. Fix: cite https://vite.dev/guide/performance#avoid-barrel-files (checked: it contains "Avoid Barrel Files").
  6. **TS-TOOL-04: weak source.** The Biome CLI page says nothing about type-aware linting. Fix: cite Biome's linter or type-inference docs (Biome 2.6.6 is current on npm) and restate what Biome 2.x covers, for example whether it has a floating-promise rule.
  7. **Node versions are wrong (header, TS-TOOL-01, LOOPS `versions`).**
     - Node 26 was released 2026-05-05 but became Active LTS only on 2026-10-28 (v26.13.0).
     - Node 24 entered Maintenance on 2026-10-20 (EOL 2028-04-30), and Node 22 is in Maintenance until 2027-04-30.
     - Evidence: https://raw.githubusercontent.com/nodejs/Release/main/schedule.json and https://nodejs.org/dist/index.json.
     - Fix: change "LTS since/from May 2026" to "released 2026-05, Active LTS since 2026-10-28", and set `"lts": "26"` (Node 24 is Maintenance LTS).
  8. **TOOL-01: TypeScript version is out of date.** The header says "TypeScript 5.8+" and nothing states the current compiler. The npm registry lists `typescript` 7.0.2 as latest (the native port), alongside `typescript-eslint` 8.72.2, `eslint` 10.14.0 and `vitest` 5.1.0. Fix: record the current TypeScript major, and check that typescript-eslint type-aware linting supports TS 7 before TS-TOOL-03 makes it a MUST.
  9. **TS-PERF-01: outdated for Vitest 5.** `import {bench} from "vitest"` fails under vitest 5.1.0 with "TypeError: bench is not a function" (spot run). The docs now describe a `bench` fixture from the test context. Fix: update the rule and its source.
  10. **TS-CONC-04, TS-CONC-05, LOOPS `race.substitute`: version limits not stated.**
      - `node --test-randomize` exists only from Node v26.1.0 and is "Early development", so it is unavailable on Node 24 and 22. Say so.
      - Vitest 5's CLI documents `--retry.count`, not `--retry` (`--retry=0` is still accepted, exit 0). Name `--retry.count` in the ban.
  11. **LOOPS coverage path does not match the argv.** The test argv passes `--coverage.reportsDirectory={coverage_dir}`, but `coverage.path` is the fixed `coverage/lcov.info`. If loops fills `{coverage_dir}` with a run directory, the file lands at `{coverage_dir}/lcov.info`. Fix: set `"path": "{coverage_dir}/lcov.info"`, or drop the flag and keep the default.

### Round 2

- **Verdict: `fail`, round 2.** Lint has no ERR and `links.sh` has no FAIL. 9 of 11 findings are resolved; findings 5 and 9 are still open, and both are small edits.
- **LOOPS block:** parses with jq.
  - `versions` = `{"stable":"26","lts":"26"}`.
  - `coverage.path` = `{coverage_dir}/lcov.info`, which matches `--coverage.reportsDirectory={coverage_dir}` in the test argv. The argv is otherwise unchanged and still confirmed by the round-1 spot run.
  - `race.substitute` now says "Node 26.1+ only" and "no --retry.count above 0".
- **New claim checked:** typescript-eslint supports TS `>=4.8.4 <6.1.0`. Confirmed from the npm registry: the `peerDependencies.typescript` of `typescript-eslint` 8.72.2 and of `@typescript-eslint/typescript-estree`.
- **Findings:**
  1. TS-ARCH-05: **resolved.** The pipes are escaped and the lint passes.
  2. TS-TOOL-07, TS-IDIOM-03: **resolved** (round 1).
  3. TS-ARCH-02, TS-ARCH-07, TS-DATA-06: **resolved.** They now cite dependency-cruiser rules, Node `process.env` and microservices.io outbox.
  4. Bun and Deno links: **resolved.** Both are replaced (bun.com/docs/compiler/executables, docs.deno.com/runtime/reference/env_variables/) and return 2xx.
  5. TS-ARCH-11: **still open (trivial).** The URL is now https://vite.dev/guide/performance#avoid-barrel-files, but the link text still reads "typescript-eslint import rules guidance", which credits the wrong source. Rename it to "Vite performance: avoid barrel files".
  6. TS-TOOL-04: **resolved.** The claim is now marked unverified and cites the Biome linter page. That is an honest hedge.
  7. Node versions: **resolved** in the header, TS-TOOL-01 and the LOOPS `lts`.
  8. TypeScript version: **resolved.** The header names TS 7.0.2. TS-TOOL-03 says to pin TS ≤6.0 for typed linting, which matches the registry data.
  9. TS-PERF-01: **still open.** The new `bench` fixture example only works in a benchmark file.
     - Re-spot-run on node:26 with vitest 5.1.0: in a `*.test.ts` file it fails with "Cannot use the `bench` test-context fixture within a regular test run … Move this code to a file matched by `benchmark.include` (default `**/*.{bench,benchmark}.?(c|m)[jt]s?(x)`)".
     - In `s/a.bench.ts` run with `npx vitest bench --run` it passes and prints the comparison table.
     - Fix: say the code goes in `*.bench.ts` files run by `vitest bench`, and cite https://vitest.dev/guide/benchmarking (200).
  10. TS-CONC-04, TS-CONC-05, `race.substitute`: **resolved.**
  11. LOOPS coverage path: **resolved.**

### Lead fix after round 2

The lead applied the verifier's remaining fixes verbatim instead of running a third loop (WORKFLOW §4 caps rounds at 2): finding 5 (link text) and finding 9 (benchmarks live in `*.bench.ts` and run with `vitest bench`, citing vitest.dev/guide/benchmarking). lint.py has no ERR and the LOOPS block passes `jq`. **Final: pass (lead-applied).**
