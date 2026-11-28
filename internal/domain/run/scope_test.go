package run

import (
	"slices"
	"strings"
	"testing"
)

func TestScopePattern(t *testing.T) {
	cases := []struct{ root, p, want string }{
		{"", "./internal/x/...", "internal/x/..."},
		{".", "./internal/x", "internal/x"},
		{"", "internal/x/", "internal/x"},
		{"", "./...", "..."},
		{"", "...", "..."},
		{"", ".", "."},
		{"svc", "./pkg/...", "svc/pkg/..."},
		{"svc", "./...", "svc/..."},
		{"svc", "src", "svc/src"},
		{"", "spec/models/user_spec.rb", "spec/models/user_spec.rb"},
		{"", "-p", ""},
		{"", "--workspace", ""},
		{"", "", ""},
		{"", "/abs/x", ""},
		{"", "../x", ""},
		{"", "src/*.py", ""},
	}
	for _, c := range cases {
		if got := ScopePattern(c.root, c.p); got != c.want {
			t.Errorf("ScopePattern(%q, %q) = %q, want %q", c.root, c.p, got, c.want)
		}
	}
}

func TestScopeCovers(t *testing.T) {
	cases := []struct {
		pats []string
		file string
		want bool
	}{
		{[]string{"internal/x/..."}, "internal/x/a.go", true},
		{[]string{"internal/x/..."}, "internal/x/sub/a.go", true},
		{[]string{"internal/x/..."}, "internal/xy/a.go", false},
		{[]string{"internal/x"}, "internal/x/a.go", true},
		{[]string{"internal/x"}, "./internal/x/a.go", true},
		{[]string{"internal/x"}, "internal/x/sub/a.go", false},
		{[]string{"..."}, "anything/at/all.py", true},
		{[]string{"."}, "main.go", true},
		{[]string{"."}, "cmd/main.go", false},
		{[]string{"spec/a_spec.rb"}, "spec/a_spec.rb", true},
		{[]string{"internal/x/..."}, "/abs/internal/x/a.go", false},
		{[]string{"..."}, "../other/a.go", false},
		{nil, "a.go", false},
	}
	for _, c := range cases {
		if got := ScopeCovers(c.pats, c.file); got != c.want {
			t.Errorf("ScopeCovers(%v, %q) = %v, want %v", c.pats, c.file, got, c.want)
		}
	}
	if got := ParseScope(" a/... , ,b "); !slices.Equal(got, []string{"a/...", "b"}) {
		t.Errorf("ParseScope = %q", got)
	}
	if got := ScopeArg([]string{"internal/y/b.go", "main.go", "internal/x/a.go", "internal/x/c.go"}); got != ".,./internal/x,./internal/y" {
		t.Errorf("ScopeArg = %q", got)
	}
}

// The full gate runs once per run: after it is green, green scoped gates
// covering the later edits keep the run clean (WORKFLOW §6).
func TestFoldScopedGateCoversEditsAfterGreenFullGate(t *testing.T) {
	green, red := Bool(true), Bool(false)
	edit := func(i int, f string) Event { return at(i, Event{Kind: KindStep, Name: "Edit", Target: f}) }
	all := func(i int, ok *bool) Event { return at(i, Event{Kind: KindGate, Name: GateAll, OK: ok}) }
	scope := func(i int, ok *bool, target string) Event {
		return at(i, Event{Kind: KindGate, Name: GateScope, OK: ok, Target: target})
	}
	cases := []struct {
		name      string
		evs       []Event
		dirty     bool
		unchecked []string
	}{
		{"edit after green all, no scope", []Event{edit(1, "a/x.go"), all(2, green), edit(3, "a/y.go")}, true, []string{"a/y.go"}},
		{"covering scope after the edit", []Event{all(1, green), edit(2, "a/y.go"), scope(3, green, "a")}, false, nil},
		{"recursive scope", []Event{all(1, green), edit(2, "a/b/y.go"), scope(3, green, "a/...")}, false, nil},
		{"scope recorded before the full gate ends", []Event{edit(1, "a/x.go"), edit(4, "a/y.go"), scope(5, green, "a"), all(2, green)}, false, nil},
		{"scope started before the edit", []Event{all(1, green), scope(2, green, "a"), edit(3, "a/y.go")}, true, []string{"a/y.go"}},
		{"scope misses the file", []Event{all(1, green), edit(2, "b/y.go"), scope(3, green, "a/...")}, true, []string{"b/y.go"}},
		{"red scope", []Event{all(1, green), edit(2, "a/y.go"), scope(3, red, "a")}, true, []string{"a/y.go"}},
		{"partly covered", []Event{all(1, green), edit(2, "a/y.go"), edit(3, "b/z.py"), scope(4, green, "a")}, true, []string{"b/z.py"}},
		{"non-source edit", []Event{all(1, green), edit(2, "README.md")}, false, nil},
		{"no full gate yet: scope does not stand in", []Event{edit(1, "a/y.go"), scope(2, green, "a")}, true, nil},
		{"red full gate: scope does not stand in", []Event{edit(1, "a/y.go"), all(2, red), edit(3, "a/y.go"), scope(4, green, "a")}, true, nil},
		{"red full gate after green", []Event{all(1, green), edit(2, "a/y.go"), scope(3, green, "a"), all(4, red)}, true, nil},
	}
	for _, c := range cases {
		s := Fold(c.evs)
		if s.SourceDirty != c.dirty || !slices.Equal(s.Unchecked, c.unchecked) {
			t.Errorf("%s: dirty=%v unchecked=%q, want %v %q", c.name, s.SourceDirty, s.Unchecked, c.dirty, c.unchecked)
		}
		if strings.HasSuffix(strings.Join(c.unchecked, ""), ".go") && !s.GoDirty {
			t.Errorf("%s: GoDirty not set", c.name)
		}
		if (EndReason(s) == "") == c.dirty && !strings.Contains(c.name, "red full gate") {
			t.Errorf("%s: EndReason %q with dirty=%v", c.name, EndReason(s), c.dirty)
		}
	}
}

func TestStopReasonOncePerRun(t *testing.T) {
	green, red := Bool(true), Bool(false)
	s := Fold([]Event{
		at(1, Event{Kind: KindGate, Name: GateAll, OK: green}),
		at(2, Event{Kind: KindStep, Name: "Edit", Target: "internal/x/a.go"}),
	})
	got := StopReason(s)
	if !strings.Contains(got, "loops gate --pkgs ./internal/x") || !strings.Contains(got, "never the full gate again") || !strings.Contains(got, "internal/x/a.go") {
		t.Errorf("after a green full gate the reason must name the scoped re-check: %q", got)
	}
	if got := EndReason(s); !strings.Contains(got, "loops gate --pkgs ./internal/x") {
		t.Errorf("EndReason after a green full gate: %q", got)
	}
	s = Fold([]Event{
		at(1, Event{Kind: KindStep, Name: "Edit", Target: "a.go"}),
		at(2, Event{Kind: KindGate, Name: GateAll, OK: red}),
	})
	if got := StopReason(s); !strings.Contains(got, "running it again changes nothing") {
		t.Errorf("red full gate on an unchanged tree: %q", got)
	}
	s = Fold([]Event{
		at(1, Event{Kind: KindGate, Name: GateAll, OK: red}),
		at(2, Event{Kind: KindStep, Name: "Edit", Target: "a.go"}),
	})
	if got := StopReason(s); !strings.Contains(got, "Run `loops gate`") {
		t.Errorf("red full gate, then a fix: %q", got)
	}
	many := []Event{at(1, Event{Kind: KindGate, Name: GateAll, OK: green})}
	for i, f := range []string{"a.go", "b.go", "c.go", "d.go", "e.go"} {
		many = append(many, at(2+i, Event{Kind: KindStep, Name: "Edit", Target: f}))
	}
	if got := StopReason(Fold(many)); !strings.Contains(got, "a.go, b.go, c.go and 2 more") {
		t.Errorf("file list not capped: %q", got)
	}
}
