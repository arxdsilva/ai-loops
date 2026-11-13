package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/application/release"
)

const releaseUsage = "usage: loops release [auto|patch|minor|major] [--dry-run] [--push]"

// Releaser is the release use case (release.New).
type Releaser interface {
	Release(ctx context.Context, o release.Options) (release.Result, error)
}

// releaseCmd holds `loops release`, off App's method set.
type releaseCmd struct{ *App }

// cmdRelease is a plain function (not an App method) to keep App small.
func cmdRelease(a *App, ctx context.Context, args []string) int {
	return releaseCmd{a}.run(ctx, args)
}

func (r releaseCmd) run(ctx context.Context, args []string) int {
	fset := r.newFlagSet("release")
	dry := fset.Bool("dry-run", false, "plan only")
	push := fset.Bool("push", false, "push HEAD and the tag")
	pos, err := parseFlagsAnywhere(fset, args)
	if err != nil || len(pos) > 1 {
		return r.usageErr(releaseUsage)
	}
	level := "auto"
	if len(pos) == 1 {
		level = pos[0]
		switch level {
		case "auto", "patch", "minor", "major":
		default:
			return r.usageErr(releaseUsage)
		}
	}
	if r.Release == nil {
		fmt.Fprintln(r.Err, "loops release: unavailable")
		return 1
	}
	res, err := r.Release.Release(ctx, release.Options{Level: level, DryRun: *dry, Push: *push})
	if err != nil {
		fmt.Fprintln(r.Err, "loops release:", err)
		return 1
	}
	if len(res.Taken) > 0 {
		fmt.Fprintf(r.Out, "release: skipping already tagged %s\n", strings.Join(res.Taken, ", "))
	}
	switch {
	case res.Skipped:
		fmt.Fprintln(r.Out, "release: skipped:", res.Reason)
	case res.DryRun:
		fmt.Fprintf(r.Out, "release: %s → %s (%s: %s) [dry run]\n", res.Current, res.Next, res.Level, res.Summary)
	default:
		fmt.Fprintf(r.Out, "release: %s → %s (%s: %s)\nreleased v%s\n", res.Current, res.Next, res.Level, res.Summary, res.Next)
	}
	return 0
}
