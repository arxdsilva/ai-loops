package backlog

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/arxdsilva/ai-loops/internal/domain/backlog"
)

// RunInput is `loops backlog run`.
type RunInput struct {
	Max int       // lanes to start in all (0 = until the queue is empty)
	Out io.Writer // one line per lane start and end
}

// RunSummary is what the loop did.
type RunSummary struct {
	Started, Done, Retried, Human, Errors, Findings int
}

type laneDone struct {
	res LaneResult
	err error
}

// Run is the supervisor without an agent (BACKLOG §8): sync, keep the
// configured lanes busy until the queue is empty (or Max lanes started),
// then sync once more and stop when nothing new was queued. Failures follow
// the lane's own policy (BACKLOG §3 step 6). ctx ends it: running lanes
// are stopped and recorded as failed attempts.
func (s *Service) Run(ctx context.Context, in RunInput) (RunSummary, error) {
	out := in.Out
	if out == nil {
		out = io.Discard
	}
	var sum RunSummary
	for {
		res, err := s.Sync(ctx)
		if err != nil {
			return sum, err
		}
		fmt.Fprintf(out, "sync: %d open, %d queued\n", res.Open, res.Queued)
		if res.Queued == 0 || (in.Max > 0 && sum.Started >= in.Max) || ctx.Err() != nil {
			return sum, ctx.Err()
		}
		ran, err := s.drain(ctx, in.Max, out, &sum)
		if err != nil {
			return sum, err
		}
		if ran == 0 && ctx.Err() == nil {
			return sum, errors.New("issues are queued but no lane could start (see the errors above)")
		}
	}
}

// drain runs lanes until no issue is left to claim or Max is reached. It
// returns how many lanes ran to an outcome.
func (s *Service) drain(ctx context.Context, maxLanes int, out io.Writer, sum *RunSummary) (int, error) {
	st, err := s.load(ctx)
	if err != nil {
		return 0, err
	}
	lanes := max(1, st.Settings.Lanes)
	done := make(chan laneDone)
	active, ran, empty := 0, 0, false
	for {
		for !empty && active < lanes && (maxLanes == 0 || sum.Started < maxLanes) && ctx.Err() == nil {
			active++
			sum.Started++
			go func() {
				res, err := s.Work(ctx, 0)
				done <- laneDone{res: res, err: err}
			}()
		}
		if active == 0 {
			return ran, nil
		}
		d := <-done
		active--
		switch {
		case d.err != nil:
			sum.Errors++
			empty = true // do not spin on a lane that cannot start
			fmt.Fprintf(out, "lane error: %v\n", d.err)
		case d.res.Empty:
			sum.Started--
			empty = true
		default:
			ran++
			report(out, d.res, sum)
		}
	}
}

func report(out io.Writer, r LaneResult, sum *RunSummary) {
	sum.Findings += len(r.Findings)
	switch r.Status {
	case backlog.Done:
		sum.Done++
		fmt.Fprintf(out, "#%d done: %s\n", r.Item.Number, r.Item.PR)
	case backlog.Queued:
		sum.Retried++
		fmt.Fprintf(out, "#%d failed (attempt %d), queued again: %s\n", r.Item.Number, r.Item.Attempts, r.Reason)
	default:
		sum.Human++
		fmt.Fprintf(out, "#%d handed to the human: %s\n", r.Item.Number, r.Reason)
	}
	for _, f := range r.Findings {
		if f.Duplicate {
			fmt.Fprintf(out, "  finding (already tracked): %s %s\n", f.Title, f.URL)
		} else {
			fmt.Fprintf(out, "  finding filed: %s %s\n", f.Title, f.URL)
		}
	}
}
