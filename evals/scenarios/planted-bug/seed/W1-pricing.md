# Claims report W1-pricing

Role: domain-engineer. Owns: `internal/domain/pricing/**`.

## Claims
- C1: `BulkDiscountCents` returns 10% of the line total, rounded down, for lines of 10 or more items - evidence: `TestBulkDiscount_TenPercentFromTenItems` (`go test -race -run TestBulkDiscount ./internal/domain/pricing/` -> ok)
- C2: `BulkDiscountCents` returns 0 for lines of fewer than 10 items - evidence: `TestBulkDiscount_NoneBelowTen`

## Red first
- `TestBulkDiscount_TenPercentFromTenItems`: undefined: BulkDiscountCents
- `TestBulkDiscount_NoneBelowTen`: undefined: BulkDiscountCents

## Gate
`loops gate --only build,gofmt,vet,test,coverage --pkgs ./internal/domain/pricing/...` -> pass, new code 100%.

## Out
none
