package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

// The lead's commands (WORKFLOW §2.7): which phases stop for the user, the
// decisions the lead takes in their place, and the brief that re-anchors the
// lead after a compaction, a resumed session or a phase change.

// leadCmd holds the lead's `loops run` subcommands.
type leadCmd struct{ a *App }

// runAsk implements `loops run ask [<list>…]`: it records the run's
// checkpoints and prints them. The list given wins; without one, the
// config's "ask" (and the older "approval": "always") applies.
func (l leadCmd) ask(ctx context.Context, args []string) int {
	fs := l.a.newFlagSet("run ask")
	pos, err := parseFlagsAnywhere(fs, args)
	if err != nil {
		return 2
	}
	list := l.a.Checkpoints
	if len(pos) > 0 {
		list = strings.Join(pos, ",")
	}
	ps, err := run.ParseAsk(list)
	if err != nil {
		return l.a.usageErr("run ask: " + err.Error())
	}
	if err := l.a.Tracker.Record(ctx, run.Event{Kind: run.KindAsk, Note: run.FormatAsk(ps)}); err != nil {
		return l.a.fail("run ask", err)
	}
	fmt.Fprintln(l.a.Out, "checkpoints: "+run.FormatAsk(ps))
	return 0
}

// runCheckpoint implements `loops run checkpoint <phase>`: it prints "ask"
// when the lead stops for the user at that phase, else "auto".
func (l leadCmd) checkpoint(ctx context.Context, args []string) int {
	fs := l.a.newFlagSet("run checkpoint")
	pos, err := parseFlagsAnywhere(fs, args)
	if err != nil {
		return 2
	}
	if len(pos) != 1 {
		return l.a.usageErr("usage: loops run checkpoint <phase|approve|pr>")
	}
	ps, err := run.ParseAsk(pos[0])
	if err != nil || len(ps) != 1 {
		return l.a.usageErr(fmt.Sprintf("run checkpoint: %q is not one phase (want one of %v, approve or pr)", pos[0], run.Phases))
	}
	st, err := l.a.Tracker.State(ctx, "")
	if err != nil {
		return l.a.fail("run checkpoint", err)
	}
	if run.Asks(st, ps[0]) {
		fmt.Fprintln(l.a.Out, "ask")
	} else {
		fmt.Fprintln(l.a.Out, "auto")
	}
	return 0
}

// runDecide implements `loops run decide --topic T --note "…"`: a decision
// the run turned on, by the lead (default) or the user.
func (l leadCmd) decide(ctx context.Context, args []string) int {
	fs := l.a.newFlagSet("run decide")
	topic := fs.String("topic", "", "what was decided, e.g. plan-limit, W1-domain:C2, scope (required)")
	note := fs.String("note", "", "the decision and its reason (required)")
	by := fs.String("by", run.ByLead, "lead|user")
	ref := fs.String("ref", "", "file the decision rests on")
	phase := fs.String("phase", "", "defaults to the current phase")
	pos, err := parseFlagsAnywhere(fs, args)
	if err != nil {
		return 2
	}
	if len(pos) > 0 || strings.TrimSpace(*topic) == "" || strings.TrimSpace(*note) == "" {
		return l.a.usageErr("usage: loops run decide --topic T --note text [--by lead|user] [--ref f] [--phase P]")
	}
	if *by != run.ByLead && *by != run.ByUser {
		return l.a.usageErr(fmt.Sprintf("run decide: --by must be lead or user, got %q", *by))
	}
	p := run.Phase(*phase)
	if p != "" && p.Index() < 0 {
		return l.a.usageErr(fmt.Sprintf("run decide: unknown phase %q", *phase))
	}
	e := run.Event{Kind: run.KindDecision, From: *by, Name: *topic, Note: *note, Ref: *ref, Phase: p}
	if err := l.a.Tracker.Record(ctx, e); err != nil {
		return l.a.fail("run decide", err)
	}
	return 0
}

// briefJSON is `loops run brief --json`.
type briefJSON struct {
	Run       string             `json:"run"`
	Dir       string             `json:"dir"`
	Charter   string             `json:"charter"`
	Phase     run.Phase          `json:"phase"`
	Ask       []run.Phase        `json:"ask"`
	Open      []run.VerdictInfo  `json:"open"`
	Decisions []run.DecisionInfo `json:"decisions"`
	Next      string             `json:"next"`
}

// runBrief implements `loops run brief [--json]`.
func (l leadCmd) brief(ctx context.Context, args []string) int {
	fs := l.a.newFlagSet("run brief")
	asJSON := fs.Bool("json", false, "machine-readable")
	pos, err := parseFlagsAnywhere(fs, args)
	if err != nil {
		return 2
	}
	if len(pos) > 0 {
		return l.a.usageErr("usage: loops run brief [--json]")
	}
	st, err := l.a.Tracker.State(ctx, "")
	if err != nil {
		return l.a.fail("run brief", err)
	}
	dir, err := l.a.Tracker.Dir(ctx, "")
	if err != nil {
		return l.a.fail("run brief", err)
	}
	if !*asJSON {
		fmt.Fprint(l.a.Out, run.Brief(st, dir))
		return 0
	}
	b, _ := json.MarshalIndent(briefJSON{
		Run: st.ID, Dir: dir, Charter: dir + "/charter.md", Phase: st.Phase, Ask: st.Ask,
		Open: run.OpenFailures(st), Decisions: st.Decisions, Next: run.Next(st),
	}, "", "  ")
	fmt.Fprintln(l.a.Out, string(b))
	return 0
}
