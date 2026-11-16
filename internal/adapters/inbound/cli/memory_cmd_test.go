package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func memSetup(t *testing.T) (*rcEnv, string) {
	t.Helper()
	e := rcSetup(t, "")
	e.app.Root = t.TempDir()
	return e, e.app.Root
}

func TestMemoryInit(t *testing.T) {
	e, root := memSetup(t)
	if code := e.do("memory", "init"); code != 0 {
		t.Fatal(code, e.err)
	}
	lines := strings.Split(strings.TrimRight(e.out.String(), "\n"), "\n")
	if len(lines) != 3 || lines[0] != "created .loops/memory/" ||
		lines[1] != "added to .loops/.gitignore: !memory/ !memory/**" ||
		!strings.HasPrefix(lines[2], "pages under .loops/memory/ are committed with the PR;") ||
		!strings.HasSuffix(lines[2], "delete the folder to leave") {
		t.Fatalf("%q", lines)
	}
	ents, _ := os.ReadDir(filepath.Join(root, ".loops"))
	var names []string
	for _, en := range ents {
		names = append(names, en.Name())
	}
	if len(names) != 2 || names[0] != ".gitignore" || names[1] != "memory" {
		t.Fatalf("%v", names)
	}
	mem, _ := os.ReadDir(filepath.Join(root, ".loops", "memory"))
	if len(mem) != 0 {
		t.Fatal("memory not empty")
	}
	first, _ := os.ReadFile(filepath.Join(root, ".loops", ".gitignore"))
	if code := e.do("memory", "init"); code != 0 {
		t.Fatal(code, e.err)
	}
	out := e.out.String()
	if !strings.HasPrefix(out, "nothing to change: .loops/memory/ is set up\n") || strings.Contains(out, "created") {
		t.Fatalf("%q", out)
	}
	second, _ := os.ReadFile(filepath.Join(root, ".loops", ".gitignore"))
	if string(first) != string(second) {
		t.Fatal("gitignore changed")
	}
}

func TestMemoryInitGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	e, root := memSetup(t)
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if code := e.do("memory", "init"); code != 0 {
		t.Fatal(code, e.err)
	}
	if err := exec.Command("git", "-C", root, "check-ignore", "-q", ".loops/memory/gotchas/x.md").Run(); err == nil {
		t.Error("memory page ignored")
	}
	if err := exec.Command("git", "-C", root, "check-ignore", "-q", ".loops/runs/r/events.jsonl").Run(); err != nil {
		t.Errorf("run data not ignored: %v", err)
	}
}

func TestMemoryUsageAndErrors(t *testing.T) {
	for _, args := range [][]string{{"memory"}, {"memory", "nope"}, {"memory", "init", "extra"}} {
		e, _ := memSetup(t)
		if code := e.do(args...); code != 2 || !strings.Contains(e.err.String(), memoryUsage) {
			t.Errorf("%v: %d %q", args, code, e.err)
		}
	}
	e, root := memSetup(t)
	if err := os.MkdirAll(filepath.Join(root, ".loops"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".loops", "memory"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if code := e.do("memory", "init"); code != 1 || !strings.Contains(e.err.String(), "loops memory init:") {
		t.Fatalf("%d %q", code, e.err)
	}
}

// A clone has .loops/memory/ but no .gitignore: init writes the file and says so.
func TestMemoryInitCloneReportsIgnore(t *testing.T) {
	e, root := memSetup(t)
	if err := os.MkdirAll(filepath.Join(root, ".loops", "memory", "gotchas"), 0o755); err != nil {
		t.Fatal(err)
	}
	if code := e.do("memory", "init"); code != 0 {
		t.Fatal(code, e.err)
	}
	out := e.out.String()
	if strings.Contains(out, "nothing to change") || strings.Contains(out, "created .loops/memory/") ||
		!strings.Contains(out, "added to .loops/.gitignore: !memory/ !memory/**\n") {
		t.Fatalf("%q", out)
	}
	if code := e.do("memory", "init"); code != 0 || !strings.HasPrefix(e.out.String(), "nothing to change") {
		t.Fatalf("%d %q", code, e.out)
	}
}
