package gitworktree

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// repo makes origin (bare) with one commit on main, and a clone of it.
func repo(t *testing.T) (clone string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	tmp, _ := filepath.EvalSymlinks(t.TempDir())
	origin := filepath.Join(tmp, "origin.git")
	git(t, tmp, "init", "--quiet", "--bare", "-b", "main", origin)
	seed := filepath.Join(tmp, "seed")
	git(t, tmp, "init", "--quiet", "-b", "main", seed)
	os.WriteFile(filepath.Join(seed, "a.txt"), []byte("a\n"), 0o644)
	git(t, seed, "add", ".")
	git(t, seed, "commit", "--quiet", "-m", "seed")
	git(t, seed, "push", "--quiet", origin, "main")
	clone = filepath.Join(tmp, "main")
	git(t, tmp, "clone", "--quiet", origin, clone)
	return clone
}

func TestAddRemoveAndMainRoot(t *testing.T) {
	clone := repo(t)
	ctx := context.Background()
	g := New(clone)
	dir := filepath.Join(DefaultWorktrees(clone), "issue-1")
	if !strings.HasSuffix(filepath.Dir(dir), "main.loops-worktrees") {
		t.Fatalf("default dir = %s", dir)
	}
	if err := g.Add(ctx, dir, "main"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.txt")); err != nil {
		t.Fatal("worktree not checked out")
	}
	if b := git(t, dir, "rev-parse", "--abbrev-ref", "HEAD"); b != "HEAD" {
		t.Fatalf("worktree not detached: %s", b)
	}
	if got := MainRoot(ctx, dir); got != clone {
		t.Fatalf("MainRoot(worktree) = %s, want %s", got, clone)
	}
	if got := MainRoot(ctx, clone); got != clone {
		t.Fatalf("MainRoot(main) = %s", got)
	}
	// A clean leftover is replaced.
	if err := g.Add(ctx, dir, "main"); err != nil {
		t.Fatalf("re-add: %v", err)
	}
	// A dirty one is kept: git refuses to remove it.
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("changed\n"), 0o644)
	if err := g.Add(ctx, dir, "main"); err == nil {
		t.Fatal("dirty worktree replaced")
	}
	if err := g.Remove(ctx, dir); err == nil {
		t.Fatal("dirty worktree removed")
	}
	git(t, dir, "checkout", "--", "a.txt")
	if err := g.Remove(ctx, dir); err != nil || g.Exists(dir) {
		t.Fatalf("remove: %v", err)
	}
	if err := g.Add(ctx, dir, "no-such-branch"); err == nil {
		t.Fatal("unknown base accepted")
	}
}

func TestMainRootOutsideGit(t *testing.T) {
	dir := t.TempDir()
	if got := MainRoot(context.Background(), dir); got != dir {
		t.Fatalf("MainRoot outside git = %s", got)
	}
}
