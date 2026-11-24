// Package lanefs reads what a backlog lane left in its worktree
// (specs/BACKLOG.md §3 step 4): its newest loops run, the live phase from
// .loops/status.json, and files inside the run directory.
package lanefs

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/runstore"
	"github.com/arxdsilva/ai-loops/internal/domain/backlog"
	"github.com/arxdsilva/ai-loops/internal/domain/run"
	"github.com/arxdsilva/ai-loops/internal/platform/loopsdir"
)

// Lanes reads lane worktrees from disk.
type Lanes struct{}

// Latest returns the worktree's newest run, its events and its pr.md.
func (Lanes) Latest(worktree string) (backlog.LaneRun, error) {
	if worktree == "" {
		return backlog.LaneRun{}, nil
	}
	dir, err := runstore.LatestRun(worktree)
	if err != nil || dir == "" {
		return backlog.LaneRun{}, err
	}
	events, err := runstore.EventsIn(dir)
	if err != nil {
		return backlog.LaneRun{Dir: dir}, err
	}
	pr, _ := os.ReadFile(filepath.Join(dir, "pr.md"))
	return backlog.LaneRun{Dir: dir, Events: events, PRBody: string(pr)}, nil
}

// Phase is the phase of the worktree's active run ("" = none).
func (Lanes) Phase(worktree string) string {
	b, err := os.ReadFile(filepath.Join(worktree, loopsdir.Name, "status.json"))
	if err != nil {
		return ""
	}
	var doc run.StatusDoc
	if json.Unmarshal(b, &doc) != nil || !doc.Active || doc.State == nil {
		return ""
	}
	return string(doc.State.Phase)
}

// ReadRef returns a file inside runDir. rel comes from the lane's event
// log, so it must stay inside the run directory.
func (Lanes) ReadRef(runDir, rel string) (string, bool) {
	rel = filepath.FromSlash(rel)
	if runDir == "" || !filepath.IsLocal(rel) {
		return "", false
	}
	b, err := os.ReadFile(filepath.Join(runDir, rel))
	return string(b), err == nil
}
