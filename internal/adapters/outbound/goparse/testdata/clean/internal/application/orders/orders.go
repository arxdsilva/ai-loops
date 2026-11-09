// Package orders is the order use case.
package orders

import "example.com/clean/internal/domain/order"

// Store loads orders.
type Store interface {
	Load(id string) (order.Order, error)
}

// Service runs the use case.
type Service struct{ store Store }

// New builds a Service.
func New(s Store) *Service { return &Service{store: s} }
