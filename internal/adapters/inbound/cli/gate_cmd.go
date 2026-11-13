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

const gateUsage = "usage: loops gate [--pkgs p,…] [--only c,…] [--skip c,…] [--enable c,…] [--base ref] [--vuln] [--security] [--fail-fast]"

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

// gateCmd holds the helpers of `loops gate` (keeps App's method count down).
type gateCmd struct{ a *App }

// gateFlags are the parsed `loops gate` flags.
type gateFlags struct {
	pkgs, only, skip, enable, base string
	vuln, security, failFast       bool
}

// cmdGate implements `loops gate`: every detected language pack's checks
// (for Go: build, gofmt, vet, staticcheck, arch, test -race with a
// coverprofile, coverage of the new code against --base, govulncheck).
func (a *App) cmdGate(ctx context.Context, args []string) int {
	f, code := gateCmd{a}.parseGate(args)
	if f == nil {
		return code
	}
	packs, code := gateCmd{a}.gatePacks()
	if code != 0 {
		return code
	}
	if msg := checkGateNames(f, packs); msg != "" {
		return a.usageErr(msg)
	}
	res, code := gateCmd{a}.runGate(ctx, f, packs)
	if code != 0 {
		return code
	}
	printGateResult(a.Out, res)
	if !res.OK {
		return 1
	}
	return 0
}

// runGate builds the options (applying the security pass for --security,
// whose notes print first) and runs the gate; a non-zero code is the exit.
func (g gateCmd) runGate(ctx context.Context, f *gateFlags, packs []langpack.Detected) (gate.Result, int) {
	dir, err := os.MkdirTemp("", "loops-gate-*")
	if err != nil {
		fmt.Fprintf(g.a.Err, "loops gate: coverprofile: %v\n", err)
		return gate.Result{}, 1
	}
	defer os.RemoveAll(dir)
	opts, err := g.gateOptions(ctx, f, packs, dir)
	if err != nil {
		fmt.Fprintf(g.a.Err, "loops gate: coverprofile: %v\n", err)
		return gate.Result{}, 1
	}
	if f.security {
		var notes []string
		opts, notes = gate.SecurityPass(opts)
		for _, n := range notes {
			fmt.Fprintln(g.a.Out, n)
		}
	}
	res, err := g.a.Gate.Run(ctx, opts)
	if err != nil {
		fmt.Fprintf(g.a.Err, "loops gate: %v\n", err)
		return gate.Result{}, 1
	}
	return res, 0
}

// parseGate parses the flags; a nil result carries the exit code.
func (g gateCmd) parseGate(args []string) (*gateFlags, int) {
	fs := g.a.newFlagSet("gate")
	f := &gateFlags{}
	fs.StringVar(&f.pkgs, "pkgs", "", "comma-separated package patterns (default: each pack's scope, ./... for Go)")
	fs.StringVar(&f.only, "only", "", "run only these checks")
	fs.StringVar(&f.skip, "skip", "", "skip these checks (coverage is required and cannot be skipped)")
	fs.StringVar(&f.enable, "enable", "", "enable these default-off checks")
	fs.StringVar(&f.base, "base", "", "git ref new code is diffed against for coverage (default: gate.base, else merge-base with origin/main)")
	fs.BoolVar(&f.vuln, "vuln", false, "enable the vuln checks (govulncheck)")
	fs.BoolVar(&f.security, "security", false, "run only each pack's security checks (Go: govulncheck, gosec); a missing tool is skipped")
	fs.BoolVar(&f.failFast, "fail-fast", false, "stop at the first failing check")
	pos, err := parseFlagsAnywhere(fs, args)
	if err != nil {
		return nil, 2
	}
	if len(pos) > 0 {
		return nil, g.a.usageErr(gateUsage)
	}
	return f, 0
}

// gatePacks detects the language packs; a non-zero code is the exit code.
func (g gateCmd) gatePacks() ([]langpack.Detected, int) {
	if g.a.Langs == nil {
		return nil, 0
	}
	packs, err := g.a.Langs.Detect(g.a.Root, g.a.Languages)
	if err != nil {
		fmt.Fprintf(g.a.Err, "loops gate: language packs: %v\n", err)
		return nil, 1
	}
	if len(packs) == 0 {
		fmt.Fprintf(g.a.Err, "loops gate: %v\n", gate.ErrNoPack)
		return nil, 1
	}
	return packs, 0
}

// checkGateNames returns the usage error for an unknown check name or a
// required check in --skip, else "".
func checkGateNames(f *gateFlags, packs []langpack.Detected) string {
	valid, required := gateChecks(packs)
	skipL := splitList(f.skip)
	for _, n := range slices.Concat(splitList(f.only), skipL, splitList(f.enable)) {
		if !slices.Contains(valid, n) {
			return fmt.Sprintf("gate: unknown check %q (valid: %s)", n, strings.Join(valid, ", "))
		}
	}
	for _, n := range skipL {
		if slices.Contains(required, n) {
			return "gate: " + n + " is required and cannot be skipped (scope a QA run with --only instead)"
		}
	}
	return ""
}

// gateOptions builds the gate options, creating the coverprofile in dir.
func (g gateCmd) gateOptions(ctx context.Context, f *gateFlags, packs []langpack.Detected, dir string) (gate.Options, error) {
	prof := filepath.Join(dir, "cover.out")
	if err := os.WriteFile(prof, nil, 0o600); err != nil {
		return gate.Options{}, err
	}
	env := wsCmd{g.a}.gateEnv(ctx)
	runDir := ""
	if env != nil {
		runDir, _, _, _ = wsCmd{g.a}.runEnv(ctx)
		runDir = scratch.Dir(runDir) // each pack's caches live in the run's scratch dir
	}
	return gate.Options{
		Root:         g.a.Root,
		Pkgs:         splitList(f.pkgs),
		Only:         splitList(f.only),
		Skip:         splitList(f.skip),
		Enable:       splitList(f.enable),
		ConfigSkip:   g.a.GateSkip,
		Vuln:         f.vuln || g.a.GateVuln,
		FailFast:     f.failFast,
		CoverProfile: prof,
		CoverDir:     filepath.Join(dir, "packs"),
		RunDir:       runDir,
		Base:         f.base,
		Env:          env,
		Packs:        packs,
	}, nil
}

// printGateResult prints one line per check, with its output indented.
func printGateResult(w io.Writer, res gate.Result) {
	for _, c := range res.Checks {
		name := c.Name
		if c.Pack != "" {
			name += " [" + c.Pack + "]"
		}
		switch {
		case c.Skipped:
			fmt.Fprintf(w, "- %s (skipped)\n", name)
			if c.Output != "" {
				fmt.Fprintf(w, "    %s\n", c.Output)
			}
		case c.OK:
			fmt.Fprintf(w, "✓ %s %.1fs\n", name, c.Duration.Seconds())
			indent(w, c.Output) // the coverage table
		default:
			fmt.Fprintf(w, "✗ %s %.1fs\n", name, c.Duration.Seconds())
			indent(w, c.Output)
		}
	}
}

// indent prints out's non-empty lines indented under a check line.
func indent(w io.Writer, out string) {
	for _, l := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if l != "" {
			fmt.Fprintf(w, "    %s\n", l)
		}
	}
}
