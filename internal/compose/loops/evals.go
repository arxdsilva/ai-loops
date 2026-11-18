package loops

import (
	"context"
	"os"
	"path/filepath"

	"github.com/arxdsilva/ai-loops/internal/adapters/inbound/cli"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/evalbox"
	"github.com/arxdsilva/ai-loops/internal/application/evalrun"
)

// evaluator wires `loops eval`: each call builds the use case for the
// evals directory and agent options of that invocation.
type evaluator struct {
	root, pluginRoot string
}

// dir resolves the evals directory: the flag, then <project>/evals, then
// <plugin root>/evals.
func (e evaluator) dir(o cli.EvalOptions) string {
	if o.Dir != "" {
		return o.Dir
	}
	for _, base := range []string{e.root, e.pluginRoot} {
		if base == "" {
			continue
		}
		d := filepath.Join(base, "evals")
		if fi, err := os.Stat(filepath.Join(d, "scenarios")); err == nil && fi.IsDir() {
			return d
		}
	}
	return filepath.Join(e.root, "evals")
}

// plugin is the plugin the agent loads: the checkout that holds the evals
// directory when it is one (so a live eval tests this tree), else the
// running plugin root.
func (e evaluator) plugin(evalsDir string) string {
	parent := filepath.Dir(evalsDir)
	if _, err := os.Stat(filepath.Join(parent, ".claude-plugin", "plugin.json")); err == nil {
		return parent
	}
	return e.pluginRoot
}

func (e evaluator) service(o cli.EvalOptions) *evalrun.Service {
	d := e.dir(o)
	svc := evalrun.New(
		evalbox.Catalog{Root: d},
		evalbox.Sandbox{Fixture: filepath.Join(d, "fixture")},
		evalbox.Agent{Template: o.Agent, Plugin: e.plugin(d), Budget: o.Budget, Out: o.Log},
		evalbox.Runs{},
	)
	svc.Keep = o.Keep
	return svc
}

func (e evaluator) Names(o cli.EvalOptions) ([]string, error) { return e.service(o).Names() }

func (e evaluator) Live(ctx context.Context, o cli.EvalOptions, name string) (evalrun.Outcome, error) {
	return e.service(o).Live(ctx, name)
}

func (e evaluator) Recorded(o cli.EvalOptions, name, runDir string) (evalrun.Outcome, error) {
	return e.service(o).Recorded(name, runDir)
}
