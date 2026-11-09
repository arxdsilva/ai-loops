// Package app wires the binary.
package app

import (
	"example.com/clean/internal/adapters/outbound/db"
	"example.com/clean/internal/application/orders"
)

// Main builds and runs the app.
func Main(args []string) int {
	_ = orders.New(db.DB{})
	return len(args)
}
