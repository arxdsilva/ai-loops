//go:build windows

package hwprobe

import (
	"context"

	"github.com/arxdsilva/ai-loops/internal/domain/hw"
)

const psMemCmd = "Get-CimInstance Win32_OperatingSystem | Select FreePhysicalMemory,TotalVisibleMemorySize | ConvertTo-Json"

func probeOS(ctx context.Context, f *hw.Facts) {
	out, err := run(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", psMemCmd)
	if err != nil {
		f.Notes = append(f.Notes, "ram: "+err.Error())
		return
	}
	if t, a, err := parseWindowsMem(out); err != nil {
		f.Notes = append(f.Notes, "ram: "+err.Error())
	} else {
		f.TotalRAMBytes, f.AvailRAMBytes = t, a
	}
	f.Notes = append(f.Notes, "load: not available on windows")
}
