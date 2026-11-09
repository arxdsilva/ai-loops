package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/application/updatecheck"
)

// cmdUpdate implements `loops update [--platform p]`: it prints the update
// command for the platform, or for every platform when it is unknown.
func (a *App) cmdUpdate(_ context.Context, args []string) int {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	platform := fs.String("platform", "", "")
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 {
		fmt.Fprintln(a.Err, "usage: loops update [--platform p]")
		return 2
	}
	p := strings.ToLower(*platform)
	if p == "" {
		p = detectPlatform(os.Getenv)
	}
	table := updatecheck.Commands()
	if c, ok := table[p]; ok {
		fmt.Fprintln(a.Out, c)
		return 0
	}
	if p != "" { // named but not in the table: generic installer route
		fmt.Fprintln(a.Out, updatecheck.Command(p))
		return 0
	}
	names := make([]string, 0, len(table))
	for n := range table {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(a.Out, "%-9s %s\n", n+":", table[n])
	}
	return 0
}
