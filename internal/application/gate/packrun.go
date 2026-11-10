package gate

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/arxdsilva/ai-loops/internal/application/langpack"
	"github.com/arxdsilva/ai-loops/internal/domain/pack"
	"github.com/arxdsilva/ai-loops/internal/domain/workspace"
)

// packRun is one pack's pipeline state: its root, the values of the
// built-in placeholders, its environment and what has run.
type packRun struct {
	p     pack.Pack
	id    string
	root  string // absolute (or as given) pack root: Options.Root joined with Detected.Root
	label string // "id@root" when several packs run

	pkgs, pkgDirs        []string
	cov, covDir          string // {coverage}, {coverage_dir}
	runDir, cache, tmp   string // {run}, {cache}, {tmp}
	scoped               string // the active run dir ("" = none): cache_env and cache_argv apply
	env                  []string
	ran                  map[string]bool
	phs                  map[string][]string // resolved pack placeholders
	staleCleaned         bool
	seedBase, seedNumber int64
}

func (s *Service) newPackRun(o Options, d langpack.Detected, i int, startedAt time.Time) *packRun {
	p := d.Pack
	id := p.ID
	if id == "" {
		id = d.ID
	}
	pr := &packRun{p: p, id: id, root: o.Root, ran: map[string]bool{}, phs: map[string][]string{},
		seedBase: startedAt.UnixNano()/1000 + int64(i)*104729}
	rootRel := strings.TrimPrefix(path.Clean(filepath.ToSlash(d.Root)), "./")
	if rootRel != "." && rootRel != "" {
		pr.root = filepath.Join(o.Root, filepath.FromSlash(rootRel))
	}
	multi := len(o.Packs) > 1
	slug := id
	if multi {
		if rootRel == "" {
			rootRel = "."
		}
		pr.label = id + "@" + rootRel
		if rootRel != "." {
			slug = id + "-" + strings.ReplaceAll(rootRel, "/", "_")
		}
	}

	pr.pkgs, pr.pkgDirs = p.Scope.Pkgs, p.Scope.PkgDirs
	if len(o.Pkgs) > 0 {
		pr.pkgs, pr.pkgDirs = o.Pkgs, fmtDirs(o.Pkgs)
	}

	switch {
	case o.CoverDir != "":
		pr.covDir = filepath.Join(o.CoverDir, slug)
	case o.CoverProfile != "":
		pr.covDir = filepath.Dir(o.CoverProfile)
	}
	switch {
	case !multi && o.CoverProfile != "":
		pr.cov = o.CoverProfile
	case pr.covDir != "":
		pr.cov = filepath.Join(pr.covDir, "coverage"+coverageExt(p.Coverage.Format))
	}

	pr.scoped = o.RunDir
	pr.runDir = o.RunDir
	if pr.runDir == "" {
		pr.runDir = o.CoverDir
	}
	if pr.runDir != "" {
		pr.cache, pr.tmp = workspace.PackCache(pr.runDir, id), workspace.PackTmp(pr.runDir, id)
	}

	// The pack's run-scoped env (cache_env, then env) with an active run;
	// without one only its env, as Go has always run with the default
	// caches outside a run.
	if o.RunDir != "" {
		pr.env = mergeEnv(o.Env, workspace.RunEnv(o.RunDir, p))
	} else {
		r := strings.NewReplacer("{run}", pr.runDir, "{cache}", pr.cache, "{tmp}", pr.tmp)
		var penv []string
		for _, kv := range sortedEnv(p.Env) {
			penv = append(penv, r.Replace(kv))
		}
		pr.env = mergeEnv(o.Env, penv)
	}

	if f := s.fs(); f != nil && o.CoverDir != "" {
		_ = f.MkdirAll(pr.covDir) // a failure surfaces when a tool writes there
	}
	return pr
}

func (pr *packRun) suffix() string {
	if pr.label == "" {
		return ""
	}
	return " (" + pr.label + ")"
}

func coverageExt(format string) string {
	switch format {
	case pack.FormatLCOV:
		return ".info"
	case pack.FormatCobertura, pack.FormatJaCoCo:
		return ".xml"
	}
	return ".out"
}

// mergeEnv overlays add onto base: a key already present keeps its
// position and takes the new value; new keys are appended.
func mergeEnv(base, add []string) []string {
	out := append([]string{}, base...)
	for _, kv := range add {
		k, _, _ := strings.Cut(kv, "=")
		i := slices.IndexFunc(out, func(x string) bool { return strings.HasPrefix(x, k+"=") })
		if i >= 0 {
			out[i] = kv
			continue
		}
		out = append(out, kv)
	}
	return out
}

func sortedEnv(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	slices.Sort(out)
	return out
}

func envValue(env []string, k string) (string, bool) {
	for i := len(env) - 1; i >= 0; i-- {
		if v, ok := strings.CutPrefix(env[i], k+"="); ok {
			return v, true
		}
	}
	return "", false
}

// abs resolves a pack-relative path against the pack root.
func (pr *packRun) abs(p string) string {
	if filepath.IsAbs(p) || pr.root == "" {
		return filepath.FromSlash(p)
	}
	return filepath.Join(pr.root, filepath.FromSlash(p))
}

// nextSeed returns a fresh {seed}: derived from the gate's start time, so
// runs differ and no extra clock reads skew check durations.
func (pr *packRun) nextSeed() string {
	pr.seedNumber++
	v := (pr.seedBase + pr.seedNumber*7919) % 1_000_000_000
	if v < 0 {
		v = -v
	}
	return strconv.FormatInt(v+1, 10)
}

// vals returns the placeholder values for one invocation of c.
func (pr *packRun) vals(c pack.Check, seed string) (map[string][]string, error) {
	v := map[string][]string{
		"pkgs": pr.pkgs, "pkg_dirs": pr.pkgDirs,
		"coverage": {pr.cov}, "coverage_dir": {pr.covDir},
		"run": {pr.runDir}, "cache": {pr.cache}, "tmp": {pr.tmp},
		"manifest_dir": {pr.root}, "coverage_args": nil,
	}
	if seed != "" {
		v["seed"] = []string{seed}
	}
	if pr.cov != "" && len(c.CoverageArgs) > 0 {
		args, err := pack.Expand(c.CoverageArgs, v)
		if err != nil {
			return nil, err
		}
		v["coverage_args"] = args
	}
	return v, nil
}

// expand fills argv's placeholders: the built-ins, then the pack-declared
// ones (resolved on first use).
func (s *Service) expand(ctx context.Context, pr *packRun, argv []string, v map[string][]string) ([]string, error) {
	if err := s.resolvePlaceholders(ctx, pr, argv, v, 0); err != nil {
		return nil, err
	}
	return pack.Expand(argv, v)
}

func (s *Service) resolvePlaceholders(ctx context.Context, pr *packRun, argv []string, v map[string][]string, depth int) error {
	for _, name := range pack.Placeholders(argv) {
		if _, ok := v[name]; ok {
			continue
		}
		if got, ok := pr.phs[name]; ok {
			v[name] = got
			continue
		}
		ph, ok := pr.p.Placeholders[name]
		if !ok {
			continue // pack.Expand reports it
		}
		if depth > 4 {
			return fmt.Errorf("placeholder {%s}: nested too deep", name)
		}
		got, err := s.resolveOne(ctx, pr, name, ph, v, depth)
		if err != nil {
			return err
		}
		pr.phs[name], v[name] = got, got
	}
	return nil
}

func (s *Service) resolveOne(ctx context.Context, pr *packRun, name string, ph pack.Placeholder, v map[string][]string, depth int) ([]string, error) {
	sub := func(argv []string) ([]string, error) {
		if err := s.resolvePlaceholders(ctx, pr, argv, v, depth+1); err != nil {
			return nil, err
		}
		return pack.Expand(argv, v)
	}
	switch {
	case len(ph.Argv) > 0:
		argv, err := sub(ph.Argv)
		if err != nil {
			return nil, err
		}
		out, err := s.stdout(ctx, pr.root, pr.env, argv)
		if err != nil {
			return nil, fmt.Errorf("placeholder {%s}: %s: %v: %s", name, strings.Join(argv, " "), err, firstLine(out))
		}
		val := strings.TrimSpace(out)
		if ph.Dirname {
			val = filepath.Dir(val)
		}
		return []string{val}, nil
	case ph.File != "":
		f := s.fs()
		if f == nil {
			return nil, fmt.Errorf("placeholder {%s}: no file access", name)
		}
		p, err := sub([]string{ph.File})
		if err != nil {
			return nil, err
		}
		text, err := f.ReadFile(pr.abs(p[0]))
		if err != nil {
			return nil, fmt.Errorf("placeholder {%s}: %v", name, err)
		}
		re, err := regexp.Compile(ph.Regex)
		if err != nil {
			return nil, fmt.Errorf("placeholder {%s}: %v", name, err)
		}
		m := re.FindStringSubmatch(text)
		if len(m) < 2 {
			return nil, fmt.Errorf("placeholder {%s}: %s does not match %q", name, p[0], ph.Regex)
		}
		return []string{m[1]}, nil
	default:
		f := s.fs()
		if f == nil {
			return nil, fmt.Errorf("placeholder {%s}: no file access", name)
		}
		pats, err := sub(ph.Glob)
		if err != nil {
			return nil, err
		}
		var out []string
		for _, g := range pats {
			m, err := globFiles(f, pr.abs(g))
			if err != nil {
				return nil, fmt.Errorf("placeholder {%s}: %v", name, err)
			}
			out = append(out, m...)
		}
		return out, nil
	}
}
