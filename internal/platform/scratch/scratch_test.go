package scratch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateDirRemove(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	runDir := filepath.Join(t.TempDir(), "20261112-demo")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := Dir(runDir); got != runDir {
		t.Fatalf("no pointer: %q", got)
	}
	dir, err := Create(runDir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(dir) != Base() || !strings.HasPrefix(filepath.Base(dir), "20261112-demo-") || !Owned(runDir, dir) {
		t.Fatalf("dir %q base %q", dir, Base())
	}
	if strings.HasPrefix(dir, filepath.Dir(runDir)) {
		t.Fatalf("scratch inside the project: %q", dir)
	}
	if fi, err := os.Stat(dir); err != nil || fi.Mode().Perm() != 0o700 {
		t.Fatalf("not private: %v %v", fi, err)
	}
	if got := Dir(runDir); got != dir {
		t.Fatalf("Dir = %q, want %q", got, dir)
	}
	if err := os.WriteFile(filepath.Join(dir, "x"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Remove(runDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("not removed: %v", err)
	}
}

func TestDirIgnoresBadPointer(t *testing.T) {
	runDir := t.TempDir()
	for _, content := range []string{"", "relative/path\n", "  \n"} {
		if err := os.WriteFile(filepath.Join(runDir, pointerName), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := Dir(runDir); got != runDir {
			t.Fatalf("%q: Dir = %q", content, got)
		}
	}
}

// Remove never deletes a folder Create did not make for this run.
func TestRemoveRefusesForeignDir(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	runDir := filepath.Join(t.TempDir(), "r1")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Remove(runDir); err != nil {
		t.Fatalf("no scratch: %v", err)
	}
	victim := t.TempDir()
	if err := os.WriteFile(filepath.Join(runDir, pointerName), []byte(victim+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Remove(runDir); err == nil {
		t.Fatal("removed a foreign directory")
	}
	if _, err := os.Stat(victim); err != nil {
		t.Fatalf("victim gone: %v", err)
	}
}

func TestCreateRefusesSharedBase(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	if err := os.MkdirAll(Base(), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(Base(), 0o777); err != nil {
		t.Fatal(err)
	}
	runDir := t.TempDir()
	if dir, err := Create(runDir); err == nil || dir != runDir {
		t.Fatalf("shared base accepted: %q %v", dir, err)
	}
	if got := Dir(runDir); got != runDir {
		t.Fatalf("pointer written on failure: %q", got)
	}
}
