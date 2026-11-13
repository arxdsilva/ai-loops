// Package update decides when to tell a user that a newer loops release
// exists: at most one network check per Interval, and one notice per new
// version (the first run after a release). Pure; the version source and
// the state file are adapters.
package update

import (
	"strings"
	"time"
)

// Interval is the minimum time between network checks. It is a documented
// policy contract (specs/PLATFORMS.md §9), not a tunable.
const Interval = 24 * time.Hour

// State is persisted between runs (user-level, not per project).
type State struct {
	LastCheck    time.Time `json:"last_check,omitzero"`
	Latest       string    `json:"latest,omitempty"`        // latest version seen, "v0.3.0"
	LastNotified string    `json:"last_notified,omitempty"` // version the user was last told about

	// Apply attempts (`loops update --apply`); additive, omitted when unset.
	Applied   string    `json:"applied,omitempty"`   // version the last automatic update attempted
	AppliedAt time.Time `json:"applied_at,omitzero"` // when that attempt ran
	ApplyErr  string    `json:"apply_err,omitempty"` // one-line failure; "" = the attempt succeeded
}

// Decision is what to do now.
type Decision struct {
	Fetch  bool   // a network check is due
	Notify bool   // tell the user about Latest
	Latest string // the newer version to announce
}

// Decide returns whether to fetch (LastCheck older than Interval) and,
// given the freshest known latest version, whether to notify: latest is a
// valid semver newer than current and not already notified.
func Decide(current string, s State, now time.Time) Decision {
	d := Decision{
		Fetch: s.LastCheck.IsZero() || s.LastCheck.After(now) || now.Sub(s.LastCheck) >= Interval,
	}
	if s.Latest != "" && s.Latest != s.LastNotified && Newer(s.Latest, current) {
		d.Notify, d.Latest = true, s.Latest
	}
	return d
}

// Newer reports whether semver a > b ("v0.3.0" or "0.3.0"; pre-releases
// sort before releases; invalid input is never newer).
func Newer(a, b string) bool {
	va, ok := parse(a)
	if !ok {
		return false
	}
	vb, ok := parse(b)
	if !ok {
		return false
	}
	return compare(va, vb) > 0
}

type semver struct {
	num [3]int
	pre []string
}

func parse(v string) (semver, bool) {
	var r semver
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	core, pre, hasPre := strings.Cut(v, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return r, false
	}
	for i, p := range parts {
		n, ok := atoi(p)
		if !ok {
			return r, false
		}
		r.num[i] = n
	}
	if hasPre {
		if pre == "" {
			return r, false
		}
		r.pre = strings.Split(pre, ".")
		for _, id := range r.pre {
			if id == "" {
				return r, false
			}
		}
	}
	return r, true
}

func atoi(s string) (int, bool) {
	if s == "" || len(s) > 9 {
		return 0, false
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

func compare(a, b semver) int {
	for i := range a.num {
		if a.num[i] != b.num[i] {
			if a.num[i] > b.num[i] {
				return 1
			}
			return -1
		}
	}
	switch {
	case len(a.pre) == 0 && len(b.pre) == 0:
		return 0
	case len(a.pre) == 0:
		return 1
	case len(b.pre) == 0:
		return -1
	}
	for i := 0; i < len(a.pre) && i < len(b.pre); i++ {
		if c := comparePre(a.pre[i], b.pre[i]); c != 0 {
			return c
		}
	}
	switch {
	case len(a.pre) > len(b.pre):
		return 1
	case len(a.pre) < len(b.pre):
		return -1
	}
	return 0
}

func comparePre(a, b string) int {
	na, aNum := atoi(a)
	nb, bNum := atoi(b)
	switch {
	case aNum && bNum:
		if na > nb {
			return 1
		} else if na < nb {
			return -1
		}
		return 0
	case aNum:
		return -1
	case bNum:
		return 1
	}
	return strings.Compare(a, b)
}
