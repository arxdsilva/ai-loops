package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/application/port"
	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
)

const portUsage = "usage: loops port build|check [--out dir] [--platform p,…] [--models file.json] [--src dir]"

// portPlan is one parsed `loops port` invocation.
type portPlan struct {
	src       fs.FS
	out       string // platforms dir
	root      string // repository root: the parent of out
	customOut bool   // --out given: the root marketplaces are skipped
	platforms []plugin.Platform
	models    map[plugin.Platform]map[plugin.Tier]string
}

// cmdPort implements `loops port build|check` (specs/PLATFORMS.md):
// build writes platforms/<name>/ for each platform plus the repo-root
// marketplace files; check builds in memory and exits 1 on drift.
func (a *App) cmdPort(ctx context.Context, args []string) int {
	if len(args) == 0 || (args[0] != "build" && args[0] != "check") {
		fmt.Fprintln(a.Err, portUsage)
		return 2
	}
	sub := args[0]
	plan, code := a.portPlan(args[1:])
	if code != 0 {
		return code
	}
	p, err := port.Load(plan.src)
	if err != nil {
		fmt.Fprintln(a.Err, "loops port:", err)
		return 1
	}
	var drift []string
	for _, pl := range plan.platforms {
		files, warns, err := port.Build(p, plan.src, pl, port.Options{Models: plan.models[pl]})
		if err != nil {
			fmt.Fprintf(a.Err, "loops port: %s: %v\n", pl, err)
			return 1
		}
		dir := filepath.Join(plan.out, string(pl))
		if sub == "check" {
			diff, err := a.Packages.Diff(dir, files)
			if err != nil {
				fmt.Fprintf(a.Err, "loops port: %s: %v\n", pl, err)
				return 1
			}
			for _, d := range diff {
				drift = append(drift, filepath.ToSlash(filepath.Join(relTo(plan.root, dir), d)))
			}
			continue
		}
		if err := a.Packages.Write(dir, files); err != nil {
			fmt.Fprintf(a.Err, "loops port: %s: %v\n", pl, err)
			return 1
		}
		for _, w := range warns {
			fmt.Fprintf(a.Err, "loops port: warning: %s\n", w)
		}
		fmt.Fprintf(a.Out, "%s: %d files, %d warnings\n", pl, len(files), len(warns))
	}
	return a.portRootFiles(sub, plan, p, drift)
}

// portRootFiles writes (build) or compares (check) the repo-root
// marketplace files, then reports check drift.
func (a *App) portRootFiles(sub string, plan portPlan, p plugin.Plugin, drift []string) int {
	roots := port.RootFiles(p)
	if plan.customOut {
		// A custom --out is not <repo>/platforms, so its parent is not the
		// repository root the marketplaces belong to.
		fmt.Fprintln(a.Out, "root marketplaces skipped (custom --out)")
		roots = nil
	}
	if sub == "build" && len(roots) == 0 {
		return 0
	}
	if sub == "build" {
		var placements []plugin.Placement
		for _, f := range roots {
			placements = append(placements, plugin.Placement{Dest: filepath.Join(plan.root, filepath.FromSlash(f.Path)), File: f})
		}
		if _, err := a.Packages.Place(placements, true); err != nil {
			fmt.Fprintln(a.Err, "loops port: root files:", err)
			return 1
		}
		fmt.Fprintf(a.Out, "root: %d files\n", len(roots))
		return 0
	}
	for _, f := range roots {
		got, err := os.ReadFile(filepath.Join(plan.root, filepath.FromSlash(f.Path)))
		if err != nil || !bytes.Equal(got, f.Data) {
			drift = append(drift, f.Path)
		}
	}
	if len(drift) > 0 {
		sort.Strings(drift)
		fmt.Fprintf(a.Err, "loops port: %d generated files are stale; run `loops port build`:\n", len(drift))
		for _, d := range drift {
			fmt.Fprintln(a.Err, "  "+d)
		}
		return 1
	}
	fmt.Fprintln(a.Out, "port: packages up to date")
	return 0
}

// portPlan parses the flags; a non-zero code means usage error (already
// reported).
func (a *App) portPlan(args []string) (portPlan, int) {
	fset := a.newFlagSet("port")
	out := fset.String("out", "", "platforms directory (default <root>/platforms)")
	only := fset.String("platform", "", "comma-separated platforms (default all)")
	modelsFile := fset.String("models", "", `JSON tier→model map per platform: {"codex":{"opus":"<id>"}}`)
	srcDir := fset.String("src", "", "canonical plugin checkout (default: embedded sources)")
	pos, err := parseFlagsAnywhere(fset, args)
	if err != nil || len(pos) > 0 {
		fmt.Fprintln(a.Err, portUsage)
		return portPlan{}, 2
	}
	plan := portPlan{src: a.Source, out: *out, customOut: *out != ""}
	if *srcDir != "" {
		plan.src = os.DirFS(*srcDir)
	}
	if plan.src == nil {
		fmt.Fprintln(a.Err, "loops port: no plugin sources (pass --src)")
		return portPlan{}, 1
	}
	if plan.out == "" {
		plan.out = filepath.Join(a.Root, "platforms")
	}
	if plan.out, err = filepath.Abs(plan.out); err != nil {
		fmt.Fprintln(a.Err, "loops port:", err)
		return portPlan{}, 1
	}
	plan.root = filepath.Dir(plan.out) // the repo root when out is the default
	if plan.platforms, err = parsePlatforms(*only); err != nil {
		fmt.Fprintln(a.Err, "loops port:", err)
		return portPlan{}, 2
	}
	if *modelsFile != "" {
		if plan.models, err = readModels(*modelsFile); err != nil {
			fmt.Fprintln(a.Err, "loops port:", err)
			return portPlan{}, 2
		}
	}
	return plan, 0
}

func parsePlatforms(s string) ([]plugin.Platform, error) {
	if strings.TrimSpace(s) == "" {
		return plugin.Platforms[:], nil
	}
	var out []plugin.Platform
	for _, n := range splitList(s) {
		pl, ok := knownPlatform(n)
		if !ok {
			return nil, fmt.Errorf("unknown platform %q (want one of %s)", n, platformNames())
		}
		out = append(out, pl)
	}
	return out, nil
}

func knownPlatform(n string) (plugin.Platform, bool) {
	for _, p := range plugin.Platforms {
		if string(p) == n {
			return p, true
		}
	}
	return "", false
}

func platformNames() string {
	var names []string
	for _, p := range plugin.Platforms {
		names = append(names, string(p))
	}
	return strings.Join(names, ", ")
}

// readModels reads a --models file: {"<platform>":{"haiku|sonnet|opus":"<model id>"}}.
func readModels(name string) (map[plugin.Platform]map[plugin.Tier]string, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	var raw map[string]map[string]string
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	out := map[plugin.Platform]map[plugin.Tier]string{}
	for pn, tiers := range raw {
		pl, ok := knownPlatform(pn)
		if !ok {
			return nil, fmt.Errorf("%s: unknown platform %q", name, pn)
		}
		out[pl] = map[plugin.Tier]string{}
		for tn, id := range tiers {
			switch t := plugin.Tier(tn); t {
			case plugin.TierHaiku, plugin.TierSonnet, plugin.TierOpus:
				out[pl][t] = id
			default:
				return nil, fmt.Errorf("%s: %s: unknown tier %q (want haiku, sonnet or opus)", name, pn, tn)
			}
		}
	}
	return out, nil
}

// relTo returns target relative to base, or target when it is not below it.
func relTo(base, target string) string {
	r, err := filepath.Rel(base, target)
	if err != nil || strings.HasPrefix(r, "..") {
		return target
	}
	return r
}

// RegeneratePackages runs `loops port build` for the checkout at root, reading
// the canonical sources from disk (not the embedded copy) so a just-edited
// plugin.json is honoured. Progress goes to out; failures to errw.
func RegeneratePackages(root string, pk PackageWriter, out, errw io.Writer) error {
	a := &App{Source: os.DirFS(root), Packages: pk, Root: root, Out: out, Err: errw}
	if code := a.cmdPort(context.Background(), []string{"build"}); code != 0 {
		return fmt.Errorf("loops port build exited %d", code)
	}
	return nil
}
