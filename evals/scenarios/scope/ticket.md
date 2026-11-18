# Price formatting and a health check

Two independent changes in two layers; neither depends on the other.

- AC-1 (domain): add `pricing.FormatCents(cents int64) string` in `internal/domain/pricing`, formatting cents as dollars with two decimals and a leading `$` (`1234` -> `$12.34`, `5` -> `$0.05`, `-250` -> `-$2.50`).
- AC-2 (HTTP adapter): `GET /healthz` on the handler in `internal/adapters/inbound/httpapi` returns `200` with the body `ok`. Other routes are unchanged.
