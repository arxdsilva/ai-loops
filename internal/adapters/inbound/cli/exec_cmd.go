package cli

import (
	"context"
	"errors"
	"fmt"
	"github.com/arxdsilva/ai-loops/internal/platform/scratch"

	"github.com/arxdsilva/ai-loops/internal/application/tracker"
	"github.com/arxdsilva/ai-loops/internal/domain/pack"
	"github.com/arxdsilva/ai-loops/internal/domain/workspace"
)

const execUsage = "usage: loops exec [--pack id] -- <cmd…>"

// cmdExec implements `loops exec [--pack id] -- <cmd…>`: the command with
// the active run's env for one language pack (cache_env and env, TOOLING §5)
// and the pack's cache_argv inserted after the tool name. The pack is the
// one --pack names, else the detected pack that uses the tool, else the only
// detected pack. Without a run, or without a fitting pack, the command runs
// plain. `loops go` is the Go special case and stays as it is.
func cmdExec(a *App, ctx context.Context, args []string) int {
	fs := a.newFlagSet("exec")
	id := fs.String("pack", "", "language pack id (default: the pack that uses the tool, or the only one)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	argv := fs.Args()
	if len(argv) == 0 {
		return a.usageErr(execUsage)
	}
	pw, ok := a.Workspace.(PackWorkspace)
	if !ok {
		fmt.Fprintln(a.Err, "loops exec: workspace unavailable")
		return 1
	}
	dir, err := a.Tracker.Dir(ctx, "")
	if errors.Is(err, tracker.ErrNoActiveRun) {
		return pw.Exec(ctx, "", nil, argv, a.In, a.Out, a.Err)
	}
	if err != nil {
		return a.fail("exec", err)
	}
	p, found := workspace.PickPack(packsOf(pw.Packs()), *id, argv[0])
	if !found && *id != "" {
		if p, found = namedPack(a, *id); !found {
			return a.usageErr("exec: no language pack " + *id)
		}
	}
	if !found {
		fmt.Fprintf(a.Err, "loops exec: no language pack for %s; running without the run's cache env\n", argv[0])
		return pw.Exec(ctx, "", nil, argv, a.In, a.Out, a.Err)
	}
	env, err := pw.PackEnv(dir, p)
	if err != nil {
		return a.fail("exec", err)
	}
	return pw.Exec(ctx, "", env, workspace.PackArgv(scratch.Dir(dir), p, argv), a.In, a.Out, a.Err)
}

// namedPack loads pack id when --pack names a pack that was not detected.
func namedPack(a *App, id string) (pack.Pack, bool) {
	if a.Langs == nil {
		return pack.Pack{}, false
	}
	p, err := a.Langs.Get(id)
	return p, err == nil && workspace.ValidPackID(p.ID)
}
