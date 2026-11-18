// Package checkout is the quote use case: it prices requested items with
// the catalog's unit prices.
package checkout

import (
	"context"
	"fmt"

	"example.com/shop/internal/domain/pricing"
)

// Catalog is the outbound port for unit prices.
type Catalog interface {
	PriceOf(ctx context.Context, sku string) (int64, error)
}

// Item is a requested SKU and quantity.
type Item struct {
	SKU string
	Qty int
}

// Quote is the priced order.
type Quote struct {
	Lines         []pricing.Line
	SubtotalCents int64
}

// Service quotes orders.
type Service struct{ catalog Catalog }

// New returns a Service over catalog.
func New(catalog Catalog) *Service { return &Service{catalog: catalog} }

// Quote prices items.
func (s *Service) Quote(ctx context.Context, items []Item) (Quote, error) {
	q := Quote{Lines: make([]pricing.Line, 0, len(items))}
	for _, it := range items {
		price, err := s.catalog.PriceOf(ctx, it.SKU)
		if err != nil {
			return Quote{}, fmt.Errorf("checkout: price of %q: %w", it.SKU, err)
		}
		q.Lines = append(q.Lines, pricing.Line{SKU: it.SKU, Qty: it.Qty, UnitCents: price})
	}
	sub, err := pricing.Subtotal(q.Lines)
	if err != nil {
		return Quote{}, err
	}
	q.SubtotalCents = sub
	return q, nil
}
