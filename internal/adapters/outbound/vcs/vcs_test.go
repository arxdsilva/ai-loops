package vcs

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func fakeBin(t *testing.T, name, script string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestForgeCommands(t *testing.T) {
	fakeBin(t, "gh", `echo "gh $@"`)
	fakeBin(t, "glab", `echo "glab $@"; echo oops >&2; [ "$2" = "view" ] && [ "$3" = "9" ] && exit 1; exit 0`)
	c := New(t.TempDir())
	ctx := context.Background()
	out, err := c.GitHub(ctx, "#5")
	if err != nil || strings.TrimSpace(string(out)) != "gh issue view #5 --json number,title,body,labels,url" {
		t.Fatalf("%q %v", out, err)
	}
	out, err = c.GitLab(ctx, "#4")
	if err != nil || !strings.Contains(string(out), "glab issue view 4 --output json") || strings.Contains(string(out), "oops") {
		t.Fatalf("%q %v", out, err)
	}
	if _, err = c.GitLab(ctx, "9"); err == nil || !strings.Contains(err.Error(), "oops") {
		t.Fatalf("%v", err)
	}
}

func TestRemoteURL(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	dir := t.TempDir()
	c := New(dir)
	if c.RemoteURL(context.Background()) != "" {
		t.Fatal("expected empty outside a repo")
	}
	for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", "git@github.com:a/b.git"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatal(string(out), err)
		}
	}
	if got := c.RemoteURL(context.Background()); got != "git@github.com:a/b.git" {
		t.Fatal(got)
	}
}

func TestOwnerRepoRef(t *testing.T) {
	fakeBin(t, "gh", `echo "gh $@"`)
	fakeBin(t, "glab", `echo "glab $@"`)
	c := New(t.TempDir())
	out, _ := c.GitHub(context.Background(), "cli/cli#12")
	if !strings.Contains(string(out), "issue view -R cli/cli 12 --json") {
		t.Fatalf("%q", out)
	}
	out, _ = c.GitLab(context.Background(), "g/p#3")
	if !strings.Contains(string(out), "issue view -R g/p 3 --output json") {
		t.Fatalf("%q", out)
	}
	out, _ = c.GitHub(context.Background(), "https://github.com/o/r/issues/1#x")
	if !strings.Contains(string(out), "view https://github.com/o/r/issues/1#x --json") {
		t.Fatalf("%q", out)
	}
}
