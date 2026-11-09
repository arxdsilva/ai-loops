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
//	port_cmd.go    port build|check                                (per-platform packages, specs/PLATFORMS.md)
//	install_cmd.go install --target                                (place a platform package)
//	watch_cmd.go   watch                                           (terminal live view)
//	version_cmd.go version [--check]                               (release check)
//	update_cmd.go  update [--platform p]                           (update commands)
//	cli.go         dispatch, hook, path, version, flag helpers
package cli

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/archrule"
	"github.com/arxdsilva/ai-loops/internal/domain/hw"
	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

// Version is the loops version reported by `loops version`.
const Version = "0.3.0"

// App holds the use cases and the process environment.
type App struct {
	Tracker Tracker
	Gate    Gater
	Arch    Archer
	Tickets Tickets
	Report  Reporter
	Hooks   Hooks

	// Source is the canonical plugin tree (embedded, or a checkout via --src).
	Source fs.FS
	// Packages writes and diffs generated package files.
	Packages PackageWriter

	HW       HWProbe    // hardware probe (loops hw)
	Parallel hw.Knobs   // .loops/config.json "parallel"
	Limits   run.Limits // .loops/config.json "limits" (opt-in; zero = none)

	Root       string          // project root
	PluginRoot string          // plugin root ("" when unknown)
	Layers     archrule.Layers // arch layer layout from .loops/config.json
	GateSkip   []string        // gate checks skipped by config
	GateVuln   bool            // gate runs govulncheck by config

	// Update checks the Go proxy (nil = update features off); UpdateState is
	// the cached state read by the status line (no network).
	Update      Updater
	UpdateState UpdateStore
	UpdateOff   bool // .loops/config.json "update_check": false

	In  io.Reader
	Out io.Writer
	Err io.Writer
}

type command struct {
	run   func(a *App, ctx context.Context, args []string) int
	usage string
}

var commands = map[string]command{
	"run":     {(*App).cmdRun, "run start|phase|event|label|loopback|verdict|override|end|dir …  record the workflow"},
	"gate":    {(*App).cmdGate, "gate [--pkgs p,…] [--only c,…] [--skip c,…] [--vuln] [--fail-fast]"},
	"ticket":  {(*App).cmdTicket, "ticket fetch [REF] [--source s] [--body-file f|-]"},
	"arch":    {(*App).cmdArch, "arch [--json] [--root dir]"},
	"status":  {(*App).cmdStatus, "status [--json|--statusline|--subagent]"},
	"report":  {(*App).cmdReport, "report [--run id] [--out file]"},
	"hook":    {(*App).cmdHook, "hook <name> [--platform p]   (agent hooks; reads hook JSON on stdin)"},
	"port":    {(*App).cmdPort, "port build|check [--out dir] [--platform p,…] [--models file] [--src dir]   generate per-platform packages"},
	"install": {(*App).cmdInstall, "install --target <platform> [--scope project|user] [--project dir] [--force] [--dry-run]"},
	"hw":      {(*App).cmdHW, "hw [--json] [--write]   hardware facts + parallelism recommendation for the planner"},
	"budget":  {(*App).cmdBudget, "budget check [--phase p] [--spawn n] [--role r] [--verbose] | show | parse [--budget …] [--agents …] [--roles …] [--max-agents n]   opt-in limits (exit 3 = over budget)"},
	"plan":    {(*App).cmdPlan, "plan check <plan.md> [--cap n] [--json]   waves vs parallelism cap and scope overlap"},
	"watch":   {(*App).cmdWatch, "watch [--interval 2s] [--once]   live run view in a terminal (any platform)"},
	"path":    {(*App).cmdPath, "path [root|specs|templates|skills|agents]"},
	"version": {(*App).cmdVersion, "version [--check [--platform p]]   print the version; --check asks the Go proxy for the latest release"},
	"update":  {(*App).cmdUpdate, "update [--platform p]   print how to update loops (all platforms when unknown)"},
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
	var name, platform string
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--platform" && i+1 < len(args):
			i++
			platform = args[i]
		case strings.HasPrefix(args[i], "--platform="):
			platform = strings.TrimPrefix(args[i], "--platform=")
		case name == "":
			name = args[i]
		default:
			name = ""
			i = len(args)
		}
	}
	if name == "" {
		fmt.Fprintln(a.Err, "usage: loops hook <name> [--platform p]")
		return 0 // hooks fail open
	}
	return a.Hooks.Handle(ctx, name, platform, a.In, a.Out, a.Err)
}

func (a *App) cmdPath(ctx context.Context, args []string) int {
	root := a.PluginRoot
	if root == "" {
		root = a.installedPluginHome()
	}
	if root == "" {
		fmt.Fprintln(a.Err, "loops: plugin root unknown (LOOPS_PLUGIN_ROOT not set)")
		return 1
	}
	sub := ""
	if len(args) > 0 && args[0] != "root" {
		sub = args[0]
	}
	p := root
	if sub != "" {
		p = strings.TrimRight(p, "/") + "/" + sub
	}
	fmt.Fprintln(a.Out, p)
	return 0
}

// installedPluginHome finds a plugin home placed by `loops install`:
// <project>/.loops/plugin, then ~/.local/share/loops/plugin. It returns ""
// when neither contains specs/.
func (a *App) installedPluginHome() string {
	cands := []string{}
	if a.Root != "" {
		// written by the session-start hook on Codex/Cursor/Gemini.
		if b, err := os.ReadFile(filepath.Join(a.Root, ".loops", "plugin-root")); err == nil {
			if r := strings.TrimSpace(string(b)); r != "" {
				cands = append(cands, r)
			}
		}
		cands = append(cands, filepath.Join(a.Root, ".loops", "plugin"))
	}
	h, herr := os.UserHomeDir()
	if herr == nil {
		cands = append(cands, filepath.Join(h, ".local", "share", "loops", "plugin"))
	}
	// user-level pointer written at session start.
	state := os.Getenv("XDG_STATE_HOME")
	if state == "" && herr == nil {
		state = filepath.Join(h, ".local", "state")
	}
	if state != "" {
		if b, err := os.ReadFile(filepath.Join(state, "loops", "plugin-root")); err == nil {
			if r := strings.TrimSpace(string(b)); r != "" {
				cands = append(cands, r)
			}
		}
	}
	for _, c := range cands {
		if fi, err := os.Stat(filepath.Join(c, "specs")); err == nil && fi.IsDir() {
			return c
		}
	}
	return ""
}
