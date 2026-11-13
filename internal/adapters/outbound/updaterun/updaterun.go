// Package updaterun runs the steps of an automatic plugin update
// (`loops update --apply`): argv lists executed directly, never through a
// shell, with an empty stdin and the caller's deadline.
package updaterun

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/arxdsilva/ai-loops/internal/application/updatecheck"
)

var _ updatecheck.Runner = (*runner)(nil)

// runner implements updatecheck.Runner with os/exec.
type runner struct {
	lookPath  func(string) (string, error) // nil = exec.LookPath
	waitDelay time.Duration                // how long Run waits for a killed child's pipes to close
	maxOutput int                          // the most output Run returns (the last bytes written)
}

// New returns a runner with a 2s wait delay and a 64 KiB output cap.
func New() *runner { return &runner{waitDelay: 2 * time.Second, maxOutput: 64 << 10} }

// Run runs argv[0] from PATH with argv[1:] and returns the combined output
// (at most the last 64 KiB). The child inherits env and cwd, reads EOF on
// stdin and is killed when ctx ends; the error then wraps ctx.Err().
func (r *runner) Run(ctx context.Context, argv []string) (string, error) {
	if len(argv) == 0 {
		return "", errors.New("updaterun: empty command")
	}
	look := r.lookPath
	if look == nil {
		look = exec.LookPath
	}
	path, err := look(argv[0])
	if err != nil {
		return "", fmt.Errorf("%w: %s", updatecheck.ErrNoCommand, argv[0])
	}
	buf := &tailBuffer{max: r.maxOutput}
	cmd := exec.CommandContext(ctx, path, argv[1:]...)
	cmd.Stdout = buf
	cmd.Stderr = buf
	cmd.WaitDelay = r.waitDelay
	err = cmd.Run()
	out := buf.String()
	if cerr := ctx.Err(); cerr != nil {
		return out, fmt.Errorf("updaterun: %s: %w", argv[0], cerr)
	}
	if err != nil {
		return out, fmt.Errorf("updaterun: %s: %w", argv[0], err)
	}
	return out, nil
}

// tailBuffer keeps at most max of the last bytes written to it.
type tailBuffer struct {
	max int
	buf []byte
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if len(p) >= b.max {
		b.buf = append(b.buf[:0], p[len(p)-b.max:]...)
		return n, nil
	}
	b.buf = append(b.buf, p...)
	if over := len(b.buf) - b.max; over > 0 {
		b.buf = append(b.buf[:0], b.buf[over:]...)
	}
	return n, nil
}

func (b *tailBuffer) String() string { return string(b.buf) }
