package cli

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

// lastTriage is the newest triage event, decoded.
func (e *rcEnv) lastTriage(t *testing.T) run.Triage {
	t.Helper()
	evs := e.events()
	for i := len(evs) - 1; i >= 0; i-- {
		if evs[i].Kind == run.KindTriage {
			var tr run.Triage
			if err := json.Unmarshal([]byte(evs[i].Note), &tr); err != nil {
				t.Fatalf("triage note: %v", err)
			}
			return tr
		}
	}
	t.Fatal("no triage event")
	return run.Triage{}
}

func TestRunTriage(t *testing.T) {
	e := rcSetup(t, "")
	if e.do("run", "triage", "--kind", "typo", "--files", "1") != 1 || !strings.Contains(e.err.String(), "no active run") {
		t.Fatalf("no run: %q", e.err)
	}
	e.do("run", "start", "--title", "x")

	if e.do("run", "triage", "--kind", "typo", "--files", "1") != 0 {
		t.Fatalf("trivial: %q", e.err)
	}
	out := e.out.String()
	for _, want := range []string{"triage: trivial (kind=typo files=1 langs=1 touches=none)", "skip: clarify, memory,", "run: none; loops gate always"} {
		if !strings.Contains(out, want) {
			t.Errorf("output misses %q:\n%s", want, out)
		}
	}
	if tr := e.lastTriage(t); tr.Tier != run.TierTrivial || len(tr.Skip) != len(run.Steps) {
		t.Errorf("recorded %+v", tr)
	}
	for step, want := range map[string]string{"qa-verify": "skip", "reflect": "skip"} {
		if e.do("run", "step", step) != 0 || strings.TrimSpace(e.out.String()) != want {
			t.Errorf("step %s = %q, want %s", step, e.out, want)
		}
	}

	// A bigger real diff adds the later steps back.
	e.do("run", "phase", "implement")
	if e.do("run", "triage", "--recheck", "--files", "9") != 0 {
		t.Fatalf("recheck: %q", e.err)
	}
	tr := e.lastTriage(t)
	if tr.Tier != run.TierStandard || !tr.Recheck || tr.Signals.Kind != run.TaskTypo {
		t.Errorf("recheck recorded %+v", tr)
	}
	if e.do("run", "step", "qa-verify"); strings.TrimSpace(e.out.String()) != "run" {
		t.Errorf("qa-verify after recheck = %q, want run", e.out)
	}
	if e.do("run", "step", "planner"); strings.TrimSpace(e.out.String()) != "skip" {
		t.Errorf("planner after recheck = %q, want skip (its phase passed)", e.out)
	}
}

func TestRunTriageOverridesAndFull(t *testing.T) {
	e := rcSetup(t, "")
	e.do("run", "start", "--title", "x")

	if e.do("run", "triage", "--kind", "bugfix", "--files", "2", "--keep", "qa-quality") != 2 || !strings.Contains(e.err.String(), "need --reason") {
		t.Fatalf("override without reason: %q", e.err)
	}
	if e.do("run", "triage", "--kind", "bugfix", "--files", "2", "--keep", "qa-quality", "--skip", "memory", "--reason", "hot path") != 0 {
		t.Fatalf("override: %q", e.err)
	}
	tr := e.lastTriage(t)
	if slices.Contains(tr.Skip, run.StepQuality) || !slices.Contains(tr.Skip, run.StepMemory) {
		t.Errorf("overrides not applied: %v", tr.Skip)
	}
	evs := e.events()
	if d := evs[len(evs)-1]; d.Kind != run.KindDecision || d.Name != "triage" || !strings.Contains(d.Note, "hot path") {
		t.Errorf("override decision: %+v", d)
	}

	if e.do("run", "triage", "--full") != 0 || !strings.Contains(e.out.String(), "triage: standard, full") {
		t.Fatalf("full: %q %q", e.out, e.err)
	}
	if tr := e.lastTriage(t); !tr.Full || tr.Skip != nil {
		t.Errorf("full recorded %+v", tr)
	}

	e.app.TriageOff = true
	if e.do("run", "triage", "--kind", "typo", "--files", "1") != 0 || !e.lastTriage(t).Full {
		t.Errorf("config off must force full: %q", e.out)
	}
}

func TestRunTriageUsage(t *testing.T) {
	e := rcSetup(t, "")
	e.do("run", "start", "--title", "x")
	bad := []struct {
		args []string
		want string
	}{
		{[]string{"run", "triage", "--files", "1"}, "--kind is required"},
		{[]string{"run", "triage", "--kind", "typo"}, "--files N is required"},
		{[]string{"run", "triage", "--kind", "chore", "--files", "1"}, "unknown kind"},
		{[]string{"run", "triage", "--kind", "typo", "--files", "1", "--touches", "ui"}, "unknown touch"},
		{[]string{"run", "triage", "--kind", "typo", "--files", "1", "--skip", "gate", "--reason", "r"}, "never a step"},
		{[]string{"run", "triage", "--recheck", "--files", "3"}, "needs an earlier triage"},
		{[]string{"run", "triage", "extra"}, "usage"},
		{[]string{"run", "step"}, "usage"},
		{[]string{"run", "step", "gate"}, "not one step"},
	}
	for _, c := range bad {
		if code := e.do(c.args...); code != 2 || !strings.Contains(e.err.String(), c.want) {
			t.Errorf("%v: code %d, err %q, want 2 and %q", c.args, code, e.err, c.want)
		}
	}
	for _, ev := range e.events() {
		if ev.Kind == run.KindTriage || ev.Kind == run.KindDecision {
			t.Errorf("a usage error recorded %+v", ev)
		}
	}
	// An untriaged run runs every step.
	if e.do("run", "step", "reflect") != 0 || strings.TrimSpace(e.out.String()) != "run" {
		t.Errorf("untriaged step = %q, want run", e.out)
	}
	e.do("run", "triage", "--kind", "bugfix", "--files", "2")
	if e.do("run", "triage", "--recheck") != 2 || !strings.Contains(e.err.String(), "needs --files") {
		t.Errorf("recheck without files: %q", e.err)
	}
}

func TestRunBriefJSONTriage(t *testing.T) {
	e := rcSetup(t, "")
	e.do("run", "start", "--title", "x")
	e.do("run", "triage", "--kind", "bugfix", "--files", "2")
	if e.do("run", "brief", "--json") != 0 {
		t.Fatalf("brief: %q", e.err)
	}
	var b briefJSON
	if err := json.Unmarshal(e.out.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	if b.Triage.Tier != run.TierSmall || !slices.Contains(b.Skipped, run.StepPlanReview) {
		t.Errorf("brief json triage = %+v skipped %v", b.Triage, b.Skipped)
	}
}
