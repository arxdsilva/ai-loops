// Package hwprobe measures the machine (best effort, per OS) for `loops hw`.
// Every failure becomes a Note on the facts, never an error.
package hwprobe

import (
	"bytes"
	"context"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/arxdsilva/ai-loops/internal/domain/hw"
)

const cmdTimeout = 2 * time.Second

// Probe measures the host.
type Probe struct{}

// New returns a Probe.
func New() *Probe { return &Probe{} }

// Probe returns facts about the machine; dir is the project dir (for disk
// free). Unknown values stay zero and are noted.
func (p *Probe) Probe(ctx context.Context, dir string) (hw.Facts, error) {
	f := hw.Facts{
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		LogicalCPUs: runtime.NumCPU(),
		GOMAXPROCS:  runtime.GOMAXPROCS(0),
	}
	probeOS(ctx, &f)
	if free, err := diskFree(dir); err != nil {
		f.Notes = append(f.Notes, "disk: "+err.Error())
	} else {
		f.DiskFreeBytes = free
	}
	f.GoVersion = goVersion(ctx)
	return f, nil
}

// run executes a command with the probe timeout and returns its stdout.
func run(ctx context.Context, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, cmdTimeout)
	defer cancel()
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

func goVersion(ctx context.Context) string {
	if out, err := run(ctx, "go", "env", "GOVERSION"); err == nil {
		if v := strings.TrimSpace(out); v != "" {
			return v
		}
	}
	return runtime.Version()
}
