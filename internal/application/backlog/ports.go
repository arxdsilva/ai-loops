// Package backlog holds the use cases of backlog mode (specs/BACKLOG.md):
// init, sync, a lane from claim to outcome, findings, hand-offs, cleanup
// and the agent-less supervisor loop. It reaches the forge, git, the state
// file and the lane agent only through the ports below.
package backlog

import (
	"context"
	"time"

	"github.com/arxdsilva/ai-loops/internal/domain/backlog"
)

// Store keeps the backlog state. Update runs fn under an exclusive lock
// (across processes: lanes run as separate processes) and saves the state
// when fn returns nil.
type Store interface {
	Exists() bool
	Load(ctx context.Context) (backlog.State, error)
	Update(ctx context.Context, fn func(*backlog.State) error) error
	// LogPath is the lane log of issue n.
	LogPath(n int) string
	// HoldLane takes issue n's lane lock for as long as the lane runs (the
	// lock dies with the last process holding it); LaneHeld reports whether
	// a live process holds it.
	HoldLane(n int) (Lease, error)
	LaneHeld(n int) bool
}

// Lease is a held lane lock. Release frees it, even when a process it was
// handed to still runs. Work hands the lane's lease to its agent (Agent.Run),
// so a lane whose `loops backlog work` was killed outright still looks alive
// until the agent and everything it started have ended (BACKLOG §3).
type Lease interface{ Release() }

// NewIssue is an issue to create.
type NewIssue struct {
	Title, Body string
	Labels      []string
	Assignees   []string
}

// Edit changes an issue's labels and assignees.
type Edit struct {
	AddLabels, RemoveLabels       []string
	AddAssignees, RemoveAssignees []string
}

// Forge is the issue tracker (GitHub through gh).
type Forge interface {
	Repo(ctx context.Context) (string, error) // owner/name of the current checkout
	DefaultBranch(ctx context.Context, repo string) (string, error)
	User(ctx context.Context) (string, error) // the authenticated login
	OpenIssues(ctx context.Context, repo string) ([]backlog.Issue, error)
	Create(ctx context.Context, repo string, in NewIssue) (string, error) // returns the URL
	Comment(ctx context.Context, repo string, n int, body string) error
	Edit(ctx context.Context, repo string, n int, e Edit) error
	// EnsureLabel creates the label, or updates it when it exists.
	EnsureLabel(ctx context.Context, repo, name, description string) error
}

// PullRequests lists the forge's open pull requests, for the open-PR cap
// (BACKLOG §7).
type PullRequests interface {
	OpenPRs(ctx context.Context, repo string) ([]string, error) // the open PRs' URLs
}

// Worktrees manages the lanes' git worktrees.
type Worktrees interface {
	// Add fetches base from origin and adds a detached worktree at dir on
	// origin/<base>, replacing a clean leftover at dir.
	Add(ctx context.Context, dir, base string) error
	// Remove removes the worktree at dir (refusing a dirty one) and prunes.
	Remove(ctx context.Context, dir string) error
	Exists(dir string) bool
}

// Agent runs a lane's agent command in dir until it exits or ctx ends,
// writing its output to logPath. The agent's processes inherit lease when
// the adapter can pass it on (nil = nothing to inherit).
type Agent interface {
	Run(ctx context.Context, dir, prompt, logPath string, s backlog.Settings, lease Lease) error
}

// Lanes reads what a lane left in its worktree.
type Lanes interface {
	Latest(worktree string) (backlog.LaneRun, error) // newest run ("" Dir = none)
	Phase(worktree string) string                    // the live run's phase ("" = none)
	ReadRef(runDir, rel string) (string, bool)       // a file inside the run dir
}

// Clock is the wall clock.
type Clock interface {
	Now() time.Time
	// Sleep waits d, or until ctx ends (returning its error).
	Sleep(ctx context.Context, d time.Duration) error
}
