package gate

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/application/langpack"
	"github.com/arxdsilva/ai-loops/internal/domain/glob"
	"github.com/arxdsilva/ai-loops/internal/domain/pack"
	"github.com/arxdsilva/ai-loops/internal/domain/workspace"
)

// Check outcomes (pack exit_codes values).
const (
	outcomePass = "pass"
	outcomeFail = "fail"
	outcomeSkip = "skip"
)

// whenFileDepth bounds the walk for a glob when_file entry.
const whenFileDepth = 6

// raceUnavailable heads the failure of a race check that cannot run on
// this host (D24): it fails, it never skips silently.
const raceUnavailable = "race policy unavailable: "

// raceHostRe recognises tool output saying the race tooling itself is
// missing on this host (Go without cgo, TSan unsupported).
var raceHostRe = regexp.MustCompile(`requires cgo|cgo: C compiler|C compiler "[^"]*" not found|ThreadSanitizer: (unsupported|not supported)|-fsanitize=thread.*(unsupported|not supported)`)

// enabled reports whether a default-off check was turned on.
func enabled(o Options, c pack.Check) bool {
	return !c.OffByDefault() || (o.Vuln && c.Kind == "vuln") || contains(o.Enable, c.Name)
}

// check runs one selected check of pr.
func (s *Service) check(ctx context.Context, o Options, pr *packRun, c pack.Check) CheckResult {
	if !enabled(o, c) {
		return CheckResult{Skipped: true}
	}
	switch c.Internal {
	case "coverage":
		return s.coverageCheck(ctx, o, pr)
	case "arch":
		return s.archCheck(ctx, pr)
	}
	race := pr.p.RaceCheck(c.Name)
	// A check that must run: required, a race check, or a default-off
	// check someone enabled (D12, D16, D24).
	must := c.Required || race || c.OffByDefault()

	argv, override := c.Argv, ""
	if len(c.WhenFile) > 0 && !s.anyExists(pr, c.WhenFile) {
		if len(c.FallbackArgv) == 0 {
			return CheckResult{Skipped: true, Output: "skipped: none of " + strings.Join(c.WhenFile, ", ") + " present"}
		}
		argv, override = c.FallbackArgv, "override: fallback_argv (no "+strings.Join(c.WhenFile, ", ")+")"
	}
	if len(argv) == 0 {
		return s.failed(c, "no argv")
	}
	if !s.available(pr, argv[0]) {
		if len(c.FallbackArgv) > 0 && !slices.Equal(argv, c.FallbackArgv) && s.available(pr, c.FallbackArgv[0]) {
			argv, override = c.FallbackArgv, "override: fallback_argv ("+argv[0]+" not installed)"
		} else {
			msg := argv[0] + " not installed"
			if c.Install != "" {
				msg += "; install: " + c.Install
			}
			if race {
				return s.failed(c, raceUnavailable+msg)
			}
			if must {
				return s.failed(c, msg)
			}
			return CheckResult{Skipped: true, Output: msg}
		}
	}
	env := mergeEnv(pr.env, sortedEnv(c.Env))
	for _, k := range c.EnvRequired {
		if _, ok := envValue(env, k); ok {
			continue
		}
		if _, ok := s.lookupEnv(k); ok {
			continue
		}
		msg := "missing env " + k
		switch {
		case race:
			return s.failed(c, raceUnavailable+msg)
		case must:
			return s.failed(c, msg)
		}
		return CheckResult{Skipped: true, Output: msg}
	}
	if s.stalePending(pr, c.Name) {
		s.removeStale(ctx, pr)
	}
	return s.runArgv(ctx, pr, c, argv, env, override, race)
}

func (s *Service) failed(c pack.Check, msg string) CheckResult {
	return CheckResult{Name: c.Name, Output: msg, note: firstLine(msg)}
}

// available decides by the resolved argv[0]: a path-like tool must exist
// under the pack root, any other is looked up on PATH.
func (s *Service) available(pr *packRun, tool string) bool {
	if strings.ContainsAny(tool, `/\`) {
		f := s.fs()
		return f != nil && f.Exists(pr.abs(tool))
	}
	return s.cmd.Has(tool)
}

// anyExists reports whether one of the when_file entries exists under the
// pack root: plain paths by existence, globs by a bounded walk.
func (s *Service) anyExists(pr *packRun, entries []string) bool {
	f := s.fs()
	if f == nil {
		return false
	}
	for _, e := range entries {
		if !hasMeta(e) {
			if f.Exists(pr.abs(e)) {
				return true
			}
			continue
		}
		found := false
		_ = f.Walk(pr.root, whenFileDepth, langpack.SkipDir, func(rel string) bool {
			found = glob.Match(e, rel)
			return found
		})
		if found {
			return true
		}
	}
	return false
}

func (s *Service) archCheck(ctx context.Context, pr *packRun) CheckResult {
	if s.arch == nil {
		return CheckResult{Output: "arch: not configured", note: "arch: not configured"}
	}
	start := s.clock.Now()
	text, pass, err := s.arch.Check(ctx, pr.root)
	out, ok := text, pass && err == nil
	if err != nil {
		out = err.Error() + "\n" + text
	}
	cr := CheckResult{OK: ok, Duration: s.clock.Now().Sub(start)}
	if !ok {
		cr.Output = trim(out, nil)
	}
	cr.note = firstLine(out)
	return cr
}

// runArgv runs c (× race variants × repeat) and judges the outcome.
func (s *Service) runArgv(ctx context.Context, pr *packRun, c pack.Check, argv, env []string, override string, race bool) CheckResult {
	variants := []map[string]string{nil}
	if race && len(pr.p.Race.Variants) > 0 {
		variants = pr.p.Race.Variants
	}
	reps := max(c.Repeat, 1)
	seeded := c.Repeat > 0 || slices.Contains(pack.Placeholders(argv), "seed")
	signal := signalFunc(c.SignalRegex)

	start := s.clock.Now()
	var seeds, notes []string
	var out string
	ok, skip := true, false
	for vi, variant := range variants {
		venv := mergeEnv(env, sortedEnv(variant))
		for r := 0; r < reps && ok && !skip; r++ {
			seed := ""
			if seeded {
				seed = pr.nextSeed()
				seeds = append(seeds, seed)
			}
			full, err := s.argvFor(ctx, pr, c, argv, seed)
			if err == nil && race {
				err = raceFlagsPresent(pr.p.Race, full, venv)
			}
			if err != nil {
				out, ok = err.Error(), false
				break
			}
			o, rerr := s.exec(ctx, pr.root, venv, full[0], full[1:]...)
			var status string
			out, status = judge(c, o, rerr)
			ok, skip = status == outcomePass, status == outcomeSkip
			if !ok && !skip && (len(variants) > 1 || reps > 1) {
				out = runLabel(variant, vi, r, reps, seed) + ":\n" + out
			}
		}
		if !ok || skip {
			break
		}
	}
	if skip {
		note := firstLine(out)
		if c.SkipOn != nil && c.SkipOn.Note != "" {
			note = c.SkipOn.Note
		}
		if c.Required {
			// A required check never skips silently (D12): a skip_on match
			// (typically "plugin not installed") fails with its hint.
			if c.Install != "" && !strings.Contains(note, c.Install) {
				note += "\ninstall: " + c.Install
			}
			return CheckResult{Duration: s.clock.Now().Sub(start), Output: note, note: firstLine(note)}
		}
		return CheckResult{Skipped: true, Output: note}
	}
	if !ok && race && raceHostRe.MatchString(out) && !strings.HasPrefix(out, raceUnavailable) {
		out = raceUnavailable + "the host cannot run " + c.Name + "'s race tooling\n" + out
	}
	cr := CheckResult{OK: ok, Duration: s.clock.Now().Sub(start)}
	if !ok {
		cr.Output = trim(out, signal)
	}
	cr.note = firstLine(out)
	if override != "" {
		notes = append(notes, override)
	}
	if len(seeds) > 0 {
		notes = append(notes, "seeds "+strings.Join(seeds, ","))
	} else if race {
		// The runner chose its own seed (pytest-randomly, ExUnit, RSpec…):
		// record it, so a failing order can be replayed.
		if m := runnerSeedRe.FindStringSubmatch(out); m != nil {
			notes = append(notes, "seed "+m[1])
		}
	}
	if len(variants) > 1 {
		notes = append(notes, fmt.Sprintf("race variants %d", len(variants)))
	}
	if len(notes) > 0 {
		cr.note = strings.TrimPrefix(cr.note+"; "+strings.Join(notes, "; "), "; ")
	}
	return cr
}

func (s *Service) argvFor(ctx context.Context, pr *packRun, c pack.Check, argv []string, seed string) ([]string, error) {
	v, err := pr.vals(c, seed)
	if err != nil {
		return nil, err
	}
	full, err := s.expand(ctx, pr, argv, v)
	if err != nil {
		return nil, err
	}
	if pr.scoped != "" {
		full = workspace.PackArgv(pr.scoped, pr.p, full)
	}
	if len(full) == 0 {
		return nil, errors.New("argv expands to nothing")
	}
	return full, nil
}

func runLabel(variant map[string]string, vi, r, reps int, seed string) string {
	var parts []string
	if variant != nil {
		parts = append(parts, "variant "+strings.Join(sortedEnv(variant), " "))
	}
	if reps > 1 {
		parts = append(parts, fmt.Sprintf("run %d/%d", r+1, reps))
	}
	if seed != "" {
		parts = append(parts, "seed "+seed)
	}
	if len(parts) == 0 {
		return fmt.Sprintf("variant %d", vi+1)
	}
	return strings.Join(parts, ", ")
}

// raceFlagsPresent is the flag-mode preflight: the race check's argv must
// carry the race flags, or its environment the race env.
func raceFlagsPresent(r pack.Race, argv, env []string) error {
	if r.Mode != pack.RaceFlag || len(r.Flags) == 0 {
		return nil
	}
	missing := ""
	for _, f := range r.Flags {
		if !slices.ContainsFunc(argv, func(a string) bool { return a == f || strings.HasPrefix(a, f+"=") }) {
			missing = f
			break
		}
	}
	if missing == "" {
		return nil
	}
	if len(r.Env) > 0 {
		all := true
		for k, want := range r.Env {
			if v, ok := envValue(env, k); !ok || !strings.Contains(v, want) {
				all = false
			}
		}
		if all {
			return nil
		}
	}
	return errors.New(raceUnavailable + "the race check runs without " + missing)
}

// runnerSeedRe finds the seed a test runner printed: pytest-randomly
// "Using --randomly-seed=N", ExUnit "with seed: N", RSpec/Minitest
// "Randomized with seed N", PHPUnit "Random Seed: N", Kaocha "--seed N".
var runnerSeedRe = regexp.MustCompile(`(?i)(?:randomly-seed=|with seed:? |random seed:\s*|--seed[ =])(\d+)`)

// judge reads one run's outcome from its output and error: exit_codes,
// fails_on output (after output_exclude), fail_regex, skip_on.
func judge(c pack.Check, out string, err error) (string, string) {
	status := outcomePass
	if err != nil {
		status = outcomeFail
	}
	if len(c.ExitCodes) > 0 {
		if v, ok := c.ExitCodes[strconv.Itoa(exitCode(err))]; ok {
			status = v
		}
	}
	if c.FailsOn == "output" {
		out = filterLines(out, c.OutputExclude)
		exitOK := status == outcomePass
		if exitOK && strings.TrimSpace(out) != "" {
			status = outcomeFail
		}
		if status == outcomePass {
			out = ""
		} else if exitOK && c.Note != "" {
			out = c.Note + "\n" + out
		}
	}
	if status == outcomePass && c.FailRegex != "" {
		if re, rerr := regexp.Compile(c.FailRegex); rerr == nil && re.MatchString(out) {
			status = outcomeFail
		}
	}
	if status == outcomeFail && c.SkipOn != nil && c.SkipOn.OutputRegex != "" {
		if re, rerr := regexp.Compile(c.SkipOn.OutputRegex); rerr == nil && re.MatchString(out) {
			status = outcomeSkip
		}
	}
	return out, status
}

// exitCode is the process exit code behind err (0 = success, -1 = the
// command did not run or exit normally).
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ec interface{ ExitCode() int }
	if errors.As(err, &ec) {
		return ec.ExitCode()
	}
	return -1
}

// filterLines drops blank lines and lines holding one of the excludes.
func filterLines(out string, exclude []string) string {
	var keep []string
	for _, l := range strings.Split(out, "\n") {
		if strings.TrimSpace(l) == "" || slices.ContainsFunc(exclude, func(x string) bool { return strings.Contains(l, x) }) {
			continue
		}
		keep = append(keep, l)
	}
	return strings.Join(keep, "\n")
}

func signalFunc(expr string) func(string) bool {
	if expr == "" {
		return nil
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil
	}
	return re.MatchString
}

func hasMeta(p string) bool { return strings.ContainsAny(p, "*?[") }

// globFiles returns the files matching an absolute pattern: a pattern
// without meta characters is the file itself, or every file under it when
// it is a directory; otherwise the walk starts at the longest literal
// prefix directory.
func globFiles(f Files, pattern string) ([]string, error) {
	pattern = filepath.Clean(pattern)
	slash := filepath.ToSlash(pattern)
	if !hasMeta(slash) {
		var out []string
		err := f.Walk(pattern, -1, skipVCS, func(rel string) bool {
			out = append(out, filepath.Join(pattern, filepath.FromSlash(rel)))
			return false
		})
		if err != nil {
			return nil, err
		}
		if len(out) == 0 && f.Exists(pattern) {
			out = []string{pattern}
		}
		return out, nil
	}
	segs := strings.Split(slash, "/")
	i := slices.IndexFunc(segs, hasMeta)
	base := strings.Join(segs[:i], "/")
	if base == "" {
		base = "/"
	}
	rest := strings.Join(segs[i:], "/")
	var out []string
	err := f.Walk(filepath.FromSlash(base), -1, skipVCS, func(rel string) bool {
		if rest == "**" || strings.Contains(rest, "/") && glob.Match(rest, rel) || path.Dir(rel) == "." && glob.Match(rest, rel) {
			out = append(out, filepath.Join(filepath.FromSlash(base), filepath.FromSlash(rel)))
		}
		return false
	})
	slices.Sort(out)
	return out, err
}

func skipVCS(name string) bool { return name == ".git" || name == ".hg" || name == ".svn" }
