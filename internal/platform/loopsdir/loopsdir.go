// Package loopsdir creates a project's .loops directory so that git ignores
// it: run data, caches, the plugin home and local config are never project
// changes. Like .pytest_cache, the directory carries its own .gitignore, so
// no project file is edited.
package loopsdir

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/arxdsilva/ai-loops/internal/platform/atomicfile"
)

// Name is the directory loops keeps in a project root.
const Name = ".loops"

// Ignore is the content of .loops/.gitignore: everything, itself included.
const Ignore = "# Created by loops: run data, caches and local settings stay out of git.\n" +
	"# To share the project settings, run: git add -f .loops/config.json\n" +
	"*\n"

// Ensure creates <root>/.loops and, when it has none, its .gitignore. It
// never rewrites an existing .gitignore. It returns the directory.
func Ensure(root string) (string, error) {
	dir := filepath.Join(root, Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return dir, err
	}
	return dir, ensureIgnore(dir)
}

// EnsureIgnore adds the .gitignore to an existing <root>/.loops and creates
// nothing when the directory is missing (projects that never used loops).
func EnsureIgnore(root string) error {
	dir := filepath.Join(root, Name)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return nil
	}
	return ensureIgnore(dir)
}

func ensureIgnore(dir string) error {
	p := filepath.Join(dir, ".gitignore")
	_, err := os.Lstat(p)
	if err == nil {
		return nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return atomicfile.WriteFile(p, []byte(Ignore), 0o644)
}
