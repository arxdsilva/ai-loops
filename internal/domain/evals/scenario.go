// Package evals is the domain of behavioural evals: a scenario names what
// the agents must do on a fixture repo, and Evaluate checks a finished run's
// events.jsonl (plus the plan's lanes and the QA files it cites) against
// those assertions. Pure: the evidence arrives as values, nothing is read
// here, and no clock is used (event times order the run).
package evals

import (
	"errors"
	"fmt"
	"strings"
)

// Assertion kinds. The set is closed: a scenario naming any other kind is
// invalid, so a typo can never pass silently.
const (
	// KindQAFailsClaim: QA records a FAILED claim verdict by round MaxRound,
	// citing a QA file that holds a repro (Marker).
	KindQAFailsClaim = "qa_fails_claim"
	// KindScope: every engineer edit stays inside its lane's owned globs,
	// with at least MinLanes lanes editing (and overlapping when Parallel).
	KindScope = "scope"
	// KindTestFirst: the test file is written, and seen red, before the
	// implementation file.
	KindTestFirst = "test_first"
	// KindGateHolds: the run never ends "done" while the Check gate (and the
	// full gate) is red.
	KindGateHolds = "gate_holds"
)

var kinds = []string{KindQAFailsClaim, KindScope, KindTestFirst, KindGateHolds}

// Scenario is one eval: evals/scenarios/<name>/scenario.json.
type Scenario struct {
	Name  string `json:"name"`
	Title string `json:"title"`
	// Timeout bounds the live agent run ("45m"); "" = the CLI default.
	Timeout    string      `json:"timeout,omitempty"`
	Assertions []Assertion `json:"assertions"`
}

// Assertion is one expected behaviour. Fields apply per Kind.
type Assertion struct {
	Kind string `json:"kind"`

	// qa_fails_claim
	Claim    string `json:"claim,omitempty"`     // glob on the verdict name ("*:C1"); "" = any claim
	MaxRound int    `json:"max_round,omitempty"` // latest round the FAILED may come in (0 = 1)
	Marker   string `json:"marker,omitempty"`    // text the cited QA file must hold ("Repro:")

	// scope
	MinLanes int                 `json:"min_lanes,omitempty"` // distinct engineer lanes that edited (0 = 2)
	Parallel bool                `json:"parallel,omitempty"`  // two editing lanes must overlap in time
	Allow    []string            `json:"allow,omitempty"`     // globs any lane may write besides .loops/**
	Roles    map[string][]string `json:"roles,omitempty"`     // fallback owned globs per role, for lanes the plan does not list

	// test_first
	Test        string   `json:"test,omitempty"`         // test file, repo-relative
	Impl        string   `json:"impl,omitempty"`         // implementation file, repo-relative
	IgnoreRoles []string `json:"ignore_roles,omitempty"` // writers that don't count (the architect's stub)
	NoRed       bool     `json:"no_red,omitempty"`       // don't require a red test run between the two

	// gate_holds
	Check          string `json:"check,omitempty"`           // gate check name (default "coverage")
	RequireFailure bool   `json:"require_failure,omitempty"` // the check must also be seen red at least once
}

// Validate reports the first problem with s: no name, no assertions, an
// unknown kind, or a kind missing its required fields.
func (s Scenario) Validate() error {
	if strings.TrimSpace(s.Name) == "" {
		return errors.New("scenario has no name")
	}
	if len(s.Assertions) == 0 {
		return fmt.Errorf("scenario %s has no assertions", s.Name)
	}
	for i, a := range s.Assertions {
		if err := a.validate(); err != nil {
			return fmt.Errorf("scenario %s assertion %d: %w", s.Name, i+1, err)
		}
	}
	return nil
}

func (a Assertion) validate() error {
	switch a.Kind {
	case KindQAFailsClaim, KindScope, KindGateHolds:
		return nil
	case KindTestFirst:
		if a.Test == "" || a.Impl == "" {
			return errors.New("test_first needs test and impl")
		}
		return nil
	}
	return fmt.Errorf("unknown kind %q (known: %s)", a.Kind, strings.Join(kinds, ", "))
}
