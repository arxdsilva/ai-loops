// Package ghforge implements the backlog's forge port (specs/BACKLOG.md)
// with the GitHub CLI. Arguments go to gh directly, never through a shell,
// and bodies go on stdin, so model-written text is never expanded.
package ghforge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	appbacklog "github.com/arxdsilva/ai-loops/internal/application/backlog"
	"github.com/arxdsilva/ai-loops/internal/domain/backlog"
)

// IssueLimit caps how many open issues one listing reads.
const IssueLimit = 5000

// labelColor is the backlog labels' color.
const labelColor = "5319E7"

// Runner runs gh with args and stdin in the checkout, returning stdout.
type Runner func(ctx context.Context, stdin string, args ...string) ([]byte, error)

// Forge talks to GitHub through gh.
type Forge struct{ gh Runner }

// New returns a Forge running gh in dir.
func New(dir string) *Forge { return &Forge{gh: execRunner(dir)} }

// NewWith returns a Forge on a custom runner (tests).
func NewWith(r Runner) *Forge { return &Forge{gh: r} }

func execRunner(dir string) Runner {
	return func(ctx context.Context, stdin string, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, "gh", args...)
		cmd.Dir = dir
		cmd.Stdin = strings.NewReader(stdin)
		var out, errb bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errb
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("gh %s: %w: %s", strings.Join(args[:min(len(args), 2)], " "), err, strings.TrimSpace(errb.String()))
		}
		return out.Bytes(), nil
	}
}

func (f *Forge) text(ctx context.Context, args ...string) (string, error) {
	out, err := f.gh(ctx, "", args...)
	return strings.TrimSpace(string(out)), err
}

// Repo is the checkout's owner/name.
func (f *Forge) Repo(ctx context.Context) (string, error) {
	return f.text(ctx, "repo", "view", "--json", "nameWithOwner", "-q", ".nameWithOwner")
}

// DefaultBranch is the repository's default branch.
func (f *Forge) DefaultBranch(ctx context.Context, repo string) (string, error) {
	return f.text(ctx, "repo", "view", repo, "--json", "defaultBranchRef", "-q", ".defaultBranchRef.name")
}

// User is the authenticated login.
func (f *Forge) User(ctx context.Context) (string, error) {
	return f.text(ctx, "api", "user", "-q", ".login")
}

type ghIssue struct {
	Number    int     `json:"number"`
	Title     string  `json:"title"`
	URL       string  `json:"url"`
	Author    login   `json:"author"`
	Labels    []name  `json:"labels"`
	Assignees []login `json:"assignees"`
}

type login struct {
	Login string `json:"login"`
}

type name struct {
	Name string `json:"name"`
}

// OpenIssues lists the repository's open issues.
func (f *Forge) OpenIssues(ctx context.Context, repo string) ([]backlog.Issue, error) {
	out, err := f.gh(ctx, "", "issue", "list", "-R", repo, "--state", "open", "--limit", strconv.Itoa(IssueLimit),
		"--json", "number,title,url,author,labels,assignees")
	if err != nil {
		return nil, err
	}
	var raw []ghIssue
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("gh issue list: %w", err)
	}
	issues := make([]backlog.Issue, 0, len(raw))
	for _, r := range raw {
		is := backlog.Issue{Number: r.Number, Title: r.Title, URL: r.URL, Author: r.Author.Login}
		for _, l := range r.Labels {
			is.Labels = append(is.Labels, l.Name)
		}
		for _, a := range r.Assignees {
			is.Assignees = append(is.Assignees, a.Login)
		}
		issues = append(issues, is)
	}
	return issues, nil
}

// Create opens an issue and returns its URL.
func (f *Forge) Create(ctx context.Context, repo string, in appbacklog.NewIssue) (string, error) {
	args := []string{"issue", "create", "-R", repo, "--title", in.Title, "--body-file", "-"}
	for _, l := range in.Labels {
		args = append(args, "--label", l)
	}
	for _, a := range in.Assignees {
		args = append(args, "--assignee", a)
	}
	out, err := f.gh(ctx, in.Body, args...)
	if err != nil {
		return "", err
	}
	lines := strings.Fields(string(out))
	if len(lines) == 0 {
		return "", fmt.Errorf("gh issue create printed no URL")
	}
	return lines[len(lines)-1], nil
}

// Comment adds a comment to issue n.
func (f *Forge) Comment(ctx context.Context, repo string, n int, body string) error {
	_, err := f.gh(ctx, body, "issue", "comment", strconv.Itoa(n), "-R", repo, "--body-file", "-")
	return err
}

// Edit changes issue n's labels and assignees (nothing to change = no call).
func (f *Forge) Edit(ctx context.Context, repo string, n int, e appbacklog.Edit) error {
	args := []string{"issue", "edit", strconv.Itoa(n), "-R", repo}
	before := len(args)
	for _, p := range []struct {
		flag string
		vals []string
	}{{"--add-label", e.AddLabels}, {"--remove-label", e.RemoveLabels}, {"--add-assignee", e.AddAssignees}, {"--remove-assignee", e.RemoveAssignees}} {
		if v := nonEmpty(p.vals); len(v) > 0 {
			args = append(args, p.flag, strings.Join(v, ","))
		}
	}
	if len(args) == before {
		return nil
	}
	_, err := f.gh(ctx, "", args...)
	return err
}

func nonEmpty(v []string) []string {
	var out []string
	for _, s := range v {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// EnsureLabel creates the label or updates its description.
func (f *Forge) EnsureLabel(ctx context.Context, repo, label, description string) error {
	_, err := f.gh(ctx, "", "label", "create", label, "-R", repo, "--description", description, "--color", labelColor, "--force")
	return err
}
