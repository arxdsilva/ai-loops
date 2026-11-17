package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

// The skipper (WORKFLOW §2.8): `loops run triage` records which steps the
// run skips, from the signals the lead reads in the request; `loops run step`
// answers run or skip for one step. The gate is never a step.

// triageFlags are the parsed `loops run triage` flags.
type triageFlags struct {
	kind, touches, keep, skip, reason string
	files, langs                      int
	unclear, full, recheck            bool
}

// triage implements `loops run triage`.
func (l leadCmd) triage(ctx context.Context, args []string) int {
	fs := l.a.newFlagSet("run triage")
	var f triageFlags
	fs.StringVar(&f.kind, "kind", "", fmt.Sprintf("what the request asks for: %v (required unless --full or --recheck)", run.TaskKinds))
	fs.IntVar(&f.files, "files", 0, "files the change touches: an estimate at intake, the real count at --recheck")
	fs.IntVar(&f.langs, "langs", 0, "language packs the change touches (default 1)")
	fs.StringVar(&f.touches, "touches", "", fmt.Sprintf("risk areas, comma-separated: %v or none", run.Touches))
	fs.BoolVar(&f.unclear, "unclear", false, "the ticket has open points: keep clarify")
	fs.BoolVar(&f.full, "full", false, "skip nothing (also config \"triage\": \"off\")")
	fs.BoolVar(&f.recheck, "recheck", false, "after implement: re-triage with the real diff; only adds steps back")
	fs.StringVar(&f.keep, "keep", "", "steps to run although the tier skips them")
	fs.StringVar(&f.skip, "skip", "", "steps to skip although the tier runs them")
	fs.StringVar(&f.reason, "reason", "", "why, required with --keep or --skip (recorded as a decision)")
	pos, err := parseFlagsAnywhere(fs, args)
	if err != nil {
		return 2
	}
	if len(pos) > 0 {
		return l.a.usageErr("usage: loops run triage --kind K --files N [--langs N] [--touches a,b] [--unclear] [--keep s,…] [--skip s,…] [--reason r] | --recheck --files N | --full")
	}
	st, err := l.a.Tracker.State(ctx, "")
	if err != nil {
		return l.a.fail("run triage", err)
	}
	t, msg := l.decideTriage(st, f)
	if msg != "" {
		return l.a.usageErr("run triage: " + msg)
	}
	keep, err := run.ParseSteps(f.keep)
	if err != nil {
		return l.a.usageErr("run triage: --keep: " + err.Error())
	}
	skip, err := run.ParseSteps(f.skip)
	if err != nil {
		return l.a.usageErr("run triage: --skip: " + err.Error())
	}
	overrides := len(keep) > 0 || len(skip) > 0
	if overrides && strings.TrimSpace(f.reason) == "" {
		return l.a.usageErr("run triage: --keep and --skip need --reason")
	}
	t = run.Adjust(t, keep, skip)
	note, _ := json.Marshal(t)
	if err := l.a.Tracker.Record(ctx, run.Event{Kind: run.KindTriage, Name: string(t.Tier), Note: string(note)}); err != nil {
		return l.a.fail("run triage", err)
	}
	if overrides {
		d := fmt.Sprintf("keep %s; skip %s; %s", run.JoinSteps(keep), run.JoinSteps(skip), f.reason)
		if err := l.a.Tracker.Record(ctx, run.Event{Kind: run.KindDecision, From: run.ByLead, Name: "triage", Note: d}); err != nil {
			return l.a.fail("run triage", err)
		}
	}
	printTriage(l.a, t)
	return 0
}

// decideTriage turns the flags into a triage, or returns a usage message.
func (l leadCmd) decideTriage(st run.State, f triageFlags) (run.Triage, string) {
	touches, err := run.ParseTouches(f.touches)
	if err != nil {
		return run.Triage{}, err.Error()
	}
	sig := run.Signals{Files: f.files, Langs: f.langs, Touches: touches, Unclear: f.unclear}
	if f.kind != "" {
		if sig.Kind, err = run.ParseTaskKind(f.kind); err != nil {
			return run.Triage{}, err.Error()
		}
	}
	switch {
	case f.full || l.a.TriageOff:
		return run.Full(sig), ""
	case f.recheck:
		prev := st.Triage
		if prev.Tier == "" {
			return run.Triage{}, "--recheck needs an earlier triage"
		}
		if f.files < 1 {
			return run.Triage{}, "--recheck needs --files N (the real count)"
		}
		if sig.Kind == "" {
			sig.Kind = prev.Signals.Kind
		}
		if sig.Langs == 0 {
			sig.Langs = prev.Signals.Langs
		}
		sig.Touches = mergeTouches(prev.Signals.Touches, sig.Touches)
		sig.Unclear = sig.Unclear || prev.Signals.Unclear
		return run.Recheck(prev, sig, st.Phase), ""
	case sig.Kind == "":
		return run.Triage{}, fmt.Sprintf("--kind is required (%v), or pass --full", run.TaskKinds)
	case f.files < 1:
		return run.Triage{}, "--files N is required (an estimate, at least 1)"
	}
	return run.Decide(sig), ""
}

// mergeTouches is a ∪ b, a's order first: a recheck never forgets a risk.
func mergeTouches(a, b []run.Touch) []run.Touch {
	out := append([]run.Touch(nil), a...)
	for _, t := range b {
		if !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	return out
}

func printTriage(a *App, t run.Triage) {
	label := string(t.Tier)
	if t.Full {
		label += ", full"
	}
	if t.Recheck {
		label += ", rechecked"
	}
	fmt.Fprintf(a.Out, "triage: %s (%s)\n", label, t.Signals)
	fmt.Fprintf(a.Out, "skip: %s\n", run.JoinSteps(t.Skip))
	var runs []run.Step
	for _, st := range run.Steps {
		if !slices.Contains(t.Skip, st) {
			runs = append(runs, st)
		}
	}
	fmt.Fprintf(a.Out, "run: %s; loops gate always\n", run.JoinSteps(runs))
}

// step implements `loops run step <id>`: "skip" when the run skips it, else
// "run". An untriaged run runs every step.
func (l leadCmd) step(ctx context.Context, args []string) int {
	fs := l.a.newFlagSet("run step")
	pos, err := parseFlagsAnywhere(fs, args)
	if err != nil {
		return 2
	}
	if len(pos) != 1 {
		return l.a.usageErr("usage: loops run step <step>")
	}
	ss, err := run.ParseSteps(pos[0])
	if err != nil || len(ss) != 1 {
		return l.a.usageErr(fmt.Sprintf("run step: %q is not one step (want one of %s)", pos[0], run.JoinSteps(run.Steps[:])))
	}
	st, err := l.a.Tracker.State(ctx, "")
	if err != nil {
		return l.a.fail("run step", err)
	}
	if run.Skips(st, ss[0]) {
		fmt.Fprintln(l.a.Out, "skip")
	} else {
		fmt.Fprintln(l.a.Out, "run")
	}
	return 0
}
