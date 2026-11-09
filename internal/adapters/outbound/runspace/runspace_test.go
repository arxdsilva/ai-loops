package runspace

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/domain/workspace"
)

var elf = "\x7fELF\x02\x01\x01\x00binary"

func needGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func write(t *testing.T, dir, name, data string, mode os.FileMode) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), mode); err != nil {
		t.Fatal(err)
	}
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// project builds a temp project whose root is resolved (macOS /var -> /private/var).
func project(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func runDir(t *testing.T, root string) string {
	t.Helper()
	d := filepath.Join(root, ".loops", "runs", "r1")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestPrepareAndEnv(t *testing.T) {
	needGit(t)
	root := project(t)
	git(t, root, "init", "-q")
	write(t, root, "tracked.go", "package x\n", 0o644)
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "init")
	write(t, root, "old.test", elf, 0o755)
	write(t, root, "ign/x.log", "log", 0o644)
	write(t, root, ".gitignore", "ign/\n", 0o644)
	rd := runDir(t, root)
	write(t, root, ".loops/runs/r0/zzz.test", elf, 0o644)
	s := New(root)
	if err := s.Prepare(context.Background(), rd); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{workspace.GoCache(rd), workspace.GoTmp(rd)} {
		if fi, err := os.Stat(d); err != nil || !fi.IsDir() {
			t.Fatal(d, err)
		}
	}
	b, err := os.ReadFile(workspace.Baseline(rd))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if lines[0] != headerGit || !slices.Contains(lines, "old.test") || !slices.Contains(lines, "ign/x.log") || !slices.Contains(lines, ".gitignore") {
		t.Fatalf("baseline %q", b)
	}
	if slices.Contains(lines, "tracked.go") || strings.Contains(string(b), ".loops") {
		t.Fatalf("baseline has tracked or .loops entries: %q", b)
	}
	dirs, err := os.ReadFile(workspace.BaselineDirs(rd))
	if err != nil || !strings.Contains(string(dirs), "ign\n") || strings.Contains(string(dirs), ".git\n") || strings.Contains(string(dirs), ".loops") {
		t.Fatalf("dirs %q %v", dirs, err)
	}
	os.RemoveAll(workspace.GoDir(rd))
	env, err := s.Env(rd)
	if err != nil || !slices.Equal(env, workspace.Env(rd)) {
		t.Fatal(env, err)
	}
	if !exists(workspace.GoTmp(rd)) || !exists(workspace.GoCache(rd)) {
		t.Fatal("Env must create the go dirs")
	}
}

func TestCleanGitRepo(t *testing.T) {
	needGit(t)
	ctx := context.Background()
	root := project(t)
	outside := project(t)
	write(t, outside, "victim.test", elf, 0o644)
	write(t, outside, "dir/evil.test", elf, 0o644)

	git(t, root, "init", "-q")
	write(t, root, "tracked.go", "package x\n", 0o644)
	write(t, root, "tool.exe", elf, 0o755) // tracked artifact: never touched
	write(t, root, "pkg/a.go", "package pkg\n", 0o644)
	write(t, root, ".gitignore", "*.log\ncoverage.txt\n", 0o644)
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "init")
	write(t, root, "old.test", elf, 0o755)        // pre-existing artifact (baseline)
	write(t, root, "keepdir/old.out", "x", 0o644) // pre-existing dir
	rd := runDir(t, root)
	s := New(root)
	if err := s.Prepare(ctx, rd); err != nil {
		t.Fatal(err)
	}

	// what the agents leave behind
	write(t, root, "pkg/pkg.test", elf, 0o755)
	write(t, root, "coverage.txt", "mode: atomic\n", 0o644) // ignored but an artifact
	write(t, root, "bin/server", elf, 0o755)                // binary without artifact name
	write(t, root, "build/out/app", elf, 0o755)             // new nested dirs, pruned
	write(t, root, "keepdir/new.prof", "p", 0o644)          // keepdir existed: stays
	write(t, root, "run.test", "#!/bin/sh\necho\n", 0o755)  // script: kept
	write(t, root, "notes.md", "# n\n", 0o644)              // text: kept
	write(t, root, "debug.log", "x", 0o644)                 // ignored, not an artifact: kept
	write(t, root, ".loops/runs/r1/proof.test", elf, 0o644) // run dir: untouched
	write(t, root, "tool.exe", elf+"changed", 0o755)        // modified tracked file
	if err := os.Symlink(filepath.Join(outside, "victim.test"), filepath.Join(root, "link.test")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "dir"), filepath.Join(root, "ext")); err != nil {
		t.Fatal(err)
	}
	// go cache with read-only entries
	write(t, workspace.GoCache(rd), "ab/abcdef-d", "cachedata", 0o444)
	if err := os.Chmod(filepath.Join(workspace.GoCache(rd), "ab"), 0o555); err != nil {
		t.Fatal(err)
	}
	write(t, workspace.GoTmp(rd), "go-build1/b001/x", "tmp", 0o644)

	dry, err := s.Clean(ctx, rd, true)
	if err != nil {
		t.Fatal(err)
	}
	if !dry.DryRun || len(dry.Removed) != 5 || dry.CacheFiles != 2 || !exists(filepath.Join(root, "pkg/pkg.test")) || !exists(workspace.GoDir(rd)) {
		t.Fatalf("dry run %+v", dry)
	}

	r, err := s.Clean(ctx, rd, false)
	if err != nil {
		t.Fatal(err)
	}
	var removed []string
	for _, it := range r.Removed {
		removed = append(removed, it.Path)
	}
	wantRemoved := []string{"bin/server", "build/out/app", "coverage.txt", "keepdir/new.prof", "pkg/pkg.test"}
	if !slices.Equal(removed, wantRemoved) {
		t.Fatalf("removed %v", removed)
	}
	for _, k := range []string{"debug.log", "ext", "link.test", "notes.md", "run.test"} {
		if !slices.Contains(r.Kept, k) {
			t.Errorf("kept %v lacks %s", r.Kept, k)
		}
	}
	if r.CacheFiles != 2 || r.CacheBytes != int64(len("cachedata")+len("tmp")) || exists(workspace.GoDir(rd)) {
		t.Fatalf("cache %+v", r)
	}
	for _, p := range []string{"tracked.go", "tool.exe", "old.test", "keepdir/old.out", "keepdir", "pkg", "run.test", "notes.md", "debug.log", "link.test", ".loops/runs/r1/proof.test", ".loops/runs/r1/baseline-untracked.txt"} {
		if !exists(filepath.Join(root, p)) {
			t.Errorf("%s was removed", p)
		}
	}
	for _, p := range []string{"bin", "build/out", "build", "pkg/pkg.test"} {
		if exists(filepath.Join(root, p)) {
			t.Errorf("%s still exists", p)
		}
	}
	if !slices.Equal(r.DirsRemoved, []string{"build/out", "bin", "build"}) {
		t.Errorf("dirs %v", r.DirsRemoved)
	}
	if !exists(filepath.Join(outside, "victim.test")) || !exists(filepath.Join(outside, "dir/evil.test")) {
		t.Fatal("followed a symlink out of the root")
	}
	if b, _ := os.ReadFile(filepath.Join(root, "tool.exe")); string(b) != elf+"changed" {
		t.Fatal("tracked file touched")
	}
	// second clean: nothing new to remove, no cache left
	r2, err := s.Clean(ctx, rd, false)
	if err != nil || len(r2.Removed) != 0 || r2.CacheFiles != 0 {
		t.Fatalf("%+v %v", r2, err)
	}
}

func TestCleanNoGitWalk(t *testing.T) {
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(os.TempDir()))
	ctx := context.Background()
	root := project(t)
	t.Setenv("GIT_DIR", filepath.Join(root, "no-such-git-dir"))
	write(t, root, "main.go", "package main\n", 0o644)
	write(t, root, "old.test", elf, 0o755)
	write(t, root, "vendor/v/v.go", "package v\n", 0o644)
	rd := runDir(t, root)
	s := New(root)
	if err := s.Prepare(ctx, rd); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(workspace.Baseline(rd))
	if !strings.HasPrefix(string(b), headerWalk+"\n") || !strings.Contains(string(b), "main.go\n") || strings.Contains(string(b), "vendor/") {
		t.Fatalf("baseline %q", b)
	}
	write(t, root, "main", elf, 0o755)
	write(t, root, "vendor/v/v.test", elf, 0o755) // skipped tree: never listed
	write(t, root, "new.txt", "hi", 0o644)
	r, err := s.Clean(ctx, rd, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Removed) != 1 || r.Removed[0].Path != "main" || !slices.Equal(r.Kept, []string{"new.txt"}) {
		t.Fatalf("%+v", r)
	}
	if !exists(filepath.Join(root, "old.test")) || !exists(filepath.Join(root, "vendor/v/v.test")) || !exists(filepath.Join(root, "main.go")) {
		t.Fatal("removed a baseline/skipped file")
	}
}

func TestCleanNoBaselineAndModeMismatch(t *testing.T) {
	ctx := context.Background()
	root := project(t)
	rd := runDir(t, root)
	write(t, root, "x.test", elf, 0o755)
	write(t, workspace.GoCache(rd), "f", "12", 0o644)
	s := New(root)
	r, err := s.Clean(ctx, rd, false)
	if err != nil || !r.NoBaseline || len(r.Removed) != 0 || r.CacheFiles != 1 || !exists(filepath.Join(root, "x.test")) {
		t.Fatalf("%+v %v", r, err)
	}
	// a baseline taken by git but a cleanup that cannot use git refuses the scan.
	if err := os.WriteFile(workspace.Baseline(rd), []byte(headerGit+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(os.TempDir()))
	t.Setenv("GIT_DIR", filepath.Join(root, "no-such-git-dir"))
	r, err = s.Clean(ctx, rd, false)
	if err != nil || !r.NoBaseline || len(r.Refused) != 1 || !exists(filepath.Join(root, "x.test")) {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestCleanGoDirSymlinkRefused(t *testing.T) {
	root := project(t)
	outside := project(t)
	write(t, outside, "precious", "x", 0o644)
	rd := runDir(t, root)
	if err := os.Symlink(outside, workspace.GoDir(rd)); err != nil {
		t.Fatal(err)
	}
	r, err := New(root).Clean(context.Background(), rd, false)
	if err != nil || len(r.Refused) == 0 || !exists(filepath.Join(outside, "precious")) {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestRemoveRefusesSwappedParent(t *testing.T) {
	root := project(t)
	outside := project(t)
	write(t, outside, "a.test", elf, 0o644)
	if err := os.Symlink(outside, filepath.Join(root, "sub")); err != nil {
		t.Fatal(err)
	}
	s := New(root)
	e, fi := s.inspect(root, "sub/a.test")
	if e.Kind != workspace.KindOther || fi != nil {
		t.Fatalf("%+v", e)
	}
	write(t, root, "real.test", elf, 0o644)
	e, fi = s.inspect(root, "real.test")
	if e.Kind != workspace.KindFile || workspace.Decide(e) != workspace.Remove {
		t.Fatalf("%+v", e)
	}
	// the file is replaced between inspection and removal: refuse.
	os.Remove(filepath.Join(root, "real.test"))
	write(t, root, "real.test", "other", 0o644)
	if err := s.remove(root, "real.test", fi); err == nil {
		t.Fatal("want refusal for a swapped file")
	}
	if err := s.remove(root, "../x", fi); err == nil {
		t.Fatal("want refusal for an unsafe path")
	}
}

func TestGoAndRunner(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go")
	}
	ctx := context.Background()
	dir := t.TempDir()
	var out, errb bytes.Buffer
	code := New(dir).Go(ctx, dir, []string{"GOCACHE=/loops/x"}, []string{"env", "GOCACHE"}, strings.NewReader(""), &out, &errb)
	if code != 0 || strings.TrimSpace(out.String()) != "/loops/x" {
		t.Fatalf("%d %q %q", code, out.String(), errb.String())
	}
	out.Reset()
	if code := Go(ctx, dir, nil, []string{"definitely-not-a-go-command"}, nil, &out, &errb); code == 0 {
		t.Fatal("want non-zero exit")
	}
	r := NewRunner()
	got, err := r.RunEnv(ctx, dir, []string{"GOTMPDIR=/loops/t"}, "go", "env", "GOTMPDIR")
	if err != nil || strings.TrimSpace(got) != "/loops/t" {
		t.Fatal(got, err)
	}
	if !r.Has("go") || r.Has("definitely-not-a-binary-xyz") {
		t.Fatal("Has")
	}
	if _, err := r.Run(ctx, dir, "go", "version"); err != nil {
		t.Fatal(err)
	}
}

func TestGoMissingBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH semantics")
	}
	t.Setenv("PATH", t.TempDir())
	var errb bytes.Buffer
	if code := Go(context.Background(), t.TempDir(), nil, []string{"version"}, nil, &bytes.Buffer{}, &errb); code != 127 || !strings.Contains(errb.String(), "go") {
		t.Fatal(code, errb.String())
	}
}
