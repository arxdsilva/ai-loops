package workspace

import (
	"bytes"
	"sort"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/glob"
)

// PackArtifacts is one detected pack's artifact globs (pack.json
// "artifacts"), scoped to the pack's root ("." or "" = the project root).
// A glob ending in "/" names a directory (D25); the others name files.
type PackArtifacts struct {
	Root  string
	Globs []string
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
			if isDir == dir && pat != "" && glob.Match(pat, r) {
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

// DecideFor is Decide that also removes a safe, regular, non-script file
// matching a pack file glob.
func DecideFor(e Entry, packs []PackArtifacts) Action {
	if Decide(e) == Remove {
		return Remove
	}
	if e.Kind != KindFile || !SafeRel(e.Rel) || bytes.HasPrefix(e.Head, []byte("#!")) {
		return Keep
	}
	if IsPackArtifactFile(e.Rel, packs) {
		return Remove
	}
	return Keep
}

// ArtifactDirs returns the directories of current (the project's directories
// now) that match a pack directory glob and are new since the baseline
// (absent from baseDirs), are SafeRel and lie outside skipped trees (a
// skipped directory itself may qualify). Only the topmost of nested matches
// is returned; the result is sorted. The adapter still checks each one on
// disk (DecideDir) before removing it.
func ArtifactDirs(current []string, baseDirs map[string]bool, packs []PackArtifacts) []string {
	var cand []string
	for _, d := range current {
		if baseDirs[d] || !SafeRel(d) || skipped(d) || !IsPackArtifactDir(d, packs) {
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
}

// DecideDir returns Remove only for a safe, real directory that holds no
// tracked file and no nested repository.
func DecideDir(d DirEntry) Action {
	if d.Kind != KindDir || !SafeRel(d.Rel) || d.Tracked || d.NestedRepo {
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
