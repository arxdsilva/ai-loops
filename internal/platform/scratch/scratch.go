// Package scratch places a run's build caches and tmp dirs (each language
// pack's cache_env, e.g. GOCACHE) outside the project, in the user's temp
// directory, so thousands of cache files never land in the repository, its
// editors' indexes or its file watchers. The temp directory is also where
// sandboxed agent shells may write.
//
// The run dir records the scratch path in a file named "scratch", so every
// process of the run (hooks, `loops exec`, the gate) uses the same folder.
// A run without that file (started by an older loops, or when the temp
// directory was unusable) keeps its caches in the run dir, as before.
package scratch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/platform/atomicfile"
)

// pointerName is the file in the run dir that holds the scratch path.
const pointerName = "scratch"

// Dir returns the run's scratch directory: the path recorded in the run
// dir, or runDir itself when none is recorded.
func Dir(runDir string) string {
	b, err := os.ReadFile(filepath.Join(runDir, pointerName))
	if err != nil {
		return runDir
	}
	p := strings.TrimSpace(string(b))
	if p == "" || !filepath.IsAbs(p) {
		return runDir
	}
	return p
}

// Base is the per-user folder that holds every run's scratch directory.
func Base() string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("loops-%d", os.Getuid()))
}

// Create makes a fresh, private scratch directory for the run under Base and
// records it in the run dir. On error nothing is recorded and Dir keeps
// returning runDir.
func Create(runDir string) (string, error) {
	base := Base()
	if err := os.MkdirAll(base, 0o700); err != nil {
		return runDir, err
	}
	// In a shared /tmp, refuse a base someone else could have prepared.
	fi, err := os.Lstat(base)
	if err != nil {
		return runDir, err
	}
	if !fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 || fi.Mode().Perm()&0o077 != 0 {
		return runDir, errors.New("scratch: " + base + " is not a private directory")
	}
	dir, err := os.MkdirTemp(base, filepath.Base(runDir)+"-")
	if err != nil {
		return runDir, err
	}
	if err := atomicfile.WriteFile(filepath.Join(runDir, pointerName), []byte(dir+"\n"), 0o644); err != nil {
		_ = os.Remove(dir)
		return runDir, err
	}
	return dir, nil
}

// Remove deletes the run's scratch directory once its caches are cleaned.
// It removes only a directory Create made: directly under Base and named
// after the run. A run without one is left alone.
func Remove(runDir string) error {
	dir := Dir(runDir)
	if dir == runDir {
		return nil
	}
	if !Owned(runDir, dir) {
		return errors.New("scratch: " + dir + " is not this run's scratch directory")
	}
	return os.RemoveAll(dir)
}

// Owned reports whether dir is a scratch directory Create made for runDir.
func Owned(runDir, dir string) bool {
	return filepath.Dir(filepath.Clean(dir)) == Base() &&
		strings.HasPrefix(filepath.Base(dir), filepath.Base(runDir)+"-")
}
