package coverage

import (
	"fmt"
	"math"
	"path"
	"sort"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/glob"
)

// DefaultMin is the minimum new-code coverage (percent); lower minimums are
// raised to it.
const DefaultMin = 70.0

// DefaultExclude lists the files never judged: wiring, test data and
// generated code. Config adds to it, never replaces it.
var DefaultExclude = []string{
	"cmd/**", "internal/compose/**", "**/testdata/**",
	"**/*.pb.go", "**/*_gen.go", "**/zz_generated*",
}

// Options tunes Evaluate.
type Options struct {
	Min     float64  // percent; values below DefaultMin are raised to it
	Exclude []string // extra globs (module-relative; "**" spans directories; no "/" = base name)
	Module  string   // module path, stripped from profile file names
	Pkgs    []string // package patterns limiting the files judged; empty = all
	// FlagUnreported fails a changed source file that the report does not
	// list at all: always for a new (added or untracked) file, and for a
	// modified one when the report lists at least one file (an empty
	// report says nothing about which files ran). Line-based formats only
	// list files a test loaded; Go's coverprofile lists every package file,
	// so the gate sets it for every pack but Go.
	FlagUnreported bool
	// SourceGlobs and TestGlobs select the judged files (a language pack's
	// source_globs/test_globs); nil = Go: *.go, not *_test.go.
	SourceGlobs, TestGlobs []string
	// BaseExclude replaces DefaultExclude (a pack's coverage.exclude plus
	// its generated globs); nil = DefaultExclude. Exclude still adds to it.
	BaseExclude []string
	Whole       bool // no diff base: judge whole-package coverage only
	// PackageTests adds the "package X has no tests" finding (Go only,
	// TOOLING D7); other packs rely on the per-file finding.
	PackageTests bool
}

// FileRow is one judged file.
type FileRow struct {
	File      string
	Stmts     int // changed statements
	Covered   int // changed statements executed by tests
	Uncovered []LineRange
	Line      int // first changed statement line
}

// Finding is one failure.
type Finding struct {
	File string // "" for the total
	Line int
	Msg  string
}

// String renders "file:line: msg" (or msg alone).
func (f Finding) String() string {
	if f.File == "" {
		return f.Msg
	}
	return fmt.Sprintf("%s:%d: %s", f.File, f.Line, f.Msg)
}

// Report is the verdict.
type Report struct {
	OK       bool
	Whole    bool
	Min      float64
	Rows     []FileRow // judged files with changed statements, by path
	Stmts    int
	Covered  int
	Findings []Finding
	Note     string // one line for the gate event
}

// Percent is the covered share of the changed statements (100 when none).
func (r Report) Percent() float64 {
	if r.Stmts == 0 {
		return 100
	}
	return float64(r.Covered) * 100 / float64(r.Stmts)
}

// Evaluate judges the changed statements of ch against the profile p.
// pkgHasTests maps module-relative package dirs to whether they contain a
// _test.go file (a missing dir counts as untested; read only with
// o.PackageTests). Every failure is listed: total below the minimum, a
// changed package without tests, a changed file whose changed statements
// are all uncovered. A block (or, for line-based formats, an instrumented
// line) intersecting several changed ranges counts once.
func Evaluate(p Profile, ch Changes, pkgHasTests map[string]bool, o Options) Report {
	o.Min = math.Max(o.Min, DefaultMin)
	blocks := byRelPath(p, o.Module)
	r := Report{Whole: o.Whole, Min: o.Min}
	var unreported []Finding
	for _, f := range Candidates(ch, o) {
		if ch[f].Generated {
			continue
		}
		if fd, ok := unreportedFinding(f, ch[f], blocks, o); ok {
			unreported = append(unreported, fd)
			continue
		}
		row := judge(f, blocks[f], ch[f].Ranges)
		if row.Stmts == 0 {
			continue
		}
		r.Rows = append(r.Rows, row)
		r.Stmts += row.Stmts
		r.Covered += row.Covered
	}
	if !o.Whole {
		if o.PackageTests {
			r.Findings = append(r.Findings, packageFindings(r.Rows, pkgHasTests)...)
		}
		r.Findings = append(r.Findings, fileFindings(r.Rows)...)
		r.Findings = append(r.Findings, unreported...)
	}
	pct := floor1(r.Percent())
	switch {
	case r.Stmts == 0 && o.Whole:
		r.Note = "no git base: no statements"
	case r.Stmts == 0:
		r.Note = "no new statements"
	case o.Whole:
		r.Note = fmt.Sprintf("no git base: whole-package coverage %.1f%% (min %g%%)", pct, o.Min)
	default:
		r.Note = fmt.Sprintf("new code %.1f%% (min %g%%)", pct, o.Min)
	}
	if r.Stmts > 0 && r.Percent() < o.Min {
		what := "new code coverage"
		if o.Whole {
			what = "package coverage"
		}
		r.Findings = append(r.Findings, Finding{Msg: fmt.Sprintf("%s %.1f%% < %g%%", what, pct, o.Min)})
	}
	r.OK = len(r.Findings) == 0
	return r
}

// unreportedFinding fails a changed file the report does not list
// (Options.FlagUnreported). Line-based reports (lcov, Cobertura, JaCoCo)
// list only the files tests loaded: a changed file missing from the report
// is code no test exercises.
func unreportedFinding(f string, fc FileChange, blocks map[string][]Block, o Options) (Finding, bool) {
	if _, in := blocks[f]; in || !o.FlagUnreported || o.Whole {
		return Finding{}, false
	}
	const hint = " is not in the coverage report: no test loads it (test it, or add it to gate.coverage_exclude)"
	switch {
	case fc.IsNew():
		return Finding{File: f, Line: 1, Msg: "new file " + path.Base(f) + hint}, true
	case len(blocks) > 0:
		line := 1
		if len(fc.Ranges) > 0 && fc.Ranges[0].Start > 0 {
			line = fc.Ranges[0].Start
		}
		return Finding{File: f, Line: line, Msg: "changed file " + path.Base(f) + hint}, true
	}
	return Finding{}, false
}

// floor1 truncates to one decimal so a failing 69.96% never prints as 70.0%.
func floor1(x float64) float64 { return math.Floor(x*10) / 10 }

func packageFindings(rows []FileRow, has map[string]bool) []Finding {
	var out []Finding
	seen := map[string]bool{}
	for _, row := range rows {
		dir := path.Dir(row.File)
		if seen[dir] || has[dir] {
			continue
		}
		seen[dir] = true
		out = append(out, Finding{File: row.File, Line: row.Line, Msg: "package " + dir + " has no tests"})
	}
	return out
}

func fileFindings(rows []FileRow) []Finding {
	var out []Finding
	for _, row := range rows {
		if row.Covered == 0 {
			out = append(out, Finding{File: row.File, Line: row.Line,
				Msg: fmt.Sprintf("new code in %s is untested (0/%d statements)", path.Base(row.File), row.Stmts)})
		}
	}
	return out
}

// judge counts the statements of the blocks that intersect a changed range
// (each block once).
func judge(file string, blocks []Block, ranges []LineRange) FileRow {
	row := FileRow{File: file}
	for _, b := range blocks {
		hit := false
		for _, rg := range ranges {
			lo, hi := max(b.StartLine, rg.Start), min(b.EndLine, rg.End)
			if lo > hi {
				continue
			}
			if !hit && (row.Line == 0 || lo < row.Line) {
				row.Line = lo
			}
			hit = true
			if b.Count == 0 {
				row.Uncovered = addRange(row.Uncovered, LineRange{lo, hi})
			}
		}
		if !hit {
			continue
		}
		row.Stmts += b.NumStmt
		if b.Count > 0 {
			row.Covered += b.NumStmt
		}
	}
	return row
}

// addRange appends r to sorted ranges, merging overlapping or adjacent ones.
func addRange(rs []LineRange, r LineRange) []LineRange {
	rs = append(rs, r)
	sort.Slice(rs, func(i, j int) bool { return rs[i].Start < rs[j].Start })
	out := rs[:1]
	for _, x := range rs[1:] {
		last := &out[len(out)-1]
		if x.Start <= last.End+1 {
			last.End = max(last.End, x.End)
			continue
		}
		out = append(out, x)
	}
	return out
}

// byRelPath re-keys the profile by module-relative path; files outside the
// module are dropped. A Normalized profile is already relative.
func byRelPath(p Profile, module string) map[string][]Block {
	if p.Normalized {
		return p.Files
	}
	out := map[string][]Block{}
	for f, bs := range p.Files {
		if rel, ok := relPath(f, module); ok {
			out[rel] = bs
		}
	}
	return out
}

func relPath(f, module string) (string, bool) {
	if module == "" {
		return f, true
	}
	rel, ok := strings.CutPrefix(f, module+"/")
	return rel, ok
}

// WholeChanges marks every module file of the profile as fully changed (no
// git base: whole-package coverage).
func WholeChanges(p Profile, module string) Changes {
	out := Changes{}
	for f := range byRelPath(p, module) {
		out[f] = FileChange{Ranges: WholeFile()}
	}
	return out
}

// Candidates returns, sorted, the changed files Evaluate judges before the
// generated-file check: non-test .go files with changed lines, in scope and
// not excluded. Callers use it to ask only about these files.
func Candidates(ch Changes, o Options) []string {
	var out []string
	for f, c := range ch {
		if len(c.Ranges) == 0 || !isSource(f, o) || excluded(f, o) || !inScope(f, o.Pkgs, o.Module) {
			continue
		}
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// PkgDirs returns the sorted distinct package dirs of files.
func PkgDirs(files []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range files {
		if d := path.Dir(f); !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	sort.Strings(out)
	return out
}

// isSource reports whether f is a judged source file (not a test).
func isSource(f string, o Options) bool {
	if o.SourceGlobs == nil {
		return strings.HasSuffix(f, ".go") && !strings.HasSuffix(f, "_test.go")
	}
	return glob.Any(o.SourceGlobs, f) && !glob.Any(o.TestGlobs, f)
}

func excluded(f string, o Options) bool {
	base := DefaultExclude
	if o.BaseExclude != nil {
		base = o.BaseExclude
	}
	for _, pat := range append(append([]string{}, base...), o.Exclude...) {
		if matchGlob(pat, f) {
			return true
		}
	}
	return false
}

// inScope reports whether f is in one of the package patterns ("./x",
// "./x/...", "x/...", "<module>/x", "./..."); no patterns = everything.
func inScope(f string, pkgs []string, module string) bool {
	if len(pkgs) == 0 {
		return true
	}
	dir := path.Dir(f)
	for _, p := range pkgs {
		p = strings.TrimPrefix(strings.TrimSpace(p), "./")
		if module != "" {
			if p == module || strings.HasPrefix(p, module+"/") {
				p = strings.TrimPrefix(strings.TrimPrefix(p, module), "/")
			}
		}
		rec := false
		if p == "..." {
			return true
		}
		if s, ok := strings.CutSuffix(p, "/..."); ok {
			p, rec = s, true
		}
		if p == "" || p == "." {
			if rec {
				return true
			}
			p = "."
		}
		if dir == p || (rec && strings.HasPrefix(dir, p+"/")) {
			return true
		}
	}
	return false
}

// matchGlob matches a slash path against a glob (see package glob).
func matchGlob(pat, p string) bool { return glob.Match(pat, p) }
