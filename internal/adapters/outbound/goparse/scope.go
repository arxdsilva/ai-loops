package goparse

import (
	"go/ast"
	"go/token"
)

// scopeWalker walks one file's function bodies in source order, tracking
// the block scopes Go's syntax opens, and records the first write to each
// package-level var that no enclosing local declaration shadows.
type scopeWalker struct {
	decl      map[string]token.Pos // the package's globals
	hit       map[string]token.Pos // global -> first write
	qualifier string               // external test file: local name of the package import
	scopes    []map[string]bool
}

func (w *scopeWalker) push() { w.scopes = append(w.scopes, map[string]bool{}) }
func (w *scopeWalker) pop()  { w.scopes = w.scopes[:len(w.scopes)-1] }

func (w *scopeWalker) declare(name string) {
	if name != "" && name != "_" {
		w.scopes[len(w.scopes)-1][name] = true
	}
}

func (w *scopeWalker) declareIdent(e ast.Expr) {
	if id, ok := e.(*ast.Ident); ok {
		w.declare(id.Name)
	}
}

func (w *scopeWalker) declareFields(fl *ast.FieldList) {
	if fl == nil {
		return
	}
	for _, f := range fl.List {
		for _, n := range f.Names {
			w.declare(n.Name)
		}
	}
}

func (w *scopeWalker) local(name string) bool {
	for i := len(w.scopes) - 1; i >= 0; i-- {
		if w.scopes[i][name] {
			return true
		}
	}
	return false
}

// target records a write to the assignment target e.
func (w *scopeWalker) target(e ast.Expr, at token.Pos) {
	name := ""
	if w.qualifier == "" {
		name = rootIdent(e)
	} else if q, sel := qualifiedRoot(e); q == w.qualifier && !w.local(q) {
		name = sel
	}
	if _, global := w.decl[name]; !global || (w.qualifier == "" && w.local(name)) {
		return
	}
	if _, seen := w.hit[name]; !seen {
		w.hit[name] = at
	}
}

// qualifiedRoot is pkg and V in pkg.V, pkg.V.f, pkg.V[i], *pkg.V.
func qualifiedRoot(e ast.Expr) (string, string) {
	for {
		switch x := e.(type) {
		case *ast.SelectorExpr:
			if id, ok := x.X.(*ast.Ident); ok {
				return id.Name, x.Sel.Name
			}
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.StarExpr:
			e = x.X
		case *ast.ParenExpr:
			e = x.X
		default:
			return "", ""
		}
	}
}

func (w *scopeWalker) funcDecl(fd *ast.FuncDecl) {
	w.push()
	w.declareFields(fd.Recv)
	w.declareFields(fd.Type.Params)
	w.declareFields(fd.Type.Results)
	w.stmt(fd.Body)
	w.pop()
}

// expr walks an expression for the function literals inside it.
func (w *scopeWalker) expr(e ast.Expr) {
	if e == nil {
		return
	}
	ast.Inspect(e, func(n ast.Node) bool {
		if fl, ok := n.(*ast.FuncLit); ok {
			w.push()
			w.declareFields(fl.Type.Params)
			w.declareFields(fl.Type.Results)
			w.stmt(fl.Body)
			w.pop()
			return false
		}
		return true
	})
}

func (w *scopeWalker) exprs(es []ast.Expr) {
	for _, e := range es {
		w.expr(e)
	}
}

func (w *scopeWalker) stmts(ss []ast.Stmt) {
	for _, s := range ss {
		w.stmt(s)
	}
}

func (w *scopeWalker) stmt(s ast.Stmt) {
	switch s := s.(type) {
	case nil:
	case *ast.BlockStmt:
		if s == nil {
			return
		}
		w.push()
		w.stmts(s.List)
		w.pop()
	case *ast.AssignStmt:
		w.exprs(s.Rhs)
		for _, l := range s.Lhs {
			if s.Tok == token.DEFINE {
				w.declareIdent(l)
				continue
			}
			w.expr(l)
			w.target(l, s.Pos())
		}
	case *ast.IncDecStmt:
		w.expr(s.X)
		w.target(s.X, s.Pos())
	case *ast.DeclStmt:
		w.genDecl(s.Decl)
	case *ast.IfStmt:
		w.push()
		w.stmt(s.Init)
		w.expr(s.Cond)
		w.stmt(s.Body)
		w.stmt(s.Else)
		w.pop()
	case *ast.ForStmt:
		w.push()
		w.stmt(s.Init)
		w.expr(s.Cond)
		w.stmt(s.Post)
		w.stmt(s.Body)
		w.pop()
	case *ast.RangeStmt:
		w.expr(s.X)
		w.push()
		for _, e := range []ast.Expr{s.Key, s.Value} {
			if e == nil {
				continue
			}
			if s.Tok == token.DEFINE {
				w.declareIdent(e)
			} else {
				w.target(e, s.Pos())
			}
		}
		w.stmt(s.Body)
		w.pop()
	case *ast.SwitchStmt:
		w.push()
		w.stmt(s.Init)
		w.expr(s.Tag)
		w.clauses(s.Body, nil)
		w.pop()
	case *ast.TypeSwitchStmt:
		w.push()
		w.stmt(s.Init)
		var bound ast.Expr
		if a, ok := s.Assign.(*ast.AssignStmt); ok && a.Tok == token.DEFINE && len(a.Lhs) == 1 {
			w.exprs(a.Rhs)
			bound = a.Lhs[0]
		} else {
			w.stmt(s.Assign)
		}
		w.clauses(s.Body, bound)
		w.pop()
	case *ast.SelectStmt:
		w.clauses(s.Body, nil)
	case *ast.LabeledStmt:
		w.stmt(s.Stmt)
	case *ast.ExprStmt:
		w.expr(s.X)
	case *ast.ReturnStmt:
		w.exprs(s.Results)
	case *ast.GoStmt:
		w.expr(s.Call)
	case *ast.DeferStmt:
		w.expr(s.Call)
	case *ast.SendStmt:
		w.expr(s.Chan)
		w.expr(s.Value)
	}
}

// clauses walks switch and select clauses, each its own block; bound is a
// type switch's `v := x.(type)` name, declared in every clause.
func (w *scopeWalker) clauses(body *ast.BlockStmt, bound ast.Expr) {
	for _, c := range body.List {
		w.push()
		if bound != nil {
			w.declareIdent(bound)
		}
		switch c := c.(type) {
		case *ast.CaseClause:
			w.exprs(c.List)
			w.stmts(c.Body)
		case *ast.CommClause:
			w.stmt(c.Comm)
			w.stmts(c.Body)
		}
		w.pop()
	}
}

func (w *scopeWalker) genDecl(d ast.Decl) {
	gd, ok := d.(*ast.GenDecl)
	if !ok {
		return
	}
	for _, sp := range gd.Specs {
		switch sp := sp.(type) {
		case *ast.ValueSpec:
			w.exprs(sp.Values)
			for _, n := range sp.Names {
				w.declare(n.Name)
			}
		case *ast.TypeSpec:
			w.declare(sp.Name.Name)
		}
	}
}
