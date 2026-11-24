package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/platform/loopsdir"
)

const initUsage = "usage: loops init"

// initNote is the last line of `loops init`.
const initNote = "commit .loops/config.json: it records this project's loops settings for the whole team (edit it like code; `loops run start` needs it)"

// cmdInit implements `loops init` (WORKFLOW §2.0): the project's first-run
// setup. It writes the minimal .loops/config.json when absent, keeps an
// existing one, re-includes it in .loops/.gitignore and prints the detected
// language packs without pinning them. Idempotent.
func cmdInit(a *App, _ context.Context, args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(a.Err, initUsage)
		return 2
	}
	res, err := loopsdir.EnsureConfig(a.Root)
	if err != nil {
		return a.fail("init", err)
	}
	if res.Created {
		fmt.Fprintln(a.Out, "created .loops/config.json")
	} else {
		fmt.Fprintln(a.Out, "kept .loops/config.json (already initialized)")
	}
	if len(res.Added) > 0 {
		fmt.Fprintln(a.Out, "added to .loops/.gitignore: "+strings.Join(res.Added, " "))
	}
	fmt.Fprintln(a.Out, "languages: "+a.detectedLangs())
	fmt.Fprintln(a.Out, initNote)
	return 0
}

// detectedLangs is the one-line language summary of `loops init`.
func (a *App) detectedLangs() string {
	if a.Langs == nil {
		return "unknown (no language packs loaded)"
	}
	ds, err := a.Langs.Detect(a.Root, a.Languages)
	if err != nil {
		return "unknown (" + err.Error() + ")"
	}
	if len(ds) == 0 {
		return `none detected (pin packs with "languages" in .loops/config.json)`
	}
	parts := make([]string, 0, len(ds))
	for _, d := range ds {
		parts = append(parts, d.ID+" at "+d.Root)
	}
	return strings.Join(parts, ", ") + ` (detected; "languages": [] keeps detecting)`
}

// requireInit refuses `loops run start` in a project without
// .loops/config.json when a.RequireInit is set.
func (a *App) requireInit() int {
	if !a.RequireInit || loopsdir.HasConfig(a.Root) {
		return 0
	}
	fmt.Fprintln(a.Err, "loops run start: this project is not initialized: run `loops init` first (it writes .loops/config.json, the project's loops settings, to commit)")
	return 1
}
