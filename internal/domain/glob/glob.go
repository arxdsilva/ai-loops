// Package glob matches slash paths against the glob syntax every loops
// component shares (coverage excludes, language-pack source and test globs,
// ownership scopes): "**" spans any number of directories, including none;
// a pattern without "/" matches the base name anywhere. Pure.
package glob

import (
	"path"
	"strings"
)

// Match reports whether the slash path p matches pat. A malformed pattern
// matches nothing.
func Match(pat, p string) bool {
	if !strings.Contains(pat, "/") {
		ok, err := path.Match(pat, path.Base(p))
		return err == nil && ok
	}
	return matchSegs(strings.Split(pat, "/"), strings.Split(p, "/"))
}

// Any reports whether p matches one of pats.
func Any(pats []string, p string) bool {
	for _, pat := range pats {
		if Match(pat, p) {
			return true
		}
	}
	return false
}

// Valid reports whether every segment of pat is a well-formed path.Match
// pattern ("**" is always valid).
func Valid(pat string) bool {
	if pat == "" {
		return false
	}
	for _, seg := range strings.Split(pat, "/") {
		if seg == "**" {
			continue
		}
		if _, err := path.Match(seg, ""); err != nil {
			return false
		}
	}
	return true
}

func matchSegs(pat, segs []string) bool {
	if len(pat) == 0 {
		return len(segs) == 0
	}
	if pat[0] == "**" {
		for i := 0; i <= len(segs); i++ {
			if matchSegs(pat[1:], segs[i:]) {
				return true
			}
		}
		return false
	}
	if len(segs) == 0 {
		return false
	}
	ok, err := path.Match(pat[0], segs[0])
	return err == nil && ok && matchSegs(pat[1:], segs[1:])
}
