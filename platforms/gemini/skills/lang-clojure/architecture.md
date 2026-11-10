# Clojure architecture

Digest of CLJ-ARCH. Layering and SOLID are universal: `specs/ARCHITECTURE.md`, `specs/SOLID.md`. Rules: `specs/languages/clojure/RULES.md`.

## Layers are namespaces (CLJ-ARCH-01)
- `domain` requires only `clojure.*` and `domain`; `app` requires `domain` and its own port protocols; `adapters.*` require `app` and `domain`; only `system`/`core` require adapters.
- Directories are the layers, because Clojure has only namespaces.

## Enforcement (CLJ-ARCH-02/03)
- `loops arch` reads `(:require [a.b :as x])` and `(:import ...)` forms and resolves `a.b.c` to `src/a/b/c.clj(c)` with `-` to `_`. It does not resolve single-segment namespaces or prefix lists (`[a.b [c] [d]]`).
- clj-kondo `:discouraged-namespace` with `:config-in-ns` per layer is the cheapest static boundary (for example `my-service.domain.*` forbids `clojure.java.io`, `next.jdbc`, `ring.*`).
- Polylith: `clojure -M:poly check` (interface-only access between components).

## Wiring and ports (CLJ-ARCH-04/05/07)
- Compose in `system.clj`: Integrant by default (`init-key`, `halt-key!`, `#ig/ref`); Component when the team already uses it; Mount only for small apps. No service locator, no global atoms of dependencies.
- Use cases take a deps map first: `(place-order {:keys [repo clock]} cmd)`. Never `(def conn ...)` at namespace level.
- Ports are protocols defined by the consumer (`app/ports.clj`, 1-5 methods); multimethods for open dispatch on data; plain maps of functions for small seams.

## Purity (CLJ-ARCH-06)
- Domain: no `System/currentTimeMillis`, `Instant/now`, `rand`, `random-uuid`, `slurp`, `spit`, `future`, `swap!` on shared atoms. Pass time and ids as data. The pack lists these in `impure_calls`; `forbidden_in_domain` lists `clojure.java.io`, `next.jdbc`, `ring.`, `reitit.`, `clojure.core.async` and friends.

## Data and size (CLJ-ARCH-08/09/10)
- Maps with namespaced keys, validated at boundaries with malli or spec; `defrecord` only for protocol dispatch or performance.
- Functions under ~15 lines, namespaces under ~400 lines, at most 4 positional params; cycles are compile errors, so a cycle exposes a layering mistake.
- Modular monolith first (Polylith components in one project); split deployables on scaling, ownership or release need.
