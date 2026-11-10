package workspace

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/domain/pack"
)

func TestExportsEnv(t *testing.T) {
	env := []string{"A=x y", "B_2=it's"}
	for shell, want := range map[string]string{
		"":           `export A='x y' B_2='it'\''s'`,
		"zsh":        `export A='x y' B_2='it'\''s'`,
		"fish":       `set -gx A 'x y'; set -gx B_2 'it\'s'`,
		"pwsh":       `$env:A='x y'; $env:B_2='it''s'`,
		"powershell": `$env:A='x y'; $env:B_2='it''s'`,
	} {
		got, err := ExportsEnv(shell, env)
		if err != nil || got != want {
			t.Errorf("%q: %q %v", shell, got, err)
		}
	}
	if got, err := ExportsEnv("sh", nil); err != nil || got != "" {
		t.Fatal(got, err)
	}
	if _, err := ExportsEnv("cmd", nil); err == nil {
		t.Fatal("want unknown shell")
	}
	for _, bad := range []string{"NOEQ", "A B=1", "1A=2", "$(x)=1", "=v"} {
		if _, err := ExportsEnv("sh", []string{bad}); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	// Exports is the two-entry case, byte for byte as before.
	got, _ := Exports("sh", "/c", "/t")
	if got != "export GOCACHE='/c' GOTMPDIR='/t'" {
		t.Fatal(got)
	}
}

func TestRunEnv(t *testing.T) {
	run := filepath.FromSlash("/r")
	goPack := pack.Pack{ID: "go", CacheEnv: map[string]string{"GOCACHE": "cache", "GOTMPDIR": "tmp"}}
	if got := RunEnv(run, goPack); !slices.Equal(got, Env(run)) {
		t.Fatalf("go: %v", got)
	}
	p := pack.Pack{
		ID:       "rust",
		CacheEnv: map[string]string{"CARGO_TARGET_DIR": "{run}/rust/target", "TMPDIR": "tmp", "bad key": "cache"},
		Env:      map[string]string{"CARGO_INCREMENTAL": "0", "TMPDIR": "ignored", "LOG": "{tmp}/x"},
	}
	want := []string{
		"CARGO_TARGET_DIR=" + run + "/rust/target",
		"TMPDIR=" + PackTmp(run, "rust"),
		"CARGO_INCREMENTAL=0",
		"LOG=" + PackTmp(run, "rust") + "/x",
	}
	if got := RunEnv(run, p); !slices.Equal(got, want) {
		t.Fatalf("%v\nwant %v", got, want)
	}
	if got := ExtraEnv(append(Env(run), "X=1"), "GOCACHE", "GOTMPDIR"); !slices.Equal(got, []string{"X=1"}) {
		t.Fatal(got)
	}
	if PackCache(run, "rust") != filepath.Join(run, "rust", "cache") {
		t.Fatal(PackCache(run, "rust"))
	}
}

func TestPackArgv(t *testing.T) {
	run := "/r"
	p := pack.Pack{ID: "swift", CacheArgv: map[string][]string{
		"./gradlew":   {"--project-cache-dir", "{run}/.gradle"},
		"swift":       {"--generic"},
		"swift build": {"--scratch-path", "{cache}"},
		"swift test":  {"--scratch-path", "{tmp}"},
	}}
	for _, c := range []struct{ in, want []string }{
		{[]string{"./gradlew", "test"}, []string{"./gradlew", "--project-cache-dir", "/r/.gradle", "test"}},
		{[]string{"gradlew.bat", "build"}, []string{"gradlew.bat", "--project-cache-dir", "/r/.gradle", "build"}},
		{[]string{"/usr/bin/swift", "build", "-c", "release"}, []string{"/usr/bin/swift", "build", "--scratch-path", PackCache(run, "swift"), "-c", "release"}},
		{[]string{"swift", "test"}, []string{"swift", "test", "--scratch-path", PackTmp(run, "swift")}},
		{[]string{"swift", "package", "resolve"}, []string{"swift", "--generic", "package", "resolve"}},
		{[]string{"swift"}, []string{"swift", "--generic"}},
		{[]string{"java", "-version"}, []string{"java", "-version"}},
	} {
		if got := PackArgv(run, p, c.in); !slices.Equal(got, c.want) {
			t.Errorf("%v: %v", c.in, got)
		}
	}
	none := pack.Pack{ID: "x", CacheArgv: map[string][]string{"swift build": {"--s"}}}
	in := []string{"swift", "test"}
	got := PackArgv(run, none, in)
	if !slices.Equal(got, in) {
		t.Fatal(got)
	}
	got[0] = "x"
	if in[0] != "swift" {
		t.Fatal("PackArgv must copy")
	}
	if got := PackArgv(run, p, nil); len(got) != 0 {
		t.Fatal(got)
	}
}

func TestPickPack(t *testing.T) {
	goP := pack.Pack{ID: "go", Checks: []pack.Check{{Name: "build", Argv: []string{"go", "build"}, Tool: "go"}}}
	py := pack.Pack{ID: "python",
		Checks:       []pack.Check{{Name: "lint", Tool: "ruff"}, {Name: "test", Argv: []string{"uv", "run", "pytest"}}},
		TestCommands: []pack.TestCommand{{ArgvPrefix: []string{"pytest"}}},
		PostEdit:     &pack.PostEdit{FormatFile: []string{"black", "{file}"}}}
	kt := pack.Pack{ID: "kotlin", CacheArgv: map[string][]string{"./gradlew": {"--x"}, "": {"--y"}}}
	all := []pack.Pack{goP, py, kt}
	for argv0, want := range map[string]string{
		"go": "go", `C:\Go\bin\go.exe`: "go", "ruff": "python", "uv": "python", "pytest": "python",
		"black": "python", "./gradlew": "kotlin", "python": "python",
	} {
		if p, ok := PickPack(all, "", argv0); !ok || p.ID != want {
			t.Errorf("%s: %v %s", argv0, ok, p.ID)
		}
	}
	if _, ok := PickPack(all, "", "make"); ok {
		t.Fatal("make matched")
	}
	if _, ok := PickPack(all, "", ""); ok {
		t.Fatal("empty matched")
	}
	if p, ok := PickPack(all, "kotlin", "go"); !ok || p.ID != "kotlin" {
		t.Fatal("--pack wins")
	}
	if _, ok := PickPack(all, "rust", "cargo"); ok {
		t.Fatal("unknown id")
	}
	// the only pack (detected at two roots) is the fallback.
	if p, ok := PickPack([]pack.Pack{py, py}, "", "make"); !ok || p.ID != "python" {
		t.Fatal("only pack")
	}
	if got := Unique([]pack.Pack{py, goP, py, {ID: "Bad/ID"}}); len(got) != 2 || got[0].ID != "python" || got[1].ID != "go" {
		t.Fatal(got)
	}
	if !ValidPackID("c-sharp") || ValidPackID("../x") || ValidPackID("") || strings.Contains(GoPack, "/") {
		t.Fatal("ValidPackID")
	}
}
