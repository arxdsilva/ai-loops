package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/application/plancheck"
	"github.com/arxdsilva/ai-loops/internal/domain/hw"
)

// cmdPlan implements `loops plan check <plan.md> [--cap n] [--json]`.
func (a *App) cmdPlan(ctx context.Context, args []string) int {
	if len(args) == 0 || args[0] != "check" {
		return a.usageErr("usage: loops plan check <plan.md> [--cap n] [--json]")
	}
	fs := a.newFlagSet("plan check")
	capFlag := fs.Int("cap", 0, "agent cap (default: effective cap)")
	asJSON := fs.Bool("json", false, "print JSON")
	pos, err := parseFlagsAnywhere(fs, args[1:])
	if err != nil {
		return 2
	}
	if len(pos) != 1 {
		return a.usageErr("usage: loops plan check <plan.md> [--cap n] [--json]")
	}
	runDir, _ := a.Tracker.Dir(ctx, "")
	text, err := readPlan(pos[0], runDir)
	if err != nil {
		return a.usageErr(err.Error())
	}
	capN, source := *capFlag, "--cap"
	if capN <= 0 {
		capN, source = a.effectiveCap(ctx, runDir)
	}
	rep := plancheck.Check(text, capN)
	if *asJSON {
		out := struct {
			plancheck.Report
			CapSource string `json:"cap_source"`
		}{rep, source}
		if out.Findings == nil {
			out.Findings = []plancheck.Finding{}
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		fmt.Fprintln(a.Out, string(b))
	} else {
		fmt.Fprintf(a.Out, "cap %d (%s)\n", capN, source)
		for _, f := range rep.Findings {
			fmt.Fprintf(a.Out, "%d %s %s\n", f.Line, f.Rule, f.Msg)
		}
		fmt.Fprintf(a.Out, "peak %d agents (wave %s), %d finding(s)\n", rep.Peak, rep.PeakWave, len(rep.Findings))
	}
	if !rep.OK() {
		return 1
	}
	return 0
}

// readPlan reads path (relative: cwd first, then the run dir).
func readPlan(path, runDir string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil && !filepath.IsAbs(path) && runDir != "" {
		var err2 error
		if b, err2 = os.ReadFile(filepath.Join(runDir, path)); err2 == nil {
			err = nil
		}
	}
	if err != nil {
		return "", fmt.Errorf("cannot read plan: %v", err)
	}
	return string(b), nil
}

// effectiveCap is min(parallel.max_agents|100, hardware, limits.max_agents,
// run limits.max_agents) and a description of what bound it.
func (a *App) effectiveCap(ctx context.Context, runDir string) (int, string) {
	best, src := a.Parallel.MaxAgents, "config: parallel.max_agents"
	if best <= 0 {
		best, src = hw.DefaultMaxAgents, "default"
	}
	consider := func(n int, s string) {
		if n > 0 && n < best {
			best, src = n, s
		}
	}
	if n, reason, ok := a.hardwareCap(ctx, runDir); ok {
		consider(n, "hardware: "+reason)
	}
	consider(a.Limits.MaxAgents, "config: limits.max_agents")
	if st, err := a.Tracker.State(ctx, ""); err == nil {
		consider(st.Limits.MaxAgents, "run limits.max_agents")
	}
	return best, src
}

func (a *App) hardwareCap(ctx context.Context, runDir string) (int, string, bool) {
	if runDir != "" {
		if b, err := os.ReadFile(filepath.Join(runDir, "research", "hardware.json")); err == nil {
			var doc struct {
				Rec hw.Recommendation `json:"recommendation"`
			}
			if json.Unmarshal(b, &doc) == nil && doc.Rec.MaxParallelAgents > 0 {
				return doc.Rec.MaxParallelAgents, strings.TrimSpace(doc.Rec.Reason), true
			}
		}
	}
	if a.HW == nil {
		return 0, "", false
	}
	f, err := a.HW.Probe(ctx, a.Root)
	if err != nil {
		return 0, "", false
	}
	rec := hw.Recommend(f, a.Parallel)
	return rec.MaxParallelAgents, rec.Reason, rec.MaxParallelAgents > 0
}
