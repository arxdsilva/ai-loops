package run

import (
	"math/rand"
	"time"
)

// Tick is allowed to be impure outside the domain.
func Tick() int {
	_ = time.Now()
	go Tick()
	return rand.Intn(3)
}
