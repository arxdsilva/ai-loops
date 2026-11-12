package run

// StatusDoc is the run-status document: what `loops status --json` prints
// and what .loops/status.json holds for the live pane (WORKFLOW §5).
type StatusDoc struct {
	Active bool   `json:"active"`
	Dir    string `json:"dir,omitempty"`
	State  *State `json:"state,omitempty"`
}
