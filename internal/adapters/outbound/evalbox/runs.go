package evalbox

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/runstore"
	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

// Runs reads run directories from disk.
type Runs struct{}

// Latest returns the newest run under dir/.loops/runs, or "".
func (Runs) Latest(dir string) (string, error) { return runstore.LatestRun(dir) }

// Events reads runDir/events.jsonl; a missing log is no events.
func (Runs) Events(runDir string) ([]run.Event, error) { return runstore.EventsIn(runDir) }

// Read returns a file inside runDir. rel comes from the event log, so it
// must stay inside the run directory.
func (Runs) Read(runDir, rel string) (string, bool) {
	rel = filepath.FromSlash(rel)
	if !filepath.IsLocal(rel) {
		return "", false
	}
	b, err := os.ReadFile(filepath.Join(runDir, rel))
	return string(b), err == nil
}

// LatestPlan returns the plan.v<N>.md with the highest N, or "".
func (r Runs) LatestPlan(runDir string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(runDir, "plan.v*.md"))
	if err != nil {
		return "", err
	}
	version := func(p string) int {
		n, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(filepath.Base(p), "plan.v"), ".md"))
		return n
	}
	sort.Slice(matches, func(i, j int) bool { return version(matches[i]) < version(matches[j]) })
	if len(matches) == 0 {
		return "", nil
	}
	b, err := os.ReadFile(matches[len(matches)-1])
	return string(b), err
}
