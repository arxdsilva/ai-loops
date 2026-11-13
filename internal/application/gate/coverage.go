package gate

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/coverage"
	"github.com/arxdsilva/ai-loops/internal/domain/pack"
)

// Coverage reads what the coverage check needs. Paths are pack-relative
// slash paths; root is the pack root.
type Coverage interface {
	// Profile returns the coverage report written at path.
	Profile(path string) (string, error)
	// Module returns the module path declared in root's go.mod (gocover).
	Module(root string) (string, error)
	// Diff returns root's Go changes against base ("" = the default base:
	// merge-base with origin/main, then main, then HEAD~1, then the empty
	// tree).
	Diff(ctx context.Context, root, base string) (Diff, error)
	// HasTests reports whether the package dir has a _test.go file.
	HasTests(root, dir string) bool
	// Generated reports whether file carries a "Code generated … DO NOT
	// EDIT." header.
	Generated(root, file string) bool
}

// PackCoverage is the language-neutral side of a Coverage port; the gate
// uses it when the port implements it, and the Go-only methods otherwise.
type PackCoverage interface {
	// DiffGlobs is Diff limited to the files matching the pack's
	// source_globs (changed and untracked).
	DiffGlobs(ctx context.Context, root, base string, globs []string) (Diff, error)
	// HasTestFiles reports whether dir holds a file matching testGlobs.
	HasTestFiles(root, dir string, testGlobs []string) bool
	// Head returns the opening lines of file (for pack.IsGenerated).
	Head(root, file string) string
}

// RealPaths resolves symlinks; the gate uses it when the file or coverage
// port implements it, so reports that name the physical pack root
// (/private/tmp/x for /tmp/x) still match.
type RealPaths interface {
	RealPath(path string) (string, error)
}

// Diff is the change set the coverage check judges.
type Diff struct {
	Base      string   // the resolved base ref or commit
	Text      string   // `git diff -U0 <base>` output, paths relative to root
	Untracked []string // new files git does not track yet: fully changed
	NoGit     bool     // root is not in a git work tree: judge whole packages
}

// CoverageConfig is the project's coverage policy (.loops/config.json gate).
type CoverageConfig struct {
	Min     float64  // percent; raised to coverage.DefaultMin
	Exclude []string // globs added to the pack's coverage.exclude
	Base    string   // default diff base
}

// WithCoverage sets the coverage port and policy; it returns s.
func (s *service) WithCoverage(c Coverage, cfg CoverageConfig) *service {
	s.cov, s.covCfg = c, cfg
	return s
}

// coverageCheck judges the new code's coverage from the pack's coverage
// report (specs/WORKFLOW.md §6).
func (s *service) coverageCheck(ctx context.Context, o Options, pr *packRun) CheckResult {
	start := s.clock.Now()
	header, rep, err := s.evaluate(ctx, o, pr)
	cr := CheckResult{Name: CheckCoverage, Duration: s.clock.Now().Sub(start)}
	if err != nil {
		cr.Output = "coverage: " + err.Error()
		cr.note = firstLine(cr.Output)
		return cr
	}
	cr.OK = rep.OK
	cr.Output = strings.TrimRight(header+"\n"+rep.String(), "\n")
	cr.note = rep.Note
	return cr
}

// stalePending reports whether name is the pack's first producing check to
// run, so stale reports must go first.
func (s *service) stalePending(pr *packRun, name string) bool {
	return !pr.staleCleaned && slices.Contains(pr.p.Coverage.ProducedBy, name)
}

// removeStale deletes the reports a previous run left at coverage.path:
// only files inside the run's coverage dir, the run dir or strictly below
// the pack root (reportPaths refuses unsafe paths).
func (s *service) removeStale(ctx context.Context, pr *packRun) {
	pr.staleCleaned = true
	f := s.fs()
	if f == nil || pr.p.Coverage.Path == "" {
		return
	}
	paths, err := s.reportPaths(ctx, pr, f)
	if err != nil {
		return
	}
	for _, p := range paths {
		if pr.removable(p) {
			_ = f.Remove(p)
		}
	}
}

// reportPaths resolves coverage.path: a file, a directory (every file under
// it) or a glob (every match). It refuses a path whose built-in
// placeholders are empty, that is the pack root itself, or that is (or
// globs from) a filesystem root: such a path would read, and removeStale
// delete, the project.
func (s *service) reportPaths(ctx context.Context, pr *packRun, f Files) ([]string, error) {
	v, err := pr.vals(pack.Check{}, "")
	if err != nil {
		return nil, err
	}
	cp := pr.p.Coverage.Path
	for _, name := range pack.Placeholders([]string{cp}) {
		if vs, builtin := v[name]; builtin && (len(vs) == 0 || strings.TrimSpace(strings.Join(vs, "")) == "") {
			return nil, fmt.Errorf("coverage.path %q: {%s} is empty", cp, name)
		}
	}
	p, err := s.expand(ctx, pr, []string{cp}, v)
	if err != nil {
		return nil, err
	}
	if len(p) != 1 {
		return nil, fmt.Errorf("coverage.path %q expands to %d values", cp, len(p))
	}
	if strings.TrimSpace(p[0]) == "" {
		return nil, fmt.Errorf("coverage.path %q expands to nothing", cp)
	}
	abs := filepath.Clean(pr.abs(p[0]))
	base := abs
	if hasMeta(filepath.ToSlash(abs)) {
		base = globBase(abs)
	} else {
		for _, dir := range []string{pr.root, pr.runDir} {
			if dir != "" && abs == filepath.Clean(pr.abs(dir)) {
				return nil, fmt.Errorf("coverage.path %q is the pack root or run dir", cp)
			}
		}
	}
	if isFSRoot(base) {
		return nil, fmt.Errorf("coverage.path %q is rooted at %s", cp, base)
	}
	return globFiles(f, abs)
}

// removable reports whether a stale report may be deleted: it lies inside
// the run's coverage dir or run dir, or strictly below the pack root.
func (pr *packRun) removable(p string) bool {
	for _, dir := range []string{pr.covDir, pr.runDir, pr.root} {
		if dir != "" && !(filepath.IsAbs(dir) && isFSRoot(dir)) && strictlyUnder(p, dir) {
			return true
		}
	}
	return false
}

// strictlyUnder reports whether p lies below dir (not dir itself).
func strictlyUnder(p, dir string) bool {
	rel, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(p))
	if err != nil || rel == "." || rel == ".." || filepath.IsAbs(rel) {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// globBase is the directory a glob walks: its segments before the first
// one holding a metacharacter.
func globBase(pattern string) string {
	segs := strings.Split(filepath.ToSlash(pattern), "/")
	i := slices.IndexFunc(segs, hasMeta)
	if i < 0 {
		return filepath.Clean(pattern)
	}
	base := strings.Join(segs[:i], "/")
	if base == "" || strings.HasSuffix(base, ":") {
		base += "/"
	}
	return filepath.FromSlash(base)
}

// isFSRoot reports whether p is a filesystem root ("/", `C:\`), or "" or ".".
func isFSRoot(p string) bool {
	if p == "" || p == "." {
		return true
	}
	c := filepath.Clean(p)
	return filepath.Dir(c) == c
}

func (s *service) noReportErr(pr *packRun) error {
	what, by := "coverage report", pr.p.Coverage.ProducedBy
	if pr.p.Coverage.Format == pack.FormatGoCover || pr.p.Coverage.Format == "" {
		what = "coverprofile"
	}
	names := strings.Join(by, "/")
	return fmt.Errorf("coverage needs the %s check's %s: run %s too (--only %s,coverage)", names, what, names, strings.Join(by, ","))
}

// report is one coverage report's text and the path it was read from.
type report struct{ path, text string }

// readReports produces (coverage.produce) and reads the pack's coverage
// reports.
func (s *service) readReports(ctx context.Context, pr *packRun) ([]report, error) {
	cp := pr.p.Coverage
	for _, step := range cp.Produce {
		if err := s.produce(ctx, pr, step); err != nil {
			return nil, err
		}
	}
	if cp.Path == "{coverage}" || s.fs() == nil {
		// The per-run file: read it through the port, as Go always has.
		t, err := s.cov.Profile(pr.cov)
		if err != nil {
			return nil, err
		}
		return []report{{pr.cov, t}}, nil
	}
	paths, err := s.reportPaths(ctx, pr, s.fs())
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no coverage report at %s", cp.Path)
	}
	var out []report
	for _, p := range paths {
		t, err := s.cov.Profile(p)
		if err != nil {
			return nil, err
		}
		out = append(out, report{p, t})
	}
	return out, nil
}

// profile parses the pack's reports and merges them. Line-based reports are
// normalized one by one first, each against the source roots under its own
// ancestors (a multi-module build writes one report per module).
func (s *service) profile(ctx context.Context, pr *packRun) (coverage.Profile, error) {
	var none coverage.Profile
	cp := pr.p.Coverage
	reps, err := s.readReports(ctx, pr)
	if err != nil {
		return none, err
	}
	gocover := cp.Format == pack.FormatGoCover || cp.Format == ""
	f := s.fs()
	n := coverage.Normalizer{PackRoot: pr.root, RealRoot: s.realPath(pr.root), SourceRoots: cp.SourceRoots,
		Exists: func(rel string) bool { return f != nil && f.Exists(pr.abs(rel)) }}
	var profs []coverage.Profile
	for _, r := range reps {
		p, err := coverage.Parse(cp.Format, r.text)
		if err != nil {
			return none, err
		}
		if !gocover {
			n.ReportDir = pr.reportDir(r.path)
			p = coverage.Normalize(p, n)
		}
		profs = append(profs, p)
	}
	if len(profs) == 1 {
		return profs[0], nil
	}
	return coverage.MergeProfiles(profs...), nil
}

// reportDir is the pack-relative slash dir of a report ("" outside the
// pack, e.g. the run's coverage dir).
func (pr *packRun) reportDir(p string) string {
	if pr.root == "" || !strictlyUnder(p, pr.root) {
		return ""
	}
	rel, err := filepath.Rel(filepath.Clean(pr.root), filepath.Dir(filepath.Clean(p)))
	if err != nil || rel == "." {
		return ""
	}
	return filepath.ToSlash(rel)
}

// realPath resolves p's symlinks through the port ("" when unavailable or
// unchanged).
func (s *service) realPath(p string) string {
	if p == "" {
		return ""
	}
	for _, x := range []any{s.fs(), s.cov} {
		if r, ok := x.(RealPaths); ok && r != nil {
			if rp, err := r.RealPath(p); err == nil && rp != p {
				return rp
			}
			return ""
		}
	}
	return ""
}

// produce runs one coverage export step; stdout_to writes its stdout.
func (s *service) produce(ctx context.Context, pr *packRun, step pack.Produce) error {
	v, err := pr.vals(pack.Check{}, "")
	if err != nil {
		return err
	}
	argv, err := s.expand(ctx, pr, step.Argv, v)
	if err != nil {
		return err
	}
	if len(argv) == 0 {
		return errors.New("coverage.produce: empty argv")
	}
	if step.StdoutTo == "" {
		out, err := s.exec(ctx, pr.root, pr.env, argv[0], argv[1:]...)
		if err != nil {
			return fmt.Errorf("coverage.produce %s: %v\n%s", strings.Join(argv, " "), err, trim(out, nil))
		}
		return nil
	}
	out, err := s.stdout(ctx, pr.root, pr.env, argv)
	if err != nil {
		return fmt.Errorf("coverage.produce %s: %v\n%s", strings.Join(argv, " "), err, trim(out, nil))
	}
	to, err := s.expand(ctx, pr, []string{step.StdoutTo}, v)
	if err != nil {
		return err
	}
	f := s.fs()
	if f == nil {
		return errors.New("coverage.produce stdout_to: no file access")
	}
	dst := pr.abs(to[0])
	_ = f.MkdirAll(filepath.Dir(dst))
	return f.WriteFile(dst, out)
}

func (s *service) evaluate(ctx context.Context, o Options, pr *packRun) (string, coverage.Report, error) {
	var none coverage.Report
	cp := pr.p.Coverage
	switch {
	case s.cov == nil:
		return "", none, errors.New("coverage port not configured")
	case !slices.ContainsFunc(cp.ProducedBy, func(n string) bool { return pr.ran[n] }):
		return "", none, s.noReportErr(pr)
	case pr.cov == "" && strings.Contains(cp.Path, "{coverage"):
		return "", none, errors.New("no coverprofile path (Options.CoverProfile)")
	}
	prof, err := s.profile(ctx, pr)
	if err != nil {
		return "", none, err
	}
	mod := ""
	if cp.Format == pack.FormatGoCover || cp.Format == "" {
		if mod, err = s.cov.Module(pr.root); err != nil {
			return "", none, err
		}
	}
	copts := coverage.Options{
		Min: s.covCfg.Min, Exclude: s.covCfg.Exclude, Module: mod, Pkgs: o.Pkgs,
		SourceGlobs: pr.p.SourceGlobs, TestGlobs: pr.p.TestGlobs,
		BaseExclude:    append(append([]string{}, cp.Exclude...), pr.p.Generated.Globs...),
		PackageTests:   pr.p.ID == "go",
		FlagUnreported: pr.p.ID != "go",
	}
	base := o.Base
	if base == "" {
		base = s.covCfg.Base
	}
	pc, generic := s.cov.(PackCoverage)
	var d Diff
	if generic {
		d, err = pc.DiffGlobs(ctx, pr.root, base, pr.p.SourceGlobs)
	} else {
		d, err = s.cov.Diff(ctx, pr.root, base)
	}
	if err != nil {
		return "", none, err
	}
	if d.NoGit {
		copts.Whole = true
		return "no git base: whole-package coverage of the selected packages",
			coverage.Evaluate(prof, coverage.WholeChanges(prof, mod), nil, copts), nil
	}
	ch, err := coverage.ParseDiff(d.Text)
	if err != nil {
		return "", none, err
	}
	for _, f := range d.Untracked {
		ch[f] = coverage.FileChange{Ranges: coverage.WholeFile()}
	}
	cands := coverage.Candidates(ch, copts)
	for _, f := range cands {
		gen := false
		if generic {
			gen = pr.p.IsGenerated(f, pc.Head(pr.root, f))
		} else {
			gen = s.cov.Generated(pr.root, f)
		}
		if gen {
			fc := ch[f]
			fc.Generated = true
			ch[f] = fc
		}
	}
	var has map[string]bool
	if copts.PackageTests {
		has = map[string]bool{}
		for _, dir := range coverage.PkgDirs(cands) {
			if generic {
				has[dir] = pc.HasTestFiles(pr.root, dir, pr.p.TestGlobs)
			} else {
				has[dir] = s.cov.HasTests(pr.root, dir)
			}
		}
	}
	return "base " + d.Base, coverage.Evaluate(prof, ch, has, copts), nil
}
