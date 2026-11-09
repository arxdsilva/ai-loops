package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/hw"
)

type hwReport struct {
	Facts          hw.Facts          `json:"facts"`
	Recommendation hw.Recommendation `json:"recommendation"`
	Knobs          hw.Knobs          `json:"knobs"`
}

// cmdHW implements `loops hw [--json] [--write]`.
func (a *App) cmdHW(ctx context.Context, args []string) int {
	fs := a.newFlagSet("hw")
	asJSON := fs.Bool("json", false, "print JSON")
	write := fs.Bool("write", false, "also write research/hardware.{json,md} into the current run dir")
	if _, err := parseFlagsAnywhere(fs, args); err != nil {
		return 2
	}
	facts, err := a.HW.Probe(ctx, a.Root)
	if err != nil {
		return a.fail("hw", err)
	}
	rep := hwReport{Facts: facts, Recommendation: hw.Recommend(facts, a.Parallel), Knobs: a.Parallel}
	js, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return a.fail("hw", err)
	}
	js = append(js, '\n')
	text := hwText(rep)
	if *write {
		dir, err := a.Tracker.Dir(ctx, "")
		if err != nil {
			return a.fail("hw", err)
		}
		if err := os.MkdirAll(filepath.Join(dir, "research"), 0o755); err != nil {
			return a.fail("hw", err)
		}
		for name, data := range map[string][]byte{"hardware.json": js, "hardware.md": []byte(text)} {
			if err := writeFileAtomic(filepath.Join(dir, "research", name), data); err != nil {
				return a.fail("hw", err)
			}
		}
	}
	if *asJSON {
		a.Out.Write(js)
	} else {
		fmt.Fprint(a.Out, text)
	}
	return 0
}

func hwText(r hwReport) string {
	f, rec := r.Facts, r.Recommendation
	var b strings.Builder
	fmt.Fprintf(&b, "# Hardware\n\n")
	fmt.Fprintf(&b, "- OS/arch: %s/%s%s\n", orUnknown(f.OS), orUnknown(f.Arch), map[bool]string{true: " (container)"}[f.Container])
	cpu := fmt.Sprintf("%d logical, GOMAXPROCS %d", f.LogicalCPUs, f.GOMAXPROCS)
	if f.CPUQuota > 0 {
		cpu += fmt.Sprintf(", cgroup limit %.2g cores", f.CPUQuota)
	}
	fmt.Fprintf(&b, "- CPU: %s\n", cpu)
	fmt.Fprintf(&b, "- RAM: %s available of %s", gibOrUnknown(f.AvailRAMBytes), gibOrUnknown(f.TotalRAMBytes))
	if f.MemLimitBytes > 0 {
		fmt.Fprintf(&b, " (cgroup limit %s)", gibOrUnknown(f.MemLimitBytes))
	}
	b.WriteString("\n")
	if f.Load1 > 0 {
		fmt.Fprintf(&b, "- Load (1m): %.2f\n", f.Load1)
	}
	fmt.Fprintf(&b, "- Disk free: %s\n", gibOrUnknown(f.DiskFreeBytes))
	fmt.Fprintf(&b, "- Go: %s\n\n", orUnknown(f.GoVersion))
	fmt.Fprintf(&b, "Recommendation: %d parallel agents, %d concurrent test runs, GOFLAGS=%s\n", rec.MaxParallelAgents, rec.MaxConcurrentTestRuns, rec.GOFLAGS)
	fmt.Fprintf(&b, "Why: %s\n", rec.Reason)
	if len(f.Notes) > 0 {
		fmt.Fprintf(&b, "Notes: %s\n", strings.Join(f.Notes, "; "))
	}
	return b.String()
}

func gibOrUnknown(v uint64) string {
	if v == 0 {
		return "unknown"
	}
	return fmt.Sprintf("%.1f GiB", float64(v)/float64(1<<30))
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}
