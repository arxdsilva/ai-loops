package workspace

import (
	"slices"
	"strings"
	"testing"
)

func TestPackArtifactMatching(t *testing.T) {
	packs := []PackArtifacts{
		{Root: ".", Globs: []string{"out/", "*.tmpx", "node_modules/.cache/"}},
		{Root: "svc/api", Globs: []string{"bin/", "**/obj/", "TestResults/", "*.trx", "cmake-build-*/"}},
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
		// a directory glob without another "/" is anchored at the pack root
		"out": true, "x/out": false, "out2": false, "a.tmpx": false,
		"svc/api/bin": true, "svc/api/x/bin": false, "bin": false, "svc/apix/bin": false,
		// "**/" matches at any depth, including the root
		"svc/api/obj": true, "svc/api/x/obj": true, "svc/api/x/y/obj": true,
		"svc/api/cmake-build-debug": true, "svc/api/x/cmake-build-debug": false,
		"node_modules/.cache": true, "x/node_modules/.cache": false,
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
	packs := []PackArtifacts{{Root: ".", Globs: []string{"out/", "target/", "**/__pycache__/", "node_modules/.cache/"}}}
	base := map[string]bool{"out2": true, "keep": true, "keep/out": true, "vendor": true}
	cur := []string{
		"out", "out/sub", "out/sub/out", // new, nested: only the top
		"keep", "keep/out", // pre-existing
		"out2", "src", "src/__pycache__", // new pycache under an old parent
		"target", "crate/target", // a skipped name itself qualifies; target/ is anchored
		"vendor", "vendor/out", // inside a skipped tree: never
		"node_modules/.cache",      // inside a skipped tree
		".loops/out", "a/.git/out", // never safe
		"__pycache__", "testdata/x/__pycache__", "fixtures/__pycache__", // fixtures are authored
		"lib/app/out", // anchored: not the root out/
	}
	got := ArtifactDirs(cur, base, packs)
	want := []string{"__pycache__", "out", "src/__pycache__", "target"}
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

func TestDecideForInsidePreexistingArtifactDir(t *testing.T) {
	packs := []PackArtifacts{
		{Root: ".", Globs: []string{"_build/"}, Source: []string{"**/src/*.erl"}, Test: []string{"**/test/*_SUITE.erl"}, Generated: []string{"**/_build/**"}},
		{Root: "svc", Globs: []string{"target/"}, Source: []string{"**/src/main/java/**/*.java"}},
	}
	for rel, want := range map[string]Action{
		"_build/test/logs/ct_run.1/x.html":      Remove, // _build/test is a build profile, not a fixture
		"_build/default/lib/app/src/a.erl":      Remove, // generated by the pack's globs
		"_build/default/lib/dep/README.md":      Keep,   // documents are never removed
		"_build/test/logs/run.sh":               Keep,   // a script (see head below)
		"svc/target/classes/A.class":            Remove,
		"svc/target/src/main/java/a/B.java":     Keep, // a source file the pack does not call generated
		"svc/target/generated-sources/a/C.java": Remove,
		"target/x":                              Keep, // the target/ glob is scoped to svc
		"src/a.erl":                             Keep,
	} {
		e := Entry{Rel: rel, Kind: KindFile, Head: []byte("data")}
		if strings.HasSuffix(rel, ".sh") {
			e.Head = []byte("#!/bin/sh\n")
		}
		_, e.InArtifactDir = ArtifactDirOf(rel, packs)
		if got := DecideFor(e, packs); got != want {
			t.Errorf("%s: %v, want %v", rel, got, want)
		}
	}
	// the adapter decides: without its word a file in an artifact dir stays
	if DecideFor(Entry{Rel: "_build/test/logs/x.html", Kind: KindFile, Head: []byte("d")}, packs) != Keep {
		t.Fatal("removed without InArtifactDir")
	}
}

func TestArtifactDirOf(t *testing.T) {
	packs := []PackArtifacts{{Root: ".", Globs: []string{"build/", "**/out/", "*.class"}}}
	for rel, want := range map[string]string{
		"build/a/b.o": "build", "build": "", "src/build/x.java": "", "a/out/b/out/c": "a/out", "x.class": "", "build.gradle": "",
	} {
		got, ok := ArtifactDirOf(rel, packs)
		if got != want || ok != (want != "") {
			t.Errorf("%s: %q %v, want %q", rel, got, ok, want)
		}
	}
}

// The reviewer's repros at the domain level: a pack file glob or an
// artifact directory never takes authored work.
func TestFileGuard(t *testing.T) {
	packs := []PackArtifacts{
		{Root: ".", Globs: []string{"lcov.info", "junit.xml", "cobertura.xml", "bom.json", "*.tgz", "*.class", "build/"},
			Source: []string{"**/*.ts", "**/src/main/java/**/*.java"}, Test: []string{"**/*.test.ts", "tests/**"},
			Generated: []string{"**/dist/**"}},
		{Root: "web", Globs: []string{"coverage/"}, Source: []string{"**/*.js"}},
	}
	cases := []struct {
		rel, head, want string
	}{
		{"lcov.info", "TN:", ""},
		{"testdata/lcov.info", "TN:", ReasonFixture},
		{"pkg/testdata/golden/junit.xml", "<x/>", ReasonFixture},
		{"a/__fixtures__/bom.json", "{}", ReasonFixture},
		{"fixtures/pkg.tgz", "\x1f\x8b", ReasonFixture},
		{"spec/cobertura.xml", "<x/>", ReasonFixture},
		{"specs/x/bom.json", "{}", ReasonFixture},
		{"test/junit.xml", "<x/>", ReasonFixture},
		{"tests/x.class", "\xca\xfe", ReasonTest}, // tests/** is a test glob
		{"build/test/x.class", "\xca\xfe", ""},    // inside build/ only build/ itself counts
		{"tests/build/x.class", "\xca\xfe", ReasonTest},
		{"x/tests/build/y.class", "\xca\xfe", ReasonFixture},
		{"build/run.sh", "#!/bin/sh", ReasonScript},
		{"build/README.md", "# r", ReasonDoc},
		{"build/guide.RST", "r", ReasonDoc},
		{"build/manual.adoc", "r", ReasonDoc},
		{"build/app.ts", "x", ReasonSource},
		{"build/app.test.ts", "x", ReasonTest},
		{"build/dist/app.ts", "x", ""},      // the pack's generated globs
		{"build/generated/app.ts", "x", ""}, // a generated* segment
		{"build/GeneratedSources/app.ts", "x", ""},
		{"build/generated.ts", "x", ReasonSource}, // only directory segments count
		{"web/coverage/prettify.js", "x", ReasonSource},
		{"other/coverage/x.js", "x", ReasonSource}, // outside web/: matched as given
		{"src/main/java/a/build/Builder.java", "x", ReasonSource},
	}
	for _, c := range cases {
		if got := FileGuard(c.rel, []byte(c.head), packs); got != c.want {
			t.Errorf("%s: %q, want %q", c.rel, got, c.want)
		}
	}
	if DecideFor(Entry{Rel: "testdata/lcov.info", Kind: KindFile, Head: []byte("TN:")}, packs) != Keep ||
		DecideFor(Entry{Rel: "lcov.info", Kind: KindFile, Head: []byte("TN:")}, packs) != Remove {
		t.Fatal("DecideFor ignores FileGuard")
	}
	// The Go rules keep today's behaviour, fixtures or not.
	if DecideFor(Entry{Rel: "testdata/x.test", Kind: KindFile, Head: []byte("d")}, packs) != Remove {
		t.Fatal("Go suffix rule changed")
	}
}

func TestDirReason(t *testing.T) {
	for _, c := range []struct {
		d    DirEntry
		want string
	}{
		{DirEntry{Rel: "out", Kind: KindDir}, ""},
		{DirEntry{Rel: "out", Kind: KindSymlink}, "a symlink"},
		{DirEntry{Rel: "out", Kind: KindOther}, "not a directory"},
		{DirEntry{Rel: "../out", Kind: KindDir}, "an unsafe path"},
		{DirEntry{Rel: "out", Kind: KindDir, Tracked: true}, "holds tracked files"},
		{DirEntry{Rel: "out", Kind: KindDir, NestedRepo: true}, "holds a nested repository"},
		{DirEntry{Rel: "out", Kind: KindDir, Unreadable: true}, "holds an unreadable entry"},
		{DirEntry{Rel: "bin", Kind: KindDir, Authored: "a script: bin/deploy.sh"}, "holds a script: bin/deploy.sh"},
	} {
		if got := DirReason(c.d); got != c.want {
			t.Errorf("%+v: %q, want %q", c.d, got, c.want)
		}
	}
}
