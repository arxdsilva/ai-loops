# Clojure idioms

Digest of CLJ-IDIOM and CLJ-PERF. Rules: `specs/languages/clojure/RULES.md`.

## Errors (CLJ-IDIOM-01/02)
- Return data for expected failures (`{:error/type :not-found}`); throw `ex-info` with a data map for exceptional ones: `(throw (ex-info "msg" {:order-id id} cause))`.
- Catch at the adapter boundary and map `ex-data` `:type` to a status; log once there. No empty `catch`, no bare `(catch Exception _ nil)`.

## Values and nil (CLJ-IDIOM-03/04/09)
- `nil` and `false` are falsey; `()`, `[]`, `{}`, `""` are truthy. Emptiness is `(seq coll)`. Use `some?`/`nil?` when `false` is a legal value; `(get m k default)` over `(or ...)`.
- Persistent collections only; `assoc`/`update`/`assoc-in`; transients stay inside one function.
- `=` is value equality, `==` numeric only; `(= 1 1.0)` is false, so normalise key types.

## Data modelling (CLJ-IDIOM-05/06)
- Maps with namespaced keywords (`:order/id`); `defrecord` only for protocol dispatch. One schema library per codebase: malli for new HTTP services, spec for library contracts.

## Resources and laziness (CLJ-IDIOM-07/08)
- `with-open` for any `Closeable`; lifecycle `halt-key!` for pools. Never return a lazy seq from inside `with-open`, a transaction or a `binding`; use `reduce`, `transduce`, `run!`, `doseq`.

## Time and macros (CLJ-IDIOM-10/12)
- `java.time` only, always with an explicit zone; no `java.util.Date` or `SimpleDateFormat`.
- A function first; a macro only for control flow or a DSL, as a thin wrapper. Never `eval` or `read-string` on input.

## Style (CLJ-IDIOM-11/14)
- kebab-case; predicates end in `?`, side-effecting in `!`, conversions `x->y`, private `defn-`. Compose with `->`, `->>`, `cond->`; destructure params.

## Reflection and performance (CLJ-IDIOM-13, CLJ-PERF-01..05)
- `(set! *warn-on-reflection* true)` in every namespace; fix every warning with a type hint.
- Benchmark with criterium, never `time`; profile with clj-async-profiler or JFR; tune the JVM (`-XX:MaxRAMPercentage=75`).
- Avoid boxed math in hot loops, `apply str` in loops; prefer transducers over chained lazy `map`/`filter`; `pmap` only for CPU-bound, uniform work.
