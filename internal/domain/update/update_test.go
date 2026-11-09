package update

import (
	"testing"
	"time"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"v0.4.0", "0.3.0", true},
		{"0.3.0", "v0.3.0", false},
		{"v0.3.1", "v0.3.0", true},
		{"v1.0.0", "v0.99.99", true},
		{"v0.10.0", "v0.9.0", true},
		{"v0.3.0", "v0.3.0-rc.1", true},
		{"v0.3.0-rc.1", "v0.3.0", false},
		{"v0.3.0-rc.2", "v0.3.0-rc.1", true},
		{"v0.3.0-rc.10", "v0.3.0-rc.2", true},
		{"v0.3.0-rc", "v0.3.0-rc.1", false},
		{"v0.3.0-rc.1", "v0.3.0-rc", true},
		{"v0.3.0-1", "v0.3.0-alpha", false},
		{"v0.3.0-beta", "v0.3.0-alpha", true},
		{"v0.3.0+meta", "v0.3.0", false},
		{"", "v0.1.0", false},
		{"v0.4.0", "", false},
		{"banana", "v0.1.0", false},
		{"v1.2", "v0.1.0", false},
		{"v1.2.x", "v0.1.0", false},
		{"v1.2.3-", "v0.1.0", false},
		{"v1.2.3-a..b", "v0.1.0", false},
	}
	for _, c := range cases {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q,%q)=%v want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestDecide(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		s    State
		want Decision
	}{
		{"zero state fetches", State{}, Decision{Fetch: true}},
		{"recent check no fetch", State{LastCheck: now.Add(-time.Hour)}, Decision{}},
		{"exactly interval fetches", State{LastCheck: now.Add(-Interval)}, Decision{Fetch: true}},
		{"future last check fetches", State{LastCheck: now.Add(time.Hour)}, Decision{Fetch: true}},
		{"newer notifies", State{LastCheck: now, Latest: "v0.4.0"}, Decision{Notify: true, Latest: "v0.4.0"}},
		{"already notified", State{LastCheck: now, Latest: "v0.4.0", LastNotified: "v0.4.0"}, Decision{}},
		{"newer than notified", State{LastCheck: now, Latest: "v0.5.0", LastNotified: "v0.4.0"}, Decision{Notify: true, Latest: "v0.5.0"}},
		{"same version", State{LastCheck: now, Latest: "v0.3.0"}, Decision{}},
		{"older latest", State{LastCheck: now, Latest: "v0.2.0"}, Decision{}},
		{"invalid latest", State{LastCheck: now, Latest: "junk"}, Decision{}},
		{"due and notify", State{LastCheck: now.Add(-48 * time.Hour), Latest: "v0.4.0"}, Decision{Fetch: true, Notify: true, Latest: "v0.4.0"}},
	}
	for _, c := range cases {
		if got := Decide("0.3.0", c.s, now); got != c.want {
			t.Errorf("%s: got %+v want %+v", c.name, got, c.want)
		}
	}
}
