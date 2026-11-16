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
	"strings"

	"github.com/arxdsilva/ai-loops/internal/platform/atomicfile"
)

// Name is the directory loops keeps in a project root.
const Name = ".loops"

// Ignore is the content of .loops/.gitignore: everything, itself included.
const Ignore = "# Created by loops: run data, caches and local settings stay out of git.\n" +
	"# To share the project settings, run: git add -f .loops/config.json\n" +
	"*\n"

// The opt-in file store (WORKFLOW §7): .loops/memory/ is re-included after the
// catch-all `*`. Git needs both lines: the directory, then its tree.
const (
	MemoryDir        = "memory"
	IgnoreMemoryDir  = "!memory/"
	IgnoreMemoryTree = "!memory/**"
)

// MemoryInit reports what EnsureMemory changed.
type MemoryInit struct {
	CreatedDir bool     // .loops/memory/ did not exist
	Added      []string // lines appended to .loops/.gitignore, in order
}

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
	content := Ignore
	// A fresh clone of a project that shares .loops/memory/ has the folder
	// but no .gitignore: keep the folder visible to git.
	if fi, err := os.Stat(filepath.Join(dir, MemoryDir)); err == nil && fi.IsDir() {
		content += IgnoreMemoryDir + "\n" + IgnoreMemoryTree + "\n"
	}
	return atomicfile.WriteFile(p, []byte(content), 0o644)
}

// EnsureMemory turns the file store on: it creates <root>/.loops/memory/ and
// re-includes it in .loops/.gitignore. It is idempotent and creates nothing
// else.
func EnsureMemory(root string) (MemoryInit, error) {
	var res MemoryInit
	_, statErr := os.Lstat(filepath.Join(root, Name, ".gitignore"))
	ignoreNew := errors.Is(statErr, fs.ErrNotExist)
	dir, err := Ensure(root)
	if err != nil {
		return res, err
	}
	mem := filepath.Join(dir, MemoryDir)
	if _, err := os.Stat(mem); errors.Is(err, fs.ErrNotExist) {
		res.CreatedDir = true
	}
	if err := os.MkdirAll(mem, 0o755); err != nil {
		return res, err
	}
	p := filepath.Join(dir, ".gitignore")
	b, err := os.ReadFile(p)
	if err != nil {
		return res, err
	}
	content := string(b)
	missing := missingMemoryLines(content)
	if len(missing) == 0 {
		if ignoreNew {
			// Ensure wrote the file (clone case): report what it holds.
			res.Added = []string{IgnoreMemoryDir, IgnoreMemoryTree}
		}
		return res, nil
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += strings.Join(missing, "\n") + "\n"
	if err := atomicfile.WriteFile(p, []byte(content), 0o644); err != nil {
		return res, err
	}
	res.Added = missing
	return res, nil
}

// missingMemoryLines returns the memory re-include lines that git would not
// honour yet: a line counts only when it follows the last `*` line.
func missingMemoryLines(content string) []string {
	lines := strings.Split(content, "\n")
	last := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "*" {
			last = i
		}
	}
	have := map[string]bool{}
	for _, l := range lines[last+1:] {
		have[strings.TrimSpace(l)] = true
	}
	var missing []string
	for _, want := range []string{IgnoreMemoryDir, IgnoreMemoryTree} {
		if !have[want] {
			missing = append(missing, want)
		}
	}
	return missing
}
