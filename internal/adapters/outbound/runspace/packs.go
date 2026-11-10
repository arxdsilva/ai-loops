package runspace

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/arxdsilva/ai-loops/internal/application/langpack"
	"github.com/arxdsilva/ai-loops/internal/domain/pack"
	"github.com/arxdsilva/ai-loops/internal/domain/workspace"
)

// Packs returns the detected language packs (none without WithPacks).
func (s *Space) Packs() []langpack.Detected {
	if s.packs == nil {
		return nil
	}
	return s.packs()
}

// uniquePacks returns one pack per id (the first detected), dropping ids
// that cannot name a run directory.
func (s *Space) uniquePacks() []pack.Pack {
	var ps []pack.Pack
	for _, d := range s.Packs() {
		ps = append(ps, d.Pack)
	}
	return workspace.Unique(ps)
}

// PackEnv creates p's <run>/<id>/{cache,tmp} and returns the environment
// that scopes its tools to the run (workspace.RunEnv).
func (s *Space) PackEnv(runDir string, p pack.Pack) ([]string, error) {
	if !workspace.ValidPackID(p.ID) {
		return nil, errors.New("pack id " + p.ID + ": not a valid directory name")
	}
	for _, d := range []string{workspace.PackCache(runDir, p.ID), workspace.PackTmp(runDir, p.ID)} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, err
		}
	}
	return workspace.RunEnv(runDir, p), nil
}

// cleanPacks removes <run>/<id> of every detected pack other than Go (whose
// dir cleanGo removes), recording each in Report.PackCaches.
func (s *Space) cleanPacks(runDir string, dryRun bool, r *workspace.Report) {
	for _, p := range s.uniquePacks() {
		if p.ID == workspace.GoPack {
			continue
		}
		if files, size, ok := cleanRunSub(runDir, workspace.PackDir(runDir, p.ID), dryRun, r); ok {
			r.PackCaches = append(r.PackCaches, workspace.DirItem{Path: p.ID, Files: files, Bytes: size})
		}
	}
}

// artifacts returns every detected pack's artifact globs, with the source,
// test and generated globs that guard authored files, scoped to its root.
func (s *Space) artifacts() []workspace.PackArtifacts {
	var out []workspace.PackArtifacts
	for _, d := range s.Packs() {
		if len(d.Pack.Artifacts) > 0 {
			out = append(out, workspace.PackArtifacts{Root: d.Root, Globs: d.Pack.Artifacts,
				Source: d.Pack.SourceGlobs, Test: d.Pack.TestGlobs, Generated: d.Pack.Generated.Globs})
		}
	}
	return out
}

// keptError is a directory the content guard keeps for review
// (workspace.DirReason), as opposed to one that could not be checked.
type keptError struct{ reason string }

func (e *keptError) Error() string { return e.reason }

// cleanArtifactDirs removes the new directories that match a pack directory
// glob (D25) and returns them (in a dry run: those it would remove), plus
// those the content guard kept, which it lists in Report.Kept with the
// reason. The caller runs it only in a git work tree. It needs a v2
// directory snapshot (header, base); each directory must still be a real
// directory inside the root that DecideDir accepts, and be the one inspected
// when it is removed.
func (s *Space) cleanArtifactDirs(realRoot, header string, base map[string]bool, tracked []string, arts []workspace.PackArtifacts, dryRun bool, r *workspace.Report) (gone, kept []string) {
	if len(arts) == 0 || header != headerDirsV2 {
		return nil, nil // no trustworthy directory snapshot: directories are never removed
	}
	cur, err := s.dirs()
	if err != nil {
		return nil, nil
	}
	for _, rel := range workspace.ArtifactDirs(cur, base, arts) {
		item, err := removeDir(realRoot, rel, tracked, arts, dryRun)
		var ke *keptError
		switch {
		case errors.As(err, &ke):
			r.Kept = append(r.Kept, rel+"/ ("+ke.reason+")")
			kept = append(kept, rel)
		case err != nil:
			r.Refused = append(r.Refused, rel+"/: "+err.Error())
			kept = append(kept, rel)
		default:
			r.ArtifactDirs = append(r.ArtifactDirs, item)
			gone = append(gone, rel)
		}
	}
	return gone, kept
}

// inspectDir describes the directory rel for workspace.DecideDir (survey
// fills the content fields) and measures it.
func inspectDir(realRoot, rel string, tracked []string, arts []workspace.PackArtifacts) (workspace.DirEntry, workspace.DirItem, os.FileInfo, error) {
	item := workspace.DirItem{Path: rel}
	e := workspace.DirEntry{Rel: rel, Kind: workspace.KindOther}
	full, ok := contained(realRoot, rel)
	if !ok {
		return e, item, nil, errors.New("refusing a path outside the project root")
	}
	fi, err := os.Lstat(full)
	if err != nil {
		return e, item, nil, err
	}
	e.Tracked = workspace.TrackedUnder(tracked, rel)
	if fi.Mode()&os.ModeSymlink != 0 {
		e.Kind = workspace.KindSymlink
	} else if fi.IsDir() {
		e.Kind = workspace.KindDir
		item.Files, item.Bytes = survey(full, rel, arts, &e)
	}
	return e, item, fi, nil
}

// survey walks the directory full (project path rel) without following
// symlinks, counting its non-directory entries and the bytes of its regular
// files, and records in e what keeps it: a .git entry, an entry it cannot
// read, or an authored file (workspace.Authored, judged from the path and
// the first bytes of each regular file). It stops at the first of those.
func survey(full, rel string, arts []workspace.PackArtifacts, e *workspace.DirEntry) (files int, size int64) {
	_ = filepath.WalkDir(full, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			e.Unreadable = true
			return fs.SkipAll
		}
		if p == full {
			return nil
		}
		if d.Name() == ".git" {
			e.NestedRepo = true
			return fs.SkipAll
		}
		if d.IsDir() {
			return nil
		}
		files++
		if !d.Type().IsRegular() {
			return nil // symlinks and specials are never opened
		}
		info, err := d.Info()
		if err != nil {
			e.Unreadable = true
			return fs.SkipAll
		}
		size += info.Size()
		head, err := readHead(p, 2)
		if err != nil {
			e.Unreadable = true
			return fs.SkipAll
		}
		sub, _ := filepath.Rel(full, p)
		frel := rel + "/" + filepath.ToSlash(sub)
		if why := workspace.Authored(frel, head, arts); why != "" {
			e.Authored = why + ": " + frel
			return fs.SkipAll
		}
		return nil
	})
	return files, size
}

// readHead returns up to n leading bytes of the file p.
func readHead(p string, n int) ([]byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b := make([]byte, n)
	m, err := io.ReadFull(f, b)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}
	return b[:m], nil
}

// removeDir removes the artifact directory rel after the checks
// cleanArtifactDirs lists (dryRun: checks and measures only). A directory
// the content guard keeps is a *keptError.
func removeDir(realRoot, rel string, tracked []string, arts []workspace.PackArtifacts, dryRun bool) (workspace.DirItem, error) {
	e, item, fi, err := inspectDir(realRoot, rel, tracked, arts)
	if err != nil {
		return item, err
	}
	if e.Kind != workspace.KindDir {
		return item, errors.New("not a removable artifact directory (" + workspace.DirReason(e) + ")")
	}
	if why := workspace.DirReason(e); why != "" {
		return item, &keptError{reason: why}
	}
	if dryRun {
		return item, nil
	}
	full, _ := contained(realRoot, rel)
	measure(full, true) // make read-only entries removable
	again, ok := contained(realRoot, rel)
	if !ok || again != full {
		return item, errors.New("refusing a path outside the project root")
	}
	now, err := os.Lstat(full)
	if err != nil || !now.IsDir() || now.Mode()&os.ModeSymlink != 0 || !os.SameFile(fi, now) {
		return item, errors.New("directory changed since it was inspected")
	}
	if err := os.RemoveAll(filepath.Clean(full)); err != nil {
		return item, err
	}
	return item, nil
}

// dirGate decides which new files inside an artifact directory may go
// (workspace.Entry.InArtifactDir): only in a git work tree, never inside a
// directory cleanArtifactDirs kept, and only when the topmost artifact
// directory holding the file existed at the baseline or passes the
// directory guard itself (a run with an older snapshot).
type dirGate struct {
	realRoot string
	tracked  []string
	arts     []workspace.PackArtifacts
	base     map[string]bool
	kept     []string
	memo     map[string]bool
}

func (g *dirGate) ok(rel string) bool {
	if g == nil || underAny(rel, g.kept) {
		return false
	}
	d, ok := workspace.ArtifactDirOf(rel, g.arts)
	if !ok {
		return false
	}
	if g.base[d] {
		return true
	}
	if v, seen := g.memo[d]; seen {
		return v
	}
	e, _, _, err := inspectDir(g.realRoot, d, g.tracked, g.arts)
	v := err == nil && workspace.DecideDir(e) == workspace.Remove
	g.memo[d] = v
	return v
}
