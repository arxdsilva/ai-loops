package cli

import (
	"context"
	"io"

	"github.com/insurgencygames/ai-loops/internal/application/gate"
	"github.com/insurgencygames/ai-loops/internal/application/ticket"
	"github.com/insurgencygames/ai-loops/internal/application/tracker"
	"github.com/insurgencygames/ai-loops/internal/domain/archrule"
	"github.com/insurgencygames/ai-loops/internal/domain/run"
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
	Handle(ctx context.Context, name string, in io.Reader, out, errw io.Writer) int
}
