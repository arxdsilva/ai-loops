package archrule

import (
	"fmt"
	"sort"
	"strings"
)

// Spec references the finding messages cite.
const (
	hexSpec   = "specs/ARCHITECTURE.md"
	solidSpec = "specs/SOLID.md"
)

// Limits of the SOLID size rules (warnings) and of the cmd rule (errors).
const (
	MaxMethods       = 40    // methods per type
	MaxFields        = 25    // fields of a struct with at least MinMethodsFields methods
	MinMethodsFields = 5     // a struct with fewer methods is data, not a god type
	MaxIfaceMethods  = 8     // methods per interface; an embedded interface counts 1
	MaxLines         = 12000 // non-test lines per package
	MaxFiles         = 60    // non-test files per package
	MaxCmdLines      = 80    // lines of a cmd/<x>/main.go
	MaxCmdString     = 40    // runes of a string literal in a cmd/<x>/main.go
)

// randCtors are the math/rand(/v2) names that build a seeded generator
// instead of drawing from the global one; the types are listed too, so a
// conversion is not mistaken for a draw.
var randCtors = map[string]bool{
	"New": true, "NewSource": true, "NewPCG": true, "NewChaCha8": true, "NewZipf": true,
	"Rand": true, "Source": true, "Source64": true, "PCG": true, "ChaCha8": true, "Zipf": true,
}

// Evaluate runs every rule over m with the layer layout l (zero fields fall
// back to DefaultLayers) and the Go rule data (GoRules), and returns the
// findings sorted by file, line and rule. It is pure: no I/O, no clock.
func Evaluate(m Module, l Layers) Report { return EvaluateWith(m, l, GoRules()) }

// EvaluateWith is Evaluate with the language data r (a pack's arch block).
// The Go-only rules (init, package globals, cmd, compose, parent imports)
// run only when m.IsGo(); size warnings stay warnings.
func EvaluateWith(m Module, l Layers, r Rules) Report {
	c := newClassifier(l)
	goMod := m.IsGo()
	fs := []Finding{}
	for _, p := range m.Packages {
		lay := c.layerOf(p.Dir)
		fs = append(fs, importFindings(m.Path, p, lay, c, r, goMod)...)
		if goMod {
			fs = append(fs, initFindings(p)...)
			fs = append(fs, globalFindings(p)...)
			fs = append(fs, declFindings(p)...)
		}
		if lay == layerDomain {
			fs = append(fs, purityFindings(p, r)...)
			fs = append(fs, impureHitFindings(p)...)
		}
		fs = append(fs, limitFindings(p, r)...)
	}
	if goMod {
		fs = append(fs, cmdFindings(m, c)...)
	}
	sort.SliceStable(fs, func(i, j int) bool {
		a, b := fs[i], fs[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Rule < b.Rule
	})
	return Report{Module: m.Path, Findings: fs}
}

func errorAt(p Pos, rule, format string, args ...any) Finding {
	return Finding{File: p.File, Line: p.Line, Rule: rule, Severity: SeverityError, Msg: rule + ": " + fmt.Sprintf(format, args...)}
}

func initFindings(p Package) []Finding {
	var out []Finding
	for _, at := range p.Inits {
		out = append(out, errorAt(at, "init", "func init() runs hidden registration at import time; delete it and register from the composition root with explicit constructor calls (%s: init)", solidSpec))
	}
	return out
}

func globalFindings(p Package) []Finding {
	var out []Finding
	for _, g := range p.Globals {
		out = append(out, errorAt(g.Decl, "global", "package var %s is assigned at runtime (first at %s:%d); make it a field of the type that owns it or a constructor option (%s: mutable package-level state)", g.Name, g.Assigned.File, g.Assigned.Line, solidSpec))
	}
	return out
}

// declFindings warns on every package-level var or const declared after the
// first func of its file (specs/languages/go/RULES.md §55).
func declFindings(p Package) []Finding {
	var out []Finding
	for _, d := range p.Decls {
		if !d.AfterFunc {
			continue
		}
		out = append(out, Finding{File: d.Pos.File, Line: d.Pos.Line, Rule: "decls", Severity: SeverityWarn,
			Msg: fmt.Sprintf("decls: package-level %s %s is declared after the first func of its file; move it into a %s (...) cluster at the top of the file, or into the one function that uses it (specs/languages/go/RULES.md §55)", d.Kind, strings.Join(d.Names, ", "), d.Kind)})
	}
	return out
}

// purityFindings: the domain is deterministic, so it reads no clock, draws
// from no global generator and starts no goroutine.
func purityFindings(p Package, r Rules) []Finding {
	var out []Finding
	for _, c := range p.Calls {
		switch {
		case c.Pkg == "time" && c.Name == "Now" && r.has("time.Now"):
			out = append(out, errorAt(c.Pos, "clock", "the domain reads the wall clock (time.Now); take the time as a parameter or through a Clock port the application injects (%s: dependency direction)", hexSpec))
		case (c.Pkg == "math/rand" || c.Pkg == "math/rand/v2") && !randCtors[c.Name] && r.has(c.Pkg):
			out = append(out, errorAt(c.Pos, "rand", "the domain draws from the global generator (%s.%s); pass a seeded *rand.Rand or a random port in (%s: dependency direction)", c.Pkg, c.Name, hexSpec))
		}
	}
	for _, at := range p.GoStmts {
		if !r.has("go") {
			break
		}
		out = append(out, errorAt(at, "go", "the domain starts a goroutine; keep the domain synchronous and let the application or an adapter own concurrency (%s: dependency direction)", hexSpec))
	}
	return out
}

// impureHitFindings reports the impure-call text hits of a non-Go parser.
func impureHitFindings(p Package) []Finding {
	var out []Finding
	for _, h := range p.Impure {
		out = append(out, errorAt(h.Pos, "impure", "the domain calls %s (clock, randomness, environment, I/O or concurrency); take the value as a parameter or through a port the application injects (%s: dependency direction)", h.Call, hexSpec))
	}
	return out
}

func limitFindings(p Package, r Rules) []Finding {
	maxMethods, maxFields := r.limit("methods", MaxMethods), r.limit("fields", MaxFields)
	maxIface, maxLines, maxFiles := r.limit("iface", MaxIfaceMethods), r.limit("lines", MaxLines), r.limit("files", MaxFiles)
	var out []Finding
	warn := func(at Pos, rule, format string, args ...any) {
		f := errorAt(at, rule, format, args...)
		f.Severity = SeverityWarn
		out = append(out, f)
	}
	for _, t := range p.Types {
		if t.Methods > maxMethods {
			warn(t.Pos, "methods", "type %s has %d methods (limit %d); split it by responsibility into smaller types (%s: measurable limits, how to split a god type)", t.Name, t.Methods, maxMethods, solidSpec)
		}
		if t.Kind == KindStruct && t.Methods >= MinMethodsFields && t.Fields > maxFields {
			warn(t.Pos, "fields", "struct %s has %d fields and %d methods (limit %d fields); group fields into owned sub-types (%s: measurable limits)", t.Name, t.Fields, t.Methods, maxFields, solidSpec)
		}
		if t.Kind == KindInterface && t.IfaceMethods > maxIface {
			warn(t.Pos, "iface", "interface %s has %d methods (limit %d); split it into consumer-owned interfaces (%s: measurable limits)", t.Name, t.IfaceMethods, maxIface, solidSpec)
		}
	}
	dir := Pos{File: p.Dir}
	if p.Lines > maxLines {
		warn(dir, "lines", "package %s has %d non-test lines (limit %d); split it into sub-packages (%s: measurable limits)", p.Dir, p.Lines, maxLines, solidSpec)
	}
	if n := len(p.Files); n > maxFiles {
		warn(dir, "files", "package %s has %d non-test files (limit %d); split it into sub-packages (%s: measurable limits)", p.Dir, n, maxFiles, solidSpec)
	}
	return out
}

// cmdFindings: every <cmd layer>/<x> directory holds one wiring-only main.go.
func cmdFindings(m Module, c classifier) []Finding {
	mains := map[string]*MainFile{}
	for _, p := range m.Packages {
		if p.Main != nil {
			mains[p.Dir] = p.Main
		}
	}
	var out []Finding
	fix := fmt.Sprintf("move the logic into a package (internal/adapters/inbound/... or internal/compose/...) and keep main to wiring (%s: cmd is wiring)", solidSpec)
	for _, d := range m.Dirs {
		if !c.isCmdDir(d.Path) {
			continue
		}
		at := Pos{File: d.Path}
		hasMain := false
		for _, e := range d.Entries {
			if e == "main.go" {
				hasMain = true
				continue
			}
			out = append(out, errorAt(at, "cmd", "%s holds %s; only main.go belongs in a cmd directory: %s", d.Path, e, fix))
		}
		if !hasMain {
			out = append(out, errorAt(at, "cmd", "%s has no main.go: %s", d.Path, fix))
			continue
		}
		if mf := mains[d.Path]; mf != nil {
			out = append(out, mainFindings(*mf, fix)...)
		}
	}
	return out
}

func mainFindings(mf MainFile, fix string) []Finding {
	var out []Finding
	if mf.Lines > MaxCmdLines {
		out = append(out, errorAt(Pos{File: mf.File, Line: 1}, "cmd", "%s has %d lines (limit %d): %s", mf.File, mf.Lines, MaxCmdLines, fix))
	}
	for _, d := range mf.Decls {
		if d.What != "func main" {
			out = append(out, errorAt(d.Pos, "cmd", "main.go has a %s (only func main is allowed): %s", d.What, fix))
		}
	}
	for _, l := range mf.Logic {
		out = append(out, errorAt(l.Pos, "cmd", "main.go has a %s: %s", l.What, fix))
	}
	for _, s := range mf.Strings {
		if s.Runes > MaxCmdString {
			out = append(out, errorAt(s.Pos, "cmd", "main.go has a string literal of %d runes (limit %d): %s", s.Runes, MaxCmdString, fix))
		}
	}
	return out
}

// isUnder reports whether dir is p or below it.
func isUnder(dir, p string) bool {
	return p == "." || dir == p || strings.HasPrefix(dir, p+"/")
}
