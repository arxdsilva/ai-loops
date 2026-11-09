// Package projroot finds the project root a loops command works on.
package projroot

import (
	"os"
	"path/filepath"
)

// Find returns $CLAUDE_PROJECT_DIR when set, else the nearest ancestor of
// start holding .loops/ or .git, else start itself.
func Find(start string) string {
	if d := os.Getenv("CLAUDE_PROJECT_DIR"); d != "" {
		return d
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
