// Package gate runs the quality-gate pipeline (specs/WORKFLOW.md §6):
// build, gofmt, vet, staticcheck (when installed), arch, test -race,
// govulncheck (when installed and enabled), in that order, recording each
// check as a run event.
package gate

import (
	"context"
	"strings"
	"time"

	"github.com/insurgencygames/ai-loops/internal/domain/run"
)

// Commander runs external commands in the project root.
type Commander interface {
	// Run executes name with args in dir and returns combined output.
	Run(ctx context.Context, dir, name string, args ...string) (string, error)
	// Has reports whether name is on PATH.
	Has(name string) bool
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
}

// CheckNames lists the valid check names, in pipeline order.
var CheckNames = []string{CheckBuild, CheckGofmt, CheckVet, CheckStaticcheck, CheckArch, CheckTest, CheckVuln}

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
	cmd   Commander
	arch  Arch
	rec   Recorder
	clock Clock
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
	stopped := false
	startedAt := s.clock.Now()
	for _, name := range CheckNames {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		cr := CheckResult{Name: name, Skipped: true}
		if !stopped && selected(o, name) && s.available(name, o) {
			cr = s.runCheck(ctx, o, name, pkgs)
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
		for _, req := range []string{CheckBuild, CheckVet, CheckTest} {
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
	return !contains(o.Skip, name) && !contains(o.ConfigSkip, name)
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

func (s *Service) runCheck(ctx context.Context, o Options, name string, pkgs []string) CheckResult {
	start := s.clock.Now()
	var out string
	var ok bool
	switch name {
	case CheckBuild:
		var err error
		out, err = s.cmd.Run(ctx, o.Root, "go", append([]string{"build"}, pkgs...)...)
		ok = err == nil
	case CheckGofmt:
		var err error
		out, err = s.cmd.Run(ctx, o.Root, "gofmt", append([]string{"-l"}, fmtDirs(o.Pkgs)...)...)
		out = filterFmt(out)
		ok = err == nil && strings.TrimSpace(out) == ""
		if ok {
			out = ""
		} else if err == nil {
			out = "files need gofmt:\n" + out
		}
	case CheckVet:
		var err error
		out, err = s.cmd.Run(ctx, o.Root, "go", append([]string{"vet"}, pkgs...)...)
		ok = err == nil
	case CheckStaticcheck:
		var err error
		out, err = s.cmd.Run(ctx, o.Root, "staticcheck", pkgs...)
		ok = err == nil
	case CheckArch:
		text, pass, err := s.arch.Check(ctx, o.Root)
		out, ok = text, pass && err == nil
		if err != nil {
			out = err.Error() + "\n" + text
		}
	case CheckTest:
		var err error
		out, err = s.cmd.Run(ctx, o.Root, "go", append([]string{"test", "-race", "-count=1"}, pkgs...)...)
		ok = err == nil
	case CheckVuln:
		var err error
		out, err = s.cmd.Run(ctx, o.Root, "govulncheck", pkgs...)
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
