package pricing

import (
	"errors"
	"testing"
)

func TestSubtotal_SumsQtyTimesPrice(t *testing.T) {
	got, err := Subtotal([]Line{{SKU: "a", Qty: 2, UnitCents: 150}, {SKU: "b", Qty: 1, UnitCents: 99}})
	if err != nil || got != 399 {
		t.Fatalf("Subtotal = %d, %v; want 399, nil", got, err)
	}
}

func TestSubtotal_RejectsInvalidLines(t *testing.T) {
	for name, l := range map[string]Line{
		"no sku":         {Qty: 1, UnitCents: 1},
		"zero qty":       {SKU: "a", UnitCents: 1},
		"negative price": {SKU: "a", Qty: 1, UnitCents: -1},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Subtotal([]Line{l}); !errors.Is(err, ErrInvalidLine) {
				t.Fatalf("err = %v, want ErrInvalidLine", err)
			}
		})
	}
}
