package evalrun

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/arxdsilva/ai-loops/internal/domain/evals"
	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

type fakeCatalog map[string]Spec

func (f fakeCatalog) Names() ([]string, error) { return []string{"a"}, nil }

func (f fakeCatalog) Load(name string) (Spec, error) {
	s, ok := f[name]
	if !ok {
		return Spec{}, errors.New("no scenario " + name)
	}
	return s, nil
}

type fakeBox struct {
	err     error
	removed []string
}

func (b *fakeBox) Prepare(context.Context, Spec) (string, error) { return "/sb", b.err }
func (b *fakeBox) Remove(dir string) error                       { b.removed = append(b.removed, dir); return nil }

type fakeAgent struct {
	err     error
	prompt  string
	timeout time.Duration
}

func (a *fakeAgent) Run(_ context.Context, dir, prompt string, timeout time.Duration) error {
	a.prompt, a.timeout = prompt, timeout
	return a.err
}

type fakeRuns struct {
	latest           string
	latestErr, evErr error
	planErr          error
	events           []run.Event
	plan             string
	files            map[string]string
	reads            int
}

func (r *fakeRuns) Latest(string) (string, error)      { return r.latest, r.latestErr }
func (r *fakeRuns) Events(string) ([]run.Event, error) { return r.events, r.evErr }
func (r *fakeRuns) LatestPlan(string) (string, error)  { return r.plan, r.planErr }
func (r *fakeRuns) Read(_, rel string) (string, bool) {
	r.reads++
	b, ok := r.files[rel]
	return b, ok
}

const plan = `### Wave 1
| id | role | owns |
|---|---|---|
| W1-d | domain-engineer | internal/domain/** |
`

func spec(a ...evals.Assertion) Spec {
	return Spec{Scenario: evals.Scenario{Name: "a", Assertions: a}, Prompt: "go"}
}

func passingRuns() *fakeRuns {
	return &fakeRuns{
		latest: "/sb/.loops/runs/r1",
		plan:   plan,
		events: []run.Event{
			{Kind: run.KindVerdict, Name: "W1-d:C1", Status: run.StatusFailed, Round: 1, Ref: "qa/W1-d.r1.md"},
			{Kind: run.KindVerdict, Name: "W1-d:C2", Status: run.StatusFailed, Round: 1, Ref: "qa/W1-d.r1.md"},
			{Kind: run.KindVerdict, Name: "plan", Status: run.StatusApprove},
			{Kind: run.KindAgentStart, Agent: "x", Role: "domain-engineer"},
			{Kind: run.KindLabel, Agent: "x", Name: "W1-d"},
			{Kind: run.KindStep, Agent: "x", Role: "domain-engineer", Name: "Write", Target: "internal/domain/p.go", OK: run.Bool(true)},
		},
		files: map[string]string{"qa/W1-d.r1.md": "Repro: go test"},
	}
}

func TestLive_PassesAndRemovesTheSandbox(t *testing.T) {
	box, agent, runs := &fakeBox{}, &fakeAgent{}, passingRuns()
	sc := spec(
		evals.Assertion{Kind: evals.KindQAFailsClaim, Marker: "Repro:"},
		evals.Assertion{Kind: evals.KindScope, MinLanes: 1},
	)
	sc.Scenario.Timeout = "5m"
	svc := New(fakeCatalog{"a": sc}, box, agent, runs)
	out, err := svc.Live(context.Background(), "a")
	if err != nil {
		t.Fatal(err)
	}
	if !out.OK || out.Run != "/sb/.loops/runs/r1" || out.Sandbox != "" || out.AgentErr != "" {
		t.Fatalf("outcome = %+v", out)
	}
	if agent.prompt != "go" || agent.timeout != 5*time.Minute {
		t.Fatalf("agent got %q, %s", agent.prompt, agent.timeout)
	}
	if len(box.removed) != 1 || box.removed[0] != "/sb" {
		t.Fatalf("removed = %v", box.removed)
	}
	if runs.reads != 1 {
		t.Fatalf("a ref cited twice must be read once; reads = %d", runs.reads)
	}
}

func TestLive_KeepAndAgentFailure(t *testing.T) {
	box := &fakeBox{}
	svc := New(fakeCatalog{"a": spec(evals.Assertion{Kind: evals.KindGateHolds})}, box, &fakeAgent{err: errors.New("exit status 1")}, &fakeRuns{})
	svc.Keep = true
	out, err := svc.Live(context.Background(), "a")
	if err != nil {
		t.Fatal(err)
	}
	if out.OK || out.Sandbox != "/sb" || out.AgentErr != "exit status 1" || out.Run != "" || len(box.removed) != 0 {
		t.Fatalf("outcome = %+v, removed %v", out, box.removed)
	}
	if out.Checks[0].Detail != "no events recorded" {
		t.Fatalf("checks = %+v", out.Checks)
	}
}

func TestLive_DefaultTimeout(t *testing.T) {
	agent := &fakeAgent{}
	svc := New(fakeCatalog{"a": spec(evals.Assertion{Kind: evals.KindGateHolds})}, &fakeBox{}, agent, &fakeRuns{})
	if _, err := svc.Live(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	if agent.timeout != DefaultTimeout {
		t.Fatalf("timeout = %s", agent.timeout)
	}
}

func TestLive_Errors(t *testing.T) {
	good := spec(evals.Assertion{Kind: evals.KindGateHolds})
	badTimeout := good
	badTimeout.Scenario.Timeout = "soon"
	for name, tc := range map[string]struct {
		svc  *Service
		want string
	}{
		"no sandbox":  {New(fakeCatalog{"a": good}, nil, nil, &fakeRuns{}), "not available"},
		"unknown":     {New(fakeCatalog{}, &fakeBox{}, &fakeAgent{}, &fakeRuns{}), "no scenario"},
		"invalid":     {New(fakeCatalog{"a": spec()}, &fakeBox{}, &fakeAgent{}, &fakeRuns{}), "no assertions"},
		"bad timeout": {New(fakeCatalog{"a": badTimeout}, &fakeBox{}, &fakeAgent{}, &fakeRuns{}), "timeout"},
		"prepare":     {New(fakeCatalog{"a": good}, &fakeBox{err: errors.New("disk full")}, &fakeAgent{}, &fakeRuns{}), "prepare: disk full"},
		"latest":      {New(fakeCatalog{"a": good}, &fakeBox{}, &fakeAgent{}, &fakeRuns{latestErr: errors.New("perm")}), "perm"},
		"events":      {New(fakeCatalog{"a": good}, &fakeBox{}, &fakeAgent{}, &fakeRuns{latest: "/r", evErr: errors.New("io")}), "io"},
		"plan":        {New(fakeCatalog{"a": good}, &fakeBox{}, &fakeAgent{}, &fakeRuns{latest: "/r", planErr: errors.New("glob")}), "glob"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := tc.svc.Live(context.Background(), "a"); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestRecorded(t *testing.T) {
	runs := passingRuns()
	svc := New(fakeCatalog{"a": spec(evals.Assertion{Kind: evals.KindScope, MinLanes: 1})}, nil, nil, runs)
	out, err := svc.Recorded("a", "/rec")
	if err != nil || !out.OK || out.Run != "/rec" {
		t.Fatalf("Recorded = %+v, %v", out, err)
	}
	if _, err := svc.Recorded("zz", "/rec"); err == nil {
		t.Fatal("unknown scenario: want an error")
	}
	if names, err := svc.Names(); err != nil || len(names) != 1 {
		t.Fatalf("Names = %v, %v", names, err)
	}
}

func TestLanes(t *testing.T) {
	if lanes("") != nil {
		t.Fatal("no plan, no lanes")
	}
	got := lanes(plan)
	if len(got) != 1 || got[0].ID != "W1-d" || got[0].Role != "domain-engineer" || got[0].Owns[0] != "internal/domain/**" {
		t.Fatalf("lanes = %+v", got)
	}
}
