# QA W1-pricing r1

## Claims
### C1: BulkDiscountCents gives 10% off from 10 items - FAILED
- Checked: read bulk.go, ran the cited test, probed Qty=10.
  - Repro: `go test -race -count=1 -run TestBulkBoundary ./internal/domain/pricing/` (snippet below)
  - Output (<=20 lines):
    ```
    bulk_test.go:9: BulkDiscountCents(Qty=10, 100c) = 0, want 100
    FAIL
    ```
  - Location: internal/domain/pricing/bulk.go:9
  - Expected vs actual: 100 / 0 (uses > instead of >=)
  - Suggested fix area: BulkDiscountCents
### C2: no discount below 10 items - VERIFIED
- Checked: TestBulkDiscount_NoneBelowTen; Qty=9 by reading.

Verdict: FAILED
