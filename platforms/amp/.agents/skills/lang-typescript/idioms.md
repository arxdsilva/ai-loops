# TypeScript idioms digest

Rules: RULES.md IDIOM. Enforcement is mostly lint (`typescript-eslint`) and `tsc`; the gate runs both.

## Errors (TS-IDIOM-01, 02)
- Throw `Error` subclasses only, with `cause`: `new Error('x', {cause: e})`. `catch (e)` is `unknown`: narrow with `instanceof` or a guard. Never an empty `catch {}`.
- Expected failures are a discriminated-union `Result` in domain and application; map to HTTP at the edge.

## Types (TS-IDIOM-03 to 07)
- Absence is `T | undefined`; use `??` and `?.`, not `||` (which swallows `0` and `''`). `null` only as a wire or DB value at the adapter.
- No `!` and no `as` on external data: validate with zod or valibot. No `any`: use `unknown` at untrusted boundaries. `@ts-expect-error` with a reason; `@ts-ignore` is banned.
- Prefer `readonly`, `ReadonlyArray`, `as const`, immutable updates. Mutating shared module state is a bug.
- States are discriminated unions with an exhaustive `switch` ending in a `never` assertion. Avoid `enum` (runtime code; breaks type stripping): use string-literal unions or `as const` objects.

## Resources and equality (TS-IDIOM-08, 09)
- `using` / `await using` (TS 5.2) or `try/finally`; close DB clients, streams and servers; `stream.pipeline` (promises), not `.pipe`.
- `===` always; structural equality with `node:util` `isDeepStrictEqual`, never `JSON.stringify`; `Map` and `Set` for non-string keys.

## Time and money (TS-IDIOM-10)
Instants are UTC ISO-8601 or epoch ms. `Temporal` status in Node was not verified as of 2026-11: use `date-fns` or `luxon` unless the repo already uses Temporal. Money is integer minor units, `bigint` or `decimal.js`, never `number` arithmetic. Clock comes from a port (`skills/consistency`).

## Style and modules (TS-IDIOM-11, 12, 14)
- Google TS style naming: `UpperCamelCase` types, `lowerCamelCase` values, `CONSTANT_CASE` constants, no `I` prefix, named exports over default. TSDoc on public API.
- ESM: `"type": "module"`, `"module": "nodenext"`, explicit extensions in relative imports, `import.meta.dirname` not `__dirname`. Dual CJS/ESM only for libraries, via the `exports` map.
- Decorators, `Proxy`, `eval`, `new Function`, monkey-patching built-ins: only inside framework adapters.

## Known bugs (TS-IDIOM-13)
`parseInt` without radix, `Array.sort()` without a comparator, `for..in` over arrays, `JSON.parse` typed as `any`, numbers above 2^53, `forEach(async ...)`.

## Performance habits (TS-PERF-01 to 05)
- Benchmarks in `*.bench.ts` run by `vitest bench` (not in `*.test.ts`), or tinybench/mitata; never `Date.now()` loops. Results are noisy: compare to a saved baseline.
- Profile with `node --cpu-prof`, `--heap-prof`, clinic.js or 0x. Size `--max-old-space-size` under the container limit; stream large bodies.
- No sync I/O, N+1 awaits, large `JSON.stringify` per request, unbounded caches, regex on untrusted input in hot paths. Build to JS for production; no `ts-node` or `tsx` in prod images.
