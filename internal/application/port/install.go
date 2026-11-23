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
// location under the home directory (specs/PLATFORMS.md §3.5, §3.6, §3.7).
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

type hookShim struct {
	file        string
	projectShim string
}

var hookShims = map[plugin.Platform]hookShim{
	plugin.Windsurf: {file: ".devin/hooks.v1.json", projectShim: "$DEVIN_PROJECT_DIR/.loops/plugin/bin/loops"},
}

var platformNotes = map[plugin.Platform]string{
	plugin.Windsurf:    "windsurf: no subagents, so skills/work runs in solo mode (PLATFORMS.md section 4)",
	plugin.Amp:         "amp: no hooks in v1; subagents use the built-in Task with role prompts",
	plugin.Cline:       "cline: no hooks in v1; skills/work runs in solo mode",
	plugin.OpenCode:    "opencode: restart OpenCode to load the agents, commands and plugin",
	plugin.Antigravity: "antigravity: run /skills reload in Antigravity app or CLI (`agy`) to load skills; Tier A resumable custom subagents (invoke_subagent + send_message) (PLATFORMS.md section 4)",
}

// InstallPlan maps a generated package's files to installer destinations
// for target. scope is "project" (under projectDir) or "user" (under
// homeDir). It is pure: nothing is read or written.
func InstallPlan(target plugin.Platform, files []plugin.File, scope string, projectDir, homeDir string) ([]plugin.Placement, []string, error) {
	if scope != ScopeProject && scope != ScopeUser {
		return nil, nil, fmt.Errorf("port: unknown scope %q (want project or user)", scope)
	}
	if !knownPlatform(target) {
		return nil, nil, fmt.Errorf("port: unknown platform %q", target)
	}
	if msg, ok := nativeInstall[target]; ok {
		return nil, []string{msg + " (no files to place)"}, nil
	}
	base := scopeBase(scope, projectDir, homeDir)
	if base == "" {
		return nil, nil, fmt.Errorf("port: %s directory is required for scope %s", scopeDirName(scope), scope)
	}
	switch target {
	case plugin.Codex:
		return planCodex(files, scope, base)
	case plugin.Antigravity:
		return planAntigravity(files, scope, base)
	default:
		return planGeneric(target, files, scope, base)
	}
}

func knownPlatform(target plugin.Platform) bool {
	for _, p := range plugin.Platforms {
		if p == target {
			return true
		}
	}
	return false
}

func scopeBase(scope, projectDir, homeDir string) string {
	if scope == ScopeUser {
		return homeDir
	}
	return projectDir
}

func scopeDirName(scope string) string {
	if scope == ScopeUser {
		return "home"
	}
	return "project"
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

func planAntigravity(files []plugin.File, scope, base string) ([]plugin.Placement, []string, error) {
	home := PluginHome(scope, base)
	nativeBase := antigravityNativeBase(scope, base)
	var native, shared []plugin.Placement
	for _, f := range files {
		p := cleanRel(f.Path)
		if p == "" || p == ".loops-generated" {
			continue
		}
		native, shared = appendAntigravityFile(native, shared, p, f, nativeBase, home, scope)
	}
	return append(native, shared...), installNotes(plugin.Antigravity, home), nil
}

func antigravityNativeBase(scope, base string) string {
	if scope == ScopeUser {
		return filepath.Join(base, ".gemini", "config")
	}
	return filepath.Join(base, ".agents")
}

func appendAntigravityFile(native, shared []plugin.Placement, p string, f plugin.File, nativeBase, home, scope string) ([]plugin.Placement, []plugin.Placement) {
	np := antigravityNormalize(p)
	if np == "hooks.json" {
		pinned := pinAntigravityHooks(f, scope, home)
		return append(native, plugin.Placement{Dest: filepath.Join(nativeBase, "hooks.json"), File: pinned}), shared
	}
	if strings.HasPrefix(np, "rules/") {
		return append(native, plugin.Placement{Dest: filepath.Join(nativeBase, filepath.FromSlash(np)), File: f}), shared
	}
	if strings.HasPrefix(np, "skills/") || strings.HasPrefix(np, "agents/") {
		rel := filepath.FromSlash(np)
		native = append(native, plugin.Placement{Dest: filepath.Join(nativeBase, rel), File: f})
		shared = append(shared, plugin.Placement{Dest: filepath.Join(home, rel), File: f})
		return native, shared
	}
	return native, append(shared, plugin.Placement{Dest: filepath.Join(home, filepath.FromSlash(p)), File: f})
}

func antigravityNormalize(p string) string {
	trimmed := strings.TrimPrefix(p, ".agents/")
	if trimmed == p {
		return p
	}
	if trimmed == "hooks.json" || strings.HasPrefix(trimmed, "skills/") || strings.HasPrefix(trimmed, "agents/") || strings.HasPrefix(trimmed, "rules/") {
		return trimmed
	}
	return p
}

func pinAntigravityHooks(f plugin.File, scope, home string) plugin.File {
	shim := filepath.ToSlash(filepath.Join(home, "bin", "loops"))
	if scope == ScopeProject {
		shim = ".loops/plugin/bin/loops"
	}
	return rewriteHookShim(f, shim)
}

func planGeneric(target plugin.Platform, files []plugin.File, scope, base string) ([]plugin.Placement, []string, error) {
	home := PluginHome(scope, base)
	skillDir := skillDirs[target]
	nativeSkills := hasNativeSkills(files)

	var native, shared []plugin.Placement
	for _, f := range files {
		p := cleanRel(f.Path)
		if p == "" || p == ".loops-generated" {
			continue
		}
		if rel, ok := nativeRel(p, scope); ok {
			f = pinHookShim(target, f, scope, home)
			native = append(native, plugin.Placement{Dest: filepath.Join(base, filepath.FromSlash(rel)), File: f})
			continue
		}
		shared = append(shared, plugin.Placement{Dest: filepath.Join(home, filepath.FromSlash(p)), File: f})
		if rest, ok := strings.CutPrefix(p, "skills/"); ok && !nativeSkills {
			native = append(native, plugin.Placement{Dest: filepath.Join(base, filepath.FromSlash(skillDir), filepath.FromSlash(rest)), File: f})
		}
	}
	return append(native, shared...), installNotes(target, home), nil
}

func hasNativeSkills(files []plugin.File) bool {
	for _, f := range files {
		top, rest, _ := strings.Cut(cleanRel(f.Path), "/")
		if nativeDirs[top] != "" && strings.HasPrefix(rest, "skills/") {
			return true
		}
	}
	return false
}

func nativeRel(p, scope string) (string, bool) {
	top, rest, _ := strings.Cut(p, "/")
	if rest == "" {
		return "", false
	}
	userRel := nativeDirs[top]
	if userRel == "" {
		return "", false
	}
	if scope == ScopeUser {
		return path.Join(userRel, rest), true
	}
	return p, true
}

func pinHookShim(target plugin.Platform, f plugin.File, scope, home string) plugin.File {
	hs, ok := hookShims[target]
	if !ok || cleanRel(f.Path) != hs.file {
		return f
	}
	// hook commands call `loops` from PATH; pin them to the plugin home's shim.
	shim := filepath.ToSlash(filepath.Join(home, "bin", "loops"))
	if scope == ScopeProject {
		shim = hs.projectShim // no machine-specific path in a committed file
	}
	return rewriteHookShim(f, shim)
}

func rewriteHookShim(f plugin.File, shim string) plugin.File {
	repl := `"\"` + shim + `\" hook`
	r := strings.NewReplacer(`"./bin/loops hook`, repl, `"loops hook`, repl)
	f.Data = []byte(r.Replace(string(f.Data)))
	return f
}

func installNotes(target plugin.Platform, home string) []string {
	notes := []string{
		fmt.Sprintf("%s: plugin home is %s; `loops path` finds it, or set LOOPS_PLUGIN_ROOT to it", target, home),
	}
	if note, ok := platformNotes[target]; ok {
		notes = append(notes, note)
	}
	return append(notes,
		"make sure `loops` is on PATH, or call "+filepath.Join(home, "bin", "loops")+" (it downloads the matching release binary on first use; no Go needed)",
		"update later with: curl -fsSL https://raw.githubusercontent.com/arxdsilva/ai-loops/main/install.sh | sh -s -- --target "+string(target)+" --force",
	)
}

func cleanRel(p string) string {
	p = path.Clean(filepath.ToSlash(p))
	if p == "." || path.IsAbs(p) || p == ".." || strings.HasPrefix(p, "../") {
		return ""
	}
	return p
}
