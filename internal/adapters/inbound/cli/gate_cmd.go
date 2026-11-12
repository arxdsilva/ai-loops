package cli

import (
	"context"
	"fmt"
	"github.com/arxdsilva/ai-loops/internal/platform/scratch"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/application/gate"
	"github.com/arxdsilva/ai-loops/internal/application/langpack"
)

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// gateChecks lists the check names of the detected packs, in pipeline
// order, and those --skip may not drop (skippable: false); without packs,
// the Go pack's names.
func gateChecks(ds []langpack.Detected) (names, required []string) {
	if len(ds) == 0 {
		return gate.CheckNames, []string{gate.CheckCoverage}
	}
	for _, d := range ds {
		for _, c := range d.Pack.Checks {
			if !slices.Contains(names, c.Name) {
				names = append(names, c.Name)
			}
			if !c.IsSkippable() && !slices.Contains(required, c.Name) {
				required = append(required, c.Name)
			}
		}
	}
	return names, required
}

const gateUsage = "usage: loops gate [--pkgs p,…] [--only c,…] [--skip c,…] [--enable c,…] [--base ref] [--vuln] [--fail-fast]"

// cmdGate implements `loops gate`: every detected language pack's checks
// (for Go: build, gofmt, vet, staticcheck, arch, test -race with a
// coverprofile, coverage of the new code against --base, govulncheck).
func (a *App) cmdGate(ctx context.Context, args []string) int {
	fs := a.newFlagSet("gate")
	pkgs := fs.String("pkgs", "", "comma-separated package patterns (default: each pack's scope, ./... for Go)")
	only := fs.String("only", "", "run only these checks")
	skip := fs.String("skip", "", "skip these checks (coverage is required and cannot be skipped)")
	enable := fs.String("enable", "", "enable these default-off checks")
	base := fs.String("base", "", "git ref new code is diffed against for coverage (default: gate.base, else merge-base with origin/main)")
	vuln := fs.Bool("vuln", false, "enable the vuln checks (govulncheck)")
	failFast := fs.Bool("fail-fast", false, "stop at the first failing check")
	pos, err := parseFlagsAnywhere(fs, args)
	if err != nil {
		return 2
	}
	if len(pos) > 0 {
		return a.usageErr(gateUsage)
	}
	var packs []langpack.Detected
	if a.Langs != nil {
		if packs, err = a.Langs.Detect(a.Root, a.Languages); err != nil {
			fmt.Fprintf(a.Err, "loops gate: language packs: %v\n", err)
			return 1
		}
		if len(packs) == 0 {
			fmt.Fprintf(a.Err, "loops gate: %v\n", gate.ErrNoPack)
			return 1
		}
	}
	valid, required := gateChecks(packs)
	onlyL, skipL, enableL := splitList(*only), splitList(*skip), splitList(*enable)
	for _, n := range slices.Concat(onlyL, skipL, enableL) {
		if !slices.Contains(valid, n) {
			return a.usageErr(fmt.Sprintf("gate: unknown check %q (valid: %s)", n, strings.Join(valid, ", ")))
		}
	}
	for _, n := range skipL {
		if slices.Contains(required, n) {
			return a.usageErr("gate: " + n + " is required and cannot be skipped (scope a QA run with --only instead)")
		}
	}
	dir, err := os.MkdirTemp("", "loops-gate-*")
	if err != nil {
		fmt.Fprintf(a.Err, "loops gate: coverprofile: %v\n", err)
		return 1
	}
	defer os.RemoveAll(dir)
	prof := filepath.Join(dir, "cover.out")
	if err := os.WriteFile(prof, nil, 0o600); err != nil {
		fmt.Fprintf(a.Err, "loops gate: coverprofile: %v\n", err)
		return 1
	}
	env := wsCmd{a}.gateEnv(ctx)
	runDir := ""
	if env != nil {
		runDir, _, _, _ = wsCmd{a}.runEnv(ctx)
		runDir = scratch.Dir(runDir) // each pack's caches live in the run's scratch dir
	}
	res, err := a.Gate.Run(ctx, gate.Options{
		Root:         a.Root,
		Pkgs:         splitList(*pkgs),
		Only:         onlyL,
		Skip:         skipL,
		Enable:       enableL,
		ConfigSkip:   a.GateSkip,
		Vuln:         *vuln || a.GateVuln,
		FailFast:     *failFast,
		CoverProfile: prof,
		CoverDir:     filepath.Join(dir, "packs"),
		RunDir:       runDir,
		Base:         *base,
		Env:          env,
		Packs:        packs,
	})
	if err != nil {
		fmt.Fprintf(a.Err, "loops gate: %v\n", err)
		return 1
	}
	for _, c := range res.Checks {
		name := c.Name
		if c.Pack != "" {
			name += " [" + c.Pack + "]"
		}
		switch {
		case c.Skipped:
			fmt.Fprintf(a.Out, "- %s (skipped)\n", name)
			if c.Output != "" {
				fmt.Fprintf(a.Out, "    %s\n", c.Output)
			}
		case c.OK:
			fmt.Fprintf(a.Out, "✓ %s %.1fs\n", name, c.Duration.Seconds())
			indent(a.Out, c.Output) // the coverage table
		default:
			fmt.Fprintf(a.Out, "✗ %s %.1fs\n", name, c.Duration.Seconds())
			indent(a.Out, c.Output)
		}
	}
	if !res.OK {
		return 1
	}
	return 0
}

// indent prints out's non-empty lines indented under a check line.
func indent(w io.Writer, out string) {
	for _, l := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if l != "" {
			fmt.Fprintf(w, "    %s\n", l)
		}
	}
}
