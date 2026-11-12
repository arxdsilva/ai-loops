package hook_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// TestMain points TMPDIR at a throwaway folder: runs started by these tests
// put their scratch dirs (build caches) there, never in the real temp dir.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "loops-pkg-test-")
	if err != nil {
		panic(err)
	}
	os.Setenv("TMPDIR", dir)
	code := m.Run()
	// go cache entries are read-only; make them removable first.
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			_ = os.Chmod(p, 0o755)
		}
		return nil
	})
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
