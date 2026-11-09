package gate

import (
	"context"
	"errors"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/coverage"
)

// Coverage reads what the coverage check needs. Paths are module-relative
// slash paths; root is the module root.
type Coverage interface {
	// Profile returns the coverprofile written at path.
	Profile(path string) (string, error)
	// Module returns the module path declared in root's go.mod.
	Module(root string) (string, error)
	// Diff returns root's changes against base ("" = the default base:
	// merge-base with origin/main, then main, then HEAD~1, then the empty
	// tree).
	Diff(ctx context.Context, root, base string) (Diff, error)
	// HasTests reports whether the package dir has a _test.go file.
	HasTests(root, dir string) bool
	// Generated reports whether file carries a "Code generated … DO NOT
	// EDIT." header.
	Generated(root, file string) bool
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
	Exclude []string // globs added to coverage.DefaultExclude
	Base    string   // default diff base
}

// WithCoverage sets the coverage port and policy; it returns s.
func (s *Service) WithCoverage(c Coverage, cfg CoverageConfig) *Service {
	s.cov, s.covCfg = c, cfg
	return s
}

// coverageCheck judges the new code's coverage from the test check's
// coverprofile (specs/WORKFLOW.md §6).
func (s *Service) coverageCheck(ctx context.Context, o Options, testRan bool) CheckResult {
	start := s.clock.Now()
	header, rep, err := s.evaluate(ctx, o, testRan)
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

func (s *Service) evaluate(ctx context.Context, o Options, testRan bool) (string, coverage.Report, error) {
	var none coverage.Report
	switch {
	case s.cov == nil:
		return "", none, errors.New("coverage port not configured")
	case !testRan:
		return "", none, errors.New("coverage needs the test check's coverprofile: run test too (--only test,coverage)")
	case o.CoverProfile == "":
		return "", none, errors.New("no coverprofile path (Options.CoverProfile)")
	}
	text, err := s.cov.Profile(o.CoverProfile)
	if err != nil {
		return "", none, err
	}
	prof, err := coverage.ParseProfile(text)
	if err != nil {
		return "", none, err
	}
	mod, err := s.cov.Module(o.Root)
	if err != nil {
		return "", none, err
	}
	copts := coverage.Options{Min: s.covCfg.Min, Exclude: s.covCfg.Exclude, Module: mod, Pkgs: o.Pkgs}
	base := o.Base
	if base == "" {
		base = s.covCfg.Base
	}
	d, err := s.cov.Diff(ctx, o.Root, base)
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
		if s.cov.Generated(o.Root, f) {
			fc := ch[f]
			fc.Generated = true
			ch[f] = fc
		}
	}
	has := map[string]bool{}
	for _, dir := range coverage.PkgDirs(cands) {
		has[dir] = s.cov.HasTests(o.Root, dir)
	}
	return "base " + d.Base, coverage.Evaluate(prof, ch, has, copts), nil
}
