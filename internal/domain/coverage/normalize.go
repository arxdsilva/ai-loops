package coverage

import (
	"path"
	"strings"
)

// Normalizer says how to make a report's file names pack-relative.
type Normalizer struct {
	PackRoot    string   // absolute pack root (slash, backslash or drive form)
	Module      string   // gocover module path; "" for other formats
	SourceRoots []string // pack-relative source dirs JaCoCo package paths resolve against
	// Exists reports whether a pack-relative file exists; it picks among
	// Cobertura and JaCoCo candidates. nil = take the first candidate.
	Exists func(rel string) bool
}

// Normalize re-keys every file of p to a slash path relative to the pack
// root, merging files that land on the same path (highest count wins).
// Absolute names lose the pack root, "./" is dropped, gocover names lose
// "Module/", Cobertura names resolve against the report's <sources>, and
// JaCoCo "package/File" names against SourceRoots. Files outside the pack
// root are dropped. A Normalized profile is returned unchanged.
func Normalize(p Profile, n Normalizer) Profile {
	if p.Normalized {
		return p
	}
	root := ""
	if strings.TrimSpace(n.PackRoot) != "" {
		root = path.Clean(slash(n.PackRoot))
	}
	acc := map[blockKey]Block{}
	for f, bs := range p.Files {
		rel, ok := n.resolve(p, root, f)
		if !ok {
			continue
		}
		for _, b := range bs {
			add(acc, rel, b)
		}
	}
	return Profile{Mode: p.Mode, Format: p.Format, Sources: p.Sources, Files: collect(acc), Normalized: true}
}

func (n Normalizer) resolve(p Profile, root, f string) (string, bool) {
	s := slash(strings.TrimSpace(f))
	if isAbs(s) {
		return local(s, root)
	}
	switch p.Format {
	case "", FormatGocover:
		if n.Module == "" {
			return local(s, root)
		}
		rel, ok := strings.CutPrefix(s, n.Module+"/")
		if !ok {
			return "", false
		}
		return local(rel, root)
	case FormatCobertura:
		var abs, rel []string
		for _, src := range p.Sources {
			src = slash(src)
			if isAbs(src) {
				abs = append(abs, path.Join(src, s))
			} else {
				rel = append(rel, path.Join(src, s))
			}
		}
		return n.pick(root, append(append(abs, rel...), s))
	case FormatJacoco:
		var cands []string
		for _, sr := range n.SourceRoots {
			cands = append(cands, path.Join(slash(sr), s))
		}
		return n.pick(root, append(cands, s))
	}
	return local(s, root)
}

// pick returns the first candidate inside the pack that Exists, else the
// first candidate inside the pack.
func (n Normalizer) pick(root string, cands []string) (string, bool) {
	first := ""
	for _, c := range cands {
		rel, ok := local(c, root)
		if !ok {
			continue
		}
		if n.Exists != nil && n.Exists(rel) {
			return rel, true
		}
		if first == "" {
			first = rel
		}
	}
	return first, first != ""
}

// local makes a slash path pack-relative: an absolute one must lie under
// root; a relative one is taken as pack-relative. Paths that leave the
// pack are rejected.
func local(s, root string) (string, bool) {
	if isAbs(s) {
		s = path.Clean(s)
		if root == "" {
			return "", false
		}
		rest, ok := under(s, root)
		if !ok {
			return "", false
		}
		s = rest
	}
	s = path.Clean(s)
	if s == "." || s == ".." || strings.HasPrefix(s, "../") || isAbs(s) {
		return "", false
	}
	return s, true
}

// under strips root from the absolute path s; drive-letter (Windows) paths
// compare case-insensitively.
func under(s, root string) (string, bool) {
	pre := strings.TrimSuffix(root, "/") + "/"
	if len(s) <= len(pre) {
		return "", false
	}
	head := s[:len(pre)]
	if head == pre || (hasDrive(pre) && strings.EqualFold(head, pre)) {
		return s[len(pre):], true
	}
	return "", false
}

// slash converts backslashes and strips a file:// scheme.
func slash(s string) string {
	s = strings.ReplaceAll(s, `\`, "/")
	if rest, ok := strings.CutPrefix(s, "file://"); ok {
		s = rest
		if len(s) > 2 && s[0] == '/' && hasDrive(s[1:]) { // file:///C:/x
			s = s[1:]
		}
	}
	return s
}

func isAbs(s string) bool { return strings.HasPrefix(s, "/") || hasDrive(s) }

func hasDrive(s string) bool {
	return len(s) >= 2 && s[1] == ':' && ('a' <= s[0]|0x20 && s[0]|0x20 <= 'z')
}
