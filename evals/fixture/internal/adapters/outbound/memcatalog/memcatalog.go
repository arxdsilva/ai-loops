// Package memcatalog is an in-memory checkout.Catalog.
package memcatalog

import (
	"context"
	"errors"
)

// ErrNotFound is returned for an unknown SKU.
var ErrNotFound = errors.New("memcatalog: sku not found")

// Catalog maps SKUs to unit prices in cents. It is read-only after
// construction, so it is safe for concurrent use.
type Catalog struct{ prices map[string]int64 }

// New returns a catalog holding a copy of prices.
func New(prices map[string]int64) *Catalog {
	c := &Catalog{prices: make(map[string]int64, len(prices))}
	for k, v := range prices {
		c.prices[k] = v
	}
	return c
}

// PriceOf returns the unit price of sku.
func (c *Catalog) PriceOf(_ context.Context, sku string) (int64, error) {
	p, ok := c.prices[sku]
	if !ok {
		return 0, ErrNotFound
	}
	return p, nil
}
