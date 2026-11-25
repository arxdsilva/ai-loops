// Package backlogstore keeps the backlog state (specs/BACKLOG.md §1) in
// <main checkout>/.loops/backlog/: state.json, written atomically under an
// exclusive lock (lanes are separate processes), one lock file per running
// lane, and the lane logs.
package backlogstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	appbacklog "github.com/arxdsilva/ai-loops/internal/application/backlog"
	"github.com/arxdsilva/ai-loops/internal/domain/backlog"
	"github.com/arxdsilva/ai-loops/internal/platform/atomicfile"
	"github.com/arxdsilva/ai-loops/internal/platform/loopsdir"
)

// Dir is the backlog directory inside .loops.
const Dir = "backlog"

// errHeld means another live process holds the lane.
var errHeld = errors.New("already has a live lane")

// Store is the state on disk.
type Store struct{ root, dir string }

// New returns the store of the checkout at root.
func New(root string) *Store {
	return &Store{root: root, dir: filepath.Join(root, loopsdir.Name, Dir)}
}

func (s *Store) statePath() string { return filepath.Join(s.dir, "state.json") }

// Exists reports whether `loops backlog init` wrote the state.
func (s *Store) Exists() bool {
	_, err := os.Stat(s.statePath())
	return err == nil
}

// Load reads the state (the zero state when there is none).
func (s *Store) Load(context.Context) (backlog.State, error) {
	b, err := os.ReadFile(s.statePath())
	if errors.Is(err, fs.ErrNotExist) {
		return backlog.State{Version: backlog.Version}, nil
	}
	if err != nil {
		return backlog.State{}, err
	}
	var st backlog.State
	if err := json.Unmarshal(b, &st); err != nil {
		return backlog.State{}, fmt.Errorf("%s: %w", s.statePath(), err)
	}
	if st.Version > backlog.Version {
		return backlog.State{}, fmt.Errorf("%s has version %d; this loops reads up to %d (update loops)", s.statePath(), st.Version, backlog.Version)
	}
	return st, nil
}

// Update runs fn on the state under the lock and saves it when fn
// succeeds.
func (s *Store) Update(ctx context.Context, fn func(*backlog.State) error) error {
	if _, err := loopsdir.Ensure(s.root); err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(s.dir, "state.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := lock(f); err != nil {
		return err
	}
	defer unlock(f)
	st, err := s.Load(ctx)
	if err != nil {
		return err
	}
	if err := fn(&st); err != nil {
		return err
	}
	st.Version = backlog.Version
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.WriteFile(s.statePath(), append(b, '\n'), 0o644)
}

// LogPath is issue n's lane log.
func (s *Store) LogPath(n int) string {
	return filepath.Join(s.dir, "logs", fmt.Sprintf("issue-%d.log", n))
}

func (s *Store) lanePath(n int) string {
	return filepath.Join(s.dir, "lanes", fmt.Sprintf("issue-%d.lock", n))
}

// HoldLane takes issue n's lane lock until release (or the process ends).
func (s *Store) HoldLane(n int) (appbacklog.Lease, error) {
	p := s.lanePath(n)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return nil, err
	}
	f, err := acquire(p)
	if errors.Is(err, errHeld) {
		return nil, fmt.Errorf("issue %d %w", n, err)
	}
	if err != nil {
		return nil, err
	}
	return &Lease{f: f}, nil
}

// Lease is a held lane lock. Its File can be handed to a child process,
// which then holds the lock too until it exits.
type Lease struct {
	once sync.Once
	f    *os.File
}

// Release unlocks the lane, even if a process the file was handed to still
// runs, and closes the file. It is safe to call more than once.
func (l *Lease) Release() { l.once.Do(func() { unlock(l.f); l.f.Close() }) }

// File is the locked file, for a child process to inherit.
func (l *Lease) File() *os.File { return l.f }

// LaneHeld reports whether a live process holds issue n's lane lock.
func (s *Store) LaneHeld(n int) bool { return held(s.lanePath(n)) }
