// Package updatestate persists update.State at
// ${XDG_STATE_HOME:-~/.local/state}/loops/update.json (atomic writes).
package updatestate

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/arxdsilva/ai-loops/internal/domain/update"
	"github.com/arxdsilva/ai-loops/internal/platform/atomicfile"
)

// File implements updatecheck.Store.
type File struct{ path string }

// New returns a File at the default path (empty path disables persistence).
func New() *File { return &File{path: defaultPath()} }

// NewAt returns a File at an explicit path.
func NewAt(path string) *File { return &File{path: path} }

func defaultPath() string {
	state := os.Getenv("XDG_STATE_HOME")
	if state == "" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return ""
		}
		state = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(state, "loops", "update.json")
}

// Load returns the saved state (zero when missing, empty-path or corrupt).
func (f *File) Load() (update.State, error) {
	var s update.State
	if f.path == "" {
		return s, nil
	}
	b, err := os.ReadFile(f.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
		return s, err
	}
	if json.Unmarshal(b, &s) != nil {
		return update.State{}, nil
	}
	return s, nil
}

// Save writes the state atomically.
func (f *File) Save(s update.State) error {
	if f.path == "" {
		return nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return err
	}
	return atomicfile.WriteFile(f.path, b, 0o644)
}
