package checkout

import (
	"context"
	"errors"
	"testing"
)

type fakeCatalog map[string]int64

var errUnknown = errors.New("unknown sku")

func (f fakeCatalog) PriceOf(_ context.Context, sku string) (int64, error) {
	p, ok := f[sku]
	if !ok {
		return 0, errUnknown
	}
	return p, nil
}

func TestQuote_PricesItemsFromTheCatalog(t *testing.T) {
	svc := New(fakeCatalog{"a": 150, "b": 99})
	q, err := svc.Quote(context.Background(), []Item{{SKU: "a", Qty: 2}, {SKU: "b", Qty: 1}})
	if err != nil || q.SubtotalCents != 399 || len(q.Lines) != 2 {
		t.Fatalf("Quote = %+v, %v; want subtotal 399 over 2 lines", q, err)
	}
}

func TestQuote_FailsOnUnknownSKU(t *testing.T) {
	_, err := New(fakeCatalog{}).Quote(context.Background(), []Item{{SKU: "x", Qty: 1}})
	if !errors.Is(err, errUnknown) {
		t.Fatalf("err = %v, want errUnknown", err)
	}
}
