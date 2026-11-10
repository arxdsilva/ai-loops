package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/arxdsilva/ai-loops/internal/application/langpack"
	"github.com/arxdsilva/ai-loops/internal/application/tracker"
	"github.com/arxdsilva/ai-loops/internal/domain/pack"
	"github.com/arxdsilva/ai-loops/internal/domain/run"
	"github.com/arxdsilva/ai-loops/internal/domain/workspace"
)

// Workspace is a run's scratch space (runspace adapter): the run-scoped Go
// cache and the end-of-run cleanup of build artifacts.
type Workspace interface {
	// Prepare creates the run's go cache dirs and snapshots the untracked files.
	Prepare(ctx context.Context, runDir string) error
	// Env ensures the go cache dirs exist and returns GOCACHE/GOTMPDIR entries.
	Env(runDir string) ([]string, error)
	// Clean removes the run's go dir and the new artifacts (dryRun reports only).
	Clean(ctx context.Context, runDir string, dryRun bool) (workspace.Report, error)
	// Go runs the go tool with env added, stdio passed through; it returns the exit code.
	Go(ctx context.Context, dir string, env, args []string, in io.Reader, out, errw io.Writer) int
}

// PackWorkspace is the optional, language-pack side of a Workspace: the
// detected packs, each pack's run-scoped env (TOOLING §5) and running any
// tool. Without it `loops run env` prints the Go line only and `loops exec`
// is unavailable.
type PackWorkspace interface {
	// Packs returns the detected language packs.
	Packs() []langpack.Detected
	// PackEnv creates <run>/<id>/{cache,tmp} and returns the pack's env.
	PackEnv(runDir string, p pack.Pack) ([]string, error)
	// Exec runs argv with env added, stdio passed through; it returns the exit code.
	Exec(ctx context.Context, dir string, env, argv []string, in io.Reader, out, errw io.Writer) int
}

// packsOf returns the packs of ds, one per id.
func packsOf(ds []langpack.Detected) []pack.Pack {
	ps := make([]pack.Pack, 0, len(ds))
	for _, d := range ds {
		ps = append(ps, d.Pack)
	}
	return workspace.Unique(ps)
}

// wsCmd holds the workspace commands (`loops run env|clean`, `loops go`,
// the run start/end hooks), off App's method set.
type wsCmd struct{ *App }

// runEnv returns the active run's dir and go environment; ok is false (with
// a nil error) when no run is active or the workspace is unavailable.
func (a wsCmd) runEnv(ctx context.Context) (dir string, env []string, ok bool, err error) {
	if a.Workspace == nil {
		return "", nil, false, nil
	}
	dir, err = a.Tracker.Dir(ctx, "")
	if errors.Is(err, tracker.ErrNoActiveRun) {
		return "", nil, false, nil
	}
	if err != nil {
		return "", nil, false, err
	}
	env, err = a.Workspace.Env(dir)
	if err != nil {
		return "", nil, false, err
	}
	return dir, env, true, nil
}

// env implements `loops run env [--shell sh|fish|powershell]`: the exports
// that scope go commands to the run (the GOCACHE/GOTMPDIR line, always
// first), then one line per other detected pack with its cache_env and env
// (for Go: any env beyond those two); nothing (exit 0) without a run.
func (a wsCmd) env(ctx context.Context, args []string) int {
	fs := a.newFlagSet("run env")
	shell := fs.String("shell", "sh", "sh|fish|powershell")
	pos, err := parseFlagsAnywhere(fs, args)
	if err != nil {
		return 2
	}
	if len(pos) > 0 {
		return a.usageErr("usage: loops run env [--shell sh|fish|powershell]")
	}
	if _, err := workspace.Exports(*shell, "", ""); err != nil {
		return a.usageErr("run env: " + err.Error())
	}
	dir, _, ok, err := a.runEnv(ctx)
	if err != nil {
		return a.fail("run env", err)
	}
	if !ok {
		return 0
	}
	pw, isPW := a.Workspace.(PackWorkspace)
	var packs []pack.Pack
	if isPW {
		packs = packsOf(pw.Packs())
	}
	// The GOCACHE/GOTMPDIR line: for a Go project, or when nothing is
	// detected (the pre-pack behaviour), never for a project without Go.
	if len(packs) == 0 || slices.ContainsFunc(packs, func(p pack.Pack) bool { return p.ID == workspace.GoPack }) {
		line, _ := workspace.Exports(*shell, workspace.GoCache(dir), workspace.GoTmp(dir))
		fmt.Fprintln(a.Out, line)
	}
	for _, p := range packs {
		env, err := pw.PackEnv(dir, p)
		if err != nil {
			return a.fail("run env", err)
		}
		if p.ID == workspace.GoPack {
			env = workspace.ExtraEnv(env, "GOCACHE", "GOTMPDIR")
		}
		line, err := workspace.ExportsEnv(*shell, env)
		if err != nil {
			return a.fail("run env", err)
		}
		if line != "" {
			fmt.Fprintln(a.Out, line)
		}
	}
	return 0
}

// cmdGo implements `loops go <args…>`: go with the active run's GOCACHE and
// GOTMPDIR (plain go without a run), stdio and exit code passed through.
func cmdGo(a *App, ctx context.Context, args []string) int {
	w := wsCmd{a}
	if a.Workspace == nil {
		fmt.Fprintln(a.Err, "loops go: workspace unavailable")
		return 1
	}
	_, env, _, err := w.runEnv(ctx)
	if err != nil {
		return a.fail("go", err)
	}
	return a.Workspace.Go(ctx, "", env, args, a.In, a.Out, a.Err)
}

// gateEnv is the environment `loops gate` adds to its commands (nil without a run).
func (a wsCmd) gateEnv(ctx context.Context) []string {
	_, env, _, err := a.runEnv(ctx)
	if err != nil {
		fmt.Fprintf(a.Err, "loops gate: run go cache unavailable, using the default: %v\n", err)
	}
	return env
}

// prepare sets up the run's workspace after `loops run start`; a failure is
// reported but never fails the start.
func (a wsCmd) prepare(ctx context.Context, dir string) {
	if a.Workspace == nil {
		return
	}
	if err := a.Workspace.Prepare(ctx, dir); err != nil {
		fmt.Fprintf(a.Err, "loops run start: workspace not prepared (no run-scoped go cache, no cleanup baseline): %v\n", err)
	}
}

// clean implements `loops run clean [--dry-run] [--json]`.
func (a wsCmd) clean(ctx context.Context, args []string) int {
	fs := a.newFlagSet("run clean")
	dry := fs.Bool("dry-run", false, "report what would be removed")
	asJSON := fs.Bool("json", false, "print the report as JSON")
	pos, err := parseFlagsAnywhere(fs, args)
	if err != nil {
		return 2
	}
	if len(pos) > 0 {
		return a.usageErr("usage: loops run clean [--dry-run] [--json]")
	}
	if a.Workspace == nil {
		fmt.Fprintln(a.Err, "loops run clean: workspace unavailable")
		return 1
	}
	dir, err := a.Tracker.Dir(ctx, "")
	if err != nil {
		return a.fail("run clean", err)
	}
	r, err := a.cleanup(ctx, dir, *dry)
	if err != nil {
		return a.fail("run clean", err)
	}
	if *asJSON {
		b, _ := json.MarshalIndent(r, "", "  ")
		fmt.Fprintln(a.Out, string(b))
	} else {
		fmt.Fprint(a.Out, r.Text())
	}
	return 0
}

// cleanup cleans dir and, unless dryRun, records the cleanup note on the
// current run.
func (a wsCmd) cleanup(ctx context.Context, dir string, dryRun bool) (workspace.Report, error) {
	r, err := a.Workspace.Clean(ctx, dir, dryRun)
	if err != nil {
		return r, err
	}
	if !dryRun {
		if err := a.Tracker.Record(ctx, run.Event{Kind: run.KindNote, Name: "cleanup", Note: r.Note()}); err != nil {
			return r, err
		}
	}
	return r, nil
}

// endClean runs the automatic cleanup of `loops run end`; failures are
// reported and never block the end.
func (a wsCmd) endClean(ctx context.Context, dir string) {
	if a.Workspace == nil {
		return
	}
	r, err := a.cleanup(ctx, dir, false)
	if err != nil {
		fmt.Fprintf(a.Err, "loops run end: cleanup: %v\n", err)
		return
	}
	fmt.Fprint(a.Out, r.Text())
}
