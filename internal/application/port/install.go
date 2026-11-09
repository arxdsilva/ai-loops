package port

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
)

// Install scopes.
const (
	ScopeProject = "project"
	ScopeUser    = "user"
)

// nativeDirs maps a package's top-level dot-directory to its user-scope
// location under the home directory (specs/PLATFORMS.md §3.5, §3.6).
var nativeDirs = map[string]string{
	".opencode": ".config/opencode",
	".agents":   ".agents",
	".devin":    ".config/devin",
	".cline":    ".cline",
}

// skillDirs is where each installer platform reads skills from (relative to
// the project or home directory).
var skillDirs = map[plugin.Platform]string{
	plugin.OpenCode: ".agents/skills",
	plugin.Amp:      ".agents/skills",
	plugin.Windsurf: ".agents/skills",
	plugin.Cline:    ".cline/skills",
}

// nativeInstall gives the install command for platforms that install
// natively and need no file placement.
var nativeInstall = map[plugin.Platform]string{
	plugin.Claude:  "claude: /plugin marketplace add arxdsilva/ai-loops, then /plugin install loops@ai-loops",
	plugin.Copilot: "copilot: copilot plugin install arxdsilva/ai-loops",
	plugin.Cursor:  "cursor: install loops from the Cursor marketplace (or copy platforms/cursor to ~/.cursor/plugins/local/loops)",
	plugin.Gemini:  "gemini: clone the repo, then `gemini extensions install ./ai-loops/platforms/gemini` (no subdirectory install exists)",
}

// InstallPlan maps a generated package's files to installer destinations
// for target. scope is "project" (under projectDir) or "user" (under
// homeDir). It is pure: nothing is read or written.
func InstallPlan(target plugin.Platform, files []plugin.File, scope string, projectDir, homeDir string) ([]plugin.Placement, []string, error) {
	if scope != ScopeProject && scope != ScopeUser {
		return nil, nil, fmt.Errorf("port: unknown scope %q (want project or user)", scope)
	}
	known := false
	for _, p := range plugin.Platforms {
		known = known || p == target
	}
	if !known {
		return nil, nil, fmt.Errorf("port: unknown platform %q", target)
	}
	if msg, ok := nativeInstall[target]; ok {
		return nil, []string{msg + " (no files to place)"}, nil
	}
	base := projectDir
	if scope == ScopeUser {
		base = homeDir
	}
	if base == "" {
		return nil, nil, fmt.Errorf("port: %s directory is required for scope %s", map[string]string{ScopeProject: "project", ScopeUser: "home"}[scope], scope)
	}
	if target == plugin.Codex {
		return planCodex(files, scope, base)
	}
	return planGeneric(target, files, scope, base)
}

// PluginHome is where the shim, specs and templates are installed:
// <project>/.loops/plugin, or ~/.local/share/loops/plugin for user scope.
func PluginHome(scope, base string) string {
	if scope == ScopeUser {
		return filepath.Join(base, ".local", "share", "loops", "plugin")
	}
	return filepath.Join(base, ".loops", "plugin")
}

func planCodex(files []plugin.File, scope, base string) ([]plugin.Placement, []string, error) {
	dir := filepath.Join(base, ".codex", "agents")
	var out []plugin.Placement
	for _, f := range files {
		if p := cleanRel(f.Path); strings.HasPrefix(p, "agents/") && strings.HasSuffix(p, ".toml") {
			out = append(out, plugin.Placement{Dest: filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(p, "agents/"))), File: f})
		}
	}
	notes := []string{
		"codex: the plugin itself is installed with `codex plugin marketplace add arxdsilva/ai-loops`, then install `loops`; this command only places the role agents",
		"codex: trust the plugin hooks with /hooks",
	}
	return out, notes, nil
}

func planGeneric(target plugin.Platform, files []plugin.File, scope, base string) ([]plugin.Placement, []string, error) {
	home := PluginHome(scope, base)
	skillDir := skillDirs[target]
	// Does the package ship skills inside a native dot-directory already?
	nativeSkills := false
	for _, f := range files {
		p := cleanRel(f.Path)
		if top, rest, _ := strings.Cut(p, "/"); nativeDirs[top] != "" && strings.HasPrefix(rest, "skills/") {
			nativeSkills = true
		}
	}
	var native, shared []plugin.Placement
	for _, f := range files {
		p := cleanRel(f.Path)
		if p == "" {
			continue
		}
		if p == ".loops-generated" {
			continue
		}
		top, rest, _ := strings.Cut(p, "/")
		if userRel, ok := nativeDirs[top]; ok && rest != "" {
			if target == plugin.Windsurf && p == ".devin/hooks.v1.json" {
				// hook commands call `loops` from PATH; pin them to the plugin home's shim.
				shim := filepath.ToSlash(filepath.Join(home, "bin", "loops"))
				if scope == ScopeProject {
					shim = "$DEVIN_PROJECT_DIR/.loops/plugin/bin/loops" // no machine-specific path in a committed file
				}
				f.Data = []byte(strings.ReplaceAll(string(f.Data), `"loops hook`, `"\"`+shim+`\" hook`))
			}
			rel := p
			if scope == ScopeUser {
				rel = path.Join(userRel, rest)
			}
			native = append(native, plugin.Placement{Dest: filepath.Join(base, filepath.FromSlash(rel)), File: f})
			continue
		}
		shared = append(shared, plugin.Placement{Dest: filepath.Join(home, filepath.FromSlash(p)), File: f})
		if top == "skills" && rest != "" && !nativeSkills {
			native = append(native, plugin.Placement{Dest: filepath.Join(base, filepath.FromSlash(skillDir), filepath.FromSlash(rest)), File: f})
		}
	}
	notes := []string{
		fmt.Sprintf("%s: plugin home is %s; `loops path` finds it, or set LOOPS_PLUGIN_ROOT to it", target, home),
	}
	switch target {
	case plugin.Windsurf:
		notes = append(notes, "windsurf: no subagents, so skills/work runs in solo mode (PLATFORMS.md section 4)")
	case plugin.Amp:
		notes = append(notes, "amp: no hooks in v1; subagents use the built-in Task with role prompts")
	case plugin.Cline:
		notes = append(notes, "cline: no hooks in v1; skills/work runs in solo mode")
	case plugin.OpenCode:
		notes = append(notes, "opencode: restart OpenCode to load the agents, commands and plugin")
	}
	notes = append(notes, "make sure `loops` is on PATH, or call "+filepath.Join(home, "bin", "loops"))
	return append(native, shared...), notes, nil
}

func cleanRel(p string) string {
	p = path.Clean(filepath.ToSlash(p))
	if p == "." || path.IsAbs(p) || p == ".." || strings.HasPrefix(p, "../") {
		return ""
	}
	return p
}
