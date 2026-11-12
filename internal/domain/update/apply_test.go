package update

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDecideApply(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	failed := func(v string, ago time.Duration) State {
		return State{Applied: v, AppliedAt: now.Add(-ago), ApplyErr: "boom"}
	}
	ok := func(v string, ago time.Duration) State {
		return State{Applied: v, AppliedAt: now.Add(-ago)}
	}
	tests := []struct {
		name    string
		current string
		latest  string
		auto    bool
		s       State
		want    ApplyDecision
	}{
		{"same version", "0.10.0", "v0.10.0", true, State{}, ApplyDecision{}},
		{"older latest", "0.10.0", "v0.9.0", true, State{}, ApplyDecision{}},
		{"empty latest", "0.10.0", "", true, State{}, ApplyDecision{}},
		{"invalid latest", "0.10.0", "nonsense", true, State{}, ApplyDecision{}},
		{"invalid current", "dev", "v0.11.0", true, State{}, ApplyDecision{}},
		{"manual", "0.10.0", "v0.11.0", false, State{}, ApplyDecision{Step: StepManual, Latest: "v0.11.0"}},
		{"manual ignores record", "0.10.0", "v0.11.0", false, ok("v0.11.0", time.Hour), ApplyDecision{Step: StepManual, Latest: "v0.11.0"}},
		{"run fresh", "0.10.0", "v0.11.0", true, State{}, ApplyDecision{Step: StepRun, Latest: "v0.11.0"}},
		{"run zero AppliedAt", "0.10.0", "v0.11.0", true, State{Applied: "v0.11.0", ApplyErr: "x"}, ApplyDecision{Step: StepRun, Latest: "v0.11.0"}},
		{"done", "0.10.0", "v0.11.0", true, ok("v0.11.0", time.Hour), ApplyDecision{Step: StepDone, Latest: "v0.11.0"}},
		{"done v-prefix equality", "0.10.0", "v0.11.0", true, ok("0.11.0", time.Hour), ApplyDecision{Step: StepDone, Latest: "v0.11.0"}},
		{"done just under 24h", "0.10.0", "v0.11.0", true, ok("v0.11.0", ApplyRetry-time.Second), ApplyDecision{Step: StepDone, Latest: "v0.11.0"}},
		{"backoff", "0.10.0", "v0.11.0", true, failed("v0.11.0", time.Hour),
			ApplyDecision{Step: StepBackoff, Latest: "v0.11.0", RetryAt: now.Add(-time.Hour).Add(ApplyRetry)}},
		{"backoff v-prefix equality", "0.10.0", "0.11.0", true, failed("v0.11.0", time.Hour),
			ApplyDecision{Step: StepBackoff, Latest: "0.11.0", RetryAt: now.Add(-time.Hour).Add(ApplyRetry)}},
		{"exactly 24h done becomes run", "0.10.0", "v0.11.0", true, ok("v0.11.0", ApplyRetry), ApplyDecision{Step: StepRun, Latest: "v0.11.0"}},
		{"exactly 24h backoff becomes run", "0.10.0", "v0.11.0", true, failed("v0.11.0", ApplyRetry), ApplyDecision{Step: StepRun, Latest: "v0.11.0"}},
		{"over 24h", "0.10.0", "v0.11.0", true, failed("v0.11.0", 30*time.Hour), ApplyDecision{Step: StepRun, Latest: "v0.11.0"}},
		{"future AppliedAt", "0.10.0", "v0.11.0", true, failed("v0.11.0", -time.Hour), ApplyDecision{Step: StepRun, Latest: "v0.11.0"}},
		{"older recorded version", "0.10.0", "v0.11.0", true, ok("v0.10.5", time.Hour), ApplyDecision{Step: StepRun, Latest: "v0.11.0"}},
		{"newer than recorded failure", "0.10.0", "v0.12.0", true, failed("v0.11.0", time.Minute), ApplyDecision{Step: StepRun, Latest: "v0.12.0"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := DecideApply(tc.current, tc.latest, tc.auto, tc.s, now); got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestOptOut(t *testing.T) {
	tests := []struct {
		name        string
		noCheck, ci string
		configOff   bool
		want        string
	}{
		{"enabled", "", "", false, ""},
		{"no check", "1", "", false, "LOOPS_NO_UPDATE_CHECK"},
		{"no check any value", "whatever", "", false, "LOOPS_NO_UPDATE_CHECK"},
		{"CI true", "", "true", false, "CI"},
		{"CI TRUE", "", "TRUE", false, "CI"},
		{"CI 1", "", "1", false, "CI"},
		{"CI false", "", "false", false, ""},
		{"CI 0", "", "0", false, ""},
		{"config", "", "", true, "update_check: false"},
		{"CI false config", "", "false", true, "update_check: false"},
		{"no check beats CI and config", "1", "true", true, "LOOPS_NO_UPDATE_CHECK"},
		{"CI beats config", "", "1", true, "CI"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := OptOut(tc.noCheck, tc.ci, tc.configOff); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIsDev(t *testing.T) {
	tests := map[string]bool{
		"": true, "  ": true, "0.0.0-dev": true, "1.2.3-dev": true, "v1.2.3-dev": true,
		"1.2.3-dev+abc": true, "0.10.0": false, "v0.10.0": false, "0.11.0-rc.1": false,
		"0.10.0+dev": false,
	}
	for v, want := range tests {
		if got := IsDev(v); got != want {
			t.Errorf("IsDev(%q) = %v, want %v", v, got, want)
		}
	}
}

func TestTag(t *testing.T) {
	tests := map[string]string{
		"": "", "  ": "", "0.3.0": "v0.3.0", "v0.3.0": "v0.3.0", " 0.3.0 ": "v0.3.0",
	}
	for v, want := range tests {
		if got := Tag(v); got != want {
			t.Errorf("Tag(%q) = %q, want %q", v, got, want)
		}
	}
}

func TestStateJSONAdditive(t *testing.T) {
	at := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	old := State{LastCheck: at, Latest: "v0.3.0", LastNotified: "v0.3.0"}
	b, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"last_check":"2026-11-12T10:00:00Z","latest":"v0.3.0","last_notified":"v0.3.0"}`
	if string(b) != want {
		t.Fatalf("got %s, want %s", b, want)
	}

	var got State
	if err := json.Unmarshal([]byte(want), &got); err != nil {
		t.Fatal(err)
	}
	if got.Applied != "" || !got.AppliedAt.IsZero() || got.ApplyErr != "" || got != old {
		t.Fatalf("old document decoded to %+v", got)
	}

	full := State{Applied: "v0.4.0", AppliedAt: at, ApplyErr: "boom"}
	b, _ = json.Marshal(full)
	var back State
	if err := json.Unmarshal(b, &back); err != nil || back != full {
		t.Fatalf("round trip %s -> %+v (%v)", b, back, err)
	}
}
