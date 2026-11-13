package port

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
)

// geminiReadOnlyTools is every Gemini CLI built-in tool except `replace`
// (write_file is kept so roles can write their own run-dir file); names from
// https://geminicli.com/docs/reference/tools/ (read 2026-11-09).
var geminiReadOnlyTools = []string{
	"activate_skill", "ask_user", "get_internal_docs", "glob", "google_web_search",
	"grep_search", "list_directory", "list_mcp_resources", "read_file",
	"read_many_files", "read_mcp_resource", "run_shell_command", "web_fetch",
	"write_file", "write_todos",
}

// geminiEvents maps canonical events to Gemini's; subagent events and
// PostToolUseFailure have no Gemini counterpart (failures arrive in AfterTool).
var geminiEvents = map[plugin.Event]string{
	plugin.EventSessionStart: "SessionStart",
	plugin.EventPreToolUse:   "BeforeTool",
	plugin.EventPostToolUse:  "AfterTool",
	plugin.EventStop:         "AfterAgent",
}

const geminiReadme = `# loops for Gemini CLI

Tier B (subagents, no resume): loop-backs spawn a new agent of the same role
(specs/PLATFORMS.md section 4).

## Install

    gemini extensions install https://github.com/arxdsilva/ai-loops --ref <tag>

If your Gemini CLI cannot install from a subdirectory, clone the repository and run:

    gemini extensions install ./ai-loops/platforms/gemini

For development, symlink the package so edits apply without reinstalling:

    gemini extensions link ./platforms/gemini

## Use

- /loops:work <task>, /loops:init, /loops:status, /loops:report
- Role subagents live in agents/ (kind: local). Read-only roles get a tool
  allowlist without write_file or replace.
- Hooks (hooks/hooks.json) run bin/loops with --platform gemini. Timeouts are in
  milliseconds. Gemini has no subagent events.

## Models

Roles inherit the session model unless the package was built with
` + "`loops port build --models <file.json>`" + ` mapping tiers to Gemini model ids.
`

const readOnlyLine = "You may write only your own file under the run dir; never edit repo code.\n\n"

// buildGemini emits platforms/gemini/ (specs/PLATFORMS.md §3.4).
func buildGemini(p plugin.Plugin, src fs.FS, o Options) ([]plugin.File, []string, error) {
	files, err := baseFiles(p, src, plugin.Gemini, o)
	if err != nil {
		return nil, nil, err
	}
	var warns []string

	manifest := map[string]string{
		"name":            p.Name,
		"version":         p.Version,
		"description":     p.Description,
		"contextFileName": "GEMINI.md",
	}
	files = append(files, emitJSON("gemini-extension.json", manifest))

	ctx := strings.TrimRight(contextText(src), "\n")
	if ctx != "" {
		ctx += "\n\n"
	}
	ctx += "Run the loops workflow with `/loops:work <task>` or by activating the `work` skill (skills/work/SKILL.md).\n"
	files = append(files, plugin.File{Path: "GEMINI.md", Data: []byte(ctx)})

	for _, a := range p.Agents {
		fm := [][2]string{
			{"name", a.Name},
			{"description", quoteStr(a.Description)},
			{"kind", "local"},
		}
		if m := model(o, a.Tier); m != "" {
			fm = append(fm, [2]string{"model", quoteStr(m)})
		}
		extra := ""
		if a.ReadOnly {
			extra = "tools:\n"
			for _, t := range geminiReadOnlyTools {
				extra += "  - " + t + "\n"
			}
			warns = append(warns, fmt.Sprintf("gemini: role %s is read-only but keeps write_file (own run-dir file) and run_shell_command, so edits are only prevented by the prompt", a.Name))
		} else if len(a.Tools) > 0 {
			warns = append(warns, fmt.Sprintf("gemini: role %s Claude tool allowlist not mapped; it inherits the session tools", a.Name))
		}
		files = append(files, plugin.File{
			Path: "agents/" + a.Name + ".md",
			Data: []byte(fmBlock(fm, extra) + roleBody(a)),
		})
	}

	for _, s := range commandSkills(p) {
		toml := fmt.Sprintf("description = %s\nprompt = %s\n",
			quoteStr(s.Description),
			quoteStr("Activate the "+s.Name+" skill and follow it. Arguments: {{args}}"))
		files = append(files, plugin.File{Path: "commands/loops/" + s.Name + ".toml", Data: []byte(toml)})
	}

	files = append(files, emitJSON("hooks/hooks.json", geminiHooks(p)))
	if hasEvent(p, plugin.EventSubagentStart) || hasEvent(p, plugin.EventSubagentStop) {
		warns = append(warns, "gemini: no subagent events; lanes come from `loops run label` only")
	}
	if hasEvent(p, plugin.EventPostToolUseFailure) {
		warns = append(warns, "gemini: PostToolUseFailure has no event; failures are delivered through AfterTool")
	}

	files = append(files, plugin.File{Path: "README.md", Data: []byte(geminiReadme)})
	sortByPath(files)
	return files, warns, nil
}

func geminiHooks(p plugin.Plugin) map[string]any {
	groups := map[string][]any{}
	seen := map[string]bool{}
	for _, h := range p.Hooks {
		ev, ok := geminiEvents[h.Event]
		if !ok && h.Event != plugin.EventPostToolUseFailure {
			continue
		}
		if h.Event == plugin.EventPostToolUseFailure {
			ev = "AfterTool"
		}
		key := ev + "/" + h.Name
		if seen[key] {
			continue
		}
		seen[key] = true
		groups[ev] = append(groups[ev], map[string]any{
			"hooks": []any{map[string]any{
				"type":    "command",
				"name":    h.Name,
				"command": `"${extensionPath}/bin/loops" hook ` + h.Name + " --platform gemini",
				"timeout": h.TimeoutSec * 1000,
			}},
		})
	}
	return map[string]any{"hooks": groups}
}

// emitJSON renders v as indented JSON (map keys sorted: deterministic).
func emitJSON(path string, v any) plugin.File {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil { // only plain maps/strings/ints are passed
		panic(err)
	}
	return plugin.File{Path: path, Data: append(b, '\n')}
}

// quoteStr renders s as a double-quoted string valid in both YAML and TOML.
func quoteStr(s string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimRight(buf.String(), "\n")
}

// fmBlock renders YAML frontmatter from ordered key/value pairs (values
// already quoted) plus raw extra lines, then a blank line.
func fmBlock(kv [][2]string, extra string) string {
	var b strings.Builder
	b.WriteString("---\n")
	for _, p := range kv {
		b.WriteString(p[0] + ": " + p[1] + "\n")
	}
	b.WriteString(extra)
	b.WriteString("---\n\n")
	return b.String()
}

func sortByPath(fs []plugin.File) {
	sort.SliceStable(fs, func(i, j int) bool { return fs[i].Path < fs[j].Path })
}

func hasEvent(p plugin.Plugin, e plugin.Event) bool {
	for _, h := range p.Hooks {
		if h.Event == e {
			return true
		}
	}
	return false
}

// toAgentsSkills moves skills/... to .agents/skills/... (and, for each
// extra root, also copies them there) for packages laid out for placement.
func toAgentsSkills(in []plugin.File, extraRoots ...string) []plugin.File {
	var out []plugin.File
	for _, f := range in {
		rest, ok := strings.CutPrefix(f.Path, "skills/")
		if !ok {
			out = append(out, f)
			continue
		}
		for _, root := range append([]string{".agents/skills/"}, extraRoots...) {
			c := f
			c.Path = root + rest
			out = append(out, c)
		}
	}
	return out
}

// commandSkills returns the skills users invoke as slash commands.
func commandSkills(p plugin.Plugin) []plugin.Skill {
	var out []plugin.Skill
	for _, s := range p.Skills {
		if s.Command || commandSkill(s.Name) {
			out = append(out, s)
		}
	}
	return out
}

// roleBody is the agent body, with the read-only write rule prepended for
// read-only roles.
func roleBody(a plugin.Agent) string {
	if a.ReadOnly {
		return readOnlyLine + a.Body
	}
	return a.Body
}

// roleFiles ships agents/<role>.md at the package root (name, description,
// intact body) so `loops path agents` resolves <role>.md on tier C/D platforms.
func roleFiles(p plugin.Plugin) []plugin.File {
	var out []plugin.File
	for _, a := range p.Agents {
		fm := [][2]string{{"name", a.Name}, {"description", quoteStr(a.Description)}}
		out = append(out, plugin.File{Path: "agents/" + a.Name + ".md", Data: []byte(fmBlock(fm, "") + a.Body)})
	}
	return out
}
