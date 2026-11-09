// Package runspace manages a run's scratch space on disk: it snapshots the
// project's untracked files at run start, scopes go commands to a run-owned
// GOCACHE/GOTMPDIR, and at run end removes that cache and the build
// artifacts agents left behind. The decisions (what is an artifact, which
// paths are safe) live in domain/workspace; this adapter only does I/O, and
// re-checks every path before deleting it:
//
//   - nothing outside the project root (each parent is resolved and must be
//     the lexical parent: no symlinked directories on the way);
//   - never a symlink (Lstat only; the go cache walk never follows links);
//   - never a git-tracked file (paths come from `git ls-files --others` and
//     are checked against `git ls-files` again);
//   - never a file that changed identity between inspection and removal.
package runspace

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/workspace"
)

// Baseline headers record how the snapshot was taken; a cleanup that cannot
// list the tree the same way refuses to scan.
const (
	headerGit  = "# loops baseline v1 git"
	headerWalk = "# loops baseline v1 walk"
)

// Space is the scratch-space adapter for the project at root.
type Space struct{ root string }

// New returns a Space for the project at root.
func New(root string) *Space { return &Space{root: root} }

// Prepare creates the run's go cache directories and writes the baseline
// snapshots (untracked files incl. ignored ones, and the directory list).
func (s *Space) Prepare(ctx context.Context, runDir string) error {
	if _, err := s.Env(runDir); err != nil {
		return err
	}
	paths, header, err := s.untracked(ctx)
	if err != nil {
		return err
	}
	if err := writeLines(workspace.Baseline(runDir), header, paths); err != nil {
		return err
	}
	dirs, err := s.dirs()
	if err != nil {
		return err
	}
	return writeLines(workspace.BaselineDirs(runDir), "# loops baseline-dirs v1", dirs)
}

// Env creates the run's GOCACHE and GOTMPDIR (go refuses a missing
// GOTMPDIR) and returns the environment entries that select them.
func (s *Space) Env(runDir string) ([]string, error) {
	for _, d := range []string{workspace.GoCache(runDir), workspace.GoTmp(runDir)} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, err
		}
	}
	return workspace.Env(runDir), nil
}

func writeLines(path, header string, lines []string) error {
	var b bytes.Buffer
	b.WriteString(header + "\n")
	for _, l := range lines {
		if strings.ContainsAny(l, "\n\r") {
			continue // unrepresentable; such paths are never SafeRel anyway
		}
		b.WriteString(l + "\n")
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b.Bytes(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// readLines returns the header line and the entries of a snapshot file.
func readLines(path string) (string, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	header := ""
	var out []string
	for first := true; sc.Scan(); first = false {
		if first {
			header = sc.Text()
			continue
		}
		if l := sc.Text(); l != "" {
			out = append(out, l)
		}
	}
	return header, out, sc.Err()
}

func (s *Space) git(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", s.root}, args...)...)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return out, nil
}

func (s *Space) isGit(ctx context.Context) bool {
	out, err := s.git(ctx, "rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

func splitNUL(b []byte) []string {
	var out []string
	for _, p := range strings.Split(string(b), "\x00") {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// untracked lists the files git does not track (ignored ones included),
// relative to root, without .loops/; outside git it walks the tree.
func (s *Space) untracked(ctx context.Context) ([]string, string, error) {
	if !s.isGit(ctx) {
		paths, err := s.walkFiles()
		return paths, headerWalk, err
	}
	out, err := s.git(ctx, "ls-files", "--others", "-z")
	if err != nil {
		return nil, "", err
	}
	var paths []string
	for _, p := range splitNUL(out) {
		if p != ".loops" && !strings.HasPrefix(p, ".loops/") {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	return paths, headerGit, nil
}

func (s *Space) tracked(ctx context.Context) (map[string]bool, error) {
	out, err := s.git(ctx, "ls-files", "-z")
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, p := range splitNUL(out) {
		set[p] = true
	}
	return set, nil
}

// walk visits the tree under root without following symlinks, skipping
// workspace.SkipDir trees; fn gets slash-relative paths.
func (s *Space) walk(fn func(rel string, d fs.DirEntry)) error {
	return filepath.WalkDir(s.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == s.root {
				return err
			}
			return nil // unreadable subtree: not ours to judge
		}
		rel, rerr := filepath.Rel(s.root, p)
		if rerr != nil || rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() && workspace.SkipDir(rel) {
			return filepath.SkipDir
		}
		fn(rel, d)
		return nil
	})
}

func (s *Space) walkFiles() ([]string, error) {
	var out []string
	err := s.walk(func(rel string, d fs.DirEntry) {
		if !d.IsDir() {
			out = append(out, rel)
		}
	})
	return out, err
}

func (s *Space) dirs() ([]string, error) {
	var out []string
	err := s.walk(func(rel string, d fs.DirEntry) {
		if d.IsDir() {
			out = append(out, rel)
		}
	})
	return out, err
}

// Clean removes the run's go directory and the new artifacts in the
// project (dryRun only reports). Problems with single paths are reported in
// Report.Refused; the error is for failures that stop the cleanup.
func (s *Space) Clean(ctx context.Context, runDir string, dryRun bool) (workspace.Report, error) {
	r := workspace.Report{DryRun: dryRun, Removed: []workspace.Item{}, Kept: []string{}}
	s.cleanGo(runDir, dryRun, &r)
	header, base, err := readLines(workspace.Baseline(runDir))
	if errors.Is(err, fs.ErrNotExist) {
		r.NoBaseline = true
		return r, nil
	}
	if err != nil {
		return r, err
	}
	cur, curHeader, err := s.untracked(ctx)
	if err != nil {
		return r, err
	}
	if header != curHeader {
		r.NoBaseline = true
		r.Refused = append(r.Refused, fmt.Sprintf("project scan: baseline taken as %q, now %q", header, curHeader))
		return r, nil
	}
	tracked := map[string]bool{}
	if curHeader == headerGit {
		if tracked, err = s.tracked(ctx); err != nil {
			return r, err
		}
	}
	realRoot, err := filepath.EvalSymlinks(s.root)
	if err != nil {
		return r, err
	}
	fresh, unsafe := workspace.NewPaths(base, cur)
	r.Kept = append(r.Kept, unsafe...)
	var removed []string
	for _, rel := range fresh {
		if tracked[rel] {
			continue
		}
		e, fi := s.inspect(realRoot, rel)
		if workspace.Decide(e) != workspace.Remove {
			r.Kept = append(r.Kept, rel)
			continue
		}
		if !dryRun {
			if err := s.remove(realRoot, rel, fi); err != nil {
				r.Refused = append(r.Refused, rel+": "+err.Error())
				continue
			}
		}
		r.Removed = append(r.Removed, workspace.Item{Path: rel, Bytes: e.Size})
		removed = append(removed, rel)
	}
	sort.Strings(r.Kept)
	if !dryRun {
		r.DirsRemoved = s.prune(realRoot, runDir, removed)
	}
	return r, nil
}

// prune removes the emptied directories that did not exist at run start.
func (s *Space) prune(realRoot, runDir string, removed []string) []string {
	_, dirs, err := readLines(workspace.BaselineDirs(runDir))
	if err != nil {
		return nil // without a directory snapshot nothing is pruned
	}
	base := map[string]bool{}
	for _, d := range dirs {
		base[d] = true
	}
	var out []string
	for _, rel := range workspace.PruneDirs(removed, base) {
		full, ok := contained(realRoot, rel)
		if !ok {
			continue
		}
		if fi, err := os.Lstat(full); err != nil || !fi.IsDir() {
			continue
		}
		if os.Remove(full) == nil { // fails unless empty
			out = append(out, rel)
		}
	}
	return out
}

// contained maps a safe relative path to its absolute path, and reports
// whether its parent directory resolves to exactly the lexical parent (no
// symlinked directory on the way out of the root).
func contained(realRoot, rel string) (string, bool) {
	if !workspace.SafeRel(rel) {
		return "", false
	}
	full := filepath.Join(realRoot, filepath.FromSlash(rel))
	parent := filepath.Dir(full)
	got, err := filepath.EvalSymlinks(parent)
	if err != nil || got != parent {
		return "", false
	}
	return full, true
}

// inspect describes rel without following symlinks. The FileInfo is the
// Lstat result used later to make sure the same file is removed.
func (s *Space) inspect(realRoot, rel string) (workspace.Entry, os.FileInfo) {
	e := workspace.Entry{Rel: rel, Kind: workspace.KindOther}
	if strings.HasSuffix(rel, "/") {
		e.Kind = workspace.KindDir
		return e, nil
	}
	full, ok := contained(realRoot, rel)
	if !ok {
		return e, nil
	}
	fi, err := os.Lstat(full)
	if err != nil {
		return e, nil
	}
	switch m := fi.Mode(); {
	case m&fs.ModeSymlink != 0:
		e.Kind = workspace.KindSymlink
		return e, fi
	case m.IsDir():
		e.Kind = workspace.KindDir
		return e, fi
	case !m.IsRegular():
		return e, fi
	}
	f, err := os.Open(full)
	if err != nil {
		return e, fi
	}
	defer f.Close()
	if ofi, err := f.Stat(); err != nil || !unchanged(fi, ofi) {
		return e, fi // swapped under us
	}
	head := make([]byte, workspace.HeadSize)
	n, _ := io.ReadFull(f, head)
	e.Kind, e.Size, e.Head = workspace.KindFile, fi.Size(), head[:n]
	return e, fi
}

// remove deletes rel after re-checking it is still the inspected regular
// file inside the root.
func (s *Space) remove(realRoot, rel string, inspected os.FileInfo) error {
	full, ok := contained(realRoot, rel)
	if !ok || inspected == nil {
		return errors.New("refusing a path outside the project root")
	}
	fi, err := os.Lstat(full)
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() || !unchanged(fi, inspected) {
		return errors.New("file changed since it was inspected")
	}
	return os.Remove(full)
}

// unchanged reports whether a and b describe the same, unmodified file.
// os.SameFile alone is not enough: Linux filesystems reuse a deleted file's
// inode for the next file created, so a swap can keep the device and inode.
// Size and modification time must match too.
func unchanged(a, b os.FileInfo) bool {
	return os.SameFile(a, b) && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

// cleanGo removes <run dir>/go after making its directories writable (go
// cache entries can be read-only). It never follows symlinks and refuses a
// go dir that is a symlink or resolves outside the run dir.
func (s *Space) cleanGo(runDir string, dryRun bool, r *workspace.Report) {
	goDir := workspace.GoDir(runDir)
	fi, err := os.Lstat(goDir)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err != nil || !fi.IsDir() || fi.Mode()&fs.ModeSymlink != 0 {
		r.Refused = append(r.Refused, goDir+": not a plain directory")
		return
	}
	realRun, err1 := filepath.EvalSymlinks(runDir)
	realGo, err2 := filepath.EvalSymlinks(goDir)
	if err1 != nil || err2 != nil || filepath.Dir(realGo) != realRun {
		r.Refused = append(r.Refused, goDir+": outside the run directory")
		return
	}
	_ = filepath.WalkDir(realGo, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if !dryRun {
				if info, err := d.Info(); err == nil && info.Mode().Perm()&0o700 != 0o700 {
					_ = os.Chmod(p, info.Mode().Perm()|0o700)
				}
			}
			return nil
		}
		r.CacheFiles++
		if info, err := d.Info(); err == nil && info.Mode().IsRegular() {
			r.CacheBytes += info.Size()
			if !dryRun && info.Mode().Perm()&0o200 == 0 {
				_ = os.Chmod(p, info.Mode().Perm()|0o200) // read-only files block removal on Windows
			}
		}
		return nil
	})
	if dryRun {
		return
	}
	if err := os.RemoveAll(realGo); err != nil {
		r.Refused = append(r.Refused, goDir+": "+err.Error())
	}
}

// Go runs the go tool for the CLI (see the package-level Go).
func (s *Space) Go(ctx context.Context, dir string, env, args []string, in io.Reader, out, errw io.Writer) int {
	return Go(ctx, dir, env, args, in, out, errw)
}

// Go runs the go tool with args in dir ("" = the current directory), adding env to the process
// environment, wiring stdio through, and returns its exit code (127 when go
// cannot be started).
func Go(ctx context.Context, dir string, env, args []string, in io.Reader, out, errw io.Writer) int {
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = in, out, errw
	err := cmd.Run()
	var ee *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &ee):
		return ee.ExitCode()
	}
	fmt.Fprintf(errw, "loops go: %v\n", err)
	return 127
}

// Runner runs the gate's external commands, optionally with extra
// environment (gate.Commander and gate.EnvCommander).
type Runner struct{}

// NewRunner returns a Runner.
func NewRunner() *Runner { return &Runner{} }

// Run executes name with args in dir, returning combined output.
func (r *Runner) Run(ctx context.Context, dir, name string, args ...string) (string, error) {
	return r.RunEnv(ctx, dir, nil, name, args...)
}

// RunEnv is Run with env appended to the process environment.
func (r *Runner) RunEnv(ctx context.Context, dir string, env []string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Has reports whether name is on PATH.
func (r *Runner) Has(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
