package gate

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/application/langpack"
	"github.com/arxdsilva/ai-loops/internal/domain/pack"
	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

// call is one command the scripted commander ran.
type call struct {
	dir  string
	env  []string
	argv string
}

type exitErr int

func (e exitErr) Error() string { return fmt.Sprintf("exit status %d", int(e)) }
func (e exitErr) ExitCode() int { return int(e) }

// scriptCmd answers each command through fn and records it.
type scriptCmd struct {
	calls  []call
	has    map[string]bool
	fn     func(argv string, env []string) (string, error)
	stdout map[string]string
}

func (c *scriptCmd) Run(ctx context.Context, dir, name string, args ...string) (string, error) {
	return c.RunEnv(ctx, dir, nil, name, args...)
}

func (c *scriptCmd) RunEnv(_ context.Context, dir string, env []string, name string, args ...string) (string, error) {
	argv := strings.TrimSpace(name + " " + strings.Join(args, " "))
	c.calls = append(c.calls, call{dir, env, argv})
	if c.fn != nil {
		return c.fn(argv, env)
	}
	return "", nil
}

func (c *scriptCmd) RunStdout(ctx context.Context, dir string, env []string, name string, args ...string) (string, error) {
	out, err := c.RunEnv(ctx, dir, env, name, args...)
	if v, ok := c.stdout[name]; ok {
		return v, err
	}
	return out, err
}

func (c *scriptCmd) Has(n string) bool {
	if v, ok := c.has[n]; ok {
		return v
	}
	return true
}

func (c *scriptCmd) argvs() []string {
	var out []string
	for _, x := range c.calls {
		out = append(out, x.argv)
	}
	return out
}

// memFS is an in-memory Files.
type memFS struct {
	files   map[string]string
	removed []string
	dirs    []string
	env     map[string]string
}

func newMemFS(files map[string]string) *memFS {
	if files == nil {
		files = map[string]string{}
	}
	return &memFS{files: files}
}

func (m *memFS) Exists(p string) bool {
	if _, ok := m.files[p]; ok {
		return true
	}
	for f := range m.files {
		if strings.HasPrefix(f, p+"/") {
			return true
		}
	}
	return false
}

func (m *memFS) ReadFile(p string) (string, error) {
	s, ok := m.files[p]
	if !ok {
		return "", errors.New(p + ": no such file")
	}
	return s, nil
}

func (m *memFS) WriteFile(p, data string) error { m.files[p] = data; return nil }
func (m *memFS) Remove(p string) error {
	m.removed = append(m.removed, p)
	delete(m.files, p)
	return nil
}
func (m *memFS) MkdirAll(d string) error { m.dirs = append(m.dirs, d); return nil }

func (m *memFS) Walk(root string, depth int, skip func(string) bool, fn func(string) bool) error {
	var rels []string
	for f := range m.files {
		rel, ok := strings.CutPrefix(f, root+"/")
		if !ok {
			continue
		}
		segs := strings.Split(rel, "/")
		if depth >= 0 && len(segs)-1 > depth {
			continue
		}
		if slices.ContainsFunc(segs[:len(segs)-1], func(s string) bool { return skip != nil && skip(s) }) {
			continue
		}
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	for _, r := range rels {
		if fn(r) {
			return nil
		}
	}
	return nil
}

func (m *memFS) LookupEnv(k string) (string, bool) { v, ok := m.env[k]; return v, ok }

func detected(p pack.Pack, root string) langpack.Detected {
	return langpack.Detected{Pack: p, ID: p.ID, Name: p.Name, Root: root}
}

func byName(res Result, name string) CheckResult {
	for _, c := range res.Checks {
		if c.Name == name {
			return c
		}
	}
	return CheckResult{Name: "<missing " + name + ">"}
}

func evNamed(rec *fakeRec, name string) *run.Event {
	for i := range rec.evs {
		if rec.evs[i].Name == name {
			return &rec.evs[i]
		}
	}
	return nil
}

func TestWhenFileFallbackAndTools(t *testing.T) {
	p := pack.Pack{ID: "x", Checks: []pack.Check{
		{Name: "fmt", Kind: "format", Argv: []string{"fmtr", "check"}, Required: true, WhenFile: []string{".fmtr.toml"}},
		{Name: "sync", Kind: "deps", Argv: []string{"uv", "sync"}, Required: true, WhenFile: []string{"uv.lock"},
			FallbackArgv: []string{"pip", "install", "-e", "."}},
		{Name: "lock", Kind: "deps", Argv: []string{"restore", "--locked"}, WhenFile: []string{"**/packages.lock.json"}},
		{Name: "wrapper", Kind: "build", Argv: []string{"./mvnw", "-q", "verify"}, Required: true},
		{Name: "opt", Kind: "lint", Argv: []string{"lintr"}, Install: "get lintr"},
		{Name: "req", Kind: "lint", Argv: []string{"needed"}, Install: "get needed", Required: true},
		{Name: "swap", Kind: "lint", Argv: []string{"fancy"}, FallbackArgv: []string{"plain", "-x"}},
	}}
	fs := newMemFS(map[string]string{"/p/a/b/packages.lock.json": "{}", "/p/mvnw": "#!/bin/sh"})
	cmd := &scriptCmd{has: map[string]bool{"lintr": false, "needed": false, "fancy": false}}
	rec := &fakeRec{}
	s := New(cmd, fakeArch{ok: true}, rec, &tick{}).WithFiles(fs)
	res, err := s.Run(context.Background(), Options{Root: "/p", Packs: []langpack.Detected{detected(p, ".")}})
	if err != nil {
		t.Fatal(err)
	}
	if c := byName(res, "fmt"); !c.Skipped || !strings.Contains(c.Output, ".fmtr.toml") {
		t.Fatalf("when_file miss must skip a required check: %+v", c)
	}
	if ev := evNamed(rec, "sync"); ev == nil || !strings.Contains(ev.Note, "override: fallback_argv (no uv.lock)") {
		t.Fatalf("%+v", ev)
	}
	if c := byName(res, "opt"); !c.Skipped || c.Output != "lintr not installed; install: get lintr" {
		t.Fatalf("%+v", c)
	}
	if c := byName(res, "req"); c.OK || c.Skipped || c.Output != "needed not installed; install: get needed" {
		t.Fatalf("%+v", c)
	}
	if ev := evNamed(rec, "swap"); ev == nil || !strings.Contains(ev.Note, "override: fallback_argv (fancy not installed)") {
		t.Fatalf("%+v", ev)
	}
	want := []string{"pip install -e .", "restore --locked", "./mvnw -q verify", "plain -x"}
	if got := cmd.argvs(); !slices.Equal(got, want) {
		t.Fatalf("%q", got)
	}
	// The when_file skip is no "required check skipped": only req fails the gate.
	if all := evNamed(rec, run.GateAll); all == nil || *all.OK || all.Note != "" {
		t.Fatalf("%+v", all)
	}
	// Without the wrapper file, the path-like tool is missing.
	delete(fs.files, "/p/mvnw")
	res, _ = s.Run(context.Background(), Options{Root: "/p", Packs: []langpack.Detected{detected(p, ".")}})
	if c := byName(res, "wrapper"); c.OK || c.Output != "./mvnw not installed" {
		t.Fatalf("%+v", c)
	}
}

func TestOutcomes(t *testing.T) {
	p := pack.Pack{ID: "x", Checks: []pack.Check{
		{Name: "codes", Kind: "format", Argv: []string{"codes"}, ExitCodes: map[string]string{"8": "fail", "1": "pass", "3": "skip"}},
		{Name: "vuln", Kind: "vuln", Argv: []string{"vuln"}, FailRegex: `"vulnerabilities"\s*:\s*\[\s*\{`},
		{Name: "env", Kind: "vuln", Argv: []string{"nvd"}, EnvRequired: []string{"NVD_TOKEN"}},
		{Name: "envreq", Kind: "lint", Argv: []string{"x"}, Required: true, EnvRequired: []string{"MISSING"}},
		{Name: "envok", Kind: "lint", Argv: []string{"y"}, EnvRequired: []string{"HOME_X", "OWN"}, Env: map[string]string{"OWN": "1"}},
		{Name: "off", Kind: "sast", Argv: []string{"sast"}, Default: "off"},
		{Name: "sig", Kind: "test", Argv: []string{"sig"}, SignalRegex: "^BOOM"},
	}}
	big := strings.Repeat("noise\n", 100) + "BOOM here\n"
	codes := map[string]error{"codes": exitErr(1)}
	cmd := &scriptCmd{fn: func(argv string, _ []string) (string, error) {
		switch argv {
		case "vuln":
			return "{\"vulnerabilities\": [\n {\"id\": 1}]}", nil
		case "sig":
			return big, exitErr(1)
		}
		return "out of " + argv, codes[argv]
	}}
	fs := newMemFS(nil)
	fs.env = map[string]string{"HOME_X": "/h"}
	o := Options{Root: "/p", Packs: []langpack.Detected{detected(p, ".")}}
	s := New(cmd, fakeArch{ok: true}, &fakeRec{}, &tick{}).WithFiles(fs).WithEnvLookup(fs)
	res, _ := s.Run(context.Background(), o)
	if c := byName(res, "codes"); !c.OK {
		t.Fatalf("exit 1 mapped to pass: %+v", c)
	}
	if c := byName(res, "vuln"); c.OK || c.Skipped {
		t.Fatalf("fail_regex on exit 0: %+v", c)
	}
	if c := byName(res, "env"); !c.Skipped || c.Output != "missing env NVD_TOKEN" {
		t.Fatalf("%+v", c)
	}
	if c := byName(res, "envreq"); c.OK || c.Output != "missing env MISSING" {
		t.Fatalf("%+v", c)
	}
	if c := byName(res, "envok"); !c.OK {
		t.Fatalf("%+v", c)
	}
	if c := byName(res, "off"); !c.Skipped || slices.Contains(cmd.argvs(), "sast") {
		t.Fatalf("%+v", c)
	}
	if c := byName(res, "sig"); !strings.HasPrefix(c.Output, "BOOM here") {
		t.Fatalf("%q", c.Output[:20])
	}
	codes["codes"] = exitErr(8)
	res, _ = New(cmd, fakeArch{ok: true}, &fakeRec{}, &tick{}).Run(context.Background(), o)
	if c := byName(res, "codes"); c.OK {
		t.Fatalf("%+v", c)
	}
	codes["codes"] = exitErr(3)
	o.Enable = []string{"off"}
	cmd.calls = nil
	res, _ = New(cmd, fakeArch{ok: true}, &fakeRec{}, &tick{}).Run(context.Background(), o)
	if c := byName(res, "codes"); !c.Skipped || c.Output != "out of codes" {
		t.Fatalf("%+v", c)
	}
	if !slices.Contains(cmd.argvs(), "sast") {
		t.Fatal("--enable did not run the off check")
	}
	if c := byName(res, "env"); !c.Skipped {
		t.Fatalf("without an env port a var is missing: %+v", c)
	}
}

func TestRepeatSeedsAndRaceVariants(t *testing.T) {
	p := pack.Pack{ID: "x", Checks: []pack.Check{
		{Name: "rep", Kind: "test", Argv: []string{"t", "--seed", "{seed}"}, Repeat: 3, Required: true},
		{Name: "unit", Kind: "test", Argv: []string{"unit"}, Required: true},
	}, Race: pack.Race{Mode: pack.RaceNone, Checks: []string{"unit"}, Variants: []map[string]string{{"ERL_FLAGS": "+S 1:1"}, {"ERL_FLAGS": "+S 8:8"}}}}
	cmd := &scriptCmd{}
	rec := &fakeRec{}
	s := New(cmd, fakeArch{ok: true}, rec, &tick{})
	res, _ := s.Run(context.Background(), Options{Root: "/p", Env: []string{"A=1"}, Packs: []langpack.Detected{detected(p, ".")}})
	if !res.OK {
		t.Fatal(names(res))
	}
	var seeds []string
	for _, c := range cmd.calls[:3] {
		f := strings.Fields(c.argv)
		seeds = append(seeds, f[2])
	}
	if seeds[0] == seeds[1] || seeds[1] == seeds[2] {
		t.Fatal(seeds)
	}
	if ev := evNamed(rec, "rep"); ev == nil || !strings.Contains(ev.Note, "seeds "+strings.Join(seeds, ",")) {
		t.Fatalf("%+v %v", ev, seeds)
	}
	if len(cmd.calls) != 5 || !slices.Equal(cmd.calls[3].env, []string{"A=1", "ERL_FLAGS=+S 1:1"}) || !slices.Equal(cmd.calls[4].env, []string{"A=1", "ERL_FLAGS=+S 8:8"}) {
		t.Fatalf("%+v", cmd.calls)
	}
	if ev := evNamed(rec, "unit"); !strings.Contains(ev.Note, "race variants 2") {
		t.Fatalf("%+v", ev)
	}
	// A failing variant stops the check and is named in the output.
	cmd2 := &scriptCmd{fn: func(argv string, env []string) (string, error) {
		if slices.Contains(env, "ERL_FLAGS=+S 8:8") {
			return "race!", exitErr(1)
		}
		return "", nil
	}}
	res, _ = New(cmd2, fakeArch{ok: true}, &fakeRec{}, &tick{}).Run(context.Background(), Options{Packs: []langpack.Detected{detected(p, ".")}})
	if c := byName(res, "unit"); c.OK || !strings.HasPrefix(c.Output, "variant ERL_FLAGS=+S 8:8:\nrace!") {
		t.Fatalf("%q", c.Output)
	}
	// Config-skipping a race check fails the full gate.
	rec3 := &fakeRec{}
	_, _ = New(&scriptCmd{}, fakeArch{ok: true}, rec3, &tick{}).Run(context.Background(), Options{ConfigSkip: []string{"unit"}, Packs: []langpack.Detected{detected(p, ".")}})
	if all := evNamed(rec3, run.GateAll); *all.OK || all.Note != "required check skipped: unit" {
		t.Fatalf("%+v", all)
	}
}

func TestRepeatStopsAtFailure(t *testing.T) {
	p := pack.Pack{ID: "x", Checks: []pack.Check{{Name: "rep", Kind: "test", Argv: []string{"t", "{seed}"}, Repeat: 3}}}
	n := 0
	cmd := &scriptCmd{fn: func(string, []string) (string, error) {
		n++
		if n == 2 {
			return "flaky", exitErr(1)
		}
		return "", nil
	}}
	res, _ := New(cmd, fakeArch{ok: true}, &fakeRec{}, &tick{}).Run(context.Background(), Options{Packs: []langpack.Detected{detected(p, ".")}})
	if c := byName(res, "rep"); c.OK || len(cmd.calls) != 2 || !strings.HasPrefix(c.Output, "run 2/3, seed ") {
		t.Fatalf("%+v %d", c, len(cmd.calls))
	}
}

func TestRaceFlagPreflight(t *testing.T) {
	p := pack.Pack{ID: "x", Checks: []pack.Check{{Name: "test", Kind: "test", Argv: []string{"swift", "test"}, Required: true}},
		Race: pack.Race{Mode: pack.RaceFlag, Flags: []string{"--sanitize=thread"}, Checks: []string{"test"}}}
	cmd := &scriptCmd{}
	res, _ := New(cmd, fakeArch{ok: true}, &fakeRec{}, &tick{}).Run(context.Background(), Options{Packs: []langpack.Detected{detected(p, ".")}})
	if c := byName(res, "test"); c.OK || c.Output != "race policy unavailable: the race check runs without --sanitize=thread" || len(cmd.calls) != 0 {
		t.Fatalf("%+v", c)
	}
	// The race env satisfies flag mode too.
	p.Race.Env = map[string]string{"SAN": "thread"}
	res, _ = New(cmd, fakeArch{ok: true}, &fakeRec{}, &tick{}).Run(context.Background(), Options{Env: []string{"SAN=thread"}, Packs: []langpack.Detected{detected(p, ".")}})
	if !byName(res, "test").OK {
		t.Fatal(byName(res, "test"))
	}
}

func TestMultiPackAndRunEnv(t *testing.T) {
	web := pack.Pack{ID: "web", Scope: pack.Scope{Pkgs: []string{"src"}},
		Env:       map[string]string{"NODE_ENV": "test", "T": "{tmp}/x"},
		CacheEnv:  map[string]string{"npm_config_cache": "cache"},
		CacheArgv: map[string][]string{"tool": {"--cache", "{cache}"}, "tool build": {"--out", "{run}/o"}},
		Checks: []pack.Check{
			{Name: "build", Kind: "build", Argv: []string{"tool", "build", "{pkgs}"}, Required: true},
			{Name: "test", Kind: "test", Argv: []string{"tool", "test", "--dir", "{coverage_dir}"}, Required: true},
		}}
	cmd := &scriptCmd{fn: func(argv string, _ []string) (string, error) {
		if argv == "go vet ./..." {
			return "vet: bad", exitErr(1)
		}
		return "", nil
	}}
	rec := &fakeRec{}
	fs := newMemFS(nil)
	s := New(cmd, fakeArch{ok: true}, rec, &tick{}).WithCoverage(&fakeCov{}, CoverageConfig{}).WithFiles(fs)
	o := Options{Root: "/p", RunDir: "/run", CoverDir: "/cov", Env: []string{"GOCACHE=/run/go/cache", "GOTMPDIR=/run/go/tmp"},
		Packs: []langpack.Detected{goOpts(Options{}).Packs[0], detected(web, "apps/web")}, ConfigSkip: []string{"build"}}
	res, _ := s.Run(context.Background(), o)
	if res.OK {
		t.Fatal("vet failed")
	}
	var webCalls []call
	for _, c := range cmd.calls {
		if c.dir == "/p/apps/web" {
			webCalls = append(webCalls, c)
		} else if !slices.Equal(c.env, o.Env) || c.dir != "/p" {
			t.Fatalf("go call %+v", c)
		}
	}
	if len(webCalls) != 1 || webCalls[0].argv != "tool --cache /run/web/cache test --dir /cov/web-apps_web" {
		t.Fatalf("%+v", webCalls)
	}
	wantEnv := []string{"GOCACHE=/run/go/cache", "GOTMPDIR=/run/go/tmp", "npm_config_cache=/run/web/cache", "NODE_ENV=test", "T=/run/web/tmp/x"}
	if !slices.Equal(webCalls[0].env, wantEnv) {
		t.Fatalf("%q", webCalls[0].env)
	}
	if ev := evNamed(rec, "vet"); ev.Pack != "go@." {
		t.Fatalf("%+v", ev)
	}
	var webTest *run.Event
	for i := range rec.evs {
		if rec.evs[i].Pack == "web@apps/web" && rec.evs[i].Name == "test" {
			webTest = &rec.evs[i]
		}
	}
	if webTest == nil || !*webTest.OK {
		t.Fatalf("%+v", rec.evs)
	}
	all := evNamed(rec, run.GateAll)
	if *all.OK || all.Note != "required check skipped: build (web@apps/web)" || all.Pack != "" {
		t.Fatalf("%+v", all)
	}
	if !slices.Contains(fs.dirs, "/cov/web-apps_web") || !slices.Contains(fs.dirs, "/cov/go") {
		t.Fatal(fs.dirs)
	}
	if c := byName(res, "test"); c.Pack != "go@." {
		t.Fatalf("%+v", c)
	}
	// The go coverage file is per pack when several run.
	if !slices.ContainsFunc(cmd.argvs(), func(a string) bool { return strings.Contains(a, "-coverprofile=/cov/go/coverage.out") }) {
		t.Fatal(cmd.argvs())
	}
	// cache_argv takes the longest matching prefix.
	cmd.calls = nil
	o.ConfigSkip, o.Packs = nil, []langpack.Detected{detected(web, "apps/web")}
	_, _ = s.Run(context.Background(), o)
	if got := cmd.argvs()[0]; got != "tool build --out /run/o src" {
		t.Fatal(got)
	}
	// Without a run: no cache env or argv, env templates use the cover dir.
	cmd.calls = nil
	o.RunDir, o.Env = "", nil
	_, _ = s.Run(context.Background(), o)
	if c := cmd.calls[0]; c.argv != "tool build src" || !slices.Equal(c.env, []string{"NODE_ENV=test", "T=/cov/web/tmp/x"}) {
		t.Fatalf("%+v", c)
	}
}

func TestPlaceholders(t *testing.T) {
	p := pack.Pack{ID: "x",
		Placeholders: map[string]pack.Placeholder{
			"cp":      {Argv: []string{"clj", "-Spath"}},
			"covdir":  {Argv: []string{"swift", "path"}, Dirname: true},
			"edition": {File: "Cargo.toml", Regex: `edition\s*=\s*"(\d+)"`},
			"objs":    {Glob: []string{"{covdir}/*.o"}},
			"bad":     {File: "nope", Regex: "x"},
		},
		Checks: []pack.Check{
			{Name: "a", Kind: "lint", Argv: []string{"use", "{cp}", "--ed={edition}", "{objs}"}},
			{Name: "b", Kind: "lint", Argv: []string{"use", "{bad}"}},
			{Name: "c", Kind: "lint", Argv: []string{"use", "{nosuch}"}},
		}}
	cmd := &scriptCmd{stdout: map[string]string{"clj": "a.jar:b.jar\n", "swift": "/b/codecov/default.json\n"}}
	fs := newMemFS(map[string]string{"/p/Cargo.toml": "[package]\nedition = \"2024\"\n", "/b/codecov/x.o": "", "/b/codecov/y.o": "", "/b/codecov/sub/z.o": ""})
	res, _ := New(cmd, fakeArch{ok: true}, &fakeRec{}, &tick{}).WithFiles(fs).Run(context.Background(), Options{Root: "/p", Packs: []langpack.Detected{detected(p, ".")}})
	got := cmd.argvs()
	if !slices.Contains(got, "use a.jar:b.jar --ed=2024 /b/codecov/x.o /b/codecov/y.o") {
		t.Fatalf("%q", got)
	}
	if c := byName(res, "b"); c.OK || !strings.Contains(c.Output, "placeholder {bad}") {
		t.Fatalf("%+v", c)
	}
	if c := byName(res, "c"); c.OK || !strings.Contains(c.Output, "unknown placeholder {nosuch}") {
		t.Fatalf("%+v", c)
	}
}

// packCov is a fake language-neutral coverage port.
type packCov struct {
	fakeCov
	files map[string]string
	globs []string
}

func (p *packCov) Profile(path string) (string, error) {
	t, ok := p.files[path]
	if !ok {
		return "", errors.New("no " + path)
	}
	return t, nil
}
func (p *packCov) DiffGlobs(_ context.Context, _, _ string, globs []string) (Diff, error) {
	p.globs = globs
	return p.diff, nil
}
func (p *packCov) HasTestFiles(string, string, []string) bool { return false }
func (p *packCov) Head(_, file string) string {
	if strings.Contains(file, "gen") {
		return "// @generated\n"
	}
	return ""
}

func TestCoverageLcovGlobProduce(t *testing.T) {
	p := pack.Pack{ID: "ts", SourceGlobs: []string{"src/**/*.ts"}, TestGlobs: []string{"**/*.test.ts"},
		Generated: pack.Generated{HeaderRegex: "^// @generated"},
		Checks: []pack.Check{
			{Name: "test", Kind: "test", Argv: []string{"vitest", "run"}, Required: true},
			{Name: "coverage", Kind: "coverage", Internal: "coverage", Required: true, Skippable: new(bool)},
		},
		Coverage: pack.Coverage{Format: "lcov", Path: "coverage/*.info", ProducedBy: []string{"test"},
			Produce: []pack.Produce{{Argv: []string{"export", "x"}, StdoutTo: "coverage/b.info"}}}}
	lcovA := "SF:/p/web/src/a.ts\nDA:1,1\nDA:2,1\nDA:3,0\nend_of_record\n"
	lcovB := "SF:src/gen.ts\nDA:1,0\nend_of_record\n"
	fs := newMemFS(map[string]string{"/p/web/coverage/stale.info": "old"})
	cmd := &scriptCmd{stdout: map[string]string{"export": lcovB}, fn: func(argv string, _ []string) (string, error) {
		if argv == "vitest run" {
			fs.files["/p/web/coverage/a.info"] = lcovA
		}
		return "", nil
	}}
	diff := "diff --git a/src/a.ts b/src/a.ts\n--- a/src/a.ts\n+++ b/src/a.ts\n@@ -0,0 +1,3 @@\n+a\n+b\n+c\n"
	cov := &packCov{fakeCov: fakeCov{diff: Diff{Base: "main", Text: diff, Untracked: []string{"src/gen.ts", "src/a.test.ts"}}}, files: fs.files}
	rec := &fakeRec{}
	s := New(cmd, fakeArch{ok: true}, rec, &tick{}).WithCoverage(cov, CoverageConfig{}).WithFiles(fs)
	res, err := s.Run(context.Background(), Options{Root: "/p", CoverDir: "/cov", Packs: []langpack.Detected{detected(p, "web")}})
	if err != nil {
		t.Fatal(err)
	}
	c := byName(res, "coverage")
	// 2 of 3 changed lines covered: below 70% fails; the generated file and
	// the test file are not judged, and no "package has no tests" (D7).
	if c.OK || !strings.Contains(c.Output, "new code coverage 66.6% < 70%") || strings.Contains(c.Output, "gen.ts") || strings.Contains(c.Output, "has no tests") {
		t.Fatalf("%+v", c)
	}
	if !slices.Contains(fs.removed, "/p/web/coverage/stale.info") || fs.files["/p/web/coverage/b.info"] != lcovB {
		t.Fatalf("removed %v", fs.removed)
	}
	if !slices.Equal(cov.globs, p.SourceGlobs) {
		t.Fatal(cov.globs)
	}
	// No report at all.
	fs2 := newMemFS(nil)
	p.Coverage.Produce = nil
	res, _ = New(&scriptCmd{}, fakeArch{ok: true}, &fakeRec{}, &tick{}).WithCoverage(cov, CoverageConfig{}).WithFiles(fs2).
		Run(context.Background(), Options{Root: "/p", Packs: []langpack.Detected{detected(p, "web")}})
	if c := byName(res, "coverage"); c.OK || !strings.Contains(c.Output, "no coverage report at coverage/*.info") {
		t.Fatalf("%+v", c)
	}
	// Coverage needs its producing check.
	res, _ = New(&scriptCmd{}, fakeArch{ok: true}, &fakeRec{}, &tick{}).WithCoverage(cov, CoverageConfig{}).WithFiles(fs2).
		Run(context.Background(), Options{Only: []string{"coverage"}, Packs: []langpack.Detected{detected(p, "web")}})
	if c := byName(res, "coverage"); c.OK || c.Output != "coverage: coverage needs the test check's coverage report: run test too (--only test,coverage)" {
		t.Fatalf("%q", c.Output)
	}
}

func TestProduceErrors(t *testing.T) {
	p := pack.Pack{ID: "sw", Checks: []pack.Check{
		{Name: "test", Kind: "test", Argv: []string{"t"}},
		{Name: "coverage", Kind: "coverage", Internal: "coverage"},
	}, Coverage: pack.Coverage{Format: "lcov", Path: "{coverage}", ProducedBy: []string{"test"},
		Produce: []pack.Produce{{Argv: []string{"llvm-cov", "export"}, StdoutTo: "{coverage}"}, {Argv: []string{"merge"}}}}}
	cmd := &scriptCmd{fn: func(argv string, _ []string) (string, error) {
		if argv == "merge" {
			return "merge broke", exitErr(2)
		}
		return "SF:a\nend_of_record\n", nil
	}}
	fs := newMemFS(nil)
	res, _ := New(cmd, fakeArch{ok: true}, &fakeRec{}, &tick{}).WithCoverage(&fakeCov{}, CoverageConfig{}).WithFiles(fs).
		Run(context.Background(), Options{CoverDir: "/cov", Packs: []langpack.Detected{detected(p, ".")}})
	if c := byName(res, "coverage"); c.OK || !strings.Contains(c.Output, "coverage.produce merge") || !strings.Contains(c.Output, "merge broke") {
		t.Fatalf("%+v", c)
	}
	if fs.files[filepath.Join("/cov", "sw", "coverage.info")] == "" {
		t.Fatal(fs.files)
	}
}

func TestGlobFiles(t *testing.T) {
	fs := newMemFS(map[string]string{"/r/a/x.xml": "", "/r/a/b/y.xml": "", "/r/a/b/c/z.xml": "", "/r/f.txt": ""})
	for pat, want := range map[string]string{
		"/r/a/*.xml":      "/r/a/x.xml",
		"/r/a/**/*.xml":   "/r/a/b/c/z.xml,/r/a/b/y.xml,/r/a/x.xml",
		"/r/a/b":          "/r/a/b/c/z.xml,/r/a/b/y.xml",
		"/r/a/**":         "/r/a/b/c/z.xml,/r/a/b/y.xml,/r/a/x.xml",
		"/r/f.txt":        "/r/f.txt",
		"/r/none":         "",
		"/r/*/b/[yz].xml": "/r/a/b/y.xml",
	} {
		got, err := globFiles(fs, pat)
		if err != nil || strings.Join(got, ",") != want {
			t.Errorf("%s: %v %v", pat, got, err)
		}
	}
}
