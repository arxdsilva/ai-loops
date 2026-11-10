package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/arxdsilva/ai-loops/internal/application/archcheck"
	"github.com/arxdsilva/ai-loops/internal/application/langpack"
	"github.com/arxdsilva/ai-loops/internal/domain/archrule"
)

// PackArcher checks every detected language pack at its root
// (archcheck.Service implements it); an Archer without it checks Go only.
type PackArcher interface {
	CheckPacks(ctx context.Context, root string, layers archrule.Layers, ds []langpack.Detected) (archcheck.Result, error)
}

// archJSON is the --json output: the report plus the delegation notes.
type archJSON struct {
	archrule.Report
	Notes []string `json:"notes,omitempty"`
}

// cmdArch implements `loops arch [--json] [--root dir]`: it checks each
// language pack detected at root (default: the project root; Go only when
// no pack is detected) against the hexagonal and SOLID rules and exits 1
// when any error-severity finding remains. A pack without a built-in parser
// passes with a note naming its native check, which loops gate runs.
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
	rep, notes, err := archCheck(ctx, a, *root)
	if err != nil {
		fmt.Fprintf(a.Err, "loops arch: %v\n", err)
		return 1
	}
	if *asJSON {
		enc := json.NewEncoder(a.Out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(archJSON{Report: rep, Notes: notes}); err != nil {
			fmt.Fprintf(a.Err, "loops arch: %v\n", err)
			return 1
		}
	} else {
		for _, n := range notes {
			fmt.Fprintln(a.Out, n)
		}
		writeArchText(a.Out, rep)
	}
	if !rep.OK() {
		return 1
	}
	return 0
}

// archCheck runs the per-pack check when the checker supports it and some
// pack is detected, else the Go check of root.
func archCheck(ctx context.Context, a *App, root string) (archrule.Report, []string, error) {
	pa, ok := a.Arch.(PackArcher)
	if !ok || a.Langs == nil {
		rep, err := a.Arch.Check(ctx, root, a.Layers)
		return rep, nil, err
	}
	ds, err := a.Langs.Detect(root, a.Languages)
	if err != nil {
		fmt.Fprintf(a.Err, "loops arch: language detection: %v; checking the Go module only\n", err)
	}
	if err != nil || len(ds) == 0 {
		rep, err := a.Arch.Check(ctx, root, a.Layers)
		return rep, nil, err
	}
	res, err := pa.CheckPacks(ctx, root, a.Layers, ds)
	return res.Report, res.Notes, err
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
