// Package gate runs the quality-gate pipeline (specs/WORKFLOW.md §6) for
// every language pack detected in the project: each pack's checks in pack
// order (for Go: build, gofmt, vet, staticcheck when installed, arch, test
// -race with a coverprofile, coverage of the new code, govulncheck when
// enabled), each recorded as a run event, then one aggregate "all" event.
// What a check runs and how its outcome is read is pack data
// (research/languages/TOOLING.md §2.4); this package has no per-language
// code.
package gate

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/arxdsilva/ai-loops/internal/application/langpack"
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

// StdoutCommander runs a command and returns its stdout alone (pack
// placeholders with argv, coverage.produce steps with stdout_to). Without
// it the gate falls back to the combined output.
type StdoutCommander interface {
	RunStdout(ctx context.Context, dir string, env []string, name string, args ...string) (string, error)
}

// Files is the file access pack features need: when_file, coverage paths
// (files, dirs and globs), stale-report cleanup, produce stdout_to,
// placeholders that read a file or glob. Paths are absolute OS paths.
type Files interface {
	Exists(path string) bool
	ReadFile(path string) (string, error)
	WriteFile(path, data string) error
	Remove(path string) error
	MkdirAll(dir string) error
	// Walk calls fn with the slash path, relative to root, of every
	// regular file under root, down to depth directory levels (<0 =
	// unlimited), never entering a directory for which skip(name) is
	// true; fn returning true stops the walk. A root that is not a
	// directory yields nothing.
	Walk(root string, depth int, skip func(name string) bool, fn func(rel string) bool) error
}

// EnvLookup reads the process environment (env_required).
type EnvLookup interface {
	LookupEnv(key string) (string, bool)
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

// ErrNoPack is returned when the project has no language pack to gate.
var ErrNoPack = errors.New("no language pack detected; pin one with languages in .loops/config.json")

// Options selects what to run.
type Options struct {
	Root string   // project root
	Pkgs []string // package patterns ({pkgs}/{pkg_dirs}); empty = each pack's scope
	Only []string // run only these checks (names below); empty = all
	Skip []string // skip these checks (ad-hoc, from the command line)
	// ConfigSkip lists checks skipped by project config. Skipping a
	// required check (or a race check) this way makes the final "all"
	// event fail.
	ConfigSkip []string
	Vuln       bool     // enable default-off checks of kind "vuln"
	Enable     []string // enable these default-off checks by name
	FailFast   bool     // stop at the first failing check
	// CoverProfile is {coverage} of a single-pack gate (the caller owns the
	// file); the coverage check reads it.
	CoverProfile string
	// CoverDir is a per-run scratch directory (the caller owns it): each
	// pack gets <CoverDir>/<pack> as {coverage_dir}, and its {coverage}
	// when several packs run.
	CoverDir string
	// RunDir is the active run's directory ("" = none): {run}, and each
	// pack's cache_env scoped under it (workspace.PackEnv).
	RunDir string
	// Base is the git ref new code is diffed against; "" = CoverageConfig.Base,
	// then the Coverage port's default.
	Base string
	// Env is added to every command's environment ("K=V"), e.g. the active
	// run's GOCACHE and GOTMPDIR.
	Env []string
	// Packs are the language packs detected for the project
	// (langpack.Service.Detect); each runs its pipeline in its root.
	Packs []langpack.Detected
}

// CheckNames lists the Go pack's check names, in pipeline order (compat:
// the checks themselves come from the packs).
var CheckNames = []string{CheckBuild, CheckGofmt, CheckVet, CheckStaticcheck, CheckArch, CheckTest, CheckCoverage, CheckVuln}

// RequiredChecks are the Go pack's required checks (compat): a full gate
// that config-skips one fails. CheckCoverage is not even skippable from the
// command line.
var RequiredChecks = []string{CheckBuild, CheckVet, CheckTest, CheckCoverage}

// StaleStaticcheckNote is shown when staticcheck cannot analyse this toolchain's code.
const StaleStaticcheckNote = "staticcheck too old for this Go toolchain — upgrade: go install honnef.co/go/tools/cmd/staticcheck@latest"

// Check names of the Go pack, in pipeline order.
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
	Pack     string // "id@root" when several packs ran; "" otherwise
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
	files  Files
	env    EnvLookup
}

// New builds a Service.
func New(cmd Commander, arch Arch, rec Recorder, clock Clock) *Service {
	return &Service{cmd: cmd, arch: arch, rec: rec, clock: clock}
}

// WithFiles sets the file port; without it the gate uses the Coverage
// port when that implements Files. It returns s.
func (s *Service) WithFiles(f Files) *Service { s.files = f; return s }

// WithEnvLookup sets the process-environment port; without it the gate
// uses the Coverage port or the Commander when one implements EnvLookup.
// It returns s.
func (s *Service) WithEnvLookup(e EnvLookup) *Service { s.env = e; return s }

func (s *Service) fs() Files {
	if s.files != nil {
		return s.files
	}
	if f, ok := s.cov.(Files); ok {
		return f
	}
	return nil
}

func (s *Service) lookupEnv(k string) (string, bool) {
	for _, x := range []any{s.env, s.cov, s.cmd} {
		if e, ok := x.(EnvLookup); ok && e != nil {
			return e.LookupEnv(k)
		}
	}
	return "", false
}

// Run executes every pack's pipeline and records one KindGate event per
// check that ran plus a final run.GateAll event (OK only when every
// non-skipped check passed and Pkgs/Only/Skip were empty, i.e. a full gate).
func (s *Service) Run(ctx context.Context, o Options) (Result, error) {
	if len(o.Packs) == 0 {
		return Result{}, ErrNoPack
	}
	res := Result{OK: true}
	stopped := false
	startedAt := s.clock.Now()
	var skippedReq string
	for i, d := range o.Packs {
		pr := s.newPackRun(o, d, i, startedAt)
		for _, c := range pr.p.Checks {
			if err := ctx.Err(); err != nil {
				return res, err
			}
			if contains(o.ConfigSkip, c.Name) && (c.Required || pr.p.RaceCheck(c.Name)) {
				skippedReq = c.Name + pr.suffix()
			}
			cr := CheckResult{Name: c.Name, Pack: pr.label, Skipped: true}
			if !stopped && selected(o, c.Name, c.IsSkippable()) {
				cr = s.check(ctx, o, pr, c)
				cr.Name, cr.Pack = c.Name, pr.label
				if !cr.Skipped {
					pr.ran[c.Name] = true
					s.record(ctx, cr)
					if !cr.OK {
						res.OK = false
						if o.FailFast {
							stopped = true
						}
					}
				}
			}
			res.Checks = append(res.Checks, cr)
		}
	}
	if len(o.Pkgs) == 0 && len(o.Only) == 0 && len(o.Skip) == 0 {
		ev := run.Event{Kind: run.KindGate, Name: run.GateAll, OK: run.Bool(res.OK), Time: startedAt.UTC()}
		if skippedReq != "" {
			// A full gate that skipped a required check is not green:
			// fail the run too, so the exit code and the Stop gate agree.
			ev.OK = run.Bool(false)
			ev.Note = "required check skipped: " + skippedReq
			res.OK = false
		}
		_ = s.rec.Record(ctx, ev) // no active run is fine; gating must not depend on recording
	}
	return res, nil
}

func selected(o Options, name string, skippable bool) bool {
	if len(o.Only) > 0 && !contains(o.Only, name) {
		return false
	}
	if contains(o.ConfigSkip, name) {
		return false
	}
	return !skippable || !contains(o.Skip, name)
}

func contains(l []string, n string) bool {
	for _, x := range l {
		if strings.TrimSpace(x) == n {
			return true
		}
	}
	return false
}

// exec runs one external command in dir with env when the Commander
// supports it.
func (s *Service) exec(ctx context.Context, dir string, env []string, name string, args ...string) (string, error) {
	if ec, ok := s.cmd.(EnvCommander); ok && len(env) > 0 {
		return ec.RunEnv(ctx, dir, env, name, args...)
	}
	return s.cmd.Run(ctx, dir, name, args...)
}

// stdout runs a command for its stdout (combined output without a
// StdoutCommander).
func (s *Service) stdout(ctx context.Context, dir string, env []string, argv []string) (string, error) {
	if sc, ok := s.cmd.(StdoutCommander); ok {
		return sc.RunStdout(ctx, dir, env, argv[0], argv[1:]...)
	}
	return s.exec(ctx, dir, env, argv[0], argv[1:]...)
}

// fmtDirs maps package patterns to directories for {pkg_dirs}; full runs
// use the pack's scope.
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

const maxOutputLines = 60

// trim keeps the first maxOutputLines lines; with signal set, the lines it
// matches (failures) come first.
func trim(out string, signal func(string) bool) string {
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) <= maxOutputLines {
		return strings.Join(lines, "\n")
	}
	if signal != nil {
		var hot, rest []string
		for _, l := range lines {
			if signal(l) {
				hot = append(hot, l)
			} else {
				rest = append(rest, l)
			}
		}
		lines = append(hot, rest...)
	}
	return strings.Join(lines[:maxOutputLines], "\n")
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
		DurationMS: cr.Duration.Milliseconds(), Note: cr.note, Pack: cr.Pack,
	})
}
