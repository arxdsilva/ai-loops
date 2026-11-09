// Package archrule holds the architecture rules as pure functions over a
// parsed package model. Worker A1 owns this package; the types below are the
// contract the rest of the tool relies on and may only grow additively.
package archrule

// Severity of a finding.
type Severity string

const (
	SeverityError Severity = "error" // fails `loops arch`
	SeverityWarn  Severity = "warn"  // reported, never fails (SOLID size limits)
)

// Finding is one rule violation.
type Finding struct {
	File     string   `json:"file"` // path relative to the module root
	Line     int      `json:"line"`
	Rule     string   `json:"rule"` // e.g. "imports", "init", "global", "cmd", "methods"
	Severity Severity `json:"severity"`
	Msg      string   `json:"msg"`
}

// Report is the outcome of a check.
type Report struct {
	Module   string    `json:"module"`
	Findings []Finding `json:"findings"`
}

// OK reports whether the report has no error-severity findings.
func (r Report) OK() bool {
	for _, f := range r.Findings {
		if f.Severity == SeverityError {
			return false
		}
	}
	return true
}

// Layers maps architecture layers to module-relative directory prefixes.
// Zero value fields fall back to DefaultLayers.
type Layers struct {
	Domain      []string `json:"domain"`
	Application []string `json:"application"`
	Ports       []string `json:"ports"`
	Inbound     []string `json:"inbound"`
	Outbound    []string `json:"outbound"`
	Platform    []string `json:"platform"`
	Compose     []string `json:"compose"`
	Cmd         []string `json:"cmd"`
	Pkg         []string `json:"pkg"`
	Adapters    []string `json:"adapters"` // root of every adapter tree; Inbound and Outbound refine it
}

// DefaultLayers is the layout of specs/HEXAGONAL_ARCHITECTURE.md §4.
func DefaultLayers() Layers {
	return Layers{
		Domain:      []string{"internal/domain"},
		Application: []string{"internal/application"},
		Ports:       []string{"internal/ports"},
		Inbound:     []string{"internal/adapters/inbound"},
		Outbound:    []string{"internal/adapters/outbound"},
		Platform:    []string{"internal/platform"},
		Compose:     []string{"internal/compose"},
		Cmd:         []string{"cmd"},
		Pkg:         []string{"pkg"},
		Adapters:    []string{"internal/adapters"},
	}
}
