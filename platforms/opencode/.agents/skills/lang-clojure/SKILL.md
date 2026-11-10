---
name: lang-clojure
description: Use when writing, reviewing, testing or setting up Clojure backend code in a loops run (the clojure language pack) - toolchain, layout, race policy, coverage, idioms and footguns.
---

# Clojure backend (JVM, deps.edn)

Applies when `loops lang detect` reports `clojure` (`deps.edn`, `project.clj` or `workspace.edn`). Rules: `specs/languages/clojure/RULES.md` (IDs `CLJ-<SEC>-NN`). Pack data: `specs/languages/clojure/pack.json` (`loops lang show clojure`). Layering and SOLID are language-neutral: `specs/ARCHITECTURE.md`, `specs/SOLID.md`.

## Project rules first

The project's own config wins over this skill: `deps.edn` aliases, `.clj-kondo/config.edn`, `.cljfmt.edn`, `tests.edn`, `.editorconfig`, framework conventions (Ring/Reitit, Pedestal, Integrant, Component, Polylith). When you follow one over a loops rule, record it: `loops run override --rule "<loops rule>" --project "<project rule, file:line>" --note "<what you did>"` (ROLES.md "Project rules first"). The practices-researcher reads `config_files` from the pack.

## Toolchain the gate runs

`loops gate` runs these in order; while iterating run a single one. `deps.edn` is the base; a project with only `project.clj` gets the `lein` variant (`lein deps`, `lein cljfmt check`, `lein cloverage`, `lein kaocha`).

| Check | Command | Notes |
|---|---|---|
| deps (required) | `clojure -P -M:test` | prefetch only; never edits `deps.edn` |
| cljfmt (required if `.cljfmt.edn`) | `clojure -Tcljfmt check` | install: `clojure -Ttools install-latest :lib io.github.weavejester/cljfmt :as cljfmt` |
| clj-kondo (required) | `clj-kondo --lint src test --parallel` | exit 2 = warnings, 3 = errors, both fail (`CLJ-TOOL-03`) |
| poly (Polylith) | `clojure -M:poly check` | when `workspace.edn` exists |
| arch (optional) | `loops arch` | scans `ns` forms, `-` to `_` resolver |
| test (required) | `clojure -M:cov -o {coverage_dir}` | cloverage on `clojure.test`; lcov at `{coverage_dir}/lcov.info` |
| test-repeat (required) | `clojure -M:test --randomize --seed {seed}`, 3 times | race substitute, see below |
| coverage (required) | loops parses the lcov file | new code at least 70% |
| nvd (off by default) | `clojure -Tnvd nvd.task/check :classpath ...` | `--vuln`; needs `NVD_API_TOKEN` and the `nvd` tool installed |

Narrow while working: `clojure -M:test --focus my.ns-test/my-test`, `--fail-fast`, or a REPL `(clojure.test/run-tests 'my.ns-test)` (`CLJ-TEST-11`). Never run the full gate as an implementer; the lead and QA do (ROLES.md).
Caches (D15): keep `~/.m2` and `~/.gitlibs` shared; the gate scopes only the JVM tmp dir (`JAVA_TOOL_OPTIONS=-Djava.io.tmpdir`). `CLJ_CACHE` does nothing with a project `deps.edn`: `.cpcache/` is cleaned after the run instead (`CLJ-OPS-05/06`).

## Race policy (WORKFLOW §6)

The JVM and Clojure have no race detector, so the pack's mode is `none`. The gate still requires the substitute and never skips it:
- `test-repeat` runs the Kaocha suite three times with `--randomize --seed {seed}`; each run prints a new seed. Any failure in any run fails the gate; reproduce with `--seed N`.
- Never use `--no-randomize`, retries or a fixed seed in the gate.
- Shared state lives in one `atom` (or `ref`); `swap!` and `dosync` functions are pure (`CLJ-CONC-01`). Tests tagged `^:concurrency` are repeated 20 times in review (`--focus-meta :concurrency`, `CLJ-CONC-08`).
- Missing `:test` alias or `tests.edn` makes the gate fail with the snippet in `testing.md`.

## Coverage recipe and wave-0 prerequisites (D13)

The architect or `clojure-engineer` adds these in wave 0 and logs each as an override if the project already differs:
- `:test` alias: Kaocha (`lambdaisland/kaocha`) with `:main-opts ["-m" "kaocha.runner"]`, plus `tests.edn` (`#kaocha/v1`, `:ns-patterns ["-test$"]`, randomize on).
- `:cov` alias: cloverage with `:main-opts ["-m" "cloverage.coverage" "-p" "src" "-s" "test" "--lcov" "--no-html"]` and `:extra-paths ["test"]`. loops appends `-o {coverage_dir}`; do not put `-o` in the alias.
- Exact version pins everywhere (`CLJ-REPO-05`), `.clj-kondo/config.edn`, `.cljfmt.edn`, `.gitignore` (`.cpcache/`, `target/`).

Details and snippets: `testing.md`, `repo.md`.

## How the loops layers map

```
src/my_service/domain/       pure maps and functions; requires only clojure.* and domain
src/my_service/app/          use cases (deps map as first arg) + ports (protocols, consumer-owned)
src/my_service/adapters/     inbound (Ring/Reitit handlers) and outbound (next.jdbc, hato); depend on app
src/my_service/system.clj    composition root (Integrant); core.clj holds -main
test/my_service/             mirrors src; files end in _test.clj
```
Domain stays pure: no `slurp`, `Instant/now`, `rand`, `future` or `next.jdbc` (`CLJ-ARCH-06`); time and ids come in as data or ports. Polylith repos: components expose an `interface` namespace and `poly check` enforces it (`CLJ-ARCH-03`).

## Ten footguns

1. Reflection: `(set! *warn-on-reflection* true)` per namespace and type hints.
2. Lazy sequences escaping `with-open` or a transaction: `doall`, `into`, `reduce` inside.
3. Side effects in `swap!`/`dosync`: they retry; pure function first, effect after.
4. `map` for side effects: `run!` or `doseq`.
5. Blocking calls inside a core.async `go` block: `a/thread` or `a/io-thread`, bounded channels.
6. `read-string`/`eval` on input: `clojure.edn/read-string` or JSON.
7. `nil` vs `false` with `or`: `(get cfg :enabled true)`.
8. `RELEASE`, branches or ranges in `deps.edn`: exact versions and full git SHAs.
9. Namespace-level `(def conn ...)` state and Mount globals: lifecycle map via Integrant.
10. Mixed numerics (`(= 1 1.0)`, long overflow, doubles for money): `+'`, `bigdec`, normalise keys.

## Load on demand

- `repo.md`: layout, namespaces, deps pinning, tools.build, Polylith, CI, release.
- `architecture.md`: namespaces as layers, ports, DI with Integrant, purity, kondo boundaries.
- `idioms.md`: errors as data, nil, immutability, specs, resources, laziness, reflection.
- `testing.md`: Kaocha, `tests.edn`, cloverage `:cov`, fakes, test.check, flakiness.
- `concurrency.md`: atoms, refs, agents, futures, core.async, executors, the race substitute.
- `security.md`: injection, deserialization, secrets, crypto, supply chain, SAST.
- `data-api.md`: next.jdbc, migrations, Ring/Reitit, outbound calls, observability, ops.
- `frameworks.md`: Ring, Reitit, Pedestal, Integrant, Component, Mount, Polylith conventions.
