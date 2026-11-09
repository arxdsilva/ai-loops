// Package projroot finds the project root a loops command works on.
package projroot

import (
	"os"
	"path/filepath"
)

// projectEnv lists the project-dir variables the supported agents export
// (specs/PLATFORMS.md §5), most specific first.
var projectEnv = [...]string{"CURSOR_PROJECT_DIR", "GEMINI_PROJECT_DIR", "DEVIN_PROJECT_DIR", "CLAUDE_PROJECT_DIR"}

// Find returns the first project-dir variable that is set (Cursor, Gemini,
// Devin, Claude), else the nearest ancestor of start holding .loops/ or
// .git, else start itself.
func Find(start string) string {
	for _, k := range projectEnv {
		if d := os.Getenv(k); d != "" {
			return d
		}
	}
	dir, err := filepath.Abs(start)
	if err != nil {
		return start
	}
	for d := dir; ; {
		for _, marker := range []string{".loops", ".git"} {
			if _, err := os.Stat(filepath.Join(d, marker)); err == nil {
				return d
			}
		}
		parent := filepath.Dir(d)
		if parent == d {
			return dir
		}
		d = parent
	}
}
