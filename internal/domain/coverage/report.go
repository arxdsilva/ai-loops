package coverage

import (
	"fmt"
	"sort"
	"strings"
)

const maxUncoveredCol = 32

// Table renders the per-file rows and the total in at most maxLines lines
// ("" when no file has changed statements). When the rows do not fit, the
// files with the most uncovered statements are shown first and the rest
// are counted.
func (r Report) Table(maxLines int) string {
	if len(r.Rows) == 0 {
		return ""
	}
	rows := append([]FileRow(nil), r.Rows...)
	more := 0
	if room := maxLines - 2; len(rows) > room { // header + total
		room-- // the "… more" line
		sort.SliceStable(rows, func(i, j int) bool {
			ui, uj := rows[i].Stmts-rows[i].Covered, rows[j].Stmts-rows[j].Covered
			if ui != uj {
				return ui > uj
			}
			return rows[i].File < rows[j].File
		})
		more, rows = len(rows)-room, rows[:room]
	}
	width := len("total")
	for _, row := range rows {
		width = max(width, len(row.File))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-*s %6s %7s %6s  %s\n", width, "file", "stmts", "covered", "%", "uncovered lines")
	for _, row := range rows {
		line := fmt.Sprintf("%-*s %6d %7d %6.1f  %s", width, row.File, row.Stmts, row.Covered,
			floor1(pct(row.Covered, row.Stmts)), ranges(row.Uncovered))
		b.WriteString(strings.TrimRight(line, " ") + "\n")
	}
	if more > 0 {
		fmt.Fprintf(&b, "… %d more files\n", more)
	}
	fmt.Fprintf(&b, "%-*s %6d %7d %6.1f\n", width, "total", r.Stmts, r.Covered, floor1(r.Percent()))
	return b.String()
}

// String is the table followed by one finding per line.
func (r Report) String() string {
	var b strings.Builder
	b.WriteString(r.Table(40))
	for _, f := range r.Findings {
		b.WriteString(f.String() + "\n")
	}
	return b.String()
}

func pct(n, d int) float64 {
	if d == 0 {
		return 100
	}
	return float64(n) * 100 / float64(d)
}

// ranges renders "7-9,11", cut to a column width.
func ranges(rs []LineRange) string {
	var parts []string
	for _, r := range rs {
		if r.Start == r.End {
			parts = append(parts, fmt.Sprint(r.Start))
		} else {
			parts = append(parts, fmt.Sprintf("%d-%d", r.Start, r.End))
		}
	}
	s := strings.Join(parts, ",")
	if len(s) > maxUncoveredCol {
		s = s[:maxUncoveredCol-1] + "…"
	}
	return s
}
