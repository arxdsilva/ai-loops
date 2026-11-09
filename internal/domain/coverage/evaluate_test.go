package coverage

import (
	"reflect"
	"strings"
	"testing"
)

const mod = "example.com/m"

// prof builds a profile from "file:sl.sc,el.ec n c" lines (module-relative names).
func prof(t *testing.T, lines ...string) Profile {
	t.Helper()
	var b strings.Builder
	b.WriteString("mode: atomic\n")
	for _, l := range lines {
		b.WriteString(mod + "/" + l + "\n")
	}
	p, err := ParseProfile(b.String())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func findingMsgs(r Report) []string {
	var out []string
	for _, f := range r.Findings {
		out = append(out, f.String())
	}
	return out
}

func TestEvaluate(t *testing.T) {
	base := prof(t,
		"internal/x/a.go:3.1,5.2 2 1",      // lines 3-5, covered
		"internal/x/a.go:7.1,9.2 3 0",      // lines 7-9, uncovered
		"internal/x/a.go:20.1,30.2 5 4",    // spans a wide range
		"internal/y/b.go:1.1,4.2 4 0",      // y: no tests
		"internal/x/a_test.go:1.1,2.2 1 0", // test files never count
		"cmd/loops/main.go:1.1,9.2 4 0",    // excluded by default
		"internal/x/gen.go:1.1,9.2 4 0",    // generated
		"other.com/z/z.go:1.1,9.2 4 0",     // outside the module (not stripped)
	)
	has := map[string]bool{"internal/x": true, "internal/y": false}
	tests := []struct {
		name     string
		changes  Changes
		opts     Options
		ok       bool
		stmts    int
		covered  int
		findings []string
		note     string
	}{
		{
			name:    "covered change passes",
			changes: Changes{"internal/x/a.go": {Ranges: []LineRange{{4, 4}}}},
			ok:      true, stmts: 2, covered: 2,
			note: "new code 100.0% (min 70%)",
		},
		{
			name:    "range touching a block edge and a spanning block",
			changes: Changes{"internal/x/a.go": {Ranges: []LineRange{{5, 7}, {25, 25}}}},
			// 3-5 (2, covered) + 7-9 (3, uncovered) + 20-30 (5, covered) = 7/10
			ok: true, stmts: 10, covered: 7,
			note: "new code 70.0% (min 70%)",
		},
		{
			name:    "block intersecting two ranges counts once",
			changes: Changes{"internal/x/a.go": {Ranges: []LineRange{{21, 21}, {29, 31}}}},
			ok:      true, stmts: 5, covered: 5,
			note: "new code 100.0% (min 70%)",
		},
		{
			name:    "below min",
			changes: Changes{"internal/x/a.go": {Ranges: []LineRange{{1, 10}}}},
			ok:      false, stmts: 5, covered: 2,
			findings: []string{"new code coverage 40.0% < 70%"},
			note:     "new code 40.0% (min 70%)",
		},
		{
			name:    "untested file and package without tests",
			changes: Changes{"internal/y/b.go": {Ranges: []LineRange{{2, 2}}}},
			ok:      false, stmts: 4, covered: 0,
			findings: []string{
				"internal/y/b.go:2: package internal/y has no tests",
				"internal/y/b.go:2: new code in b.go is untested (0/4 statements)",
				"new code coverage 0.0% < 70%",
			},
			note: "new code 0.0% (min 70%)",
		},
		{
			name: "ignored files: tests, excluded, generated, non-go, outside module, deleted-only",
			changes: Changes{
				"internal/x/a_test.go":     {Ranges: []LineRange{{1, 2}}},
				"cmd/loops/main.go":        {Ranges: []LineRange{{1, 9}}},
				"internal/x/gen.go":        {Ranges: []LineRange{{1, 9}}, Generated: true},
				"README.md":                {Ranges: []LineRange{{1, 9}}},
				"internal/x/a.go":          {},
				"internal/x/types.go":      {Ranges: []LineRange{{1, 50}}}, // no statements in the profile
				"internal/x/x.pb.go":       {Ranges: []LineRange{{1, 50}}},
				"internal/x/testdata/t.go": {Ranges: []LineRange{{1, 5}}},
			},
			ok: true, note: "no new statements",
		},
		{
			name:    "custom exclude and min floor",
			changes: Changes{"internal/y/b.go": {Ranges: []LineRange{{1, 4}}}, "internal/x/a.go": {Ranges: []LineRange{{3, 3}}}},
			opts:    Options{Min: 10, Exclude: []string{"internal/y/**"}},
			ok:      true, stmts: 2, covered: 2,
			note: "new code 100.0% (min 70%)",
		},
		{
			name:    "higher min",
			changes: Changes{"internal/x/a.go": {Ranges: []LineRange{{5, 7}, {25, 25}}}},
			opts:    Options{Min: 75.5},
			ok:      false, stmts: 10, covered: 7,
			findings: []string{"new code coverage 70.0% < 75.5%"},
			note:     "new code 70.0% (min 75.5%)",
		},
		{
			name:    "basename pattern",
			changes: Changes{"internal/y/b.go": {Ranges: []LineRange{{1, 4}}}},
			opts:    Options{Exclude: []string{"b.go"}},
			ok:      true, note: "no new statements",
		},
		{
			name:    "scoped to packages",
			changes: Changes{"internal/y/b.go": {Ranges: []LineRange{{1, 4}}}, "internal/x/a.go": {Ranges: []LineRange{{3, 3}}}},
			opts:    Options{Pkgs: []string{"./internal/x/..."}},
			ok:      true, stmts: 2, covered: 2,
			note: "new code 100.0% (min 70%)",
		},
		{
			name:    "scoped by exact dir and import path",
			changes: Changes{"internal/y/b.go": {Ranges: []LineRange{{1, 4}}}, "internal/x/a.go": {Ranges: []LineRange{{3, 3}}}},
			opts:    Options{Pkgs: []string{"example.com/m/internal/y"}},
			ok:      false, stmts: 4, covered: 0,
			findings: []string{
				"internal/y/b.go:1: package internal/y has no tests",
				"internal/y/b.go:1: new code in b.go is untested (0/4 statements)",
				"new code coverage 0.0% < 70%",
			},
			note: "new code 0.0% (min 70%)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := tt.opts
			o.Module = mod
			r := Evaluate(base, tt.changes, has, o)
			if r.OK != tt.ok || r.Stmts != tt.stmts || r.Covered != tt.covered || r.Note != tt.note {
				t.Fatalf("got ok=%v %d/%d note=%q findings=%v", r.OK, r.Covered, r.Stmts, r.Note, findingMsgs(r))
			}
			if got := findingMsgs(r); !reflect.DeepEqual(got, tt.findings) {
				t.Fatalf("findings %q", got)
			}
		})
	}
}

func TestEvaluateRowsAndTable(t *testing.T) {
	p := prof(t,
		"internal/x/a.go:3.1,5.2 2 1",
		"internal/x/a.go:7.1,9.2 3 0",
		"internal/x/a.go:11.1,11.9 1 0",
		"internal/x/b.go:1.1,2.2 1 1",
	)
	ch := Changes{
		"internal/x/b.go": {Ranges: []LineRange{{1, 1}}},
		"internal/x/a.go": {Ranges: []LineRange{{1, 12}}},
	}
	r := Evaluate(p, ch, map[string]bool{"internal/x": true}, Options{Module: mod})
	want := []FileRow{
		{File: "internal/x/a.go", Stmts: 6, Covered: 2, Line: 3, Uncovered: []LineRange{{7, 9}, {11, 11}}},
		{File: "internal/x/b.go", Stmts: 1, Covered: 1, Line: 1},
	}
	if !reflect.DeepEqual(r.Rows, want) {
		t.Fatalf("%+v", r.Rows)
	}
	if r.Percent() < 42.8 || r.Percent() > 42.9 {
		t.Fatal(r.Percent())
	}
	tbl := r.Table(40)
	for _, s := range []string{"internal/x/a.go", "6", "33.3", "7-9,11", "total", "42.8"} {
		if !strings.Contains(tbl, s) {
			t.Fatalf("table lacks %q:\n%s", s, tbl)
		}
	}
	for _, l := range strings.Split(tbl, "\n") {
		if strings.HasSuffix(l, " ") {
			t.Fatalf("trailing space in %q", l)
		}
	}
	out := r.String()
	if !strings.Contains(out, tbl) || !strings.Contains(out, "new code coverage 42.8% < 70%") {
		t.Fatal(out)
	}
	if (Report{}).Percent() != 100 {
		t.Fatal("empty report is 100%")
	}
}

func TestTableTruncates(t *testing.T) {
	var rows []FileRow
	for i := 0; i < 60; i++ {
		rows = append(rows, FileRow{File: "f" + itoa3(i) + ".go", Stmts: 10, Covered: i % 10})
	}
	r := Report{Rows: rows, Stmts: 600, Covered: 270}
	tbl := r.Table(40)
	lines := strings.Split(strings.TrimRight(tbl, "\n"), "\n")
	if len(lines) != 40 {
		t.Fatalf("%d lines:\n%s", len(lines), tbl)
	}
	if !strings.Contains(tbl, "more files") || !strings.Contains(lines[len(lines)-1], "total") {
		t.Fatal(tbl)
	}
	// The worst files (most uncovered statements) are the ones shown.
	if !strings.Contains(tbl, "f000.go") || strings.Contains(tbl, "f009.go") {
		t.Fatal(tbl)
	}
	if got := (Report{Note: "no new statements"}).Table(40); got != "" {
		t.Fatalf("%q", got)
	}
	long := Report{Rows: []FileRow{{File: "a.go", Stmts: 30, Uncovered: manyRanges(30)}}, Stmts: 30}
	if tl := long.Table(40); !strings.Contains(tl, "…") {
		t.Fatal(tl)
	}
}

func manyRanges(n int) []LineRange {
	var out []LineRange
	for i := 0; i < n; i++ {
		out = append(out, LineRange{i * 10, i*10 + 1})
	}
	return out
}

func itoa3(i int) string {
	return string([]byte{byte('0' + i/100), byte('0' + i/10%10), byte('0' + i%10)})
}

func TestEvaluateWhole(t *testing.T) {
	p := prof(t,
		"internal/x/a.go:3.1,5.2 2 1",
		"internal/x/a.go:7.1,9.2 3 0",
		"internal/y/b.go:1.1,4.2 4 0",
		"cmd/loops/main.go:1.1,9.2 4 0",
	)
	ch := WholeChanges(p, mod)
	if _, ok := ch["cmd/loops/main.go"]; !ok || len(ch) != 3 {
		t.Fatalf("%+v", ch)
	}
	r := Evaluate(p, ch, nil, Options{Module: mod, Whole: true, Pkgs: []string{"./internal/x"}})
	if r.OK || r.Stmts != 5 || r.Covered != 2 {
		t.Fatalf("%+v", r)
	}
	if got := findingMsgs(r); !reflect.DeepEqual(got, []string{"package coverage 40.0% < 70%"}) {
		t.Fatal(got)
	}
	if r.Note != "no git base: whole-package coverage 40.0% (min 70%)" {
		t.Fatal(r.Note)
	}
	ok := Evaluate(p, ch, nil, Options{Module: mod, Whole: true, Pkgs: []string{"./internal/x/..."}, Min: 0})
	if ok.OK {
		t.Fatal("40% must fail the floor of 70")
	}
	none := Evaluate(Profile{}, Changes{}, nil, Options{Whole: true})
	if !none.OK || none.Note != "no git base: no statements" {
		t.Fatalf("%+v", none)
	}
}

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pat, path string
		want      bool
	}{
		{"cmd/**", "cmd/loops/main.go", true},
		{"cmd/**", "internal/cmd/x.go", false},
		{"**/testdata/**", "testdata/a.go", true},
		{"**/testdata/**", "a/b/testdata/c/d.go", true},
		{"**/*.pb.go", "x.pb.go", true},
		{"**/*.pb.go", "a/b/x.pb.go", true},
		{"**/zz_generated*", "a/zz_generated.deepcopy.go", true},
		{"internal/compose/**", "internal/compose/loops/loops.go", true},
		{"*.go", "a/b.go", true}, // no slash: basename anywhere
		{"internal/*.go", "internal/a/b.go", false},
		{"internal/*/b.go", "internal/a/b.go", true},
		{"a/**/b.go", "a/b.go", true},
		{"a/**/b.go", "a/x/y/b.go", true},
		{"[", "a.go", false}, // bad pattern never matches
		{"a/[", "a/b", false},
	}
	for _, tt := range tests {
		if got := matchGlob(tt.pat, tt.path); got != tt.want {
			t.Errorf("%q %q: got %v", tt.pat, tt.path, got)
		}
	}
}

func TestCandidatesAndDirs(t *testing.T) {
	ch := Changes{
		"internal/x/a.go":      {Ranges: []LineRange{{1, 1}}},
		"internal/x/a_test.go": {Ranges: []LineRange{{1, 1}}},
		"cmd/loops/main.go":    {Ranges: []LineRange{{1, 1}}},
		"main.go":              {Ranges: []LineRange{{1, 1}}},
		"doc.md":               {Ranges: []LineRange{{1, 1}}},
		"internal/y/b.go":      {},
	}
	got := Candidates(ch, Options{Module: mod})
	if !reflect.DeepEqual(got, []string{"internal/x/a.go", "main.go"}) {
		t.Fatal(got)
	}
	if d := PkgDirs(append(got, "internal/x/c.go")); !reflect.DeepEqual(d, []string{".", "internal/x"}) {
		t.Fatal(d)
	}
}

func TestWholeFile(t *testing.T) {
	if w := WholeFile(); len(w) != 1 || w[0].Start != 1 || w[0].End < 1<<30 {
		t.Fatal(w)
	}
}
