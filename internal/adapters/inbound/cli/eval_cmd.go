package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/arxdsilva/ai-loops/internal/application/evalrun"
)

// EvalOptions configures one `loops eval` invocation.
type EvalOptions struct {
	Dir    string    // evals/ directory ("" = <project>/evals, then <plugin>/evals)
	Agent  string    // agent command template ("" = the default claude command)
	Budget string    // spend cap per scenario, USD
	Keep   bool      // keep the sandbox after a live run
	Log    io.Writer // receives the agent's output
}

// Evaluator runs behavioural evals (application/evalrun, wired in compose).
type Evaluator interface {
	Names(o EvalOptions) ([]string, error)
	Live(ctx context.Context, o EvalOptions, name string) (evalrun.Outcome, error)
	Recorded(o EvalOptions, name, runDir string) (evalrun.Outcome, error)
}

const evalUsage = "usage: loops eval list | <scenario|all> [--run <run dir>] [--evals dir] [--agent \"<cmd {prompt} …>\"] [--budget usd] [--keep] [--json]"

// cmdEval implements `loops eval`.
func cmdEval(a *App, ctx context.Context, args []string) int {
	if a.Evals == nil {
		fmt.Fprintln(a.Err, "loops eval: not available in this build")
		return 1
	}
	fs := a.newFlagSet("eval")
	o := EvalOptions{Log: a.Err}
	fs.StringVar(&o.Dir, "evals", "", "evals directory")
	fs.StringVar(&o.Agent, "agent", "", "agent command template")
	fs.StringVar(&o.Budget, "budget", "10", "spend cap per scenario (USD)")
	fs.BoolVar(&o.Keep, "keep", false, "keep the sandbox")
	runDir := fs.String("run", "", "evaluate this recorded run directory instead of running the agent")
	asJSON := fs.Bool("json", false, "print JSON")
	pos, err := parseFlagsAnywhere(fs, args)
	if err != nil {
		return 2
	}
	if len(pos) != 1 {
		return a.usageErr(evalUsage)
	}
	names, err := a.Evals.Names(o)
	if err != nil {
		fmt.Fprintln(a.Err, "loops eval:", err)
		return 1
	}
	switch pos[0] {
	case "list":
		for _, n := range names {
			fmt.Fprintln(a.Out, n)
		}
		return 0
	case "all":
		if *runDir != "" {
			return a.usageErr("loops eval: --run takes one scenario, not all")
		}
	default:
		names = pos[:1]
	}
	if *runDir == "" {
		fmt.Fprintf(a.Err, "loops eval: live run of %d scenario(s); each runs a real agent and spends tokens (cap $%s each)\n", len(names), o.Budget)
	}
	outs := make([]evalrun.Outcome, 0, len(names))
	failed := false
	for _, n := range names {
		var out evalrun.Outcome
		if *runDir != "" {
			out, err = a.Evals.Recorded(o, n, *runDir)
		} else {
			out, err = a.Evals.Live(ctx, o, n)
		}
		if err != nil {
			fmt.Fprintf(a.Err, "loops eval: %s: %v\n", n, err)
			return 1
		}
		failed = failed || !out.OK
		outs = append(outs, out)
		if !*asJSON {
			printEval(a.Out, out, *runDir == "")
		}
	}
	if *asJSON {
		b, _ := json.MarshalIndent(outs, "", "  ")
		fmt.Fprintln(a.Out, string(b))
	} else {
		passed := 0
		for _, o := range outs {
			if o.OK {
				passed++
			}
		}
		fmt.Fprintf(a.Out, "%d/%d scenarios passed\n", passed, len(outs))
	}
	if failed {
		return 1
	}
	return 0
}

// printEval prints one outcome. After a live run without --keep the
// sandbox is gone, so only the run id is shown.
func printEval(w io.Writer, o evalrun.Outcome, live bool) {
	fmt.Fprintf(w, "%s %s\n", verdictWord(o.OK), o.Scenario)
	for _, c := range o.Checks {
		fmt.Fprintf(w, "  %-4s %-14s %s\n", verdictWord(c.OK), c.Kind, c.Detail)
	}
	switch {
	case o.Run == "":
		fmt.Fprintln(w, "  run: none recorded")
	case live && o.Sandbox == "":
		fmt.Fprintf(w, "  run: %s (sandbox removed; --keep keeps it)\n", filepath.Base(o.Run))
	default:
		fmt.Fprintf(w, "  run: %s\n", o.Run)
	}
	if o.AgentErr != "" {
		fmt.Fprintf(w, "  agent: %s\n", o.AgentErr)
	}
	if o.Sandbox != "" {
		fmt.Fprintf(w, "  sandbox: %s\n", o.Sandbox)
	}
}

func verdictWord(ok bool) string {
	if ok {
		return "PASS"
	}
	return "FAIL"
}
