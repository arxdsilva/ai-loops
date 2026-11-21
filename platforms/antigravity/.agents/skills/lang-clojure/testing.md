# Clojure testing and coverage

Digest of CLJ-TEST. Universal testing rules: `specs/TESTING.md` (when present); skill `loops:testing` has the loops flow. Rules: `specs/languages/clojure/RULES.md`.

## Runner and layout (CLJ-TEST-01/02)
- `clojure.test` (`deftest`, `is`, `testing`, `are`) run by Kaocha through the `:test` alias. `matcher-combinators` for structural assertions.
- `test/` mirrors `src/`; namespaces end in `-test`, files in `_test.clj`/`_test.cljc`; `:ns-patterns ["-test$"]`. Split `:unit` and `:integration` suites; select with `--focus` or `--skip-meta`.

## Wave-0 prerequisites (D13)
`deps.edn`:
```clojure
:aliases
{:test {:extra-paths ["test"]
        :extra-deps {lambdaisland/kaocha {:mvn/version "<pin>"}}
        :main-opts ["-m" "kaocha.runner"]}
 :cov  {:extra-paths ["test"]
        :extra-deps {cloverage/cloverage {:mvn/version "<pin>"}}
        :main-opts ["-m" "cloverage.coverage" "-p" "src" "-s" "test" "--lcov" "--no-html"]}}
```
`tests.edn`: `#kaocha/v1 {:tests [{:id :unit :test-paths ["test"] :ns-patterns ["-test$"]}] :plugins [:kaocha.plugin/randomize]}`. Do not put `-o` in `:cov`; the gate appends `-o {coverage_dir}`. Leiningen: the `lein-cloverage` plugin and a `kaocha` alias.

## Coverage (CLJ-TEST-06/07)
- `clojure -M:cov -o {coverage_dir}` writes `{coverage_dir}/lcov.info` (relative `SF:src/...`, `DA:` records). It runs `clojure.test`, not Kaocha.
- New-code coverage at least 70%: loops intersects the `DA:` lines with the diff. Exclude only generated namespaces (`--ns-exclude-regex`).

## Substitute for the race detector (CLJ-CONC-08, CLJ-TEST-10)
- `test-repeat`: `clojure -M:test --randomize --seed {seed}`, 3 runs. Kaocha prints the seed; reproduce with `--seed N`; `--no-randomize` only to debug locally, never in the gate.
- Inject clock, rand and uuid through ports; put timeouts on futures and promises in tests; never share atoms across tests.

## Style and doubles (CLJ-TEST-03/04/05)
- `are` for tables; fixtures in `test/<ns>/fixtures.clj`; `use-fixtures :each` for resets.
- Fakes (a `reify` or record of the port protocol, or a map of fns), not mocking libraries. `with-redefs` is global and thread-unsafe: rare, never in parallel tests.
- Integration tests use real Postgres (Testcontainers via `clj-test-containers`), a rolled-back transaction or fresh schema per test, and Integrant `halt!`/`init` to reset the system.

## Properties and speed (CLJ-TEST-08/09/11)
- `test.check` (`defspec`, `prop/for-all`) for parsers, pure rules and idempotency; at least 100 iterations in CI; seed with `:seed` to reproduce. Mutation testing is optional, never gated.
- REPL loop: `(clojure.test/run-tests 'my.ns-test)`, `--focus my.ns-test/my-test`, `--watch`, `--fail-fast`.
