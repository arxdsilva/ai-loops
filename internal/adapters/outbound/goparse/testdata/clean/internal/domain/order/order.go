// Package order is a pure domain package.
package order

import "strings"

// Order is an order.
type Order struct{ ID string }

// Key normalises the id.
func (o Order) Key() string { return strings.ToLower(o.ID) }
