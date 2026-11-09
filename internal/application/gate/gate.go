// Package gate runs the quality-gate pipeline (specs/WORKFLOW.md §6):
// build, gofmt, vet, staticcheck (when installed), arch, test -race with a
// coverprofile, coverage of the new code, govulncheck (when installed and
// enabled), in that order, recording each check as a run event.
package gate

import (
	"context"
	"strings"
	"time"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

// Commander runs external commands in the project root.
type Commander interface {
	// Run executes name with args in dir and returns combined output.
	Run(ctx context.Context, dir, name string, args ...string) (string, error)
	// Has reports whether name is on PATH.
	Has(name string) bool
}

// EnvCommander is a Commander that can add environment entries ("K=V") to
// the command's process environment. Options.Env needs it; a plain
// Commander runs without the extra environment.
type EnvCommander interface {
	RunEnv(ctx context.Context, dir string, env []string, name string, args ...string) (string, error)
}

// Arch runs the architecture check and returns (failing findings rendered as
// text, ok).
type Arch interface {
	Check(ctx context.Context, root string) (string, bool, error)
}

// Recorder records gate events into the current run (may return
// tracker.ErrNoActiveRun, which the gate ignores).
type Recorder interface {
	Record(ctx context.Context, e run.Event) error
}

// Clock is the injected wall clock (durations).
type Clock interface{ Now() time.Time }

// Options selects what to run.
type Options struct {
	Root string   // project root
	Pkgs []string // package patterns; empty = "./..."
	Only []string // run only these checks (names below); empty = all
	Skip []string // skip these checks (ad-hoc, from the command line)
	// ConfigSkip lists checks skipped by project config. Only staticcheck,
	// govulncheck, gofmt and arch may be skipped this way; skipping build,
	// vet or test makes the final "all" event fail.
	ConfigSkip []string
	Vuln       bool // include govulncheck
	FailFast   bool // stop at the first failing check
	// CoverProfile is where the test check writes its coverprofile (the
	// caller owns the file); the coverage check reads it.
	CoverProfile string
	// Base is the git ref new code is diffed against; "" = CoverageConfig.Base,
	// then the Coverage port's default.
	Base string
	// Env is added to every command's environment ("K=V"), e.g. the active
	// run's GOCACHE and GOTMPDIR.
	Env []string
}

// CheckNames lists the valid check names, in pipeline order.
var CheckNames = []string{CheckBuild, CheckGofmt, CheckVet, CheckStaticcheck, CheckArch, CheckTest, CheckCoverage, CheckVuln}

// RequiredChecks may not be skipped: a full gate that config-skips one
// fails. CheckCoverage is not even skippable from the command line.
var RequiredChecks = []string{CheckBuild, CheckVet, CheckTest, CheckCoverage}

// StaleStaticcheckNote is shown when staticcheck cannot analyse this toolchain's code.
const StaleStaticcheckNote = "staticcheck too old for this Go toolchain \u2014 upgrade: go install honnef.co/go/tools/cmd/staticcheck@latest"

// Check names, in pipeline order.
const (
	CheckBuild       = "build"
	CheckGofmt       = "gofmt"
	CheckVet         = "vet"
	CheckStaticcheck = "staticcheck"
	CheckArch        = "arch"
	CheckTest        = "test"
	CheckCoverage    = "coverage"
	CheckVuln        = "govulncheck"
)

// CheckResult is one check's outcome.
type CheckResult struct {
	Name     string
	OK       bool
	Skipped  bool   // tool not installed or not selected
	Output   string // trimmed to the useful part (failures), at most ~60 lines
	Duration time.Duration

	note string // first output line, for the gate event
}

// Result is the whole pipeline's outcome.
type Result struct {
	Checks []CheckResult
	OK     bool
}

// Service runs gates.
type Service struct {
	cmd    Commander
	arch   Arch
	rec    Recorder
	clock  Clock
	cov    Coverage
	covCfg CoverageConfig
}

// New builds a Service.
func New(cmd Commander, arch Arch, rec Recorder, clock Clock) *Service {
	return &Service{cmd: cmd, arch: arch, rec: rec, clock: clock}
}

// Run executes the pipeline and records one KindGate event per check plus a
// final run.GateAll event (OK only when every non-skipped check passed and
// Pkgs/Only were empty, i.e. a full gate).
func (s *Service) Run(ctx context.Context, o Options) (Result, error) {
	pkgs := o.Pkgs
	if len(pkgs) == 0 {
		pkgs = []string{"./..."}
	}
	res := Result{OK: true}
	stopped, testRan := false, false
	startedAt := s.clock.Now()
	for _, name := range CheckNames {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		cr := CheckResult{Name: name, Skipped: true}
		if !stopped && selected(o, name) && s.available(name, o) {
			if name == CheckCoverage {
				cr = s.coverageCheck(ctx, o, testRan)
			} else {
				cr = s.runCheck(ctx, o, name, pkgs)
			}
			testRan = testRan || (name == CheckTest && !cr.Skipped)
			if !cr.Skipped {
				s.record(ctx, cr)
			}
			if !cr.Skipped && !cr.OK {
				res.OK = false
				if o.FailFast {
					stopped = true
				}
			}
		}
		res.Checks = append(res.Checks, cr)
	}
	if len(o.Pkgs) == 0 && len(o.Only) == 0 && len(o.Skip) == 0 {
		ev := run.Event{Kind: run.KindGate, Name: run.GateAll, OK: run.Bool(res.OK), Time: startedAt.UTC()}
		for _, req := range RequiredChecks {
			if contains(o.ConfigSkip, req) {
				// A full gate that skipped a required check is not green:
				// fail the run too, so the exit code and the Stop gate agree.
				ev.OK = run.Bool(false)
				ev.Note = "required check skipped: " + req
				res.OK = false
			}
		}
		_ = s.rec.Record(ctx, ev) // no active run is fine; gating must not depend on recording
	}
	return res, nil
}

func selected(o Options, name string) bool {
	if len(o.Only) > 0 && !contains(o.Only, name) {
		return false
	}
	if contains(o.ConfigSkip, name) {
		return false
	}
	return name == CheckCoverage || !contains(o.Skip, name)
}

func contains(l []string, n string) bool {
	for _, x := range l {
		if strings.TrimSpace(x) == n {
			return true
		}
	}
	return false
}

func (s *Service) available(name string, o Options) bool {
	switch name {
	case CheckStaticcheck:
		return s.cmd.Has("staticcheck")
	case CheckVuln:
		return o.Vuln && s.cmd.Has("govulncheck")
	}
	return true
}

// exec runs one external command with o.Env when the Commander supports it.
func (s *Service) exec(ctx context.Context, o Options, name string, args ...string) (string, error) {
	if ec, ok := s.cmd.(EnvCommander); ok && len(o.Env) > 0 {
		return ec.RunEnv(ctx, o.Root, o.Env, name, args...)
	}
	return s.cmd.Run(ctx, o.Root, name, args...)
}

func (s *Service) runCheck(ctx context.Context, o Options, name string, pkgs []string) CheckResult {
	start := s.clock.Now()
	var out string
	var ok bool
	switch name {
	case CheckBuild:
		var err error
		out, err = s.exec(ctx, o, "go", append([]string{"build"}, pkgs...)...)
		ok = err == nil
	case CheckGofmt:
		var err error
		out, err = s.exec(ctx, o, "gofmt", append([]string{"-l"}, fmtDirs(o.Pkgs)...)...)
		out = filterFmt(out)
		ok = err == nil && strings.TrimSpace(out) == ""
		if ok {
			out = ""
		} else if err == nil {
			out = "files need gofmt:\n" + out
		}
	case CheckVet:
		var err error
		out, err = s.exec(ctx, o, "go", append([]string{"vet"}, pkgs...)...)
		ok = err == nil
	case CheckStaticcheck:
		var err error
		out, err = s.exec(ctx, o, "staticcheck", pkgs...)
		ok = err == nil
	case CheckArch:
		text, pass, err := s.arch.Check(ctx, o.Root)
		out, ok = text, pass && err == nil
		if err != nil {
			out = err.Error() + "\n" + text
		}
	case CheckTest:
		var err error
		args := []string{"test", "-race", "-count=1"}
		if o.CoverProfile != "" {
			args = append(args, "-covermode=atomic", "-coverprofile="+o.CoverProfile)
		}
		out, err = s.exec(ctx, o, "go", append(args, pkgs...)...)
		ok = err == nil
	case CheckVuln:
		var err error
		out, err = s.exec(ctx, o, "govulncheck", pkgs...)
		ok = err == nil
	}
	cr := CheckResult{Name: name, OK: ok, Duration: s.clock.Now().Sub(start)}
	if name == CheckStaticcheck && !ok && (strings.Contains(out, "unsupported version") || strings.Contains(out, "internal error in importing")) {
		return CheckResult{Name: name, Skipped: true, Output: StaleStaticcheckNote}
	}
	if !ok {
		cr.Output = trim(out, name == CheckTest)
	}
	cr.note = firstLine(out)
	return cr
}

// fmtDirs maps package patterns to directories for gofmt; full runs use ".".
func fmtDirs(pkgs []string) []string {
	if len(pkgs) == 0 {
		return []string{"."}
	}
	var dirs []string
	for _, p := range pkgs {
		p = strings.TrimSuffix(p, "/...")
		p = strings.TrimPrefix(p, "./")
		if p == "..." || p == "" {
			p = "."
		}
		dirs = append(dirs, p)
	}
	return dirs
}

func filterFmt(out string) string {
	var keep []string
	for _, l := range strings.Split(out, "\n") {
		if strings.TrimSpace(l) == "" || strings.Contains(l, "vendor/") || strings.Contains(l, "testdata/") {
			continue
		}
		keep = append(keep, l)
	}
	return strings.Join(keep, "\n")
}

const maxOutputLines = 60

func trim(out string, test bool) string {
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) <= maxOutputLines {
		return strings.Join(lines, "\n")
	}
	if test {
		var hot, rest []string
		for _, l := range lines {
			if isTestSignal(l) {
				hot = append(hot, l)
			} else {
				rest = append(rest, l)
			}
		}
		lines = append(hot, rest...)
	}
	return strings.Join(lines[:maxOutputLines], "\n")
}

func isTestSignal(l string) bool {
	return strings.HasPrefix(l, "FAIL") || strings.Contains(l, "--- FAIL") || strings.Contains(l, "panic") ||
		strings.Contains(l, "DATA RACE") || strings.Contains(l, "_test.go:")
}

func firstLine(out string) string {
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			if len(l) > 200 {
				l = l[:200]
			}
			return l
		}
	}
	return ""
}

func (s *Service) record(ctx context.Context, cr CheckResult) {
	_ = s.rec.Record(ctx, run.Event{
		Kind: run.KindGate, Name: cr.Name, OK: run.Bool(cr.OK),
		DurationMS: cr.Duration.Milliseconds(), Note: cr.note,
	})
}
