package loopsdir

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestEnsureCreatesDirAndIgnore(t *testing.T) {
	root := t.TempDir()
	dir, err := Ensure(root)
	if err != nil || dir != filepath.Join(root, ".loops") {
		t.Fatalf("%q %v", dir, err)
	}
	b, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil || string(b) != Ignore {
		t.Fatalf("%q %v", b, err)
	}
	// An existing .gitignore is never rewritten.
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Ensure(root); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, ".gitignore")); string(b) != "mine\n" {
		t.Fatalf("rewritten: %q", b)
	}
}

func TestEnsureIgnoreOnlyExistingDir(t *testing.T) {
	root := t.TempDir()
	if err := EnsureIgnore(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".loops")); !os.IsNotExist(err) {
		t.Fatalf("created .loops in a project without it: %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, ".loops"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := EnsureIgnore(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".loops", ".gitignore")); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureErrors(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".loops"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Ensure(root); err == nil {
		t.Fatal(".loops as a file accepted")
	}
}

// git lists nothing under .loops but config.json (the project's settings,
// committed by the team): not run data, not the .gitignore itself.
func TestGitSeesOnlyConfig(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	root := t.TempDir()
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	dir, err := Ensure(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "runs", "r1"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"config.json", "current", "runs/r1/events.jsonl"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, err := exec.Command("git", "-C", root, "status", "--porcelain", "--untracked-files=all").CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "?? .loops/config.json" {
		t.Fatalf("git sees: %q %v", out, err)
	}
}

func readIgnore(t *testing.T, root string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, ".loops", ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestEnsureMemoryFresh(t *testing.T) {
	root := t.TempDir()
	res, err := EnsureMemory(root)
	if err != nil || !res.CreatedDir || !reflect.DeepEqual(res.Added, []string{"!memory/", "!memory/**"}) {
		t.Fatalf("%+v %v", res, err)
	}
	if got := readIgnore(t, root); got != Ignore+"!memory/\n!memory/**\n" {
		t.Fatalf("%q", got)
	}
	var seen []string
	_ = filepath.WalkDir(filepath.Join(root, ".loops"), func(p string, d os.DirEntry, err error) error {
		rel, _ := filepath.Rel(root, p)
		seen = append(seen, rel)
		return nil
	})
	want := []string{".loops", ".loops/.gitignore", ".loops/memory"}
	if !reflect.DeepEqual(seen, want) {
		t.Fatalf("%v", seen)
	}
	// Idempotent.
	before := readIgnore(t, root)
	res, err = EnsureMemory(root)
	if err != nil || res.CreatedDir || len(res.Added) != 0 || readIgnore(t, root) != before {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestEnsureMemoryOddIgnore(t *testing.T) {
	cases := []struct {
		name, in, out string
		added         []string
	}{
		{"no trailing newline", "mine\n*", "mine\n*\n!memory/\n!memory/**\n", []string{"!memory/", "!memory/**"}},
		{"dir after star", "*\n!memory/\n", "*\n!memory/\n!memory/**\n", []string{"!memory/**"}},
		{"dir only above star", "!memory/\n*\n", "!memory/\n*\n!memory/\n!memory/**\n", []string{"!memory/", "!memory/**"}},
		{"padded lines", "*\n  !memory/  \n!memory/**\n", "*\n  !memory/  \n!memory/**\n", nil},
	}
	for _, c := range cases {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".loops"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".loops", ".gitignore"), []byte(c.in), 0o644); err != nil {
			t.Fatal(err)
		}
		res, err := EnsureMemory(root)
		if err != nil || !reflect.DeepEqual(res.Added, c.added) || readIgnore(t, root) != c.out {
			t.Errorf("%s: %+v %v %q", c.name, res, err, readIgnore(t, root))
		}
	}
}

func TestEnsureMemoryNoIgnoreFile(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".loops"), 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := EnsureMemory(root)
	if err != nil || !res.CreatedDir || len(res.Added) != 2 {
		t.Fatalf("%+v %v", res, err)
	}
	if got := readIgnore(t, root); got != Ignore+"!memory/\n!memory/**\n" {
		t.Fatalf("%q", got)
	}
}

func TestEnsureMemoryErrors(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".loops"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".loops", "memory"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureMemory(root); err == nil {
		t.Fatal("memory as a file accepted")
	}
	root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".loops", ".gitignore"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureMemory(root); err == nil {
		t.Fatal(".gitignore as a directory accepted")
	}
	file := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureMemory(file); err == nil {
		t.Fatal("unusable root accepted")
	}
}

// A clone has .loops/memory/ but no .gitignore: both entry points keep the
// folder visible to git and still write plain Ignore without the folder.
func TestEnsureCloneKeepsMemoryVisible(t *testing.T) {
	for name, fn := range map[string]func(string) error{
		"Ensure":       func(r string) error { _, err := Ensure(r); return err },
		"EnsureIgnore": EnsureIgnore,
	} {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".loops", "memory", "gotchas"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".loops", "memory", "gotchas", "a.md"), []byte("a"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := fn(root); err != nil {
			t.Fatal(err)
		}
		if got := readIgnore(t, root); got != Ignore+"!memory/\n!memory/**\n" {
			t.Fatalf("%s: %q", name, got)
		}
		if _, err := exec.LookPath("git"); err == nil {
			if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
				t.Fatalf("%v %s", err, out)
			}
			if err := exec.Command("git", "-C", root, "check-ignore", "-q", ".loops/memory/gotchas/b.md").Run(); err == nil {
				t.Errorf("%s: memory page ignored", name)
			}
		}
		plain := t.TempDir()
		if err := os.MkdirAll(filepath.Join(plain, ".loops"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := fn(plain); err != nil {
			t.Fatal(err)
		}
		if got := readIgnore(t, plain); got != Ignore {
			t.Fatalf("%s: %q", name, got)
		}
	}
}

func TestEnsureMemoryGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	root := t.TempDir()
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if _, err := EnsureMemory(root); err != nil {
		t.Fatal(err)
	}
	check := func(path string) error {
		return exec.Command("git", "-C", root, "check-ignore", "-q", path).Run()
	}
	if err := check(".loops/memory/gotchas/x.md"); err == nil {
		t.Error("memory page ignored")
	}
	if err := check(".loops/runs/r/events.jsonl"); err != nil {
		t.Errorf("run data not ignored: %v", err)
	}
}

func TestMissingMemoryLines(t *testing.T) {
	both := []string{"!memory/", "!memory/**"}
	cases := []struct {
		in   string
		want []string
	}{
		{"", both},
		{"*\n", both},
		{"*\n!memory/\n!memory/**\n", nil},
		{"!memory/\n!memory/**\n*\n", both},
		{"*\n!memory/**\n", []string{"!memory/"}},
		{"*\n!memory/\n*\n", both},
	}
	for _, c := range cases {
		if got := missingLines(c.in, IgnoreMemoryDir, IgnoreMemoryTree); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q: %v", c.in, got)
		}
	}
}

func TestEnsureConfigFresh(t *testing.T) {
	root := t.TempDir()
	if HasConfig(root) {
		t.Fatal("fresh root has a config")
	}
	res, err := EnsureConfig(root)
	if err != nil || !res.Created || len(res.Added) != 0 {
		t.Fatalf("%+v %v", res, err) // the default Ignore already re-includes it
	}
	b, err := os.ReadFile(filepath.Join(root, ".loops", "config.json"))
	if err != nil || string(b) != InitConfig || !HasConfig(root) {
		t.Fatalf("%q %v", b, err)
	}
	if got := readIgnore(t, root); got != Ignore || !strings.Contains(got, "*\n!config.json\n") {
		t.Fatalf("%q", got)
	}
	res, err = EnsureConfig(root) // idempotent
	if err != nil || res.Created || len(res.Added) != 0 {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestEnsureConfigKeepsExistingAndPatchesOldIgnore(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".loops")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	mine := `{"ask":"plan"}`
	_ = os.WriteFile(filepath.Join(dir, "config.json"), []byte(mine), 0o644)
	_ = os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*\n!memory/\n"), 0o644) // from before loops init
	res, err := EnsureConfig(root)
	if err != nil || res.Created || !reflect.DeepEqual(res.Added, []string{"!config.json"}) {
		t.Fatalf("%+v %v", res, err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "config.json")); string(b) != mine {
		t.Fatalf("existing config rewritten: %q", b)
	}
	if got := readIgnore(t, root); got != "*\n!memory/\n!config.json\n" {
		t.Fatalf("%q", got)
	}
}

func TestEnsureConfigDirIsNotAConfig(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".loops", "config.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if HasConfig(root) {
		t.Fatal("a directory is not a config")
	}
}
