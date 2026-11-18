# Sales tax

Add `pricing.TaxCents(subtotalCents int64, rateBasisPoints int) int64`: the tax on a subtotal at a rate in basis points (`725` = 7.25%), rounded half up to the cent.

- AC-1: `TaxCents(1000, 725)` is `73`; `TaxCents(200, 725)` is `15` (14.5 rounds up); `TaxCents(0, 725)` is `0`.
- AC-2: a negative subtotal or a negative rate returns `0` (no panic, no error).

Put the function in `internal/domain/pricing/tax.go` and its tests in `internal/domain/pricing/tax_test.go`.
