package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/platform/loopsdir"
)

const memoryUsage = "usage: loops memory init"

// memoryInitNote is the last line of `loops memory init`.
const memoryInitNote = `pages under .loops/memory/ are committed with the PR; set "memory": {"store": "files"} in .loops/config.json to prefer them over ai-memory; delete the folder to leave`

// cmdMemory implements `loops memory init`: the opt-in file store
// (.loops/memory/, WORKFLOW §7). It is idempotent and changes nothing else.
func cmdMemory(a *App, _ context.Context, args []string) int {
	if len(args) != 1 || args[0] != "init" {
		fmt.Fprintln(a.Err, memoryUsage)
		return 2
	}
	res, err := loopsdir.EnsureMemory(a.Root)
	if err != nil {
		return a.fail("memory init", err)
	}
	changed := false
	if res.CreatedDir {
		fmt.Fprintln(a.Out, "created .loops/memory/")
		changed = true
	}
	if len(res.Added) > 0 {
		fmt.Fprintln(a.Out, "added to .loops/.gitignore: "+strings.Join(res.Added, " "))
		changed = true
	}
	if !changed {
		fmt.Fprintln(a.Out, "nothing to change: .loops/memory/ is set up")
	}
	fmt.Fprintln(a.Out, memoryInitNote)
	return 0
}
