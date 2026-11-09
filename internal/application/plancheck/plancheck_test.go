package plancheck

import (
	"fmt"
	"strings"
	"testing"
)

const hdr = "| id | role | model - reason | job | owns | must not touch | tests | claims |\n|---|---|---|---|---|---|---|---|\n"

func row(id, role, owns string) string {
	return fmt.Sprintf("| %s | %s | sonnet - x | job | %s | - | go test | C1 |\n", id, role, owns)
}

func rules(r Report) string {
	var s []string
	for _, f := range r.Findings {
		s = append(s, f.Rule)
	}
	return strings.Join(s, ",")
}

func TestCheck(t *testing.T) {
	big := "### Wave 1: big\n" + hdr
	for i := 0; i < 120; i++ {
		big += row(fmt.Sprintf("W1-a%d", i), "eng", fmt.Sprintf("`pkg%d/**`", i))
	}
	tests := []struct {
		name, plan string
		cap        int
		want       string
		peak       int
		peakWave   string
	}{
		{"disjoint duplicate roles", "### Wave 1: a\n" + hdr + row("W1-a", "eng", "`internal/domain/a/**`") + row("W1-b", "eng", "`internal/domain/b/**`, `cmd/x.go`"), 8, "", 2, "W1"},
		{"overlap dir", "### Wave 1\n" + hdr + row("W1-a", "eng", "internal/domain/**") + row("W1-b", "eng", "internal/domain/a/x.go"), 8, "overlap", 2, "W1"},
		{"overlap identical file", "### Wave 1\n" + hdr + row("W1-a", "eng", "a/b.go") + row("W1-b", "qa", "a/b.go"), 8, "overlap", 2, "W1"},
		{"stems disjoint", "### Wave 1\n" + hdr + row("W1-a", "eng", "a/x_*.go") + row("W1-b", "eng", "a/y_*.go"), 8, "", 2, "W1"},
		{"different waves ok", "### Wave 1\n" + hdr + row("W1-a", "eng", "a/**") + "\n### Wave 2\n" + hdr + row("W2-a", "eng", "a/**"), 8, "", 1, "W1"},
		{"sub-waves separate", "### Wave 1\n" + hdr + row("W1a-a", "eng", "a/**") + row("W1a-b", "eng", "b/**") + row("W1b-a", "eng", "a/**"), 2, "", 2, "W1a"},
		{"cap exceeded", "### Wave 1\n" + hdr + row("W1-a", "e", "a/**") + row("W1-b", "e", "b/**") + row("W1-c", "e", "c/**"), 2, "cap", 3, "W1"},
		{"120 over 100", big, 100, "cap", 120, "W1"},
		{"heading wave without id prefix", "### Wave 3: z\n" + hdr + row("x", "e", "a/**") + row("y", "e", "a/b/**"), 8, "overlap", 2, "W3"},
		{"no table", "# Plan\nnothing\n", 8, "parse", 0, ""},
		{"table without owns", "| id | role |\n|---|---|\n| a | b |\n", 8, "parse", 0, ""},
		{"empty owns none", "### Wave 1\n" + hdr + row("W1-a", "e", "-") + row("W1-b", "e", "none"), 8, "", 2, "W1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := Check(tc.plan, tc.cap)
			if rules(r) != tc.want || r.Peak != tc.peak || r.PeakWave != tc.peakWave || r.Cap != tc.cap {
				t.Fatalf("rules %q peak %d/%s cap %d: %+v", rules(r), r.Peak, r.PeakWave, r.Cap, r.Findings)
			}
			if r.OK() != (tc.want == "") {
				t.Fatal("OK mismatch")
			}
		})
	}
}

func TestFindingLines(t *testing.T) {
	plan := "### Wave 1\n" + hdr + row("W1-a", "e", "a/**") + row("W1-b", "e", "a/x")
	r := Check(plan, 8)
	if len(r.Findings) != 1 || r.Findings[0].Line != 5 || !strings.Contains(r.Findings[0].Msg, "W1-a") {
		t.Fatalf("%+v", r.Findings)
	}
}

func TestConcurrencyBlockIgnored(t *testing.T) {
	plan := "## Concurrency\n| wave | agents | cap |\n|---|---|---|\n| 1 | 2 | 8 |\n\n### Wave 1\n" + hdr + row("W1-a", "e", "a/**")
	if r := Check(plan, 8); !r.OK() || r.Peak != 1 {
		t.Fatalf("%+v", r)
	}
}
