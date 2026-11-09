// Package cli is the `loops` command line: skills call it through Bash,
// hooks through `loops hook <name>`, the status line through `loops status`.
//
// Command ownership (one file per command group):
//
//	run_cmd.go     run start|phase|event|label|loopback|verdict|end|dir   (tracker)
//	gate_cmd.go    gate                                            (gate pipeline)
//	ticket_cmd.go  ticket fetch                                    (tickets)
//	arch_cmd.go    arch                                            (arch checker)
//	status_cmd.go  status [--json|--statusline|--subagent]         (status line, mod pane)
//	report_cmd.go  report                                          (HTML report)
//	cli.go         dispatch, hook, path, version, flag helpers
package cli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/insurgencygames/ai-loops/internal/domain/archrule"
)

// Version is the loops version reported by `loops version`.
const Version = "0.1.0"

// App holds the use cases and the process environment.
type App struct {
	Tracker Tracker
	Gate    Gater
	Arch    Archer
	Tickets Tickets
	Report  Reporter
	Hooks   Hooks

	Root       string          // project root
	PluginRoot string          // plugin root ("" when unknown)
	Layers     archrule.Layers // arch layer layout from .loops/config.json
	GateSkip   []string        // gate checks skipped by config
	GateVuln   bool            // gate runs govulncheck by config

	In  io.Reader
	Out io.Writer
	Err io.Writer
}

type command struct {
	run   func(a *App, ctx context.Context, args []string) int
	usage string
}

var commands = map[string]command{
	"run":     {(*App).cmdRun, "run start|phase|event|label|loopback|verdict|end|dir …  record the workflow"},
	"gate":    {(*App).cmdGate, "gate [--pkgs p,…] [--only c,…] [--skip c,…] [--vuln] [--fail-fast]"},
	"ticket":  {(*App).cmdTicket, "ticket fetch [REF] [--source s] [--body-file f|-]"},
	"arch":    {(*App).cmdArch, "arch [--json] [--root dir]"},
	"status":  {(*App).cmdStatus, "status [--json|--statusline|--subagent]"},
	"report":  {(*App).cmdReport, "report [--run id] [--out file]"},
	"hook":    {(*App).cmdHook, "hook <name>   (Claude Code hooks; reads hook JSON on stdin)"},
	"path":    {(*App).cmdPath, "path [root|specs|templates|skills|agents]"},
	"version": {(*App).cmdVersion, "version"},
}

// Run dispatches args (without the program name) and returns the exit code.
func (a *App) Run(ctx context.Context, args []string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		a.usage()
		return 0
	}
	c, ok := commands[args[0]]
	if !ok {
		fmt.Fprintf(a.Err, "loops: unknown command %q\n", args[0])
		a.usage()
		return 2
	}
	return c.run(a, ctx, args[1:])
}

func (a *App) usage() {
	names := make([]string, 0, len(commands))
	for n := range commands {
		names = append(names, n)
	}
	sort.Strings(names)
	fmt.Fprintln(a.Err, "usage: loops <command> [args]")
	for _, n := range names {
		fmt.Fprintf(a.Err, "  %s\n", commands[n].usage)
	}
}

func (a *App) cmdHook(ctx context.Context, args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(a.Err, "usage: loops hook <name>")
		return 0 // hooks fail open
	}
	return a.Hooks.Handle(ctx, args[0], a.In, a.Out, a.Err)
}

func (a *App) cmdPath(ctx context.Context, args []string) int {
	if a.PluginRoot == "" {
		fmt.Fprintln(a.Err, "loops: plugin root unknown (LOOPS_PLUGIN_ROOT not set)")
		return 1
	}
	sub := ""
	if len(args) > 0 && args[0] != "root" {
		sub = args[0]
	}
	p := a.PluginRoot
	if sub != "" {
		p = strings.TrimRight(p, "/") + "/" + sub
	}
	fmt.Fprintln(a.Out, p)
	return 0
}

func (a *App) cmdVersion(ctx context.Context, args []string) int {
	fmt.Fprintln(a.Out, "loops", Version)
	return 0
}
