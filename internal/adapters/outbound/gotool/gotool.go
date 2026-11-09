// Package gotool runs external commands (go, gofmt, staticcheck, gh, …)
// for the gate pipeline and the post-edit hook.
package gotool

import (
	"context"
	"os/exec"
)

// Runner implements gate.Commander.
type Runner struct{}

// New returns a Runner.
func New() *Runner { return &Runner{} }

// Run executes name with args in dir, returning combined output.
func (r *Runner) Run(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Has reports whether name is on PATH.
func (r *Runner) Has(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
