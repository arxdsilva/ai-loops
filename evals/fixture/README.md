# shop

A tiny price-quote service, laid out hexagonally:

- `internal/domain/pricing`: pure pricing rules (amounts in cents).
- `internal/application/checkout`: the quote use case and its `Catalog` port.
- `internal/adapters/inbound/httpapi`: `POST /quote`.
- `internal/adapters/outbound/memcatalog`: an in-memory catalog.
- `cmd/shop`: wiring.

Tests: `go test -race ./...`.
