// Package pack is the language-pack model: everything loops needs to know
// about a backend language to gate, guard, scope caches, clean up and check
// architecture without per-language code (research/languages/TOOLING.md §2).
// A pack is data (specs/languages/<id>/pack.json); this package parses,
// validates, selects build-tool variants and expands argv placeholders. Pure:
// file access goes through the View interface the caller supplies.
package pack

import "slices"

// Schema is the pack schema version this package reads.
const Schema = 1

// Pack is one language pack.
type Pack struct {
	Schema       int                    `json:"schema"`
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Versions     Versions               `json:"versions"`
	Detect       []Marker               `json:"detect"`
	DetectNot    []Marker               `json:"detect_not,omitempty"`
	Variants     []Variant              `json:"variants,omitempty"`
	Scope        Scope                  `json:"scope"`
	SourceGlobs  []string               `json:"source_globs"`
	TestGlobs    []string               `json:"test_globs"`
	Generated    Generated              `json:"generated"`
	Manifest     string                 `json:"manifest,omitempty"`
	Env          map[string]string      `json:"env,omitempty"`
	Checks       []Check                `json:"checks"`
	TestCommands []TestCommand          `json:"test_commands"`
	Race         Race                   `json:"race"`
	Coverage     Coverage               `json:"coverage"`
	PostEdit     *PostEdit              `json:"post_edit"`
	CacheEnv     map[string]string      `json:"cache_env"`
	SharedEnv    []string               `json:"shared_env,omitempty"`
	CacheArgv    map[string][]string    `json:"cache_argv,omitempty"`
	EnvHints     map[string]string      `json:"env_hints,omitempty"`
	Artifacts    []string               `json:"artifacts"`
	Arch         Arch                   `json:"arch"`
	Modules      *Modules               `json:"modules,omitempty"`
	Placeholders map[string]Placeholder `json:"placeholders,omitempty"`
	TestJobs     string                 `json:"test_jobs,omitempty"`
	ConfigFiles  []string               `json:"config_files,omitempty"`
	Sources      map[string]string      `json:"sources,omitempty"`
	Notes        []string               `json:"notes,omitempty"`

	// Variant is the name of the variant Resolve applied ("" = base pack).
	Variant string `json:"-"`
}

// Versions records the language versions the pack was written against.
type Versions struct {
	Stable string `json:"stable"`
	LTS    string `json:"lts,omitempty"`
	AsOf   string `json:"as_of"`
}

// Marker is one detection condition, relative to a candidate root: File
// (a file or directory exists) or Glob (some entry matches), optionally
// with Contains (File's content holds this substring).
type Marker struct {
	File     string `json:"file,omitempty"`
	Glob     string `json:"glob,omitempty"`
	Contains string `json:"contains,omitempty"`
}

// Variant overlays the base pack when its Detect markers match (the first
// matching variant wins): checks merge by name, other set fields replace.
type Variant struct {
	Name         string              `json:"name"`
	Detect       []Marker            `json:"detect"`
	Checks       []Check             `json:"checks,omitempty"`
	TestCommands []TestCommand       `json:"test_commands,omitempty"`
	Coverage     *Coverage           `json:"coverage,omitempty"`
	PostEdit     *PostEdit           `json:"post_edit,omitempty"`
	CacheEnv     map[string]string   `json:"cache_env,omitempty"`
	CacheArgv    map[string][]string `json:"cache_argv,omitempty"`
	Env          map[string]string   `json:"env,omitempty"`
	Artifacts    []string            `json:"artifacts,omitempty"`
	Race         *Race               `json:"race,omitempty"`
}

// Scope holds the default values of the list placeholders {pkgs} and
// {pkg_dirs}; `loops gate --pkgs` replaces them.
type Scope struct {
	Pkgs    []string `json:"pkgs,omitempty"`
	PkgDirs []string `json:"pkg_dirs,omitempty"`
}

// Generated identifies generated sources, which coverage never judges.
type Generated struct {
	Globs       []string `json:"globs,omitempty"`
	HeaderRegex string   `json:"header_regex,omitempty"`
}

// Check is one gate check. Internal checks ("arch", "coverage") have no
// argv: loops runs them itself.
type Check struct {
	Name          string            `json:"name"`
	Kind          string            `json:"kind"`
	Argv          []string          `json:"argv,omitempty"`
	Internal      string            `json:"internal,omitempty"`
	Required      bool              `json:"required"`
	Tool          string            `json:"tool,omitempty"`
	Install       string            `json:"install,omitempty"`
	Default       string            `json:"default,omitempty"`
	Skippable     *bool             `json:"skippable,omitempty"`
	WhenFile      []string          `json:"when_file,omitempty"`
	FallbackArgv  []string          `json:"fallback_argv,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	EnvRequired   []string          `json:"env_required,omitempty"`
	FailsOn       string            `json:"fails_on,omitempty"`
	OutputExclude []string          `json:"output_exclude,omitempty"`
	FailRegex     string            `json:"fail_regex,omitempty"`
	ExitCodes     map[string]string `json:"exit_codes,omitempty"`
	SkipOn        *SkipOn           `json:"skip_on,omitempty"`
	CoverageArgs  []string          `json:"coverage_args,omitempty"`
	SignalRegex   string            `json:"signal_regex,omitempty"`
	Repeat        int               `json:"repeat,omitempty"`
	Note          string            `json:"note,omitempty"`
}

// SkipOn turns a known tool failure (output matching Regex) into a skip.
type SkipOn struct {
	OutputRegex string `json:"output_regex"`
	Note        string `json:"note"`
}

// IsSkippable reports whether `loops gate --skip` may skip the check.
func (c Check) IsSkippable() bool { return c.Skippable == nil || *c.Skippable }

// OffByDefault reports whether the check runs only when enabled.
func (c Check) OffByDefault() bool { return c.Default == "off" }

// TestCommand is a command prefix the test guard treats as a test run.
type TestCommand struct {
	ArgvPrefix []string `json:"argv_prefix"`
}

// Race is the pack's race policy (TAXONOMY CONC-04, WORKFLOW §6).
type Race struct {
	Mode       string              `json:"mode"`
	Flags      []string            `json:"flags,omitempty"`
	Env        map[string]string   `json:"env,omitempty"`
	Checks     []string            `json:"checks"`
	Variants   []map[string]string `json:"variants,omitempty"`
	Reason     string              `json:"reason,omitempty"`
	Substitute string              `json:"substitute,omitempty"`
}

// Race modes.
const (
	RaceFlag  = "flag"
	RaceBuild = "build"
	RaceNone  = "none"
)

// Coverage says where the coverage report is and how to read it.
type Coverage struct {
	Format      string    `json:"format"`
	Path        string    `json:"path"`
	ProducedBy  []string  `json:"produced_by"`
	Produce     []Produce `json:"produce,omitempty"`
	Granularity string    `json:"granularity,omitempty"`
	Exclude     []string  `json:"exclude,omitempty"`
	SourceRoots []string  `json:"source_roots,omitempty"`
}

// Coverage formats loops parses.
const (
	FormatGoCover   = "gocover"
	FormatLCOV      = "lcov"
	FormatCobertura = "cobertura"
	FormatJaCoCo    = "jacoco"
)

// Produce is a post-test export step; StdoutTo writes its stdout to a path.
type Produce struct {
	Argv     []string `json:"argv"`
	StdoutTo string   `json:"stdout_to,omitempty"`
}

// PostEdit is the per-file hook after an agent edits a source file.
type PostEdit struct {
	FormatFile []string `json:"format_file,omitempty"`
	LintDir    []string `json:"lint_dir,omitempty"`
}

// Arch is the data the architecture checker needs.
type Arch struct {
	Parser                string         `json:"parser"`
	ImportRegex           string         `json:"import_regex,omitempty"`
	Resolver              string         `json:"resolver,omitempty"`
	NativeCheck           string         `json:"native_check,omitempty"`
	ForbiddenInDomain     []string       `json:"forbidden_in_domain,omitempty"`
	ThirdPartyForbiddenIn []string       `json:"third_party_forbidden_in,omitempty"`
	ImpureCalls           []string       `json:"impure_calls,omitempty"`
	SizeWarnings          map[string]int `json:"size_warnings,omitempty"`
}

// Arch parsers.
const (
	ParserGo    = "go"
	ParserRegex = "regex"
	ParserNone  = "none"
)

// Modules enables per-module gating (Go multi-module repos, D19).
type Modules struct {
	Discover []string `json:"discover"`
	Marker   string   `json:"marker"`
}

// Placeholder is a pack-declared value resolved before a check runs: the
// trimmed stdout of Argv (its dirname when Dirname), the first submatch of
// Regex in File, or the paths matching Glob.
type Placeholder struct {
	Argv    []string `json:"argv,omitempty"`
	Dirname bool     `json:"dirname,omitempty"`
	File    string   `json:"file,omitempty"`
	Regex   string   `json:"regex,omitempty"`
	Glob    []string `json:"glob,omitempty"`
}

// Check kinds.
var Kinds = []string{"deps", "build", "format", "lint", "typecheck", "arch", "test", "coverage", "vuln", "sast", "sbom"}

// Formats lists the coverage formats loops parses.
var Formats = []string{FormatGoCover, FormatLCOV, FormatCobertura, FormatJaCoCo}

// Builtins lists the placeholders loops fills itself.
var Builtins = []string{"file", "dir", "manifest_dir", "pkgs", "pkg_dirs", "coverage", "coverage_dir", "coverage_args", "run", "cache", "tmp", "seed"}

// CheckByName returns the named check.
func (p Pack) CheckByName(name string) (Check, bool) {
	for _, c := range p.Checks {
		if c.Name == name {
			return c, true
		}
	}
	return Check{}, false
}

// RaceCheck reports whether the named check carries the race policy.
func (p Pack) RaceCheck(name string) bool { return slices.Contains(p.Race.Checks, name) }
