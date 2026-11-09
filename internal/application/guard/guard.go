// Package guard decides whether a tool call is allowed: the read guard
// (no whole-file Reads of big files, no long shell views of repo files) and
// the agent model guard (untyped agents must name a model). Ported from
// rimworld-go .agents/hooks/read_guard.py and agent_model_guard.py.
package guard

import "encoding/json"

// Files is what the read guard needs from the filesystem.
type Files interface {
	// Lines counts newline-terminated lines (a final unterminated line counts).
	Lines(path string) (int, error)
	// Exists reports whether path is an existing regular file.
	Exists(path string) bool
	// Glob expands a shell glob (filepath.Glob semantics).
	Glob(pattern string) ([]string, error)
}

// Config holds the read-guard thresholds (env READ_GUARD_MIN_LINES,
// READ_GUARD_MAX_VIEW_LINES; READ_GUARD=off disables via Off).
type Config struct {
	MinLines     int  // whole-file Read denied above this many lines (default 500)
	MaxViewLines int  // shell views denied above this many lines (default 150)
	Off          bool // READ_GUARD=off
}

// DefaultConfig returns the rimworld-go thresholds.
func DefaultConfig() Config { return Config{MinLines: 500, MaxViewLines: 150} }

// Guard evaluates tool calls.
type Guard struct {
	files Files
	cfg   Config
}

// New builds a Guard.
func New(files Files, cfg Config) *Guard { return &Guard{files: files, cfg: cfg} }

// CheckRead returns a deny reason ("" = allow) for a Read or Bash tool call.
// toolInput is the hook's raw tool_input; cwd the call's cwd; repo the
// project root (only files under repo are guarded).
func (g *Guard) CheckRead(toolName string, toolInput json.RawMessage, cwd, repo string) string {
	if g.cfg.Off {
		return ""
	}
	minLines, maxView := g.cfg.MinLines, g.cfg.MaxViewLines
	if minLines == 0 {
		minLines = DefaultConfig().MinLines
	}
	if maxView == 0 {
		maxView = DefaultConfig().MaxViewLines
	}
	var in map[string]any
	if err := json.Unmarshal(toolInput, &in); err != nil {
		return ""
	}
	switch toolName {
	case "Read":
		return g.checkReadTool(in, repo, minLines)
	case "Bash":
		command, _ := in["command"].(string)
		return g.checkBash(command, cwd, repo, maxView)
	}
	return ""
}

// CheckModel returns a deny reason ("" = allow) for an Agent tool call:
// subagent_type missing, "" or "general-purpose" with no model is denied.
func CheckModel(toolInput json.RawMessage) string {
	return checkModel(toolInput)
}
