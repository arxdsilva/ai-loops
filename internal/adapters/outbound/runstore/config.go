package runstore

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/arxdsilva/ai-loops/internal/application/langpack"
	"github.com/arxdsilva/ai-loops/internal/domain/archrule"
	"github.com/arxdsilva/ai-loops/internal/domain/hw"
	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

// Config is .loops/config.json. Every field is optional.
type Config struct {
	// Layers overrides the arch checker's layer prefixes (zero fields keep defaults).
	Layers archrule.Layers `json:"layers"`
	// Approval is "always" to ask the user to approve every plan; "" = only with --approve.
	Approval string `json:"approval,omitempty"`
	// AutoPR lets product-reviewer push and open the PR/MR without asking.
	AutoPR bool `json:"auto_pr,omitempty"`
	// Gate tunes `loops gate`.
	Gate GateConfig `json:"gate"`
	// Memory overrides the ai-memory scope; empty = .ai-memory.toml or git remote owner/repo.
	Memory MemoryConfig `json:"memory"`
	// Parallel is the always-on parallelism default (max_agents 100 when unset)
	// and the hardware-formula knobs (specs/WORKFLOW.md §2.2).
	Parallel hw.Knobs `json:"parallel"`
	// Limits are opt-in per-run/per-phase limits; zero = none (WORKFLOW §4a).
	Limits run.Limits `json:"limits"`
	// Languages pins the language packs and their roots; empty = detect
	// (loops lang detect).
	Languages []langpack.Override `json:"languages,omitempty"`
	// UpdateCheck false turns off the new-release check; nil = on.
	UpdateCheck *bool `json:"update_check,omitempty"`
}

// GateConfig tunes the gate pipeline.
type GateConfig struct {
	Skip []string `json:"skip,omitempty"` // check names to skip
	Vuln bool     `json:"vuln,omitempty"` // run govulncheck
	// CoverageMin is the minimum new-code coverage in percent; values below
	// 70 (and unset) mean 70.
	CoverageMin float64 `json:"coverage_min,omitempty"`
	// CoverageExclude adds globs to the default coverage excludes.
	CoverageExclude []string `json:"coverage_exclude,omitempty"`
	// Base is the git ref new code is diffed against; "" = merge-base with
	// origin/main (then main, then HEAD~1).
	Base string `json:"base,omitempty"`
}

// MemoryConfig scopes ai-memory pages.
type MemoryConfig struct {
	Workspace string `json:"workspace,omitempty"`
	Project   string `json:"project,omitempty"`
}

// LoadConfig reads <root>/.loops/config.json; a missing file is the zero Config.
func LoadConfig(root string) (Config, error) {
	var c Config
	b, err := os.ReadFile(filepath.Join(root, ".loops", "config.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, err
	}
	return c, nil
}
