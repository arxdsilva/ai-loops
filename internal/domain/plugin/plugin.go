// Package plugin is the platform-neutral model of the loops plugin: the
// canonical sources (Claude Code layout at the repo root) parsed into
// skills, role agents and hook bindings, which per-platform emitters turn
// into Codex, Cursor, Gemini, OpenCode, … packages (specs/PLATFORMS.md).
package plugin

// Platform is a target coding agent.
type Platform string

const (
	Claude   Platform = "claude"
	Copilot  Platform = "copilot"
	Codex    Platform = "codex"
	Cursor   Platform = "cursor"
	Gemini   Platform = "gemini"
	OpenCode Platform = "opencode"
	Amp      Platform = "amp"
	Windsurf Platform = "windsurf"
	Cline    Platform = "cline"
)

// Platforms lists every supported platform in documentation order.
var Platforms = [...]Platform{Claude, Copilot, Codex, Cursor, Gemini, OpenCode, Amp, Windsurf, Cline}

// Tier is a role's model tier (ROLES.md "Model tiers").
type Tier string

const (
	TierHaiku   Tier = "haiku"
	TierSonnet  Tier = "sonnet"
	TierOpus    Tier = "opus"
	TierInherit Tier = "inherit"
)

// Event is a canonical (Claude Code) hook event name.
type Event string

const (
	EventSessionStart       Event = "SessionStart"
	EventPreToolUse         Event = "PreToolUse"
	EventPostToolUse        Event = "PostToolUse"
	EventPostToolUseFailure Event = "PostToolUseFailure"
	EventSubagentStart      Event = "SubagentStart"
	EventSubagentStop       Event = "SubagentStop"
	EventStop               Event = "Stop"
)

// Plugin is the parsed canonical plugin.
type Plugin struct {
	Name        string
	Version     string
	Description string
	Author      string
	Homepage    string
	Repository  string
	License     string
	Keywords    []string
	Skills      []Skill
	Agents      []Agent
	Hooks       []Hook
}

// Skill is one skills/<name>/ directory.
type Skill struct {
	Name         string
	Description  string
	ArgumentHint string
	// Command is true for skills users invoke as slash commands
	// (work, init, status, report): platforms without skill invocation get
	// a command/prompt file that activates the skill.
	Command bool
	Dir     string   // path in the source FS, e.g. "skills/work"
	Files   []string // every file under Dir, source-FS paths, sorted
}

// Agent is one agents/<name>.md role definition.
type Agent struct {
	Name            string
	Description     string
	Tier            Tier
	Color           string
	Tools           []string // Claude tool allowlist, if any
	DisallowedTools []string // Claude tool denylist
	// ReadOnly is true when the role may not edit code (DisallowedTools
	// contains Edit). Platforms without tool lists map it to their
	// read-only switch.
	ReadOnly bool
	Body     string // markdown body after the frontmatter
}

// Hook binds a canonical event to a `loops hook <Name>` handler.
type Hook struct {
	Event      Event
	Matcher    string // "" = all
	Name       string // loops hook name: session-start, read-guard, …
	TimeoutSec int
}

// File is one generated output file, path relative to the package root.
type File struct {
	Path string
	Data []byte
	Exec bool // executable bit (shims)
}

// Placement is one installer copy: a package file and its absolute
// destination.
type Placement struct {
	Dest string
	File File
}
