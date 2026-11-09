// Package runstore stores runs on disk under <project>/.loops/:
//
//	.loops/config.json          project config (Config)
//	.loops/current              ID of the current run (one per project)
//	.loops/runs/<id>/events.jsonl  append-only event log
//	.loops/runs/<id>/...        ticket.md, research/, plan.v*.md, qa/, proofs/, report.html
//
// Appends are single write(2) calls on an O_APPEND file under an exclusive
// flock (unix), so concurrent hook processes never interleave lines.
package runstore

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
	"github.com/arxdsilva/ai-loops/internal/platform/atomicfile"
)

// Store implements tracker.EventLog and tracker.Pointer.
type Store struct{ root string }

// New returns a Store for the project at root.
func New(root string) *Store { return &Store{root: root} }

// Base returns <root>/.loops.
func (s *Store) Base() string { return filepath.Join(s.root, ".loops") }

// Dir returns the run directory.
func (s *Store) Dir(runID string) string { return filepath.Join(s.Base(), "runs", runID) }

// Append appends e as one JSON line.
func (s *Store) Append(ctx context.Context, runID string, e run.Event) error {
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	dir := s.Dir(runID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "events.jsonl"), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := lock(f); err != nil {
		return err
	}
	defer unlock(f)
	// A torn tail (crash mid-write) must not swallow this event.
	if st, err := f.Stat(); err == nil && st.Size() > 0 {
		var last [1]byte
		if _, err := f.ReadAt(last[:], st.Size()-1); err == nil && last[0] != '\n' {
			line = append([]byte{'\n'}, line...)
		}
	}
	_, err = f.Write(line)
	return err
}

// Create reserves runID by creating its directory; fs.ErrExist if taken.
func (s *Store) Create(ctx context.Context, runID string) error {
	if err := os.MkdirAll(filepath.Dir(s.Dir(runID)), 0o755); err != nil {
		return err
	}
	return os.Mkdir(s.Dir(runID), 0o755)
}

// Load reads all events; malformed lines are skipped. A missing log yields
// no events and no error.
func (s *Store) Load(ctx context.Context, runID string) ([]run.Event, error) {
	f, err := os.Open(filepath.Join(s.Dir(runID), "events.jsonl"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []run.Event
	r := bufio.NewReaderSize(f, 1<<16)
	for {
		line, err := r.ReadBytes('\n')
		if t := strings.TrimSpace(string(line)); t != "" {
			var e run.Event
			if json.Unmarshal([]byte(t), &e) == nil && e.Kind != "" {
				out = append(out, e)
			}
		}
		if err != nil {
			break
		}
	}
	return out, nil
}

func (s *Store) currentPath() string { return filepath.Join(s.Base(), "current") }

// Current returns the current run ID, "" when none.
func (s *Store) Current() (string, error) {
	b, err := os.ReadFile(s.currentPath())
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// SetCurrent writes .loops/current atomically.
func (s *Store) SetCurrent(runID string) error {
	if err := os.MkdirAll(s.Base(), 0o755); err != nil {
		return err
	}
	return atomicfile.WriteFile(s.currentPath(), []byte(runID+"\n"), 0o644)
}

// Clear removes .loops/current.
func (s *Store) Clear() error {
	err := os.Remove(s.currentPath())
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
