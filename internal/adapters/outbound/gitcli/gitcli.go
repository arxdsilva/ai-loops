// Package gitcli implements release.Git with the git CLI in the repo root.
package gitcli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// gitCLI runs git in Root.
type gitCLI struct{ root string }

// New returns a gitCLI rooted at the repository.
func New(root string) *gitCLI { return &gitCLI{root: root} }

func (g *gitCLI) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.root
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return out.String(), nil
}

// LastTag is the newest v<digit>* tag reachable from HEAD ("" when none).
func (g *gitCLI) LastTag(ctx context.Context) (string, error) {
	out, err := g.run(ctx, "tag", "--list", "v[0-9]*", "--merged", "HEAD", "--sort=-v:refname")
	if err != nil {
		return "", err
	}
	first, _, _ := strings.Cut(strings.TrimSpace(out), "\n")
	return strings.TrimSpace(first), nil
}

// CommitsSince returns each commit's full message since tag (all history
// when tag is "").
func (g *gitCLI) CommitsSince(ctx context.Context, tag string) ([]string, error) {
	args := []string{"log", "--format=%B%x00"}
	if tag != "" {
		args = append(args, tag+"..HEAD")
	}
	out, err := g.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	var msgs []string
	for _, m := range strings.Split(out, "\x00") {
		if m = strings.TrimSpace(m); m != "" {
			msgs = append(msgs, m)
		}
	}
	return msgs, nil
}

// HeadTags lists the tags pointing at HEAD.
func (g *gitCLI) HeadTags(ctx context.Context) ([]string, error) {
	out, err := g.run(ctx, "tag", "--points-at", "HEAD")
	if err != nil {
		return nil, err
	}
	return strings.Fields(out), nil
}

// HeadSubject is HEAD's subject line.
func (g *gitCLI) HeadSubject(ctx context.Context) (string, error) {
	out, err := g.run(ctx, "log", "-1", "--format=%s")
	return strings.TrimSpace(out), err
}

// Dirty reports uncommitted changes (untracked files included).
func (g *gitCLI) Dirty(ctx context.Context) (bool, error) {
	out, err := g.run(ctx, "status", "--porcelain")
	return strings.TrimSpace(out) != "", err
}

// Commit stages paths (those that exist) and commits only them.
func (g *gitCLI) Commit(ctx context.Context, msg string, paths []string) error {
	var existing []string
	for _, p := range paths {
		if _, err := os.Stat(filepath.Join(g.root, filepath.FromSlash(p))); err == nil {
			existing = append(existing, p)
		}
	}
	if len(existing) == 0 {
		return fmt.Errorf("git commit: none of the release paths exist")
	}
	if _, err := g.run(ctx, append([]string{"add", "--"}, existing...)...); err != nil {
		return err
	}
	_, err := g.run(ctx, append([]string{"commit", "-m", msg, "--"}, existing...)...)
	return err
}

// Tag creates an annotated tag at HEAD.
func (g *gitCLI) Tag(ctx context.Context, tag, msg string) error {
	_, err := g.run(ctx, "tag", "-a", tag, "-m", msg)
	return err
}

// TagExists reports whether the tag exists locally (fetched tags included).
func (g *gitCLI) TagExists(ctx context.Context, tag string) (bool, error) {
	out, err := g.run(ctx, "tag", "--list", tag)
	return strings.TrimSpace(out) != "", err
}

// Push pushes refs to origin atomically, so a rejected branch update never
// leaves its tag behind.
func (g *gitCLI) Push(ctx context.Context, ref ...string) error {
	_, err := g.run(ctx, append([]string{"push", "--atomic", "origin"}, ref...)...)
	return err
}
