// Package backlog is the queue of backlog mode (specs/BACKLOG.md): which
// open issues are eligible, in what order lanes take them, and how a lane's
// outcome moves an issue between statuses. Pure: no I/O, no wall clock.
package backlog

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"
)

// Defaults for the settings (BACKLOG §8).
const (
	DefaultLanes        = 2
	DefaultAttempts     = 2
	DefaultTimeout      = 4 * time.Hour
	DefaultFindingLabel = "loops-finding"
	DefaultHumanLabel   = "loops-needs-human"
	DefaultClaimLabel   = "loops-wip"
	MinLaneAgents       = 2
	// NoPriority sorts issues without a P<digit> label after P9.
	NoPriority = 10
)

// DefaultExclude are labels that keep an issue out of the queue besides
// the finding, hand-off and claim labels.
var DefaultExclude = []string{"wontfix", "duplicate", "invalid", "question"}

// Version is the state document's schema version.
const Version = 1

// Status is where an issue stands in the backlog (BACKLOG §2).
type Status string

// The statuses.
const (
	Queued  Status = "queued"  // eligible, waiting for a lane
	Running Status = "running" // a lane holds it
	Done    Status = "done"    // a lane opened its PR
	Failed  Status = "failed"  // a lane ended without a PR (may be re-queued)
	Human   Status = "human"   // handed to the human (BACKLOG §6)
	Skipped Status = "skipped" // set aside by the supervisor
)

// ParseStatus checks a status a user typed.
func ParseStatus(s string) (Status, error) {
	switch st := Status(s); st {
	case Queued, Running, Done, Failed, Human, Skipped:
		return st, nil
	}
	return "", fmt.Errorf("unknown status %q (queued, running, done, failed, human, skipped)", s)
}

// Settings are the backlog's knobs, set by `loops backlog init`.
type Settings struct {
	Repo         string   `json:"repo"`  // owner/name
	Human        string   `json:"human"` // forge login findings and hand-offs are assigned to
	Base         string   `json:"base"`  // branch lanes start from
	Lanes        int      `json:"lanes"`
	MaxAgents    int      `json:"max_agents"` // per lane
	Attempts     int      `json:"attempts"`
	Timeout      string   `json:"timeout"` // per lane, a Go duration
	Labels       []string `json:"labels,omitempty"`
	Exclude      []string `json:"exclude,omitempty"`
	Authors      []string `json:"authors,omitempty"` // empty = any author
	Agent        string   `json:"agent,omitempty"`   // lane command template ("" = default)
	Budget       string   `json:"budget,omitempty"`  // USD per lane ("" = none)
	Worktrees    string   `json:"worktrees"`
	FindingLabel string   `json:"finding_label"`
	HumanLabel   string   `json:"human_label"`
	ClaimLabel   string   `json:"claim_label"`
}

// Normalize fills defaults and checks the settings.
func (s Settings) Normalize() (Settings, error) {
	if s.Lanes <= 0 {
		s.Lanes = DefaultLanes
	}
	if s.Attempts <= 0 {
		s.Attempts = DefaultAttempts
	}
	if s.MaxAgents < MinLaneAgents {
		s.MaxAgents = MinLaneAgents
	}
	if s.Timeout == "" {
		s.Timeout = DefaultTimeout.String()
	}
	if s.FindingLabel == "" {
		s.FindingLabel = DefaultFindingLabel
	}
	if s.HumanLabel == "" {
		s.HumanLabel = DefaultHumanLabel
	}
	if s.ClaimLabel == "" {
		s.ClaimLabel = DefaultClaimLabel
	}
	switch {
	case s.Human == "":
		return s, errors.New("the human (--human <login>) is required: findings and hand-offs are assigned to them")
	case s.Repo == "" || !strings.Contains(s.Repo, "/"):
		return s, fmt.Errorf("repo %q is not owner/name", s.Repo)
	case s.Base == "":
		return s, errors.New("the base branch is required")
	case s.Worktrees == "":
		return s, errors.New("the worktrees directory is required")
	}
	if d, err := time.ParseDuration(s.Timeout); err != nil || d <= 0 {
		return s, fmt.Errorf("timeout %q is not a positive duration", s.Timeout)
	}
	return s, nil
}

// TimeoutDuration is the lane timeout (DefaultTimeout when unparsable).
func (s Settings) TimeoutDuration() time.Duration {
	if d, err := time.ParseDuration(s.Timeout); err == nil && d > 0 {
		return d
	}
	return DefaultTimeout
}

// LaneAgents splits the machine's agent cap between the lanes (BACKLOG §4).
func LaneAgents(machineCap, lanes int) int {
	if lanes <= 0 {
		lanes = 1
	}
	return max(MinLaneAgents, machineCap/lanes)
}

// Issue is an open issue as the forge lists it.
type Issue struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	URL       string   `json:"url"`
	Author    string   `json:"author"`
	Labels    []string `json:"labels"`
	Assignees []string `json:"assignees"`
}

// Eligible says whether an issue may be queued, and why not (BACKLOG §2).
func (s Settings) Eligible(is Issue) (bool, string) {
	if len(is.Assignees) > 0 {
		return false, "assigned to " + strings.Join(is.Assignees, ",")
	}
	if len(s.Authors) > 0 && !containsFold(s.Authors, is.Author) {
		return false, "author " + is.Author + " is not allowed"
	}
	if len(s.Labels) > 0 && !slices.ContainsFunc(is.Labels, func(l string) bool { return containsFold(s.Labels, l) }) {
		return false, "no wanted label"
	}
	excluded := append(slices.Clone(s.Exclude), s.FindingLabel, s.HumanLabel, s.ClaimLabel)
	for _, l := range is.Labels {
		if containsFold(excluded, l) {
			return false, "label " + l
		}
	}
	return true, ""
}

func containsFold(list []string, s string) bool {
	return slices.ContainsFunc(list, func(x string) bool { return x != "" && strings.EqualFold(x, s) })
}

// Priority is the issue's priority: the digit of its first label holding
// P<digit> as a word (P0 highest), else NoPriority.
func Priority(labels []string) int {
	for _, l := range labels {
		if p, ok := labelPriority(l); ok {
			return p
		}
	}
	return NoPriority
}

func labelPriority(label string) (int, bool) {
	r := []rune(label)
	for i := 0; i+1 < len(r); i++ {
		if r[i] != 'P' && r[i] != 'p' {
			continue
		}
		if !unicode.IsDigit(r[i+1]) || (i > 0 && isWord(r[i-1])) || (i+2 < len(r) && isWord(r[i+2])) {
			continue
		}
		return int(r[i+1] - '0'), true
	}
	return 0, false
}

func isWord(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }
