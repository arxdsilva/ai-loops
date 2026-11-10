package runspace

import (
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

const javaClass = "\xca\xfe\xba\xbe\x00\x00\x00\x41class"

func javaPack(arts ...string) pack.Pack {
	return pack.Pack{ID: "java", Artifacts: arts,
		SourceGlobs: []string{"**/src/main/java/**/*.java"}, TestGlobs: []string{"**/src/test/java/**/*.java", "**/*Test.java"},
		Generated: pack.Generated{Globs: []string{"**/target/generated-sources/**", "**/build/generated/**"}}}
}

func detect(ps ...pack.Pack) []langpack.Detected {
	var out []langpack.Detected
	for _, p := range ps {
		out = append(out, langpack.Detected{Pack: p, ID: p.ID, Root: "."})
	}
	return out
}

// gitProject is a committed git repo holding files (rel -> content).
func gitProject(t *testing.T, files map[string]string) string {
	t.Helper()
	needGit(t)
	root := project(t)
	git(t, root, "init", "-q")
	for rel, data := range files {
		write(t, root, rel, data, 0o644)
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "init", "--allow-empty")
	return root
}

func mustExist(t *testing.T, root string, rels ...string) {
	t.Helper()
	for _, rel := range rels {
		if !exists(filepath.Join(root, rel)) {
			t.Errorf("DELETED %s", rel)
		}
	}
}

func mustBeGone(t *testing.T, root string, rels ...string) {
	t.Helper()
	for _, rel := range rels {
		if exists(filepath.Join(root, rel)) {
			t.Errorf("%s still exists", rel)
		}
	}
}

// The reviewer's repro (WORKFLOW §2.6): uncommitted, agent-written work in
// directories whose names match pack directory globs is kept for review.
func TestCleanKeepsAuthoredWork(t *testing.T) {
	ctx := context.Background()
	root := gitProject(t, map[string]string{"pom.xml": "<project/>", "src/main/java/com/acme/App.java": "class App{}", "mix.exs": "x"})
	rd := runDir(t, root)
	cs := pack.Pack{ID: "csharp", Artifacts: []string{"bin/", "obj/", "artifacts/", "publish/"}}
	ex := pack.Pack{ID: "elixir", Artifacts: []string{"_build/", "cover/", "doc/", "tmp/"}}
	ts := pack.Pack{ID: "typescript", Artifacts: []string{"lcov.info", "junit.xml", "cobertura.xml", "bom.json", "*.tgz"}}
	s := withPacks(New(root), detect(javaPack("target/", "build/", ".gradle/", "out/", "*.class"), cs, ex, ts)...)
	if err := s.Prepare(ctx, rd); err != nil {
		t.Fatal(err)
	}
	// the run's work, not yet committed (--no-pr, abandoned or failed run)
	write(t, root, "src/main/java/com/acme/build/Builder.java", "package com.acme.build; class Builder{}", 0o644)
	write(t, root, "bin/deploy.sh", "#!/bin/sh\necho deploy\n", 0o755)
	write(t, root, "bin/app.dll", "MZ", 0o644)
	write(t, root, "doc/guide.md", "# hand-written guide\n", 0o644)
	write(t, root, "doc/img/diagram.png", "png", 0o644)
	write(t, root, "lib/app/out/Formatter.java", "class Formatter{}", 0o644)
	write(t, root, "testdata/lcov.info", "TN:\n", 0o644)
	write(t, root, "src/test/fixtures/junit.xml", "<x/>", 0o644)
	write(t, root, "lcov.info", "TN:\n", 0o644) // a real leftover at the root
	for _, dry := range []bool{true, false} {
		r, err := s.Clean(ctx, rd, dry)
		if err != nil {
			t.Fatal(err)
		}
		if len(r.ArtifactDirs) != 0 || len(r.Removed) != 1 || r.Removed[0].Path != "lcov.info" {
			t.Fatalf("dry=%v: dirs %+v removed %+v kept %v refused %v", dry, r.ArtifactDirs, r.Removed, r.Kept, r.Refused)
		}
		for _, k := range []string{"bin/ (holds a script: bin/deploy.sh)", "doc/ (holds a document: doc/guide.md)",
			"bin/app.dll", "doc/img/diagram.png", "testdata/lcov.info", "lib/app/out/Formatter.java"} {
			if !slices.Contains(r.Kept, k) {
				t.Errorf("dry=%v: kept %v lacks %q", dry, r.Kept, k)
			}
		}
	}
	mustExist(t, root, "src/main/java/com/acme/build/Builder.java", "bin/deploy.sh", "bin/app.dll", "doc/guide.md",
		"doc/img/diagram.png", "lib/app/out/Formatter.java", "testdata/lcov.info", "src/test/fixtures/junit.xml")
	mustBeGone(t, root, "lcov.info")
}

// A pack that asks for depth ("**/build/") still never takes a source dir.
func TestCleanDeepGlobKeepsSources(t *testing.T) {
	ctx := context.Background()
	root := gitProject(t, map[string]string{"pom.xml": "<project/>"})
	rd := runDir(t, root)
	s := withPacks(New(root), detect(javaPack("**/build/", "**/out/"))...)
	if err := s.Prepare(ctx, rd); err != nil {
		t.Fatal(err)
	}
	write(t, root, "src/main/java/com/acme/build/Builder.java", "class Builder{}", 0o644)
	write(t, root, "src/test/java/com/acme/out/FooTest.java", "class FooTest{}", 0o644)
	write(t, root, "mod/build/classes/A.class", javaClass, 0o644)
	write(t, root, "mod/build/generated/src/main/java/G.java", "class G{}", 0o644)
	r, err := s.Clean(ctx, rd, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.ArtifactDirs) != 1 || r.ArtifactDirs[0].Path != "mod/build" {
		t.Fatalf("dirs %+v kept %v", r.ArtifactDirs, r.Kept)
	}
	for _, k := range []string{"src/main/java/com/acme/build/ (holds a source file: src/main/java/com/acme/build/Builder.java)",
		"src/test/java/com/acme/out/ (holds a test file: src/test/java/com/acme/out/FooTest.java)"} {
		if !slices.Contains(r.Kept, k) {
			t.Errorf("kept %v lacks %q", r.Kept, k)
		}
	}
	mustExist(t, root, "src/main/java/com/acme/build/Builder.java", "src/test/java/com/acme/out/FooTest.java")
	mustBeGone(t, root, "mod/build")
}

// What cleanup is for: build output the run left behind goes.
func TestCleanRemovesBuildOutput(t *testing.T) {
	ctx := context.Background()
	root := gitProject(t, map[string]string{"pom.xml": "<project/>", "mix.exs": "x", ".gitignore": "_build/\n"})
	write(t, root, "_build/test/logs/old.html", "old", 0o644) // pre-existing, ignored
	rd := runDir(t, root)
	java := javaPack("target/", "*.class")
	java.SourceGlobs = []string{"**/*.java"}
	py := pack.Pack{ID: "python", Artifacts: []string{"__pycache__/"}, SourceGlobs: []string{"**/*.py"}}
	erl := pack.Pack{ID: "erlang", Artifacts: []string{"_build/"}, SourceGlobs: []string{"**/src/*.erl"}, Generated: pack.Generated{Globs: []string{"**/_build/**"}}}
	ds := append(detect(java, py), langpack.Detected{Pack: erl, ID: "erlang", Root: "."}, langpack.Detected{Pack: erl, ID: "erlang", Root: "svc"})
	s := withPacks(New(root), ds...)
	if err := s.Prepare(ctx, rd); err != nil {
		t.Fatal(err)
	}
	write(t, root, "target/classes/com/acme/App.class", javaClass, 0o644)
	write(t, root, "target/generated-sources/annotations/com/acme/App_.java", "class App_{}", 0o644)
	write(t, root, "target/generated-test-sources/FooTest.java", "class FooTest{}", 0o644)
	write(t, root, "__pycache__/m.cpython-312.pyc", "pyc", 0o644)
	write(t, root, "svc/_build/default/lib/app/ebin/app.beam", "FOR1", 0o644)
	write(t, root, "svc/_build/default/lib/app/src/app.erl", "-module(app).", 0o644) // copied: generated
	write(t, root, "_build/test/logs/ct_run.1/index.html", "<html/>", 0o644)         // new in a pre-existing _build/
	write(t, root, "_build/test/logs/ct_run.1/notes.md", "# kept", 0o644)
	r, err := s.Clean(ctx, rd, false)
	if err != nil {
		t.Fatal(err)
	}
	var dirs []string
	for _, d := range r.ArtifactDirs {
		dirs = append(dirs, d.Path)
	}
	if !slices.Equal(dirs, []string{"__pycache__", "svc/_build", "target"}) {
		t.Fatalf("dirs %v kept %v refused %v", dirs, r.Kept, r.Refused)
	}
	if len(r.Removed) != 1 || r.Removed[0].Path != "_build/test/logs/ct_run.1/index.html" {
		t.Fatalf("removed %+v kept %v", r.Removed, r.Kept)
	}
	mustBeGone(t, root, "target", "__pycache__", "svc/_build", "_build/test/logs/ct_run.1/index.html")
	mustExist(t, root, "_build/test/logs/old.html", "_build/test/logs/ct_run.1/notes.md")
}

// Outside git nothing is known to be tracked: no directory is removed, and
// files inside artifact directories are not artifacts by location.
func TestCleanNoGitKeepsDirs(t *testing.T) {
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(os.TempDir()))
	ctx := context.Background()
	root := project(t)
	t.Setenv("GIT_DIR", filepath.Join(root, "no-such-git-dir"))
	write(t, root, "_build/old.log", "old", 0o644)
	rd := runDir(t, root)
	s := withPacks(New(root), detect(pack.Pack{ID: "x", Artifacts: []string{"build/", "_build/", "lcov.info", "*.class"}})...)
	if err := s.Prepare(ctx, rd); err != nil {
		t.Fatal(err)
	}
	write(t, root, "build/A.class", javaClass, 0o644)
	write(t, root, "build/notes.txt", "mine", 0o644)
	write(t, root, "_build/new.log", "new", 0o644)
	write(t, root, "testdata/lcov.info", "TN:", 0o644)
	write(t, root, "lcov.info", "TN:", 0o644)
	r, err := s.Clean(ctx, rd, false)
	if err != nil {
		t.Fatal(err)
	}
	var removed []string
	for _, it := range r.Removed {
		removed = append(removed, it.Path)
	}
	if len(r.ArtifactDirs) != 0 || !slices.Equal(removed, []string{"build/A.class", "lcov.info"}) {
		t.Fatalf("dirs %+v removed %v kept %v", r.ArtifactDirs, removed, r.Kept)
	}
	mustExist(t, root, "build/notes.txt", "_build/new.log", "_build/old.log", "testdata/lcov.info")
}

// A directory cleanup cannot read fully is kept: a nested repository in an
// unreadable subdirectory would otherwise be chmod'ed and deleted.
func TestCleanKeepsUnreadableDir(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("permissions do not restrict this user")
	}
	ctx := context.Background()
	root := gitProject(t, map[string]string{"go.mod": "module m\n"})
	rd := runDir(t, root)
	s := withPacks(New(root), detect(pack.Pack{ID: "x", Artifacts: []string{"out/", "tmp/"}})...)
	if err := s.Prepare(ctx, rd); err != nil {
		t.Fatal(err)
	}
	write(t, root, "out/a.o", "obj", 0o644)
	write(t, root, "out/locked/inner/.git/HEAD", "ref: refs/heads/main\n", 0o644)
	write(t, root, "out/locked/inner/work.txt", "precious", 0o644)
	write(t, root, "tmp/secret.bin", "s", 0o000)
	locked := filepath.Join(root, "out/locked")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	r, err := s.Clean(ctx, rd, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.ArtifactDirs) != 0 {
		t.Fatalf("removed %+v", r.ArtifactDirs)
	}
	for _, k := range []string{"out/ (holds an unreadable entry)", "tmp/ (holds an unreadable entry)"} {
		if !slices.Contains(r.Kept, k) {
			t.Errorf("kept %v lacks %q", r.Kept, k)
		}
	}
	if err := os.Chmod(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	mustExist(t, root, "out/a.o", "out/locked/inner/.git/HEAD", "out/locked/inner/work.txt", "tmp/secret.bin")
}

// With an older directory snapshot (v1) no directory is removed, and the
// files in an artifact directory go one by one only when the directory
// passes the guard itself.
func TestCleanV1SnapshotGuardsArtifactDirFiles(t *testing.T) {
	ctx := context.Background()
	root := gitProject(t, map[string]string{"mix.exs": "x"})
	rd := runDir(t, root)
	s := withPacks(New(root), detect(pack.Pack{ID: "elixir", Artifacts: []string{"doc/", "cover/"}})...)
	if err := s.Prepare(ctx, rd); err != nil {
		t.Fatal(err)
	}
	_, dirs, _ := readLines(workspace.BaselineDirs(rd))
	if err := writeLines(workspace.BaselineDirs(rd), headerDirsV1, dirs); err != nil {
		t.Fatal(err)
	}
	write(t, root, "doc/guide.md", "# mine", 0o644)
	write(t, root, "doc/img.png", "png", 0o644)
	write(t, root, "doc/sub/more.png", "png", 0o644)
	write(t, root, "cover/excoveralls.html", "<html/>", 0o644)
	r, err := s.Clean(ctx, rd, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.ArtifactDirs) != 0 || len(r.Removed) != 1 || r.Removed[0].Path != "cover/excoveralls.html" {
		t.Fatalf("dirs %+v removed %+v kept %v", r.ArtifactDirs, r.Removed, r.Kept)
	}
	mustExist(t, root, "doc/guide.md", "doc/img.png", "doc/sub/more.png")
	if !strings.Contains(strings.Join(r.Kept, " "), "doc/img.png") {
		t.Fatal(r.Kept)
	}
}
