// Package versionfile reads and rewrites the "version" of
// .claude-plugin/plugin.json in place, leaving everything else (key order,
// indentation, escapes) untouched.
package versionfile

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
)

// The top-level version is the first "version" key at two-space indent.
var versionRe = regexp.MustCompile(`(?m)^(  "version"\s*:\s*")([^"\\]*)(")`)

// versionFile is a plugin.json on disk.
type versionFile struct{ path string }

// New returns the version file at path.
func New(path string) *versionFile { return &versionFile{path: path} }

// ReadVersion returns the "version" value.
func (f *versionFile) ReadVersion() (string, error) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		return "", err
	}
	var m struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return "", fmt.Errorf("%s: %w", f.path, err)
	}
	if m.Version == "" {
		return "", fmt.Errorf("%s: no version", f.path)
	}
	return m.Version, nil
}

// WriteVersion replaces the "version" value and validates the result.
func (f *versionFile) WriteVersion(v string) error {
	if !regexp.MustCompile(`^[0-9A-Za-z.+-]+$`).MatchString(v) {
		return fmt.Errorf("versionfile: invalid version %q", v)
	}
	data, err := os.ReadFile(f.path)
	if err != nil {
		return err
	}
	if !versionRe.Match(data) {
		return fmt.Errorf("%s: no top-level \"version\" key found", f.path)
	}
	done := false
	out := versionRe.ReplaceAllFunc(data, func(m []byte) []byte {
		if done {
			return m
		}
		done = true
		sub := versionRe.FindSubmatch(m)
		return append(append(append([]byte{}, sub[1]...), v...), sub[3]...)
	})
	if !json.Valid(out) {
		return fmt.Errorf("%s: result is not valid JSON", f.path)
	}
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, f.path)
}
