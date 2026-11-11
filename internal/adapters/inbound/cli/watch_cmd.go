package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

const (
	ansiClear  = "\x1b[H\x1b[2J"
	ansiBold   = "\x1b[1m"
	watchSteps = 8
)

// cmdWatch implements `loops watch [--interval d] [--once]`: a terminal live
// view of the current run for platforms without a status line or pane.
func (a *App) cmdWatch(ctx context.Context, args []string) int {
	fset := flag.NewFlagSet("watch", flag.ContinueOnError)
	fset.SetOutput(io.Discard)
	interval := fset.Duration("interval", 2*time.Second, "")
	once := fset.Bool("once", false, "")
	if err := fset.Parse(args); err != nil || fset.NArg() > 0 || *interval <= 0 {
		fmt.Fprintln(a.Err, "usage: loops watch [--interval 2s] [--once]")
		return 2
	}
	tty := isTTY(a.Out)

	if *once {
		st, _, active := a.currentState(ctx)
		fmt.Fprint(a.Out, renderWatch(st, active, tty))
		return 0
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()
	tick := time.NewTicker(*interval)
	defer tick.Stop()
	for {
		st, _, active := a.currentState(ctx)
		if tty {
			fmt.Fprint(a.Out, ansiClear)
		}
		fmt.Fprint(a.Out, renderWatch(st, active, tty))
		if active && st.Ended {
			fmt.Fprintln(a.Out, "\nrun ended; render the full report with: loops report")
			return 0
		}
		select {
		case <-ctx.Done():
			return 0
		case <-tick.C:
		}
	}
}

// isTTY reports whether w is a character device (a terminal).
func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// renderWatch builds one frame.
func renderWatch(s run.State, active, color bool) string {
	if !active {
		return paint(color, ansiDim, "loops · no active run (waiting)") + "\n"
	}
	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format, a...); b.WriteByte('\n') }

	// header
	state := paint(color, ansiYellow, "▶ running")
	if s.Ended {
		state = "■ " + s.Outcome
		if s.Outcome == "" {
			state = "■ ended"
		}
	}
	title := runTitle(s)
	if s.Title != "" && s.Title != title {
		title += " · " + truncRunes(s.Title, 60)
	}
	head := paint(color, ansiBold, title) + "  " + state
	if e := elapsed(s); e != "" {
		head += "  " + e
	}
	if s.PR != "" {
		head += "  PR " + s.PR
	}
	w("%s", head)
	w("%s", phaseStrip(s, color))

	if n := len(s.Overrides); n > 0 {
		last := s.Overrides[n-1]
		w("Overrides %d  last: %s → %s", n, truncRunes(last.LoopsRule, 30), truncRunes(last.ProjectRule, 40))
	}
	if lim := limitsLine(s); lim != "" {
		w("Limits  %s", lim)
		if budgetPaused(s) {
			b := s.Budgets[len(s.Budgets)-1]
			w("%s", paint(color, ansiYellow, "⏸ budget: "+truncRunes(b.Reason, 70)))
		}
	}

	// lanes
	if len(s.Lanes) > 0 {
		tokens := false
		for _, l := range s.Lanes {
			tokens = tokens || l.Tokens > 0
		}
		w("")
		hdr := fmt.Sprintf("%-18s %-24s %-4s %-7s %5s  %s", "ROLE", "LABEL", "WAVE", "STATE", "STEPS", "QA")
		if tokens {
			hdr = fmt.Sprintf("%-18s %-24s %-4s %-7s %5s %7s  %s", "ROLE", "LABEL", "WAVE", "STATE", "STEPS", "TOKENS", "QA")
		}
		w("%s", paint(color, ansiDim, hdr))
		for _, l := range s.Lanes {
			st := "done"
			if l.Running {
				st = paint(color, ansiYellow, "running")
				st += strings.Repeat(" ", 7-len("running"))
			} else {
				st = fmt.Sprintf("%-7s", st)
			}
			wave := ""
			if l.Wave > 0 {
				wave = fmt.Sprintf("W%d", l.Wave)
			}
			if tokens {
				tk := ""
				if l.Tokens > 0 {
					tk = fmtTokens(l.Tokens)
				}
				w("%-18s %-24s %-4s %s %5d %7s  %s", truncRunes(l.Role, 18), truncRunes(l.Label, 24), wave, st, l.Steps, tk, qaCell(l, color))
				continue
			}
			w("%-18s %-24s %-4s %s %5d  %s", truncRunes(l.Role, 18), truncRunes(l.Label, 24), wave, st, l.Steps, qaCell(l, color))
		}
	}

	// gates
	if len(s.Gates) > 0 {
		var g []string
		for _, r := range s.Gates {
			if r.OK {
				g = append(g, paint(color, ansiGreen, r.Name+"✓"))
			} else {
				g = append(g, paint(color, ansiRed, r.Name+"✗"))
			}
		}
		w("")
		w("gates  %s", strings.Join(g, " "))
	}

	// acceptance criteria
	if acs := acStatuses(s); len(acs) > 0 {
		var parts []string
		for _, v := range acs {
			mark := "?"
			switch v.Status {
			case run.StatusVerified:
				mark = paint(color, ansiGreen, "✓")
			case run.StatusFailed:
				mark = paint(color, ansiRed, "✗")
			}
			parts = append(parts, v.Name+mark)
		}
		w("ACs    %s", strings.Join(parts, " "))
	}

	// recent steps
	if n := len(s.Recent); n > 0 {
		w("")
		w("%s", paint(color, ansiDim, fmt.Sprintf("last steps (%d total, %d errors)", s.Steps, s.Errors)))
		from := 0
		if n > watchSteps {
			from = n - watchSteps
		}
		for _, st := range s.Recent[from:] {
			ok := " "
			if !st.OK {
				ok = paint(color, ansiRed, "✗")
			}
			who := st.Role
			if who == "" {
				who = "lead"
			}
			w("%s %-16s %-10s %s", ok, truncRunes(who, 16), truncRunes(st.Tool, 10), truncRunes(st.Target, 60))
		}
	}
	return b.String()
}

// limitsLine summarises per-phase token usage against limits, in phase order.
func limitsLine(s run.State) string {
	if s.Limits.IsZero() {
		return ""
	}
	var parts []string
	for _, p := range run.Phases {
		pl, ok := s.Limits.Phases[p]
		if !ok {
			continue
		}
		switch {
		case pl.MaxTokens > 0:
			parts = append(parts, fmt.Sprintf("%s %s/%s tok", p.Title(), fmtTokens(s.PhaseTokens[p]), fmtTokens(pl.MaxTokens)))
		case pl.MaxAgents > 0:
			parts = append(parts, fmt.Sprintf("%s ≤%d agents", p.Title(), pl.MaxAgents))
		}
	}
	if s.Limits.MaxAgents > 0 {
		parts = append(parts, fmt.Sprintf("≤%d concurrent", s.Limits.MaxAgents))
	}
	return strings.Join(parts, "  ")
}

// phaseStrip renders the seven phases with the current one marked and the
// loop counters (plan rounds, QA / product loop-backs) against their limits.
func phaseStrip(s run.State, color bool) string {
	cur := s.Phase.Index()
	count := map[run.Phase]string{}
	if s.PlanRounds > 0 {
		count[run.PhasePlan] = fmt.Sprintf(" r%d/%d", s.PlanRounds, run.MaxPlanRounds)
	}
	lb := map[run.Phase]int{}
	for _, l := range s.LoopBacks {
		lb[l.Phase]++
	}
	if n := lb[run.PhaseQA]; n > 0 {
		count[run.PhaseQA] = fmt.Sprintf(" ↺%d/%d", n, run.MaxQARounds)
	}
	if n := lb[run.PhaseProduct]; n > 0 {
		count[run.PhaseProduct] = fmt.Sprintf(" ↺%d/%d", n, run.MaxProductRounds)
	}
	parts := make([]string, len(run.Phases))
	for i, p := range run.Phases {
		label := p.Title() + count[p]
		switch {
		case i == cur && !s.Ended:
			parts[i] = paint(color, ansiYellow, "◉ "+label)
		case i < cur || s.Ended && i <= cur:
			parts[i] = paint(color, ansiGreen, "● "+label)
		default:
			parts[i] = paint(color, ansiDim, "○ "+label)
		}
	}
	return strings.Join(parts, "  ")
}

func qaCell(l run.Lane, color bool) string {
	switch l.QA {
	case run.StatusVerified:
		return paint(color, ansiGreen, fmt.Sprintf("✓ verified r%d", max(l.QARounds, 1)))
	case run.StatusFailed:
		return paint(color, ansiRed, fmt.Sprintf("✗ failed r%d", max(l.QARounds, 1)))
	}
	return "-"
}

// acStatuses returns the latest verdict per acceptance criteria (product
// phase verdicts and AC-named ones), in first-seen order.
func acStatuses(s run.State) []run.VerdictInfo {
	idx := map[string]int{}
	var out []run.VerdictInfo
	for _, v := range s.Verdicts {
		if v.Phase != run.PhaseProduct && !strings.HasPrefix(v.Name, "AC") {
			continue
		}
		if i, ok := idx[v.Name]; ok {
			out[i] = v
			continue
		}
		idx[v.Name] = len(out)
		out = append(out, v)
	}
	return out
}
