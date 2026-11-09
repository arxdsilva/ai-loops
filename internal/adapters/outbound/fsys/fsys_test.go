package fsys

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLines(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name, content string
		want          int
	}{
		{"empty", "", 0},
		{"terminated", "a\nb\n", 2},
		{"unterminated final line counts", "a\nb", 2},
		{"single unterminated", "a", 1},
		{"blank lines", "\n\n", 2},
		{"large", string(make([]byte, 70000)) + "\nx", 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(dir, "f")
			if err := os.WriteFile(p, []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := New().Lines(p)
			if err != nil || got != tc.want {
				t.Fatalf("got %d, %v want %d", got, err, tc.want)
			}
		})
	}
	if _, err := New().Lines(filepath.Join(dir, "missing")); err == nil {
		t.Error("missing file should error")
	}
}

func TestExistsAndGlob(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.go")
	if err := os.WriteFile(f, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fs := New()
	if !fs.Exists(f) {
		t.Error("file should exist")
	}
	if fs.Exists(dir) || fs.Exists(filepath.Join(dir, "nope")) {
		t.Error("dir and missing are not regular files")
	}
	got, err := fs.Glob(filepath.Join(dir, "*.go"))
	if err != nil || len(got) != 1 || got[0] != f {
		t.Fatalf("glob: %v %v", got, err)
	}
	if _, err := fs.Glob("["); err == nil {
		t.Error("bad pattern should error")
	}
}

func TestGlobSkipsDotfiles(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"a.go", ".hidden", ".dot/x"} {
		p := filepath.Join(dir, n)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fs := New()
	got, _ := fs.Glob(filepath.Join(dir, "*"))
	if len(got) != 1 || got[0] != filepath.Join(dir, "a.go") {
		t.Errorf("star: %v", got)
	}
	got, _ = fs.Glob(filepath.Join(dir, "*", "x"))
	if len(got) != 0 {
		t.Errorf("star dir must skip .dot: %v", got)
	}
	got, _ = fs.Glob(filepath.Join(dir, ".h*"))
	if len(got) != 1 {
		t.Errorf("explicit dot pattern: %v", got)
	}
}
