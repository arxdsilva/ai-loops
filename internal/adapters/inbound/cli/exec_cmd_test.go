package cli

import (
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/application/langpack"
	"github.com/arxdsilva/ai-loops/internal/domain/pack"
	"github.com/arxdsilva/ai-loops/internal/domain/workspace"
)

// fakePackWS is a Workspace with the PackWorkspace side.
type fakePackWS struct {
	*fakeWS
	packs   []langpack.Detected
	envDirs []string
	packErr error
	execEnv []string
	execArg []string
	execRan bool
	code    int
}

func (w *fakePackWS) Packs() []langpack.Detected { return w.packs }
func (w *fakePackWS) PackEnv(dir string, p pack.Pack) ([]string, error) {
	if w.packErr != nil {
		return nil, w.packErr
	}
	w.envDirs = append(w.envDirs, dir+"/"+p.ID)
	return workspace.RunEnv(dir, p), nil
}
func (w *fakePackWS) Exec(_ context.Context, _ string, env, argv []string, _ io.Reader, out, _ io.Writer) int {
	w.execRan, w.execEnv, w.execArg = true, env, argv
	io.WriteString(out, "ran\n")
	return w.code
}

// execLangs serves Get from a map (the rest is unused by exec).
type execLangs struct{ get map[string]pack.Pack }

func (f *execLangs) All() ([]pack.Pack, error) { return nil, nil }
func (f *execLangs) Get(id string) (pack.Pack, error) {
	if p, ok := f.get[id]; ok {
		return p, nil
	}
	return pack.Pack{}, errors.New("no pack " + id)
}
func (f *execLangs) Detect(string, []langpack.Override) ([]langpack.Detected, error) {
	return nil, nil
}

var (
	goPk  = pack.Pack{ID: "go", CacheEnv: map[string]string{"GOCACHE": "cache", "GOTMPDIR": "tmp"}, Checks: []pack.Check{{Name: "build", Tool: "go"}}}
	fooPk = pack.Pack{ID: "foo", CacheEnv: map[string]string{"FOO_CACHE": "cache"}, Env: map[string]string{"FOO_MODE": "ci"},
		CacheArgv: map[string][]string{"foo build": {"--cache", "{cache}"}}}
	barPk = pack.Pack{ID: "bar", Checks: []pack.Check{{Name: "lint", Tool: "barlint"}}}
)

func packWSSetup(t *testing.T, ps ...pack.Pack) (*rcEnv, *fakePackWS) {
	t.Helper()
	e, ws := wsSetup(t)
	pw := &fakePackWS{fakeWS: ws}
	for _, p := range ps {
		pw.packs = append(pw.packs, langpack.Detected{Pack: p, ID: p.ID, Root: "."})
	}
	e.app.Workspace = pw
	return e, pw
}

func TestExecNoRunRunsPlain(t *testing.T) {
	e, pw := packWSSetup(t, fooPk)
	pw.code = 4
	if code := e.do("exec", "--", "foo", "build", "-x"); code != 4 || pw.execEnv != nil || !slices.Equal(pw.execArg, []string{"foo", "build", "-x"}) {
		t.Fatalf("%d %v %v", code, pw.execEnv, pw.execArg)
	}
	if e.out.String() != "ran\n" {
		t.Fatal(e.out)
	}
}

func TestExecWithRun(t *testing.T) {
	e, pw := packWSSetup(t, goPk, fooPk, barPk)
	e.do("run", "start", "--title", "x")
	dir := e.log.Dir(e.ptr.id)
	// by tool name, with cache_argv after the longest matching prefix.
	if code := e.do("exec", "--", "foo", "build", "-x"); code != 0 {
		t.Fatal(code, e.err)
	}
	if !slices.Equal(pw.execEnv, workspace.RunEnv(dir, fooPk)) || !slices.Equal(pw.execArg, []string{"foo", "build", "--cache", workspace.PackCache(dir, "foo"), "-x"}) {
		t.Fatalf("%v %v", pw.execEnv, pw.execArg)
	}
	// go through exec gets exactly the env `loops go` gets.
	e.do("exec", "go", "test", "./...")
	if !slices.Equal(pw.execEnv, workspace.Env(dir)) || !slices.Equal(pw.execArg, []string{"go", "test", "./..."}) {
		t.Fatalf("%v %v", pw.execEnv, pw.execArg)
	}
	e.do("exec", "barlint", ".")
	if len(pw.execEnv) != 0 || pw.envDirs[len(pw.envDirs)-1] != dir+"/bar" {
		t.Fatalf("%v %v", pw.execEnv, pw.envDirs)
	}
	// --pack wins over the tool name.
	e.do("exec", "--pack", "foo", "--", "go", "version")
	if !slices.Equal(pw.execEnv, workspace.RunEnv(dir, fooPk)) {
		t.Fatal(pw.execEnv)
	}
	// no pack uses make and there are three: plain, with a note.
	if code := e.do("exec", "make"); code != 0 || pw.execEnv != nil || !strings.Contains(e.err.String(), "no language pack for make") {
		t.Fatal(code, pw.execEnv, e.err)
	}
	pw.packErr = errors.New("read-only")
	if code := e.do("exec", "foo"); code != 1 || !strings.Contains(e.err.String(), "read-only") {
		t.Fatal(code, e.err)
	}
}

func TestExecOnlyPackAndNamedPack(t *testing.T) {
	e, pw := packWSSetup(t, fooPk)
	e.do("run", "start", "--title", "x")
	dir := e.log.Dir(e.ptr.id)
	if e.do("exec", "make", "all") != 0 || !slices.Equal(pw.execEnv, workspace.RunEnv(dir, fooPk)) {
		t.Fatal("only pack", pw.execEnv)
	}
	// --pack naming an undetected pack loads it.
	e.app.Langs = &execLangs{get: map[string]pack.Pack{"bar": barPk}}
	if code := e.do("exec", "--pack", "bar", "barlint"); code != 0 || pw.envDirs[len(pw.envDirs)-1] != dir+"/bar" {
		t.Fatal(code, pw.envDirs, e.err)
	}
	if code := e.do("exec", "--pack", "nope", "x"); code != 2 || !strings.Contains(e.err.String(), "no language pack nope") {
		t.Fatal(code, e.err)
	}
	e.app.Langs = nil
	if code := e.do("exec", "--pack", "bar", "x"); code != 2 {
		t.Fatal(code)
	}
}

func TestExecUsageAndErrors(t *testing.T) {
	e, _ := packWSSetup(t)
	if code := e.do("exec"); code != 2 || !strings.Contains(e.err.String(), "usage: loops exec") {
		t.Fatal(code, e.err)
	}
	if code := e.do("exec", "--bogus", "x"); code != 2 {
		t.Fatal(code)
	}
	e.app.Workspace = e.app.Workspace.(*fakePackWS).fakeWS // no PackWorkspace side
	if code := e.do("exec", "x"); code != 1 || !strings.Contains(e.err.String(), "workspace unavailable") {
		t.Fatal(code, e.err)
	}
}

func TestRunEnvPacks(t *testing.T) {
	goExtra := goPk
	goExtra.Env = map[string]string{"CGO_ENABLED": "1"}
	e, pw := packWSSetup(t, goExtra, fooPk, barPk)
	e.do("run", "start", "--title", "x")
	dir := e.log.Dir(e.ptr.id)
	goLine, _ := workspace.Exports("sh", workspace.GoCache(dir), workspace.GoTmp(dir))
	want := goLine + "\n" +
		"export CGO_ENABLED='1'\n" +
		"export FOO_CACHE='" + workspace.PackCache(dir, "foo") + "' FOO_MODE='ci'\n"
	if code := e.do("run", "env"); code != 0 || e.out.String() != want {
		t.Fatalf("%d %q\nwant %q", code, e.out, want)
	}
	if len(pw.envDirs) != 3 {
		t.Fatal("pack dirs not ensured", pw.envDirs)
	}
	if code := e.do("run", "env", "--shell", "fish"); code != 0 || !strings.Contains(e.out.String(), "set -gx FOO_CACHE '"+workspace.PackCache(dir, "foo")+"'; set -gx FOO_MODE 'ci'") {
		t.Fatal(e.out)
	}
	pw.packErr = errors.New("disk")
	if code := e.do("run", "env"); code != 1 || !strings.Contains(e.err.String(), "disk") {
		t.Fatal(code, e.err)
	}
}
