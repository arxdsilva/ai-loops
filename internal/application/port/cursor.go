package port

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
)

// cursorRoot is the plugin-root variable Cursor sets for plugin hooks.
const cursorRoot = "${CURSOR_PLUGIN_ROOT}"

// cursorEvents maps canonical events to Cursor's camelCase hook events.
var cursorEvents = map[plugin.Event]string{
	plugin.EventSessionStart:       "sessionStart",
	plugin.EventPreToolUse:         "preToolUse",
	plugin.EventPostToolUse:        "postToolUse",
	plugin.EventPostToolUseFailure: "postToolUseFailure",
	plugin.EventSubagentStart:      "subagentStart",
	plugin.EventSubagentStop:       "subagentStop",
	plugin.EventStop:               "stop",
}

// cursorStopLoopLimit caps how often the stop gate may send the agent back.
const cursorStopLoopLimit = 5

// cursorBackground says which roles run in background (parallel waves)
// and which block the lead (ROLES.md phases): researchers, implementers,
// docs and QA run as waves; planning, contract, product and reflection
// are single blocking agents.
var cursorBackground = map[string]bool{
	"web-researcher":       true,
	"practices-researcher": true,
	"code-researcher":      true,
	"hardware-researcher":  true,
	"planner":              false,
	"plan-reviewer":        false,
	"architect":            false,
	"domain-engineer":      true,
	"application-engineer": true,
	"api-engineer":         true,
	"persistence-engineer": true,
	"integration-engineer": true,
	"test-engineer":        true,
	"docs-writer":          true,
	"qa-verifier":          true,
	"product-reviewer":     false,
	"reflector":            false,
}

// cursorManualSkills are the commands the model must not invoke on its own.
var cursorManualSkills = map[string]bool{"init": true, "status": true, "report": true}

// buildCursor emits platforms/cursor/ (specs/PLATFORMS.md §3.3).
func buildCursor(p plugin.Plugin, src fs.FS, o Options) ([]plugin.File, []string, error) {
	files, err := baseFiles(p, src, plugin.Cursor, o)
	if err != nil {
		return nil, nil, err
	}
	var warns []string

	m := map[string]any{"name": p.Name, "version": p.Version, "description": p.Description}
	addMeta(m, p)
	manifest, err := jsonFile(".cursor-plugin/plugin.json", m)
	if err != nil {
		return nil, nil, err
	}
	files = append(files, manifest)

	var allow, unknown, readOnly []string
	for _, a := range p.Agents {
		if a.ReadOnly {
			readOnly = append(readOnly, a.Name)
		}
		bg, ok := cursorBackground[a.Name]
		if !ok {
			unknown = append(unknown, a.Name)
		}
		files = append(files, plugin.File{Path: "agents/" + a.Name + ".md", Data: []byte(cursorAgent(a, o, bg))})
		if len(a.Tools) > 0 || (len(a.DisallowedTools) > 0 && !a.ReadOnly) {
			allow = append(allow, a.Name)
		}
	}
	if len(allow) > 0 {
		warns = append(warns, "cursor: tool lists dropped for "+strings.Join(allow, ", "))
	}
	if len(readOnly) > 0 {
		warns = append(warns, "cursor: readonly: true would block each role's own run-dir file, so read-only is an instruction only for "+strings.Join(readOnly, ", "))
	}
	if len(unknown) > 0 {
		warns = append(warns, "cursor: no phase for "+strings.Join(unknown, ", ")+"; is_background: false")
	}

	for _, s := range p.Skills {
		if !s.Command {
			continue
		}
		name := s.Dir + "/SKILL.md"
		data, err := fs.ReadFile(src, name)
		if err != nil {
			return nil, nil, fmt.Errorf("port: %w", err)
		}
		out, err := setFrontmatterKey(name, data, "disable-model-invocation", "true", cursorManualSkills[s.Name])
		if err != nil {
			return nil, nil, fmt.Errorf("port: %w", err)
		}
		files = putFile(files, plugin.File{Path: name, Data: out})
	}

	hf, err := jsonFile("hooks/hooks.json", map[string]any{"version": 1, "hooks": cursorHooks(p)})
	if err != nil {
		return nil, nil, err
	}
	files = append(files, hf)

	ctx := contextText(src)
	if strings.TrimSpace(ctx) == "" {
		warns = append(warns, "cursor: templates/PLUGIN_CONTEXT.md is empty; rules/loops.mdc has no plugin context")
	}
	rule := "---\ndescription: " + yamlString("loops plugin context: how to run the loops workflow and its `loops` binary") + "\nalwaysApply: true\n---\n\n" + ctx
	files = append(files,
		plugin.File{Path: "rules/loops.mdc", Data: []byte(rule)},
		plugin.File{Path: "README.md", Data: []byte(cursorReadme(p, o))},
	)
	sortFiles(files)
	return files, warns, nil
}

// cursorAgent renders a Cursor subagent: frontmatter then the role body.
// Every agent is readonly: false because each role writes its own run-dir
// file; read-only roles get readOnlyNote at the top of the body instead.
func cursorAgent(a plugin.Agent, o Options, background bool) string {
	body := a.Body
	if a.ReadOnly {
		body = readOnlyNote + "\n\n" + body
	}
	m := model(o, a.Tier)
	if m == "" {
		m = "inherit"
	}
	return "---\n" +
		"name: " + a.Name + "\n" +
		"description: " + yamlString(a.Description) + "\n" +
		"model: " + yamlString(m) + "\n" +
		"readonly: false\n" +
		fmt.Sprintf("is_background: %t\n", background) +
		"---\n\n" + body
}

type cursorHook struct {
	Command   string `json:"command"`
	Timeout   int    `json:"timeout,omitempty"`
	LoopLimit int    `json:"loop_limit,omitempty"`
}

// cursorHooks maps the canonical hooks to Cursor's event map. Cursor
// entries carry no matcher (tool names differ); the binary filters.
func cursorHooks(p plugin.Plugin) map[string][]cursorHook {
	out := map[string][]cursorHook{}
	byEvent := hooksByEvent(p.Hooks)
	for _, ev := range eventOrder {
		for _, h := range byEvent[ev] {
			c := cursorHook{Command: hookCommand(cursorRoot, h.Name, plugin.Cursor), Timeout: h.TimeoutSec}
			if ev == plugin.EventStop {
				c.LoopLimit = cursorStopLoopLimit
			}
			out[cursorEvents[ev]] = append(out[cursorEvents[ev]], c)
		}
	}
	return out
}

// setFrontmatterKey sets key: value in a markdown file's frontmatter when
// on is true, and removes the key when on is false; the rest of the file is
// kept byte for byte.
func setFrontmatterKey(name string, data []byte, key, value string, on bool) ([]byte, error) {
	if _, _, err := parseFrontmatter(name, data); err != nil {
		return nil, err
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.SplitAfter(text, "\n")
	end := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---\n" || lines[i] == "---" {
			end = i
			break
		}
	}
	var out []string
	out = append(out, lines[0])
	for _, l := range lines[1:end] {
		if k, _, ok := strings.Cut(l, ":"); ok && k == key {
			continue
		}
		out = append(out, l)
	}
	if on {
		out = append(out, key+": "+value+"\n")
	}
	out = append(out, lines[end:]...)
	return []byte(strings.Join(out, "")), nil
}

func cursorReadme(p plugin.Plugin, o Options) string {
	return fmt.Sprintf(`# %[1]s for Cursor

Generated by `+"`loops port build`"+` from the canonical plugin (repository root). Do not edit; see specs/PLATFORMS.md §3.3.

## Install

- **Marketplace:** the repository root holds `+"`.cursor-plugin/marketplace.json`"+`, which points at `+"`platforms/cursor`"+`. Install `+"`%[1]s`"+` from the Cursor marketplace, or import the repository into a team marketplace.
- **Local development:** copy this directory to `+"`~/.cursor/plugins/local/%[1]s`"+` and restart Cursor.

Commands: `+"`/work`"+` (also model-invocable), `+"`/init`, `/status`, `/report`"+` (manual only).

## What is in the package

- `+"`agents/`"+`: the role subagents (`+"`is_background`"+` for roles that run in parallel waves). No role is `+"`readonly`"+`: every role writes its own run-dir file, so read-only roles are told in their instructions not to edit repo code.
- `+"`skills/`"+`, `+"`specs/`"+`, `+"`templates/`"+`, `+"`ROLES.md`"+`: the canonical workflow.
- `+"`hooks/hooks.json`"+`: session context, read and model guards, post-edit gofmt, tracing, subagent lanes and the stop gate (`+"`loop_limit: %[5]d`"+`).
- `+"`rules/loops.mdc`"+`: the always-on plugin context.

## Binary

`+"`bin/loops`"+` uses `+"`loops`"+` from PATH when it reports version %[2]s, else a cached build, else runs `+"`go install %[3]s/cmd/loops@%[4]s`"+` on first use (Go required; the module must be reachable).

## Caveats

- Tool allowlists are not expressible, and read-only roles are restricted by instruction only.
- Models: roles use `+"`inherit`"+` unless `+"`loops port build --models <file.json>`"+` maps the tiers.
`, p.Name, strings.TrimPrefix(o.Version, "v"), o.Module, o.Version, cursorStopLoopLimit)
}

// cursorMarketplace is the repo-root .cursor-plugin/marketplace.json.
func cursorMarketplace(p plugin.Plugin) plugin.File {
	owner := p.Author
	if owner == "" {
		owner = p.Name
	}
	f, _ := jsonFile(".cursor-plugin/marketplace.json", map[string]any{
		"name":     "ai-loops",
		"owner":    map[string]any{"name": owner},
		"metadata": map[string]any{"description": p.Description, "version": p.Version},
		"plugins": []any{map[string]any{
			"name":        p.Name,
			"source":      "platforms/cursor",
			"description": p.Description,
			"version":     p.Version,
		}},
	})
	return f
}
