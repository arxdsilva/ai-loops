package run

import (
	"reflect"
	"strings"
	"testing"
)

func TestCheckBudget(t *testing.T) {
	lim := Limits{MaxAgents: 4, Phases: map[Phase]PhaseLimit{
		PhaseImplement: {MaxTokens: 2_000_000, MaxAgents: 3, Roles: []string{"domain-engineer", "api-engineer"}},
		PhaseResearch:  {MaxTokens: 200_000},
	}}
	st := State{
		Phase: PhaseImplement,
		Lanes: []Lane{
			{Agent: "r1", Phase: PhaseResearch},
			{Agent: "i1", Phase: PhaseImplement, Running: true},
			{Agent: "i2", Phase: PhaseImplement, Running: true},
		},
		PhaseTokens: map[Phase]int64{PhaseResearch: 250_000, PhaseImplement: 1_000_000},
	}
	spent := st
	spent.PhaseTokens = map[Phase]int64{PhaseImplement: 2_100_000}
	cases := []struct {
		name   string
		s      State
		l      Limits
		phase  Phase
		n      int
		role   string
		limit  string // "" = OK
		reason string // substring
	}{
		{"zero limits", st, Limits{}, PhaseImplement, 99, "anything", "", ""},
		{"fits", st, lim, PhaseImplement, 1, "domain-engineer", "", ""},
		{"role case and prefix", st, lim, PhaseImplement, 1, "loops:API-Engineer", "", ""},
		{"run cap", st, lim, PhaseImplement, 3, "domain-engineer", "max_agents", "2 running + 3 new would exceed the run cap of 4"},
		{"run cap exact fits", st, Limits{MaxAgents: 3}, PhaseQA, 1, "", "", ""},
		{"role not allowed", st, lim, PhaseImplement, 1, "qa-verifier", "roles", "implement allows only domain-engineer, api-engineer (not qa-verifier)"},
		{"role missing", st, lim, PhaseImplement, 1, "", "roles", "pass --role"},
		{"phase agents", st, lim, PhaseImplement, 2, "domain-engineer", "agents", "implement agent budget: 2 opened + 2 new would exceed 3"},
		{"tokens spent", spent, lim, PhaseImplement, 1, "domain-engineer", "tokens", "implement token budget spent: 2.1M of 2M"},
		{"tokens exactly at limit", State{PhaseTokens: map[Phase]int64{PhaseResearch: 200_000}}, lim, PhaseResearch, 1, "", "tokens", "research token budget spent: 200k of 200k"},
		{"other phase unlimited", st, lim, PhaseQA, 1, "qa-verifier", "", ""},
		{"research under budget", State{PhaseTokens: map[Phase]int64{PhaseResearch: 199_999}}, lim, PhaseResearch, 1, "", "", ""},
		{"n below 1 counts as 1", st, Limits{MaxAgents: 2}, PhaseQA, 0, "", "max_agents", "2 running + 1 new"},
	}
	for _, c := range cases {
		d := CheckBudget(c.s, c.l, c.phase, c.n, c.role)
		if (c.limit == "") != d.OK || d.Limit != c.limit {
			t.Errorf("%s: got %+v want limit %q", c.name, d, c.limit)
			continue
		}
		if c.limit == "" && d.Reason != "" {
			t.Errorf("%s: reason on OK: %q", c.name, d.Reason)
		}
		if !strings.Contains(d.Reason, c.reason) {
			t.Errorf("%s: reason %q lacks %q", c.name, d.Reason, c.reason)
		}
	}
}

func TestFormatTokens(t *testing.T) {
	for n, want := range map[int64]string{
		0: "0", 950: "950", 1000: "1k", 1234: "1.2k", 200_000: "200k", 2_000_000: "2M", 2_100_000: "2.1M",
		1_500_000_000: "1.5G", -5: "-5",
	} {
		if got := FormatTokens(n); got != want {
			t.Errorf("FormatTokens(%d) = %q want %q", n, got, want)
		}
	}
}

func TestParseTokens(t *testing.T) {
	for in, want := range map[string]int64{
		"200k": 200_000, "200K": 200_000, "2M": 2_000_000, "2m": 2_000_000, "1.5M": 1_500_000, "3G": 3_000_000_000, "42": 42, " 7k ": 7000,
	} {
		got, err := ParseTokens(in)
		if err != nil || got != want {
			t.Errorf("ParseTokens(%q) = %d, %v want %d", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "k", "abc", "-1k", "2T", "1.2.3M", "0"} {
		if _, err := ParseTokens(bad); err == nil {
			t.Errorf("ParseTokens(%q): want error", bad)
		}
	}
}

func TestParseLimits(t *testing.T) {
	l, err := ParseLimits("research=200k,implement=2M", "implement=12,qa=4", "implement=domain-engineer,api-engineer;qa=qa-verifier", 6)
	if err != nil {
		t.Fatal(err)
	}
	want := Limits{MaxAgents: 6, Phases: map[Phase]PhaseLimit{
		PhaseResearch:  {MaxTokens: 200_000},
		PhaseImplement: {MaxTokens: 2_000_000, MaxAgents: 12, Roles: []string{"domain-engineer", "api-engineer"}},
		PhaseQA:        {MaxAgents: 4, Roles: []string{"qa-verifier"}},
	}}
	if !reflect.DeepEqual(l, want) {
		t.Fatalf("got %+v\nwant %+v", l, want)
	}
	// a comma-separated item with '=' starts a new phase in --roles too.
	l, err = ParseLimits("", "", "implement=a,b,qa=c", 0)
	if err != nil || !reflect.DeepEqual(l.Phases[PhaseImplement].Roles, []string{"a", "b"}) || !reflect.DeepEqual(l.Phases[PhaseQA].Roles, []string{"c"}) {
		t.Fatalf("%+v %v", l, err)
	}
	if l, err := ParseLimits("", "", "", 0); err != nil || !l.IsZero() || l.Phases != nil {
		t.Fatalf("empty: %+v %v", l, err)
	}
	for _, bad := range [][4]string{
		{"bogus=1k", "", "", ""}, {"research", "", "", ""}, {"research=lots", "", "", ""},
		{"", "implement=x", "", ""}, {"", "implement=0", "", ""}, {"", "nope=3", "", ""},
		{"", "", "a,b", ""}, {"", "", "implement=", ""}, {"", "", "zz=a", ""}, {"", "", "", "-1"},
	} {
		n := 0
		if bad[3] == "-1" {
			n = -1
		}
		if _, err := ParseLimits(bad[0], bad[1], bad[2], n); err == nil {
			t.Errorf("ParseLimits(%q): want error", bad)
		}
	}
}

func TestLimitsMerge(t *testing.T) {
	cfg := Limits{MaxAgents: 8, Phases: map[Phase]PhaseLimit{
		PhaseResearch:  {MaxTokens: 100},
		PhaseImplement: {MaxTokens: 1000, MaxAgents: 5, Roles: []string{"x"}},
	}}
	run := Limits{Phases: map[Phase]PhaseLimit{PhaseImplement: {MaxTokens: 2000}, PhaseQA: {MaxAgents: 2}}}
	got := cfg.Merge(run)
	want := Limits{MaxAgents: 8, Phases: map[Phase]PhaseLimit{
		PhaseResearch:  {MaxTokens: 100},
		PhaseImplement: {MaxTokens: 2000, MaxAgents: 5, Roles: []string{"x"}},
		PhaseQA:        {MaxAgents: 2},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	if cfg.Phases[PhaseImplement].MaxTokens != 1000 || len(cfg.Phases) != 2 {
		t.Fatal("Merge mutated the receiver")
	}
	if got := (Limits{}).Merge(Limits{MaxAgents: 3}); got.MaxAgents != 3 || got.Phases != nil {
		t.Fatalf("%+v", got)
	}
	if got := (Limits{MaxAgents: 3}).Merge(Limits{}); !reflect.DeepEqual(got, Limits{MaxAgents: 3}) {
		t.Fatalf("%+v", got)
	}
}
