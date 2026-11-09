package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/insurgencygames/ai-loops/internal/application/gate"
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

// cmdGate implements `loops gate [--pkgs p,…] [--only c,…] [--skip c,…] [--vuln] [--fail-fast]`.
func (a *App) cmdGate(ctx context.Context, args []string) int {
	fs := a.newFlagSet("gate")
	pkgs := fs.String("pkgs", "", "comma-separated package patterns (default ./...)")
	only := fs.String("only", "", "run only these checks")
	skip := fs.String("skip", "", "skip these checks")
	vuln := fs.Bool("vuln", false, "include govulncheck")
	failFast := fs.Bool("fail-fast", false, "stop at the first failing check")
	pos, err := parseFlagsAnywhere(fs, args)
	if err != nil {
		return 2
	}
	if len(pos) > 0 {
		return a.usageErr("usage: loops gate [--pkgs p,…] [--only c,…] [--skip c,…] [--vuln] [--fail-fast]")
	}
	onlyL, skipL := splitList(*only), splitList(*skip)
	for _, n := range append(append([]string{}, onlyL...), skipL...) {
		if !validCheck(n) {
			return a.usageErr(fmt.Sprintf("gate: unknown check %q (valid: %s)", n, strings.Join(gate.CheckNames, ", ")))
		}
	}
	res, err := a.Gate.Run(ctx, gate.Options{
		Root:       a.Root,
		Pkgs:       splitList(*pkgs),
		Only:       onlyL,
		Skip:       skipL,
		ConfigSkip: a.GateSkip,
		Vuln:       *vuln || a.GateVuln,
		FailFast:   *failFast,
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
		default:
			fmt.Fprintf(a.Out, "✗ %s %.1fs\n", c.Name, c.Duration.Seconds())
			for _, l := range strings.Split(strings.TrimRight(c.Output, "\n"), "\n") {
				if l != "" {
					fmt.Fprintf(a.Out, "    %s\n", l)
				}
			}
		}
	}
	if !res.OK {
		return 1
	}
	return 0
}
