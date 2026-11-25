package backlog

import (
	"testing"
	"time"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

func TestNewProgress(t *testing.T) {
	s := run.State{Phase: run.PhaseProduct, Steps: 40, Errors: 2, PlanRounds: 2, PR: "https://x/pull/3",
		Lanes:     []run.Lane{{Agent: "a", Running: true}, {Agent: "b"}, {Agent: "c", Running: true}},
		Gates:     []run.GateResult{{Name: "build", OK: true}, {Name: "test", OK: false}},
		LoopBacks: []run.LoopBack{{Phase: run.PhaseQA}},
		Verdicts: []run.VerdictInfo{
			{Name: "AC1", Status: run.StatusFailed, Phase: run.PhaseProduct},
			{Name: "AC1", Status: run.StatusVerified, Phase: run.PhaseProduct}, // the latest wins
			{Name: "AC2", Status: run.StatusFailed, Phase: run.PhaseProduct},
			{Name: "W1:C1", Status: run.StatusVerified, Phase: run.PhaseQA}, // a QA claim, not an AC
		},
		Recent: []run.StepInfo{{Tool: "Bash", Target: "go test"}, {Role: "go-engineer", Tool: "Edit", Target: "x.go"}},
	}
	p := NewProgress(s)
	want := Progress{Phase: run.PhaseProduct, PR: "https://x/pull/3", Steps: 40, Errors: 2, Agents: 3, Busy: 2,
		GatesOK: 1, Gates: 2, ACsOK: 1, ACs: 2, Rounds: 2, Last: "go-engineer Edit x.go"}
	if p != want {
		t.Fatalf("progress =\n%+v\nwant\n%+v", p, want)
	}
	if got := NewProgress(run.State{Recent: []run.StepInfo{{Tool: "Read"}}}).Last; got != "lead Read" {
		t.Fatalf("lead step = %q", got)
	}
}

func TestNewBoard(t *testing.T) {
	t0 := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	st := State{Settings: Settings{Repo: "o/r"}, Synced: t0,
		Findings: []Finding{{Key: "a"}}, Pending: []Pending{{Title: "b"}, {Title: "c"}},
		Items: []Item{
			{Number: 1, Status: Queued, Priority: 2},
			{Number: 2, Status: Queued, Priority: 0},
			{Number: 3, Status: Done, PR: "https://x/pull/1", Updated: t0.Add(time.Minute)},
			{Number: 4, Status: Failed, LanePRs: []LanePR{{URL: "https://x/pull/2"}}, Updated: t0.Add(3 * time.Minute)},
			{Number: 5, Status: Human, Updated: t0.Add(2 * time.Minute)},
			{Number: 6, Status: Running},
		}}
	lanes := []LaneView{{Item: st.Items[5], Alive: true}}
	b := NewBoard(st, lanes)
	if b.Counts[Queued] != 2 || b.Counts[Running] != 1 || b.Counts[Done] != 1 || b.PRs != 2 || b.Findings != 1 || b.Pending != 2 {
		t.Fatalf("board = %+v", b)
	}
	if len(b.Next) != 2 || b.Next[0].Number != 2 || b.Next[1].Number != 1 {
		t.Fatalf("next = %+v", b.Next)
	}
	var recent []int
	for _, it := range b.Recent {
		recent = append(recent, it.Number)
	}
	if len(recent) != 3 || recent[0] != 4 || recent[1] != 5 || recent[2] != 3 {
		t.Fatalf("recent = %v, want newest first [4 5 3]", recent)
	}
	if len(b.Running) != 1 || !b.Running[0].Alive || !b.Synced.Equal(t0) {
		t.Fatalf("running = %+v", b.Running)
	}

	var many State
	for n := 1; n <= RecentShown+3; n++ {
		many.Items = append(many.Items, Item{Number: n, Status: Done}, Item{Number: 100 + n, Status: Queued})
	}
	if b := NewBoard(many, nil); len(b.Recent) != RecentShown || len(b.Next) != RecentShown || b.Recent[0].Number != RecentShown+3 {
		t.Fatalf("capped board: recent %d, next %d, first %d", len(b.Recent), len(b.Next), b.Recent[0].Number)
	}
}
