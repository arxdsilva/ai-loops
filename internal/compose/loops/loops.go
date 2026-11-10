// Package loops composes the loops binary: it builds every concrete adapter
// and use case and hands them to the CLI as interfaces.
package loops

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	ailoops "github.com/arxdsilva/ai-loops"
	"github.com/arxdsilva/ai-loops/internal/adapters/inbound/cli"
	"github.com/arxdsilva/ai-loops/internal/adapters/inbound/hook"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/fsys"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/gitcli"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/gocover"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/goparse"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/goproxy"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/gotool"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/htmlreport"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/hwprobe"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/packfs"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/portfs"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/runspace"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/runstore"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/updatestate"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/vcs"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/versionfile"
	"github.com/arxdsilva/ai-loops/internal/application/archcheck"
	"github.com/arxdsilva/ai-loops/internal/application/gate"
	"github.com/arxdsilva/ai-loops/internal/application/langpack"
	apprelease "github.com/arxdsilva/ai-loops/internal/application/release"
	"github.com/arxdsilva/ai-loops/internal/application/ticket"
	"github.com/arxdsilva/ai-loops/internal/application/tracker"
	"github.com/arxdsilva/ai-loops/internal/application/updatecheck"
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
	// runspace.Runner adds the active run's GOCACHE/GOTMPDIR to gate commands.
	gates := gate.New(runspace.NewRunner(), archGate{svc: arch, layers: cfg.Layers}, trk, clock).
		WithCoverage(gocover.New(), gate.CoverageConfig{
			Min: cfg.Gate.CoverageMin, Exclude: cfg.Gate.CoverageExclude, Base: cfg.Gate.Base,
		})
	tickets := ticket.New(vcs.New(root))

	upState := updatestate.New()
	updates := updatecheck.New(goproxy.New("github.com/arxdsilva/ai-loops"), upState, clock)
	updateOff := cfg.UpdateCheck != nil && !*cfg.UpdateCheck

	version := embeddedVersion()
	pkgs := portfs.New()
	releases := apprelease.New(gitcli.New(root),
		versionfile.New(filepath.Join(root, ".claude-plugin", "plugin.json")),
		func() error { return cli.RegeneratePackages(root, pkgs, io.Discard, errw) })

	app := &cli.App{
		Version:   version,
		Release:   releases,
		Tracker:   trk,
		Workspace: runspace.New(root),
		Gate:      gates,
		Arch:      arch,
		Tickets:   tickets,
		Report:    htmlreport.New(),
		Hooks: hook.New(hook.Deps{
			Rec: trk, Files: fsys.New(), Cmd: gotool.New(), Env: os.Getenv, Getwd: os.Getwd,
			Update: updates, Version: version, UpdateOff: updateOff,
		}),
		Update:      updates,
		UpdateState: upState,
		UpdateOff:   updateOff,
		Langs:       langpack.New(packfs.Embedded{FS: ailoops.Source}, packfs.OS{}),
		Languages:   cfg.Languages,
		HW:          hwprobe.New(),
		Parallel:    cfg.Parallel,
		Limits:      cfg.Limits,
		Source:      ailoops.Source,
		Packages:    pkgs,
		Root:        root,
		PluginRoot:  os.Getenv("LOOPS_PLUGIN_ROOT"),
		Layers:      cfg.Layers,
		GateSkip:    cfg.Gate.Skip,
		GateVuln:    cfg.Gate.Vuln,
		In:          in,
		Out:         out,
		Err:         errw,
	}
	return app.Run(ctx, args)
}

// quiet reports whether args run in a context where stderr noise is harmful
// (hooks fail open silently; status lines must print exactly one line).
func quiet(args []string) bool {
	return len(args) > 0 && (args[0] == "hook" || args[0] == "status")
}

// DevVersion is reported when the embedded plugin.json cannot be parsed.
const DevVersion = "0.0.0-dev"

// embeddedVersion is the "version" of the embedded .claude-plugin/plugin.json,
// the single source of truth for the loops version.
func embeddedVersion() string {
	data, err := ailoops.Source.ReadFile(".claude-plugin/plugin.json")
	if err != nil {
		return DevVersion
	}
	return parseVersion(data)
}

func parseVersion(data []byte) string {
	var m struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(data, &m) != nil || m.Version == "" {
		return DevVersion
	}
	return m.Version
}
