package hook

import (
	"context"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

// Recorder is the run tracker as the hooks need it.
type Recorder interface {
	Record(ctx context.Context, e run.Event) error
	Current(ctx context.Context) (string, error)
	State(ctx context.Context, runID string) (run.State, error)
	Events(ctx context.Context, runID string) ([]run.Event, error)
	Dir(ctx context.Context, runID string) (string, error)
}

// Commander runs formatters and vet.
type Commander interface {
	Run(ctx context.Context, dir, name string, args ...string) (string, error)
	Has(name string) bool
}

// Deps are the handler's collaborators.
type Deps struct {
	Rec   Recorder
	Files interface { // guard.Files
		Lines(path string) (int, error)
		Exists(path string) bool
		Glob(pattern string) ([]string, error)
	}
	Cmd   Commander
	Env   func(string) string
	Getwd func() (string, error)
	// WriteFile atomically writes a file (creating parents); nil uses the OS.
	WriteFile func(path string, data []byte) error
}
