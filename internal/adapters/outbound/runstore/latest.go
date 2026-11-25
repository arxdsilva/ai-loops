package runstore

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
	"github.com/arxdsilva/ai-loops/internal/platform/loopsdir"
)

// LatestRun returns the newest run directory under root/.loops/runs (run
// ids start with their start time, so the greatest name is the newest), or
// "" when there is none.
func LatestRun(root string) (string, error) {
	base := filepath.Join(root, loopsdir.Name, "runs")
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

// RunDirs returns every run directory under root/.loops/runs, oldest first
// (run ids start with their start time); none is a nil slice.
func RunDirs(root string) ([]string, error) {
	base := filepath.Join(root, loopsdir.Name, "runs")
	entries, err := os.ReadDir(base)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries { // ReadDir sorts by name
		if e.IsDir() {
			out = append(out, filepath.Join(base, e.Name()))
		}
	}
	return out, nil
}

// Logs reads run logs straight from disk (`loops stats`).
type Logs struct{}

// Dirs lists the run directories of the project at root.
func (Logs) Dirs(root string) ([]string, error) { return RunDirs(root) }

// Events reads one run directory's events.jsonl.
func (Logs) Events(runDir string) ([]run.Event, error) { return EventsIn(runDir) }

// EventsIn reads runDir/events.jsonl; a missing log is no events.
func EventsIn(runDir string) ([]run.Event, error) {
	f, err := os.Open(filepath.Join(runDir, "events.jsonl"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ReadEvents(f), nil
}
