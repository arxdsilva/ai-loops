package importscan

import (
	"encoding/json"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/archrule"
)

// --- relative-path: TypeScript / JavaScript ----------------------------------

// tsExts are tried, in order, after a specifier without a known file.
var tsExts = []string{".ts", ".tsx", ".mts", ".cts", ".d.ts", ".js", ".jsx", ".mjs", ".cjs"}

// jsToTS maps an emitted extension written in a specifier (ESM style
// "./x.js") to the sources it may come from.
var jsToTS = map[string][]string{".js": {".ts", ".tsx"}, ".jsx": {".tsx"}, ".mjs": {".mts"}, ".cjs": {".cts"}}

// nodeBuiltins are the Node.js core modules importable without "node:".
var nodeBuiltins = map[string]bool{
	"assert": true, "async_hooks": true, "buffer": true, "child_process": true, "cluster": true, "console": true,
	"crypto": true, "dgram": true, "diagnostics_channel": true, "dns": true, "events": true, "fs": true,
	"http": true, "http2": true, "https": true, "inspector": true, "module": true, "net": true, "os": true,
	"path": true, "perf_hooks": true, "process": true, "querystring": true, "readline": true, "repl": true,
	"stream": true, "string_decoder": true, "timers": true, "tls": true, "tty": true, "url": true,
	"util": true, "v8": true, "vm": true, "wasi": true, "worker_threads": true, "zlib": true,
}

// cHeaders are the C and POSIX headers treated as the standard library;
// extensionless headers (<vector>) are the C++ standard library.
var cHeaders = map[string]bool{
	"assert.h": true, "ctype.h": true, "errno.h": true, "fenv.h": true, "float.h": true, "inttypes.h": true,
	"limits.h": true, "locale.h": true, "math.h": true, "setjmp.h": true, "signal.h": true, "stdarg.h": true,
	"stdbool.h": true, "stddef.h": true, "stdint.h": true, "stdio.h": true, "stdlib.h": true, "string.h": true,
	"time.h": true, "uchar.h": true, "wchar.h": true, "wctype.h": true, "unistd.h": true, "fcntl.h": true,
	"pthread.h": true, "dirent.h": true, "dlfcn.h": true, "poll.h": true, "netdb.h": true,
}

// includeRoots are tried after the including file's directory.
var includeRoots = []string{"include", "src", "."}

// Source patterns of the tsconfig (JSON with comments) reader and the C#
// project-reference resolver.
var (
	trailingComma = regexp.MustCompile(`,(\s*[}\]])`)
	csNamespace   = regexp.MustCompile(`(?m)^\s*namespace\s+([A-Za-z_][\w.]*)`)
	csProjectRef  = regexp.MustCompile(`<ProjectReference\s+Include\s*=\s*"([^"]+)"`)
)

// relativePath resolves relative specifiers against the importing file,
// tsconfig.json compilerOptions.paths aliases (exact keys and one "*") and
// baseUrl, trying the TypeScript extensions and index files. Node core
// modules (and node:*) are the standard library; other bare specifiers are
// third-party. A tsconfig "extends" chain is not followed.
type relativePath struct {
	dirUnit
	t       *tree
	baseURL string
	aliases []alias
}

type alias struct {
	key     string // "@app/*" or "@config"
	targets []string
}

type tsconfig struct {
	CompilerOptions struct {
		BaseURL string              `json:"baseUrl"`
		Paths   map[string][]string `json:"paths"`
	} `json:"compilerOptions"`
}

func (r *relativePath) index(t *tree, _ []srcFile) {
	r.t = t
	for _, name := range []string{"tsconfig.json", "tsconfig.base.json"} {
		text, ok := t.read(name)
		if !ok {
			continue
		}
		var c tsconfig
		if json.Unmarshal([]byte(stripJSONC(text)), &c) != nil {
			continue
		}
		r.baseURL = path.Clean(c.CompilerOptions.BaseURL)
		if c.CompilerOptions.BaseURL == "" {
			r.baseURL = ""
		}
		for k, v := range c.CompilerOptions.Paths {
			r.aliases = append(r.aliases, alias{key: k, targets: v})
		}
		// the longest literal prefix wins, as in tsc
		sort.Slice(r.aliases, func(i, j int) bool {
			a, b := strings.TrimSuffix(r.aliases[i].key, "*"), strings.TrimSuffix(r.aliases[j].key, "*")
			if len(a) != len(b) {
				return len(a) > len(b)
			}
			return a < b
		})
		return
	}
}

func (r *relativePath) resolve(from, imp, _ string) (string, string) {
	if strings.HasPrefix(imp, "node:") || nodeBuiltins[strings.SplitN(imp, "/", 2)[0]] {
		return archrule.ImportStd, ""
	}
	if strings.HasPrefix(imp, "./") || strings.HasPrefix(imp, "../") || imp == "." || imp == ".." {
		target := path.Join(path.Dir(from), imp)
		if !inside(target) {
			return archrule.ImportThird, ""
		}
		return internal(r.dirOf(target))
	}
	base := r.baseURL
	if base == "" {
		base = "."
	}
	for _, a := range r.aliases {
		star, ok := matchAlias(a.key, imp)
		if !ok {
			continue
		}
		var first string
		for _, tgt := range a.targets {
			cand := path.Join(base, strings.Replace(tgt, "*", star, 1))
			if !inside(cand) {
				continue
			}
			if f, ok := r.file(cand); ok {
				return internal(path.Dir(f))
			}
			if first == "" {
				first = cand
			}
		}
		if first != "" {
			return internal(r.dirOf(first))
		}
	}
	if r.baseURL != "" {
		if f, ok := r.file(path.Join(r.baseURL, imp)); ok {
			return internal(path.Dir(f))
		}
	}
	return archrule.ImportThird, ""
}

// matchAlias matches a paths key: exact, or one "*" capturing the rest.
func matchAlias(key, imp string) (string, bool) {
	pre, suf, star := strings.Cut(key, "*")
	if !star {
		return "", key == imp
	}
	if len(imp) < len(pre)+len(suf) || !strings.HasPrefix(imp, pre) || !strings.HasSuffix(imp, suf) {
		return "", false
	}
	return imp[len(pre) : len(imp)-len(suf)], true
}

// file finds the source a specifier target names.
func (r *relativePath) file(target string) (string, bool) {
	if r.t.has(target) {
		return target, true
	}
	ext := path.Ext(target)
	for _, alt := range jsToTS[ext] {
		if f := strings.TrimSuffix(target, ext) + alt; r.t.has(f) {
			return f, true
		}
	}
	for _, e := range tsExts {
		if r.t.has(target + e) {
			return target + e, true
		}
	}
	for _, e := range tsExts {
		if f := target + "/index" + e; r.t.has(f) {
			return f, true
		}
	}
	return "", false
}

// dirOf is the directory of the file target names, or target itself when it
// is a directory, or its parent when nothing matches.
func (r *relativePath) dirOf(target string) string {
	if f, ok := r.file(target); ok {
		return path.Dir(f)
	}
	if r.t.dirs[target] {
		return target
	}
	return path.Dir(target)
}

// stripJSONC removes // and /* */ comments outside strings and trailing
// commas, which tsconfig.json allows.
func stripJSONC(s string) string {
	var b strings.Builder
	inStr := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inStr:
			b.WriteByte(c)
			if c == '\\' && i+1 < len(s) {
				i++
				b.WriteByte(s[i])
			} else if c == '"' {
				inStr = false
			}
		case c == '"':
			inStr = true
			b.WriteByte(c)
		case c == '/' && i+1 < len(s) && s[i+1] == '/':
			for i < len(s) && s[i] != '\n' {
				i++
			}
			b.WriteByte('\n')
		case c == '/' && i+1 < len(s) && s[i+1] == '*':
			end := strings.Index(s[i+2:], "*/")
			if end < 0 {
				i = len(s)
			} else {
				i += end + 3
			}
		default:
			b.WriteByte(c)
		}
	}
	return trailingComma.ReplaceAllString(b.String(), "$1")
}

// --- namespace-psr4: PHP ---------------------------------------------------

// psr4 maps a namespaced name through composer.json autoload(-dev).psr-4 to
// a directory: the namespace's own directory when it exists, else the
// directory of the class file. A name without a namespace separator is a
// global class (DateTime, PDO): the standard library.
type psr4 struct {
	dirUnit
	t     *tree
	roots []psr4Root
}

type psr4Root struct {
	prefix string // ends in \
	dirs   []string
}

func (r *psr4) index(t *tree, _ []srcFile) {
	r.t = t
	text, ok := t.read("composer.json")
	if !ok {
		return
	}
	var c map[string]map[string]json.RawMessage // section -> standard -> raw
	if json.Unmarshal([]byte(text), &c) != nil {
		return
	}
	for _, section := range []string{"autoload", "autoload-dev"} {
		var m map[string]json.RawMessage
		if raw := c[section]["psr-4"]; raw == nil || json.Unmarshal(raw, &m) != nil {
			continue
		}
		for ns, v := range m {
			var dirs []string
			var one string
			if json.Unmarshal(v, &one) == nil {
				dirs = []string{one}
			} else if json.Unmarshal(v, &dirs) != nil {
				continue
			}
			root := psr4Root{prefix: strings.TrimSuffix(ns, `\`) + `\`}
			for _, d := range dirs {
				root.dirs = append(root.dirs, path.Clean(strings.TrimPrefix(d, "./")))
			}
			r.roots = append(r.roots, root)
		}
	}
	sort.Slice(r.roots, func(i, j int) bool {
		if len(r.roots[i].prefix) != len(r.roots[j].prefix) {
			return len(r.roots[i].prefix) > len(r.roots[j].prefix)
		}
		return r.roots[i].prefix < r.roots[j].prefix
	})
}

func (r *psr4) resolve(_, imp, _ string) (string, string) {
	imp = strings.TrimPrefix(imp, `\`)
	for _, root := range r.roots {
		if !strings.HasPrefix(imp+`\`, root.prefix) {
			continue
		}
		rest := strings.ReplaceAll(strings.TrimSuffix(strings.TrimPrefix(imp+`\`, root.prefix), `\`), `\`, "/")
		for _, d := range root.dirs {
			full := path.Join(d, rest)
			if r.t.dirs[full] {
				return internal(full)
			}
			if r.t.has(full + ".php") {
				return internal(path.Dir(full + ".php"))
			}
		}
		return internal(path.Dir(path.Join(root.dirs[0], rest+".php")))
	}
	if !strings.Contains(imp, `\`) {
		return archrule.ImportStd, ""
	}
	return archrule.ImportThird, ""
}

// --- include-path: C++ ---------------------------------------------------------

// includePath resolves a quoted include relative to the including file,
// then (quoted or angled) against include/, src/ and the pack root.
type includePath struct {
	dirUnit
	t *tree
}

func (r *includePath) index(t *tree, _ []srcFile) { r.t = t }

func (r *includePath) resolve(from, imp, raw string) (string, string) {
	var cands []string
	if !strings.Contains(raw, "<") {
		cands = append(cands, path.Join(path.Dir(from), imp))
	}
	for _, root := range includeRoots {
		cands = append(cands, path.Join(root, imp))
	}
	for _, c := range cands {
		if inside(c) && r.t.has(c) {
			return internal(path.Dir(c))
		}
	}
	if path.Ext(imp) == "" || cHeaders[imp] || strings.HasPrefix(imp, "sys/") || strings.HasPrefix(imp, "arpa/") || strings.HasPrefix(imp, "netinet/") {
		return archrule.ImportStd, ""
	}
	return archrule.ImportThird, ""
}

// --- project-reference: C# -----------------------------------------------------

// projectRef makes the project directory (the nearest one holding a
// .csproj) the unit. A using directive resolves through the namespaces the
// sources declare to the declaring project; System.* is the standard
// library. Each <ProjectReference> is an edge from its project too.
type projectRef struct {
	projects []string // project dirs, deepest first
	ns       map[string][]string
	refs     []edge
}

func (r *projectRef) index(t *tree, srcs []srcFile) {
	r.ns = map[string][]string{}
	r.refs = nil
	projs := t.sorted(func(f string) bool { return strings.HasSuffix(f, ".csproj") })
	r.projects = nil
	for _, p := range projs {
		r.projects = append(r.projects, path.Dir(p))
	}
	sort.Slice(r.projects, func(i, j int) bool { return len(r.projects[i]) > len(r.projects[j]) })
	for _, f := range srcs {
		for _, m := range csNamespace.FindAllStringSubmatch(f.text, -1) {
			addIndex(r.ns, m[1], r.unit(f.rel))
		}
	}
	for _, p := range projs {
		text, _ := t.read(p)
		lines := lineStarts(text)
		for _, loc := range csProjectRef.FindAllStringSubmatchIndex(text, -1) {
			inc := strings.ReplaceAll(text[loc[2]:loc[3]], `\`, "/")
			target := path.Dir(path.Join(path.Dir(p), inc))
			imp := archrule.Import{
				Path: strings.TrimSuffix(path.Base(inc), ".csproj"),
				Pos:  archrule.Pos{File: p, Line: sort.SearchInts(lines, loc[0]+1)},
				Kind: archrule.ImportThird,
			}
			if inside(target) {
				imp.Kind, imp.Dir = archrule.ImportInternal, target
			}
			r.refs = append(r.refs, edge{unit: path.Dir(p), imp: imp})
		}
	}
}

func (r *projectRef) unit(rel string) string {
	dir := path.Dir(rel)
	for _, p := range r.projects {
		if under(dir, p) {
			return p
		}
	}
	return dir
}

func (r *projectRef) edges() []edge { return r.refs }

func (r *projectRef) resolve(_, imp, _ string) (string, string) {
	if dir, ok := longestPrefix(r.ns, imp, "."); ok {
		return internal(dir)
	}
	return external(imp, ".", "System")
}
