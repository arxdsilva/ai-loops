package ghforge

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	appbacklog "github.com/arxdsilva/ai-loops/internal/application/backlog"
	"github.com/arxdsilva/ai-loops/internal/domain/backlog"
)

type call struct {
	stdin string
	args  []string
}

func recorder(out string, err error) (*[]call, Runner) {
	var calls []call
	return &calls, func(_ context.Context, stdin string, args ...string) ([]byte, error) {
		calls = append(calls, call{stdin, args})
		return []byte(out), err
	}
}

func TestOpenIssues(t *testing.T) {
	calls, r := recorder(`[{"number":7,"title":"T","url":"u","author":{"login":"a"},"labels":[{"name":"P1"},{"name":"bug"}],"assignees":[{"login":"b"}]}]`, nil)
	got, err := NewWith(r).OpenIssues(context.Background(), "o/r")
	want := []backlog.Issue{{Number: 7, Title: "T", URL: "u", Author: "a", Labels: []string{"P1", "bug"}, Assignees: []string{"b"}}}
	if err != nil || len(got) != 1 || got[0].Number != want[0].Number || !slices.Equal(got[0].Labels, want[0].Labels) ||
		!slices.Equal(got[0].Assignees, want[0].Assignees) || got[0].Author != "a" {
		t.Fatalf("issues = %+v, %v", got, err)
	}
	if a := strings.Join((*calls)[0].args, " "); !strings.Contains(a, "issue list -R o/r --state open") {
		t.Fatalf("args = %s", a)
	}
	_, r = recorder("not json", nil)
	if _, err := NewWith(r).OpenIssues(context.Background(), "o/r"); err == nil {
		t.Fatal("bad json accepted")
	}
	_, r = recorder("", errors.New("auth"))
	if _, err := NewWith(r).OpenIssues(context.Background(), "o/r"); err == nil {
		t.Fatal("gh error swallowed")
	}
}

func TestCreateCommentEdit(t *testing.T) {
	calls, r := recorder("Creating issue\nhttps://github.com/o/r/issues/9\n", nil)
	f := NewWith(r)
	ctx := context.Background()
	url, err := f.Create(ctx, "o/r", appbacklog.NewIssue{Title: "A `b` $c", Body: "body $(x)", Labels: []string{"l"}, Assignees: []string{"h"}})
	if err != nil || url != "https://github.com/o/r/issues/9" {
		t.Fatalf("create = %q, %v", url, err)
	}
	c := (*calls)[0]
	if c.stdin != "body $(x)" || !slices.Contains(c.args, "A `b` $c") || !slices.Contains(c.args, "--body-file") ||
		strings.Join(c.args[len(c.args)-4:], " ") != "--label l --assignee h" {
		t.Fatalf("create call = %+v", c)
	}
	if err := f.Comment(ctx, "o/r", 3, "hi"); err != nil || (*calls)[1].stdin != "hi" || (*calls)[1].args[2] != "3" {
		t.Fatalf("comment = %+v, %v", (*calls)[1], err)
	}
	if err := f.Edit(ctx, "o/r", 3, appbacklog.Edit{}); err != nil || len(*calls) != 2 {
		t.Fatal("empty edit should not call gh")
	}
	if err := f.Edit(ctx, "o/r", 3, appbacklog.Edit{AddLabels: []string{"a", "", "b"}, RemoveAssignees: []string{"h"}}); err != nil {
		t.Fatal(err)
	}
	if a := strings.Join((*calls)[2].args, " "); a != "issue edit 3 -R o/r --add-label a,b --remove-assignee h" {
		t.Fatalf("edit args = %s", a)
	}
	if err := f.EnsureLabel(ctx, "o/r", "x", "d"); err != nil || !slices.Contains((*calls)[3].args, "--force") {
		t.Fatalf("label = %+v, %v", (*calls)[3], err)
	}
	_, r = recorder("", nil)
	if _, err := NewWith(r).Create(ctx, "o/r", appbacklog.NewIssue{}); err == nil {
		t.Fatal("no URL accepted")
	}
}

func TestRepoUserBranch(t *testing.T) {
	calls, r := recorder(" o/r \n", nil)
	f := NewWith(r)
	ctx := context.Background()
	if v, err := f.Repo(ctx); v != "o/r" || err != nil {
		t.Fatalf("repo = %q, %v", v, err)
	}
	f.User(ctx)
	f.DefaultBranch(ctx, "o/r")
	if (*calls)[1].args[0] != "api" || (*calls)[2].args[2] != "o/r" {
		t.Fatalf("calls = %+v", *calls)
	}
}

func TestExecRunner(t *testing.T) {
	// gh may be absent: either way the runner must return, not hang, and
	// errors must name the command.
	_, err := execRunner(t.TempDir())(context.Background(), "", "definitely-not-a-gh-command")
	if err != nil && !strings.Contains(err.Error(), "gh definitely-not-a-gh-command") {
		t.Fatalf("error = %v", err)
	}
	_ = New(t.TempDir())
}
