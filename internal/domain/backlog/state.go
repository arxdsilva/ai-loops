package backlog

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// ErrEmpty means no issue is queued.
var ErrEmpty = errors.New("the queue is empty")

// Item is one issue the backlog knows, with its lane history.
type Item struct {
	Number   int    `json:"number"`
	Title    string `json:"title"`
	URL      string `json:"url,omitempty"`
	Priority int    `json:"priority"`
	Status   Status `json:"status"`
	Attempts int    `json:"attempts,omitempty"`
	Worktree string `json:"worktree,omitempty"`
	// Lanes counts the lanes ever started on the issue (attempts reset when
	// the human gives it back; this never does), so each lane gets its own
	// worktree; Past are the earlier lanes' worktrees not yet cleaned.
	Lanes  int      `json:"lanes,omitempty"`
	Past   []string `json:"past_worktrees,omitempty"`
	RunDir string   `json:"run_dir,omitempty"` // the lane's last loops run
	PR     string   `json:"pr,omitempty"`
	Note   string   `json:"note,omitempty"` // why it failed, was handed off or skipped
	// HandedOff is true once the forge took the hand-off label and assignee
	// (BACKLOG §6). Held is true once a sync after that saw the issue held
	// (not eligible). Only an issue seen held and then eligible again is
	// the human giving it back: a stale listing or a label that did not
	// stick never re-queues it.
	HandedOff bool `json:"handed_off,omitempty"`
	Held      bool `json:"held,omitempty"`
	// Commented is true once the hand-off comment is posted, so a hand-off
	// retried by a later sync never posts it twice.
	Commented bool      `json:"handoff_commented,omitempty"`
	Started   time.Time `json:"started,omitzero"`
	Updated   time.Time `json:"updated,omitzero"`
}

// Finding is an issue filed from a lane's finding (BACKLOG §5).
type Finding struct {
	Key   string `json:"key"` // FindingKey(title)
	Title string `json:"title"`
	URL   string `json:"url"`
	From  int    `json:"from,omitempty"` // the lane's issue
}

// Pending is a lane finding not filed yet (BACKLOG §5): it stays in the
// state until it is filed or found to be a duplicate.
type Pending struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	From  int    `json:"from,omitempty"` // the lane's issue
}

// State is .loops/backlog/state.json.
type State struct {
	Version  int       `json:"version"`
	Settings Settings  `json:"settings"`
	Items    []Item    `json:"items"`
	Findings []Finding `json:"findings,omitempty"`
	Pending  []Pending `json:"pending_findings,omitempty"`
	Synced   time.Time `json:"synced,omitzero"`
}

// AddPending records findings to file, skipping one already pending from
// the same issue.
func (s *State) AddPending(ps ...Pending) {
	for _, p := range ps {
		if !s.IsPending(p.Title, p.From) {
			s.Pending = append(s.Pending, p)
		}
	}
}

// IsPending says whether issue from's finding title is still to be filed.
func (s *State) IsPending(title string, from int) bool {
	return slices.ContainsFunc(s.Pending, func(p Pending) bool { return p.From == from && FindingKey(p.Title) == FindingKey(title) })
}

// DropPending removes issue from's finding title from the pending list.
func (s *State) DropPending(title string, from int) {
	s.Pending = slices.DeleteFunc(s.Pending, func(p Pending) bool { return p.From == from && FindingKey(p.Title) == FindingKey(title) })
}

// Item returns the item for issue n.
func (s *State) Item(n int) (*Item, bool) {
	for i := range s.Items {
		if s.Items[i].Number == n {
			return &s.Items[i], true
		}
	}
	return nil, false
}

// Count returns how many items have status st.
func (s *State) Count(st Status) int {
	n := 0
	for _, it := range s.Items {
		if it.Status == st {
			n++
		}
	}
	return n
}

// SyncResult says what Sync changed.
type SyncResult struct {
	Open, Added, Dropped, Queued int
	// Filed counts the pending findings this sync filed (or matched to an
	// open issue), and Handed the hand-offs it completed (filled by the use
	// case).
	Filed, Handed int
	// Gate is the open-PR cap after the sync (filled by the use case; a zero
	// Cap means no cap, or the forge's PRs could not be listed).
	Gate Gate `json:"gate"`
}

// Gate is the open-PR cap at a claim (BACKLOG §7).
type Gate struct {
	Open int `json:"open"` // the lanes' PRs still open, plus the running lanes
	Cap  int `json:"cap"`  // 0 = no cap
}

// Closed says whether starting another lane would pass the cap.
func (g Gate) Closed() bool { return g.Cap > 0 && g.Open >= g.Cap }

// Gate counts what the open-PR cap holds: every running lane (each may open
// a PR), and every item whose PR is among open (the forge's open PR URLs,
// listed at since) or was recorded after since, so a PR opened while the
// list was in flight still counts. The two are separate reservations: a
// re-queued issue whose earlier PR is still open counts twice while a new
// lane runs on it.
func (s *State) Gate(open []string, since time.Time) Gate {
	urls := map[string]bool{}
	for _, u := range open {
		urls[prKey(u)] = true
	}
	g := Gate{Cap: s.Settings.PRCap()}
	for _, it := range s.Items {
		if it.Status == Running {
			g.Open++
		}
		if it.PR != "" && (urls[prKey(it.PR)] || (it.Status == Done && it.Updated.After(since))) {
			g.Open++
		}
	}
	return g
}

// prKey identifies a PR by its number ("pull/12"), so a URL written with
// another host form or a trailing slash still matches the forge's listing.
func prKey(url string) string {
	u := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(url), "/"))
	if i := strings.LastIndex(u, "/pull/"); i >= 0 {
		if n, err := strconv.Atoi(u[i+len("/pull/"):]); err == nil {
			return "pull/" + strconv.Itoa(n)
		}
	}
	return u
}

// Sync rebuilds the queue from the open issues (BACKLOG §2): new eligible
// issues are queued, queued issues no longer eligible (or closed) are
// dropped, and every other status is kept, so no issue is reworked.
func (s *State) Sync(open []Issue, now time.Time) SyncResult {
	res := SyncResult{Open: len(open)}
	eligible := map[int]Issue{}
	for _, is := range open {
		if ok, _ := s.Settings.Eligible(is); ok {
			eligible[is.Number] = is
		}
	}
	kept := s.Items[:0]
	for _, it := range s.Items {
		if it.Status == Human && it.HandedOff {
			is, ok := eligible[it.Number]
			switch {
			case !ok:
				it.Held = true
			case it.Held:
				// The human unassigned it and removed the hand-off label:
				// it is ready for a lane again, with fresh attempts.
				it.Status, it.Attempts, it.Note, it.Updated = Queued, 0, "", now
				it.freshHandoff()
				it.Title, it.URL, it.Priority = is.Title, is.URL, Priority(is.Labels)
				res.Added++
			}
		}
		if it.Status == Queued {
			is, ok := eligible[it.Number]
			if !ok {
				res.Dropped++
				continue
			}
			it.Title, it.URL, it.Priority = is.Title, is.URL, Priority(is.Labels)
		}
		kept = append(kept, it)
	}
	s.Items = kept
	for _, is := range open {
		if _, ok := eligible[is.Number]; !ok {
			continue
		}
		if _, known := s.Item(is.Number); known {
			continue
		}
		s.Items = append(s.Items, Item{Number: is.Number, Title: is.Title, URL: is.URL,
			Priority: Priority(is.Labels), Status: Queued, Updated: now})
		res.Added++
	}
	s.Synced = now
	res.Queued = s.Count(Queued)
	return res
}

// Queue returns the queued items in lane order: priority, then number.
func (s *State) Queue() []Item {
	var q []Item
	for _, it := range s.Items {
		if it.Status == Queued {
			q = append(q, it)
		}
	}
	slices.SortFunc(q, func(a, b Item) int {
		if a.Priority != b.Priority {
			return a.Priority - b.Priority
		}
		return a.Number - b.Number
	})
	return q
}

// Claim marks issue n (0 = the first queued) running for a lane in the
// worktree dir(n, lane) names, lane counting from 1 per issue.
func (s *State) Claim(n int, dir func(n, lane int) string, now time.Time) (Item, error) {
	if n == 0 {
		q := s.Queue()
		if len(q) == 0 {
			return Item{}, ErrEmpty
		}
		n = q[0].Number
	}
	it, ok := s.Item(n)
	if !ok {
		return Item{}, fmt.Errorf("issue %d is not in the backlog (loops backlog sync)", n)
	}
	if it.Status != Queued {
		return Item{}, fmt.Errorf("issue %d is %s, not queued", n, it.Status)
	}
	if it.Worktree != "" {
		it.Past = append(it.Past, it.Worktree)
	}
	it.Lanes++
	it.Status, it.Worktree, it.Note = Running, dir(n, it.Lanes), ""
	it.Attempts++
	it.Started, it.Updated = now, now
	return *it, nil
}

// Outcome is how a lane ended.
type Outcome struct {
	PR     string // the PR the run opened ("" = none)
	RunDir string
	Reason string // why it failed ("" when a PR was opened)
	// NeedsHuman is set when the lane recorded a needs-human decision
	// (BACKLOG §4): retrying cannot help, so it is handed off at once.
	NeedsHuman bool
}

// Finish records a lane's outcome and applies the failure policy (BACKLOG
// §3 step 6): done with a PR; re-queued while attempts remain, unless the
// lane asked for a human; handed to the human otherwise. It returns the new status.
func (s *State) Finish(n int, o Outcome, now time.Time) (Status, error) {
	it, ok := s.Item(n)
	if !ok {
		return "", fmt.Errorf("issue %d is not in the backlog", n)
	}
	it.RunDir, it.Updated = o.RunDir, now
	switch {
	case o.PR != "":
		it.Status, it.PR, it.Note = Done, o.PR, ""
	case it.Attempts < s.Settings.Attempts && !o.NeedsHuman:
		it.Status, it.Note = Queued, o.Reason
	default:
		it.Status, it.Note = Human, o.Reason
		it.freshHandoff()
	}
	return it.Status, nil
}

// freshHandoff starts a new hand-off: nothing posted or confirmed yet.
func (it *Item) freshHandoff() { it.HandedOff, it.Held, it.Commented = false, false, false }

// Unconfirmed returns the issues handed to the human whose hand-off the
// forge has not confirmed yet (BACKLOG §6): sync retries them.
func (s *State) Unconfirmed() []Item {
	var out []Item
	for _, it := range s.Items {
		if it.Status == Human && !it.HandedOff {
			out = append(out, it)
		}
	}
	return out
}

// Set is the supervisor's ruling on issue n (`loops backlog finish`).
// Re-queueing an issue whose attempts are spent needs force.
func (s *State) Set(n int, st Status, note string, force bool, now time.Time) (Item, error) {
	it, ok := s.Item(n)
	if !ok {
		return Item{}, fmt.Errorf("issue %d is not in the backlog", n)
	}
	if st == Queued && it.Attempts >= s.Settings.Attempts && !force {
		return Item{}, fmt.Errorf("issue %d used %d of %d attempts; pass --force to queue it again", n, it.Attempts, s.Settings.Attempts)
	}
	it.Status, it.Updated = st, now
	if note != "" {
		it.Note = note
	}
	// A ruling ends any earlier hand-off; one to human starts a new one.
	it.freshHandoff()
	return *it, nil
}

// Stale returns the running items whose lane is gone: a restarted
// supervisor re-queues them (attempt kept) rather than leaving them stuck.
func (s *State) Stale(alive func(Item) bool) []int {
	var out []int
	for _, it := range s.Items {
		if it.Status == Running && !alive(it) {
			out = append(out, it.Number)
		}
	}
	return out
}

// FindingKey normalizes a title for duplicate detection: lower case, runs
// of other characters collapsed to one space.
func FindingKey(title string) string {
	var b strings.Builder
	space := false
	for _, r := range strings.ToLower(title) {
		if isWord(r) && r != '_' {
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteRune(r)
			space = false
			continue
		}
		space = true
	}
	return b.String()
}

// Duplicate returns the URL of an earlier finding or open issue with the
// same key as title, or "".
func (s *State) Duplicate(title string, open []Issue) string {
	key := FindingKey(title)
	if key == "" {
		return ""
	}
	for _, f := range s.Findings {
		if f.Key == key {
			return f.URL
		}
	}
	for _, is := range open {
		if FindingKey(is.Title) == key {
			return is.URL
		}
	}
	return ""
}
