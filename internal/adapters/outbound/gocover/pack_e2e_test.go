package gocover

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/application/gate"
	"github.com/arxdsilva/ai-loops/internal/application/langpack"
	"github.com/arxdsilva/ai-loops/internal/domain/pack"
	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

// shCmd runs real commands (gate.Commander, EnvCommander, StdoutCommander).
type shCmd struct{}

func (shCmd) Run(ctx context.Context, dir, name string, args ...string) (string, error) {
	return shCmd{}.RunEnv(ctx, dir, nil, name, args...)
}

func (shCmd) RunEnv(ctx context.Context, dir string, env []string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir, cmd.Env = dir, append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (shCmd) RunStdout(ctx context.Context, dir string, env []string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir, cmd.Env = dir, append(os.Environ(), env...)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	return out.String(), err
}

func (shCmd) Has(n string) bool { _, err := exec.LookPath(n); return err == nil }

type evRec struct{ evs []run.Event }

func (r *evRec) Record(_ context.Context, e run.Event) error { r.evs = append(r.evs, e); return nil }

// shPack is a test pack whose checks are `sh -c` scripts: it exercises
// fails_on output, when_file, env, repeat with seeds, race variants,
// exit_codes, coverage.produce, an lcov glob and stale-report cleanup.
func shPack(t *testing.T, appDir string) pack.Pack {
	t.Helper()
	lcov := "SF:" + filepath.ToSlash(filepath.Join(appDir, "src", "a.sh")) + "\nDA:1,1\nDA:2,1\nend_of_record\n"
	js := `{"schema":1,"id":"shx","name":"Shell test","versions":{"stable":"1","as_of":"2026-11"},
 "detect":[{"file":"shx.toml"}],"scope":{"pkgs":["src"],"pkg_dirs":["src"]},
 "source_globs":["src/**/*.sh"],"test_globs":["**/*_test.sh"],"generated":{"globs":["**/gen_*.sh"]},
 "env":{"SHX_MODE":"gate"},
 "checks":[
  {"name":"build","kind":"build","argv":["sh","-c","test \"$SHX_MODE\" = gate && test -d \"$1\"","build","{pkgs}"],"required":true,"tool":"sh"},
  {"name":"fmt","kind":"format","argv":["sh","-c","echo vendor/x.sh; echo"],"fails_on":"output","output_exclude":["vendor/"],"note":"files need fmt:"},
  {"name":"style","kind":"lint","argv":["sh","-c","exit 9"],"when_file":["style.cfg"]},
  {"name":"codes","kind":"lint","argv":["sh","-c","exit 3"],"exit_codes":{"3":"pass"}},
  {"name":"test","kind":"test","argv":["sh","-c","echo seed=$1 >> seeds.txt; echo $SCHED >> sched.txt; mkdir -p out; printf '%s' \"$LCOV\" > out/a.info","test","{seed}"],
   "env":{"LCOV":` + strconvQuote(lcov) + `},"repeat":2,"required":true},
  {"name":"coverage","kind":"coverage","internal":"coverage","required":true,"skippable":false}
 ],
 "test_commands":[{"argv_prefix":["sh"]}],
 "race":{"mode":"none","checks":["test"],"variants":[{"SCHED":"one"},{"SCHED":"many"}],"substitute":"two scheduler variants"},
 "coverage":{"format":"lcov","path":"out/*.info","produced_by":["test"],
   "produce":[{"argv":["sh","-c","printf 'SF:src/a.sh\\nDA:2,1\\nend_of_record\\n'"],"stdout_to":"out/b.info"}]},
 "post_edit":null,"cache_env":{},"artifacts":[],"arch":{"parser":"none"}}`
	p, err := pack.Parse([]byte(js))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func strconvQuote(s string) string {
	b := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(s)
	return `"` + b + `"`
}

func TestFakePackEndToEnd(t *testing.T) {
	needGit(t)
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	dir := t.TempDir()
	app := filepath.Join(dir, "app")
	git(t, dir, "init", "-q")
	write(t, dir, "README", "x\n")
	write(t, app, "shx.toml", "")
	commitAll(t, dir, "base")
	git(t, dir, "checkout", "-q", "-b", "feature")
	write(t, app, "src/a.sh", "echo a\necho b\n")
	write(t, app, "src/gen_x.sh", "echo gen\n")
	write(t, app, "out/stale.info", "SF:src/zzz.sh\nDA:1,0\nend_of_record\n")
	commitAll(t, dir, "feature")

	p := shPack(t, app)
	rec := &evRec{}
	r := New()
	s := gate.New(shCmd{}, nil, rec, clock{}).WithCoverage(r, gate.CoverageConfig{})
	res, err := s.Run(context.Background(), gate.Options{
		Root: dir, Base: "main", CoverDir: t.TempDir(),
		Packs: []langpack.Detected{{Pack: p, ID: p.ID, Name: p.Name, Root: "app"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range res.Checks {
		st := "ok"
		switch {
		case c.Skipped:
			st = "skip"
		case !c.OK:
			st = "FAIL:" + c.Output
		}
		got = append(got, c.Name+":"+st)
	}
	if !res.OK || strings.Join(got, " ") != "build:ok fmt:ok style:skip codes:ok test:ok coverage:ok" {
		t.Fatalf("%v", got)
	}
	seeds, _ := os.ReadFile(filepath.Join(app, "seeds.txt"))
	sched, _ := os.ReadFile(filepath.Join(app, "sched.txt"))
	if strings.Count(string(seeds), "seed=") != 4 || string(sched) != "one\none\nmany\nmany\n" {
		t.Fatalf("seeds %q sched %q", seeds, sched)
	}
	if _, err := os.Stat(filepath.Join(app, "out", "stale.info")); !os.IsNotExist(err) {
		t.Fatal("stale report not removed")
	}
	var cov, test run.Event
	for _, e := range rec.evs {
		switch e.Name {
		case "coverage":
			cov = e
		case "test":
			test = e
		}
	}
	if cov.Note != "new code 100.0% (min 70%)" || !strings.Contains(test.Note, "seeds ") || !strings.Contains(test.Note, "race variants 2") {
		t.Fatalf("cov %+v test %+v", cov, test)
	}
	if c := res.Checks[5]; !strings.Contains(c.Output, "src/a.sh") || strings.Contains(c.Output, "gen_x") || strings.Contains(c.Output, "zzz") {
		t.Fatal(c.Output)
	}

	// A fails_on: output check with output left fails with its note.
	p.Checks[1].Argv = []string{"sh", "-c", "echo bad.sh"}
	res, _ = s.Run(context.Background(), gate.Options{Root: dir, Base: "main", Only: []string{"fmt"},
		Packs: []langpack.Detected{{Pack: p, ID: p.ID, Root: "app"}}})
	if c := res.Checks[1]; c.OK || c.Output != "files need fmt:\nbad.sh" {
		t.Fatalf("%+v", c)
	}
}

// TestGoPackInSubdir gates a Go module that lives below the repo root: the
// diff, the coverprofile and HasTests all key files relative to the pack.
func TestGoPackInSubdir(t *testing.T) {
	needGit(t)
	if testing.Short() {
		t.Skip("runs go test")
	}
	t.Setenv("GOWORK", "off")
	t.Setenv("GOFLAGS", "")
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	write(t, dir, "README", "x\n")
	write(t, dir, "svc/go.mod", "module example.com/svc\n\ngo 1.22\n")
	write(t, dir, "svc/a/a.go", aGo)
	write(t, dir, "svc/a/a_test.go", aTestGo)
	commitAll(t, dir, "base")
	git(t, dir, "checkout", "-q", "-b", "feature")
	write(t, dir, "svc/a/new.go", newGo)
	write(t, dir, "svc/b/b.go", bGo)
	s := gate.New(shCmd{}, nil, &evRec{}, clock{}).WithCoverage(New(), gate.CoverageConfig{})
	res, err := s.Run(context.Background(), gate.Options{
		Root: dir, Only: []string{gate.CheckTest, gate.CheckCoverage}, Base: "main",
		CoverProfile: filepath.Join(t.TempDir(), "cover.out"), Packs: goPacks(t, "svc"),
	})
	if err != nil {
		t.Fatal(err)
	}
	c := res.Checks[6]
	for _, want := range []string{"a/new.go:4: new code in new.go is untested", "b/b.go:4: package b has no tests"} {
		if !strings.Contains(c.Output, want) {
			t.Fatalf("lacks %q:\n%s", want, c.Output)
		}
	}
	if strings.Contains(c.Output, "svc/") {
		t.Fatalf("paths not pack-relative:\n%s", c.Output)
	}
}

func TestFilesPorts(t *testing.T) {
	r := New()
	dir := t.TempDir()
	write(t, dir, "a/x_test.py", "")
	write(t, dir, "a/b/c/deep.txt", "")
	write(t, dir, "node_modules/m/i.js", "")
	write(t, dir, "g.go", "// header\n// Code generated by x. DO NOT EDIT.\n\npackage g\nvar after = 1\n")
	var rels []string
	if err := r.Walk(dir, 2, func(n string) bool { return n == "node_modules" }, func(rel string) bool {
		rels = append(rels, rel)
		return false
	}); err != nil {
		t.Fatal(err)
	}
	slices.Sort(rels)
	if strings.Join(rels, ",") != "a/x_test.py,g.go" {
		t.Fatal(rels)
	}
	n := 0
	_ = r.Walk(dir, -1, nil, func(string) bool { n++; return true })
	if n != 1 || r.Walk(filepath.Join(dir, "g.go"), -1, nil, func(string) bool { t.Fatal("file root walked"); return true }) != nil {
		t.Fatal(n)
	}
	if !r.HasTestFiles(dir, "a", []string{"**/*_test.py"}) || r.HasTestFiles(dir, "a/b", []string{"**/*_test.py"}) || r.HasTestFiles(dir, "nope", nil) {
		t.Fatal("HasTestFiles")
	}
	if h := r.Head(dir, "g.go"); !strings.HasSuffix(h, "package g\n") || !r.Generated(dir, "g.go") || r.Head(dir, "none") != "" {
		t.Fatalf("%q", h)
	}
	p := filepath.Join(dir, "w", "f.txt")
	if err := r.MkdirAll(filepath.Dir(p)); err != nil || r.WriteFile(p, "hi") != nil || !r.Exists(p) {
		t.Fatal(err)
	}
	if s, err := r.ReadFile(p); err != nil || s != "hi" {
		t.Fatal(s, err)
	}
	if r.Remove(p) != nil || r.Exists(p) || r.Remove(p) != nil {
		t.Fatal("remove")
	}
	t.Setenv("LOOPS_GATE_X", "1")
	if v, ok := r.LookupEnv("LOOPS_GATE_X"); !ok || v != "1" {
		t.Fatal(v)
	}
}

func TestPathspecs(t *testing.T) {
	got := pathspecs([]string{"**/*.go", "src/**/*.ts", "*.py", "**/*_test.go", "lib/*.ex"})
	want := []string{"*.go", ":(glob)src/**/*.ts", ":(glob)**/*.py", "*_test.go", ":(glob)lib/*.ex"}
	if !slices.Equal(got, want) {
		t.Fatalf("%q", got)
	}
	if !slices.Equal(pathspecs(nil), []string{"."}) {
		t.Fatal("empty")
	}
}
