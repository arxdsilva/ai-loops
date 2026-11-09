// Package tracker records a run's events and answers what state it is in.
// It is the single writer API for events.jsonl: skills reach it through the
// `loops run` CLI and hooks through the hook adapter.
package tracker

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/insurgencygames/ai-loops/internal/domain/run"
)

// ErrNoActiveRun is returned when an operation needs the current run and
// none is active. Hooks treat it as "do nothing".
var ErrNoActiveRun = errors.New("no active loops run")

// EventLog stores a run's events (append-only) and its working directory.
type EventLog interface {
	// Append atomically appends one event to the run's log; safe for
	// concurrent callers in different processes.
	Append(ctx context.Context, runID string, e run.Event) error
	// Load returns every event of the run in file order.
	Load(ctx context.Context, runID string) ([]run.Event, error)
	// Create reserves runID atomically (creating its directory). It returns
	// an error wrapping fs.ErrExist when the ID is already taken.
	Create(ctx context.Context, runID string) error
	// Dir returns the run's directory (created by Append if missing).
	Dir(runID string) string
}

// Pointer stores which run is current for the project.
type Pointer interface {
	Current() (string, error) // "" with nil error when none
	SetCurrent(runID string) error
	Clear() error
}

// Clock is the injected wall clock.
type Clock interface{ Now() time.Time }

// StartInput describes a new run.
type StartInput struct {
	Title  string // short title, becomes part of the run ID slug
	Ticket string // ticket ref (#123, URL, ABC-123) or "" for free text
	Source string // github | gitlab | jira | linear | text
}

// Service is the tracker use case.
type Service struct {
	log   EventLog
	ptr   Pointer
	clock Clock
}

// New builds a Service.
func New(log EventLog, ptr Pointer, clock Clock) *Service {
	return &Service{log: log, ptr: ptr, clock: clock}
}

// Start creates a run (ID = UTC timestamp + slug of Title, e.g.
// "20261109-1530-add-orders"), makes it current, records KindRun and enters
// PhaseIntake. It returns the run ID.
func (s *Service) Start(ctx context.Context, in StartInput) (string, error) {
	now := s.clock.Now().UTC()
	base := now.Format("20060102-1504") + "-" + slug(in.Title)
	id := base
	for n := 2; ; n++ {
		err := s.log.Create(ctx, id)
		if err == nil {
			break
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", err
		}
		id = fmt.Sprintf("%s-%d", base, n)
	}
	if err := s.log.Append(ctx, id, run.Event{Time: now, Kind: run.KindRun, Run: id, Name: in.Title, Target: in.Ticket, Note: in.Source}); err != nil {
		return "", err
	}
	if err := s.log.Append(ctx, id, run.Event{Time: now, Kind: run.KindPhase, Run: id, Phase: run.PhaseIntake}); err != nil {
		return "", err
	}
	if err := s.ptr.SetCurrent(id); err != nil {
		return "", err
	}
	return id, nil
}

func slug(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	out := b.String()
	if len(out) > 40 {
		out = out[:40]
	}
	out = strings.Trim(out, "-")
	if out == "" {
		out = "run"
	}
	return out
}

// Record appends e to the current run, filling Run and Time when empty.
// Returns ErrNoActiveRun when no run is current.
func (s *Service) Record(ctx context.Context, e run.Event) error {
	id, err := s.Current(ctx)
	if err != nil {
		return err
	}
	if e.Run == "" {
		e.Run = id
	}
	if e.Time.IsZero() {
		e.Time = s.clock.Now().UTC()
	}
	return s.log.Append(ctx, id, e)
}

// End records KindEnd with outcome and clears the current pointer.
func (s *Service) End(ctx context.Context, outcome string) error {
	if err := s.Record(ctx, run.Event{Kind: run.KindEnd, Status: outcome}); err != nil {
		return err
	}
	return s.ptr.Clear()
}

// Current returns the current run ID or ErrNoActiveRun.
func (s *Service) Current(ctx context.Context) (string, error) {
	id, err := s.ptr.Current()
	if err != nil {
		return "", err
	}
	if id == "" {
		return "", ErrNoActiveRun
	}
	return id, nil
}

func (s *Service) resolve(ctx context.Context, runID string) (string, error) {
	if runID != "" {
		return runID, nil
	}
	return s.Current(ctx)
}

// State folds the run's events; runID "" means the current run.
func (s *Service) State(ctx context.Context, runID string) (run.State, error) {
	evs, err := s.Events(ctx, runID)
	if err != nil {
		return run.State{}, err
	}
	return run.Fold(evs), nil
}

// Events returns the run's raw events; runID "" means the current run.
func (s *Service) Events(ctx context.Context, runID string) ([]run.Event, error) {
	id, err := s.resolve(ctx, runID)
	if err != nil {
		return nil, err
	}
	return s.log.Load(ctx, id)
}

// Dir returns the run's directory; runID "" means the current run.
func (s *Service) Dir(ctx context.Context, runID string) (string, error) {
	id, err := s.resolve(ctx, runID)
	if err != nil {
		return "", err
	}
	return s.log.Dir(id), nil
}
