package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/arxdsilva/ai-loops/internal/application/updatecheck"
	"github.com/arxdsilva/ai-loops/internal/domain/update"
)

// Applier checks for a release now and runs the platform's update of the
// installed plugin (updatecheck.Service). current is the installed plugin's
// version.
type Applier interface {
	Apply(ctx context.Context, current, platform string) updatecheck.Outcome
}

// PluginVersionReader returns the "version" of
// <root>/.claude-plugin/plugin.json.
type PluginVersionReader interface {
	PluginVersion(root string) (string, error)
}

// cmdUpdate implements `loops update [--platform p] [--apply]`: it prints
// the update command for the platform, or for every platform when it is
// unknown; --apply updates the installed plugin (specs/PLATFORMS.md §9).
func (a *App) cmdUpdate(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	platform := fs.String("platform", "", "")
	apply := fs.Bool("apply", false, "")
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 {
		fmt.Fprintln(a.Err, "usage: loops update [--platform p] [--apply]")
		return 2
	}
	p := strings.ToLower(*platform)
	if p == "" {
		p = detectPlatform(os.Getenv)
	}
	if *apply {
		return updateApply(ctx, a, p)
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

// applyBudget bounds one `loops update --apply` (check plus route).
const applyBudget = 60 * time.Second

// pluginInfo is the installed plugin as the CLI resolved it.
type pluginInfo struct {
	Root, Version string
	Source        string // "plugin" or "binary"
	Why           string // set when Source == "binary"
}

// pluginRootCandidates lists the plugin-root candidates in priority order
// (specs/PLATFORMS.md §9): the process's, the project's pointer, the user's.
func pluginRootCandidates(a *App) []string {
	var out []string
	add := func(s string) {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	add(a.PluginRoot)
	read := func(path string) {
		if b, err := os.ReadFile(path); err == nil {
			add(string(b))
		}
	}
	if a.Root != "" {
		read(filepath.Join(a.Root, ".loops", "plugin-root"))
	}
	state := os.Getenv("XDG_STATE_HOME")
	if state == "" {
		if h, err := os.UserHomeDir(); err == nil {
			state = filepath.Join(h, ".local", "state")
		}
	}
	if state != "" {
		read(filepath.Join(state, "loops", "plugin-root"))
	}
	return out
}

// resolvePlugin finds the installed plugin root and its version, falling
// back to the binary's version.
func resolvePlugin(a *App) pluginInfo {
	root := ""
	for _, c := range pluginRootCandidates(a) {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			root = c
			break
		}
	}
	if root == "" {
		return pluginInfo{Version: a.Version, Source: "binary", Why: "plugin root unknown"}
	}
	if a.Plugins != nil {
		if v, err := a.Plugins.PluginVersion(root); err == nil && v != "" {
			return pluginInfo{Root: root, Version: v, Source: "plugin"}
		}
	}
	return pluginInfo{Root: root, Version: a.Version, Source: "binary", Why: "no plugin.json at " + root}
}

// updateApply runs `loops update --apply` for platform p. It always
// exits 0 and prints exactly one `loops update: ...` line.
func updateApply(ctx context.Context, a *App, p string) int { // not an App method: App is at the SOLID method limit
	say := func(s string) int {
		fmt.Fprintln(a.Out, "loops update: "+s)
		return 0
	}
	if why := update.OptOut(os.Getenv("LOOPS_NO_UPDATE_CHECK"), os.Getenv("CI"), a.UpdateOff); why != "" {
		return say("skipped (" + why + ")")
	}
	if a.Applier == nil {
		return say("skipped (update check unavailable)")
	}
	info := resolvePlugin(a)
	if info.Root != "" {
		if _, err := os.Lstat(filepath.Join(info.Root, ".git")); err == nil {
			return say("skipped (dev checkout at " + info.Root + ")")
		}
	}
	if strings.TrimSpace(info.Version) == "" {
		return say("skipped (plugin version unknown)")
	}
	if strings.TrimSpace(info.Version) == "" {
		return say("skipped (plugin version unknown)")
	}
	if update.IsDev(info.Version) {
		return say("skipped (dev build " + update.Tag(info.Version) + ")")
	}
	cctx, cancel := context.WithTimeout(ctx, applyBudget)
	defer cancel()
	o := a.Applier.Apply(cctx, info.Version, p)
	return say(outcomeLine(o, info) + applySuffix(info, a.Version))
}

func outcomeLine(o updatecheck.Outcome, info pluginInfo) string {
	cur, latest := update.Tag(info.Version), update.Tag(o.Latest)
	at := func(t time.Time) string { return t.UTC().Format(time.RFC3339) }
	switch o.Kind {
	case updatecheck.KindUpToDate:
		return "up to date (" + cur + ")"
	case updatecheck.KindUpdated:
		return fmt.Sprintf("updated %s -> %s on %s; this session keeps the %s plugin until %s; the new plugin's bin/loops fetches the %s binary on first use", cur, latest, o.Platform, cur, o.Reload, latest)
	case updatecheck.KindPending:
		return fmt.Sprintf("%s was installed at %s; this session keeps the %s plugin until %s", latest, at(o.At), cur, o.Reload)
	case updatecheck.KindFailed:
		return fmt.Sprintf("update to %s failed (%s); continuing on %s; to update by hand: %s", latest, o.Detail, cur, o.Manual)
	case updatecheck.KindBackoff:
		return fmt.Sprintf("update to %s failed at %s (%s); next try after %s; to update by hand: %s", latest, at(o.At), o.Detail, at(o.RetryAt), o.Manual)
	case updatecheck.KindManual:
		plat := o.Platform
		if plat == "" {
			plat = "this platform"
		}
		return fmt.Sprintf("%s available (running %s); no automatic update on %s; to update: %s", latest, cur, plat, o.Manual)
	case updatecheck.KindCheckFailed:
		return fmt.Sprintf("check failed (%s); continuing on %s", o.Detail, cur)
	}
	return string(o.Kind)
}

// applySuffix names where the version came from and which loops to call
// when the running binary is not the plugin's.
func applySuffix(info pluginInfo, binVersion string) string {
	if info.Source == "binary" {
		return " [version of this loops binary: " + info.Why + "]"
	}
	if update.Tag(binVersion) != update.Tag(info.Version) {
		return fmt.Sprintf(" [this loops binary is %s; call %s/bin/loops]", update.Tag(binVersion), info.Root)
	}
	return ""
}
