package gitcli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func write(t *testing.T, dir, name, data string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	dir := t.TempDir()
	ctx := context.Background()
	g := New(dir)
	git(t, dir, "init", "-q")
	write(t, dir, "a.txt", "1")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", "feat: first")
	if tag, err := g.LastTag(ctx); err != nil || tag != "" {
		t.Fatalf("tag %q %v", tag, err)
	}
	all, err := g.CommitsSince(ctx, "")
	if err != nil || !reflect.DeepEqual(all, []string{"feat: first"}) {
		t.Fatalf("%q %v", all, err)
	}
	git(t, dir, "tag", "-a", "v0.1.0", "-m", "x")
	git(t, dir, "tag", "-a", "v0.10.0", "-m", "x")
	git(t, dir, "tag", "-a", "v0.9.0", "-m", "x")
	if tag, _ := g.LastTag(ctx); tag != "v0.10.0" {
		t.Fatalf("tag %q", tag)
	}
	if tags, _ := g.HeadTags(ctx); len(tags) != 3 {
		t.Fatalf("%q", tags)
	}
	write(t, dir, "a.txt", "2")
	git(t, dir, "commit", "-q", "-am", "fix: second\n\nBREAKING CHANGE: yes")
	cs, _ := g.CommitsSince(ctx, "v0.10.0")
	if len(cs) != 1 || cs[0] != "fix: second\n\nBREAKING CHANGE: yes" {
		t.Fatalf("%q", cs)
	}
	if s, _ := g.HeadSubject(ctx); s != "fix: second" {
		t.Fatalf("%q", s)
	}
	if d, _ := g.Dirty(ctx); d {
		t.Fatal("clean tree dirty")
	}
	write(t, dir, "a.txt", "3")
	write(t, dir, "other.txt", "o")
	if d, _ := g.Dirty(ctx); !d {
		t.Fatal("dirty tree clean")
	}
	// Commit only the listed paths (a missing path is ignored).
	git(t, dir, "config", "user.name", "t")
	git(t, dir, "config", "user.email", "t@t")
	git(t, dir, "config", "commit.gpgsign", "false")
	git(t, dir, "config", "tag.gpgsign", "false")
	if err := g.Commit(ctx, "release v1", []string{"a.txt", "missing"}); err != nil {
		t.Fatal(err)
	}
	if s, _ := g.HeadSubject(ctx); s != "release v1" {
		t.Fatalf("%q", s)
	}
	out, _ := g.run(ctx, "status", "--porcelain")
	if out != "?? other.txt\n" {
		t.Fatalf("unrelated file committed: %q", out)
	}
	if err := g.Tag(ctx, "v1.0.0", "loops v1.0.0"); err != nil {
		t.Fatal(err)
	}
	if ty, _ := g.run(ctx, "cat-file", "-t", "v1.0.0"); ty != "tag\n" {
		t.Fatalf("not annotated: %q", ty)
	}
	if err := g.Commit(ctx, "x", []string{"missing"}); err == nil {
		t.Fatal("no paths accepted")
	}
	if err := g.Push(ctx, "HEAD"); err == nil {
		t.Fatal("push without origin succeeded")
	}
	if _, err := New(notInRepo(t)).LastTag(ctx); err == nil {
		t.Fatal("non-repo accepted")
	}
}

// notInRepo returns a fresh temp dir and stops git's upward discovery at its
// parent, so the test holds even when TMPDIR lies inside a git work tree.
func notInRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	parent := filepath.Dir(dir)
	if r, err := filepath.EvalSymlinks(parent); err == nil {
		parent = r
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", parent)
	return dir
}

// A release whose branch push is rejected (main moved on) must not leave its
// tag on origin: that stale tag blocked every later release.
func TestPushIsAtomicAndTagExists(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	ctx := context.Background()
	origin, a, b := t.TempDir(), t.TempDir(), t.TempDir()
	git(t, origin, "init", "-q", "--bare", "-b", "main")
	git(t, a, "init", "-q", "-b", "main")
	write(t, a, "a.txt", "1")
	git(t, a, "add", ".")
	git(t, a, "commit", "-q", "-m", "feat: first")
	git(t, a, "remote", "add", "origin", origin)
	git(t, a, "push", "-q", "origin", "main")
	git(t, b, "clone", "-q", origin, ".")
	write(t, b, "b.txt", "1")
	git(t, b, "add", ".")
	git(t, b, "commit", "-q", "-m", "fix: moved on")
	git(t, b, "push", "-q", "origin", "main")

	write(t, a, "a.txt", "2")
	git(t, a, "commit", "-q", "-am", "release v0.1.0")
	git(t, a, "tag", "-a", "v0.1.0", "-m", "x")
	g := New(a)
	if ok, err := g.TagExists(ctx, "v0.1.0"); err != nil || !ok {
		t.Fatalf("tag exists: %v %v", ok, err)
	}
	if ok, err := g.TagExists(ctx, "v9.9.9"); err != nil || ok {
		t.Fatalf("missing tag reported: %v %v", ok, err)
	}
	if err := g.Push(ctx, "HEAD:main", "v0.1.0"); err == nil {
		t.Fatal("non-fast-forward push succeeded")
	}
	if ok, _ := New(origin).TagExists(ctx, "v0.1.0"); ok {
		t.Fatal("rejected push left its tag on origin")
	}
}
