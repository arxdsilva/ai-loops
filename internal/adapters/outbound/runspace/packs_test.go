package runspace

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/application/langpack"
	"github.com/arxdsilva/ai-loops/internal/domain/pack"
	"github.com/arxdsilva/ai-loops/internal/domain/workspace"
)

func fooPack() pack.Pack {
	return pack.Pack{ID: "foo", CacheEnv: map[string]string{"FOO_CACHE": "cache"}, Env: map[string]string{"FOO_MODE": "ci"},
		Artifacts: []string{"out/", "*.tmpx"}, CacheArgv: map[string][]string{"sh": {"-e"}}}
}

func goPack() pack.Pack {
	return pack.Pack{ID: "go", CacheEnv: map[string]string{"GOCACHE": "cache", "GOTMPDIR": "tmp"}, Artifacts: []string{"*.test"}}
}

func withPacks(s *Space, ds ...langpack.Detected) *Space {
	return s.WithPacks(func() []langpack.Detected { return ds })
}

// End to end in a temp git repo: a fake pack's run dirs and env, a command
// run with that env, and the cleanup of its new artifact dir and files.
func TestPackEndToEnd(t *testing.T) {
	needGit(t)
	ctx := context.Background()
	root := project(t)
	outside := project(t)
	write(t, outside, "precious/out.tmpx", "x", 0o644)
	git(t, root, "init", "-q")
	write(t, root, "tracked.txt", "t\n", 0o644)
	write(t, root, "src/out/tracked.tmpx", "kept: tracked\n", 0o644) // tracked inside a matching dir
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "init")
	write(t, root, "out2/old.bin", "old", 0o644)  // pre-existing dir
	write(t, root, "lib/out/old.o", "old", 0o644) // pre-existing matching dir
	write(t, root, "old.tmpx", "old", 0o644)      // pre-existing artifact file
	rd := runDir(t, root)
	s := withPacks(New(root), langpack.Detected{Pack: goPack(), ID: "go", Root: "."}, langpack.Detected{Pack: fooPack(), ID: "foo", Root: "."})
	if err := s.Prepare(ctx, rd); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{workspace.GoCache(rd), workspace.GoTmp(rd), workspace.PackCache(rd, "foo"), workspace.PackTmp(rd, "foo")} {
		if fi, err := os.Stat(d); err != nil || !fi.IsDir() {
			t.Fatal(d, err)
		}
	}
	if h, _, _ := readLines(workspace.BaselineDirs(rd)); h != headerDirsV2 {
		t.Fatal(h)
	}

	env, err := s.PackEnv(rd, fooPack())
	if err != nil || !slices.Equal(env, []string{"FOO_CACHE=" + workspace.PackCache(rd, "foo"), "FOO_MODE=ci"}) {
		t.Fatal(env, err)
	}
	if runtime.GOOS != "windows" {
		var out bytes.Buffer
		argv := workspace.PackArgv(rd, fooPack(), []string{"sh", "-c", `echo "$FOO_CACHE $FOO_MODE $-"`})
		if code := s.Exec(ctx, root, env, argv, nil, &out, &out); code != 0 || !strings.HasPrefix(out.String(), workspace.PackCache(rd, "foo")+" ci ") || !strings.Contains(strings.TrimSpace(out.String())[len(workspace.PackCache(rd, "foo"))+4:], "e") {
			t.Fatalf("%d %q", code, out.String())
		}
	}

	// what the run leaves behind
	write(t, root, "out/a.o", "obj", 0o644)
	write(t, root, "out/deep/b.o", "obj2", 0o444)
	if err := os.Chmod(filepath.Join(root, "out/deep"), 0o555); err != nil {
		t.Fatal(err)
	}
	write(t, root, "pkg/out/c.o", "c", 0o644) // new matching dir under a new parent
	write(t, root, "x.tmpx", "tmp", 0o644)
	write(t, root, "notes.md", "n", 0o644)
	write(t, root, "nested/out/.git/HEAD", "ref", 0o644) // a nested repository: refused
	write(t, root, "trk/out/t.txt", "t", 0o644)          // tracked after the start: refused
	git(t, root, "add", "trk/out/t.txt")
	if err := os.Symlink(filepath.Join(outside, "precious"), filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	write(t, workspace.PackCache(rd, "foo"), "ro/entry", "cached", 0o444)
	if err := os.Chmod(filepath.Join(workspace.PackCache(rd, "foo"), "ro"), 0o555); err != nil {
		t.Fatal(err)
	}

	dry, err := s.Clean(ctx, rd, true)
	if err != nil || len(dry.ArtifactDirs) != 2 || !exists(filepath.Join(root, "out")) || !exists(workspace.PackDir(rd, "foo")) {
		t.Fatalf("dry %+v %v", dry, err)
	}
	r, err := s.Clean(ctx, rd, false)
	if err != nil {
		t.Fatal(err)
	}
	var dirs []string
	for _, d := range r.ArtifactDirs {
		dirs = append(dirs, d.Path)
	}
	if !slices.Equal(dirs, []string{"out", "pkg/out"}) || r.ArtifactDirs[0].Files != 2 || r.ArtifactDirs[0].Bytes != 7 {
		t.Fatalf("artifact dirs %+v", r.ArtifactDirs)
	}
	if len(r.Removed) != 1 || r.Removed[0].Path != "x.tmpx" {
		t.Fatalf("removed %+v", r.Removed)
	}
	if len(r.PackCaches) != 1 || r.PackCaches[0].Path != "foo" || r.PackCaches[0].Files != 1 || exists(workspace.PackDir(rd, "foo")) || exists(workspace.GoDir(rd)) {
		t.Fatalf("pack caches %+v", r.PackCaches)
	}
	if !slices.Contains(r.DirsRemoved, "pkg") {
		t.Errorf("pkg not pruned: %v", r.DirsRemoved)
	}
	refused := strings.Join(r.Refused, "\n")
	if !strings.Contains(refused, "nested/out/") || !strings.Contains(refused, "trk/out/") {
		t.Errorf("refused %q", refused)
	}
	for _, k := range []string{"notes.md", "linked"} {
		if !slices.Contains(r.Kept, k) {
			t.Errorf("kept %v lacks %s", r.Kept, k)
		}
	}
	for _, p := range []string{"tracked.txt", "src/out/tracked.tmpx", "out2/old.bin", "lib/out/old.o", "old.tmpx", "notes.md", "nested/out/.git/HEAD", "trk/out/t.txt", "linked"} {
		if !exists(filepath.Join(root, p)) {
			t.Errorf("%s was removed", p)
		}
	}
	for _, p := range []string{"out", "pkg", "x.tmpx"} {
		if exists(filepath.Join(root, p)) {
			t.Errorf("%s still exists", p)
		}
	}
	if !exists(filepath.Join(outside, "precious/out.tmpx")) {
		t.Fatal("followed a symlink out of the root")
	}
	if !strings.Contains(r.Text(), "foo cache: 1 files") || !strings.Contains(r.Text(), "removed out/ (2 files") {
		t.Fatal(r.Text())
	}
}

// A pre-existing skipped dir (target/) is listed in the v2 snapshot, so it
// is not new; a v1 snapshot (older loops) never lets directories go.
func TestArtifactDirsNeedV2Snapshot(t *testing.T) {
	needGit(t)
	ctx := context.Background()
	root := project(t)
	git(t, root, "init", "-q")
	write(t, root, "target/debug/old", "x", 0o644)
	rd := runDir(t, root)
	rust := pack.Pack{ID: "rust", Artifacts: []string{"target/", "out/"}}
	s := withPacks(New(root), langpack.Detected{Pack: rust, ID: "rust", Root: "."}, langpack.Detected{Pack: pack.Pack{ID: "Bad/ID"}, ID: "Bad/ID"})
	if err := s.Prepare(ctx, rd); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(workspace.BaselineDirs(rd))
	if !strings.Contains(string(b), "\ntarget\n") || strings.Contains(string(b), "target/debug") {
		t.Fatalf("dirs %q", b)
	}
	write(t, root, "out/x", "x", 0o644)
	write(t, root, "target/debug/new", "x", 0o644)
	// rewrite the snapshot as an older loops would have: v1 header.
	_, dirs, _ := readLines(workspace.BaselineDirs(rd))
	if err := writeLines(workspace.BaselineDirs(rd), headerDirsV1, dirs); err != nil {
		t.Fatal(err)
	}
	r, err := s.Clean(ctx, rd, false)
	if err != nil || len(r.ArtifactDirs) != 0 || !exists(filepath.Join(root, "out/x")) {
		t.Fatalf("v1: %+v %v", r, err)
	}
	if err := writeLines(workspace.BaselineDirs(rd), headerDirsV2, dirs); err != nil {
		t.Fatal(err)
	}
	r, err = s.Clean(ctx, rd, false)
	if err != nil || len(r.ArtifactDirs) != 1 || r.ArtifactDirs[0].Path != "out" || exists(filepath.Join(root, "out")) || !exists(filepath.Join(root, "target/debug/old")) {
		t.Fatalf("v2: %+v %v", r, err)
	}
	if _, err := s.PackEnv(rd, pack.Pack{ID: "../x"}); err == nil {
		t.Fatal("bad id accepted")
	}
}

func TestRemoveDirRefusals(t *testing.T) {
	root := project(t)
	outside := project(t)
	write(t, outside, "f", "x", 0o644)
	if err := os.Symlink(outside, filepath.Join(root, "out")); err != nil {
		t.Fatal(err)
	}
	if _, err := removeDir(root, "out", nil, false); err == nil || !exists(filepath.Join(outside, "f")) {
		t.Fatal("symlinked dir removed")
	}
	if _, err := removeDir(root, "../x", nil, false); err == nil {
		t.Fatal("unsafe path")
	}
	if _, err := removeDir(root, "missing", nil, false); err == nil {
		t.Fatal("missing dir")
	}
	write(t, root, "d/out/f", "x", 0o644)
	if _, err := removeDir(root, "d/out", []string{"d/out/f"}, false); err == nil {
		t.Fatal("tracked")
	}
	if it, err := removeDir(root, "d/out", nil, false); err != nil || it.Files != 1 || exists(filepath.Join(root, "d/out")) {
		t.Fatal(it, err)
	}
}

func TestExecAndCleanRunSub(t *testing.T) {
	var errb bytes.Buffer
	s := New(t.TempDir())
	if code := s.Exec(context.Background(), "", nil, nil, nil, &bytes.Buffer{}, &errb); code != 127 {
		t.Fatal(code)
	}
	if code := s.Exec(context.Background(), "", nil, []string{"definitely-not-a-binary-xyz"}, nil, &bytes.Buffer{}, &errb); code != 127 || !strings.Contains(errb.String(), "loops exec") {
		t.Fatal(code, errb.String())
	}
	if s.Packs() != nil {
		t.Fatal("no packs without WithPacks")
	}
	// a pack dir that is a file is refused, not removed.
	rd := t.TempDir()
	write(t, rd, "foo", "x", 0o644)
	var r workspace.Report
	withPacks(New(t.TempDir()), langpack.Detected{Pack: fooPack(), ID: "foo"}).cleanPacks(rd, false, &r)
	if len(r.Refused) != 1 || !exists(filepath.Join(rd, "foo")) {
		t.Fatalf("%+v", r)
	}
}
