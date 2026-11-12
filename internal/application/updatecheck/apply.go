package updatecheck

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/arxdsilva/ai-loops/internal/domain/update"
)

// Runner runs one update step: argv[0] looked up on PATH with argv[1:], no
// shell, empty stdin. It returns the combined output.
type Runner interface {
	Run(ctx context.Context, argv []string) (string, error)
}

// ErrNoCommand is wrapped by Runner adapters when argv[0] is not on PATH.
var ErrNoCommand = errors.New("not on PATH")

// Kind is the outcome of an apply.
type Kind string

// Outcome kinds (the CLI prints one line per kind).
const (
	KindUpToDate    Kind = "up-to-date"
	KindUpdated     Kind = "updated"
	KindFailed      Kind = "failed"
	KindManual      Kind = "manual"
	KindPending     Kind = "pending"
	KindBackoff     Kind = "backoff"
	KindCheckFailed Kind = "check-failed"
)

// Outcome is everything Apply reports; every failure is a Kind, never an
// error, so callers fail open.
type Outcome struct {
	Kind     Kind
	Current  string
	Latest   string
	Platform string
	Manual   string    // Command(platform); empty for up-to-date and check-failed
	Reload   string    // Reload(platform)
	Detail   string    // one line, at most 200 bytes
	At       time.Time // pending, backoff: the recorded AppliedAt
	RetryAt  time.Time // backoff
}

// WithRunner sets the runner for automatic update routes (nil = no
// automatic route) and returns s.
func (s *Service) WithRunner(r Runner) *Service {
	s.runner = r
	return s
}

// maxDetail is the longest Outcome.Detail in bytes.
const maxDetail = 200

// Apply checks now (ignoring the throttle) and, when the installed plugin
// (version current) is behind on a platform with an automatic route, runs
// that route and records the attempt. It never returns an error: every
// failure is a Kind.
func (s *Service) Apply(ctx context.Context, current, platform string) Outcome {
	st, _ := s.store.Load() // unreadable state behaves like a first run
	now := s.clock.Now()
	p := normPlatform(platform)
	out := Outcome{Current: current, Platform: p, Reload: Reload(p)}

	v, err := s.src.Latest(ctx)
	if err != nil || v == "" {
		return checkFailed(out, err)
	}
	st.Latest, st.LastCheck = v, now
	out.Latest = update.Tag(v)

	route, ok := Route(p)
	d := update.DecideApply(current, v, ok && s.runner != nil, st, now)
	if d.Step != update.StepNone {
		out.Manual = Command(p)
	}
	out = s.decide(ctx, out, &st, d, route, now)
	_ = s.store.Save(st) // a save error never changes the outcome
	return out
}

// checkFailed marks out as a failed check; err may be nil (empty release).
func checkFailed(out Outcome, err error) Outcome {
	out.Kind, out.Detail = KindCheckFailed, "no release found"
	if err != nil {
		out.Detail = oneLine(err.Error())
	}
	return out
}

// decide fills out for the step and records a run attempt in st.
func (s *Service) decide(ctx context.Context, out Outcome, st *update.State, d update.ApplyDecision, route [][]string, now time.Time) Outcome {
	switch d.Step {
	case update.StepManual:
		out.Kind = KindManual
	case update.StepDone:
		out.Kind, out.At = KindPending, st.AppliedAt
	case update.StepBackoff:
		out.Kind, out.At, out.RetryAt = KindBackoff, st.AppliedAt, d.RetryAt
		out.Detail = oneLine(st.ApplyErr)
	case update.StepRun:
		detail := s.runRoute(ctx, route)
		st.Applied, st.AppliedAt, st.ApplyErr = st.Latest, now, detail
		out.Kind, out.Detail = KindUpdated, detail
		if detail != "" {
			out.Kind = KindFailed
		}
	default: // StepNone
		out.Kind = KindUpToDate
	}
	return out
}

// runRoute runs each step in order and returns "" on success or the
// one-line failure detail of the first failing step.
func (s *Service) runRoute(ctx context.Context, route [][]string) string {
	for _, argv := range route {
		output, err := s.runner.Run(ctx, argv)
		if err == nil {
			continue
		}
		return oneLine(strings.Join(argv, " ") + ": " + reason(ctx, argv, output, err))
	}
	return ""
}

func reason(ctx context.Context, argv []string, output string, err error) string {
	switch {
	case errors.Is(err, ErrNoCommand):
		return argv[0] + " not on PATH"
	case ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded):
		return "timed out"
	}
	r := err.Error()
	if last := lastLine(output); last != "" {
		r += ": " + last
	}
	return r
}

func lastLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" {
			return l
		}
	}
	return ""
}

// oneLine collapses whitespace runs to single spaces and cuts to maxDetail
// bytes on a rune boundary.
func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= maxDetail {
		return s
	}
	cut := maxDetail
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
