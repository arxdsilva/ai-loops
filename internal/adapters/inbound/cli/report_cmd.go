package cli

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// cmdReport implements `loops report [--run id] [--out file]`.
func (a *App) cmdReport(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	runID := fs.String("run", "", "run id (default: current)")
	out := fs.String("out", "", "output file (default: <run dir>/report.html)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(a.Err, "usage: loops report [--run id] [--out file]")
		return 2
	}
	st, err := a.Tracker.State(ctx, *runID)
	if err != nil {
		fmt.Fprintln(a.Err, "loops report:", err)
		return 1
	}
	events, err := a.Tracker.Events(ctx, *runID)
	if err != nil {
		fmt.Fprintln(a.Err, "loops report:", err)
		return 1
	}
	if len(events) == 0 {
		name := *runID
		if name == "" {
			name = "current"
		}
		fmt.Fprintf(a.Err, "loops report: run %q not found\n", name)
		return 1
	}
	dir, err := a.Tracker.Dir(ctx, *runID)
	if err != nil {
		fmt.Fprintln(a.Err, "loops report:", err)
		return 1
	}
	var buf bytes.Buffer
	if err := a.Report.Render(&buf, st, events, dir); err != nil {
		fmt.Fprintln(a.Err, "loops report:", err)
		return 1
	}
	path := *out
	if path == "" {
		path = filepath.Join(dir, "report.html")
	}
	if err := writeFileAtomic(path, buf.Bytes()); err != nil {
		fmt.Fprintln(a.Err, "loops report:", err)
		return 1
	}
	fmt.Fprintln(a.Out, path)
	return 0
}

func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".report-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Chmod(name, 0o644); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}
