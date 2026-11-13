package update

import (
	"strings"
	"time"
)

// ApplyRetry is how long a recorded attempt for one version suppresses
// another attempt for the same version (backoff after a failure, pending
// after a success). A documented policy contract (specs/PLATFORMS.md §9),
// not a tunable.
const ApplyRetry = 24 * time.Hour

// Step is what `loops update --apply` should do now.
type Step int

// Steps, in contract order.
const (
	StepNone    Step = iota // up to date, or latest unknown or invalid
	StepManual              // newer release, no automatic route: print it
	StepRun                 // newer release with an automatic route: run it
	StepDone                // this version was installed within ApplyRetry
	StepBackoff             // this version failed within ApplyRetry
)

// ApplyDecision is the outcome of DecideApply. RetryAt is set only for
// StepBackoff.
type ApplyDecision struct {
	Step    Step
	Latest  string
	RetryAt time.Time
}

// DecideApply decides whether to update the installed plugin from current
// to latest. auto reports whether the platform has an automatic route.
//
// Rules, in order: not newer is StepNone; no automatic route is StepManual;
// a recorded attempt for this version younger than ApplyRetry is StepDone
// (it succeeded) or StepBackoff (it failed); otherwise StepRun. An attempt
// recorded in the future (clock skew) does not suppress.
func DecideApply(current, latest string, auto bool, s State, now time.Time) ApplyDecision {
	if !Newer(latest, current) {
		return ApplyDecision{}
	}
	d := ApplyDecision{Latest: latest}
	if !auto {
		d.Step = StepManual
		return d
	}
	d.Step = StepRun
	if Tag(s.Applied) != Tag(latest) || s.AppliedAt.IsZero() || s.AppliedAt.After(now) {
		return d
	}
	if age := now.Sub(s.AppliedAt); age >= ApplyRetry {
		return d
	}
	if s.ApplyErr == "" {
		d.Step = StepDone
		return d
	}
	d.Step = StepBackoff
	d.RetryAt = s.AppliedAt.Add(ApplyRetry)
	return d
}

// OptOut returns why the update check is off, or "" when it is enabled.
// noCheck is LOOPS_NO_UPDATE_CHECK, ci is CI and configOff is
// .loops/config.json "update_check": false; they are checked in that order.
func OptOut(noCheck, ci string, configOff bool) string {
	if noCheck != "" {
		return "LOOPS_NO_UPDATE_CHECK"
	}
	if c := strings.ToLower(ci); c == "true" || c == "1" {
		return "CI"
	}
	if configOff {
		return "update_check: false"
	}
	return ""
}

// IsDev reports whether version is a development build: empty, or a
// pre-release whose identifier ends in "dev" ("0.0.0-dev").
func IsDev(version string) bool {
	v := strings.TrimSpace(version)
	if v == "" {
		return true
	}
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	_, pre, ok := strings.Cut(v, "-")
	return ok && strings.HasSuffix(pre, "dev")
}

// Tag returns v trimmed with a leading "v" ("0.3.0" -> "v0.3.0"); "" stays "".
func Tag(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}
