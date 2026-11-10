package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/application/tracker"
	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

// newFlagSet returns a FlagSet that reports errors to a.Err and never exits.
func (a *App) newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(a.Err)
	return fs
}

// parseFlagsAnywhere parses fs over args, allowing flags before, between and
// after positional arguments (the stdlib stops at the first positional). It
// returns the positionals in order.
func parseFlagsAnywhere(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		rest := fs.Args()
		if len(rest) == 0 {
			return pos, nil
		}
		pos = append(pos, rest[0])
		args = rest[1:]
	}
}

// cmdRun implements `loops run <sub>`: start, phase, event, label, loopback,
// verdict, override, end, dir, env, clean. Every sub-command except start
// and env needs an active run and otherwise prints a message on stderr and
// exits 1 (env prints nothing and exits 0).
func (a *App) cmdRun(ctx context.Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(a.Err, "usage: loops run start|phase|event|label|loopback|verdict|override|end|dir|env|clean …")
		return 2
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "start":
		return a.runStart(ctx, rest)
	case "phase":
		return a.runPhase(ctx, rest)
	case "event":
		return a.runEvent(ctx, rest)
	case "label":
		return a.runLabel(ctx, rest)
	case "loopback":
		return a.runLoopback(ctx, rest)
	case "verdict":
		return a.runVerdict(ctx, rest)
	case "override":
		return a.runOverride(ctx, rest)
	case "end":
		return a.runEnd(ctx, rest)
	case "dir":
		return a.runDir(ctx, rest)
	case "env":
		return wsCmd{a}.env(ctx, rest)
	case "clean":
		return wsCmd{a}.clean(ctx, rest)
	}
	fmt.Fprintf(a.Err, "loops run: unknown subcommand %q\n", sub)
	return 2
}

// fail prints err on stderr (a friendlier line for "no active run") and returns 1.
func (a *App) fail(cmd string, err error) int {
	if errors.Is(err, tracker.ErrNoActiveRun) {
		fmt.Fprintf(a.Err, "loops %s: no active run (start one with `loops run start --title …`)\n", cmd)
	} else {
		fmt.Fprintf(a.Err, "loops %s: %v\n", cmd, err)
	}
	return 1
}

func (a *App) usageErr(msg string) int {
	fmt.Fprintln(a.Err, "loops: "+msg)
	return 2
}

func (a *App) runStart(ctx context.Context, args []string) int {
	fs := a.newFlagSet("run start")
	title := fs.String("title", "", "short run title (required)")
	ticket := fs.String("ticket", "", "ticket ref")
	source := fs.String("source", "", "github|gitlab|jira|linear|text")
	pos, err := parseFlagsAnywhere(fs, args)
	if err != nil {
		return 2
	}
	if len(pos) > 0 {
		return a.usageErr("run start: unexpected argument " + pos[0])
	}
	if strings.TrimSpace(*title) == "" {
		return a.usageErr("run start: --title is required")
	}
	id, err := a.Tracker.Start(ctx, tracker.StartInput{Title: *title, Ticket: *ticket, Source: *source})
	if err != nil {
		return a.fail("run start", err)
	}
	dir, err := a.Tracker.Dir(ctx, id)
	if err != nil {
		return a.fail("run start", err)
	}
	wsCmd{a}.prepare(ctx, dir)
	fmt.Fprintf(a.Out, "run %s\ndir %s\n", id, dir)
	return 0
}

func (a *App) runPhase(ctx context.Context, args []string) int {
	fs := a.newFlagSet("run phase")
	note := fs.String("note", "", "note")
	pos, err := parseFlagsAnywhere(fs, args)
	if err != nil {
		return 2
	}
	if len(pos) != 1 {
		return a.usageErr("usage: loops run phase <phase> [--note text]")
	}
	p := run.Phase(pos[0])
	if p.Index() < 0 {
		return a.usageErr(fmt.Sprintf("run phase: unknown phase %q (want one of %v)", pos[0], run.Phases))
	}
	if err := a.Tracker.Record(ctx, run.Event{Kind: run.KindPhase, Phase: p, Note: *note}); err != nil {
		return a.fail("run phase", err)
	}
	return 0
}

func (a *App) runEvent(ctx context.Context, args []string) int {
	fs := a.newFlagSet("run event")
	kind := fs.String("kind", "", "event kind (required)")
	name := fs.String("name", "", "")
	target := fs.String("target", "", "")
	from := fs.String("from", "", "")
	to := fs.String("to", "", "")
	status := fs.String("status", "", "")
	ok := fs.String("ok", "", "true|false")
	round := fs.Int("round", 0, "")
	wave := fs.Int("wave", 0, "")
	role := fs.String("role", "", "")
	agent := fs.String("agent", "", "")
	model := fs.String("model", "", "")
	ref := fs.String("ref", "", "")
	note := fs.String("note", "", "")
	phase := fs.String("phase", "", "")
	ms := fs.Int64("ms", 0, "duration in milliseconds")
	pos, err := parseFlagsAnywhere(fs, args)
	if err != nil {
		return 2
	}
	if len(pos) > 0 || *kind == "" {
		return a.usageErr("usage: loops run event --kind K [--name --target --from --to --status --ok --round --wave --role --agent --model --ref --note --phase --ms]")
	}
	e := run.Event{
		Kind: run.Kind(*kind), Name: *name, Target: *target, From: *from, To: *to, Status: *status,
		Round: *round, Wave: *wave, Role: *role, Agent: *agent, Model: *model, Ref: *ref, Note: *note,
		Phase: run.Phase(*phase), DurationMS: *ms,
	}
	if e.Phase != "" && e.Phase.Index() < 0 {
		return a.usageErr(fmt.Sprintf("run event: unknown phase %q", *phase))
	}
	if e.Kind == run.KindLimits {
		if err := validateLimitsJSON(e.Note); err != nil {
			return a.usageErr("run event --kind limits: " + err.Error() + " (build it with `loops budget parse`)")
		}
	}
	if *ok != "" {
		b, err := strconv.ParseBool(*ok)
		if err != nil {
			return a.usageErr(fmt.Sprintf("run event: --ok must be true or false, got %q", *ok))
		}
		e.OK = run.Bool(b)
	}
	if err := a.Tracker.Record(ctx, e); err != nil {
		return a.fail("run event", err)
	}
	return 0
}

func (a *App) runLabel(ctx context.Context, args []string) int {
	fs := a.newFlagSet("run label")
	agent := fs.String("agent", "", "Claude agent id (required)")
	name := fs.String("name", "", "plan agent id, e.g. W1-domain (required)")
	role := fs.String("role", "", "")
	wave := fs.Int("wave", 0, "")
	model := fs.String("model", "", "model the agent actually runs on (after any fallback, WORKFLOW §3a)")
	pos, err := parseFlagsAnywhere(fs, args)
	if err != nil {
		return 2
	}
	if len(pos) > 0 || *agent == "" || *name == "" {
		return a.usageErr("usage: loops run label --agent <id> --name <plan id> [--role R] [--wave N] [--model M]")
	}
	e := run.Event{Kind: run.KindLabel, Agent: *agent, Name: *name, Role: *role, Wave: *wave, Model: *model}
	if err := a.Tracker.Record(ctx, e); err != nil {
		return a.fail("run label", err)
	}
	return 0
}

// currentPhase returns the active run's phase (and ErrNoActiveRun if none).
func (a *App) currentPhase(ctx context.Context) (run.Phase, error) {
	st, err := a.Tracker.State(ctx, "")
	if err != nil {
		return "", err
	}
	return st.Phase, nil
}

func (a *App) runLoopback(ctx context.Context, args []string) int {
	fs := a.newFlagSet("run loopback")
	from := fs.String("from", "", "reviewer (required)")
	to := fs.String("to", "", "author (required)")
	round := fs.Int("round", 0, "")
	note := fs.String("note", "", "")
	phase := fs.String("phase", "", "defaults to the current phase")
	pos, err := parseFlagsAnywhere(fs, args)
	if err != nil {
		return 2
	}
	if len(pos) > 0 || *from == "" || *to == "" {
		return a.usageErr("usage: loops run loopback --from R --to A --round N [--note text] [--phase P]")
	}
	p := run.Phase(*phase)
	if p == "" {
		if p, err = a.currentPhase(ctx); err != nil {
			return a.fail("run loopback", err)
		}
	} else if p.Index() < 0 {
		return a.usageErr(fmt.Sprintf("run loopback: unknown phase %q", *phase))
	}
	e := run.Event{Kind: run.KindLoopBack, From: *from, To: *to, Round: *round, Note: *note, Phase: p}
	if err := a.Tracker.Record(ctx, e); err != nil {
		return a.fail("run loopback", err)
	}
	return 0
}

func (a *App) runVerdict(ctx context.Context, args []string) int {
	fs := a.newFlagSet("run verdict")
	name := fs.String("name", "", "claim, AC or plan id (required)")
	status := fs.String("status", "", "APPROVE|CHANGES|VERIFIED|FAILED (required)")
	round := fs.Int("round", 0, "")
	ref := fs.String("ref", "", "")
	agent := fs.String("agent", "", "")
	role := fs.String("role", "", "")
	pos, err := parseFlagsAnywhere(fs, args)
	if err != nil {
		return 2
	}
	if len(pos) > 0 || *name == "" || *status == "" {
		return a.usageErr("usage: loops run verdict --name X --status S [--round N] [--ref f] [--agent A] [--role R]")
	}
	st := strings.ToUpper(*status)
	switch st {
	case run.StatusApprove, run.StatusChanges, run.StatusVerified, run.StatusFailed:
	default:
		return a.usageErr(fmt.Sprintf("run verdict: unknown status %q (want APPROVE, CHANGES, VERIFIED or FAILED)", *status))
	}
	p, err := a.currentPhase(ctx)
	if err != nil {
		return a.fail("run verdict", err)
	}
	e := run.Event{Kind: run.KindVerdict, Name: *name, Status: st, Round: *round, Ref: *ref, Agent: *agent, Role: *role, Phase: p}
	if err := a.Tracker.Record(ctx, e); err != nil {
		return a.fail("run verdict", err)
	}
	return 0
}

// runOverride records a project rule that superseded a loops rule (WORKFLOW §1).
func (a *App) runOverride(ctx context.Context, args []string) int {
	fs := a.newFlagSet("run override")
	rule := fs.String("rule", "", "loops rule superseded, e.g. \"SOLID §9\" (required)")
	project := fs.String("project", "", "project rule that won, with file:line (required)")
	note := fs.String("note", "", "what was done instead")
	agent := fs.String("agent", "", "")
	role := fs.String("role", "", "")
	pos, err := parseFlagsAnywhere(fs, args)
	if err != nil {
		return 2
	}
	if len(pos) > 0 || strings.TrimSpace(*rule) == "" || strings.TrimSpace(*project) == "" {
		return a.usageErr("usage: loops run override --rule <loops rule> --project <project rule, file:line> [--note text] [--agent A] [--role R]")
	}
	p, err := a.currentPhase(ctx)
	if err != nil {
		return a.fail("run override", err)
	}
	e := run.Event{Kind: run.KindOverride, Name: *rule, Target: *project, Note: *note, Agent: *agent, Role: *role, Phase: p}
	if err := a.Tracker.Record(ctx, e); err != nil {
		return a.fail("run override", err)
	}
	return 0
}

func (a *App) runEnd(ctx context.Context, args []string) int {
	fs := a.newFlagSet("run end")
	outcome := fs.String("outcome", run.OutcomeDone, "done|abandoned|failed")
	noClean := fs.Bool("no-clean", false, "keep the run's go cache and the build artifacts left in the project")
	pos, err := parseFlagsAnywhere(fs, args)
	if err != nil {
		return 2
	}
	if len(pos) > 0 {
		return a.usageErr("usage: loops run end [--outcome done|abandoned|failed] [--no-clean]")
	}
	switch *outcome {
	case run.OutcomeDone, run.OutcomeAbandoned, run.OutcomeFailed:
	default:
		return a.usageErr(fmt.Sprintf("run end: unknown outcome %q (want done, abandoned or failed)", *outcome))
	}
	if !*noClean {
		dir, err := a.Tracker.Dir(ctx, "")
		if err != nil {
			return a.fail("run end", err)
		}
		wsCmd{a}.endClean(ctx, dir) // records the cleanup note before the end event
	}
	if err := a.Tracker.End(ctx, *outcome); err != nil {
		return a.fail("run end", err)
	}
	return 0
}

func (a *App) runDir(ctx context.Context, args []string) int {
	fs := a.newFlagSet("run dir")
	fs.SetOutput(io.Discard)
	if _, err := parseFlagsAnywhere(fs, args); err != nil {
		return 2
	}
	dir, err := a.Tracker.Dir(ctx, "")
	if err != nil {
		return a.fail("run dir", err)
	}
	fmt.Fprintln(a.Out, dir)
	return 0
}
