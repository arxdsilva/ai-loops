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
