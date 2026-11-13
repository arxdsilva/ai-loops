package hook

import (
	"path/filepath"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/application/langpack"
	"github.com/arxdsilva/ai-loops/internal/domain/pack"
)

// legacyGo is the Go pack as the Go-only hooks applied it: at the project
// root, gofmt (goimports when on PATH) then go vet on the file's package.
var legacyGo = langpack.Detected{ID: "go", Name: "Go", Root: ".", Pack: pack.Pack{
	ID: "go", Name: "Go", SourceGlobs: []string{"**/*.go"}, Manifest: "go.mod",
	PostEdit: &pack.PostEdit{FormatFile: []string{"gofmt", "-w", "{file}"}, LintDir: []string{"go", "vet", "./{dir}"}},
}}

// detection is one Handle call's language-pack detection result.
type detection struct {
	packs []langpack.Detected
	ok    bool // false: no Langs, or detection failed (Go-only fallback)
}

// detect returns the project's detected packs, once per Handle call. ok is
// false when there is no Langs or it failed: callers then fall back to the
// Go-only behaviour.
func (h *handler) detect(p payload) ([]langpack.Detected, bool) {
	if h.langs == nil {
		h.langs = &detection{}
		if h.d.Langs != nil {
			if root := h.repo(p); root != "" {
				if ds, err := h.d.Langs.Detect(root); err == nil {
					h.langs.packs, h.langs.ok = ds, true
				}
			}
		}
	}
	return h.langs.packs, h.langs.ok
}

// packsOf returns the packs of ds.
func packsOf(ds []langpack.Detected) []pack.Pack {
	out := make([]pack.Pack, 0, len(ds))
	for _, d := range ds {
		out = append(out, d.Pack)
	}
	return out
}

// owner picks the detected pack that owns file (absolute): the deepest pack
// root containing it for which the file is a source or test file. It
// returns the pack, its absolute root and the file relative to that root.
func owner(ds []langpack.Detected, repo, file string) (d langpack.Detected, root, rel string, ok bool) {
	best := -1
	for _, c := range ds {
		r := filepath.Join(repo, filepath.FromSlash(c.Root))
		x, err := filepath.Rel(r, file)
		if err != nil || x == ".." || strings.HasPrefix(x, ".."+string(filepath.Separator)) || filepath.IsAbs(x) {
			continue
		}
		x = filepath.ToSlash(x)
		if !c.Pack.IsSource(x) && !c.Pack.IsTest(x) {
			continue
		}
		if depth := strings.Count(filepath.ToSlash(r), "/"); depth > best {
			best, d, root, rel, ok = depth, c, r, x, true
		}
	}
	return d, root, rel, ok
}
