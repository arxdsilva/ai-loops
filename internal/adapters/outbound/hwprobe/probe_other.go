//go:build !linux && !darwin && !windows

package hwprobe

import (
	"context"
	"runtime"

	"github.com/arxdsilva/ai-loops/internal/domain/hw"
)

func probeOS(_ context.Context, f *hw.Facts) {
	f.Notes = append(f.Notes, "ram/load: no probe for "+runtime.GOOS)
}
