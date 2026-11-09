package sim

import (
	"math/rand"
	r2 "math/rand/v2"
	clock "time"
)

// Step is impure on purpose.
func Step() int {
	_ = clock.Now()
	now := clock.Now
	_ = now
	_ = clock.Second
	go Step()
	g := rand.New(rand.NewSource(1))
	_ = g.Intn(3)
	_ = r2.New(r2.NewPCG(1, 2))
	return rand.Intn(3) + r2.N[int](3)
}

var draw = rand.Intn

// Values takes rand functions as values; a type reference is not a draw.
func Values() float64 {
	f := r2.Float64
	var _ *rand.Rand
	return f() + float64(draw(2))
}
