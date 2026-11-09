//go:build darwin

package hwprobe

import (
	"context"

	"github.com/arxdsilva/ai-loops/internal/domain/hw"
)

func probeOS(ctx context.Context, f *hw.Facts) {
	if out, err := run(ctx, "sysctl", "-n", "hw.memsize"); err != nil {
		f.Notes = append(f.Notes, "ram total: "+err.Error())
	} else if v, err := parseSysctlUint(out); err != nil {
		f.Notes = append(f.Notes, "ram total: "+err.Error())
	} else {
		f.TotalRAMBytes = v
	}
	if out, err := run(ctx, "vm_stat"); err != nil {
		f.Notes = append(f.Notes, "ram avail: "+err.Error())
	} else if v, err := parseVMStat(out); err != nil {
		f.Notes = append(f.Notes, "ram avail: "+err.Error())
	} else {
		f.AvailRAMBytes = v
	}
	if out, err := run(ctx, "sysctl", "-n", "vm.loadavg"); err != nil {
		f.Notes = append(f.Notes, "load: "+err.Error())
	} else if v, err := parseDarwinLoadavg(out); err != nil {
		f.Notes = append(f.Notes, "load: "+err.Error())
	} else {
		f.Load1 = v
	}
}
