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
func (s *Service) WithCoverage(c Coverage, cfg CoverageConfig) *Service {
	s.cov, s.covCfg = c, cfg
	return s
}

// coverageCheck judges the new code's coverage from the pack's coverage
// report (specs/WORKFLOW.md §6).
func (s *Service) coverageCheck(ctx context.Context, o Options, pr *packRun) CheckResult {
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
func (s *Service) stalePending(pr *packRun, name string) bool {
	return !pr.staleCleaned && slices.Contains(pr.p.Coverage.ProducedBy, name)
}

// removeStale deletes the reports a previous run left at coverage.path.
func (s *Service) removeStale(ctx context.Context, pr *packRun) {
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
		_ = f.Remove(p)
	}
}

// reportPaths resolves coverage.path: a file, a directory (every file under
// it) or a glob (every match).
func (s *Service) reportPaths(ctx context.Context, pr *packRun, f Files) ([]string, error) {
	v, err := pr.vals(pack.Check{}, "")
	if err != nil {
		return nil, err
	}
	p, err := s.expand(ctx, pr, []string{pr.p.Coverage.Path}, v)
	if err != nil {
		return nil, err
	}
	if len(p) != 1 {
		return nil, fmt.Errorf("coverage.path %q expands to %d values", pr.p.Coverage.Path, len(p))
	}
	return globFiles(f, pr.abs(p[0]))
}

func (s *Service) noReportErr(pr *packRun) error {
	what, by := "coverage report", pr.p.Coverage.ProducedBy
	if pr.p.Coverage.Format == pack.FormatGoCover || pr.p.Coverage.Format == "" {
		what = "coverprofile"
	}
	names := strings.Join(by, "/")
	return fmt.Errorf("coverage needs the %s check's %s: run %s too (--only %s,coverage)", names, what, names, strings.Join(by, ","))
}

// readReports produces (coverage.produce), reads and merges the pack's
// coverage reports.
func (s *Service) readReports(ctx context.Context, pr *packRun) (coverage.Profile, error) {
	var none coverage.Profile
	cp := pr.p.Coverage
	for _, step := range cp.Produce {
		if err := s.produce(ctx, pr, step); err != nil {
			return none, err
		}
	}
	var texts []string
	if cp.Path == "{coverage}" || s.fs() == nil {
		// The per-run file: read it through the port, as Go always has.
		t, err := s.cov.Profile(pr.cov)
		if err != nil {
			return none, err
		}
		texts = append(texts, t)
	} else {
		paths, err := s.reportPaths(ctx, pr, s.fs())
		if err != nil {
			return none, err
		}
		if len(paths) == 0 {
			return none, fmt.Errorf("no coverage report at %s", cp.Path)
		}
		for _, p := range paths {
			t, err := s.cov.Profile(p)
			if err != nil {
				return none, err
			}
			texts = append(texts, t)
		}
	}
	var profs []coverage.Profile
	for _, t := range texts {
		p, err := coverage.Parse(cp.Format, t)
		if err != nil {
			return none, err
		}
		profs = append(profs, p)
	}
	if len(profs) == 1 {
		return profs[0], nil
	}
	return coverage.MergeProfiles(profs...), nil
}

// produce runs one coverage export step; stdout_to writes its stdout.
func (s *Service) produce(ctx context.Context, pr *packRun, step pack.Produce) error {
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

func (s *Service) evaluate(ctx context.Context, o Options, pr *packRun) (string, coverage.Report, error) {
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
	prof, err := s.readReports(ctx, pr)
	if err != nil {
		return "", none, err
	}
	gocover := cp.Format == pack.FormatGoCover || cp.Format == ""
	mod := ""
	if gocover {
		if mod, err = s.cov.Module(pr.root); err != nil {
			return "", none, err
		}
	} else {
		f := s.fs()
		prof = coverage.Normalize(prof, coverage.Normalizer{PackRoot: pr.root, SourceRoots: cp.SourceRoots,
			Exists: func(rel string) bool { return f != nil && f.Exists(pr.abs(rel)) }})
	}
	copts := coverage.Options{
		Min: s.covCfg.Min, Exclude: s.covCfg.Exclude, Module: mod, Pkgs: o.Pkgs,
		SourceGlobs: pr.p.SourceGlobs, TestGlobs: pr.p.TestGlobs,
		BaseExclude:  append(append([]string{}, cp.Exclude...), pr.p.Generated.Globs...),
		PackageTests: pr.p.ID == "go",
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
