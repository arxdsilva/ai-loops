// Package gocover implements the gate's host ports: gate.Coverage and
// gate.PackCoverage (coverage reports, go.mod, what git says changed against
// a base, test and generated files for any language pack), gate.Files and
// gate.EnvLookup.
package gocover

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/application/gate"
	"github.com/arxdsilva/ai-loops/internal/domain/coverage"
	"github.com/arxdsilva/ai-loops/internal/domain/glob"
)

// The reader implements every gate host port.
var (
	_ gate.Coverage     = (*reader)(nil)
	_ gate.PackCoverage = (*reader)(nil)
	_ gate.Files        = (*reader)(nil)
	_ gate.RealPaths    = (*reader)(nil)
	_ gate.EnvLookup    = (*reader)(nil)
)

// reader implements gate.Coverage with the filesystem and the git CLI.
type reader struct{}

// New returns a reader.
func New() *reader { return &reader{} }

// Profile returns the coverprofile at path.
func (r *reader) Profile(path string) (string, error) {
	b, err := os.ReadFile(path)
	return string(b), err
}

// Module returns the module path declared in root/go.mod.
func (r *reader) Module(root string) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	for _, l := range strings.Split(string(b), "\n") {
		l, _, _ = strings.Cut(l, "//")
		f := strings.Fields(l)
		if len(f) == 2 && f[0] == "module" {
			if u, err := strconv.Unquote(f[1]); err == nil {
				return u, nil
			}
			return f[1], nil
		}
	}
	return "", fmt.Errorf("%s: no module line", filepath.Join(root, "go.mod"))
}

// HasTests reports whether root/dir holds a _test.go file.
func (r *reader) HasTests(root, dir string) bool {
	m, _ := filepath.Glob(filepath.Join(root, filepath.FromSlash(dir), "*_test.go"))
	return len(m) > 0
}

// HasTestFiles reports whether root/dir directly holds a file whose
// root-relative slash path matches one of testGlobs.
func (r *reader) HasTestFiles(root, dir string, testGlobs []string) bool {
	es, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
	if err != nil {
		return false
	}
	for _, e := range es {
		if !e.Type().IsRegular() {
			continue
		}
		if glob.Any(testGlobs, path.Join(dir, e.Name())) {
			return true
		}
	}
	return false
}

// Head returns root/file's opening lines (at most 50, up to and including
// a "package " clause), for pack.IsGenerated.
func (r *reader) Head(root, file string) string {
	f, err := os.Open(filepath.Join(root, filepath.FromSlash(file)))
	if err != nil {
		return ""
	}
	defer f.Close()
	var head strings.Builder
	sc := bufio.NewScanner(f)
	for n := 0; n < 50 && sc.Scan(); n++ {
		head.WriteString(sc.Text() + "\n")
		if strings.HasPrefix(sc.Text(), "package ") {
			break
		}
	}
	return head.String()
}

// Generated reports whether root/file starts with a "Code generated … DO
// NOT EDIT." comment (before its package clause).
func (r *reader) Generated(root, file string) bool {
	return coverage.IsGenerated(r.Head(root, file))
}

// Diff returns `git diff -U0 <merge-base>` for root (paths relative to root)
// and the untracked .go files. The diff is three-dot: it starts at
// merge-base(HEAD, base), so commits on the base branch that this branch
// never touched are not judged. base "" picks the default branch (see
// resolveBase). Diff.Base names the base chosen and the commit diffed from.
func (r *reader) Diff(ctx context.Context, root, base string) (gate.Diff, error) {
	return r.DiffGlobs(ctx, root, base, []string{"**/*.go"})
}

// DiffGlobs is Diff for the files matching globs (a pack's source_globs,
// root-relative): paths in the result are relative to root, so a pack in a
// subdirectory gets pack-relative paths.
func (r *reader) DiffGlobs(ctx context.Context, root, base string, globs []string) (gate.Diff, error) {
	specs := pathspecs(globs)
	if _, err := runGit(ctx, root, "rev-parse", "--is-inside-work-tree"); err != nil {
		return gate.Diff{NoGit: true}, nil
	}
	resolved, label, err := resolveBase(ctx, root, base)
	if err != nil {
		return gate.Diff{}, err
	}
	text, err := runGit(ctx, root, append([]string{"-c", "core.quotepath=on", "diff", "-U0", "-M", "--no-color", "--no-ext-diff",
		"--relative", "--src-prefix=a/", "--dst-prefix=b/", resolved, "--"}, specs...)...)
	if err != nil {
		return gate.Diff{}, err
	}
	others, err := runGit(ctx, root, append([]string{"ls-files", "-z", "--others", "--exclude-standard", "--"}, specs...)...)
	if err != nil {
		return gate.Diff{}, err
	}
	var untracked []string
	for _, f := range strings.Split(others, "\x00") {
		if f != "" {
			untracked = append(untracked, filepath.ToSlash(f))
		}
	}
	sort.Strings(untracked)
	return gate.Diff{Base: label, Text: text, Untracked: untracked}, nil
}

// pathspecs turns root-relative globs into git pathspecs: "**/*.ext" is
// the classic "*.ext" (whose "*" spans directories); anything else uses
// git's glob magic, where "**/" also matches no directory.
func pathspecs(globs []string) []string {
	var out []string
	for _, g := range globs {
		if rest, ok := strings.CutPrefix(g, "**/"); ok && !strings.ContainsAny(rest, "/[?") && strings.Count(rest, "*") == 1 && strings.HasPrefix(rest, "*") {
			out = append(out, rest)
			continue
		}
		if !strings.Contains(g, "/") {
			g = "**/" + g // a pattern without "/" matches the base name anywhere
		}
		out = append(out, ":(glob)"+g)
	}
	if len(out) == 0 {
		out = []string{"."}
	}
	return out
}

// resolveBase returns the commit to diff from and a label naming it. An
// explicit base diffs from merge-base(HEAD, base); a commit that is already
// an ancestor (a push's before-SHA) is its own merge-base. With no base, it
// tries the default branch in order: origin/HEAD's target, origin/main,
// origin/master, main, master; then HEAD~1; then the empty tree.
func resolveBase(ctx context.Context, root, base string) (string, string, error) {
	if base != "" {
		if _, err := runGit(ctx, root, "rev-parse", "--verify", "--quiet", base+"^{commit}"); err != nil {
			return "", "", fmt.Errorf("unknown diff base %q", base)
		}
		mb, err := runGit(ctx, root, "merge-base", "HEAD", base)
		if err != nil {
			return "", "", fmt.Errorf("no merge-base between HEAD and %q: the branch shares no history with it", base)
		}
		mb = strings.TrimSpace(mb)
		return mb, base + " (merge-base " + short(mb) + ")", nil
	}
	for _, ref := range defaultBranches(ctx, root) {
		if mb, err := runGit(ctx, root, "merge-base", "HEAD", ref); err == nil {
			mb = strings.TrimSpace(mb)
			return mb, ref + " (merge-base " + short(mb) + ")", nil
		}
	}
	if out, err := runGit(ctx, root, "rev-parse", "--verify", "--quiet", "HEAD~1^{commit}"); err == nil {
		rev := strings.TrimSpace(out)
		return rev, "HEAD~1 (" + short(rev) + "; no default branch found)", nil
	}
	// No history to compare with: everything is new.
	out, err := runGit(ctx, root, "hash-object", "-t", "tree", os.DevNull)
	return strings.TrimSpace(out), "empty tree (no history: every file is new)", err
}

// defaultBranches lists the candidate default branches, best first.
func defaultBranches(ctx context.Context, root string) []string {
	var refs []string
	if out, err := runGit(ctx, root, "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD"); err == nil {
		if ref, ok := strings.CutPrefix(strings.TrimSpace(out), "refs/remotes/"); ok {
			refs = append(refs, ref)
		}
	}
	return append(refs, "origin/main", "origin/master", "main", "master")
}

func short(rev string) string {
	if len(rev) > 12 {
		return rev[:12]
	}
	return rev
}

func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		return "", errors.New("git " + strings.Join(args, " ") + ": " + err.Error() + ": " + msg)
	}
	return out.String(), nil
}

// Exists reports whether p exists.
func (r *reader) Exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// RealPath returns p with its symlinks resolved (gate.RealPaths).
func (r *reader) RealPath(p string) (string, error) { return filepath.EvalSymlinks(p) }

// ReadFile returns p's content.
func (r *reader) ReadFile(p string) (string, error) {
	b, err := os.ReadFile(p)
	return string(b), err
}

// WriteFile writes data to p.
func (r *reader) WriteFile(p, data string) error { return os.WriteFile(p, []byte(data), 0o644) }

// Remove deletes the file p; a missing file is fine.
func (r *reader) Remove(p string) error {
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// MkdirAll creates dir and its parents.
func (r *reader) MkdirAll(dir string) error { return os.MkdirAll(dir, 0o755) }

// LookupEnv reads the process environment.
func (r *reader) LookupEnv(k string) (string, bool) { return os.LookupEnv(k) }

// Walk calls fn with the root-relative slash path of each regular file
// under root, down to depth directory levels (<0 = unlimited), skipping the
// directories skip names; fn returning true stops the walk. Unreadable
// entries are skipped; a root that is not a directory yields nothing.
func (r *reader) Walk(root string, depth int, skip func(name string) bool, fn func(rel string) bool) error {
	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		return nil
	}
	stop := errors.New("stop")
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() && p != root {
				return fs.SkipDir
			}
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil || rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if (skip != nil && skip(d.Name())) || (depth >= 0 && strings.Count(rel, "/")+1 > depth) {
				return fs.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() && fn(rel) {
			return stop
		}
		return nil
	})
	if errors.Is(err, stop) {
		return nil
	}
	return err
}
