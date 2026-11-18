// Package pricing holds the shop's pure pricing rules. Amounts are integer
// cents.
package pricing

import "errors"

// ErrInvalidLine is returned for a line with no SKU, a non-positive
// quantity or a negative price.
var ErrInvalidLine = errors.New("pricing: invalid line")

// Line is one item of an order.
type Line struct {
	SKU       string
	Qty       int
	UnitCents int64
}

// Validate reports whether l can be priced.
func (l Line) Validate() error {
	if l.SKU == "" || l.Qty <= 0 || l.UnitCents < 0 {
		return ErrInvalidLine
	}
	return nil
}

// Subtotal returns the sum of quantity times unit price over lines.
func Subtotal(lines []Line) (int64, error) {
	var sum int64
	for _, l := range lines {
		if err := l.Validate(); err != nil {
			return 0, err
		}
		sum += int64(l.Qty) * l.UnitCents
	}
	return sum, nil
}
