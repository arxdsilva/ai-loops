package sim

import (
	"testing"
	"time"
)

func TestStep(t *testing.T) {
	_ = time.Now()
	go Step()
}
