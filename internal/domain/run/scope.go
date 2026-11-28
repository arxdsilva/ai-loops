package run

import (
	"path"
	"slices"
	"strings"
	"time"
)

// The full gate runs once per run (WORKFLOW §6). After it is green, a file
// edited later is checked by a green scoped gate (`loops gate --pkgs`, every
// check) whose scope covers it and which started after the edit; only an
// uncovered edit makes the sources dirty again.

// ScopePattern turns one package pattern of a pack rooted at rootRel (a
// slash path relative to the project root; "" or "." = the root) into a
// scope pattern relative to the project root: "dir" (the files directly in
// dir, or dir itself when it names a file), "dir/..." (everything under
// dir) or "..." (everything). It returns "" for a pattern that names no
// path, such as a tool flag ("-p", "--workspace").
func ScopePattern(rootRel, p string) string {
	p = strings.TrimSpace(p)
	if p == "" || strings.HasPrefix(p, "-") || strings.ContainsAny(p, "*?[") {
		return ""
	}
	rec := p == "..." || strings.HasSuffix(p, "/...")
	p = strings.TrimSuffix(strings.TrimSuffix(p, "..."), "/")
	if p == "" {
		p = "."
	}
	rootRel = strings.TrimSpace(rootRel)
	if rootRel == "" {
		rootRel = "."
	}
	if path.IsAbs(p) {
		return ""
	}
	dir := path.Clean(path.Join(rootRel, p))
	if dir == ".." || strings.HasPrefix(dir, "../") {
		return ""
	}
	switch {
	case rec && dir == ".":
		return "..."
	case rec:
		return dir + "/..."
	}
	return dir
}

// ParseScope splits a GateScope event's Target into its patterns.
func ParseScope(target string) []string {
	var out []string
	for _, p := range strings.Split(target, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ScopeCovers reports whether file (a slash path relative to the project
// root) lies in one of the scope patterns (ScopePattern).
func ScopeCovers(patterns []string, file string) bool {
	file = path.Clean(strings.TrimPrefix(file, "./"))
	if path.IsAbs(file) || file == ".." || strings.HasPrefix(file, "../") {
		return false
	}
	dir := path.Dir(file)
	for _, p := range patterns {
		switch {
		case p == "...":
			return true
		case strings.HasSuffix(p, "/..."):
			base := strings.TrimSuffix(p, "/...")
			if file == base || strings.HasPrefix(file, base+"/") {
				return true
			}
		case p == file || p == dir:
			return true
		}
	}
	return false
}

// ScopeArg renders files as a `loops gate --pkgs` value: the distinct
// directories that hold them, as "./dir" patterns, sorted.
func ScopeArg(files []string) string {
	var dirs []string
	for _, f := range files {
		d := path.Dir(path.Clean(strings.TrimPrefix(f, "./")))
		p := "./" + d
		if d == "." {
			p = "."
		}
		if !slices.Contains(dirs, p) {
			dirs = append(dirs, p)
		}
	}
	slices.Sort(dirs)
	return strings.Join(dirs, ",")
}

// checkTrack is Fold's bookkeeping for the once-per-run full gate: the last
// edit of each source file, the latest full gate and the green scoped gates.
type checkTrack struct {
	edits  map[string]time.Time
	allAt  time.Time
	allOK  bool
	hasAll bool
	scopes []scopeRun
}

type scopeRun struct {
	pats []string
	at   time.Time
}

func (c *checkTrack) edit(file string, at time.Time) {
	if c.edits == nil {
		c.edits = map[string]time.Time{}
	}
	if prev, ok := c.edits[file]; !ok || at.After(prev) {
		c.edits[file] = at
	}
}

func (c *checkTrack) gate(e Event) {
	ok := e.OK != nil && *e.OK
	switch e.Name {
	case GateAll:
		c.allAt, c.allOK, c.hasAll = e.Time, ok, true
	case GateScope:
		if ok {
			c.scopes = append(c.scopes, scopeRun{pats: ParseScope(e.Target), at: e.Time})
		}
	}
}

// apply recomputes the dirty flags once the latest full gate is green: a
// source file edited after it is dirty unless a green scoped gate that
// started at or after the edit covers it. A red or missing full gate leaves
// the flags as foldGate set them.
func (c *checkTrack) apply(s *State) {
	if !c.hasAll || !c.allOK {
		return
	}
	var left []string
	for f, at := range c.edits {
		if at.After(c.allAt) && !c.covered(f, at) {
			left = append(left, f)
		}
	}
	slices.Sort(left)
	s.Unchecked = left
	s.SourceDirty = len(left) > 0
	s.GoDirty = slices.ContainsFunc(left, func(f string) bool { return strings.HasSuffix(f, ".go") })
}

func (c *checkTrack) covered(file string, at time.Time) bool {
	for _, sc := range c.scopes {
		if !sc.at.Before(at) && ScopeCovers(sc.pats, file) {
			return true
		}
	}
	return false
}

// latestAll returns the latest full-gate result, if any.
func latestAll(s State) (GateResult, bool) {
	for _, g := range s.Gates {
		if g.Name == GateAll {
			return g, true
		}
	}
	return GateResult{}, false
}

// redUnchanged reports whether the latest full gate is red and no source
// file changed after it started: running it again would change nothing.
func redUnchanged(s State) bool {
	g, ok := latestAll(s)
	return ok && !g.OK && !s.LastSourceChange.After(g.At) && !s.LastGoChange.After(g.At)
}
