// Package langpack loads the embedded language packs and detects which of
// them apply to a project, and where (polyglot repos get one pack per root).
// The pack files and the project tree are reached through consumer-owned
// ports.
package langpack

import (
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/pack"
)

// Source returns the raw pack files, keyed by a name used in errors
// (e.g. "specs/languages/go/pack.json").
type Source interface {
	PackFiles() (map[string][]byte, error)
}

// Tree reads the project being worked on.
type Tree interface {
	// View returns read access rooted at root/dir (dir is a slash path, "." = root).
	View(root, dir string) pack.View
	// Dirs lists root's directories (slash paths, "." first) down to depth,
	// skipping any directory for which skip(name) is true.
	Dirs(root string, depth int, skip func(name string) bool) ([]string, error)
}

// Override pins a pack to a root (.loops/config.json "languages"); a
// non-empty list replaces detection.
type Override struct {
	ID      string `json:"id"`
	Root    string `json:"root,omitempty"`
	Variant string `json:"variant,omitempty"`
}

// Detected is one pack applied at one root, with its variant resolved.
type Detected struct {
	Pack pack.Pack `json:"-"`
	ID   string    `json:"id"`
	Name string    `json:"name"`
	Root string    `json:"root"`
	// Variant is the build-tool variant in effect ("" = the base pack).
	Variant string `json:"variant,omitempty"`
}

// DetectDepth is how deep detection looks for pack roots below the project root.
const DetectDepth = 3

// skipDirs are never pack roots: dependencies, build output, caches.
var skipDirs = map[string]bool{
	"vendor": true, "node_modules": true, "target": true, "build": true, "_build": true, "deps": true,
	"dist": true, "out": true, "bin": true, "obj": true, "testdata": true, "coverage": true,
	"__pycache__": true, "venv": true, "platforms": true,
}

// SkipDir reports whether detection ignores a directory with this name.
func SkipDir(name string) bool { return strings.HasPrefix(name, ".") || skipDirs[name] }

// Service loads and detects packs.
type Service struct {
	src  Source
	tree Tree
}

// New builds a Service.
func New(src Source, tree Tree) *Service { return &Service{src: src, tree: tree} }

// All parses every pack, sorted by id; any invalid pack is an error.
func (s *Service) All() ([]pack.Pack, error) {
	files, err := s.src.PackFiles()
	if err != nil {
		return nil, err
	}
	var out []pack.Pack
	var errs []error
	ids := map[string]string{}
	for name, b := range files {
		p, err := pack.Parse(b)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
			continue
		}
		if want := path.Base(path.Dir(name)); want != "." && want != p.ID {
			errs = append(errs, fmt.Errorf("%s: id %q does not match its directory %q", name, p.ID, want))
		}
		if prev, dup := ids[p.ID]; dup {
			errs = append(errs, fmt.Errorf("%s: id %q already defined in %s", name, p.ID, prev))
		}
		ids[p.ID] = name
		out = append(out, p)
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// Get returns the pack with this id.
func (s *Service) Get(id string) (pack.Pack, error) {
	all, err := s.All()
	if err != nil {
		return pack.Pack{}, err
	}
	for _, p := range all {
		if p.ID == id {
			return p, nil
		}
	}
	return pack.Pack{}, fmt.Errorf("no language pack %q (known: %s)", id, strings.Join(ids(all), ", "))
}

// Detect returns the packs that apply to the project at root. With
// overrides, exactly those are returned (each variant resolved at its root,
// or pinned). Otherwise every directory down to DetectDepth is a candidate;
// a pack already detected at an ancestor is not detected again below it
// (Maven modules, Cargo workspace members), but different packs may nest.
func (s *Service) Detect(root string, overrides []Override) ([]Detected, error) {
	all, err := s.All()
	if err != nil {
		return nil, err
	}
	byID := map[string]pack.Pack{}
	for _, p := range all {
		byID[p.ID] = p
	}
	if len(overrides) > 0 {
		return s.pinned(root, byID, overrides)
	}
	dirs, err := s.tree.Dirs(root, DetectDepth, SkipDir)
	if err != nil {
		return nil, err
	}
	sort.Slice(dirs, func(i, j int) bool {
		if di, dj := depth(dirs[i]), depth(dirs[j]); di != dj {
			return di < dj
		}
		return dirs[i] < dirs[j]
	})
	var out []Detected
	for _, dir := range dirs {
		v := s.tree.View(root, dir)
		for _, p := range all {
			if covered(out, p.ID, dir) || !p.Detected(v) {
				continue
			}
			out = append(out, detected(p.Resolve(v), dir))
		}
	}
	return out, nil
}

func (s *Service) pinned(root string, byID map[string]pack.Pack, ovs []Override) ([]Detected, error) {
	var out []Detected
	for _, o := range ovs {
		p, ok := byID[o.ID]
		if !ok {
			return nil, fmt.Errorf("languages: no pack %q", o.ID)
		}
		dir := path.Clean(strings.TrimPrefix(o.Root, "./"))
		if dir == "" || strings.HasPrefix(dir, "..") || path.IsAbs(dir) {
			dir = "."
		}
		var err error
		if o.Variant != "" {
			p, err = p.WithVariant(o.Variant)
			if err != nil {
				return nil, err
			}
		} else {
			p = p.Resolve(s.tree.View(root, dir))
		}
		out = append(out, detected(p, dir))
	}
	return out, nil
}

func detected(p pack.Pack, dir string) Detected {
	return Detected{Pack: p, ID: p.ID, Name: p.Name, Root: dir, Variant: p.Variant}
}

// covered reports whether pack id is already detected at dir or an ancestor.
func covered(ds []Detected, id, dir string) bool {
	for _, d := range ds {
		if d.ID == id && (d.Root == "." || d.Root == dir || strings.HasPrefix(dir, d.Root+"/")) {
			return true
		}
	}
	return false
}

func depth(dir string) int {
	if dir == "." {
		return 0
	}
	return strings.Count(dir, "/") + 1
}

func ids(ps []pack.Pack) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.ID)
	}
	return out
}
