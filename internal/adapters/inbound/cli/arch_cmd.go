package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/insurgencygames/ai-loops/internal/domain/archrule"
)

// cmdArch implements `loops arch [--json] [--root dir]`: it checks the
// module at root (default: the project root) against the hexagonal and
// SOLID rules and exits 1 when any error-severity finding remains.
func (a *App) cmdArch(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("arch", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	fs.Usage = func() {
		fmt.Fprintln(a.Err, "usage: loops arch [--json] [--root dir]")
		fmt.Fprintln(a.Err, "layers come from the current project's .loops/config.json, not from --root's")
		fs.PrintDefaults()
	}
	asJSON := fs.Bool("json", false, "print the report as JSON")
	root := fs.String("root", a.Root, "module root to check")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(a.Err, "loops arch: unexpected argument %q\n", fs.Arg(0))
		fs.Usage()
		return 2
	}
	if *root == "" {
		*root = "."
	}
	rep, err := a.Arch.Check(ctx, *root, a.Layers)
	if err != nil {
		fmt.Fprintf(a.Err, "loops arch: %v\n", err)
		return 1
	}
	if *asJSON {
		enc := json.NewEncoder(a.Out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			fmt.Fprintf(a.Err, "loops arch: %v\n", err)
			return 1
		}
	} else {
		writeArchText(a.Out, rep)
	}
	if !rep.OK() {
		return 1
	}
	return 0
}

// writeArchText prints `file:line rule severity msg`, errors first, then a
// summary line.
func writeArchText(w io.Writer, rep archrule.Report) {
	counts := map[archrule.Severity]int{}
	for _, sev := range []archrule.Severity{archrule.SeverityError, archrule.SeverityWarn} {
		for _, f := range rep.Findings {
			if f.Severity == sev {
				counts[sev]++
				fmt.Fprintf(w, "%s:%d %s %s %s\n", f.File, f.Line, f.Rule, f.Severity, f.Msg)
			}
		}
	}
	fmt.Fprintf(w, "arch: %d errors, %d warnings\n", counts[archrule.SeverityError], counts[archrule.SeverityWarn])
}
