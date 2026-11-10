package runspace

import (
	"errors"
	"os"
	"path/filepath"
	"sort"

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

// artifacts returns every detected pack's artifact globs, scoped to its root.
func (s *Space) artifacts() []workspace.PackArtifacts {
	var out []workspace.PackArtifacts
	for _, d := range s.Packs() {
		if len(d.Pack.Artifacts) > 0 {
			out = append(out, workspace.PackArtifacts{Root: d.Root, Globs: d.Pack.Artifacts})
		}
	}
	return out
}

// cleanArtifactDirs removes the new directories that match a pack directory
// glob (D25) and returns them (in a dry run: those it would remove). It needs
// a v2 directory snapshot; each directory must still be a real directory
// inside the root, hold no tracked file and no nested repository, and be the
// one inspected when it is removed.
func (s *Space) cleanArtifactDirs(realRoot, runDir string, tracked map[string]bool, arts []workspace.PackArtifacts, dryRun bool, r *workspace.Report) []string {
	if len(arts) == 0 {
		return nil
	}
	header, baseDirs, err := readLines(workspace.BaselineDirs(runDir))
	if err != nil || header != headerDirsV2 {
		return nil // no trustworthy directory snapshot: directories are never removed
	}
	cur, err := s.dirs()
	if err != nil {
		return nil
	}
	base := map[string]bool{}
	for _, d := range baseDirs {
		base[d] = true
	}
	cands := workspace.ArtifactDirs(cur, base, arts)
	if len(cands) == 0 {
		return nil
	}
	trackedList := make([]string, 0, len(tracked))
	for t := range tracked {
		trackedList = append(trackedList, t)
	}
	sort.Strings(trackedList)
	var gone []string
	for _, rel := range cands {
		item, err := removeDir(realRoot, rel, trackedList, dryRun)
		if err != nil {
			r.Refused = append(r.Refused, rel+"/: "+err.Error())
			continue
		}
		r.ArtifactDirs = append(r.ArtifactDirs, item)
		gone = append(gone, rel)
	}
	return gone
}

// removeDir removes the artifact directory rel after the checks
// cleanArtifactDirs lists (dryRun: checks and measures only).
func removeDir(realRoot, rel string, tracked []string, dryRun bool) (workspace.DirItem, error) {
	item := workspace.DirItem{Path: rel}
	full, ok := contained(realRoot, rel)
	if !ok {
		return item, errors.New("refusing a path outside the project root")
	}
	fi, err := os.Lstat(full)
	if err != nil {
		return item, err
	}
	e := workspace.DirEntry{Rel: rel, Kind: workspace.KindOther, Tracked: workspace.TrackedUnder(tracked, rel)}
	if fi.Mode()&os.ModeSymlink != 0 {
		e.Kind = workspace.KindSymlink
	} else if fi.IsDir() {
		e.Kind = workspace.KindDir
	}
	if e.Kind == workspace.KindDir {
		item.Files, item.Bytes, e.NestedRepo = measure(full, false)
	}
	if workspace.DecideDir(e) != workspace.Remove {
		return item, errors.New("not a removable artifact directory (symlink, tracked files or a nested repository)")
	}
	if dryRun {
		return item, nil
	}
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
