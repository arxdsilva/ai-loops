package goparse

import (
	"go/ast"
	"go/token"
	"strconv"

	"github.com/insurgencygames/ai-loops/internal/domain/archrule"
)

// assignedGlobals lists the package-level vars declared in files that some
// function body (in files or tests) assigns to, increments, or indexes into
// for assignment. Exempt: `_` (so `var _ I = (*T)(nil)`), sync/atomic
// values, and a `var (...)` group that holds one (the mutex or Once
// guarding a cache is declared beside it).
//
// Shadowing is scope-aware, syntax only: parameters, results and the
// receiver shadow the whole body; a :=, var, type, range or clause binding
// shadows from its declaration to the end of its block. Files of an
// external test package (package x_test) never see the package's globals
// unqualified, so only their writes through the package's import (x.V = 1)
// count. Ported from rimworld-go internal/arch/solid_test.go, with
// positions, block scoping and the package-clause check added.
func assignedGlobals(fset *token.FileSet, importPath string, files, tests []*ast.File) []archrule.Global {
	if len(files) == 0 {
		return nil
	}
	pkgName := files[0].Name.Name
	decl := map[string]token.Pos{}
	for _, af := range files {
		for _, d := range af.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue
			}
			guarded := false
			for _, s := range gd.Specs {
				if syncValue(s.(*ast.ValueSpec)) {
					guarded = true
				}
			}
			if guarded {
				continue
			}
			for _, s := range gd.Specs {
				for _, n := range s.(*ast.ValueSpec).Names {
					if n.Name != "_" {
						decl[n.Name] = n.Pos()
					}
				}
			}
		}
	}
	if len(decl) == 0 {
		return nil
	}
	hit := map[string]token.Pos{}
	for _, af := range append(append([]*ast.File(nil), files...), tests...) {
		w := &scopeWalker{decl: decl, hit: hit}
		if af.Name.Name != pkgName {
			w.qualifier = importName(af, importPath)
			if w.qualifier == "" {
				continue // an external test that does not import the package
			}
		}
		for _, d := range af.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
				w.funcDecl(fd)
			}
		}
	}
	var out []archrule.Global
	for name, at := range hit {
		p, a := fset.Position(decl[name]), fset.Position(at)
		out = append(out, archrule.Global{
			Name:     name,
			Decl:     archrule.Pos{File: p.Filename, Line: p.Line},
			Assigned: archrule.Pos{File: a.Filename, Line: a.Line},
		})
	}
	sortGlobals(out)
	return out
}

// importName is the local name af gives the import of path, "" if none.
func importName(af *ast.File, path string) string {
	for _, imp := range af.Imports {
		ip, _ := strconv.Unquote(imp.Path.Value)
		if ip != path {
			continue
		}
		if imp.Name != nil {
			if imp.Name.Name == "_" || imp.Name.Name == "." {
				return ""
			}
			return imp.Name.Name
		}
		return defaultName(ip)
	}
	return ""
}

// syncValue reports a var whose type or initializer comes from sync or
// atomic (a mutex, a Once*, a Pool, an atomic counter), or an anonymous
// struct holding one: state those packages make safe to share.
func syncValue(vs *ast.ValueSpec) bool {
	if fromSync(vs.Type) {
		return true
	}
	for _, v := range vs.Values {
		if fromSync(v) {
			return true
		}
	}
	return false
}

func fromSync(e ast.Expr) bool {
	for e != nil {
		switch x := e.(type) {
		case *ast.StarExpr:
			e = x.X
		case *ast.CallExpr:
			e = x.Fun
		case *ast.CompositeLit:
			e = x.Type
		case *ast.IndexExpr:
			e = x.X
		case *ast.IndexListExpr:
			e = x.X
		case *ast.StructType:
			for _, f := range x.Fields.List {
				if fromSync(f.Type) {
					return true
				}
			}
			return false
		case *ast.SelectorExpr:
			id, ok := x.X.(*ast.Ident)
			return ok && (id.Name == "sync" || id.Name == "atomic")
		default:
			return false
		}
	}
	return false
}

// rootIdent is the identifier an assignment target starts from: x in x,
// x.f, x[i], *x, (x).
func rootIdent(e ast.Expr) string {
	for {
		switch x := e.(type) {
		case *ast.Ident:
			return x.Name
		case *ast.SelectorExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.StarExpr:
			e = x.X
		case *ast.ParenExpr:
			e = x.X
		default:
			return ""
		}
	}
}
