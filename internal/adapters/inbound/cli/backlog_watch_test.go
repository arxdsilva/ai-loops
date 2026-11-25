package cli

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/arxdsilva/ai-loops/internal/domain/backlog"
	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

func TestBacklogWatchOnce(t *testing.T) {
	now := time.Now()
	f := &fakeBacklog{board: backlog.Board{Settings: testSettings, Synced: now.Add(-3 * time.Minute),
		Counts: map[backlog.Status]int{backlog.Queued: 4, backlog.Running: 3, backlog.Done: 1, backlog.Human: 1},
		PRs:    2, Findings: 1, Pending: 1,
		Running: []backlog.LaneView{
			{Item: backlog.Item{Number: 7, Title: "Fix login", Attempts: 1, Started: now.Add(-25 * time.Minute)}, Alive: true,
				Run: &backlog.Progress{Phase: run.PhaseImplement, Agents: 5, Busy: 3, Steps: 142, Errors: 2, Gates: 3, GatesOK: 2,
					Rounds: 1, Updated: now.Add(-10 * time.Second), Last: "go-engineer Edit x.go"}},
			{Item: backlog.Item{Number: 8, Title: "Slow one", Attempts: 2}, Alive: true,
				Run: &backlog.Progress{Phase: run.PhaseQA, Updated: now.Add(-time.Hour), Last: "lead Bash go test"}},
			{Item: backlog.Item{Number: 9, Title: "Ghost", Attempts: 1}},
		},
		Next: []backlog.Item{{Number: 3, Title: "Next one", Priority: 1}},
		Recent: []backlog.Item{
			{Number: 5, Title: "Shipped", Status: backlog.Done, PR: "https://x/pull/5"},
			{Number: 6, Title: "Stuck", Status: backlog.Human, Note: "needs a product decision"},
		}}}
	a, out, errw := backlogApp(f)
	if code := a.Run(context.Background(), []string{"backlog", "watch", "--once"}); code != 0 {
		t.Fatalf("exit %d: %s", code, errw.String())
	}
	got := out.String()
	for _, want := range []string{
		"backlog o/r · base main · human @hum", "synced 3m ago",
		"queued 4 · running 3 · done 1 · failed 0 · human 1 · skipped 0",
		"lanes 3/2 × 4 agents · lane PRs 2 (≤5 open) · findings 1 filed, 1 pending",
		"▶ #7 Fix login  attempt 1/2 · 25m",
		"●●●◉○○○  implement · agents 3/5 · steps 142!2 · gates 2/3 · ↺1",
		"last: go-engineer Edit x.go",
		"quiet for 1h00m: last lead Bash go test",
		"#9 Ghost", "lane process gone", "no live loops run in the worktree (starting, or wrapping up)",
		"next (4 queued)", "#3     P1  Next one",
		"✓ #5     Shipped  https://x/pull/5", "⚑ #6     Stuck  (human: needs a product decision)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("board lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "\x1b[") {
		t.Fatal("colour codes on a non-terminal")
	}

	f.err = errors.New("no backlog here")
	a, _, errw = backlogApp(f)
	if code := a.Run(context.Background(), []string{"backlog", "watch", "--once"}); code != 1 || !strings.Contains(errw.String(), "no backlog here") {
		t.Fatalf("missing state: exit %d, %q", code, errw.String())
	}
}

func TestRenderBoardEmpty(t *testing.T) {
	got := renderBoard(backlog.Board{Settings: backlog.Settings{Repo: "o/r", Lanes: 2, MaxAgents: 3, MaxPRs: -1}, Counts: map[backlog.Status]int{}}, time.Now(), true)
	for _, want := range []string{"never synced", "no lanes running", "lane PRs 0 · findings 0 filed"} {
		if !strings.Contains(got, want) {
			t.Errorf("empty board lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "next (") || strings.Contains(got, "recent") {
		t.Errorf("empty board lists issues:\n%s", got)
	}
}

func TestAgo(t *testing.T) {
	for d, want := range map[time.Duration]string{-time.Second: "0s", 45 * time.Second: "45s", 12 * time.Minute: "12m", 185 * time.Minute: "3h05m"} {
		if got := ago(d); got != want {
			t.Errorf("ago(%v) = %q, want %q", d, got, want)
		}
	}
}
