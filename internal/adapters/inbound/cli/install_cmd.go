package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/application/port"
	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
)

const installUsage = "usage: loops install --target <platform> [--scope project|user] [--project dir] [--force] [--dry-run]"

// packageBuilder builds target's package from the canonical sources.
type packageBuilder func(src fs.FS, target plugin.Platform) ([]plugin.File, []string, error)

// buildPackage loads the canonical plugin and builds target's package.
func buildPackage(src fs.FS, target plugin.Platform) ([]plugin.File, []string, error) {
	p, err := port.Load(src)
	if err != nil {
		return nil, nil, err
	}
	return port.Build(p, src, target, port.Options{
		Module:  "github.com/arxdsilva/ai-loops",
		Version: "v" + Version,
	})
}

// installHints are the per-platform next steps printed after an install.
var installHints = map[plugin.Platform]string{
	plugin.Codex:    "Codex: restart Codex, then trust the plugin hooks with /hooks",
	plugin.OpenCode: "OpenCode: restart OpenCode; commands appear as /loops/<name>",
	plugin.Amp:      "Amp: start a new thread; skills load from .agents/skills",
	plugin.Windsurf: "Windsurf: reload the window; run the work skill in solo mode",
	plugin.Cline:    "Cline: reload the window; run the work skill in solo mode",
}

// cmdInstall implements `loops install`.
func (a *App) cmdInstall(ctx context.Context, args []string) int {
	return a.install(args, buildPackage)
}

func (a *App) install(args []string, build packageBuilder) int {
	fset := flag.NewFlagSet("install", flag.ContinueOnError)
	fset.SetOutput(io.Discard)
	target := fset.String("target", "", "")
	scope := fset.String("scope", port.ScopeProject, "")
	project := fset.String("project", "", "")
	force := fset.Bool("force", false, "")
	dry := fset.Bool("dry-run", false, "")
	if err := fset.Parse(args); err != nil || *target == "" || fset.NArg() > 0 {
		fmt.Fprintln(a.Err, installUsage)
		return 2
	}
	pl, ok := parsePlatform(*target)
	if !ok {
		fmt.Fprintf(a.Err, "loops install: unknown platform %q (want one of %s)\n", *target, platformList())
		return 2
	}
	if *scope != port.ScopeProject && *scope != port.ScopeUser {
		fmt.Fprintf(a.Err, "loops install: unknown scope %q (want project or user)\n", *scope)
		return 2
	}
	if a.Source == nil || a.Packages == nil {
		fmt.Fprintln(a.Err, "loops install: plugin sources unavailable")
		return 1
	}
	projDir := *project
	if projDir == "" {
		projDir = a.Root
	}
	if projDir != "" {
		if abs, err := filepath.Abs(projDir); err == nil {
			projDir = abs
		}
	}
	home := ""
	if *scope == port.ScopeUser {
		h, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(a.Err, "loops install:", err)
			return 1
		}
		home = h
	}

	switch pl {
	case plugin.Claude, plugin.Copilot, plugin.Cursor, plugin.Gemini:
		// native install: print the command only, without building the package.
		_, notes, err := port.InstallPlan(pl, nil, *scope, projDir, home)
		if err != nil {
			fmt.Fprintln(a.Err, "loops install:", err)
			return 1
		}
		for _, n := range notes {
			fmt.Fprintln(a.Out, "note: "+n)
		}
		return 0
	}

	files, warns, err := build(a.Source, pl)
	if err != nil {
		fmt.Fprintln(a.Err, "loops install:", err)
		return 1
	}
	placements, notes, err := port.InstallPlan(pl, files, *scope, projDir, home)
	if err != nil {
		fmt.Fprintln(a.Err, "loops install:", err)
		return 1
	}
	for _, w := range warns {
		fmt.Fprintln(a.Err, "warning:", w)
	}

	base := projDir
	if *scope == port.ScopeUser {
		base = home
	}
	manifest := filepath.Join(port.PluginHome(*scope, base), manifestFile(pl))
	placements, protected := protectForeignConfigs(placements)
	placements, linkNotes := guardSymlinks(base, *scope, *force, placements)
	for _, n := range linkNotes {
		fmt.Fprintln(a.Out, "note: "+n)
	}
	stale, staleNotes := staleFiles(manifest, base, pl, *scope, placements)
	for _, n := range staleNotes {
		fmt.Fprintln(a.Out, "note: "+n)
	}
	for _, p := range protected {
		fmt.Fprintf(a.Out, "note: %s exists and was not created by loops; not overwritten. Merge the `loops hook` entries from the package by hand\n", p)
	}

	if *dry {
		for _, s := range stale {
			fmt.Fprintln(a.Out, "would remove stale: "+s)
		}
		fmt.Fprintf(a.Out, "loops install --target %s --scope %s (dry run): %d file(s)\n", pl, *scope, len(placements))
		for _, p := range placements {
			fmt.Fprintln(a.Out, "  "+p.Dest)
		}
	} else if len(placements) > 0 {
		written, err := a.Packages.Place(placements, *force)
		if err != nil {
			fmt.Fprintln(a.Err, "loops install:", err)
			return 1
		}
		for _, s := range stale {
			if fi, err := os.Lstat(s); err != nil || !fi.Mode().IsRegular() {
				continue // re-checked right before removal: gone, replaced by a symlink or a directory
			}
			if os.Remove(s) == nil {
				fmt.Fprintln(a.Out, "removed stale: "+s)
			}
		}
		if mp, err := manifestPlacement(manifest, base, placements); err == nil {
			guarded, _ := guardSymlinks(base, *scope, true, []plugin.Placement{mp})
			if _, err := a.Packages.Place(guarded, true); err != nil {
				fmt.Fprintln(a.Err, "loops install: writing manifest:", err)
			}
		}
		fmt.Fprintf(a.Out, "installed %s (%s scope): %d written, %d skipped (existing; use --force to overwrite)\n",
			pl, *scope, len(written), len(placements)-len(written))
		for _, d := range written {
			fmt.Fprintln(a.Out, "  "+d)
		}
	}
	for _, n := range notes {
		fmt.Fprintln(a.Out, "note: "+n)
	}
	if h, ok := installHints[pl]; ok && !*dry {
		fmt.Fprintln(a.Out, "next: "+h)
	}
	return 0
}

func parsePlatform(s string) (plugin.Platform, bool) {
	for _, p := range plugin.Platforms {
		if string(p) == strings.ToLower(s) {
			return p, true
		}
	}
	return "", false
}

func platformList() string {
	names := make([]string, len(plugin.Platforms))
	for i, p := range plugin.Platforms {
		names[i] = string(p)
	}
	return strings.Join(names, ", ")
}

// manifestFile is the per-platform record of what an install placed, kept
// under the plugin home.
func manifestFile(platform plugin.Platform) string { return "installed-" + string(platform) + ".json" }

// manifestEntry is one placed file: a path relative to the install base and
// the sha256 of the content loops wrote.
type manifestEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type installManifest struct {
	Files []manifestEntry `json:"files"`
}

func sum(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func manifestPlacement(path, base string, ps []plugin.Placement) (plugin.Placement, error) {
	m := installManifest{Files: []manifestEntry{}}
	for _, p := range ps {
		rel, err := filepath.Rel(base, p.Dest)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		m.Files = append(m.Files, manifestEntry{filepath.ToSlash(rel), sum(p.File.Data)})
	}
	sort.Slice(m.Files, func(i, j int) bool { return m.Files[i].Path < m.Files[j].Path })
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return plugin.Placement{}, err
	}
	return plugin.Placement{Dest: path, File: plugin.File{Path: filepath.Base(path), Data: append(b, '\n')}}, nil
}

// platformDirs are the base-relative prefixes a platform's install may
// place into (and so the only places stale removal may touch).
func platformDirs(pl plugin.Platform, scope string) []string {
	dirs := []string{filepath.ToSlash(slashRel(port.PluginHome(scope, "")))}
	switch pl {
	case plugin.Codex:
		dirs = append(dirs, ".codex/agents")
	case plugin.OpenCode:
		dirs = append(dirs, ".opencode", ".config/opencode", ".agents/skills")
	case plugin.Amp:
		dirs = append(dirs, ".agents/skills")
	case plugin.Windsurf:
		dirs = append(dirs, ".agents/skills", ".devin", ".config/devin")
	case plugin.Cline:
		dirs = append(dirs, ".agents/skills", ".cline/skills")
	}
	return dirs
}

func slashRel(p string) string { return strings.TrimPrefix(filepath.ToSlash(p), "/") }

// staleFiles returns files a previous install of this platform placed that
// the current one does not, as safe-to-delete absolute paths. A malformed
// manifest, or an entry that escapes the base or the platform's directories,
// crosses a symlink, or whose content no longer matches the recorded hash, is
// skipped with a note.
func staleFiles(manifest, base string, pl plugin.Platform, scope string, ps []plugin.Placement) (stale, notes []string) {
	b, err := os.ReadFile(manifest)
	if err != nil {
		return nil, nil
	}
	var old installManifest
	if json.Unmarshal(b, &old) != nil {
		return nil, []string{"ignoring malformed " + manifest + "; nothing removed"}
	}
	cur := map[string]bool{}
	for _, p := range ps {
		cur[p.Dest] = true
	}
	dirs := platformDirs(pl, scope)
	for _, e := range old.Files {
		abs, why := safeEntry(base, e, dirs)
		if why != "" {
			notes = append(notes, fmt.Sprintf("not removing %q: %s", e.Path, why))
			continue
		}
		if !cur[abs] {
			stale = append(stale, abs)
		}
	}
	return stale, notes
}

// safeEntry validates one manifest entry and returns its absolute path.
func safeEntry(base string, e manifestEntry, dirs []string) (string, string) {
	if e.Path == "" || filepath.IsAbs(e.Path) || path.IsAbs(e.Path) {
		return "", "absolute or empty path"
	}
	clean := path.Clean(filepath.ToSlash(e.Path))
	if clean == ".." || strings.HasPrefix(clean, "../") || clean != filepath.ToSlash(e.Path) {
		return "", "path escapes the install base"
	}
	in := false
	for _, d := range dirs {
		in = in || strings.HasPrefix(clean, d+"/")
	}
	if !in {
		return "", "outside this platform's install directories"
	}
	cur := base
	for _, part := range strings.Split(clean, "/") {
		cur = filepath.Join(cur, part)
		fi, err := os.Lstat(cur)
		if err != nil {
			return "", "already gone"
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return "", "path crosses a symlink"
		}
	}
	data, err := os.ReadFile(cur)
	if err != nil || sum(data) != e.SHA256 {
		return "", "content changed since loops wrote it"
	}
	return cur, ""
}

// protectForeignConfigs drops JSON config placements whose destination
// exists and was not created by loops (no `loops hook` inside): those are
// never overwritten, even with --force.
func protectForeignConfigs(ps []plugin.Placement) (keep []plugin.Placement, protected []string) {
	for _, p := range ps {
		if strings.HasSuffix(p.Dest, ".json") {
			if b, err := os.ReadFile(p.Dest); err == nil && !strings.Contains(string(b), "loops hook") {
				protected = append(protected, p.Dest)
				continue
			}
		}
		keep = append(keep, p)
	}
	return keep, protected
}

// guardSymlinks drops placements whose destination path crosses a symlink
// below base. Project scope never follows one (a cloned repo could point
// .agents outside the project). User scope may follow symlinks inside $HOME
// (dotfile setups) but, with force, never when the resolved path leaves it.
func guardSymlinks(base, scope string, force bool, ps []plugin.Placement) (keep []plugin.Placement, notes []string) {
	realBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		realBase = base
	}
	for _, p := range ps {
		rel, err := filepath.Rel(base, filepath.Dir(p.Dest))
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			keep = append(keep, p)
			continue
		}
		cur, linked := base, false
		if rel != "." {
			for _, part := range strings.Split(rel, string(filepath.Separator)) {
				next := filepath.Join(cur, part)
				fi, err := os.Lstat(next)
				if err != nil {
					break
				}
				cur = next // deepest existing component
				if fi.Mode()&os.ModeSymlink != 0 {
					linked = true
				}
			}
		}
		if fi, err := os.Lstat(p.Dest); err == nil && fi.Mode()&os.ModeSymlink != 0 {
			linked = true
		}
		deny := linked && scope == port.ScopeProject
		if linked && scope == port.ScopeUser && force {
			real, err := filepath.EvalSymlinks(cur)
			if err != nil || !(real == realBase || strings.HasPrefix(real, realBase+string(filepath.Separator))) {
				deny = true
			}
		}
		if deny {
			notes = append(notes, fmt.Sprintf("not writing %s: path crosses a symlink", p.Dest))
			continue
		}
		keep = append(keep, p)
	}
	return keep, notes
}
