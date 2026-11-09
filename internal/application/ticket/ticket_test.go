package ticket

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type fakeIssues struct {
	gh, gl []byte
	err    error
}

func (f fakeIssues) GitHub(context.Context, string) ([]byte, error) { return f.gh, f.err }
func (f fakeIssues) GitLab(context.Context, string) ([]byte, error) { return f.gl, f.err }

func TestDetect(t *testing.T) {
	gl := "git@gitlab.com:g/p.git"
	cases := []struct{ ref, remote, want string }{
		{"", "", SourceText},
		{"#123", "", SourceGitHub},
		{"123", "git@github.com:a/b.git", SourceGitHub},
		{"#123", gl, SourceGitLab},
		{"owner/repo#5", "", SourceGitHub},
		{"group/proj#12", gl, SourceGitLab},
		{"https://github.com/o/r/issues/1", gl, SourceGitHub},
		{"https://gitlab.example.com/g/p/-/issues/3", "", SourceGitLab},
		{"ABC-123", "", SourceJira},
		{"https://linear.app/t/issue/LIN-1", "", SourceLinear},
		{"fix the thing", "", SourceText},
	}
	for _, c := range cases {
		if got := Detect(c.ref, c.remote); got != c.want {
			t.Errorf("Detect(%q,%q)=%s want %s", c.ref, c.remote, got, c.want)
		}
	}
}

func TestFetchGitHubAndGitLab(t *testing.T) {
	ctx := context.Background()
	s := New(fakeIssues{
		gh: []byte(`{"number":7,"title":"T","body":"intro\n## Acceptance criteria\n- first\n- AC-2: second\n","url":"http://gh/7","labels":[{"name":"bug"},{"name":"p1"}]}`),
		gl: []byte(`{"iid":3,"title":"G","description":"- [ ] a\n- [x] b","web_url":"http://gl/3","labels":["x","y"]}`),
	})
	tk, err := s.Fetch(ctx, SourceGitHub, "#7", "")
	if err != nil || tk.Title != "T" || tk.URL != "http://gh/7" || !reflect.DeepEqual(tk.Labels, []string{"bug", "p1"}) ||
		!reflect.DeepEqual(tk.Criteria, []string{"first", "second"}) || tk.Ref != "#7" || tk.Source != SourceGitHub {
		t.Fatalf("%+v %v", tk, err)
	}
	tk, err = s.Fetch(ctx, SourceGitLab, "3", "")
	if err != nil || tk.Title != "G" || tk.URL != "http://gl/3" || tk.Body != "- [ ] a\n- [x] b" || !reflect.DeepEqual(tk.Criteria, []string{"a", "b"}) || !reflect.DeepEqual(tk.Labels, []string{"x", "y"}) {
		t.Fatalf("%+v %v", tk, err)
	}
}

func TestFetchErrors(t *testing.T) {
	ctx := context.Background()
	s := New(fakeIssues{err: errors.New("gh down"), gh: []byte("{")})
	if _, err := s.Fetch(ctx, SourceGitHub, "#1", ""); err == nil {
		t.Fatal("issues err")
	}
	s = New(fakeIssues{gh: []byte("{")})
	if _, err := s.Fetch(ctx, SourceGitHub, "#1", ""); err == nil {
		t.Fatal("bad json")
	}
	if _, err := s.Fetch(ctx, SourceGitHub, "", ""); err == nil {
		t.Fatal("no ref")
	}
	if _, err := s.Fetch(ctx, SourceText, "", "  \n"); err == nil {
		t.Fatal("empty body")
	}
	if _, err := s.Fetch(ctx, "bogus", "", "x"); err == nil {
		t.Fatal("source")
	}
}

func TestFetchText(t *testing.T) {
	s := New(fakeIssues{})
	ctx := context.Background()
	tk, err := s.Fetch(ctx, SourceJira, "ABC-1", "\nAdd orders\nsome detail\n\nAC:\n1. does x\n2. does y\n\n## Notes\n- not ac\n")
	if err != nil || tk.Title != "Add orders" || tk.Ref != "ABC-1" || !reflect.DeepEqual(tk.Criteria, []string{"does x", "does y"}) {
		t.Fatalf("%+v %v", tk, err)
	}
	tk, _ = s.Fetch(ctx, SourceText, "", "preamble\n# Real Title\nbody")
	if tk.Title != "Real Title" || !strings.Contains(tk.Body, "preamble") || strings.Contains(tk.Body, "Real Title") {
		t.Fatalf("%+v", tk)
	}
}

func TestMarkdown(t *testing.T) {
	got := Markdown(Ticket{Source: "github", Ref: "#1", Title: "T", URL: "http://u", Labels: []string{"a", "b"}, Criteria: []string{"x", "y"}, Body: "B"})
	want := "# T\n\nSource: github · Ref: #1 · http://u\nLabels: a, b\n\n## Acceptance criteria\n- AC-1: x\n- AC-2: y\n\n## Body\nB\n"
	if got != want {
		t.Fatalf("%q", got)
	}
	got = Markdown(Ticket{Source: "text", Title: "T", Body: "B"})
	want = "# T\n\nSource: text · Ref: -\n\n## Acceptance criteria\n_none in ticket — planner derives them_\n\n## Body\nB\n"
	if got != want {
		t.Fatalf("%q", got)
	}
}
