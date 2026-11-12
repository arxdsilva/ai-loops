package loopsdir

import (
	"os"
	"os/exec"
	"path/filepath"
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

// git must not list anything under .loops, the .gitignore included.
func TestGitSeesNothing(t *testing.T) {
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
	if err != nil || strings.TrimSpace(string(out)) != "" {
		t.Fatalf("git sees: %q %v", out, err)
	}
}
