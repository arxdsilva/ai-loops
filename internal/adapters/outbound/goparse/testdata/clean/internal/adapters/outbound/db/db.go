// Package db stores orders in files.
package db

import (
	"os"

	"example.com/clean/internal/domain/order"
)

// DB is a file store.
type DB struct{ dir string }

// Load reads one order.
func (d DB) Load(id string) (order.Order, error) {
	_, err := os.ReadFile(d.dir + "/" + id)
	return order.Order{ID: id}, err
}
