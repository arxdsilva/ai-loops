package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
	"github.com/arxdsilva/ai-loops/internal/domain/runstats"
)

// RunLogs reads recorded runs from disk (outbound/runstore).
type RunLogs interface {
	// Dirs lists the run directories under root/.loops/runs, oldest first.
	Dirs(root string) ([]string, error)
	// Events reads one run directory's events.jsonl.
	Events(runDir string) ([]run.Event, error)
}

const statsUsage = "usage: loops stats [--json] [--root dir] [<run dir>...]"

// statsJSON is `loops stats --json`.
type statsJSON struct {
	Runs    []runstats.Run   `json:"runs"`
	Summary runstats.Summary `json:"summary"`
}

// cmdStats implements `loops stats`: measures runs from their events.jsonl
// (every run under <root>/.loops/runs, or the run dirs given).
func (a *App) cmdStats(_ context.Context, args []string) int {
	fs := a.newFlagSet("stats")
	asJSON := fs.Bool("json", false, "machine-readable")
	root := fs.String("root", a.Root, "project root whose .loops/runs to measure")
	dirs, err := parseFlagsAnywhere(fs, args)
	if err != nil {
		return 2
	}
	if a.Runs == nil {
		fmt.Fprintln(a.Err, "loops stats: unavailable (no run log reader)")
		return 1
	}
	if len(dirs) == 0 {
		if dirs, err = a.Runs.Dirs(*root); err != nil {
			fmt.Fprintf(a.Err, "loops stats: %v\n", err)
			return 1
		}
	}
	runs, err := a.loadRunStats(dirs)
	if err != nil {
		fmt.Fprintf(a.Err, "loops stats: %v\n", err)
		return 1
	}
	s := runstats.Summarize(runs)
	if *asJSON {
		if runs == nil {
			runs = []runstats.Run{}
		}
		b, _ := json.MarshalIndent(statsJSON{Runs: runs, Summary: s}, "", "  ")
		fmt.Fprintln(a.Out, string(b))
		return 0
	}
	if len(runs) == 0 {
		fmt.Fprintln(a.Out, "no runs found")
		return 0
	}
	printStats(a.Out, runs, s)
	return 0
}

func fmtMin(m float64) string { return fmt.Sprintf("%.1f", m) }

// fmtTok prints a token count as 12, 3.4k or 1.2M.
func fmtTok(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 10_000:
		return fmt.Sprintf("%dk", n/1000)
	case n >= 1000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	}
	return fmt.Sprint(n)
}

// printStats writes one line per run, then the footer.
func printStats(w io.Writer, runs []runstats.Run, s runstats.Summary) {
	for _, r := range runs {
		var ph []string
		for _, p := range run.Phases {
			if m, ok := r.PhaseMin[string(p)]; ok && m > 0 {
				ph = append(ph, fmt.Sprintf("%s:%s(%s)", p, fmtMin(m), fmtMin(r.LeadMin[string(p)])))
			}
		}
		fmt.Fprintf(w, "%s tier=%s wall=%sm agents=%d peak=%d tok=%s lead=%s lb=%s test=%dms denies=%d",
			r.ID, r.Tier, fmtMin(r.WallMin), r.Agents, r.Peak, fmtTok(r.TotalTokens()), fmtTok(r.LeadTokens),
			countsStr(r.LoopBacks), r.TestMS, sumCounts(r.Denies))
		if len(ph) > 0 {
			fmt.Fprintf(w, " | %s", strings.Join(ph, " "))
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "-- %d runs, median wall %sm; phases are min(min with no agent running)\n", s.Runs, fmtMin(s.MedianWallMin))
	if len(s.Phases) > 0 {
		parts := make([]string, 0, len(s.Phases))
		for _, p := range s.Phases {
			parts = append(parts, fmt.Sprintf("%s %s(%s)", p.Phase, fmtMin(p.Min), fmtMin(p.LeadMin)))
		}
		fmt.Fprintf(w, "median per phase: %s\n", strings.Join(parts, " "))
	}
	if len(s.Roles) > 0 {
		fmt.Fprintf(w, "%-24s %6s %8s %8s %6s\n", "role", "agents", "tokens", "median", "share")
		for _, r := range s.Roles {
			agents, med := "-", "-"
			if r.Role != runstats.LeadRole {
				agents, med = fmt.Sprint(r.Agents), fmtTok(r.Median)
			}
			fmt.Fprintf(w, "%-24s %6s %8s %8s %5.0f%%\n", r.Role, agents, fmtTok(r.Tokens), med, r.Share*100)
		}
	}
}

// countsStr prints a count map as "3" (the total) followed by the by-key
// breakdown when there is more than one key: "3 (qa-verifier:2,qa-quality:1)".
func countsStr(m map[string]int) string {
	total := sumCounts(m)
	if len(m) < 2 {
		return fmt.Sprint(total)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s:%d", k, m[k]))
	}
	return fmt.Sprintf("%d(%s)", total, strings.Join(parts, ","))
}

func sumCounts(m map[string]int) int {
	n := 0
	for _, c := range m {
		n += c
	}
	return n
}

func (a *App) loadRunStats(dirs []string) ([]runstats.Run, error) {
	var out []runstats.Run
	for _, d := range dirs {
		evs, err := a.Runs.Events(d)
		if err != nil {
			return nil, err
		}
		if len(evs) == 0 {
			continue
		}
		r := runstats.Compute(evs)
		if r.ID == "" {
			r.ID = filepath.Base(d)
		}
		out = append(out, r)
	}
	return out, nil
}
