# Clojure frameworks

Digest of CLJ-FW. Project conventions win over loops layering when they conflict; log the override. Rules: `specs/languages/clojure/RULES.md`.

## Choosing (CLJ-FW-01)
- Clojure favours libraries: Ring with Jetty or http-kit, Reitit (data-driven routes), Pedestal (interceptors), Compojure in older code. No authoritative usage figures exist; follow what the repo already uses.

## Conventions (CLJ-FW-02/03)
- Reitit: data routes and coercion live in the inbound adapter. Pedestal interceptors are adapters and hold no domain logic.
- Handlers are thin map-to-map functions calling an app-layer function with an explicit deps map.
- Do not put the whole Component/Integrant system into request maps; inject only the needed collaborators through closures or middleware. Handlers never reach into `system` through a global var.

## Footguns (CLJ-FW-04)
- Mount's namespace-level state blocks independent test systems.
- `wrap-params` must precede `wrap-keyword-params` and body parsers.
- `ring-json` keywordizing untrusted keys interns keywords.
- Reitit middleware order is per-route data, not global.
- Pedestal `:enter`/`:leave` run in reverse order on leave.

## Lifecycle and monorepos (CLJ-ARCH-04, CLJ-REPO-15)
- Integrant is the default composition root; Component for teams already on it; Mount only for small apps.
- Polylith: components behind an `interface` namespace, bases for entry points, `poly check` in CI.
