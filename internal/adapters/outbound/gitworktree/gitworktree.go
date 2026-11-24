// Package gitworktree adds and removes the backlog lanes' git worktrees
// (specs/BACKLOG.md §3) and finds the main checkout from any worktree.
package gitworktree

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Git runs git in the main checkout.
type Git struct{ root string }

// New returns a Git rooted at the main checkout.
func New(root string) *Git { return &Git{root: root} }

func run(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

// Add fetches origin's base and adds a detached worktree at dir on it. A
// leftover worktree at dir is removed first; git refuses a dirty one, so
// unsaved work is never lost.
func (g *Git) Add(ctx context.Context, dir, base string) error {
	if _, err := run(ctx, g.root, "fetch", "--quiet", "origin", base); err != nil {
		return err
	}
	if g.Exists(dir) {
		if err := g.Remove(ctx, dir); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return err
	}
	_, err := run(ctx, g.root, "worktree", "add", "--detach", dir, "origin/"+base)
	return err
}

// Remove removes the worktree at dir (git refuses a dirty one) and prunes
// stale worktree records.
func (g *Git) Remove(ctx context.Context, dir string) error {
	if _, err := run(ctx, g.root, "worktree", "remove", dir); err != nil {
		return err
	}
	_, err := run(ctx, g.root, "worktree", "prune")
	return err
}

// Exists reports whether dir exists.
func (g *Git) Exists(dir string) bool {
	_, err := os.Stat(dir)
	return err == nil
}

// MainRoot returns the main checkout of the repository holding dir (dir
// itself when it is the main checkout, or not in git).
func MainRoot(ctx context.Context, dir string) string {
	common, err := run(ctx, dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil || filepath.Base(common) != ".git" {
		return dir
	}
	return filepath.Dir(common)
}

// DefaultWorktrees is where lanes' worktrees go: beside the main checkout,
// in <name>.loops-worktrees.
func DefaultWorktrees(main string) string {
	return filepath.Join(filepath.Dir(main), filepath.Base(main)+".loops-worktrees")
}
