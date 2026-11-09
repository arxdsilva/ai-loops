package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

func watchState() run.State {
	s := fixtureState()
	s.Recent = []run.StepInfo{}
	for i := 0; i < 10; i++ {
		s.Recent = append(s.Recent, run.StepInfo{Role: "adapter-engineer", Tool: "Edit", Target: "internal/x" + string(rune('a'+i)) + ".go", OK: i != 9})
	}
	s.Verdicts = []run.VerdictInfo{
		{Name: "AC1", Status: run.StatusFailed, Phase: run.PhaseProduct},
		{Name: "AC1", Status: run.StatusVerified, Phase: run.PhaseProduct},
		{Name: "AC2", Status: run.StatusFailed, Phase: run.PhaseProduct},
		{Name: "W1-domain:C1", Status: run.StatusVerified, Phase: run.PhaseQA},
	}
	return s
}

func TestWatchOnceFrame(t *testing.T) {
	a, out, errb := newApp(&fakeTracker{st: watchState()}, "")
	if code := a.Run(context.Background(), []string{"watch", "--once"}); code != 0 || errb.Len() != 0 {
		t.Fatalf("exit %d %q", code, errb)
	}
	frame := out.String()
	if strings.Contains(frame, "\x1b") {
		t.Error("ANSI emitted for a non-TTY")
	}
	for _, want := range []string{"GH-42", "▶ running", "1h05m", "◉ QA ↺1/3", "Plan r2/3", "● Intake", "○ Reflect",
		"domain-engineer", "W1-domain", "✓ verified r1", "✗ failed r1", "running", "build✓", "vet✗", "AC1✓", "AC2✗",
		"87 total, 0 errors", "internal/xj.go", "internal/xc.go"} {
		if !strings.Contains(frame, want) {
			t.Errorf("missing %q\n%s", want, frame)
		}
	}
	if strings.Contains(frame, "internal/xb.go") {
		t.Error("more than 8 steps shown")
	}
	t.Log("\n" + frame)
}

func TestWatchOnceIdle(t *testing.T) {
	a, out, _ := newApp(&fakeTracker{none: true}, "")
	if code := a.Run(context.Background(), []string{"watch", "--once"}); code != 0 || !strings.Contains(out.String(), "no active run") {
		t.Errorf("%d %q", code, out)
	}
}

func TestWatchColorOnTTYRender(t *testing.T) {
	f := renderWatch(watchState(), true, true)
	if !strings.Contains(f, ansiGreen) || !strings.Contains(f, ansiRed) {
		t.Error("expected colour")
	}
	if plain := renderWatch(watchState(), true, false); stripANSI(f) != plain {
		t.Errorf("colour and plain frames differ in content:\n%q\n%q", stripANSI(f), plain)
	}
}

func TestWatchExitsWhenRunEnds(t *testing.T) {
	s := watchState()
	s.Ended, s.Outcome = true, "done"
	a, out, _ := newApp(&fakeTracker{st: s}, "")
	done := make(chan int, 1)
	go func() { done <- a.Run(context.Background(), []string{"watch", "--interval", "10ms"}) }()
	select {
	case code := <-done:
		if code != 0 || !strings.Contains(out.String(), "loops report") || !strings.Contains(out.String(), "■ done") {
			t.Errorf("%d %q", code, out)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("watch did not exit on ended run")
	}
}

func TestWatchStopsOnCancelAndRejectsBadFlags(t *testing.T) {
	a, _, _ := newApp(&fakeTracker{st: watchState()}, "")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- a.Run(ctx, []string{"watch", "--interval", "10ms"}) }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case code := <-done:
		if code != 0 {
			t.Errorf("exit %d", code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("watch ignored cancellation")
	}
	for _, args := range [][]string{{"watch", "--interval", "0s"}, {"watch", "--bogus"}, {"watch", "x"}} {
		a, _, errb := newApp(&fakeTracker{}, "")
		if code := a.Run(context.Background(), args); code != 2 || errb.Len() == 0 {
			t.Errorf("%v: %d", args, code)
		}
	}
}

func TestWatchOverridesAndLimits(t *testing.T) {
	s := limitsState()
	s.Budgets = []run.BudgetInfo{{Phase: run.PhaseQA, Limit: "tokens", Reason: "qa tokens 2M reached", At: s.Started.Add(40 * time.Minute)}}
	frame := renderWatch(s, true, false)
	for _, want := range []string{"Overrides 2", "last: commit → AGENTS.md#git", "Limits", "QA", "1.2M/2M tok", "TOKENS", "12k", "⏸ budget"} {
		if !strings.Contains(frame, want) {
			t.Errorf("missing %q\n%s", want, frame)
		}
	}
	t.Log("\n" + frame)
}

func TestWatchNoLimitsNoExtras(t *testing.T) {
	frame := renderWatch(watchState(), true, false)
	for _, bad := range []string{"Overrides", "Limits", "TOKENS", "⏸"} {
		if strings.Contains(frame, bad) {
			t.Errorf("unexpected %q\n%s", bad, frame)
		}
	}
}
