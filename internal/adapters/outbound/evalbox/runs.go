package evalbox

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/runstore"
	"github.com/arxdsilva/ai-loops/internal/domain/run"
	"github.com/arxdsilva/ai-loops/internal/platform/loopsdir"
)

// Runs reads run directories from disk.
type Runs struct{}

// Latest returns the newest run under dir/.loops/runs (run ids start with
// their start time, so the greatest name is the newest), or "".
func (Runs) Latest(dir string) (string, error) {
	base := filepath.Join(dir, loopsdir.Name, "runs")
	entries, err := os.ReadDir(base)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	latest := ""
	for _, e := range entries {
		if e.IsDir() && e.Name() > latest {
			latest = e.Name()
		}
	}
	if latest == "" {
		return "", nil
	}
	return filepath.Join(base, latest), nil
}

// Events reads runDir/events.jsonl; a missing log is no events.
func (Runs) Events(runDir string) ([]run.Event, error) {
	f, err := os.Open(filepath.Join(runDir, "events.jsonl"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return runstore.ReadEvents(f), nil
}

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
