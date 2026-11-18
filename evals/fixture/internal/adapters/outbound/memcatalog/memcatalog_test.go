package memcatalog

import (
	"context"
	"errors"
	"testing"
)

func TestPriceOf(t *testing.T) {
	src := map[string]int64{"a": 150}
	c := New(src)
	src["a"] = 1 // the catalog keeps its own copy
	if p, err := c.PriceOf(context.Background(), "a"); err != nil || p != 150 {
		t.Fatalf("PriceOf(a) = %d, %v; want 150, nil", p, err)
	}
	if _, err := c.PriceOf(context.Background(), "zz"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("PriceOf(zz) err = %v, want ErrNotFound", err)
	}
}
