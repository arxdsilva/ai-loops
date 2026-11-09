package state

import (
	"sync"
	"sync/atomic"
)

var counter int

var assigned = map[string]int{}

var shadowed int

var (
	mu    sync.Mutex
	cache map[string]int
)

var hits atomic.Int64

var _ Store = (*mem)(nil)

var readOnly = []string{"a"}

var ptr = new(int)

// Store gets.
type Store interface{ Get() int }

type mem struct{ n int }

func (m *mem) Get() int { return m.n }

// Put stores.
func Put(k string, v int) {
	assigned[k] = v
	mu.Lock()
	cache = map[string]int{k: v}
	mu.Unlock()
	hits.Add(1)
	_ = readOnly[0]
	_ = shadowed
}

// Shadow shadows a global with a parameter.
func Shadow(shadowed int) int {
	shadowed = 3
	return shadowed
}

// Local shadows a global with a local.
func Local() {
	var x struct{ f int }
	x.f = 1
	readOnly := []string{}
	readOnly = append(readOnly, "b")
	_ = readOnly
}

// Set writes through a global pointer.
func Set() {
	*ptr = 1
}
