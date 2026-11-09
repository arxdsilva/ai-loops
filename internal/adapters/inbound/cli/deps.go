package cli

import (
	"context"
	"io"

	"github.com/arxdsilva/ai-loops/internal/application/gate"
	"github.com/arxdsilva/ai-loops/internal/application/ticket"
	"github.com/arxdsilva/ai-loops/internal/application/tracker"
	"github.com/arxdsilva/ai-loops/internal/domain/archrule"
	"github.com/arxdsilva/ai-loops/internal/domain/hw"
	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

// Tracker is what the CLI needs from the run tracker.
type Tracker interface {
	Start(ctx context.Context, in tracker.StartInput) (string, error)
	Record(ctx context.Context, e run.Event) error
	End(ctx context.Context, outcome string) error
	State(ctx context.Context, runID string) (run.State, error)
	Events(ctx context.Context, runID string) ([]run.Event, error)
	Dir(ctx context.Context, runID string) (string, error)
}

// Gater runs the gate pipeline.
type Gater interface {
	Run(ctx context.Context, o gate.Options) (gate.Result, error)
}

// Archer runs the architecture check.
type Archer interface {
	Check(ctx context.Context, root string, layers archrule.Layers) (archrule.Report, error)
}

// Tickets fetches tickets.
type Tickets interface {
	Fetch(ctx context.Context, source, ref, body string) (ticket.Ticket, error)
}

// Reporter renders the HTML report.
type Reporter interface {
	Render(w io.Writer, s run.State, events []run.Event, dir string) error
}

// Hooks handles `loops hook <name>`.
type Hooks interface {
	Handle(ctx context.Context, name, platform string, in io.Reader, out, errw io.Writer) int
}

// PackageWriter writes generated package files to disk and reports drift.
type PackageWriter interface {
	// Write writes files under dir (creating directories, setting the exec
	// bit) and removes files under dir that are not in files, so dir is an
	// exact image. It refuses to touch a dir without a .loops-generated
	// marker unless the dir is empty or missing.
	Write(dir string, files []plugin.File) error
	// Diff returns the relative paths that differ between dir and files
	// (missing, extra or changed).
	Diff(dir string, files []plugin.File) ([]string, error)
	// Place copies files to absolute destinations (installer), skipping
	// existing files unless force; it returns the destinations written.
	Place(placements []plugin.Placement, force bool) ([]string, error)
}

// HWProbe measures the machine.
type HWProbe interface {
	Probe(ctx context.Context, dir string) (hw.Facts, error)
}
