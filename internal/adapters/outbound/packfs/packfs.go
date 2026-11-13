// Package packfs reads language packs from an fs.FS (the embedded plugin
// tree) and gives the pack detector read access to a project on disk.
package packfs

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/pack"
)

// PackGlob is where packs live in the plugin tree.
const PackGlob = "specs/languages/*/pack.json"

// maxRead caps how much of a marker file is read for "contains" checks.
const maxRead = 1 << 20

// Embedded reads packs from a plugin tree.
type Embedded struct{ FS fs.FS }

// PackFiles returns every specs/languages/*/pack.json.
func (e Embedded) PackFiles() (map[string][]byte, error) {
	names, err := fs.Glob(e.FS, PackGlob)
	if err != nil {
		return nil, err
	}
	out := map[string][]byte{}
	for _, n := range names {
		b, err := fs.ReadFile(e.FS, n)
		if err != nil {
			return nil, err
		}
		out[n] = b
	}
	return out, nil
}

// OS reads the project tree from disk.
type OS struct{}

// View returns a pack.View rooted at root/dir.
func (OS) View(root, dir string) pack.View {
	return osView{base: filepath.Join(root, filepath.FromSlash(dir))}
}

// Dirs lists directories under root down to depth (slash paths, "." first),
// not following symlinks and skipping names for which skip is true.
func (OS) Dirs(root string, depth int, skip func(string) bool) ([]string, error) {
	out := []string{"."}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == root {
				return err
			}
			return nil // unreadable subtree: not a candidate
		}
		if p == root || !d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if skip(d.Name()) {
			return filepath.SkipDir
		}
		if strings.Count(rel, "/")+1 > depth {
			return filepath.SkipDir
		}
		out = append(out, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out[1:])
	return out, nil
}

type osView struct{ base string }

func (v osView) abs(rel string) string {
	return filepath.Join(v.base, filepath.FromSlash(path.Clean(rel)))
}

func (v osView) Exists(rel string) bool {
	_, err := os.Stat(v.abs(rel))
	return err == nil
}

func (v osView) Read(rel string) (string, bool) {
	f, err := os.Open(v.abs(rel))
	if err != nil {
		return "", false
	}
	defer f.Close()
	buf := make([]byte, maxRead)
	n, _ := f.Read(buf)
	return string(buf[:n]), n > 0
}

func (v osView) List(dir string) []string {
	es, err := os.ReadDir(v.abs(dir))
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(es))
	for _, e := range es {
		out = append(out, e.Name())
	}
	return out
}
