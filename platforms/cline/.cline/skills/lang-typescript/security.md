# TypeScript security digest

Rules: RULES.md SEC.

## Validation at the edge (TS-SEC-05)
Types vanish at runtime. Validate every inbound payload, env var and queue message in the adapter with zod or valibot (`schema.parse` or `safeParse`), derive types with `z.infer`, and pass only parsed values inward. Fastify uses JSON Schema (ajv); NestJS uses `class-validator` with `ValidationPipe({whitelist: true, forbidNonWhitelisted: true})`. `req.body as T` is a lie.

## Injection and unsafe parsing (TS-SEC-01, 02)
- Parameterised queries only (`pg` `$1`, Prisma, Drizzle, Kysely builders); never template strings from input. `execFile` or `spawn` with an argv array; never `exec` or `shell: true` on untrusted data.
- Prototype pollution: schema-validate JSON, reject `__proto__` and `constructor` keys, no recursive merge or `Object.assign` of input, use `Map` or `Object.create(null)`. No `eval`, no `vm` as a sandbox, no `node-serialize`, no unsafe YAML tags; disable XXE in XML parsers.

## Web hardening (TS-SEC-03)
`helmet` (`@fastify/helmet`, Hono `secureHeaders`), explicit CORS origin allowlist, CSRF tokens or SameSite cookies, SSRF allowlist without redirects to private ranges, path canonicalisation (`path.resolve` then prefix check).

## Secrets and crypto (TS-SEC-04, 10)
Read secrets from env or a vault only in `main.ts` or the config adapter; never commit `.env`. argon2id (`argon2`) or `crypto.scrypt` for passwords; `crypto.randomBytes` or `randomUUID` for tokens, never `Math.random`; `crypto.timingSafeEqual` to compare secrets. Never log secrets or full bodies: pino `redact`.

## Supply chain (TS-SEC-06, 07)
- Install scripts are untrusted: `npm ci --ignore-scripts` (or `ignore-scripts=true`); pnpm blocks dependency builds by default and uses `allowBuilds`, `strictDepBuilds`, `blockExoticSubdeps`, `minimumReleaseAge` and `trustPolicy: no-downgrade`. `npm audit signatures` verifies provenance.
- Review lockfile diffs, watch for typosquats and new maintainers, use scoped registries. Never install a package an agent suggests without checking it exists and how old it is: hallucinated names get squatted.
- Audit: `npm audit --audit-level=high --omit=dev` (pnpm `audit --prod`) and `osv-scanner scan source -r .`, both off by default in the gate (`loops gate --vuln`); `npm sbom --sbom-format cyclonedx` for an SBOM (TS-TOOL-08).

## SAST and runtime (TS-SEC-08, 09)
Semgrep (`semgrep scan --config p/typescript --config p/nodejs`) or CodeQL; `eslint-plugin-security` is a weak supplement. Node's permission model (`--permission --allow-fs-read=...`) can limit damage, but flags change between releases: check the pinned version's docs.

## Review checklist (TS-DOC-03)
`any`, `as` or `!` introductions, floating promises, missing edge validation, unbounded queries, blocking calls, new dependencies and install scripts, lockfile diff, secrets.
