// Package fsys implements guard.Files on the OS filesystem.
package fsys

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FS implements guard.Files.
type FS struct{}

// New returns an FS.
func New() *FS { return &FS{} }

// Lines counts lines; a final unterminated line counts.
func (FS) Lines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	buf := make([]byte, 1<<16)
	n, last := 0, byte('\n')
	for {
		k, err := f.Read(buf)
		if k > 0 {
			n += bytes.Count(buf[:k], []byte{'\n'})
			last = buf[k-1]
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
	}
	if last != '\n' {
		n++
	}
	return n, nil
}

// Exists reports whether path is a regular file.
func (FS) Exists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular()
}

// Glob expands pattern like a shell: a wildcard does not match a leading dot
// unless the pattern element itself starts with one.
func (FS) Glob(pattern string) ([]string, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	sep := string(filepath.Separator)
	pelems := strings.Split(pattern, sep)
	var out []string
	for _, m := range matches {
		melems := strings.Split(m, sep)
		keep := len(melems) == len(pelems)
		for i := 0; keep && i < len(melems); i++ {
			if strings.HasPrefix(melems[i], ".") && !strings.HasPrefix(pelems[i], ".") {
				keep = false
			}
		}
		if keep {
			out = append(out, m)
		}
	}
	return out, nil
}
