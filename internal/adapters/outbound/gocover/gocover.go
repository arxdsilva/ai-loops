// Package gocover implements gate.Coverage: it reads coverprofiles and
// go.mod, asks git what changed against a base, and inspects package dirs.
package gocover

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/application/gate"
	"github.com/arxdsilva/ai-loops/internal/domain/coverage"
)

// Reader implements gate.Coverage with the filesystem and the git CLI.
type Reader struct{}

// New returns a Reader.
func New() *Reader { return &Reader{} }

var _ gate.Coverage = (*Reader)(nil)

// Profile returns the coverprofile at path.
func (r *Reader) Profile(path string) (string, error) {
	b, err := os.ReadFile(path)
	return string(b), err
}

// Module returns the module path declared in root/go.mod.
func (r *Reader) Module(root string) (string, error) {
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
func (r *Reader) HasTests(root, dir string) bool {
	m, _ := filepath.Glob(filepath.Join(root, filepath.FromSlash(dir), "*_test.go"))
	return len(m) > 0
}

// Generated reports whether root/file starts with a "Code generated … DO
// NOT EDIT." comment (before its package clause).
func (r *Reader) Generated(root, file string) bool {
	f, err := os.Open(filepath.Join(root, filepath.FromSlash(file)))
	if err != nil {
		return false
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
	return coverage.IsGenerated(head.String())
}

// Diff returns `git diff -U0 <merge-base>` for root (paths relative to root)
// and the untracked .go files. The diff is three-dot: it starts at
// merge-base(HEAD, base), so commits on the base branch that this branch
// never touched are not judged. base "" picks the default branch (see
// resolveBase). Diff.Base names the base chosen and the commit diffed from.
func (r *Reader) Diff(ctx context.Context, root, base string) (gate.Diff, error) {
	if _, err := runGit(ctx, root, "rev-parse", "--is-inside-work-tree"); err != nil {
		return gate.Diff{NoGit: true}, nil
	}
	resolved, label, err := resolveBase(ctx, root, base)
	if err != nil {
		return gate.Diff{}, err
	}
	text, err := runGit(ctx, root, "-c", "core.quotepath=on", "diff", "-U0", "-M", "--no-color", "--no-ext-diff",
		"--relative", "--src-prefix=a/", "--dst-prefix=b/", resolved, "--", "*.go")
	if err != nil {
		return gate.Diff{}, err
	}
	others, err := runGit(ctx, root, "ls-files", "-z", "--others", "--exclude-standard", "--", "*.go")
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
