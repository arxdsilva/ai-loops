package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/arxdsilva/ai-loops/internal/domain/backlog"
	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

// quietAfter is how long a live lane's run may go without an update before
// the board marks it quiet (a long test run, or a stuck agent).
const quietAfter = 10 * time.Minute

// backlogWatch implements `loops backlog watch [--interval d] [--once]`: the
// backlog's live board in the terminal, what `loops watch` is for one run
// (BACKLOG §8). It reads the state and the lanes' worktrees and writes
// nothing, so it runs beside the supervisor and the lanes.
func (a *App) backlogWatch(ctx context.Context, svc BacklogBoard, args []string) int {
	fs := a.newFlagSet("backlog watch")
	interval := fs.Duration("interval", 2*time.Second, "")
	once := fs.Bool("once", false, "")
	if pos, err := parseFlagsAnywhere(fs, args); err != nil || len(pos) > 0 || *interval <= 0 {
		return a.usageErr("usage: loops backlog watch [--interval 2s] [--once]")
	}
	tty := isTTY(a.Out)
	frame := func() (string, error) {
		b, err := svc.Board(ctx)
		if err != nil {
			return "", err
		}
		return renderBoard(b, time.Now(), tty), nil
	}
	if *once {
		f, err := frame()
		if err != nil {
			return a.fail("backlog watch", err)
		}
		fmt.Fprint(a.Out, f)
		return 0
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()
	tick := time.NewTicker(*interval)
	defer tick.Stop()
	for {
		f, err := frame()
		if err != nil {
			return a.fail("backlog watch", err)
		}
		if tty {
			fmt.Fprint(a.Out, ansiClear)
		}
		fmt.Fprint(a.Out, f)
		select {
		case <-ctx.Done():
			return 0
		case <-tick.C:
		}
	}
}

// renderBoard draws one frame of the board at now.
func renderBoard(b backlog.Board, now time.Time, color bool) string {
	var sb strings.Builder
	w := func(format string, v ...any) { fmt.Fprintf(&sb, format, v...); sb.WriteByte('\n') }
	set := b.Settings

	head := paint(color, ansiBold, "backlog "+set.Repo)
	if set.Base != "" {
		head += " · base " + set.Base
	}
	if set.Human != "" {
		head += " · human @" + set.Human
	}
	synced := "never synced"
	if !b.Synced.IsZero() {
		synced = "synced " + ago(now.Sub(b.Synced)) + " ago"
	}
	w("%s  %s", head, paint(color, ansiDim, synced))
	w("%s", countsLine(b, color))
	caps := fmt.Sprintf("lanes %d/%d × %d agents · lane PRs %d", b.Counts[backlog.Running], set.Lanes, set.MaxAgents, b.PRs)
	if c := set.PRCap(); c > 0 {
		caps += fmt.Sprintf(" (≤%d open)", c)
	}
	caps += fmt.Sprintf(" · findings %d filed", b.Findings)
	if b.Pending > 0 {
		caps += fmt.Sprintf(", %d pending", b.Pending)
	}
	w("%s", paint(color, ansiDim, caps))

	w("")
	if len(b.Running) == 0 {
		w("%s", paint(color, ansiDim, "no lanes running"))
	}
	for _, l := range b.Running {
		boardLane(w, l, set, now, color)
	}

	if len(b.Next) > 0 {
		w("")
		w("%s", paint(color, ansiDim, fmt.Sprintf("next (%d queued)", b.Counts[backlog.Queued])))
		for _, it := range b.Next {
			w("  #%-5d P%-2s %s", it.Number, prio(it.Priority), truncRunes(it.Title, 70))
		}
	}
	if len(b.Recent) > 0 {
		w("")
		w("%s", paint(color, ansiDim, "recent"))
		for _, it := range b.Recent {
			w("  %s #%-5d %s", outcomeMark(it.Status, color), it.Number, recentText(it))
		}
	}
	return sb.String()
}

func countsLine(b backlog.Board, color bool) string {
	var parts []string
	for _, st := range []backlog.Status{backlog.Queued, backlog.Running, backlog.Done, backlog.Failed, backlog.Human, backlog.Skipped} {
		s := fmt.Sprintf("%s %d", st, b.Counts[st])
		switch {
		case b.Counts[st] == 0:
			s = paint(color, ansiDim, s)
		case st == backlog.Running:
			s = paint(color, ansiYellow, s)
		case st == backlog.Done:
			s = paint(color, ansiGreen, s)
		case st == backlog.Failed || st == backlog.Human:
			s = paint(color, ansiRed, s)
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, " · ")
}

// boardLane writes a running lane: its issue, then its run's phase strip
// and counters, then its newest step (or why it has none).
func boardLane(w func(string, ...any), l backlog.LaneView, set backlog.Settings, now time.Time, color bool) {
	it := l.Item
	took := ""
	if !it.Started.IsZero() {
		took = " · " + ago(now.Sub(it.Started))
	}
	w("%s #%d %s  %s", paint(color, ansiYellow, "▶"), it.Number, truncRunes(it.Title, 60),
		paint(color, ansiDim, fmt.Sprintf("attempt %d/%d%s", it.Attempts, set.Attempts, took)))
	if !l.Alive {
		w("    %s", paint(color, ansiRed, "lane process gone: the next loops backlog sync records it"))
	}
	p := l.Run
	if p == nil {
		w("    %s", paint(color, ansiDim, "no live loops run in the worktree (starting, or wrapping up)"))
		return
	}
	line := miniStrip(p, color) + "  " + string(p.Phase)
	line += fmt.Sprintf(" · agents %d/%d · steps %d", p.Busy, p.Agents, p.Steps)
	if p.Errors > 0 {
		line += paint(color, ansiRed, "!"+strconv.Itoa(p.Errors))
	}
	if p.Gates > 0 {
		g := fmt.Sprintf("gates %d/%d", p.GatesOK, p.Gates)
		if p.GatesOK == p.Gates {
			g = paint(color, ansiGreen, g)
		} else {
			g = paint(color, ansiRed, g)
		}
		line += " · " + g
	}
	if p.ACs > 0 {
		line += fmt.Sprintf(" · ACs %d/%d", p.ACsOK, p.ACs)
	}
	if p.Rounds > 0 {
		line += paint(color, ansiYellow, fmt.Sprintf(" · ↺%d", p.Rounds))
	}
	w("    %s", line)
	switch {
	case p.Ended:
		w("    %s", paint(color, ansiDim, strings.TrimSpace("run ended "+p.Outcome)+prSuffix(p.PR)+": the lane is wrapping up"))
	case !p.Updated.IsZero() && l.Alive && now.Sub(p.Updated) > quietAfter:
		w("    %s", paint(color, ansiYellow, "quiet for "+ago(now.Sub(p.Updated))+": last "+truncRunes(p.Last, 60)))
	case p.Last != "":
		w("    %s", paint(color, ansiDim, "last: "+truncRunes(p.Last, 70)+prSuffix(p.PR)))
	}
}

// miniStrip is the seven phases as dots: done, current, ahead.
func miniStrip(p *backlog.Progress, color bool) string {
	cur := p.Phase.Index()
	var sb strings.Builder
	for i := range run.Phases {
		switch {
		case i < cur || p.Ended && i == cur:
			sb.WriteString(paint(color, ansiGreen, "●"))
		case i == cur:
			sb.WriteString(paint(color, ansiYellow, "◉"))
		default:
			sb.WriteString(paint(color, ansiDim, "○"))
		}
	}
	return sb.String()
}

func outcomeMark(st backlog.Status, color bool) string {
	switch st {
	case backlog.Done:
		return paint(color, ansiGreen, "✓")
	case backlog.Failed:
		return paint(color, ansiRed, "✗")
	case backlog.Human:
		return paint(color, ansiRed, "⚑")
	}
	return paint(color, ansiDim, "–")
}

func recentText(it backlog.Item) string {
	s := truncRunes(it.Title, 50)
	switch {
	case it.Status == backlog.Done && it.PR != "":
		return s + "  " + it.PR
	case it.Note != "":
		return s + "  (" + string(it.Status) + ": " + truncRunes(it.Note, 60) + ")"
	}
	return s + "  (" + string(it.Status) + ")"
}

func prSuffix(pr string) string {
	if pr == "" {
		return ""
	}
	return " · PR " + pr
}

// ago is a short duration: 45s, 12m, 3h05m.
func ago(d time.Duration) string {
	switch {
	case d < 0:
		return "0s"
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
}
