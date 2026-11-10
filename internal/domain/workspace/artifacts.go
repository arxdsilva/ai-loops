package workspace

import (
	"bytes"
	"path"
	"sort"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/glob"
)

// PackArtifacts is one detected pack's cleanup data, scoped to the pack's
// root ("." or "" = the project root): Globs are its artifact globs
// (pack.json "artifacts"); Source, Test and Generated are its source_globs,
// test_globs and generated.globs, which keep authored files from removal.
//
// A glob ending in "/" names a directory (D25). Without another "/" it is
// anchored at the pack root: "build/" is <root>/build only, "**/build/"
// matches at any depth, and a directory glob holding "/" ("node_modules/.cache/")
// keeps glob semantics. The other globs name files and match as glob.Match
// does (no "/": the base name at any depth).
type PackArtifacts struct {
	Root      string
	Globs     []string
	Source    []string
	Test      []string
	Generated []string
}

// within returns rel relative to root, and whether rel lies under root.
func within(root, rel string) (string, bool) {
	if root == "" || root == "." {
		return rel, true
	}
	if r, ok := strings.CutPrefix(rel, root+"/"); ok {
		return r, true
	}
	return "", false
}

// matchDir matches a directory glob (without its "/") against r, the
// pack-root-relative path: anchored unless the glob holds a "/".
func matchDir(pat, r string) bool {
	if !strings.Contains(pat, "/") {
		ok, err := path.Match(pat, r)
		return err == nil && ok
	}
	return glob.Match(pat, r)
}

// matches reports whether rel (under some pack's root) matches one of its
// globs of the wanted kind (dir: ending in "/").
func matches(rel string, packs []PackArtifacts, dir bool) bool {
	for _, p := range packs {
		r, ok := within(p.Root, rel)
		if !ok {
			continue
		}
		for _, g := range p.Globs {
			pat, isDir := strings.CutSuffix(g, "/")
			if isDir != dir || pat == "" {
				continue
			}
			if (dir && matchDir(pat, r)) || (!dir && glob.Match(pat, r)) {
				return true
			}
		}
	}
	return false
}

// IsPackArtifactFile reports whether the file rel matches a pack file glob.
func IsPackArtifactFile(rel string, packs []PackArtifacts) bool { return matches(rel, packs, false) }

// IsPackArtifactDir reports whether the directory rel matches a pack
// directory glob (one ending in "/").
func IsPackArtifactDir(rel string, packs []PackArtifacts) bool { return matches(rel, packs, true) }

// ArtifactDirOf returns the topmost proper parent directory of rel that
// matches a pack directory glob.
func ArtifactDirOf(rel string, packs []PackArtifacts) (string, bool) {
	parts := strings.Split(rel, "/")
	for i := 1; i < len(parts); i++ {
		if d := strings.Join(parts[:i], "/"); IsPackArtifactDir(d, packs) {
			return d, true
		}
	}
	return "", false
}

// Reasons a path is authored work that cleanup keeps for review.
const (
	ReasonScript  = "a script"
	ReasonDoc     = "a document"
	ReasonSource  = "a source file"
	ReasonTest    = "a test file"
	ReasonFixture = "a test fixture"
)

// docExts are the documentation extensions cleanup never removes.
var docExts = []string{".md", ".rst", ".adoc"}

// fixtureNames are directory names that hold authored test data; the
// strong ones also keep an artifact directory below them.
var (
	fixtureNames       = map[string]bool{"testdata": true, "fixtures": true, "__fixtures__": true, "test": true, "tests": true, "spec": true, "specs": true}
	strongFixtureNames = map[string]bool{"testdata": true, "fixtures": true, "__fixtures__": true}
)

// generatedDir reports whether a directory segment of rel (not its base
// name) starts with "generated" (target/generated-sources/, GeneratedSources/).
func generatedDir(rel string) bool {
	for d := path.Dir(rel); d != "." && d != "/" && d != ""; d = path.Dir(d) {
		if strings.HasPrefix(strings.ToLower(path.Base(d)), "generated") {
			return true
		}
	}
	return false
}

// Authored returns why the file rel, whose first bytes are head, is
// authored work ("" when nothing marks it so): a shebang, a document
// (*.md, *.rst, *.adoc), or a file matching a pack's test or source globs
// that is not generated (the pack's generated globs, or a directory segment
// starting with "generated"). Every pack is consulted, conservatively: one
// whose root does not hold rel matches rel as given.
func Authored(rel string, head []byte, packs []PackArtifacts) string {
	if bytes.HasPrefix(head, []byte("#!")) {
		return ReasonScript
	}
	low := strings.ToLower(path.Base(rel))
	for _, ext := range docExts {
		if strings.HasSuffix(low, ext) {
			return ReasonDoc
		}
	}
	if generatedDir(rel) {
		return ""
	}
	for _, p := range packs {
		r, ok := within(p.Root, rel)
		if !ok {
			r = rel
		}
		switch {
		case glob.Any(p.Generated, r):
		case glob.Any(p.Test, r):
			return ReasonTest
		case glob.Any(p.Source, r):
			return ReasonSource
		}
	}
	return ""
}

// segmentIn reports whether a segment of the slash path dir is in names.
func segmentIn(dir string, names map[string]bool) bool {
	for _, s := range strings.Split(dir, "/") {
		if names[s] {
			return true
		}
	}
	return false
}

// FileGuard returns why a file a pack glob would remove must be kept (""
// when it may go): Authored, or a fixture (a directory segment named
// testdata, fixtures, __fixtures__, test, tests, spec or specs). Inside an
// artifact directory only the segments leading to that directory count, so
// _build/test/ (a build profile) is not a fixture.
func FileGuard(rel string, head []byte, packs []PackArtifacts) string {
	if why := Authored(rel, head, packs); why != "" {
		return why
	}
	dir := path.Dir(rel)
	if d, ok := ArtifactDirOf(rel, packs); ok {
		dir = d
	}
	if dir != "." && segmentIn(dir, fixtureNames) {
		return ReasonFixture
	}
	return ""
}

// DecideFor is Decide that also removes a safe, regular, non-script file
// that matches a pack file glob or, when the adapter vouches for it
// (Entry.InArtifactDir), lies in an artifact directory, provided FileGuard
// does not keep it.
func DecideFor(e Entry, packs []PackArtifacts) Action {
	if Decide(e) == Remove {
		return Remove
	}
	if e.Kind != KindFile || !SafeRel(e.Rel) || bytes.HasPrefix(e.Head, []byte("#!")) {
		return Keep
	}
	if !e.InArtifactDir && !IsPackArtifactFile(e.Rel, packs) {
		return Keep
	}
	if FileGuard(e.Rel, e.Head, packs) != "" {
		return Keep
	}
	return Remove
}

// ArtifactDirs returns the directories of current (the project's directories
// now) that match a pack directory glob and are new since the baseline
// (absent from baseDirs), are SafeRel, lie outside skipped trees (a
// skipped directory itself may qualify) and outside a testdata, fixtures or
// __fixtures__ directory. Only the topmost of nested matches is returned;
// the result is sorted. The adapter still checks each one on disk
// (DecideDir) before removing it.
func ArtifactDirs(current []string, baseDirs map[string]bool, packs []PackArtifacts) []string {
	var cand []string
	for _, d := range current {
		if baseDirs[d] || !SafeRel(d) || skipped(d) || segmentIn(d, strongFixtureNames) || !IsPackArtifactDir(d, packs) {
			continue
		}
		cand = append(cand, d)
	}
	sort.Strings(cand)
	var out []string
	for _, d := range cand {
		if n := len(out); n > 0 && strings.HasPrefix(d, out[n-1]+"/") {
			continue // inside a directory already selected
		}
		out = append(out, d)
	}
	return out
}

// DirEntry is a candidate artifact directory as the adapter found it.
type DirEntry struct {
	Rel  string
	Kind EntryKind // KindDir unless it is a symlink or vanished
	// Tracked is set when git tracks the path or anything under it.
	Tracked bool
	// NestedRepo is set when a .git entry lies anywhere inside it.
	NestedRepo bool
	// Unreadable is set when the walk could not read an entry inside it
	// (a .git there would go unseen).
	Unreadable bool
	// Authored names the first authored file inside it, with why
	// ("a script: bin/deploy.sh"; Authored).
	Authored string
}

// DirReason returns why the directory must be kept ("" when it may go): a
// symlink or not a directory, an unsafe path, tracked content, a nested
// repository, an unreadable entry or an authored file.
func DirReason(d DirEntry) string {
	switch {
	case d.Kind == KindSymlink:
		return "a symlink"
	case d.Kind != KindDir:
		return "not a directory"
	case !SafeRel(d.Rel):
		return "an unsafe path"
	case d.Tracked:
		return "holds tracked files"
	case d.NestedRepo:
		return "holds a nested repository"
	case d.Unreadable:
		return "holds an unreadable entry"
	case d.Authored != "":
		return "holds " + d.Authored
	}
	return ""
}

// DecideDir returns Remove only for a directory DirReason does not keep.
func DecideDir(d DirEntry) Action {
	if DirReason(d) != "" {
		return Keep
	}
	return Remove
}

// Under reports whether rel is dir or lies inside it.
func Under(rel, dir string) bool { return rel == dir || strings.HasPrefix(rel, dir+"/") }

// TrackedUnder reports whether tracked (sorted) holds dir or a path inside it.
func TrackedUnder(tracked []string, dir string) bool {
	i := sort.SearchStrings(tracked, dir)
	for ; i < len(tracked); i++ {
		t := tracked[i]
		if Under(t, dir) {
			return true
		}
		if !strings.HasPrefix(t, dir) {
			return false
		}
	}
	return false
}
