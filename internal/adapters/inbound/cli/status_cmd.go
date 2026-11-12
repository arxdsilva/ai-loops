package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

// cmdStatus implements `loops status [--json|--statusline|--subagent]`.
func (a *App) cmdStatus(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	asJSON := fs.Bool("json", false, "")
	line := fs.Bool("statusline", false, "")
	sub := fs.Bool("subagent", false, "")
	if err := fs.Parse(args); err != nil {
		if *line || *sub {
			return 0
		}
		fmt.Fprintln(a.Err, "usage: loops status [--json|--statusline|--subagent]")
		return 2
	}

	st, dir, active := a.currentState(ctx)
	switch {
	case *line:
		fmt.Fprintln(a.Out, statusLine(a.readStatusInput(), st, active)+updateMarker(a))
		return 0
	case *sub:
		a.writeSubagentLines(st, active)
		return 0
	case *asJSON:
		env := run.StatusDoc{}
		if active {
			env = run.StatusDoc{Active: true, Dir: dir, State: &st}
		}
		b, err := json.MarshalIndent(env, "", "  ")
		if err != nil {
			fmt.Fprintln(a.Err, "loops status:", err)
			return 1
		}
		fmt.Fprintln(a.Out, string(b))
		return 0
	}
	if !active {
		fmt.Fprintln(a.Out, "loops: no active run")
		return 0
	}
	fmt.Fprintln(a.Out, plainLine(st))
	return 0
}

// currentState loads the current run; any failure means "no active run".
func (a *App) currentState(ctx context.Context) (run.State, string, bool) {
	if a.Tracker == nil {
		return run.State{}, "", false
	}
	st, err := a.Tracker.State(ctx, "")
	if err != nil {
		return run.State{}, "", false
	}
	dir, _ := a.Tracker.Dir(ctx, "")
	return st, dir, true
}
