package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/arxdsilva/ai-loops/internal/platform/loopsdir"
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

// installHints are the per-platform next steps printed after an install.
var installHints = map[plugin.Platform]string{
	plugin.Codex:       "Codex: restart Codex, then trust the plugin hooks with /hooks",
	plugin.OpenCode:    "OpenCode: restart OpenCode; commands appear as /loops/<name>",
	plugin.Amp:         "Amp: start a new thread; skills load from .agents/skills",
	plugin.Windsurf:    "Windsurf: reload the window; run the work skill in solo mode",
	plugin.Cline:       "Cline: reload the window; run the work skill in solo mode",
	plugin.Antigravity: "Antigravity: run /skills reload in Antigravity app or CLI (`agy`) to load skills",
}

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
		Module: "github.com/arxdsilva/ai-loops",
	})
}

// cmdInstall implements `loops install`.
func (a *App) cmdInstall(ctx context.Context, args []string) int {
	return a.install(args, buildPackage)
}

type installOpts struct {
	pl      plugin.Platform
	scope   string
	projDir string
	home    string
	base    string
	force   bool
	dry     bool
}

func (a *App) install(args []string, build packageBuilder) int {
	o, code := a.parseInstallOpts(args)
	if code != 0 {
		return code
	}
	if a.Source == nil || a.Packages == nil {
		fmt.Fprintln(a.Err, "loops install: plugin sources unavailable")
		return 1
	}

	switch o.pl {
	case plugin.Claude, plugin.Copilot, plugin.Cursor, plugin.Gemini:
		// native install: print the command only, without building the package.
		_, notes, err := port.InstallPlan(o.pl, nil, o.scope, o.projDir, o.home)
		if err != nil {
			fmt.Fprintln(a.Err, "loops install:", err)
			return 1
		}
		printPrefixed(a.Out, "note: ", notes)
		return 0
	}

	files, warns, err := build(a.Source, o.pl)
	if err != nil {
		fmt.Fprintln(a.Err, "loops install:", err)
		return 1
	}
	placements, notes, err := port.InstallPlan(o.pl, files, o.scope, o.projDir, o.home)
	if err != nil {
		fmt.Fprintln(a.Err, "loops install:", err)
		return 1
	}
	printPrefixed(a.Err, "warning: ", warns)
	return a.applyInstall(o, placements, notes)
}

func (a *App) parseInstallOpts(args []string) (installOpts, int) {
	fset := flag.NewFlagSet("install", flag.ContinueOnError)
	fset.SetOutput(io.Discard)
	target := fset.String("target", "", "")
	scope := fset.String("scope", port.ScopeProject, "")
	project := fset.String("project", "", "")
	force := fset.Bool("force", false, "")
	dry := fset.Bool("dry-run", false, "")
	if err := fset.Parse(args); err != nil || *target == "" || fset.NArg() > 0 {
		fmt.Fprintln(a.Err, installUsage)
		return installOpts{}, 2
	}
	pl, ok := parsePlatform(*target)
	if !ok {
		fmt.Fprintf(a.Err, "loops install: unknown platform %q (want one of %s)\n", *target, platformList())
		return installOpts{}, 2
	}
	if *scope != port.ScopeProject && *scope != port.ScopeUser {
		fmt.Fprintf(a.Err, "loops install: unknown scope %q (want project or user)\n", *scope)
		return installOpts{}, 2
	}
	projDir, home, base, err := resolveInstallDirs(*scope, *project, a.Root)
	if err != nil {
		fmt.Fprintln(a.Err, "loops install:", err)
		return installOpts{}, 1
	}
	return installOpts{pl: pl, scope: *scope, projDir: projDir, home: home, base: base, force: *force, dry: *dry}, 0
}

func resolveInstallDirs(scope, project, root string) (projDir, home, base string, err error) {
	projDir = project
	if projDir == "" {
		projDir = root
	}
	if projDir != "" {
		if abs, absErr := filepath.Abs(projDir); absErr == nil {
			projDir = abs
		}
	}
	base = projDir
	if scope == port.ScopeUser {
		h, hErr := os.UserHomeDir()
		if hErr != nil {
			return "", "", "", hErr
		}
		home, base = h, h
	}
	return projDir, home, base, nil
}

func (a *App) applyInstall(o installOpts, placements []plugin.Placement, notes []string) int {
	pluginHome := port.PluginHome(o.scope, o.base)
	manifest := filepath.Join(pluginHome, manifestFile(o.pl))
	placements, stale := a.preparePlacements(o, pluginHome, manifest, placements)
	if o.dry {
		runDryInstall(a.Out, o.pl, o.scope, stale, placements)
	} else if len(placements) > 0 {
		if code := a.writePlacements(o, manifest, placements, stale); code != 0 {
			return code
		}
	}
	printPrefixed(a.Out, "note: ", notes)
	if h, ok := installHints[o.pl]; ok && !o.dry {
		fmt.Fprintln(a.Out, "next: "+h)
	}
	return 0
}

func (a *App) preparePlacements(o installOpts, pluginHome, manifest string, placements []plugin.Placement) ([]plugin.Placement, []string) {
	placements, protected := protectForeignConfigs(placements, pluginHome, manifest, o.base)
	placements, linkNotes := guardSymlinks(o.base, o.scope, o.force, placements)
	printPrefixed(a.Out, "note: ", linkNotes)
	stale, staleNotes := staleFiles(manifest, o.base, o.pl, o.scope, placements)
	printPrefixed(a.Out, "note: ", staleNotes)
	for _, p := range protected {
		fmt.Fprintf(a.Out, "note: %s exists and was not created by loops; not overwritten. Merge the `loops hook` entries from the package by hand\n", p)
	}
	return placements, stale
}

func printPrefixed(w io.Writer, prefix string, lines []string) {
	for _, line := range lines {
		fmt.Fprintln(w, prefix+line)
	}
}

func runDryInstall(out io.Writer, pl plugin.Platform, scope string, stale []string, placements []plugin.Placement) {
	printPrefixed(out, "would remove stale: ", stale)
	fmt.Fprintf(out, "loops install --target %s --scope %s (dry run): %d file(s)\n", pl, scope, len(placements))
	for _, p := range placements {
		fmt.Fprintln(out, "  "+p.Dest)
	}
}

func (a *App) writePlacements(o installOpts, manifest string, placements []plugin.Placement, stale []string) int {
	written, err := a.Packages.Place(placements, o.force)
	if err != nil {
		fmt.Fprintln(a.Err, "loops install:", err)
		return 1
	}
	if o.scope == port.ScopeProject {
		_ = loopsdir.EnsureIgnore(o.projDir) // the plugin home under .loops/ stays out of git
	}
	removeStale(a.Out, stale)
	writeInstallManifest(a.Err, a.Packages, o, manifest, placements)
	fmt.Fprintf(a.Out, "installed %s (%s scope): %d written, %d skipped (existing; use --force to overwrite)\n",
		o.pl, o.scope, len(written), len(placements)-len(written))
	printPrefixed(a.Out, "  ", written)
	return 0
}

func removeStale(out io.Writer, stale []string) {
	for _, s := range stale {
		if fi, err := os.Lstat(s); err != nil || !fi.Mode().IsRegular() {
			continue // re-checked right before removal: gone, replaced by a symlink or a directory
		}
		if os.Remove(s) == nil {
			fmt.Fprintln(out, "removed stale: "+s)
		}
	}
}

func writeInstallManifest(errOut io.Writer, pkgs PackageWriter, o installOpts, manifest string, placements []plugin.Placement) {
	mp, err := manifestPlacement(manifest, o.base, placements)
	if err != nil {
		return
	}
	guarded, _ := guardSymlinks(o.base, o.scope, true, []plugin.Placement{mp})
	if _, err := pkgs.Place(guarded, true); err != nil {
		fmt.Fprintln(errOut, "loops install: writing manifest:", err)
	}
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
	case plugin.Antigravity:
		dirs = append(dirs, ".agents", ".gemini/config")
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
	clean, why := validEntryRel(e, dirs)
	if why != "" {
		return "", why
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

func validEntryRel(e manifestEntry, dirs []string) (string, string) {
	if e.Path == "" || filepath.IsAbs(e.Path) || path.IsAbs(e.Path) {
		return "", "absolute or empty path"
	}
	clean := path.Clean(filepath.ToSlash(e.Path))
	if clean == ".." || strings.HasPrefix(clean, "../") || clean != filepath.ToSlash(e.Path) {
		return "", "path escapes the install base"
	}
	for _, d := range dirs {
		if strings.HasPrefix(clean, d+"/") {
			return clean, ""
		}
	}
	return "", "outside this platform's install directories"
}

// protectForeignConfigs drops JSON config placements outside pluginHome whose
// existing destination was not created by loops: those are never overwritten,
// even with --force.
func protectForeignConfigs(ps []plugin.Placement, pluginHome, manifest, base string) (keep []plugin.Placement, protected []string) {
	recorded := manifestHashes(manifest, base)
	for _, p := range ps {
		if !isLoopsOwnedJSON(p, pluginHome, recorded) {
			protected = append(protected, p.Dest)
			continue
		}
		keep = append(keep, p)
	}
	return keep, protected
}

func manifestHashes(manifest, base string) map[string]string {
	b, err := fs.ReadFile(os.DirFS(filepath.Dir(manifest)), filepath.Base(manifest))
	if err != nil {
		return nil
	}
	var m installManifest
	if json.Unmarshal(b, &m) != nil {
		return nil
	}
	out := make(map[string]string, len(m.Files))
	for _, e := range m.Files {
		if e.Path != "" && e.SHA256 != "" {
			out[filepath.Join(base, filepath.FromSlash(e.Path))] = e.SHA256
		}
	}
	return out
}

func isLoopsOwnedJSON(p plugin.Placement, pluginHome string, recorded map[string]string) bool {
	if !strings.HasSuffix(p.Dest, ".json") || underDir(p.Dest, pluginHome) {
		return true
	}
	b, err := os.ReadFile(p.Dest)
	if err != nil {
		return true
	}
	return bytes.Equal(b, p.File.Data) || hasLoopsHook(b) || recorded[p.Dest] == sum(b)
}

func underDir(dest, dir string) bool {
	if dir == "" {
		return false
	}
	rel, err := filepath.Rel(dir, dest)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func hasLoopsHook(b []byte) bool {
	s := string(b)
	return strings.Contains(s, "loops hook") ||
		strings.Contains(s, `bin/loops\" hook`) ||
		strings.Contains(s, `bin/loops" hook`)
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
		if denySymlink(base, realBase, scope, force, p.Dest) {
			notes = append(notes, fmt.Sprintf("not writing %s: path crosses a symlink", p.Dest))
			continue
		}
		keep = append(keep, p)
	}
	return keep, notes
}

func denySymlink(base, realBase, scope string, force bool, dest string) bool {
	rel, ok := relToBase(base, filepath.Dir(dest))
	if !ok {
		return false
	}
	cur, linked := deepestSymlinkCheck(base, rel)
	if fi, err := os.Lstat(dest); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		linked = true
	}
	if !linked {
		return false
	}
	if scope == port.ScopeProject {
		return true
	}
	return scope == port.ScopeUser && force && escapesBase(cur, realBase)
}

func relToBase(base, dir string) (string, bool) {
	rel, err := filepath.Rel(base, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

func deepestSymlinkCheck(base, rel string) (string, bool) {
	cur, linked := base, false
	if rel == "." {
		return cur, false
	}
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
	return cur, linked
}

func escapesBase(cur, realBase string) bool {
	real, err := filepath.EvalSymlinks(cur)
	return err != nil || !(real == realBase || strings.HasPrefix(real, realBase+string(filepath.Separator)))
}
