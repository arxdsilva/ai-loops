// Package vcs talks to git and the forge CLIs (gh, glab).
package vcs

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// CLI implements ticket.Issues.
type CLI struct{ root string }

// New returns a CLI rooted at the project.
func New(root string) *CLI { return &CLI{root: root} }

func (c *CLI) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = c.root
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return out.Bytes(), nil
}

// GitHub returns `gh issue view <ref> --json number,title,body,labels,url`.
func (c *CLI) GitHub(ctx context.Context, ref string) ([]byte, error) {
	args := append([]string{"issue", "view"}, repoRef(ref)...)
	return c.run(ctx, "gh", append(args, "--json", "number,title,body,labels,url")...)
}

// GitLab returns `glab issue view <ref> --output json`.
func (c *CLI) GitLab(ctx context.Context, ref string) ([]byte, error) {
	args := append([]string{"issue", "view"}, repoRef(strings.TrimPrefix(ref, "#"))...)
	return c.run(ctx, "glab", append(args, "--output", "json")...)
}

// RemoteURL returns `git remote get-url origin`, "" when none.
func (c *CLI) RemoteURL(ctx context.Context) string {
	out, err := c.run(ctx, "git", "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// repoRef turns "owner/repo#N" into ["-R","owner/repo","N"]; other refs
// (numbers, "#N", URLs) pass through unchanged.
func repoRef(ref string) []string {
	if i := strings.LastIndex(ref, "#"); i > 0 && !strings.Contains(ref, "://") {
		return []string{"-R", ref[:i], ref[i+1:]}
	}
	return []string{ref}
}
