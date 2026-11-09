// Package release decides the next version from the commits since the last
// release. Pure: git and the version file are adapters.
package release

import (
	"fmt"
	"strconv"
	"strings"
)

// Levels are the accepted bump levels.
const (
	Auto  = "auto"
	Patch = "patch"
	Minor = "minor"
	Major = "major"
)

// Bump returns the next version for current and the level actually applied.
// level is auto, patch, minor or major; commits are "subject\n\nbody" texts.
//
// auto picks major when any commit is breaking (a "BREAKING CHANGE" in its
// body, or "!:" in a conventional subject such as "feat!: x"), else minor
// when any subject is a feat, else patch.
//
// 0.x convention: while the major version is 0, an auto-detected breaking
// change bumps minor instead of major (leaving 0.x is a deliberate act: pass
// "major" explicitly).
func Bump(current, level string, commits []string) (next, chosen string, err error) {
	v, ok := parseCore(current)
	if !ok {
		return "", "", fmt.Errorf("release: invalid version %q", current)
	}
	switch level {
	case Auto:
		s := Summarize(commits)
		switch {
		case s.Breaking > 0 && v[0] > 0:
			chosen = Major
		case s.Breaking > 0 || s.Feat > 0:
			chosen = Minor
		default:
			chosen = Patch
		}
	case Patch, Minor, Major:
		chosen = level
	default:
		return "", "", fmt.Errorf("release: unknown level %q (want auto, patch, minor or major)", level)
	}
	switch chosen {
	case Major:
		v = [3]int{v[0] + 1, 0, 0}
	case Minor:
		v = [3]int{v[0], v[1] + 1, 0}
	default:
		v[2]++
	}
	return fmt.Sprintf("%d.%d.%d", v[0], v[1], v[2]), chosen, nil
}

func parseCore(s string) ([3]int, bool) {
	var v [3]int
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	s, _, _ = strings.Cut(s, "+")
	s, _, _ = strings.Cut(s, "-")
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

// Summary counts commits by kind.
type Summary struct{ Feat, Fix, Breaking, Other int }

// Summarize classifies conventional-commit messages. A breaking commit is
// counted under Breaking only.
func Summarize(commits []string) Summary {
	var s Summary
	for _, c := range commits {
		subject, body, _ := strings.Cut(c, "\n")
		typ, bang := conventional(subject)
		switch {
		case bang || strings.Contains(body, "BREAKING CHANGE"):
			s.Breaking++
		case typ == "feat":
			s.Feat++
		case typ == "fix":
			s.Fix++
		default:
			s.Other++
		}
	}
	return s
}

// conventional parses "type(scope)!: description".
func conventional(subject string) (typ string, bang bool) {
	head, _, ok := strings.Cut(subject, ":")
	if !ok {
		return "", false
	}
	if strings.HasSuffix(head, "!") {
		bang, head = true, strings.TrimSuffix(head, "!")
	}
	if i := strings.IndexByte(head, '('); i >= 0 {
		if !strings.HasSuffix(head, ")") {
			return "", false
		}
		head = head[:i]
	}
	if head == "" || strings.ContainsAny(head, " \t") {
		return "", false
	}
	return head, bang
}

// String renders "3 feat, 2 fix, 1 breaking, 1 other" (zero kinds omitted).
func (s Summary) String() string {
	var parts []string
	for _, p := range []struct {
		n    int
		name string
	}{{s.Feat, "feat"}, {s.Fix, "fix"}, {s.Breaking, "breaking"}, {s.Other, "other"}} {
		if p.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", p.n, p.name))
		}
	}
	if len(parts) == 0 {
		return "no commits"
	}
	return strings.Join(parts, ", ")
}
