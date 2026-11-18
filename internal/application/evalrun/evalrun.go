// Package evalrun runs behavioural evals (`loops eval`): it prepares a copy
// of the fixture repo for a scenario, runs an agent on it, and evaluates the
// run the agent left behind (domain/evals). It also evaluates a run that was
// recorded earlier, with no agent. Every effect goes through a port.
package evalrun

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/arxdsilva/ai-loops/internal/application/plancheck"
	"github.com/arxdsilva/ai-loops/internal/domain/evals"
	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

// DefaultTimeout bounds a live scenario whose scenario.json sets none.
const DefaultTimeout = 45 * time.Minute

// Spec is a loaded scenario: its assertions and the prompt the agent gets.
type Spec struct {
	Scenario evals.Scenario
	// Prompt is the agent prompt with the ticket filled in.
	Prompt string
	// Dir is the scenario directory (overlay.patch, seed/).
	Dir string
}

// Catalog lists and loads scenarios (evals/scenarios/<name>/).
type Catalog interface {
	Names() ([]string, error)
	Load(name string) (Spec, error)
}

// Sandbox prepares a throwaway git repo from the fixture with the
// scenario's overlay applied, outside the user's project.
type Sandbox interface {
	Prepare(ctx context.Context, spec Spec) (dir string, err error)
	Remove(dir string) error
}

// Agent runs the coding agent on dir with prompt, for at most timeout.
type Agent interface {
	Run(ctx context.Context, dir, prompt string, timeout time.Duration) error
}

// Runs reads what a loops run left in a project.
type Runs interface {
	// Latest returns the newest run directory under dir/.loops/runs, or ""
	// when there is none.
	Latest(dir string) (string, error)
	Events(runDir string) ([]run.Event, error)
	// Read returns a run-dir-relative file; ok is false when it can't be read.
	Read(runDir, rel string) (body string, ok bool)
	// LatestPlan returns the highest plan.v<N>.md, or "" when there is none.
	LatestPlan(runDir string) (string, error)
}

// Service is the eval use case.
type Service struct {
	cat   Catalog
	box   Sandbox
	agent Agent
	runs  Runs
	// Keep leaves the sandbox on disk after a live run.
	Keep bool
}

// New builds the service; box and agent may be nil when only recorded runs
// are evaluated.
func New(cat Catalog, box Sandbox, agent Agent, runs Runs) *Service {
	return &Service{cat: cat, box: box, agent: agent, runs: runs}
}

// Names lists the scenarios, sorted.
func (s *Service) Names() ([]string, error) { return s.cat.Names() }

// Outcome is one scenario's result plus how the live run went.
type Outcome struct {
	evals.Result
	// Sandbox is the prepared repo (kept only with Keep).
	Sandbox string `json:"sandbox,omitempty"`
	// AgentErr is the agent's failure (exit status, timeout), if any; the
	// run it left is still evaluated.
	AgentErr string `json:"agent_error,omitempty"`
}

// Live prepares the sandbox, runs the agent and evaluates the run it
// recorded.
func (s *Service) Live(ctx context.Context, name string) (Outcome, error) {
	if s.box == nil || s.agent == nil {
		return Outcome{}, errors.New("live evals are not available")
	}
	spec, err := s.load(name)
	if err != nil {
		return Outcome{}, err
	}
	timeout := DefaultTimeout
	if spec.Scenario.Timeout != "" {
		if timeout, err = time.ParseDuration(spec.Scenario.Timeout); err != nil {
			return Outcome{}, fmt.Errorf("scenario %s: timeout: %w", name, err)
		}
	}
	dir, err := s.box.Prepare(ctx, spec)
	if err != nil {
		return Outcome{}, fmt.Errorf("scenario %s: prepare: %w", name, err)
	}
	out := Outcome{}
	if s.Keep {
		out.Sandbox = dir
	} else {
		defer func() { _ = s.box.Remove(dir) }()
	}
	if err := s.agent.Run(ctx, dir, spec.Prompt, timeout); err != nil {
		out.AgentErr = err.Error()
	}
	runDir, err := s.runs.Latest(dir)
	if err != nil {
		return out, err
	}
	out.Result, err = s.evaluate(spec.Scenario, runDir)
	return out, err
}

// Recorded evaluates scenario name against a run directory recorded
// earlier: no sandbox, no agent, no tokens.
func (s *Service) Recorded(name, runDir string) (Outcome, error) {
	spec, err := s.load(name)
	if err != nil {
		return Outcome{}, err
	}
	res, err := s.evaluate(spec.Scenario, runDir)
	return Outcome{Result: res}, err
}

func (s *Service) load(name string) (Spec, error) {
	spec, err := s.cat.Load(name)
	if err != nil {
		return Spec{}, err
	}
	return spec, spec.Scenario.Validate()
}

// evaluate gathers the evidence of runDir ("" = no run recorded) and checks
// it.
func (s *Service) evaluate(sc evals.Scenario, runDir string) (evals.Result, error) {
	ev := evals.Evidence{Refs: map[string]string{}}
	if runDir != "" {
		var err error
		if ev.Events, err = s.runs.Events(runDir); err != nil {
			return evals.Result{}, err
		}
		plan, err := s.runs.LatestPlan(runDir)
		if err != nil {
			return evals.Result{}, err
		}
		ev.Lanes = lanes(plan)
		for _, e := range ev.Events {
			if e.Kind != run.KindVerdict || e.Ref == "" {
				continue
			}
			if _, done := ev.Refs[e.Ref]; done {
				continue
			}
			if body, ok := s.runs.Read(runDir, e.Ref); ok {
				ev.Refs[e.Ref] = body
			}
		}
	}
	res := evals.Evaluate(sc, ev)
	res.Run = runDir
	return res, nil
}

// lanes reads the plan's waves table.
func lanes(plan string) []evals.Lane {
	if plan == "" {
		return nil
	}
	var out []evals.Lane
	for _, a := range plancheck.Check(plan, 0).Agents {
		out = append(out, evals.Lane{ID: a.ID, Role: a.Role, Owns: a.Owns})
	}
	return out
}
