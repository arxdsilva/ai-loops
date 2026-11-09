package goparse

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/insurgencygames/ai-loops/internal/domain/archrule"
)

// pkgScan gathers the facts of one package while its files are parsed.
type pkgScan struct {
	fset  *token.FileSet
	pkg   archrule.Package
	types map[string]*archrule.Type
	order []string // type names in first-seen order
}

func parsePackage(modPath string, d dirInfo) (archrule.Package, error) {
	s := &pkgScan{fset: token.NewFileSet(), types: map[string]*archrule.Type{}}
	s.pkg.Dir = d.rel
	s.pkg.ImportPath = modPath
	if d.rel != "." {
		s.pkg.ImportPath = modPath + "/" + d.rel
	}
	var files, tests []*ast.File
	for _, name := range d.files {
		af, src, err := s.parse(d, name)
		if err != nil {
			return archrule.Package{}, err
		}
		files = append(files, af)
		s.pkg.Files = append(s.pkg.Files, s.rel(d, name))
		s.pkg.Lines += strings.Count(string(src), "\n")
		s.file(af)
		if name == "main.go" {
			s.pkg.Main = s.mainFile(af, s.rel(d, name), src)
		}
	}
	for _, name := range d.testFiles {
		af, _, err := s.parse(d, name)
		if err != nil {
			return archrule.Package{}, err
		}
		tests = append(tests, af)
		s.pkg.TestFiles = append(s.pkg.TestFiles, s.rel(d, name))
	}
	s.pkg.Globals = assignedGlobals(s.fset, s.pkg.ImportPath, files, tests)
	for _, n := range s.order {
		s.pkg.Types = append(s.pkg.Types, *s.types[n])
	}
	return s.pkg, nil
}

func (s *pkgScan) rel(d dirInfo, name string) string {
	if d.rel == "." {
		return name
	}
	return path.Join(d.rel, name)
}

// parse reads and parses one file, naming it by its module-relative path
// so that every position the file set reports is module-relative.
func (s *pkgScan) parse(d dirInfo, name string) (*ast.File, []byte, error) {
	src, err := os.ReadFile(filepath.Join(d.abs, name))
	if err != nil {
		return nil, nil, err
	}
	af, err := parser.ParseFile(s.fset, s.rel(d, name), src, parser.SkipObjectResolution)
	if err != nil {
		return nil, nil, err
	}
	return af, src, nil
}

func (s *pkgScan) pos(p token.Pos) archrule.Pos {
	pp := s.fset.Position(p)
	return archrule.Pos{File: pp.Filename, Line: pp.Line}
}

// file gathers the facts of one non-test file.
func (s *pkgScan) file(af *ast.File) {
	for _, imp := range af.Imports {
		ip, _ := strconv.Unquote(imp.Path.Value)
		s.pkg.Imports = append(s.pkg.Imports, archrule.Import{Path: ip, Pos: s.pos(imp.Pos())})
	}
	for _, d := range af.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Recv != nil {
				s.typ(receiverType(d.Recv), d.Pos()).Methods++
			} else if d.Name.Name == "init" {
				s.pkg.Inits = append(s.pkg.Inits, s.pos(d.Pos()))
			}
		case *ast.GenDecl:
			s.typeDecls(d)
		}
	}
	s.uses(af)
}

// typ returns the type named name, creating it at p when first seen.
func (s *pkgScan) typ(name string, p token.Pos) *archrule.Type {
	t, ok := s.types[name]
	if !ok {
		t = &archrule.Type{Name: name, Pos: s.pos(p)}
		s.types[name] = t
		s.order = append(s.order, name)
	}
	return t
}

func (s *pkgScan) typeDecls(d *ast.GenDecl) {
	if d.Tok != token.TYPE {
		return
	}
	for _, spec := range d.Specs {
		ts := spec.(*ast.TypeSpec)
		t := s.typ(ts.Name.Name, ts.Pos())
		t.Pos = s.pos(ts.Pos()) // the declaration wins over a method seen first
		switch tt := ts.Type.(type) {
		case *ast.StructType:
			t.Kind, t.Fields = archrule.KindStruct, countFields(tt.Fields)
		case *ast.InterfaceType:
			t.Kind, t.IfaceMethods = archrule.KindInterface, countFields(tt.Methods)
		default:
			t.Kind = archrule.KindOther
		}
	}
}

// watched are the packages whose selected functions the domain rules need.
var watched = map[string]bool{"time": true, "math/rand": true, "math/rand/v2": true}

// importNames maps the local names of the watched imports of af to their
// paths. Dot and blank imports are not tracked.
func importNames(af *ast.File) map[string]string {
	names := map[string]string{}
	for _, imp := range af.Imports {
		ip, _ := strconv.Unquote(imp.Path.Value)
		if !watched[ip] {
			continue
		}
		name := defaultName(ip)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		if name != "_" && name != "." {
			names[name] = ip
		}
	}
	return names
}

// defaultName is a package's name as its import path suggests: the last
// element, or the one before a /vN major-version suffix.
func defaultName(ip string) string {
	elems := strings.Split(ip, "/")
	last := elems[len(elems)-1]
	if len(elems) > 1 && len(last) > 1 && last[0] == 'v' && isDigits(last[1:]) {
		return elems[len(elems)-2]
	}
	return last
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// uses records go statements, time.Now and every math/rand(/v2) selector,
// whether called, taken as a value or used as a type: the rules decide.
func (s *pkgScan) uses(af *ast.File) {
	names := importNames(af)
	ast.Inspect(af, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.GoStmt:
			s.pkg.GoStmts = append(s.pkg.GoStmts, s.pos(x.Pos()))
		case *ast.SelectorExpr:
			ip, sel := selected(x, names)
			if (ip == "time" && sel == "Now") || ip == "math/rand" || ip == "math/rand/v2" {
				s.pkg.Calls = append(s.pkg.Calls, archrule.Call{Pkg: ip, Name: sel, Pos: s.pos(x.Pos())})
			}
		}
		return true
	})
}

// selected resolves pkg.Name (possibly instantiated: pkg.Name[T]) against
// the file's import names.
func selected(e ast.Expr, names map[string]string) (string, string) {
	for {
		switch x := e.(type) {
		case *ast.ParenExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.IndexListExpr:
			e = x.X
		case *ast.SelectorExpr:
			id, ok := x.X.(*ast.Ident)
			if !ok {
				return "", ""
			}
			return names[id.Name], x.Sel.Name
		default:
			return "", ""
		}
	}
}

// mainFile gathers what the cmd rule needs from a main.go.
func (s *pkgScan) mainFile(af *ast.File, rel string, src []byte) *archrule.MainFile {
	mf := &archrule.MainFile{File: rel, Lines: strings.Count(string(src), "\n")}
	for _, d := range af.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			what := "func " + d.Name.Name
			if d.Recv != nil {
				what = "method " + d.Name.Name
			}
			mf.Decls = append(mf.Decls, archrule.Fact{What: what, Pos: s.pos(d.Pos())})
		case *ast.GenDecl:
			if d.Tok != token.IMPORT {
				mf.Decls = append(mf.Decls, archrule.Fact{What: d.Tok.String() + " declaration", Pos: s.pos(d.Pos())})
			}
		}
	}
	logic := func(n ast.Node, what string) {
		mf.Logic = append(mf.Logic, archrule.Fact{What: what, Pos: s.pos(n.Pos())})
	}
	ast.Inspect(af, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.ImportSpec:
			return false // import paths are not the string literals the rule means
		case *ast.ForStmt, *ast.RangeStmt:
			logic(x, "for/range statement")
		case *ast.SwitchStmt, *ast.TypeSwitchStmt:
			logic(x, "switch statement")
		case *ast.SelectStmt:
			logic(x, "select statement")
		case *ast.GoStmt:
			logic(x, "go statement")
		case *ast.FuncLit:
			logic(x, "function literal")
		case *ast.BasicLit:
			if x.Kind == token.STRING {
				mf.Strings = append(mf.Strings, archrule.StringLit{Runes: utf8.RuneCountInString(x.Value) - 2, Pos: s.pos(x.Pos())})
			}
		}
		return true
	})
	return mf
}

// receiverType is the bare type name of a method receiver: *T, T, T[K].
func receiverType(recv *ast.FieldList) string {
	if len(recv.List) == 0 {
		return "?"
	}
	e := recv.List[0].Type
	for {
		switch x := e.(type) {
		case *ast.StarExpr:
			e = x.X
		case *ast.ParenExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.IndexListExpr:
			e = x.X
		case *ast.Ident:
			return x.Name
		default:
			return "?"
		}
	}
}

// countFields counts names in a field list; an embedded field or
// interface counts one.
func countFields(fl *ast.FieldList) int {
	if fl == nil {
		return 0
	}
	n := 0
	for _, f := range fl.List {
		if len(f.Names) == 0 {
			n++
		} else {
			n += len(f.Names)
		}
	}
	return n
}

// sortGlobals orders globals by declaration.
func sortGlobals(gs []archrule.Global) {
	sort.Slice(gs, func(i, j int) bool {
		if gs[i].Decl.File != gs[j].Decl.File {
			return gs[i].Decl.File < gs[j].Decl.File
		}
		if gs[i].Decl.Line != gs[j].Decl.Line {
			return gs[i].Decl.Line < gs[j].Decl.Line
		}
		return gs[i].Name < gs[j].Name
	})
}
