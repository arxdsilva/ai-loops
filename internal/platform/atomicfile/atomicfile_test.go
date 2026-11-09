package atomicfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f")
	for _, c := range []string{"one", "two-longer"} {
		if err := WriteFile(p, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
		b, _ := os.ReadFile(p)
		if string(b) != c {
			t.Fatal(string(b))
		}
	}
	ents, _ := os.ReadDir(filepath.Dir(p))
	if len(ents) != 1 {
		t.Fatalf("leftovers: %v", ents)
	}
	if err := WriteFile(filepath.Join(p, "x", "y"), nil, 0o644); err == nil {
		t.Fatal("want error")
	}
}
