// Package importscan builds archrule.Module for a language pack whose
// arch.parser is "regex" (research/languages/TOOLING.md §6.1): it walks the
// pack root, reads the pack's source files, matches arch.import_regex and
// hands every import string to the pack's arch.resolver, which maps it to a
// pack-root-relative directory (internal), the standard library or a
// third-party package. It also records the arch.impure_calls hits of every
// source line; archrule reports them in domain packages.
//
// It is text only. Known blind spots: imports in block comments or strings
// that the regex matches, impure-call names inside strings, and anything
// a build tool computes (generated sources, source sets declared in code).
package importscan

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/application/langpack"
	"github.com/arxdsilva/ai-loops/internal/domain/archrule"
	"github.com/arxdsilva/ai-loops/internal/domain/pack"
)

// scanner implements archcheck.Scanner.
type scanner struct{}

// New returns a scanner.
func New() *scanner { return &scanner{} }

// srcFile is one source file read from the pack root.
type srcFile struct {
	rel  string // pack-root-relative, slash-separated
	text string
}

// tree is the walked pack root.
type tree struct {
	root  string
	files map[string]bool // every non-hidden file
	dirs  map[string]bool // every walked directory, "." included
	cache map[string]string
}

func (t *tree) has(rel string) bool { return t.files[rel] }

// read returns a file's content; missing or unreadable files read as absent.
func (t *tree) read(rel string) (string, bool) {
	if s, ok := t.cache[rel]; ok {
		return s, true
	}
	if !t.files[rel] {
		return "", false
	}
	b, err := os.ReadFile(filepath.Join(t.root, filepath.FromSlash(rel)))
	if err != nil {
		return "", false
	}
	t.cache[rel] = string(b)
	return string(b), true
}

// sorted lists the files for which keep is true.
func (t *tree) sorted(keep func(string) bool) []string {
	var out []string
	for f := range t.files {
		if keep(f) {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}

// Scan builds the module model of the pack p rooted at root.
func (s *scanner) Scan(ctx context.Context, root string, p pack.Pack) (archrule.Module, error) {
	if p.Arch.Parser != pack.ParserRegex {
		return archrule.Module{}, fmt.Errorf("importscan: pack %s has arch.parser %q, want regex", p.ID, p.Arch.Parser)
	}
	re, err := regexp.Compile("(?m)" + p.Arch.ImportRegex)
	if err != nil {
		return archrule.Module{}, fmt.Errorf("importscan: pack %s: arch.import_regex: %w", p.ID, err)
	}
	res, err := newResolver(p.Arch.Resolver)
	if err != nil {
		return archrule.Module{}, fmt.Errorf("importscan: pack %s: %w", p.ID, err)
	}
	t, err := walk(ctx, root)
	if err != nil {
		return archrule.Module{}, err
	}
	var srcs []srcFile
	var tests []string
	for _, rel := range t.sorted(func(string) bool { return true }) {
		if err := ctx.Err(); err != nil {
			return archrule.Module{}, err
		}
		switch {
		case p.IsSource(rel):
			text, ok := t.read(rel)
			if ok && !p.IsGenerated(rel, text) {
				srcs = append(srcs, srcFile{rel: rel, text: text})
			}
		case p.IsTest(rel):
			tests = append(tests, rel)
		}
	}
	res.index(t, srcs)
	return build(p, re, res, srcs, tests), nil
}

// build groups the files into packages (the resolver's units) and fills
// their imports, impure hits, line counts and test files.
func build(p pack.Pack, re *regexp.Regexp, res resolver, srcs []srcFile, tests []string) archrule.Module {
	pkgs := map[string]*archrule.Package{}
	get := func(unit string) *archrule.Package {
		if pkgs[unit] == nil {
			pkgs[unit] = &archrule.Package{Dir: unit, ImportPath: unit}
		}
		return pkgs[unit]
	}
	extend := p.Arch.Resolver == resolverCrate
	for _, f := range srcs {
		pk := get(res.unit(f.rel))
		pk.Files = append(pk.Files, f.rel)
		pk.Lines += strings.Count(f.text, "\n")
		pk.Imports = append(pk.Imports, imports(f, re, res, extend)...)
		pk.Impure = append(pk.Impure, impureHits(f, p.Arch.ImpureCalls)...)
	}
	for _, rel := range tests {
		pk := get(res.unit(rel))
		pk.TestFiles = append(pk.TestFiles, rel)
	}
	for _, e := range res.edges() {
		pk := get(e.unit)
		pk.Imports = append(pk.Imports, e.imp)
	}
	m := archrule.Module{Path: p.ID, Parser: pack.ParserRegex}
	for _, pk := range pkgs {
		m.Packages = append(m.Packages, *pk)
	}
	sort.Slice(m.Packages, func(i, j int) bool { return m.Packages[i].Dir < m.Packages[j].Dir })
	return m
}

// imports matches re over the whole file (so multi-line import forms
// match) and resolves each import string: the first non-empty capture
// group, or the whole match when the regex has none.
func imports(f srcFile, re *regexp.Regexp, res resolver, extend bool) []archrule.Import {
	lines := lineStarts(f.text)
	var out []archrule.Import
	for _, loc := range re.FindAllStringSubmatchIndex(f.text, -1) {
		s, e := loc[0], loc[1]
		for g := 1; 2*g < len(loc); g++ {
			if loc[2*g] >= 0 && loc[2*g+1] > loc[2*g] {
				s, e = loc[2*g], loc[2*g+1]
				break
			}
		}
		if extend {
			e = extendPath(f.text, e)
		}
		line := sort.SearchInts(lines, s+1)
		if isComment(f.text[lines[line-1]:s], false) {
			continue
		}
		imp := f.text[s:e]
		kind, dir := res.resolve(f.rel, imp, f.text[loc[0]:loc[1]])
		out = append(out, archrule.Import{Path: imp, Pos: archrule.Pos{File: f.rel, Line: line}, Kind: kind, Dir: dir})
	}
	return out
}

// lineStarts returns the offset of every line start; line n (1-based)
// starts at out[n-1].
func lineStarts(text string) []int {
	out := []int{0}
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			out = append(out, i+1)
		}
	}
	return out
}

// extendPath grows a Rust path capture over the following ::segment parts,
// for import regexes that capture only the first segment.
func extendPath(text string, e int) int {
	for strings.HasPrefix(text[e:], "::") {
		j := e + 2
		for j < len(text) && isIdent(text[j]) {
			j++
		}
		if j == e+2 {
			break
		}
		e = j
	}
	return e
}

func isIdent(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// isComment reports whether a line (or the line prefix before a match)
// starts a comment: //, /*, a block-comment "* " body line, ; (Lisp) or --.
// With hash, # starts one too (the purity scan; imports keep it for C++
// #include).
func isComment(line string, hash bool) bool {
	l := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(l, "//"), strings.HasPrefix(l, "/*"), strings.HasPrefix(l, ";"), strings.HasPrefix(l, "--"):
		return true
	case l == "*", strings.HasPrefix(l, "* "), strings.HasPrefix(l, "*/"):
		return true
	case hash && strings.HasPrefix(l, "#"):
		return true
	}
	return false
}

// impureHits records one hit per impure_calls entry per non-comment line.
func impureHits(f srcFile, calls []string) []archrule.Hit {
	if len(calls) == 0 {
		return nil
	}
	var out []archrule.Hit
	for i, line := range strings.Split(f.text, "\n") {
		if isComment(line, true) {
			continue
		}
		for _, c := range calls {
			if c != "" && strings.Contains(line, c) {
				out = append(out, archrule.Hit{Call: c, Pos: archrule.Pos{File: f.rel, Line: i + 1}})
			}
		}
	}
	return out
}

// walk lists root's files and directories, skipping hidden directories and
// the ones language detection skips (dependencies, build output, caches).
func walk(ctx context.Context, root string) (*tree, error) {
	t := &tree{root: root, files: map[string]bool{}, dirs: map[string]bool{}, cache: map[string]string{}}
	err := filepath.WalkDir(root, func(abs string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(root, abs)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel != "." && langpack.SkipDir(d.Name()) {
				return filepath.SkipDir
			}
			t.dirs[rel] = true
			return nil
		}
		if d.Type().IsRegular() && !strings.HasPrefix(d.Name(), ".") {
			t.files[rel] = true
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("importscan: walk %s: %w", root, err)
	}
	return t, nil
}

// under reports whether rel is dir or below it.
func under(rel, dir string) bool {
	return dir == "." || rel == dir || strings.HasPrefix(rel, dir+"/")
}

// inside reports whether a cleaned relative path stays inside the root.
func inside(rel string) bool {
	return rel != ".." && !strings.HasPrefix(rel, "../") && !path.IsAbs(rel)
}
