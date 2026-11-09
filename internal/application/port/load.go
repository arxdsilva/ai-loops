package port

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
)

// eventOrder is the canonical order of hook events (Load sorts by it).
var eventOrder = []plugin.Event{
	plugin.EventSessionStart,
	plugin.EventPreToolUse,
	plugin.EventPostToolUse,
	plugin.EventPostToolUseFailure,
	plugin.EventSubagentStart,
	plugin.EventSubagentStop,
	plugin.EventStop,
}

func eventRank(e plugin.Event) int {
	for i, x := range eventOrder {
		if x == e {
			return i
		}
	}
	return -1
}

func loadPlugin(src fs.FS) (plugin.Plugin, error) {
	const name = ".claude-plugin/plugin.json"
	data, err := fs.ReadFile(src, name)
	if err != nil {
		return plugin.Plugin{}, fmt.Errorf("port: %w", err)
	}
	var m struct {
		Name        string          `json:"name"`
		Version     string          `json:"version"`
		Description string          `json:"description"`
		Author      json.RawMessage `json:"author"`
		Homepage    string          `json:"homepage"`
		Repository  json.RawMessage `json:"repository"`
		License     string          `json:"license"`
		Keywords    []string        `json:"keywords"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return plugin.Plugin{}, fmt.Errorf("port: %s: %w", name, err)
	}
	if m.Name == "" || m.Version == "" {
		return plugin.Plugin{}, fmt.Errorf("port: %s: name and version are required", name)
	}
	author, err := nameOrString(m.Author, "name")
	if err != nil {
		return plugin.Plugin{}, fmt.Errorf("port: %s: author: %w", name, err)
	}
	repo, err := nameOrString(m.Repository, "url")
	if err != nil {
		return plugin.Plugin{}, fmt.Errorf("port: %s: repository: %w", name, err)
	}
	return plugin.Plugin{
		Name: m.Name, Version: m.Version, Description: m.Description,
		Author: author, Homepage: m.Homepage, Repository: repo,
		License: m.License, Keywords: m.Keywords,
	}, nil
}

// nameOrString decodes a JSON string or an object's field (author.name,
// repository.url).
func nameOrString(raw json.RawMessage, field string) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", fmt.Errorf("want a string or an object")
	}
	v, _ := obj[field].(string)
	return v, nil
}

func loadSkills(src fs.FS) ([]plugin.Skill, error) {
	entries, err := fs.ReadDir(src, "skills")
	if err != nil {
		return nil, fmt.Errorf("port: %w", err)
	}
	var skills []plugin.Skill
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := "skills/" + e.Name()
		file := dir + "/SKILL.md"
		data, err := fs.ReadFile(src, file)
		if err != nil {
			return nil, fmt.Errorf("port: %w", err)
		}
		fm, _, err := parseFrontmatter(file, data)
		if err != nil {
			return nil, fmt.Errorf("port: %w", err)
		}
		if fm["name"] != e.Name() {
			return nil, fmt.Errorf("port: %s: name %q does not match the directory %q", file, fm["name"], e.Name())
		}
		if fm["description"] == "" {
			return nil, fmt.Errorf("port: %s: description is required", file)
		}
		var files []string
		err = fs.WalkDir(src, dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				files = append(files, p)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("port: %s: %w", dir, err)
		}
		sort.Strings(files)
		skills = append(skills, plugin.Skill{
			Name: fm["name"], Description: fm["description"], ArgumentHint: fm["argument-hint"],
			Command: commandSkill(fm["name"]), Dir: dir, Files: files,
		})
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skills, nil
}

func loadAgents(src fs.FS) ([]plugin.Agent, error) {
	entries, err := fs.ReadDir(src, "agents")
	if err != nil {
		return nil, fmt.Errorf("port: %w", err)
	}
	var agents []plugin.Agent
	for _, e := range entries {
		if e.IsDir() || path.Ext(e.Name()) != ".md" {
			continue
		}
		file := "agents/" + e.Name()
		data, err := fs.ReadFile(src, file)
		if err != nil {
			return nil, fmt.Errorf("port: %w", err)
		}
		fm, body, err := parseFrontmatter(file, data)
		if err != nil {
			return nil, fmt.Errorf("port: %w", err)
		}
		a, err := agentFrom(file, fm, body)
		if err != nil {
			return nil, fmt.Errorf("port: %w", err)
		}
		if a.Name != strings.TrimSuffix(e.Name(), ".md") {
			return nil, fmt.Errorf("port: %s: name %q does not match the file name", file, a.Name)
		}
		agents = append(agents, a)
	}
	sort.Slice(agents, func(i, j int) bool { return agents[i].Name < agents[j].Name })
	return agents, nil
}

func agentFrom(file string, fm frontmatter, body string) (plugin.Agent, error) {
	a := plugin.Agent{Name: fm["name"], Description: fm["description"], Color: fm["color"], Body: body}
	if a.Name == "" || a.Description == "" {
		return a, fmt.Errorf("%s: name and description are required", file)
	}
	switch t := plugin.Tier(fm["model"]); t {
	case "":
		a.Tier = plugin.TierInherit
	case plugin.TierHaiku, plugin.TierSonnet, plugin.TierOpus, plugin.TierInherit:
		a.Tier = t
	default:
		return a, fmt.Errorf("%s: unknown model tier %q (want haiku, sonnet, opus or inherit)", file, t)
	}
	var err error
	if a.Tools, err = list(fm["tools"]); err != nil {
		return a, fmt.Errorf("%s: tools: %w", file, err)
	}
	if a.DisallowedTools, err = list(fm["disallowedTools"]); err != nil {
		return a, fmt.Errorf("%s: disallowedTools: %w", file, err)
	}
	for _, t := range a.DisallowedTools {
		if t == "Edit" {
			a.ReadOnly = true
		}
	}
	return a, nil
}

func loadHooks(src fs.FS) ([]plugin.Hook, error) {
	const name = "hooks/hooks.json"
	data, err := fs.ReadFile(src, name)
	if err != nil {
		return nil, fmt.Errorf("port: %w", err)
	}
	var cfg struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
				Timeout int    `json:"timeout"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("port: %s: %w", name, err)
	}
	var hooks []plugin.Hook
	for ev, groups := range cfg.Hooks {
		event := plugin.Event(ev)
		if eventRank(event) < 0 {
			return nil, fmt.Errorf("port: %s: unknown event %q", name, ev)
		}
		for _, g := range groups {
			matcher := g.Matcher
			if matcher == "*" {
				matcher = ""
			}
			for _, h := range g.Hooks {
				if h.Type != "command" {
					return nil, fmt.Errorf("port: %s: %s: unsupported hook type %q", name, ev, h.Type)
				}
				hn, err := hookName(h.Command)
				if err != nil {
					return nil, fmt.Errorf("port: %s: %s: %w", name, ev, err)
				}
				hooks = append(hooks, plugin.Hook{Event: event, Matcher: matcher, Name: hn, TimeoutSec: h.Timeout})
			}
		}
	}
	// Stable sort by event rank keeps the file's group order within an event.
	sort.SliceStable(hooks, func(i, j int) bool { return eventRank(hooks[i].Event) < eventRank(hooks[j].Event) })
	return hooks, nil
}

// hookName extracts <name> from a `… hook <name> [flags]` command.
func hookName(cmd string) (string, error) {
	f := strings.Fields(cmd)
	for i := 0; i+1 < len(f); i++ {
		if f[i] == "hook" && !strings.HasPrefix(f[i+1], "-") {
			return f[i+1], nil
		}
	}
	return "", fmt.Errorf("command %q does not call `loops hook <name>`", cmd)
}
