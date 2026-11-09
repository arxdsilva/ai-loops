package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/arxdsilva/ai-loops/internal/application/tracker"
	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

// exitOverBudget is `loops budget check`'s exit code when a limit blocks the spawn.
const exitOverBudget = 3

// cmdBudget implements `loops budget check|show|parse` (WORKFLOW §4a).
func (a *App) cmdBudget(ctx context.Context, args []string) int {
	if len(args) == 0 {
		return a.usageErr("usage: loops budget check --phase P [--spawn N] [--role R] [--verbose] | show | parse [--budget …] [--agents …] [--roles …] [--max-agents N]")
	}
	b := budgetCmd{a}
	switch args[0] {
	case "check":
		return b.check(ctx, args[1:])
	case "show":
		return b.show(ctx, args[1:])
	case "parse":
		return b.parse(args[1:])
	}
	return a.usageErr(fmt.Sprintf("budget: unknown subcommand %q (want check, show or parse)", args[0]))
}

// budgetCmd holds the `loops budget` subcommands, off App's method set.
type budgetCmd struct{ *App }

// activeLimits returns the run state and the limits in force: the config
// limits overlaid with the run's recorded limits (the run wins field by
// field). ok is false when no run is active.
func (a budgetCmd) activeLimits(ctx context.Context) (st run.State, l run.Limits, ok bool, err error) {
	st, err = a.Tracker.State(ctx, "")
	if errors.Is(err, tracker.ErrNoActiveRun) {
		return run.State{}, a.Limits, false, nil
	}
	if err != nil {
		return run.State{}, run.Limits{}, false, err
	}
	return st, a.Limits.Merge(st.Limits), true, nil
}

func (a budgetCmd) check(ctx context.Context, args []string) int {
	fs := a.newFlagSet("budget check")
	phase := fs.String("phase", "", "phase the agents would open in (default: the current phase)")
	spawn := fs.Int("spawn", 1, "agents about to be spawned")
	role := fs.String("role", "", "role of the agents")
	verbose := fs.Bool("verbose", false, "print a line when the spawn fits")
	pos, err := parseFlagsAnywhere(fs, args)
	if err != nil {
		return 2
	}
	if len(pos) > 0 {
		return a.usageErr("budget check: unexpected argument " + pos[0])
	}
	if *spawn < 1 {
		return a.usageErr(fmt.Sprintf("budget check: --spawn must be ≥ 1, got %d", *spawn))
	}
	p := run.Phase(*phase)
	if p != "" && p.Index() < 0 {
		return a.usageErr(fmt.Sprintf("budget check: unknown phase %q (want one of %v)", *phase, run.Phases))
	}
	st, lim, active, err := a.activeLimits(ctx)
	if err != nil {
		return a.fail("budget check", err)
	}
	if !active {
		return 0 // nothing to limit
	}
	if p == "" {
		p = st.Phase
	}
	d := run.CheckBudget(st, lim, p, *spawn, *role)
	if d.OK {
		if *verbose {
			fmt.Fprintf(a.Out, "fits: %d × %s in %s\n", *spawn, orAny(*role), p)
		}
		return 0
	}
	fmt.Fprintf(a.Out, "over budget (%s): %s\nStop spawning in %s, let running agents finish, and ask the user to raise the limit, continue past it once, or stop the run (WORKFLOW §4a).\n", d.Limit, d.Reason, p)
	if err := a.Tracker.Record(ctx, run.Event{Kind: run.KindBudget, Phase: p, Role: *role, Name: d.Limit, Note: d.Reason}); err != nil {
		fmt.Fprintf(a.Err, "loops budget check: recording the budget event: %v\n", err)
	}
	return exitOverBudget
}

func orAny(role string) string {
	if role == "" {
		return "agent"
	}
	return role
}

func (a budgetCmd) show(ctx context.Context, args []string) int {
	fs := a.newFlagSet("budget show")
	pos, err := parseFlagsAnywhere(fs, args)
	if err != nil {
		return 2
	}
	if len(pos) > 0 {
		return a.usageErr("budget show: unexpected argument " + pos[0])
	}
	st, lim, active, err := a.activeLimits(ctx)
	if err != nil {
		return a.fail("budget show", err)
	}
	if lim.IsZero() {
		fmt.Fprintln(a.Out, "no limits set")
		return 0
	}
	writeBudget(a.Out, st, lim, active)
	return 0
}

// writeBudget prints usage against lim: the run cap, then one row per
// limited phase in workflow order, then the blocked spawns.
func writeBudget(w io.Writer, st run.State, lim run.Limits, active bool) {
	if active {
		fmt.Fprintf(w, "run %s, phase %s\n", st.ID, st.Phase)
	} else {
		fmt.Fprintln(w, "no active run: limits from .loops/config.json, no usage yet")
	}
	if lim.MaxAgents > 0 {
		running := 0
		for _, l := range st.Lanes {
			if l.Running {
				running++
			}
		}
		fmt.Fprintf(w, "run cap: %d running / %d max agents\n", running, lim.MaxAgents)
	}
	if len(lim.Phases) > 0 {
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "phase\ttokens\tagents\troles")
		for _, p := range run.Phases {
			pl, ok := lim.Phases[p]
			if !ok {
				continue
			}
			opened := 0
			for _, l := range st.Lanes {
				if l.Phase == p {
					opened++
				}
			}
			tokens := run.FormatTokens(st.PhaseTokens[p])
			if pl.MaxTokens > 0 {
				tokens += " / " + run.FormatTokens(pl.MaxTokens)
			}
			agents := fmt.Sprint(opened)
			if pl.MaxAgents > 0 {
				agents += fmt.Sprintf(" / %d", pl.MaxAgents)
			}
			roles := "any"
			if len(pl.Roles) > 0 {
				roles = strings.Join(pl.Roles, ", ")
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", p, tokens, agents, roles)
		}
		_ = tw.Flush()
	}
	if n := len(st.Budgets); n > 0 {
		last := st.Budgets[n-1]
		fmt.Fprintf(w, "blocked spawns: %d (latest: %s)\n", n, last.Reason)
	}
}

func (a budgetCmd) parse(args []string) int {
	fs := a.newFlagSet("budget parse")
	budget := fs.String("budget", "", "tokens per phase, e.g. research=200k,implement=2M")
	agents := fs.String("agents", "", "agents per phase, e.g. implement=12")
	roles := fs.String("roles", "", "roles allowed per phase, e.g. implement=domain-engineer,api-engineer;qa=qa-verifier")
	maxAgents := fs.Int("max-agents", 0, "concurrent agents for the whole run")
	pos, err := parseFlagsAnywhere(fs, args)
	if err != nil {
		return 2
	}
	if len(pos) > 0 {
		return a.usageErr("budget parse: unexpected argument " + pos[0])
	}
	l, err := run.ParseLimits(*budget, *agents, *roles, *maxAgents)
	if err != nil {
		return a.usageErr("budget parse: " + err.Error())
	}
	b, err := json.Marshal(l)
	if err != nil {
		return a.fail("budget parse", err)
	}
	fmt.Fprintln(a.Out, string(b))
	return 0
}

// validateLimitsJSON checks a `run event --kind limits` note: one JSON
// object of run.Limits with known fields and phases and no negative values.
func validateLimitsJSON(note string) error {
	if !strings.HasPrefix(strings.TrimSpace(note), "{") {
		return errors.New("--note must be a JSON object of limits")
	}
	dec := json.NewDecoder(strings.NewReader(note))
	dec.DisallowUnknownFields()
	var l run.Limits
	if err := dec.Decode(&l); err != nil {
		return fmt.Errorf("--note is not limits JSON: %v", err)
	}
	if dec.More() {
		return errors.New("--note has data after the limits object")
	}
	if l.MaxAgents < 0 {
		return fmt.Errorf("max_agents must be ≥ 0, got %d", l.MaxAgents)
	}
	for p, pl := range l.Phases {
		if p.Index() < 0 {
			return fmt.Errorf("unknown phase %q (want one of %v)", p, run.Phases)
		}
		if pl.MaxTokens < 0 || pl.MaxAgents < 0 {
			return fmt.Errorf("%s: max_tokens and max_agents must be ≥ 0", p)
		}
	}
	return nil
}
