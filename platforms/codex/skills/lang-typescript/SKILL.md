---
name: lang-typescript
description: Use when writing, reviewing, testing or setting up TypeScript or JavaScript (Node, Bun) backend code in a loops run (the typescript language pack) - toolchain, layout, race policy, coverage, idioms and footguns.
---

# TypeScript / JavaScript backend (Node 26 LTS, as of 2026-11)

Rule book: `specs/languages/typescript/RULES.md` (IDs `TS-<SEC>-NN`). Pack data: `specs/languages/typescript/pack.json`. Universal rules: `specs/ARCHITECTURE.md`, `specs/SOLID.md`; do not restate them.

## 1. When it applies
`loops lang detect` reports `typescript` (a `package.json` or `tsconfig.json` at the root). The pack picks a variant from the lockfile: `package-lock.json` is npm (the base), `pnpm-lock.yaml`, `yarn.lock` (Yarn Berry), `bun.lock`. Deno has no variant: `deno.json` projects use the commands in `testing.md` as project overrides.

## 2. Project rules first
Follow the repo's own configs over this pack: ESLint flat config or Biome, Prettier, `tsconfig`, the package manager, the test runner, the framework's layout. Log every deviation: `loops run override --rule "<loops rule>" --project "<project rule, file:line>" --note "<what you did>"` (`ROLES.md`, Project rules first). Never add a second formatter or linter next to the project's.

## 3. The toolchain the gate runs
`loops gate` runs the checks below in order (WORKFLOW §6). While iterating, run one check's command, never the whole repo by hand. A check with `when_file` runs only if that config is committed, and then it is required.

| Check | Command (npm) | Runs when |
|---|---|---|
| `deps` | `npm ci --ignore-scripts` (pnpm `install --frozen-lockfile`, yarn `install --immutable`, bun `install --frozen-lockfile`) | a lockfile is committed |
| `typecheck` | `npx --no-install tsc --noEmit` | `tsconfig.json` |
| `prettier` | `npx --no-install prettier --check .` | a Prettier config |
| `eslint` | `npx --no-install eslint --max-warnings=0 .` | `eslint.config.*` |
| `biome` | `npx --no-install biome ci .` | `biome.json[c]`; replaces Prettier and ESLint |
| `build` | `npm run build --if-present` | always |
| `arch`, `depcruise` | `loops arch`; `depcruise src --config .dependency-cruiser.cjs` | `depcruise` needs its config |
| `test`, `test-repeat` | vitest (jest when no vitest or vite config; `bun test` in the bun variant), see §4 | always |
| `coverage` | new-code lcov coverage at least 70% (WORKFLOW §6) | after `test` |
| `npm-audit`, `osv-scanner` | `npm audit --audit-level=high --omit=dev`; `osv-scanner scan source -r .` | `--vuln` only |

Tools run through `npx --no-install` (or the manager's own exec), so versions come from the lockfile; never `npm install -g` (TS-OPS-07). Installs always use `--ignore-scripts` (TS-SEC-06); a dependency that needs a build step is allowed explicitly (pnpm `allowBuilds`), by the architect. `post_edit` formats the one edited file with `prettier --write`. Biome projects override that in project config. Yarn Classic: use `--frozen-lockfile` and `yarn run build` as an override.

## 4. Race policy
JavaScript has no race detector, so the policy is `race.mode = none` (WORKFLOW §6, GENERIC §5.2). Two required checks carry it, and a skipped one fails the full gate:
1. `test`: the whole suite, `--sequence.shuffle.files --sequence.shuffle.tests --sequence.concurrent`. vitest prints "Running tests with seed ..."; paste that seed in a failure report and replay with `--sequence.seed=N`.
2. `test-repeat`: `vitest run --changed --repeats=3` with the same shuffle and concurrency, on the files you changed.

Retries stay off: never `--retry.count` above 0, `retry:` in config, or `jest.retryTimes`; never `--no-randomize`, `--sequence.shuffle=false` or `--sequence.concurrent=false`. A flaky test is a shared-state bug: find the module state, the un-awaited promise, or the check-then-act across `await` (`concurrency.md`). Node's `node:test` needs `--test-randomize --test-concurrency` (Node 26.1+); jest 30 needs `--randomize`.

## 5. Coverage recipe
`vitest run --coverage --coverage.provider=v8 --coverage.reporter=lcov --coverage.reportsDirectory={coverage_dir}` writes `{coverage_dir}/lcov.info`; loops reads `DA:` lines of changed non-test source files. Wave 0 prerequisites (architect or `typescript-engineer`, each logged as an override when it deviates from the repo):
- `@vitest/coverage-v8` in devDependencies (jest: built in; bun: built in).
- `vitest.config.ts` with `test.coverage.include: ['src/**/*.ts']` so untested files count, and `coverage.exclude` for `*.d.ts`, generated code, migrations, `main.ts`. Without a vitest or vite config the gate falls back to jest, so commit this file.
- `testTimeout` and `hookTimeout` set, `isolate: true` and the `forks` pool (the defaults), `restoreMocks: true`.

## 6. Layer map
```
src/domain/        pure: entities, value objects, errors; no node:*, no framework, no clock
src/application/   use cases + consumer-owned port interfaces (Clock, IdGen, repos)
src/adapters/inbound/{http,queue,cli}   routes, schema validation (zod), handlers
src/adapters/outbound/{postgres,http-client,kafka}
src/compose/       composition root: new X(deps)
src/main.ts        the only file reading process.env, starting the server, handling signals
```
Tests sit next to code as `*.test.ts`; integration and e2e under `test/`. `loops arch` reads import statements (ES, `import type`, `export from`, `import()`, `require`) with the relative-path resolver, so `tsconfig` `paths` aliases need `./`-relative imports or a dependency-cruiser config. NestJS: `domain/` and `application/` stay decorator-free (`frameworks.md`).

## 7. Footguns
1. Floating promise: `await`, or `void p.catch(log)`.
2. `forEach(async ...)` does not await: `Promise.all` with a cap, or `for...of`.
3. Blocking the event loop: no `*Sync`, big `JSON.parse`, or backtracking regex on request paths.
4. `req.body as T` is not validation: `Schema.parse` (zod or valibot) at the edge.
5. `strict` without `noUncheckedIndexedAccess`: `arr[i]` is typed as present.
6. `npm install` or install scripts in CI: `npm ci --ignore-scripts`.
7. Prototype pollution: no recursive merge or `Object.assign` of input; use `Map`.
8. Check-then-act across `await`: DB unique constraint or per-key lock.
9. `CMD npm start` loses SIGTERM: exec-form `node dist/main.js`, graceful `server.close()`.
10. Type stripping is not type checking and `enum` breaks it: keep `tsc --noEmit` in the gate.

## 8. Load on demand
- `repo.md`: layout, lockfile, toolchain pins, `.npmrc`, CI order, supply chain.
- `architecture.md`: layer mapping, imports, dependency-cruiser, DI, size limits.
- `idioms.md`: errors, null handling, unions, ESM, dates and money.
- `testing.md`: vitest, fakes, Testcontainers, coverage, Bun and Deno recipes.
- `concurrency.md`: event loop, `await` interleaving, AbortSignal, shutdown, the race substitute.
- `security.md`: validation, injection, supply chain, secrets, headers.
- `data-api.md`: persistence, APIs, observability, config and packaging.
- `frameworks.md`: NestJS, Fastify, Hono and Express fit.
