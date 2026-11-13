package coverage

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// generatedRe matches the standard generated-file marker line.
var generatedRe = regexp.MustCompile(`^// Code generated .* DO NOT EDIT\.$`)

// LineRange is an inclusive range of 1-based lines.
type LineRange struct{ Start, End int }

// FileChange is what changed in one file: the new-side line ranges, and
// whether the file is generated (the adapter detects it).
type FileChange struct {
	Ranges    []LineRange
	Generated bool
	// Added is true for a file the diff adds (--- /dev/null); untracked
	// files are marked by the caller with WholeFile ranges.
	Added bool
}

// IsNew reports whether the whole file is new: added in the diff, or
// untracked (WholeFile).
func (fc FileChange) IsNew() bool {
	return fc.Added || (len(fc.Ranges) == 1 && fc.Ranges[0] == WholeFile()[0])
}

// Changes maps module-relative slash paths to their changes.
type Changes map[string]FileChange

// WholeFile is the range that covers every line of a file (new, untracked
// files count as fully changed).
func WholeFile() []LineRange { return []LineRange{{1, math.MaxInt32}} }

// ParseDiff parses unified `git diff -U0` output (with the default a/ and b/
// prefixes) into new-side changed line ranges per file. Deleted files are
// dropped; renamed files appear under their new name (with no ranges when
// the content did not change); binary files are ignored.
func ParseDiff(text string) (Changes, error) {
	out := Changes{}
	cur := ""
	added := false
	oldLeft, newLeft := 0, 0
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if oldLeft > 0 || newLeft > 0 { // hunk body: never a header
			switch {
			case strings.HasPrefix(line, "-"):
				oldLeft--
			case strings.HasPrefix(line, "+"):
				newLeft--
			case strings.HasPrefix(line, " "), line == "":
				oldLeft--
				newLeft--
			}
			continue
		}
		switch {
		case strings.HasPrefix(line, "diff --git "):
			cur, added = "", false
		case strings.HasPrefix(line, "--- "):
			added = strings.TrimRight(strings.TrimPrefix(line, "--- "), "\t") == "/dev/null"
		case strings.HasPrefix(line, "rename to "):
			name, err := unquote(strings.TrimPrefix(line, "rename to "))
			if err != nil {
				return nil, fmt.Errorf("diff line %d: %w", i+1, err)
			}
			cur = name
			out[cur] = FileChange{}
		case strings.HasPrefix(line, "+++ "):
			name, err := diffPath(strings.TrimPrefix(line, "+++ "))
			if err != nil {
				return nil, fmt.Errorf("diff line %d: %w", i+1, err)
			}
			cur = name
			if cur != "" {
				fc := out[cur]
				fc.Added = fc.Added || added
				out[cur] = fc
			}
		case strings.HasPrefix(line, "@@ "):
			r, oldN, newN, err := hunk(line)
			if err != nil {
				return nil, fmt.Errorf("diff line %d: %w", i+1, err)
			}
			oldLeft, newLeft = oldN, newN
			if cur != "" && r.End >= r.Start {
				fc := out[cur]
				fc.Ranges = append(fc.Ranges, r)
				out[cur] = fc
			}
		}
	}
	return out, nil
}

// diffPath returns the path of a "+++ " line without its b/ prefix; "" for
// /dev/null.
func diffPath(s string) (string, error) {
	s, err := unquote(strings.TrimRight(s, "\t"))
	if err != nil {
		return "", err
	}
	if s == "/dev/null" {
		return "", nil
	}
	return strings.TrimPrefix(s, "b/"), nil
}

// unquote undoes git's C-style quoting of unusual path names.
func unquote(s string) (string, error) {
	if !strings.HasPrefix(s, "\"") {
		return s, nil
	}
	u, err := strconv.Unquote(s)
	if err != nil {
		return "", fmt.Errorf("bad quoted path %s", s)
	}
	return u, nil
}

// hunk parses "@@ -a[,b] +c[,d] @@ …" into the new-side range and the
// old/new line counts of its body.
func hunk(line string) (LineRange, int, int, error) {
	f := strings.Fields(line)
	if len(f) < 4 || f[3] != "@@" || !strings.HasPrefix(f[1], "-") || !strings.HasPrefix(f[2], "+") {
		return LineRange{}, 0, 0, fmt.Errorf("bad hunk header %q", line)
	}
	_, oldN, err := span(f[1][1:])
	if err != nil {
		return LineRange{}, 0, 0, err
	}
	start, newN, err := span(f[2][1:])
	if err != nil {
		return LineRange{}, 0, 0, err
	}
	return LineRange{start, start + newN - 1}, oldN, newN, nil
}

// span parses "a" or "a,b" (b defaults to 1).
func span(s string) (int, int, error) {
	a, b, hasB := strings.Cut(s, ",")
	start, err := strconv.Atoi(a)
	if err != nil {
		return 0, 0, fmt.Errorf("bad hunk range %q", s)
	}
	n := 1
	if hasB {
		if n, err = strconv.Atoi(b); err != nil {
			return 0, 0, fmt.Errorf("bad hunk range %q", s)
		}
	}
	return start, n, nil
}

// IsGenerated reports whether a Go file's head (its first lines) carries the
// standard "// Code generated … DO NOT EDIT." marker before the package
// clause.
func IsGenerated(head string) bool {
	for _, l := range strings.Split(head, "\n") {
		l = strings.TrimSuffix(l, "\r")
		if strings.HasPrefix(l, "package ") {
			return false
		}
		if generatedRe.MatchString(l) {
			return true
		}
	}
	return false
}
