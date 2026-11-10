package workspace

import (
	"slices"
	"strings"
	"testing"
)

func TestPackArtifactMatching(t *testing.T) {
	packs := []PackArtifacts{
		{Root: ".", Globs: []string{"out/", "*.tmpx", "node_modules/.cache/"}},
		{Root: "svc/api", Globs: []string{"bin/", "obj/", "TestResults/", "*.trx"}},
	}
	for rel, want := range map[string]bool{
		"a.tmpx": true, "deep/b.tmpx": true, "out": false, "a.tmp": false,
		"svc/api/r.trx": true, "svc/other/r.trx": false, "svc/api": false,
	} {
		if got := IsPackArtifactFile(rel, packs); got != want {
			t.Errorf("file %s: %v", rel, got)
		}
	}
	for rel, want := range map[string]bool{
		"out": true, "x/out": true, "out2": false, "a.tmpx": false,
		"svc/api/bin": true, "svc/api/x/obj": true, "bin": false, "svc/apix/bin": false,
		"node_modules/.cache": true,
	} {
		if got := IsPackArtifactDir(rel, packs); got != want {
			t.Errorf("dir %s: %v", rel, got)
		}
	}
}

func TestDecideFor(t *testing.T) {
	packs := []PackArtifacts{{Root: ".", Globs: []string{"*.tmpx", "out/"}}}
	cases := []struct {
		e    Entry
		want Action
	}{
		{Entry{Rel: "a.tmpx", Kind: KindFile, Head: []byte("data")}, Remove},
		{Entry{Rel: "x.test", Kind: KindFile, Head: []byte("data")}, Remove}, // Go names still apply
		{Entry{Rel: "a.tmpx", Kind: KindFile, Head: []byte("#!/bin/sh")}, Keep},
		{Entry{Rel: "a.tmpx", Kind: KindSymlink}, Keep},
		{Entry{Rel: "../a.tmpx", Kind: KindFile}, Keep},
		{Entry{Rel: "out", Kind: KindFile}, Keep}, // a dir glob never names a file
		{Entry{Rel: "notes.md", Kind: KindFile}, Keep},
	}
	for _, c := range cases {
		if got := DecideFor(c.e, packs); got != c.want {
			t.Errorf("%+v: %v", c.e, got)
		}
	}
}

func TestArtifactDirs(t *testing.T) {
	packs := []PackArtifacts{{Root: ".", Globs: []string{"out/", "target/", "__pycache__/", "node_modules/.cache/"}}}
	base := map[string]bool{"out2": true, "keep": true, "keep/out": true, "vendor": true}
	cur := []string{
		"out", "out/sub", "out/sub/out", // new, nested: only the top
		"keep", "keep/out", // pre-existing
		"out2", "src", "src/__pycache__", // new pycache under an old parent
		"target", "crate/target", // skipped names themselves qualify
		"vendor", "vendor/out", // inside a skipped tree: never
		"node_modules/.cache",      // inside a skipped tree
		".loops/out", "a/.git/out", // never safe
	}
	got := ArtifactDirs(cur, base, packs)
	want := []string{"crate/target", "out", "src/__pycache__", "target"}
	if !slices.Equal(got, want) {
		t.Fatalf("%v", got)
	}
	if ArtifactDirs(cur, base, nil) != nil {
		t.Fatal("no packs, no dirs")
	}
}

func TestDecideDir(t *testing.T) {
	for _, c := range []struct {
		d    DirEntry
		want Action
	}{
		{DirEntry{Rel: "out", Kind: KindDir}, Remove},
		{DirEntry{Rel: "out", Kind: KindSymlink}, Keep},
		{DirEntry{Rel: "out", Kind: KindOther}, Keep},
		{DirEntry{Rel: "out", Kind: KindDir, Tracked: true}, Keep},
		{DirEntry{Rel: "out", Kind: KindDir, NestedRepo: true}, Keep},
		{DirEntry{Rel: ".loops/x", Kind: KindDir}, Keep},
	} {
		if got := DecideDir(c.d); got != c.want {
			t.Errorf("%+v: %v", c.d, got)
		}
	}
}

func TestTrackedUnder(t *testing.T) {
	tracked := []string{"a.go", "out-x/a", "out.txt", "out/b", "outz"}
	if !TrackedUnder(tracked, "out") || TrackedUnder(tracked, "ou") || TrackedUnder(tracked, "zz") || TrackedUnder(nil, "out") {
		t.Fatal("TrackedUnder")
	}
	if !TrackedUnder([]string{"out"}, "out") || TrackedUnder([]string{"out-x/a", "out.txt", "outz"}, "out") {
		t.Fatal("TrackedUnder exact/near")
	}
	if !Under("a/b", "a") || Under("ab", "a") || !Under("a", "a") {
		t.Fatal("Under")
	}
}

func TestSkipDirExtended(t *testing.T) {
	for _, d := range []string{"vendor", "x/node_modules", "_build/deps", "deps", ".venv", "venv", "target", "a/.gradle", ".loops", ".git"} {
		if !SkipDir(d) {
			t.Errorf("%s not skipped", d)
		}
	}
	for _, d := range []string{"_build", "src", "x/.loops", "targets"} {
		if SkipDir(d) {
			t.Errorf("%s skipped", d)
		}
	}
}

func TestReportPackLines(t *testing.T) {
	r := Report{CacheFiles: 1, CacheBytes: 1 << 20,
		PackCaches:   []DirItem{{Path: "rust", Files: 2, Bytes: 1 << 20}},
		ArtifactDirs: []DirItem{{Path: "out", Files: 3, Bytes: 1 << 20}},
		Removed:      []Item{{Path: "a.tmpx", Bytes: 1 << 20}}, Kept: []string{}}
	if r.Files() != 7 || r.Bytes() != 4<<20 {
		t.Fatal(r.Files(), r.Bytes())
	}
	txt := r.Text()
	for _, want := range []string{"cleanup: 7 files, 4.0 MB freed", "  go cache: 1 files", "  rust cache: 2 files, 1.0 MB", "  removed out/ (3 files, 1.0 MB)"} {
		if !strings.Contains(txt, want) {
			t.Errorf("%q lacks %q", txt, want)
		}
	}
	r.DryRun = true
	if !strings.Contains(r.Text(), "would remove out/ (3 files") {
		t.Fatal(r.Text())
	}
}
