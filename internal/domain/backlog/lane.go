package backlog

import (
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

// LaneRun is what a lane left in its worktree: its newest loops run.
type LaneRun struct {
	Dir    string      // the run directory ("" = no run started)
	Events []run.Event // its events.jsonl
	PRBody string      // its pr.md ("" = none)
}

// Outcome reads how the lane ended (BACKLOG §3 step 4). agentErr is the
// agent process's failure ("" when it exited cleanly).
func (l LaneRun) Outcome(agentErr string) Outcome {
	o := Outcome{RunDir: l.Dir}
	if l.Dir == "" || len(l.Events) == 0 {
		o.Reason = join("no loops run started", agentErr)
		return o
	}
	st := run.Fold(l.Events)
	switch {
	case !st.Ended:
		o.Reason = join("the agent stopped before ending its run (phase "+string(st.Phase)+")", agentErr)
	case st.Outcome == "done" && st.PR != "":
		o.PR = st.PR
	case st.Outcome == "done":
		o.Reason = "the run ended done without opening a PR"
	default:
		o.Reason = "the run ended " + st.Outcome
	}
	if o.PR == "" {
		if d := lastDecision(st); d != "" {
			o.Reason += "; last decision: " + d
		}
	}
	return o
}

func lastDecision(st run.State) string {
	if len(st.Decisions) == 0 {
		return ""
	}
	d := st.Decisions[len(st.Decisions)-1]
	return d.Topic + ": " + d.Note
}

func join(a, b string) string {
	if b == "" {
		return a
	}
	return a + " (" + b + ")"
}

// LaneFinding is one finding a lane recorded (BACKLOG §5).
type LaneFinding struct {
	Title string
	Ref   string // a file in the run directory with details ("" = none)
}

// Findings returns the lane's findings: every note event named "finding",
// then each Open follow-ups item of pr.md, without repeats.
func (l LaneRun) Findings() []LaneFinding {
	var out []LaneFinding
	seen := map[string]bool{}
	add := func(title, ref string) {
		key := FindingKey(title)
		if key == "" || seen[key] {
			return
		}
		seen[key] = true
		out = append(out, LaneFinding{Title: strings.TrimSpace(title), Ref: ref})
	}
	for _, e := range l.Events {
		if e.Kind == run.KindNote && e.Name == "finding" {
			add(e.Note, e.Ref)
		}
	}
	for _, f := range FollowUps(l.PRBody) {
		add(f, "")
	}
	return out
}

// FollowUps returns the bullet items under a "## Open follow-ups" heading,
// skipping "none". Continuation lines are dropped: the title is the item's
// first line.
func FollowUps(md string) []string {
	var out []string
	in := false
	for line := range strings.SplitSeq(md, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "#") {
			in = strings.EqualFold(strings.TrimSpace(strings.TrimLeft(t, "#")), "Open follow-ups")
			continue
		}
		if !in || line != strings.TrimLeft(line, " \t") {
			continue
		}
		item, ok := strings.CutPrefix(t, "- ")
		if !ok {
			item, ok = strings.CutPrefix(t, "* ")
		}
		item = strings.TrimSpace(item)
		if !ok || item == "" || strings.EqualFold(strings.Trim(item, "\"'`."), "none") {
			continue
		}
		out = append(out, item)
	}
	return out
}
