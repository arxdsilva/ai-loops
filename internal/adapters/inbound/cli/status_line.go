package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

const (
	ansiReset  = "\x1b[0m"
	ansiDim    = "\x1b[2m"
	ansiGreen  = "\x1b[32m"
	ansiRed    = "\x1b[31m"
	ansiYellow = "\x1b[33m"
)

// statusInput is the subset of Claude Code's statusLine JSON we tolerate.
type statusInput struct {
	SessionID string `json:"session_id"`
	Cwd       string `json:"cwd"`
	Workspace struct {
		ProjectDir string `json:"project_dir"`
	} `json:"workspace"`
	Model struct {
		DisplayName string `json:"display_name"`
	} `json:"model"`
}

func (a *App) readStatusInput() statusInput {
	var in statusInput
	if a.In == nil {
		return in
	}
	_ = json.Unmarshal(readBounded(a.In), &in) // empty or malformed stdin is fine
	return in
}

const (
	stdinWait = 300 * time.Millisecond
	stdinCap  = 64 << 10
)

// readBounded reads r until EOF, a complete JSON document, the size cap or
// stdinWait, whichever comes first. A blocked reader is abandoned.
func readBounded(r io.Reader) []byte {
	var (
		mu   sync.Mutex
		buf  []byte
		done = make(chan struct{})
	)
	go func() {
		defer close(done)
		chunk := make([]byte, 4096)
		for {
			n, err := r.Read(chunk)
			mu.Lock()
			buf = append(buf, chunk[:n]...)
			full := len(buf) >= stdinCap
			complete := json.Valid(buf)
			mu.Unlock()
			if err != nil || full || complete {
				return
			}
		}
	}()
	select {
	case <-done:
	case <-time.After(stdinWait):
	}
	mu.Lock()
	defer mu.Unlock()
	if len(buf) > stdinCap {
		return append([]byte(nil), buf[:stdinCap]...)
	}
	return append([]byte(nil), buf...)
}

func truncRunes(s string, n int) string {
	r := []rune(s)
	if n <= 0 || len(r) <= n {
		return s
	}
	if n == 1 {
		return string(r[:1])
	}
	return string(r[:n-1]) + "…"
}

func runTitle(s run.State) string {
	if s.Ticket != "" {
		return s.Ticket
	}
	if s.Title != "" {
		return s.Title
	}
	return s.ID
}

func elapsed(s run.State) string {
	if s.Started.IsZero() || s.Updated.Before(s.Started) {
		return ""
	}
	d := s.Updated.Sub(s.Started)
	if d >= time.Hour {
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}

func paint(on bool, color, s string) string {
	if !on {
		return s
	}
	return color + s + ansiReset
}

type laneCounts struct{ running, total, verified, failed, maxWave int }

func countLanes(s run.State) laneCounts {
	var c laneCounts
	for _, l := range s.Lanes {
		c.total++
		if l.Running {
			c.running++
			if l.Wave > c.maxWave {
				c.maxWave = l.Wave
			}
		}
		switch l.QA {
		case run.StatusVerified:
			c.verified++
		case run.StatusFailed:
			c.failed++
		}
	}
	return c
}

func qaLoopBacks(s run.State) int {
	n := 0
	for _, lb := range s.LoopBacks {
		if lb.Phase == run.PhaseQA {
			n++
		}
	}
	return n
}

// fmtTokens renders a token count in k/M units ("12k", "1.2M").
func fmtTokens(n int64) string {
	switch {
	case n >= 1_000_000:
		t := n / 100_000 // truncated tenths of a million
		if t%10 == 0 {
			return fmt.Sprintf("%dM", t/10)
		}
		return fmt.Sprintf("%d.%dM", t/10, t%10)
	case n >= 1000:
		return fmt.Sprintf("%dk", n/1000)
	}
	return fmt.Sprintf("%d", n)
}

// phaseTokSeg is "tok <used>/<limit>" for the current phase when it has a
// token limit, else "".
func phaseTokSeg(s run.State) string {
	if s.Limits.IsZero() {
		return ""
	}
	pl, ok := s.Limits.Phases[s.Phase]
	if !ok || pl.MaxTokens <= 0 {
		return ""
	}
	return "tok " + fmtTokens(s.PhaseTokens[s.Phase]) + "/" + fmtTokens(pl.MaxTokens)
}

// budgetPaused reports whether the latest budget block is in the current
// phase and newer than the last phase change.
func budgetPaused(s run.State) bool {
	if len(s.Budgets) == 0 {
		return false
	}
	b := s.Budgets[len(s.Budgets)-1]
	if b.Phase != s.Phase {
		return false
	}
	if n := len(s.PhaseLog); n > 0 && !b.At.After(s.PhaseLog[n-1].At) {
		return false
	}
	if !s.LimitsAt.IsZero() && !b.At.After(s.LimitsAt) {
		return false
	}
	// a successful spawn in this phase after the block ends the pause
	for _, l := range s.Lanes {
		if l.Phase == s.Phase && l.Started.After(b.At) {
			return false
		}
	}
	return true
}

// statusSegments builds the segments shared by --statusline (coloured) and the
// plain human line (uncoloured).
func statusSegments(s run.State, color bool) []string {
	segs := []string{paint(color, ansiYellow, "▶") + " " + truncRunes(runTitle(s), 24)}
	if s.Ended {
		segs[0] = "■ " + truncRunes(runTitle(s), 24)
	}
	if i := s.Phase.Index(); i >= 0 {
		p := fmt.Sprintf("%d/%d %s", i+1, len(run.Phases), s.Phase.Title())
		c := countLanes(s)
		if c.maxWave > 0 {
			p += fmt.Sprintf(" W%d", c.maxWave)
		}
		segs = append(segs, p)
	}
	c := countLanes(s)
	if c.total > 0 {
		segs = append(segs, paint(color && c.running > 0, ansiYellow, fmt.Sprintf("%d/%d lanes", c.running, c.total)))
	}
	lb := qaLoopBacks(s)
	if c.verified+c.failed+lb > 0 {
		segs = append(segs, "QA "+paint(color && c.verified > 0, ansiGreen, fmt.Sprintf("%d✓", c.verified))+" "+
			paint(color && c.failed > 0, ansiRed, fmt.Sprintf("%d✗", c.failed))+fmt.Sprintf(" %d↺", lb))
	}
	if n := len(s.Overrides); n > 0 {
		segs = append(segs, fmt.Sprintf("overrides %d", n))
	}
	if t := phaseTokSeg(s); t != "" {
		segs = append(segs, t)
	}
	if budgetPaused(s) {
		segs = append(segs, paint(color, ansiYellow, "⏸ budget"))
	}
	if s.PlanRounds > 0 {
		segs = append(segs, fmt.Sprintf("plan r%d", s.PlanRounds))
	}
	if len(s.Gates) > 0 {
		var g []string
		for _, r := range s.Gates {
			if r.OK {
				g = append(g, paint(color, ansiGreen, r.Name+"✓"))
			} else {
				g = append(g, paint(color, ansiRed, r.Name+"✗"))
			}
		}
		segs = append(segs, "gates "+strings.Join(g, " "))
	}
	if s.Steps > 0 {
		segs = append(segs, fmt.Sprintf("%d steps", s.Steps))
	}
	if e := elapsed(s); e != "" {
		segs = append(segs, e)
	}
	return segs
}

func statusLine(_ statusInput, s run.State, active bool) string {
	if !active {
		return ansiDim + "loops · idle" + ansiReset
	}
	return strings.Join(statusSegments(s, true), " · ")
}

func plainLine(s run.State) string {
	line := strings.Join(statusSegments(s, false), " · ")
	if s.Outcome != "" {
		line += " · " + s.Outcome
	}
	return line
}

// subagentInput is the stdin of Claude Code's subagentStatusLine.
type subagentInput struct {
	Columns int `json:"columns"`
	Tasks   []struct {
		ID          string `json:"id"`
		AgentType   string `json:"agentType"`
		Description string `json:"description"`
	} `json:"tasks"`
}

func (a *App) writeSubagentLines(s run.State, active bool) {
	if !active || a.In == nil {
		return
	}
	b := readBounded(a.In)
	var in subagentInput
	if json.Unmarshal(b, &in) != nil {
		return
	}
	for _, t := range in.Tasks {
		role, ok := strings.CutPrefix(t.AgentType, "loops:")
		if !ok {
			continue
		}
		label, steps, qa := "", 0, "pending"
		l := laneForTask(s, t.ID, role)
		if l != nil {
			label, steps = l.Label, l.Steps
			if l.QA != "" {
				qa = strings.ToLower(l.QA)
			}
		}
		if label == "" {
			label = truncRunes(t.Description, 40)
		}
		content := fmt.Sprintf("%s · %s · %d steps · QA %s", role, label, steps, qa)
		if l != nil && l.Tokens > 0 {
			content += " · " + fmtTokens(l.Tokens) + " tok"
		}
		out, err := json.Marshal(struct {
			ID      string `json:"id"`
			Content string `json:"content"`
		}{t.ID, truncRunes(content, in.Columns)})
		if err == nil {
			fmt.Fprintln(a.Out, string(out))
		}
	}
}

// laneForTask matches a task to its lane by agent id; it falls back to the
// role only when exactly one running lane has that role (else nil).
func laneForTask(s run.State, id, role string) *run.Lane {
	var byRole *run.Lane
	n := 0
	for i := range s.Lanes {
		l := &s.Lanes[i]
		if id != "" && l.Agent == id {
			return l
		}
		if l.Role == role && l.Running {
			byRole = l
			n++
		}
	}
	if n == 1 {
		return byRole
	}
	return nil
}
