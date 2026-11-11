package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

func TestRunAsk(t *testing.T) {
	e := rcSetup(t, "")
	e.app.Checkpoints = "pr"
	e.do("run", "start", "--title", "x")

	// No list: the config's checkpoints.
	if e.do("run", "ask") != 0 || e.out.String() != "checkpoints: product\n" {
		t.Fatalf("config: %q %q", e.out, e.err)
	}
	if last := e.events()[len(e.events())-1]; last.Kind != run.KindAsk || last.Note != "product" {
		t.Fatalf("event: %+v", last)
	}
	// A list given wins over the config, and items may be split across args.
	if e.do("run", "ask", "approve", "qa") != 0 || e.out.String() != "checkpoints: plan,qa\n" {
		t.Fatalf("explicit: %q %q", e.out, e.err)
	}
	if e.do("run", "ask", "none") != 0 || e.out.String() != "checkpoints: none\n" {
		t.Fatalf("none: %q", e.out)
	}
	if e.do("run", "ask", "plan,merge") != 2 || !strings.Contains(e.err.String(), "unknown checkpoint merge") {
		t.Fatalf("bad list: %q", e.err)
	}
	if last := e.events()[len(e.events())-1]; last.Note != "none" {
		t.Fatalf("a bad list must record nothing: %+v", last)
	}
}

func TestRunCheckpoint(t *testing.T) {
	e := rcSetup(t, "")
	if e.do("run", "checkpoint", "plan") != 1 || !strings.Contains(e.err.String(), "no active run") {
		t.Fatalf("no run: %q", e.err)
	}
	e.do("run", "start", "--title", "x")
	e.do("run", "ask", "plan,pr")
	for arg, want := range map[string]string{"plan": "ask", "approve": "ask", "pr": "ask", "product": "ask", "qa": "auto", "intake": "auto"} {
		if e.do("run", "checkpoint", arg) != 0 || strings.TrimSpace(e.out.String()) != want {
			t.Errorf("checkpoint %s = %q, want %s", arg, e.out, want)
		}
	}
	for _, bad := range [][]string{{"run", "checkpoint"}, {"run", "checkpoint", "all"}, {"run", "checkpoint", "nope"}, {"run", "checkpoint", "plan", "qa"}} {
		if e.do(bad...) != 2 {
			t.Errorf("%v must be a usage error", bad)
		}
	}
}

func TestRunDecide(t *testing.T) {
	e := rcSetup(t, "")
	e.do("run", "start", "--title", "x")
	e.do("run", "phase", "qa")
	if e.do("run", "decide", "--topic", "W1:C2", "--note", "moved to Open: not needed by any AC", "--ref", "qa/W1.r3.md") != 0 {
		t.Fatal(e.err)
	}
	d := e.events()[len(e.events())-1]
	if d.Kind != run.KindDecision || d.From != run.ByLead || d.Name != "W1:C2" || d.Ref != "qa/W1.r3.md" || d.Note == "" {
		t.Fatalf("%+v", d)
	}
	if e.do("run", "decide", "--topic", "scope", "--note", "CSV only", "--by", "user", "--phase", "intake") != 0 {
		t.Fatal(e.err)
	}
	if d := e.events()[len(e.events())-1]; d.From != run.ByUser || d.Phase != run.PhaseIntake {
		t.Fatalf("%+v", d)
	}
	for _, bad := range [][]string{
		{"run", "decide", "--note", "n"},
		{"run", "decide", "--topic", "t"},
		{"run", "decide", "--topic", " ", "--note", "n"},
		{"run", "decide", "--topic", "t", "--note", "n", "--by", "bot"},
		{"run", "decide", "--topic", "t", "--note", "n", "--phase", "merge"},
		{"run", "decide", "--topic", "t", "--note", "n", "extra"},
	} {
		if e.do(bad...) != 2 {
			t.Errorf("%v must be a usage error", bad)
		}
	}
}

func TestRunBriefAndPhase(t *testing.T) {
	e := rcSetup(t, "")
	if e.do("run", "brief") != 1 {
		t.Fatal("brief needs a run")
	}
	e.do("run", "start", "--title", "Add orders", "--ticket", "#9")
	e.do("run", "ask", "plan")
	// Entering a phase prints the brief.
	if e.do("run", "phase", "plan") != 0 {
		t.Fatal(e.err)
	}
	out := e.out.String()
	for _, want := range []string{"lead (master agent) of loops run", `"Add orders", ticket #9`, "charter.md", "Phase: plan (path intake → plan).", "Checkpoints: plan.", "Next: spawn the planner"} {
		if !strings.Contains(out, want) {
			t.Errorf("phase output lacks %q:\n%s", want, out)
		}
	}
	e.do("run", "verdict", "--name", "plan", "--status", "CHANGES", "--round", "1", "--ref", "plan-review.r1.md")
	e.do("run", "decide", "--topic", "scope", "--note", "CSV only")
	if e.do("run", "brief", "--json") != 0 {
		t.Fatal(e.err)
	}
	var b briefJSON
	if err := json.Unmarshal(e.out.Bytes(), &b); err != nil {
		t.Fatalf("%v: %s", err, e.out)
	}
	if b.Phase != run.PhasePlan || len(b.Ask) != 1 || len(b.Open) != 1 || b.Open[0].Name != "plan" ||
		len(b.Decisions) != 1 || !strings.HasSuffix(b.Charter, "/charter.md") || !strings.Contains(b.Next, "round 2 of 3") || b.Run == "" || b.Dir == "" {
		t.Fatalf("%+v", b)
	}
	if e.do("run", "brief", "extra") != 2 {
		t.Fatal("brief takes no argument")
	}
}
