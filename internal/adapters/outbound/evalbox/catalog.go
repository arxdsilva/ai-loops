// Package evalbox is the disk and process side of `loops eval`: the
// scenario catalog under evals/, the throwaway sandbox repo built from the
// fixture, the reader for the run the agent left there, and the agent
// process itself.
//
//	evals/fixture/                     the Go repo every scenario starts from
//	evals/scenarios/<name>/scenario.json  assertions (domain/evals.Scenario)
//	evals/scenarios/<name>/ticket.md      the ticket
//	evals/scenarios/<name>/prompt.md      the agent prompt; {{ticket}} is replaced
//	evals/scenarios/<name>/overlay.patch  optional: applied uncommitted on the fixture
//	evals/scenarios/<name>/seed/          optional: copied to .loops/eval-seed/
package evalbox

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/application/evalrun"
	"github.com/arxdsilva/ai-loops/internal/domain/evals"
)

// TicketPlaceholder in prompt.md is replaced by ticket.md.
const TicketPlaceholder = "{{ticket}}"

// Catalog reads scenarios from an evals/ directory.
type Catalog struct{ Root string }

func (c Catalog) scenarios() string { return filepath.Join(c.Root, "scenarios") }

// Names lists the scenario directories that hold a scenario.json, sorted.
func (c Catalog) Names() ([]string, error) {
	entries, err := os.ReadDir(c.scenarios())
	if err != nil {
		return nil, fmt.Errorf("evals: %w", err)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(c.scenarios(), e.Name(), "scenario.json")); err == nil {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

// Load reads scenario name. Unknown JSON fields are an error, so a typo in
// an assertion field cannot pass silently.
func (c Catalog) Load(name string) (evalrun.Spec, error) {
	if !filepath.IsLocal(name) || strings.ContainsAny(name, `/\`) {
		return evalrun.Spec{}, fmt.Errorf("evals: bad scenario name %q", name)
	}
	dir := filepath.Join(c.scenarios(), name)
	raw, err := os.ReadFile(filepath.Join(dir, "scenario.json"))
	if errors.Is(err, fs.ErrNotExist) {
		names, _ := c.Names()
		return evalrun.Spec{}, fmt.Errorf("evals: no scenario %q (known: %s)", name, strings.Join(names, ", "))
	}
	if err != nil {
		return evalrun.Spec{}, err
	}
	var sc evals.Scenario
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&sc); err != nil {
		return evalrun.Spec{}, fmt.Errorf("evals: %s/scenario.json: %w", name, err)
	}
	if sc.Name != name {
		return evalrun.Spec{}, fmt.Errorf("evals: %s/scenario.json names %q", name, sc.Name)
	}
	ticket, err := os.ReadFile(filepath.Join(dir, "ticket.md"))
	if err != nil {
		return evalrun.Spec{}, fmt.Errorf("evals: %s: %w", name, err)
	}
	prompt, err := os.ReadFile(filepath.Join(dir, "prompt.md"))
	if err != nil {
		return evalrun.Spec{}, fmt.Errorf("evals: %s: %w", name, err)
	}
	return evalrun.Spec{
		Scenario: sc,
		Prompt:   strings.ReplaceAll(string(prompt), TicketPlaceholder, strings.TrimSpace(string(ticket))),
		Dir:      dir,
	}, nil
}
