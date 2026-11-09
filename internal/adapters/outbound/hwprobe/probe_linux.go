//go:build linux

package hwprobe

import (
	"context"

	"github.com/arxdsilva/ai-loops/internal/domain/hw"
)

func probeOS(_ context.Context, f *hw.Facts) {
	probeLinuxAt(f, "/proc", "/sys/fs/cgroup", "/.dockerenv")
}
