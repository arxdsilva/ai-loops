// Package loops composes the loops binary: it builds every concrete adapter
// and use case and hands them to the CLI as interfaces.
package loops

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	ailoops "github.com/arxdsilva/ai-loops"
	"github.com/arxdsilva/ai-loops/internal/adapters/inbound/cli"
	"github.com/arxdsilva/ai-loops/internal/adapters/inbound/hook"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/fsys"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/goparse"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/gotool"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/htmlreport"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/hwprobe"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/portfs"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/runstore"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/vcs"
	"github.com/arxdsilva/ai-loops/internal/application/archcheck"
	"github.com/arxdsilva/ai-loops/internal/application/gate"
	"github.com/arxdsilva/ai-loops/internal/application/ticket"
	"github.com/arxdsilva/ai-loops/internal/application/tracker"
	"github.com/arxdsilva/ai-loops/internal/domain/archrule"
	"github.com/arxdsilva/ai-loops/internal/platform/projroot"
)

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// archGate adapts the arch checker to gate.Arch.
type archGate struct {
	svc    *archcheck.Service
	layers archrule.Layers
}

func (g archGate) Check(ctx context.Context, root string) (string, bool, error) {
	rep, err := g.svc.Check(ctx, root, g.layers)
	if err != nil {
		return "", false, err
	}
	var b strings.Builder
	for _, f := range rep.Findings {
		if f.Severity == archrule.SeverityError {
			b.WriteString(f.File + ":" + itoa(f.Line) + " " + f.Rule + " " + f.Msg + "\n")
		}
	}
	return b.String(), rep.OK(), nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for ; n > 0; n /= 10 {
		d = append([]byte{byte('0' + n%10)}, d...)
	}
	return string(d)
}

// Main builds the app for the current process and runs args.
func Main(ctx context.Context, args []string, in io.Reader, out, errw io.Writer) int {
	cwd, _ := os.Getwd()
	root := projroot.Find(cwd)
	cfg, err := runstore.LoadConfig(root)
	if err != nil && !quiet(args) {
		// A broken config falls back to defaults, but say so: silently
		// ignoring approval/auto_pr/gate settings would be worse.
		fmt.Fprintf(errw, "loops: ignoring .loops/config.json: %v\n", err)
	}

	clock := systemClock{}
	store := runstore.New(root)
	trk := tracker.New(store, store, clock)
	arch := archcheck.New(goparse.New())
	gates := gate.New(gotool.New(), archGate{svc: arch, layers: cfg.Layers}, trk, clock)
	tickets := ticket.New(vcs.New(root))

	app := &cli.App{
		Tracker: trk,
		Gate:    gates,
		Arch:    arch,
		Tickets: tickets,
		Report:  htmlreport.New(),
		Hooks: hook.New(hook.Deps{
			Rec: trk, Files: fsys.New(), Cmd: gotool.New(), Env: os.Getenv, Getwd: os.Getwd,
		}),
		HW:         hwprobe.New(),
		Parallel:   cfg.Parallel,
		Limits:     cfg.Limits,
		Source:     ailoops.Source,
		Packages:   portfs.New(),
		Root:       root,
		PluginRoot: os.Getenv("LOOPS_PLUGIN_ROOT"),
		Layers:     cfg.Layers,
		GateSkip:   cfg.Gate.Skip,
		GateVuln:   cfg.Gate.Vuln,
		In:         in,
		Out:        out,
		Err:        errw,
	}
	return app.Run(ctx, args)
}

// quiet reports whether args run in a context where stderr noise is harmful
// (hooks fail open silently; status lines must print exactly one line).
func quiet(args []string) bool {
	return len(args) > 0 && (args[0] == "hook" || args[0] == "status")
}
