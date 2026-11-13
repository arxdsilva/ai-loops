package release

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type fakeGit struct {
	lastTag  string
	commits  []string
	headTags []string
	subject  string
	dirty    bool
	taken    map[string]bool
	calls    []string
	err      map[string]error
}

func (g *fakeGit) rec(name string, a ...string) error {
	g.calls = append(g.calls, name+" "+strings.Join(a, " "))
	return g.err[name]
}
func (g *fakeGit) LastTag(context.Context) (string, error) { return g.lastTag, g.rec("LastTag") }
func (g *fakeGit) CommitsSince(_ context.Context, tag string) ([]string, error) {
	return g.commits, g.rec("CommitsSince", tag)
}
func (g *fakeGit) HeadTags(context.Context) ([]string, error) { return g.headTags, g.rec("HeadTags") }
func (g *fakeGit) HeadSubject(context.Context) (string, error) {
	return g.subject, g.rec("HeadSubject")
}
func (g *fakeGit) Dirty(context.Context) (bool, error) { return g.dirty, g.rec("Dirty") }
func (g *fakeGit) Commit(_ context.Context, msg string, paths []string) error {
	return g.rec("Commit", msg, strings.Join(paths, ","))
}
func (g *fakeGit) Tag(_ context.Context, tag, msg string) error { return g.rec("Tag", tag, msg) }
func (g *fakeGit) TagExists(_ context.Context, tag string) (bool, error) {
	return g.taken[tag], g.rec("TagExists", tag)
}
func (g *fakeGit) Push(_ context.Context, refs ...string) error {
	return g.rec("Push", refs...)
}

type fakeFiles struct {
	v     string
	calls *[]string
	err   error
}

func (f *fakeFiles) ReadVersion() (string, error) { return f.v, nil }
func (f *fakeFiles) WriteVersion(v string) error {
	*f.calls = append(*f.calls, "WriteVersion "+v)
	return f.err
}

func setup(g *fakeGit) (*service, *int) {
	regen := 0
	f := &fakeFiles{v: "0.3.0", calls: &g.calls}
	return New(g, f, func() error { regen++; g.calls = append(g.calls, "Regenerate"); return nil }), &regen
}

func has(calls []string, prefix string) bool {
	for _, c := range calls {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

func TestReleaseHappyPath(t *testing.T) {
	g := &fakeGit{lastTag: "v0.3.0", commits: []string{"feat: a", "fix: b"}, subject: "feat: a"}
	s, regen := setup(g)
	res, err := s.Release(context.Background(), Options{Level: "auto", Push: true})
	if err != nil || res.Skipped || res.Current != "0.3.0" || res.Next != "0.4.0" || res.Level != "minor" || res.Summary != "1 feat, 1 fix" {
		t.Fatalf("%+v %v", res, err)
	}
	if *regen != 1 {
		t.Fatal("regenerate")
	}
	want := []string{
		"LastTag ", "CommitsSince v0.3.0", "HeadTags ", "HeadSubject ", "Dirty ",
		"TagExists v0.4.0", "WriteVersion 0.4.0", "Regenerate",
		"Commit release v0.4.0 " + strings.Join(ReleasePaths, ","),
		"Tag v0.4.0 loops v0.4.0",
		"Push HEAD v0.4.0",
	}
	if !reflect.DeepEqual(g.calls, want) {
		t.Fatalf("calls:\n%q\nwant\n%q", g.calls, want)
	}
}

func TestReleaseNoPush(t *testing.T) {
	g := &fakeGit{lastTag: "v0.3.0", commits: []string{"fix: b"}, subject: "fix: b"}
	s, _ := setup(g)
	if _, err := s.Release(context.Background(), Options{Level: "auto"}); err != nil || has(g.calls, "Push") {
		t.Fatalf("%v %q", err, g.calls)
	}
}

func TestReleaseSkips(t *testing.T) {
	tests := map[string]*fakeGit{
		"head tagged":    {lastTag: "v0.3.0", commits: []string{"fix: b"}, headTags: []string{"v0.3.0"}},
		"release commit": {lastTag: "v0.3.0", commits: []string{"release v0.3.0"}, subject: "release v0.3.0"},
		"no commits":     {lastTag: "v0.3.0", subject: "x"},
	}
	for name, g := range tests {
		t.Run(name, func(t *testing.T) {
			s, regen := setup(g)
			res, err := s.Release(context.Background(), Options{Level: "auto"})
			if err != nil || !res.Skipped || res.Reason == "" || *regen != 0 || has(g.calls, "WriteVersion") || has(g.calls, "Commit release") {
				t.Fatalf("%+v %v %q", res, err, g.calls)
			}
		})
	}
}

func TestReleaseDirty(t *testing.T) {
	g := &fakeGit{lastTag: "v0.3.0", commits: []string{"fix: b"}, subject: "fix: b", dirty: true}
	s, regen := setup(g)
	if _, err := s.Release(context.Background(), Options{Level: "auto"}); err == nil || *regen != 0 || has(g.calls, "WriteVersion") {
		t.Fatalf("dirty tree accepted: %v", err)
	}
	res, err := s.Release(context.Background(), Options{Level: "auto", DryRun: true})
	if err != nil || !res.DryRun || res.Next != "0.3.1" || has(g.calls, "WriteVersion") || has(g.calls, "Commit release") || has(g.calls, "Tag v") {
		t.Fatalf("dry-run: %+v %v %q", res, err, g.calls)
	}
}

func TestReleaseNoTagUsesAllCommits(t *testing.T) {
	g := &fakeGit{commits: []string{"feat: a"}, subject: "feat: a"}
	s, _ := setup(g)
	res, err := s.Release(context.Background(), Options{Level: "auto", DryRun: true})
	if err != nil || res.Next != "0.4.0" || !has(g.calls, "CommitsSince ") {
		t.Fatalf("%+v %v %q", res, err, g.calls)
	}
}

func TestReleaseErrors(t *testing.T) {
	boom := errors.New("boom")
	for _, op := range []string{"LastTag", "CommitsSince", "HeadTags", "HeadSubject", "Dirty", "TagExists", "Commit", "Tag", "Push"} {
		t.Run(op, func(t *testing.T) {
			g := &fakeGit{lastTag: "v0.3.0", commits: []string{"fix: b"}, subject: "x", err: map[string]error{op: boom}}
			s, _ := setup(g)
			if _, err := s.Release(context.Background(), Options{Level: "auto", Push: true}); !errors.Is(err, boom) {
				t.Fatalf("%v", err)
			}
		})
	}
	g := &fakeGit{lastTag: "v0.3.0", commits: []string{"fix: b"}, subject: "x"}
	s, _ := setup(g)
	if _, err := s.Release(context.Background(), Options{Level: "bogus"}); err == nil {
		t.Fatal("bad level")
	}
	f := &fakeFiles{v: "0.3.0", calls: &g.calls, err: boom}
	s = New(g, f, func() error { return nil })
	if _, err := s.Release(context.Background(), Options{Level: "auto"}); !errors.Is(err, boom) {
		t.Fatalf("write: %v", err)
	}
	s = New(g, &fakeFiles{v: "0.3.0", calls: &g.calls}, func() error { return boom })
	if _, err := s.Release(context.Background(), Options{Level: "auto"}); !errors.Is(err, boom) {
		t.Fatalf("regen: %v", err)
	}
}

// A tag left behind by a release whose commit never reached the branch
// (v0.3.1 below) must not block the next release: it moves past it.
func TestReleaseSkipsTakenTags(t *testing.T) {
	g := &fakeGit{lastTag: "v0.3.0", commits: []string{"fix: b"}, subject: "fix: b",
		taken: map[string]bool{"v0.3.1": true, "v0.3.2": true}}
	s, _ := setup(g)
	res, err := s.Release(context.Background(), Options{Level: "auto", Push: true})
	if err != nil || res.Next != "0.3.3" || !reflect.DeepEqual(res.Taken, []string{"v0.3.1", "v0.3.2"}) {
		t.Fatalf("%+v %v", res, err)
	}
	if !has(g.calls, "Tag v0.3.3") || !has(g.calls, "Push HEAD v0.3.3") || !has(g.calls, "WriteVersion 0.3.3") {
		t.Fatalf("calls %q", g.calls)
	}
	dry := &fakeGit{lastTag: "v0.3.0", commits: []string{"feat: a"}, subject: "x", taken: map[string]bool{"v0.4.0": true}}
	s, _ = setup(dry)
	if res, err := s.Release(context.Background(), Options{Level: "auto", DryRun: true}); err != nil || res.Next != "0.4.1" {
		t.Fatalf("dry run: %+v %v", res, err)
	}
}

func TestReleaseTakenWalkIsBounded(t *testing.T) {
	g := &allTaken{fakeGit{lastTag: "v0.3.0", commits: []string{"fix: b"}, subject: "x"}}
	s := New(g, &fakeFiles{v: "0.3.0", calls: &g.calls}, func() error { return nil })
	if _, err := s.Release(context.Background(), Options{Level: "auto"}); err == nil || has(g.calls, "WriteVersion") {
		t.Fatalf("unbounded walk: %v", err)
	}
}

type allTaken struct{ fakeGit }

func (g *allTaken) TagExists(context.Context, string) (bool, error) { return true, nil }
