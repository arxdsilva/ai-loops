package release

import "testing"

func TestBump(t *testing.T) {
	tests := []struct {
		name, cur, level string
		commits          []string
		next, chosen     string
		wantErr          bool
	}{
		{"auto fix", "1.2.3", "auto", []string{"fix: a", "docs: b"}, "1.2.4", "patch", false},
		{"auto feat", "1.2.3", "auto", []string{"fix: a", "feat: b"}, "1.3.0", "minor", false},
		{"auto feat scope", "1.2.3", "auto", []string{"feat(cli): b"}, "1.3.0", "minor", false},
		{"auto subject bang", "1.2.3", "auto", []string{"feat!: b"}, "2.0.0", "major", false},
		{"auto scope bang", "1.2.3", "auto", []string{"refactor(api)!: b"}, "2.0.0", "major", false},
		{"auto body breaking", "1.2.3", "auto", []string{"fix: a\n\nBREAKING CHANGE: gone"}, "2.0.0", "major", false},
		{"subject mention is not breaking", "1.2.3", "auto", []string{"fix: BREAKING CHANGE detection"}, "1.2.4", "patch", false},
		{"0.x breaking is minor", "0.3.0", "auto", []string{"feat!: x"}, "0.4.0", "minor", false},
		{"0.x feat is minor", "0.3.1", "auto", []string{"feat: x"}, "0.4.0", "minor", false},
		{"0.x fix is patch", "0.3.0", "auto", []string{"fix: x"}, "0.3.1", "patch", false},
		{"featx is not feat", "1.0.0", "auto", []string{"feature stuff"}, "1.0.1", "patch", false},
		{"explicit patch", "1.2.3", "patch", []string{"feat!: x"}, "1.2.4", "patch", false},
		{"explicit minor resets patch", "1.2.3", "minor", nil, "1.3.0", "minor", false},
		{"explicit major", "1.2.3", "major", nil, "2.0.0", "major", false},
		{"explicit major on 0.x stays major", "0.3.0", "major", nil, "1.0.0", "major", false},
		{"v prefix", "v1.2.3", "patch", nil, "1.2.4", "patch", false},
		{"pre-release dropped", "1.2.3-dev", "patch", nil, "1.2.4", "patch", false},
		{"bad level", "1.2.3", "huge", nil, "", "", true},
		{"bad version", "x", "auto", nil, "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next, chosen, err := Bump(tt.cur, tt.level, tt.commits)
			if (err != nil) != tt.wantErr || next != tt.next || chosen != tt.chosen {
				t.Fatalf("got %q %q %v; want %q %q err=%v", next, chosen, err, tt.next, tt.chosen, tt.wantErr)
			}
		})
	}
}

func TestSummarize(t *testing.T) {
	s := Summarize([]string{"feat: a", "feat(x): b", "fix: c", "chore: d", "fix!: e"})
	if s.Feat != 2 || s.Fix != 1 || s.Breaking != 1 || s.Other != 1 {
		t.Fatalf("%+v", s)
	}
	if got := s.String(); got != "2 feat, 1 fix, 1 breaking, 1 other" {
		t.Fatalf("%q", got)
	}
	if (Summary{}).String() != "no commits" {
		t.Fatal("empty")
	}
}
