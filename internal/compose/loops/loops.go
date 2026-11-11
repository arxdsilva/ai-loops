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
	"sync"
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
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/importscan"
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

// detector memoises language-pack detection for one process: the gate,
// arch, hooks and workspace all ask, and the walk should happen once.
type detector struct {
	svc       *langpack.Service
	root      string
	overrides []langpack.Override
	once      sync.Once
	ds        []langpack.Detected
	err       error
}

func (d *detector) detect() ([]langpack.Detected, error) {
	d.once.Do(func() { d.ds, d.err = d.svc.Detect(d.root, d.overrides) })
	return d.ds, d.err
}

// all returns the detected packs, or none when detection fails.
func (d *detector) all() []langpack.Detected {
	ds, _ := d.detect()
	return ds
}

// Detect implements hook.Langs (the root is the project root it was built for).
func (d *detector) Detect(string) ([]langpack.Detected, error) { return d.detect() }

// archGate adapts the arch checker to gate.Arch over the detected packs.
type archGate struct {
	svc    *archcheck.Service
	layers archrule.Layers
	langs  *detector
}

func (g archGate) Check(ctx context.Context, root string) (string, bool, error) {
	return archcheck.Gate{Service: g.svc, Layers: g.layers, Packs: g.langs.all()}.Check(ctx, root)
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
	langs := langpack.New(packfs.Embedded{FS: ailoops.Source}, packfs.OS{})
	detected := &detector{svc: langs, root: root, overrides: cfg.Languages}
	arch := archcheck.New(goparse.New()).WithScanner(importscan.New())
	// runspace.Runner adds the active run's GOCACHE/GOTMPDIR to gate commands.
	gates := gate.New(runspace.NewRunner(), archGate{svc: arch, layers: cfg.Layers, langs: detected}, trk, clock).
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
		Workspace: runspace.New(root).WithPacks(detected.all),
		Gate:      gates,
		Arch:      arch,
		Tickets:   tickets,
		Report:    htmlreport.New(),
		Hooks: hook.New(hook.Deps{
			Rec: trk, Files: fsys.New(), Cmd: gotool.New(), Env: os.Getenv, Getwd: os.Getwd,
			Update: updates, Version: version, UpdateOff: updateOff, Langs: detected,
		}),
		Update:      updates,
		UpdateState: upState,
		UpdateOff:   updateOff,
		Langs:       langs,
		Languages:   cfg.Languages,
		HW:          hwprobe.New(),
		Parallel:    cfg.Parallel,
		Limits:      cfg.Limits,
		Checkpoints: cfg.Checkpoints(),
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
