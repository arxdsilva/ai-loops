package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/application/gate"
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

func validCheck(n string) bool {
	for _, v := range gate.CheckNames {
		if v == n {
			return true
		}
	}
	return false
}

const gateUsage = "usage: loops gate [--pkgs p,…] [--only c,…] [--skip c,…] [--base ref] [--vuln] [--fail-fast]"

// cmdGate implements `loops gate`: build, gofmt, vet, staticcheck, arch,
// test (-race, with a coverprofile), coverage of the new code against
// --base (default: merge-base with origin/main), govulncheck.
func (a *App) cmdGate(ctx context.Context, args []string) int {
	fs := a.newFlagSet("gate")
	pkgs := fs.String("pkgs", "", "comma-separated package patterns (default ./...)")
	only := fs.String("only", "", "run only these checks")
	skip := fs.String("skip", "", "skip these checks (coverage is required and cannot be skipped)")
	base := fs.String("base", "", "git ref new code is diffed against for coverage (default: gate.base, else merge-base with origin/main)")
	vuln := fs.Bool("vuln", false, "include govulncheck")
	failFast := fs.Bool("fail-fast", false, "stop at the first failing check")
	pos, err := parseFlagsAnywhere(fs, args)
	if err != nil {
		return 2
	}
	if len(pos) > 0 {
		return a.usageErr(gateUsage)
	}
	onlyL, skipL := splitList(*only), splitList(*skip)
	for _, n := range append(append([]string{}, onlyL...), skipL...) {
		if !validCheck(n) {
			return a.usageErr(fmt.Sprintf("gate: unknown check %q (valid: %s)", n, strings.Join(gate.CheckNames, ", ")))
		}
	}
	for _, n := range skipL {
		if n == gate.CheckCoverage {
			return a.usageErr("gate: coverage is required and cannot be skipped (scope a QA run with --only instead)")
		}
	}
	prof, err := os.CreateTemp("", "loops-cover-*.out")
	if err != nil {
		fmt.Fprintf(a.Err, "loops gate: coverprofile: %v\n", err)
		return 1
	}
	prof.Close()
	defer os.Remove(prof.Name())
	res, err := a.Gate.Run(ctx, gate.Options{
		Root:         a.Root,
		Pkgs:         splitList(*pkgs),
		Only:         onlyL,
		Skip:         skipL,
		ConfigSkip:   a.GateSkip,
		Vuln:         *vuln || a.GateVuln,
		FailFast:     *failFast,
		CoverProfile: prof.Name(),
		Base:         *base,
		Env:          wsCmd{a}.gateEnv(ctx),
	})
	if err != nil {
		fmt.Fprintf(a.Err, "loops gate: %v\n", err)
		return 1
	}
	for _, c := range res.Checks {
		switch {
		case c.Skipped:
			fmt.Fprintf(a.Out, "- %s (skipped)\n", c.Name)
			if c.Output != "" {
				fmt.Fprintf(a.Out, "    %s\n", c.Output)
			}
		case c.OK:
			fmt.Fprintf(a.Out, "✓ %s %.1fs\n", c.Name, c.Duration.Seconds())
			indent(a.Out, c.Output) // the coverage table
		default:
			fmt.Fprintf(a.Out, "✗ %s %.1fs\n", c.Name, c.Duration.Seconds())
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
