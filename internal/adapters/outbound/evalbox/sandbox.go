package evalbox

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/application/evalrun"
)

// BaseTag marks the fixture commit; the fixture's gate.base points at it,
// so new-code coverage is measured from it even if the run commits.
const BaseTag = "eval-base"

// SeedDir is where a scenario's seed/ lands in the sandbox (inside .loops/,
// so it is never part of the change under test).
const SeedDir = ".loops/eval-seed"

// Sandbox builds a git repo from the fixture in a fresh temp directory,
// outside the user's project.
type Sandbox struct {
	// Fixture is evals/fixture.
	Fixture string
	// TempDir is the parent for sandboxes ("" = the OS temp dir).
	TempDir string
}

// Prepare copies the fixture, commits it on main (tagged BaseTag), then applies the
// scenario's overlay.patch (left uncommitted: it is the change under test)
// and copies its seed/.
func (s Sandbox) Prepare(ctx context.Context, spec evalrun.Spec) (string, error) {
	dir, err := os.MkdirTemp(s.TempDir, "loops-eval-"+spec.Scenario.Name+"-")
	if err != nil {
		return "", err
	}
	// The agent sees the resolved path (macOS: /var -> /private/var); the
	// hooks only make edit targets repo-relative when the two agree.
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	if err := s.prepare(ctx, dir, spec); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	return dir, nil
}

func (s Sandbox) prepare(ctx context.Context, dir string, spec evalrun.Spec) error {
	if err := copyTree(s.Fixture, dir); err != nil {
		return fmt.Errorf("copy fixture: %w", err)
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"add", "-A"},
		{"commit", "-q", "--no-verify", "-m", "fixture"},
		{"tag", BaseTag},
	} {
		if err := git(ctx, dir, args...); err != nil {
			return err
		}
	}
	// git runs inside the sandbox, so the patch path must be absolute.
	patch, err := filepath.Abs(filepath.Join(spec.Dir, "overlay.patch"))
	if err != nil {
		return err
	}
	if _, err := os.Stat(patch); err == nil {
		if err := git(ctx, dir, "apply", patch); err != nil {
			return err
		}
	}
	seed := filepath.Join(spec.Dir, "seed")
	if fi, err := os.Stat(seed); err == nil && fi.IsDir() {
		if err := copyTree(seed, filepath.Join(dir, filepath.FromSlash(SeedDir))); err != nil {
			return fmt.Errorf("copy seed: %w", err)
		}
	}
	return nil
}

// Remove deletes a sandbox.
func (s Sandbox) Remove(dir string) error { return os.RemoveAll(dir) }

// git runs git in dir with a fixed identity and no signing, so the user's
// global config cannot break the fixture commit.
func git(ctx context.Context, dir string, args ...string) error {
	full := append([]string{"-c", "user.name=loops-eval", "-c", "user.email=eval@loops.invalid", "-c", "commit.gpgsign=false"}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(out.String()))
	}
	return nil
}

// copyTree copies the regular files and directories of src into dst.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
}
