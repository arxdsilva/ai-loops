package backlog

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

var t0 = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func settings() Settings {
	s, err := Settings{Repo: "o/r", Human: "h", Base: "main", Worktrees: "/wt", Exclude: DefaultExclude}.Normalize()
	if err != nil {
		panic(err)
	}
	return s
}

func TestNormalize(t *testing.T) {
	s := settings()
	if s.Lanes != DefaultLanes || s.Attempts != DefaultAttempts || s.MaxAgents != MinLaneAgents ||
		s.TimeoutDuration() != DefaultTimeout || s.FindingLabel != DefaultFindingLabel ||
		s.HumanLabel != DefaultHumanLabel || s.ClaimLabel != DefaultClaimLabel {
		t.Fatalf("defaults not filled: %+v", s)
	}
	bad := []struct {
		name string
		s    Settings
		want string
	}{
		{"no human", Settings{Repo: "o/r", Base: "main", Worktrees: "/wt"}, "human"},
		{"bad repo", Settings{Repo: "r", Human: "h", Base: "main", Worktrees: "/wt"}, "owner/name"},
		{"no base", Settings{Repo: "o/r", Human: "h", Worktrees: "/wt"}, "base"},
		{"no worktrees", Settings{Repo: "o/r", Human: "h", Base: "main"}, "worktrees"},
		{"bad timeout", Settings{Repo: "o/r", Human: "h", Base: "main", Worktrees: "/wt", Timeout: "soon"}, "timeout"},
		{"negative timeout", Settings{Repo: "o/r", Human: "h", Base: "main", Worktrees: "/wt", Timeout: "-1h"}, "timeout"},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.s.Normalize(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
	if (Settings{Timeout: "bogus"}).TimeoutDuration() != DefaultTimeout {
		t.Fatal("unparsable timeout should fall back to the default")
	}
}

func TestParseStatus(t *testing.T) {
	for _, s := range []string{"queued", "running", "done", "failed", "human", "skipped"} {
		if got, err := ParseStatus(s); err != nil || string(got) != s {
			t.Fatalf("ParseStatus(%q) = %q, %v", s, got, err)
		}
	}
	if _, err := ParseStatus("merged"); err == nil {
		t.Fatal("unknown status accepted")
	}
}

func TestLaneAgents(t *testing.T) {
	cases := []struct{ capacity, lanes, want int }{
		{16, 2, 8}, {16, 0, 16}, {3, 2, MinLaneAgents}, {0, 4, MinLaneAgents},
	}
	for _, c := range cases {
		if got := LaneAgents(c.capacity, c.lanes); got != c.want {
			t.Errorf("LaneAgents(%d,%d) = %d, want %d", c.capacity, c.lanes, got, c.want)
		}
	}
}

func TestPriority(t *testing.T) {
	cases := []struct {
		labels []string
		want   int
	}{
		{[]string{"priority: P2"}, 2},
		{[]string{"bug", "p0"}, 0},
		{[]string{"P1-high"}, 1},
		{[]string{"MP3", "P12", "help"}, NoPriority},
		{[]string{"spec-backlog", "priority: P3"}, 3},
		{nil, NoPriority},
	}
	for _, c := range cases {
		if got := Priority(c.labels); got != c.want {
			t.Errorf("Priority(%v) = %d, want %d", c.labels, got, c.want)
		}
	}
}

func TestEligible(t *testing.T) {
	s := settings()
	s.Authors = []string{"Alice"}
	s.Labels = []string{"bug", "enhancement"}
	ok := Issue{Number: 1, Author: "alice", Labels: []string{"BUG"}}
	cases := []struct {
		name string
		is   Issue
		want string // "" = eligible
	}{
		{"eligible", ok, ""},
		{"assigned", Issue{Author: "alice", Labels: []string{"bug"}, Assignees: []string{"bob"}}, "assigned"},
		{"other author", Issue{Author: "mallory", Labels: []string{"bug"}}, "author"},
		{"no wanted label", Issue{Author: "alice", Labels: []string{"docs"}}, "wanted label"},
		{"excluded", Issue{Author: "alice", Labels: []string{"bug", "wontfix"}}, "wontfix"},
		{"finding", Issue{Author: "alice", Labels: []string{"bug", DefaultFindingLabel}}, DefaultFindingLabel},
		{"handed off", Issue{Author: "alice", Labels: []string{"bug", DefaultHumanLabel}}, DefaultHumanLabel},
		{"claimed", Issue{Author: "alice", Labels: []string{"bug", DefaultClaimLabel}}, DefaultClaimLabel},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, why := s.Eligible(tc.is)
			if tc.want == "" {
				if !got {
					t.Fatalf("not eligible: %s", why)
				}
				return
			}
			if got || !strings.Contains(why, tc.want) {
				t.Fatalf("Eligible = %v %q, want false mentioning %q", got, why, tc.want)
			}
		})
	}
	any := settings()
	if ok, why := any.Eligible(Issue{Author: "anyone"}); !ok {
		t.Fatalf("no filters should admit any author: %s", why)
	}
}

func issue(n int, labels ...string) Issue {
	return Issue{Number: n, Title: "issue " + string(rune('A'+n%26)), URL: "u" + string(rune('0'+n%10)), Labels: labels}
}

func numbers(items []Item) []int {
	var out []int
	for _, it := range items {
		out = append(out, it.Number)
	}
	return out
}

func TestSyncAndQueueOrder(t *testing.T) {
	st := State{Settings: settings()}
	res := st.Sync([]Issue{issue(9), issue(3, "priority: P2"), issue(5, "priority: P0"), issue(4, "priority: P2"), issue(7, "wontfix")}, t0)
	if res.Open != 5 || res.Added != 4 || res.Queued != 4 || res.Dropped != 0 {
		t.Fatalf("first sync = %+v", res)
	}
	if got := numbers(st.Queue()); !slices.Equal(got, []int{5, 3, 4, 9}) {
		t.Fatalf("queue order = %v", got)
	}
	if !st.Synced.Equal(t0) {
		t.Fatal("synced time not recorded")
	}

	// 3 is worked to a PR, 4 handed off, 9 closes upstream, 5 gets assigned.
	st.Items[1].Status = Done
	it, _ := st.Item(4)
	it.Status = Human
	assigned := issue(5, "priority: P0")
	assigned.Assignees = []string{"someone"}
	res = st.Sync([]Issue{assigned, issue(3, "priority: P2"), issue(4, "priority: P2", DefaultHumanLabel), issue(11, "priority: P1")}, t0.Add(time.Hour))
	if res.Dropped != 2 || res.Added != 1 || res.Queued != 1 {
		t.Fatalf("second sync = %+v", res)
	}
	if it, _ := st.Item(3); it.Status != Done {
		t.Fatal("a done issue must never be re-queued by sync")
	}
	if it, _ := st.Item(4); it.Status != Human {
		t.Fatal("a handed-off issue still carrying the label must stay with the human")
	}
	if got := numbers(st.Queue()); !slices.Equal(got, []int{11}) {
		t.Fatalf("queue = %v", got)
	}

	// The human clears the hand-off: 4 returns with fresh attempts.
	it, _ = st.Item(4)
	it.Attempts, it.Note = 2, "stuck"
	st.Sync([]Issue{issue(4, "priority: P2"), issue(11, "priority: P1")}, t0.Add(time.Hour))
	if it, _ := st.Item(4); it.Status != Human {
		t.Fatal("a hand-off the forge never confirmed must not come back on its own")
	}
	it, _ = st.Item(4)
	it.HandedOff = true
	// A listing that still shows it eligible right after the hand-off is
	// stale (or the label did not stick): it stays with the human.
	st.Sync([]Issue{issue(4, "priority: P2"), issue(11, "priority: P1")}, t0.Add(2*time.Hour))
	if it, _ := st.Item(4); it.Status != Human {
		t.Fatal("re-queued without ever being seen held")
	}
	// Seen held, then released: back to the queue with fresh attempts.
	st.Sync([]Issue{issue(4, "priority: P2", DefaultHumanLabel), issue(11, "priority: P1")}, t0.Add(3*time.Hour))
	res = st.Sync([]Issue{issue(4, "priority: P2"), issue(11, "priority: P1")}, t0.Add(4*time.Hour))
	it, _ = st.Item(4)
	if res.Added != 1 || it.Status != Queued || it.Attempts != 0 || it.Note != "" || it.HandedOff || it.Held {
		t.Fatalf("hand-off not cleared: %+v %+v", res, it)
	}
}

func TestClaim(t *testing.T) {
	st := State{Settings: settings()}
	if _, err := st.Claim(0, "/wt/x", t0); err != ErrEmpty {
		t.Fatalf("empty queue: %v", err)
	}
	st.Sync([]Issue{issue(2, "P3"), issue(8, "P1")}, t0)
	it, err := st.Claim(0, "/wt/issue-8", t0)
	if err != nil || it.Number != 8 || it.Status != Running || it.Attempts != 1 || it.Worktree != "/wt/issue-8" {
		t.Fatalf("claim next = %+v, %v", it, err)
	}
	if _, err := st.Claim(8, "/wt", t0); err == nil || !strings.Contains(err.Error(), "running") {
		t.Fatalf("claiming a running issue: %v", err)
	}
	if _, err := st.Claim(99, "/wt", t0); err == nil || !strings.Contains(err.Error(), "sync") {
		t.Fatalf("claiming an unknown issue: %v", err)
	}
	if it, err := st.Claim(2, "/wt/issue-2", t0); err != nil || it.Number != 2 {
		t.Fatalf("claim by number = %+v, %v", it, err)
	}
}

func TestFinishPolicy(t *testing.T) {
	st := State{Settings: settings()}
	st.Sync([]Issue{issue(1), issue(2)}, t0)
	if _, err := st.Finish(42, Outcome{}, t0); err == nil {
		t.Fatal("finishing an unknown issue")
	}
	st.Claim(1, "/wt/1", t0)
	if s, _ := st.Finish(1, Outcome{PR: "https://pr/1", RunDir: "/r"}, t0); s != Done {
		t.Fatalf("PR outcome = %s", s)
	}
	if it, _ := st.Item(1); it.PR != "https://pr/1" || it.RunDir != "/r" {
		t.Fatalf("PR not recorded: %+v", it)
	}

	st.Claim(2, "/wt/2", t0)
	if s, _ := st.Finish(2, Outcome{Reason: "the run ended failed"}, t0); s != Queued {
		t.Fatalf("first failure should re-queue, got %s", s)
	}
	st.Claim(2, "/wt/2", t0)
	if s, _ := st.Finish(2, Outcome{Reason: "again"}, t0); s != Human {
		t.Fatalf("out of attempts should hand off, got %s", s)
	}
	if it, _ := st.Item(2); it.Note != "again" || it.Attempts != 2 {
		t.Fatalf("hand-off note/attempts: %+v", it)
	}
}

func TestSet(t *testing.T) {
	st := State{Settings: settings()}
	st.Sync([]Issue{issue(1)}, t0)
	st.Claim(1, "/wt", t0)
	st.Finish(1, Outcome{Reason: "x"}, t0)
	st.Claim(1, "/wt", t0)
	st.Finish(1, Outcome{Reason: "y"}, t0)
	if _, err := st.Set(1, Queued, "", false, t0); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("re-queue without force: %v", err)
	}
	it, err := st.Set(1, Queued, "try again", true, t0)
	if err != nil || it.Status != Queued || it.Note != "try again" {
		t.Fatalf("forced re-queue = %+v, %v", it, err)
	}
	if it, _ := st.Set(1, Skipped, "", false, t0); it.Note != "try again" {
		t.Fatal("an empty note must keep the previous one")
	}
	if _, err := st.Set(5, Done, "", false, t0); err == nil {
		t.Fatal("unknown issue accepted")
	}
}

func TestStale(t *testing.T) {
	st := State{Items: []Item{{Number: 1, Status: Running}, {Number: 2, Status: Running}, {Number: 3, Status: Queued}}}
	got := st.Stale(func(it Item) bool { return it.Number == 2 })
	if !slices.Equal(got, []int{1}) {
		t.Fatalf("stale = %v", got)
	}
}

func TestFindingKeyAndDuplicate(t *testing.T) {
	if got := FindingKey("  Fix: the `Foo` bar -- NOW!! "); got != "fix the foo bar now" {
		t.Fatalf("key = %q", got)
	}
	if FindingKey("snake_case") != "snake case" {
		t.Fatal("underscore should separate words")
	}
	st := State{Findings: []Finding{{Key: FindingKey("Leak in pool"), URL: "f1"}}}
	if got := st.Duplicate("leak IN pool.", nil); got != "f1" {
		t.Fatalf("finding duplicate = %q", got)
	}
	if got := st.Duplicate("Race in cache", []Issue{{Title: "race in  cache", URL: "i9"}}); got != "i9" {
		t.Fatalf("open-issue duplicate = %q", got)
	}
	if st.Duplicate("brand new", nil) != "" || st.Duplicate("!!!", nil) != "" {
		t.Fatal("false duplicate")
	}
}

func ev(kind run.Kind, mod func(*run.Event)) run.Event {
	e := run.Event{Time: t0, Kind: kind}
	if mod != nil {
		mod(&e)
	}
	return e
}

func TestLaneOutcome(t *testing.T) {
	start := ev(run.KindRun, func(e *run.Event) { e.Run = "r1"; e.Name = "t" })
	decide := ev(run.KindDecision, func(e *run.Event) { e.Name = "needs-human"; e.Note = "no spec for X" })
	pr := ev(run.KindPR, func(e *run.Event) { e.Ref = "https://pr/7" })
	end := func(s string) run.Event { return ev(run.KindEnd, func(e *run.Event) { e.Status = s }) }
	cases := []struct {
		name     string
		lane     LaneRun
		agentErr string
		pr       string
		reason   []string
	}{
		{"no run", LaneRun{}, "exit status 1", "", []string{"no loops run started", "exit status 1"}},
		{"not ended", LaneRun{Dir: "/r", Events: []run.Event{start}}, "timeout", "", []string{"stopped before ending", "timeout"}},
		{"done with PR", LaneRun{Dir: "/r", Events: []run.Event{start, pr, end("done")}}, "", "https://pr/7", nil},
		{"done no PR", LaneRun{Dir: "/r", Events: []run.Event{start, end("done")}}, "", "", []string{"without opening a PR"}},
		{"abandoned", LaneRun{Dir: "/r", Events: []run.Event{start, decide, end("abandoned")}}, "", "", []string{"ended abandoned", "needs-human: no spec for X"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := tc.lane.Outcome(tc.agentErr)
			if o.PR != tc.pr || o.RunDir != tc.lane.Dir {
				t.Fatalf("outcome = %+v", o)
			}
			for _, w := range tc.reason {
				if !strings.Contains(o.Reason, w) {
					t.Fatalf("reason %q lacks %q", o.Reason, w)
				}
			}
			if tc.reason == nil && o.Reason != "" {
				t.Fatalf("unexpected reason %q", o.Reason)
			}
		})
	}
}

func TestLaneFindings(t *testing.T) {
	lane := LaneRun{
		Events: []run.Event{
			ev(run.KindNote, func(e *run.Event) { e.Name = "finding"; e.Note = "Cache leaks on reload"; e.Ref = "findings/1.md" }),
			ev(run.KindNote, func(e *run.Event) { e.Name = "cleanup"; e.Note = "ignored" }),
			ev(run.KindNote, func(e *run.Event) { e.Name = "finding"; e.Note = "  " }),
		},
		PRBody: "# T\n\n## Open follow-ups\n- cache leaks on reload.\n- Parser drops BOM\n  continued detail\n* none\n- \n\n## Proof\n- not a finding\n",
	}
	got := lane.Findings()
	want := []LaneFinding{{Title: "Cache leaks on reload", Ref: "findings/1.md"}, {Title: "Parser drops BOM"}}
	if !slices.Equal(got, want) {
		t.Fatalf("findings = %+v", got)
	}
	if FollowUps("## Open follow-ups\n- none\n") != nil || FollowUps("") != nil {
		t.Fatal("none must yield nothing")
	}
}
