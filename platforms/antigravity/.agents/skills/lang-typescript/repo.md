# TypeScript repo and toolchain digest

Rules: `specs/languages/typescript/RULES.md` REPO and TOOL. Universal repo hygiene: `specs/REPO_PRACTICES.md`.

## Layout (TS-REPO-01, 02, 11)
```
package.json  (type: module, engines, packageManager, scripts)   one lockfile
.nvmrc  .npmrc  tsconfig.json  tsconfig.build.json
eslint.config.mjs | biome.json   .prettierrc.json   vitest.config.ts   .dependency-cruiser.cjs
src/ (domain application adapters compose main.ts)   test/   migrations/   Dockerfile  .dockerignore
dist/ coverage/ node_modules/   git-ignored
```
- Sources in `src/`, unit tests colocated `*.test.ts`, integration and e2e in `test/`, output in `dist/`.
- Monorepo: native workspaces plus TS project references (`composite`, `tsc -b`). Turborepo or Nx only for 3+ packages with real cache needs (TS-REPO-02, 03).
- Ignore `node_modules/`, `dist/`, `coverage/`, `*.tsbuildinfo`, `.turbo/`, `.env*` (keep `.env.example`); add `.editorconfig` and `.gitattributes` (`* text=auto eol=lf`).

## Package manager and lockfile (TS-REPO-05, 06)
- One manager, `"packageManager": "pnpm@x.y.z"`, exactly one committed lockfile. Node 26 does not bundle Corepack, so `packageManager` is honoured only where the manager enforces it (pnpm, npm 11).
- Frozen installs only in CI and in the gate: `npm ci`, `pnpm install --frozen-lockfile`, `yarn install --immutable`, `bun install --frozen-lockfile`; always `--ignore-scripts` (TS-SEC-06). `npm install` mutates the lock and hides drift.
- Pin direct deps exactly (`save-exact=true`) or rely on the lock plus Renovate.

## Toolchain pins (TS-REPO-10, TS-TOOL-01)
- Run on Active or Maintenance LTS: Node 26 (Active), 24 and 22 (Maintenance). 20 and 25 are EOL; 27 is alpha.
- Pin three ways, which must agree: `"engines": {"node": ">=24 <27"}` with `engine-strict=true` in `.npmrc`, `.nvmrc` or `.node-version`, and the Docker base tag. Use fnm, mise or volta as the manager.
- TypeScript: typescript-eslint supports `>=4.8.4 <6.1.0` (as of 2026-11), so a repo that lints with it pins `typescript` to a supported 5.x or 6.0; TS 7 (native port) can run `tsc` only until support lands (TS-TOOL-03).

## tsconfig (TS-TOOL-06, 07, 10)
- `strict`, `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`, `noImplicitOverride`, `verbatimModuleSyntax`, `noFallthroughCasesInSwitch`, `skipLibCheck`; `module: nodenext`.
- `tsc --noEmit` is its own gate step; esbuild, swc, vitest and Node strip types without checking.
- Running `.ts` directly (Node type stripping) is for scripts and tests, with `erasableSyntaxOnly` and `.ts` extensions in imports; services still build to `dist/`.

## Lint and format (TS-TOOL-02, 03, 04, 05)
- Prettier plus ESLint flat config with `typescript-eslint` `strictTypeChecked` and `projectService: true`, `--max-warnings=0`; or Biome alone (never both on the same files). Biome's type-aware coverage (for example floating promises) is unverified: keep a floating-promise check somewhere.
- Enable `no-floating-promises`, `no-misused-promises`, `await-thenable`, `no-explicit-any`, `consistent-type-imports`, `eqeqeq`.

## CI order (TS-REPO-12)
Frozen install, format check, lint, `tsc --noEmit`, build, tests with coverage, audit (`npm audit`, OSV), package. Cache the package-manager store keyed on the lockfile hash, not `node_modules`. Matrix over supported Node majors.

## Updates and publishing (TS-REPO-07, 08, 09, 13)
- Renovate or Dependabot, grouped, with a release cooldown (Renovate and pnpm `minimumReleaseAge`, npm `min-release-age`). `knip` finds unused deps and exports.
- Libraries: SemVer, Changesets, `npm publish --provenance` from Actions with `id-token: write`, a `files` allowlist, `npm publish --dry-run`.
- Conventional Commits; Changesets or release-please writes changelogs and tags.

## Runtimes (TS-OPS-08)
Node is the default. Bun (`bun.lock`, `bun test`) and Deno (`deno.json`) change install, test and coverage commands; choose them deliberately and use their variants or overrides.
