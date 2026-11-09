// Package updatecheck tells users a newer loops release exists (the first
// run after a release, at most one network check a day).
package updatecheck

import (
	"context"
	"time"

	"github.com/arxdsilva/ai-loops/internal/domain/update"
)

// Source returns the latest released version ("v0.3.0").
type Source interface {
	Latest(ctx context.Context) (string, error)
}

// Store persists update.State.
type Store interface {
	Load() (update.State, error)
	Save(update.State) error
}

// Clock is the injected wall clock.
type Clock interface{ Now() time.Time }

// Notice is a pending "update available" message.
type Notice struct {
	Current string
	Latest  string
}

// Service runs the check.
type Service struct {
	src   Source
	store Store
	clock Clock
}

// New builds a Service.
func New(src Source, store Store, clock Clock) *Service {
	return &Service{src: src, store: store, clock: clock}
}

// Check loads state, fetches when due (errors are swallowed: state keeps
// the previous Latest and LastCheck advances so an offline machine is not
// re-probed every run), saves, and returns a Notice when the user should be
// told. markNotified=true records the notice as delivered (session-start);
// false only peeks (`loops version --check`).
func (s *Service) Check(ctx context.Context, current string, markNotified bool) (*Notice, error) {
	st, _ := s.store.Load() // unreadable state behaves like a first run
	now := s.clock.Now()
	var saveErr error
	if update.Decide(current, st, now).Fetch {
		if v, err := s.src.Latest(ctx); err == nil && v != "" {
			st.Latest = v
		}
		st.LastCheck = now
		saveErr = s.store.Save(st)
	}
	d := update.Decide(current, st, now)
	if !d.Notify {
		return nil, saveErr
	}
	if markNotified {
		st.LastNotified = d.Latest
		if err := s.store.Save(st); err != nil {
			saveErr = err
		}
	}
	return &Notice{Current: current, Latest: d.Latest}, saveErr
}

// Fetch queries the source now, ignoring the throttle, and records the
// result (LastCheck, Latest). On a source error the state is left alone.
func (s *Service) Fetch(ctx context.Context) (update.State, error) {
	st, _ := s.store.Load()
	v, err := s.src.Latest(ctx)
	if err != nil {
		return st, err
	}
	st.Latest, st.LastCheck = v, s.clock.Now()
	return st, s.store.Save(st)
}
