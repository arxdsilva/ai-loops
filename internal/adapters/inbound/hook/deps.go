package hook

import (
	"context"

	"github.com/arxdsilva/ai-loops/internal/application/langpack"
	"github.com/arxdsilva/ai-loops/internal/application/updatecheck"
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
	// Update checks for a newer loops release (nil disables the check).
	Update Updater
	// Version is the running loops version, compared with the latest release.
	Version string
	// UpdateOff is the project's "update_check": false opt-out.
	UpdateOff bool
	// Langs detects the project's language packs (optional). When nil, or
	// when detection fails, post-edit, test-guard and toolchain-env behave
	// as the Go-only hooks did.
	Langs Langs
	// ReadFile reads a file for pack placeholders; nil uses the OS.
	ReadFile func(path string) ([]byte, error)
}

// Langs is language-pack detection as the hooks need it: the lead wires
// langpack.Service.Detect with the project's "languages" overrides.
type Langs interface {
	Detect(root string) ([]langpack.Detected, error)
}

// Updater is the update-notice service (the updatecheck.New service).
type Updater interface {
	Check(ctx context.Context, current string, markNotified bool) (*updatecheck.Notice, error)
}
