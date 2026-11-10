# Go frameworks digest

Rules: `specs/languages/go/RULES.md §48`, `§41`. Research IDs `GO-FW-01..06`. Project rules win: when the project already uses a framework, follow it and record the override (`loops run override`).

## Std first (GO-FW-06)
Prefer a new standard-library capability to a third party where they are equal: `log/slog`, `ServeMux` routing, `slices`/`maps`/`iter`, `os.Root`, `errors.AsType`, `encoding/json/v2` (default on 1.27), and `uuid` (1.27, import path `"uuid"`: `uuid.New`, `uuid.NewV7`, `uuid.Parse`). Re-check against the newest release before adding a dependency (RULES §48 order: stdlib, then a library, then our own).

## Landscape (GO-FW-01)
`net/http` with chi, gin, echo, Fiber, gorilla/mux, gRPC and Connect. Frameworks are optional in Go. No authoritative 2026 usage ranking was found, so treat the list as qualitative.

## Web (GO-FW-02)
- chi: stdlib-compatible; routers and middleware in the inbound adapter; `chi.URLParam` or `r.PathValue`.
- gin and echo: handler signatures differ from `http.Handler`; wrap them so use cases never see `*gin.Context` or `echo.Context`.
- Connect: handlers implement generated interfaces; one adapter per service.

## Data (GO-FW-03)
GORM (`AutoMigrate` is not a migration strategy), ent (generated), sqlc, bun. SQL-first with sqlc is the loops default (`data-api.md`); keep any ORM's model types inside the adapter.

## Messaging (GO-FW-05)
Watermill, Benthos and broker clients are adapters; the domain does not know them.

## Footguns (GO-FW-04)
- `gin.Context` leaking into domain or application.
- GORM `AutoMigrate` in production.
- Global `http.DefaultServeMux`, registered as a side effect of importing `net/http/pprof`.
- `fx` or `dig` hiding the object graph (RULES §5; wire by hand in `compose`).
- gorilla/mux method mismatches returning 405 or 404 unexpectedly.

## Adapter rule
Framework types live only in the adapter that owns them and never cross a port (`architecture.md`; `loops arch` enforces import direction and third-party imports in core).
