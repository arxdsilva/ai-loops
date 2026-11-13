package importscan

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/archrule"
)

// Resolver names (pack arch.resolver).
const (
	resolverPackageDir = "package-dir"
	resolverRelative   = "relative-path"
	resolverPSR4       = "namespace-psr4"
	resolverInclude    = "include-path"
	resolverCrate      = "crate-path"
	resolverTarget     = "target-name"
	resolverClojure    = "dotted-module-underscore"
	resolverCSharp     = "project-reference"
)

// Source patterns of the package-dir (Java, Kotlin, Scala), crate-path (Rust)
// and dotted-module-underscore (Clojure) resolvers.
var (
	packageLine      = regexp.MustCompile(`^\s*package\s+([A-Za-z_][\w.]*)\s*;?\s*$`)
	cratePackageName = regexp.MustCompile(`(?m)^\s*name\s*=\s*"([^"]+)"`)
	nsForm           = regexp.MustCompile(`\(\s*ns\s+(?:\^\S+\s+)*([^\s()\[\]{}]+)`)
)

// resolver maps an import string to a pack-root-relative directory.
type resolver interface {
	// index reads what resolution needs: package headers, config files.
	index(t *tree, srcs []srcFile)
	// unit is the package (directory-like unit) a source file belongs to.
	unit(rel string) string
	// resolve classifies imp, written in file from; raw is the whole regex
	// match. kind is an archrule.Import* constant; dir is set for internal.
	resolve(from, imp, raw string) (kind, dir string)
	// edges are dependencies declared outside source files (C# project
	// references).
	edges() []edge
}

type edge struct {
	unit string
	imp  archrule.Import
}

func newResolver(name string) (resolver, error) {
	switch name {
	case resolverPackageDir:
		return &packageDir{}, nil
	case resolverRelative:
		return &relativePath{}, nil
	case resolverPSR4:
		return &psr4{}, nil
	case resolverInclude:
		return &includePath{}, nil
	case resolverCrate:
		return &cratePath{}, nil
	case resolverTarget:
		return &targetName{}, nil
	case resolverClojure:
		return &clojureNS{}, nil
	case resolverCSharp:
		return &projectRef{}, nil
	}
	return nil, fmt.Errorf("unknown arch.resolver %q", name)
}

// dirUnit is the default unit: the file's directory.
type dirUnit struct{}

func (dirUnit) unit(rel string) string { return path.Dir(rel) }
func (dirUnit) edges() []edge          { return nil }

// external classifies an unresolved import: std when it starts with one of
// the prefixes (each matched whole or, with a sep, followed by it), else
// third-party.
func external(imp, sep string, std ...string) (string, string) {
	for _, s := range std {
		if imp == s || sep != "" && strings.HasPrefix(imp, s+sep) {
			return archrule.ImportStd, ""
		}
	}
	return archrule.ImportThird, ""
}

func internal(dir string) (string, string) { return archrule.ImportInternal, dir }

// longestPrefix finds the longest sep-separated prefix of imp that is a key
// of idx, and returns its first (sorted) value.
func longestPrefix(idx map[string][]string, imp, sep string) (string, bool) {
	segs := strings.Split(imp, sep)
	for i := len(segs); i > 0; i-- {
		if dirs := idx[strings.Join(segs[:i], sep)]; len(dirs) > 0 {
			return dirs[0], true
		}
	}
	return "", false
}

// addIndex appends v to idx[k] once, keeping the values sorted.
func addIndex(idx map[string][]string, k, v string) {
	for _, have := range idx[k] {
		if have == v {
			return
		}
	}
	idx[k] = append(idx[k], v)
	sort.Strings(idx[k])
}

// --- package-dir: Java, Kotlin, Scala --------------------------------------

// packageDir indexes the package header of every file, because Kotlin does
// not enforce directory = package, and maps an imported fully-qualified name
// to the directory of the files declaring its longest known package prefix.
type packageDir struct {
	dirUnit
	pkgs map[string][]string
}

func (r *packageDir) index(_ *tree, srcs []srcFile) {
	r.pkgs = map[string][]string{}
	for _, f := range srcs {
		if pkg := packageOf(f.text); pkg != "" {
			addIndex(r.pkgs, pkg, path.Dir(f.rel))
		}
	}
}

// packageOf joins the leading package clauses (Scala chains them:
// package a.b / package c is a.b.c), skipping blanks, comments and
// file annotations.
func packageOf(text string) string {
	var parts []string
	for _, line := range strings.Split(text, "\n") {
		l := strings.TrimSpace(line)
		if l == "" || isComment(l, false) || strings.HasPrefix(l, "@file:") {
			continue
		}
		m := packageLine.FindStringSubmatch(l)
		if m == nil {
			break
		}
		parts = append(parts, m[1])
	}
	return strings.Join(parts, ".")
}

func (r *packageDir) resolve(_, imp, _ string) (string, string) {
	if dir, ok := longestPrefix(r.pkgs, imp, "."); ok {
		return internal(dir)
	}
	return external(imp, ".", "java", "javax", "jdk", "sun", "kotlin", "scala")
}

// --- crate-path: Rust -------------------------------------------------------

// cratePath maps crate::a::b, super:: and self:: to the module directory
// src/a/b (the module lives in src/a/b.rs or src/a/b/mod.rs) and workspace
// crates (by their Cargo.toml [package] name) to their src directory. Other
// crates are third-party; std, core and alloc are the standard library. A
// module is its own unit: src/a/b.rs is the unit src/a/b.
type cratePath struct {
	t      *tree
	crates map[string]string
}

func (r *cratePath) index(t *tree, _ []srcFile) {
	r.t, r.crates = t, map[string]string{}
	for _, f := range t.sorted(func(f string) bool { return path.Base(f) == "Cargo.toml" }) {
		text, _ := t.read(f)
		_, pkgSection, ok := strings.Cut(text, "[package]")
		if !ok {
			continue
		}
		if m := cratePackageName.FindStringSubmatch(pkgSection); m != nil {
			r.crates[strings.ReplaceAll(m[1], "-", "_")] = path.Join(path.Dir(f), "src")
		}
	}
}

func (r *cratePath) unit(rel string) string {
	switch path.Base(rel) {
	case "mod.rs", "lib.rs", "main.rs":
		return path.Dir(rel)
	}
	return strings.TrimSuffix(rel, ".rs")
}

func (r *cratePath) edges() []edge { return nil }

// srcRoot is the nearest enclosing src directory of a file.
func (r *cratePath) srcRoot(from string) string {
	for d := path.Dir(from); d != "." && d != "/"; d = path.Dir(d) {
		if path.Base(d) == "src" {
			return d
		}
	}
	return "src"
}

func (r *cratePath) resolve(from, imp, _ string) (string, string) {
	segs := strings.Split(imp, "::")
	for len(segs) > 0 && segs[len(segs)-1] == "" {
		segs = segs[:len(segs)-1]
	}
	if len(segs) == 0 {
		return archrule.ImportThird, ""
	}
	var base string
	switch first := segs[0]; {
	case first == "crate":
		base, segs = r.srcRoot(from), segs[1:]
	case first == "self":
		base, segs = r.unit(from), segs[1:]
	case first == "super":
		base = r.unit(from)
		for len(segs) > 0 && segs[0] == "super" {
			base, segs = path.Dir(base), segs[1:]
		}
	case r.crates[first] != "":
		base, segs = r.crates[first], segs[1:]
	default:
		return external(first, "", "std", "core", "alloc", "proc_macro", "test")
	}
	for i := len(segs); i > 0; i-- {
		d := path.Join(base, strings.Join(segs[:i], "/"))
		if r.t.has(d+".rs") || r.t.has(d+"/mod.rs") || r.t.dirs[d] {
			return internal(d)
		}
	}
	return internal(base)
}

// --- target-name: Swift ------------------------------------------------------

// targetName maps import X to the SwiftPM target directory Sources/X.
type targetName struct {
	dirUnit
	t *tree
}

func (r *targetName) index(t *tree, _ []srcFile) { r.t = t }

func (r *targetName) resolve(_, imp, _ string) (string, string) {
	if d := "Sources/" + imp; r.t.dirs[d] {
		return internal(d)
	}
	return external(imp, "", "Swift", "Foundation", "FoundationEssentials", "CoreFoundation", "Dispatch",
		"Darwin", "Glibc", "Musl", "WinSDK", "os", "XCTest", "Testing", "Combine", "Observation",
		"Synchronization", "RegexBuilder", "Distributed", "_Concurrency", "SwiftUI", "UIKit", "AppKit")
}

// --- dotted-module-underscore: Clojure ---------------------------------------

// clojureNS maps a namespace to the unit of the file declaring it (ns form),
// or by path: a.b-c is src/a/b_c.clj, .cljc or .cljs. A namespace is its
// own unit: src/a/b_c.clj is the unit src/a/b_c.
type clojureNS struct {
	t  *tree
	ns map[string][]string
}

func (r *clojureNS) index(t *tree, srcs []srcFile) {
	r.t, r.ns = t, map[string][]string{}
	for _, f := range srcs {
		if m := nsForm.FindStringSubmatch(f.text); m != nil {
			addIndex(r.ns, m[1], r.unit(f.rel))
		}
	}
}

func (r *clojureNS) unit(rel string) string { return strings.TrimSuffix(rel, path.Ext(rel)) }

func (r *clojureNS) edges() []edge { return nil }

func (r *clojureNS) resolve(_, imp, _ string) (string, string) {
	if u := r.ns[imp]; len(u) > 0 {
		return internal(u[0])
	}
	p := strings.ReplaceAll(strings.ReplaceAll(imp, ".", "/"), "-", "_")
	for _, root := range []string{"src", "src/main/clojure", "src/clj", "src/cljc", "."} {
		for _, ext := range []string{".clj", ".cljc", ".cljs"} {
			if f := path.Join(root, p); r.t.has(f + ext) {
				return internal(f)
			}
		}
	}
	return external(imp, ".", "clojure", "java", "javax")
}
