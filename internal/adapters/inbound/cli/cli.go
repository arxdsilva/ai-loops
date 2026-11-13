// Package cli is the `loops` command line: skills call it through Bash,
// hooks through `loops hook <name>`, the status line through `loops status`.
//
// Command ownership (one file per command group):
//
//	run_cmd.go     run start|phase|event|label|loopback|verdict|end|dir|env|clean (tracker)
//	go_cmd.go      go, run env|clean, start/end workspace hooks    (run-scoped caches, cleanup)
//	exec_cmd.go    exec [--pack id] -- <cmd…>                      (run-scoped pack env for any tool)
//	gate_cmd.go    gate                                            (gate pipeline; --security: the packs' security checks)
//	lang_cmd.go    lang list|show|validate|detect                  (language packs)
//	ticket_cmd.go  ticket fetch                                    (tickets)
//	arch_cmd.go    arch                                            (arch checker)
//	status_cmd.go  status [--json|--statusline|--subagent]         (status line, mod pane)
//	report_cmd.go  report                                          (HTML report)
//	port_cmd.go    port build|check                                (per-platform packages, specs/PLATFORMS.md)
//	install_cmd.go install --target                                (place a platform package)
//	watch_cmd.go   watch                                           (terminal live view)
//	version_cmd.go version [--check]                               (release check)
//	update_cmd.go  update [--platform p] [--apply]                 (update commands; --apply updates the installed plugin)
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

	"github.com/arxdsilva/ai-loops/internal/application/langpack"
	"github.com/arxdsilva/ai-loops/internal/domain/archrule"
	"github.com/arxdsilva/ai-loops/internal/domain/hw"
	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

// App holds the use cases and the process environment.
type App struct {
	// Version is the loops version (.claude-plugin/plugin.json "version").
	Version string

	Tracker Tracker
	Gate    Gater
	Arch    Archer
	Tickets Tickets
	Report  Reporter
	Hooks   Hooks
	// Workspace is the run's scratch space: run-scoped go cache, cleanup.
	Workspace Workspace

	// Source is the canonical plugin tree (embedded, or a checkout via --src).
	Source fs.FS
	// Packages writes and diffs generated package files.
	Packages PackageWriter

	// Langs loads and detects language packs; Languages pins them
	// (.loops/config.json "languages"; empty = detect).
	Langs     LangPacks
	Languages []langpack.Override

	HW       HWProbe    // hardware probe (loops hw)
	Parallel hw.Knobs   // .loops/config.json "parallel"
	Limits   run.Limits // .loops/config.json "limits" (opt-in; zero = none)
	// Checkpoints is the configured checkpoint list (.loops/config.json
	// "ask", plus "approval": "always"); `loops run ask` without a list uses it.
	Checkpoints string

	Root       string          // project root
	PluginRoot string          // plugin root ("" when unknown)
	Layers     archrule.Layers // arch layer layout from .loops/config.json
	GateSkip   []string        // gate checks skipped by config
	GateVuln   bool            // gate runs govulncheck by config

	// Update checks the Go proxy (nil = update features off); UpdateState is
	// the cached state read by the status line (no network).
	Update      Updater
	Release     Releaser // loops release (nil = unavailable)
	UpdateState UpdateStore
	UpdateOff   bool // .loops/config.json "update_check": false
	// Applier runs `loops update --apply` (nil = unavailable); Plugins reads
	// the installed plugin's version (nil = unknown).
	Applier Applier
	Plugins PluginVersionReader

	In  io.Reader
	Out io.Writer
	Err io.Writer
}

type command struct {
	run   func(a *App, ctx context.Context, args []string) int
	usage string
}

var commands = map[string]command{
	"run":     {(*App).cmdRun, "run start|phase|event|label|loopback|verdict|override|ask [list]|checkpoint <phase>|decide --topic --note [--by lead|user]|brief [--json]|end [--no-clean]|dir|env [--shell s]|clean [--dry-run] [--json] …  record the workflow; ask/checkpoint/decide/brief: the lead's checkpoints, decisions and re-anchor; env/clean: run-scoped caches and artifact cleanup"},
	"go":      {cmdGo, "go <args…>   go with the active run's GOCACHE/GOTMPDIR (plain go without a run)"},
	"exec":    {cmdExec, "exec [--pack id] -- <cmd…>   a command with the active run's cache env and cache_argv for its language pack (plain without a run)"},
	"gate":    {(*App).cmdGate, "gate [--pkgs p,…] [--only c,…] [--skip c,…] [--enable c,…] [--base ref] [--vuln] [--security] [--fail-fast]   every detected language pack's checks (loops lang show <id>): build, format, lint, arch, tests under the race policy, coverage of new code (≥70%), vuln when enabled; --security runs only the packs' security checks"},
	"ticket":  {(*App).cmdTicket, "ticket fetch [REF] [--source s] [--body-file f|-]"},
	"arch":    {(*App).cmdArch, "arch [--json] [--root dir]"},
	"status":  {(*App).cmdStatus, "status [--json|--statusline|--subagent]"},
	"report":  {(*App).cmdReport, "report [--run id] [--out file]"},
	"hook":    {(*App).cmdHook, "hook <name> [--platform p]   (agent hooks; reads hook JSON on stdin)"},
	"port":    {(*App).cmdPort, "port build|check [--out dir] [--platform p,…] [--models file] [--src dir]   generate per-platform packages"},
	"install": {(*App).cmdInstall, "install --target <platform> [--scope project|user] [--project dir] [--force] [--dry-run]"},
	"hw":      {(*App).cmdHW, "hw [--json] [--write]   hardware facts + parallelism recommendation for the planner"},
	"budget":  {(*App).cmdBudget, "budget check [--phase p] [--spawn n] [--role r] [--verbose] | show | parse [--budget …] [--agents …] [--roles …] [--max-agents n]   opt-in limits (exit 3 = over budget)"},
	"lang":    {cmdLang, "lang list [--json] | show <id> [--variant v] | validate [pack.json…] | detect [--json] [--write] [--root dir]   language packs (specs/languages/<id>/pack.json)"},
	"plan":    {(*App).cmdPlan, "plan check <plan.md> [--cap n] [--json]   waves vs parallelism cap and scope overlap"},
	"watch":   {(*App).cmdWatch, "watch [--interval 2s] [--once]   live run view in a terminal (any platform)"},
	"path":    {(*App).cmdPath, "path [root|specs|templates|skills|agents]"},
	"release": {cmdRelease, "release [auto|patch|minor|major] [--dry-run] [--push]   bump the version from the commits, regenerate packages, commit and tag"},
	"version": {(*App).cmdVersion, "version [--check [--platform p]]   print the version; --check asks the Go proxy for the latest release"},
	"update":  {(*App).cmdUpdate, "update [--platform p] [--apply]   print how to update loops (all platforms when unknown); --apply checks now and runs the platform's update of the installed plugin (specs/PLATFORMS.md §9)"},
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
