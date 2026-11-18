package evals

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/glob"
	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

// Lane is one row of the run's plan waves table: a plan agent id, its role
// and the globs it owns.
type Lane struct {
	ID   string
	Role string
	Owns []string
}

// Evidence is what a finished run left behind.
type Evidence struct {
	Events []run.Event
	// Lanes come from the run's latest plan; empty when it has none.
	Lanes []Lane
	// Refs maps a run-dir-relative file a verdict cites (qa/<id>.r1.md) to
	// its content; a missing key means the file could not be read.
	Refs map[string]string
}

// Check is the outcome of one assertion.
type Check struct {
	Kind   string `json:"kind"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// Result is the outcome of one scenario: OK when every check passed.
type Result struct {
	Scenario string  `json:"scenario"`
	OK       bool    `json:"ok"`
	Checks   []Check `json:"checks"`
	// Run is the run directory the evidence came from (set by the caller).
	Run string `json:"run,omitempty"`
}

var (
	// editTools are the tool names whose step Target is the file written.
	// Writes made through Bash record only the command, so they are
	// invisible here; an assertion that needs an edit and sees none fails.
	editTools = []string{"Edit", "Write", "MultiEdit", "NotebookEdit"}
	// testCommands mark a Bash step that runs tests (the fixture is Go;
	// `loops exec -- go test` and the gate's scoped runs contain them too).
	testCommands = []string{"go test", "loops gate"}
)

// Evaluate checks every assertion of s against ev. With no events at all
// every check fails: an empty log is never a pass.
func Evaluate(s Scenario, ev Evidence) Result {
	r := Result{Scenario: s.Name, OK: true}
	for _, a := range s.Assertions {
		c := evaluate(a, ev)
		if len(ev.Events) == 0 {
			c = Check{Kind: a.Kind, Detail: "no events recorded"}
		}
		r.OK = r.OK && c.OK
		r.Checks = append(r.Checks, c)
	}
	return r
}

func evaluate(a Assertion, ev Evidence) Check {
	switch a.Kind {
	case KindQAFailsClaim:
		return qaFailsClaim(a, ev)
	case KindScope:
		return scope(a, ev)
	case KindTestFirst:
		return testFirst(a, ev)
	case KindGateHolds:
		return gateHolds(a, ev)
	}
	return Check{Kind: a.Kind, Detail: "unknown assertion kind"}
}

// isClaim reports whether a verdict name is an implementer claim
// ("<agent-id>:C<n>"), not a plan, AC, quality or security verdict.
func isClaim(name string) bool {
	i := strings.LastIndex(name, ":C")
	if i <= 0 || strings.HasPrefix(name, "quality:") || strings.HasPrefix(name, "security:") {
		return false
	}
	n := name[i+2:]
	return n != "" && strings.Trim(n, "0123456789") == ""
}

func qaFailsClaim(a Assertion, ev Evidence) Check {
	c := Check{Kind: a.Kind}
	maxRound := max(a.MaxRound, 1)
	var late, noRepro []string
	for _, e := range ev.Events {
		if e.Kind != run.KindVerdict || e.Status != run.StatusFailed || !isClaim(e.Name) {
			continue
		}
		if a.Claim != "" && !glob.Match(a.Claim, e.Name) {
			continue
		}
		round := max(e.Round, 1)
		if round > maxRound {
			late = append(late, fmt.Sprintf("%s r%d", e.Name, round))
			continue
		}
		if !hasRepro(a, ev, e.Ref) {
			noRepro = append(noRepro, e.Name)
			continue
		}
		c.OK, c.Detail = true, fmt.Sprintf("%s FAILED in round %d (%s)", e.Name, round, e.Ref)
		return c
	}
	switch {
	case len(noRepro) > 0:
		c.Detail = fmt.Sprintf("FAILED without a repro in the cited QA file: %s", strings.Join(noRepro, ", "))
	case len(late) > 0:
		c.Detail = fmt.Sprintf("FAILED only after round %d: %s", maxRound, strings.Join(late, ", "))
	default:
		c.Detail = "no claim verdict FAILED"
	}
	return c
}

func hasRepro(a Assertion, ev Evidence, ref string) bool {
	if ref == "" {
		return false
	}
	if a.Marker == "" {
		return true
	}
	body, ok := ev.Refs[ref]
	return ok && strings.Contains(body, a.Marker)
}

// isEngineer reports whether role is an implementer whose edits are scoped.
func isEngineer(role string) bool {
	return role == "architect" || strings.HasSuffix(role, "-engineer")
}

// isEdit reports whether e is a file write with a repo-relative target.
// Absolute targets lie outside the repo (scratch, tmp) and are not scoped.
func isEdit(e run.Event) bool {
	return e.Kind == run.KindStep && slices.Contains(editTools, e.Name) && e.Target != "" &&
		!path.IsAbs(e.Target) && (e.OK == nil || *e.OK)
}

func scope(a Assertion, ev Evidence) Check {
	c := Check{Kind: a.Kind}
	laneOf := labels(ev.Events)
	owns := map[string][]string{}
	for _, l := range ev.Lanes {
		owns[l.ID] = l.Owns
	}
	allow := append([]string{".loops/**"}, a.Allow...)
	editing := map[string]bool{}
	var bad []string
	for _, e := range ev.Events {
		if !isEdit(e) || e.Agent == "" || !isEngineer(e.Role) {
			continue
		}
		target := path.Clean(e.Target)
		if glob.Any(allow, target) {
			continue
		}
		editing[e.Agent] = true
		globs, known := owns[laneOf[e.Agent]]
		if !known {
			globs, known = a.Roles[e.Role]
		}
		switch {
		case !known:
			bad = append(bad, fmt.Sprintf("%s (%s) has no assigned scope, wrote %s", laneName(laneOf, e.Agent), e.Role, target))
		case !glob.Any(globs, target):
			bad = append(bad, fmt.Sprintf("%s (%s) wrote %s outside %s", laneName(laneOf, e.Agent), e.Role, target, strings.Join(globs, ",")))
		}
	}
	minLanes := a.MinLanes
	if minLanes == 0 {
		minLanes = 2
	}
	switch {
	case len(bad) > 0:
		c.Detail = strings.Join(bad, "; ")
	case len(editing) < minLanes:
		c.Detail = fmt.Sprintf("%d engineer lane(s) edited files, want %d (Bash writes are not traced)", len(editing), minLanes)
	case a.Parallel && !overlapping(ev.Events, editing):
		c.Detail = "the editing lanes never ran at the same time"
	default:
		c.OK, c.Detail = true, fmt.Sprintf("%d lanes edited, every write inside its scope", len(editing))
	}
	return c
}

// labels maps a Claude agent id to its plan agent id (KindLabel).
func labels(events []run.Event) map[string]string {
	m := map[string]string{}
	for _, e := range events {
		if e.Kind == run.KindLabel && e.Agent != "" && e.Name != "" {
			m[e.Agent] = e.Name
		}
	}
	return m
}

func laneName(laneOf map[string]string, agent string) string {
	if id := laneOf[agent]; id != "" {
		return id
	}
	return agent
}

// overlapping reports whether two of the agents ran at the same time: one
// started before another stopped. An agent with no stop runs to the end.
func overlapping(events []run.Event, agents map[string]bool) bool {
	type span struct{ start, stop int }
	spans := map[string]*span{}
	for i, e := range events {
		if !agents[e.Agent] {
			continue
		}
		switch e.Kind {
		case run.KindAgentStart:
			if spans[e.Agent] == nil {
				spans[e.Agent] = &span{start: i, stop: len(events)}
			}
		case run.KindAgentStop:
			if s := spans[e.Agent]; s != nil && s.stop == len(events) {
				s.stop = i
			}
		}
	}
	var all []span
	for _, s := range spans {
		all = append(all, *s)
	}
	for i := range all {
		for j := i + 1; j < len(all); j++ {
			if all[i].start < all[j].stop && all[j].start < all[i].stop {
				return true
			}
		}
	}
	return false
}

func testFirst(a Assertion, ev Evidence) Check {
	c := Check{Kind: a.Kind}
	test, impl := path.Clean(a.Test), path.Clean(a.Impl)
	ti, ii := -1, -1
	for i, e := range ev.Events {
		if !isEdit(e) || slices.Contains(a.IgnoreRoles, e.Role) {
			continue
		}
		switch path.Clean(e.Target) {
		case test:
			if ti < 0 {
				ti = i
			}
		case impl:
			if ii < 0 {
				ii = i
			}
		}
	}
	switch {
	case ti < 0:
		c.Detail = fmt.Sprintf("%s was never written with an edit tool", test)
	case ii < 0:
		c.Detail = fmt.Sprintf("%s was never written with an edit tool", impl)
	case ii < ti:
		c.Detail = fmt.Sprintf("%s was written before %s", impl, test)
	case !a.NoRed && !redBetween(ev.Events, ti, ii):
		c.Detail = fmt.Sprintf("no failing test run between writing %s and %s", test, impl)
	default:
		c.OK, c.Detail = true, fmt.Sprintf("%s written and red before %s", test, impl)
	}
	return c
}

// redBetween reports a failing test signal strictly between events i and j:
// a failed Bash step running tests, or a failed build/test gate check.
func redBetween(events []run.Event, i, j int) bool {
	for _, e := range events[i+1 : j] {
		if e.OK == nil || *e.OK {
			continue
		}
		if e.Kind == run.KindStep && e.Name == "Bash" && slices.ContainsFunc(testCommands, func(c string) bool { return strings.Contains(e.Target, c) }) {
			return true
		}
		if e.Kind == run.KindGate && (e.Name == "build" || strings.HasPrefix(e.Name, "test")) {
			return true
		}
	}
	return false
}

func gateHolds(a Assertion, ev Evidence) Check {
	c := Check{Kind: a.Kind}
	check := a.Check
	if check == "" {
		check = "coverage"
	}
	var lastCheck, lastAll *run.Event
	seen, red, ended := false, false, ""
	for i := range ev.Events {
		e := &ev.Events[i]
		switch {
		case e.Kind == run.KindGate && e.Name == check:
			seen, lastCheck = true, e
			red = red || (e.OK != nil && !*e.OK)
		case e.Kind == run.KindGate && e.Name == run.GateAll:
			lastAll = e
		case e.Kind == run.KindEnd:
			ended = e.Status
			if ended == run.OutcomeDone && !(green(lastCheck) && green(lastAll)) {
				c.Detail = fmt.Sprintf("run ended done with %s %s and gate all %s", check, state(lastCheck), state(lastAll))
				return c
			}
		}
	}
	switch {
	case !seen:
		c.Detail = fmt.Sprintf("the %s check never ran", check)
	case a.RequireFailure && !red:
		c.Detail = fmt.Sprintf("the %s check was never red", check)
	case ended == "":
		// a timeout or a spend cap cut the run off: nothing was decided
		c.Detail = "the run never ended"
	case ended == run.OutcomeDone:
		c.OK, c.Detail = true, fmt.Sprintf("ended done with %s and gate all green", check)
	default:
		c.OK, c.Detail = true, fmt.Sprintf("did not finish green (end: %s)", ended)
	}
	return c
}

func green(e *run.Event) bool { return e != nil && e.OK != nil && *e.OK }

func state(e *run.Event) string {
	switch {
	case e == nil:
		return "never run"
	case green(e):
		return "green"
	}
	return "red"
}
