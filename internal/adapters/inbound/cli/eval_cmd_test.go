package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/application/evalrun"
	"github.com/arxdsilva/ai-loops/internal/domain/evals"
)

type fakeEvals struct {
	names    []string
	namesErr error
	results  map[string]bool
	err      error
	live     []string
	recorded []string
	opts     EvalOptions
}

func (f *fakeEvals) Names(o EvalOptions) ([]string, error) { f.opts = o; return f.names, f.namesErr }

func (f *fakeEvals) outcome(name, runDir string) evalrun.Outcome {
	ok := f.results[name]
	return evalrun.Outcome{Result: evals.Result{
		Scenario: name, OK: ok, Run: runDir,
		Checks: []evals.Check{{Kind: evals.KindScope, OK: ok, Detail: "detail of " + name}},
	}}
}

func (f *fakeEvals) Live(_ context.Context, o EvalOptions, name string) (evalrun.Outcome, error) {
	f.live = append(f.live, name)
	switch name {
	case "gate":
		out := f.outcome(name, "/sb/gate/.loops/runs/r1")
		out.Sandbox, out.AgentErr = "/sb/gate", "exit status 1"
		return out, f.err
	case "scope":
		return f.outcome(name, "/gone/.loops/runs/r2"), f.err
	}
	return f.outcome(name, ""), f.err
}

func (f *fakeEvals) Recorded(o EvalOptions, name, runDir string) (evalrun.Outcome, error) {
	f.recorded = append(f.recorded, name+"@"+runDir)
	return f.outcome(name, runDir), f.err
}

func evalApp(f Evaluator) (*App, *bytes.Buffer, *bytes.Buffer) {
	var out, errw bytes.Buffer
	return &App{Evals: f, Out: &out, Err: &errw}, &out, &errw
}

func TestEval_ListAndAll(t *testing.T) {
	f := &fakeEvals{names: []string{"gate", "scope", "test-first"}, results: map[string]bool{"gate": true}}
	a, out, errw := evalApp(f)
	if c := a.Run(context.Background(), []string{"eval", "list"}); c != 0 || out.String() != "gate\nscope\ntest-first\n" {
		t.Fatalf("list: %d %q", c, out)
	}
	out.Reset()
	if c := a.Run(context.Background(), []string{"eval", "all", "--budget", "3", "--keep", "--agent", "x {prompt}"}); c != 1 {
		t.Fatalf("all with a failing scenario: exit %d, want 1", c)
	}
	if strings.Join(f.live, ",") != "gate,scope,test-first" || f.opts.Budget != "3" || !f.opts.Keep || f.opts.Agent != "x {prompt}" {
		t.Fatalf("live = %v, opts = %+v", f.live, f.opts)
	}
	for _, want := range []string{"PASS gate", "FAIL scope", "detail of scope", "run: /sb/gate/.loops/runs/r1", "run: r2 (sandbox removed; --keep keeps it)", "run: none recorded", "agent: exit status 1", "sandbox: /sb/gate", "1/3 scenarios passed"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if !strings.Contains(errw.String(), "spends tokens (cap $3 each)") {
		t.Errorf("no cost warning: %q", errw)
	}
}

func TestEval_RecordedOneScenario(t *testing.T) {
	f := &fakeEvals{names: []string{"gate"}, results: map[string]bool{"gate": true}}
	a, out, errw := evalApp(f)
	if c := a.Run(context.Background(), []string{"eval", "gate", "--run", "/rec"}); c != 0 {
		t.Fatalf("exit %d: %s %s", c, out, errw)
	}
	if strings.Join(f.recorded, ",") != "gate@/rec" || len(f.live) != 0 || errw.Len() != 0 {
		t.Fatalf("recorded = %v live = %v err = %q", f.recorded, f.live, errw)
	}
	if !strings.Contains(out.String(), "run: /rec") || !strings.Contains(out.String(), "1/1 scenarios passed") {
		t.Fatalf("output:\n%s", out)
	}
}

func TestEval_JSON(t *testing.T) {
	f := &fakeEvals{names: []string{"gate"}, results: map[string]bool{"gate": true}}
	a, out, _ := evalApp(f)
	if c := a.Run(context.Background(), []string{"eval", "gate", "--run", "/rec", "--json"}); c != 0 {
		t.Fatalf("exit %d", c)
	}
	var got []evalrun.Outcome
	if err := json.Unmarshal(out.Bytes(), &got); err != nil || len(got) != 1 || !got[0].OK || got[0].Scenario != "gate" {
		t.Fatalf("json = %s (%v)", out, err)
	}
}

func TestEval_Errors(t *testing.T) {
	for name, tc := range map[string]struct {
		f    Evaluator
		args []string
		code int
		want string
	}{
		"unavailable":    {nil, []string{"eval", "all"}, 1, "not available"},
		"no args":        {&fakeEvals{}, []string{"eval"}, 2, "usage: loops eval"},
		"bad flag":       {&fakeEvals{}, []string{"eval", "all", "--nope"}, 2, ""},
		"run with all":   {&fakeEvals{}, []string{"eval", "all", "--run", "/r"}, 2, "--run takes one scenario"},
		"names fail":     {&fakeEvals{namesErr: errors.New("no evals dir")}, []string{"eval", "all"}, 1, "no evals dir"},
		"scenario fails": {&fakeEvals{err: errors.New("prepare: boom")}, []string{"eval", "gate"}, 1, "gate: prepare: boom"},
	} {
		t.Run(name, func(t *testing.T) {
			var a *App
			var errw *bytes.Buffer
			if tc.f == nil {
				a, _, errw = evalApp(nil)
			} else {
				a, _, errw = evalApp(tc.f)
			}
			if c := a.Run(context.Background(), tc.args); c != tc.code || !strings.Contains(errw.String(), tc.want) {
				t.Fatalf("exit %d (want %d), stderr %q (want %q)", c, tc.code, errw, tc.want)
			}
		})
	}
}
