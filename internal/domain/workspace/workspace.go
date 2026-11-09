// Package workspace is the pure model of a run's scratch space: the
// run-scoped Go cache (GOCACHE/GOTMPDIR under the run directory) and the
// end-of-run cleanup that deletes the build artifacts agents left in the
// project tree.
//
// The cleanup is deliberately conservative. A path is a deletion candidate
// only when it is new since the run started (absent from the baseline
// snapshot), lies inside the project root, is not under .loops/ or .git/, is
// a regular file (never a symlink or a directory) and is an artifact: its
// name matches a Go build/test/profile output pattern, or its first bytes are
// an executable binary header (ELF, Mach-O, PE). Scripts (#!) are never
// artifacts. Everything else new is reported "left for review".
//
// Paths are slash-separated and relative to the project root; the adapter
// owns every filesystem and git call.
package workspace

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// File names inside the run directory.
const (
	baselineName     = "baseline-untracked.txt"
	baselineDirsName = "baseline-dirs.txt"
)

// GoDir is the run's Go scratch directory (<run dir>/go).
func GoDir(runDir string) string { return filepath.Join(runDir, "go") }

// GoCache is the run's GOCACHE (<run dir>/go/cache).
func GoCache(runDir string) string { return filepath.Join(runDir, "go", "cache") }

// GoTmp is the run's GOTMPDIR (<run dir>/go/tmp).
func GoTmp(runDir string) string { return filepath.Join(runDir, "go", "tmp") }

// Baseline is the snapshot of untracked (incl. ignored) files at run start.
func Baseline(runDir string) string { return filepath.Join(runDir, baselineName) }

// BaselineDirs is the snapshot of the project's directories at run start.
func BaselineDirs(runDir string) string { return filepath.Join(runDir, baselineDirsName) }

// Env is the environment that scopes go commands to the run.
func Env(runDir string) []string {
	return []string{"GOCACHE=" + GoCache(runDir), "GOTMPDIR=" + GoTmp(runDir)}
}

// ShellQuote quotes s for a POSIX shell (single quotes).
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Exports renders the statements that set GOCACHE and GOTMPDIR in shell
// ("sh"/"bash"/"zsh"/"" = POSIX, "fish", "powershell"/"pwsh").
func Exports(shell, cache, tmp string) (string, error) {
	switch shell {
	case "", "sh", "bash", "zsh", "dash", "ksh":
		return "export GOCACHE=" + ShellQuote(cache) + " GOTMPDIR=" + ShellQuote(tmp), nil
	case "fish":
		q := func(s string) string {
			return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
		}
		return "set -gx GOCACHE " + q(cache) + "; set -gx GOTMPDIR " + q(tmp), nil
	case "powershell", "pwsh":
		q := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
		return "$env:GOCACHE=" + q(cache) + "; $env:GOTMPDIR=" + q(tmp), nil
	}
	return "", fmt.Errorf("unknown shell %q (want sh, fish or powershell)", shell)
}

// loopsPath reports whether rel is .loops or under it.
func loopsPath(rel string) bool { return rel == ".loops" || strings.HasPrefix(rel, ".loops/") }

// SafeRel reports whether rel is a clean, relative, slash path inside the
// root that cleanup may touch: no "..", no control characters or
// backslashes, not under .loops/ and not inside any .git directory.
func SafeRel(rel string) bool {
	if rel == "" || rel == "." || strings.HasPrefix(rel, "/") || path.Clean(rel) != rel || loopsPath(rel) {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return false
	}
	for i := 0; i < len(rel); i++ {
		if c := rel[i]; c < 0x20 || c == 0x7f || c == '\\' {
			return false
		}
	}
	for _, part := range strings.Split(rel, "/") {
		if part == ".git" {
			return false
		}
	}
	return true
}

// NewPaths returns the current paths that are not in the baseline (sorted,
// deduplicated), ignoring .loops/. Paths that fail SafeRel are returned
// separately as unsafe and must never be touched. A trailing "/" (a nested
// repository git reports as a directory) is kept on the returned path.
func NewPaths(baseline, current []string) (fresh, unsafe []string) {
	seen := map[string]bool{}
	for _, b := range baseline {
		seen[b] = true
	}
	for _, c := range current {
		if seen[c] {
			continue
		}
		seen[c] = true
		trimmed := strings.TrimSuffix(c, "/")
		switch {
		case loopsPath(trimmed):
		case !SafeRel(trimmed):
			unsafe = append(unsafe, c)
		default:
			fresh = append(fresh, c)
		}
	}
	sort.Strings(fresh)
	sort.Strings(unsafe)
	return fresh, unsafe
}

// artifactSuffixes are lower-case name endings of Go build/test/profile outputs.
var artifactSuffixes = []string{".test", ".test.exe", ".out", ".prof", ".pprof", ".cov", ".exe"}

// IsArtifactName reports whether a file's base name is a Go build, test or
// profiling output: *.test, *.test.exe, *.out, *.prof, *.pprof, *.cov,
// coverage*.txt, cover*.out, __debug_bin*, *.exe.
func IsArtifactName(base string) bool {
	n := strings.ToLower(base)
	if strings.HasPrefix(n, "__debug_bin") {
		return true
	}
	if strings.HasPrefix(n, "coverage") && strings.HasSuffix(n, ".txt") {
		return true
	}
	for _, s := range artifactSuffixes {
		if strings.HasSuffix(n, s) && len(n) > len(s) {
			return true
		}
	}
	return false
}

// HeadSize is how many leading bytes IsExecutableBinary needs to judge a file.
const HeadSize = 4096

// IsExecutableBinary reports whether head (a file's first bytes) starts an
// ELF, Mach-O (thin or fat) or PE executable. Java class files share the
// fat Mach-O magic and are told apart by the architecture count.
func IsExecutableBinary(head []byte) bool {
	if len(head) < 4 {
		return false
	}
	switch {
	case bytes.HasPrefix(head, []byte("\x7fELF")):
		return true
	case bytes.HasPrefix(head, []byte{0xfe, 0xed, 0xfa, 0xce}), bytes.HasPrefix(head, []byte{0xfe, 0xed, 0xfa, 0xcf}),
		bytes.HasPrefix(head, []byte{0xce, 0xfa, 0xed, 0xfe}), bytes.HasPrefix(head, []byte{0xcf, 0xfa, 0xed, 0xfe}):
		return true
	case bytes.HasPrefix(head, []byte{0xca, 0xfe, 0xba, 0xbe}):
		// fat Mach-O: big-endian arch count (small); Java: minor/major version (major >= 45).
		return len(head) >= 8 && binary.BigEndian.Uint32(head[4:8]) < 45
	case bytes.HasPrefix(head, []byte("MZ")):
		if len(head) < 0x40 {
			return false
		}
		off := int(binary.LittleEndian.Uint32(head[0x3c:0x40]))
		return off >= 0x40 && off+4 <= len(head) && bytes.Equal(head[off:off+4], []byte("PE\x00\x00"))
	}
	return false
}

// EntryKind is what a path is on disk (never following symlinks).
type EntryKind int

// Entry kinds.
const (
	KindOther   EntryKind = iota // fifo, socket, device, vanished
	KindFile                     // regular file
	KindSymlink                  // symbolic link (never followed, never deleted)
	KindDir                      // directory
)

// Entry is one new path as the adapter found it.
type Entry struct {
	Rel  string
	Kind EntryKind
	Size int64
	Head []byte // first HeadSize bytes of a regular file
}

// Action is what cleanup does with an entry.
type Action int

// Actions.
const (
	Keep   Action = iota // leave it, list it for review
	Remove               // delete it
)

// Decide returns Remove only for a safe, regular, non-script file that is
// an artifact by name or by executable header.
func Decide(e Entry) Action {
	if e.Kind != KindFile || !SafeRel(e.Rel) || bytes.HasPrefix(e.Head, []byte("#!")) {
		return Keep
	}
	if IsArtifactName(path.Base(e.Rel)) || IsExecutableBinary(e.Head) {
		return Remove
	}
	return Keep
}

// skipNames are directory names whose trees are never snapshotted or pruned.
var skipNames = map[string]bool{".git": true, "vendor": true, "node_modules": true}

// SkipDir reports whether the directory rel is outside the snapshot:
// .git, vendor and node_modules anywhere, and the root .loops.
func SkipDir(rel string) bool {
	return rel == ".loops" || skipNames[path.Base(rel)]
}

// PruneDirs returns the parent directories of the removed files that did not
// exist at run start (absent from baseDirs) and are outside skipped trees,
// deepest first. The adapter removes each only if it is empty.
func PruneDirs(removed []string, baseDirs map[string]bool) []string {
	set := map[string]bool{}
	for _, f := range removed {
		if skipped(f) {
			continue
		}
		for d := path.Dir(f); d != "." && d != "/" && !baseDirs[d]; d = path.Dir(d) {
			set[d] = true
		}
	}
	out := make([]string, 0, len(set))
	for d := range set {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		di, dj := strings.Count(out[i], "/"), strings.Count(out[j], "/")
		if di != dj {
			return di > dj
		}
		return out[i] < out[j]
	})
	return out
}

// skipped reports whether any directory component of rel is a skipped tree.
func skipped(rel string) bool {
	for d := path.Dir(rel); d != "." && d != "/"; d = path.Dir(d) {
		if SkipDir(d) {
			return true
		}
	}
	return false
}

// Item is one removed file.
type Item struct {
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

// Report is the outcome of a cleanup.
type Report struct {
	DryRun      bool     `json:"dry_run,omitempty"`
	CacheFiles  int      `json:"cache_files"`
	CacheBytes  int64    `json:"cache_bytes"`
	Removed     []Item   `json:"removed"`
	DirsRemoved []string `json:"dirs_removed,omitempty"`
	Kept        []string `json:"kept"`
	Refused     []string `json:"refused,omitempty"`
	// NoBaseline is set when the run has no start snapshot (a run started
	// by an older loops): only the Go cache is cleaned.
	NoBaseline bool `json:"no_baseline,omitempty"`
}

// Files is the number of files removed (Go cache plus project artifacts).
func (r Report) Files() int { return r.CacheFiles + len(r.Removed) }

// Bytes is the number of bytes freed.
func (r Report) Bytes() int64 {
	n := r.CacheBytes
	for _, it := range r.Removed {
		n += it.Bytes
	}
	return n
}

func mb(n int64) string { return fmt.Sprintf("%.1f MB", float64(n)/(1<<20)) }

// Note is the one-line summary recorded on the run's cleanup event.
func (r Report) Note() string {
	return fmt.Sprintf("%d files, %s freed; %d kept for review", r.Files(), mb(r.Bytes()), len(r.Kept))
}

// Text renders the human summary.
func (r Report) Text() string {
	var b strings.Builder
	verb := "removed"
	if r.DryRun {
		verb = "would remove"
		fmt.Fprintf(&b, "cleanup (dry run): would remove %d files, %s; %d kept for review\n", r.Files(), mb(r.Bytes()), len(r.Kept))
	} else {
		fmt.Fprintf(&b, "cleanup: %s\n", r.Note())
	}
	fmt.Fprintf(&b, "  go cache: %d files, %s\n", r.CacheFiles, mb(r.CacheBytes))
	for _, it := range r.Removed {
		fmt.Fprintf(&b, "  %s %s (%s)\n", verb, it.Path, mb(it.Bytes))
	}
	for _, d := range r.DirsRemoved {
		fmt.Fprintf(&b, "  %s dir %s\n", verb, d)
	}
	for _, k := range r.Kept {
		fmt.Fprintf(&b, "  left for review: %s\n", k)
	}
	for _, x := range r.Refused {
		fmt.Fprintf(&b, "  refused: %s\n", x)
	}
	if r.NoBaseline {
		b.WriteString("  no baseline snapshot for this run: project files were not scanned\n")
	}
	return b.String()
}
