package port

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
)

func TestCodexManifest(t *testing.T) {
	files, _ := build(t, plugin.Codex, Options{})
	var m struct {
		Schema      string `json:"$schema"`
		Name        string
		Version     string
		Description string
		Author      struct{ Name string }
		Homepage    string
		Repository  string
		License     string
		Keywords    []string
		Extensions  map[string]map[string]any
	}
	mustJSON(t, files, "plugin.json", &m)
	if m.Schema != "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json" || m.Name != "loops" || m.Version != "0.2.0" ||
		m.Description != "Desc <x> & y" || m.Author.Name != "Ann" || m.Homepage != "https://h" || m.Repository != "https://r" ||
		m.License != "MIT" || !reflect.DeepEqual(m.Keywords, []string{"go", "qa"}) {
		t.Fatalf("manifest = %+v", m)
	}
	if m.Extensions["com.openai"]["hooks"] != "./hooks/hooks.json" {
		t.Fatalf("extensions = %+v", m.Extensions)
	}
	if !strings.Contains(mustFile(t, files, "plugin.json"), "<x> & y") {
		t.Error("manifest HTML-escaped")
	}
}

type claudeHooks struct {
	Hooks map[string][]struct {
		Matcher string
		Hooks   []struct {
			Type    string
			Command string
			Timeout int
		}
	}
}

func TestCodexHooks(t *testing.T) {
	files, warns := build(t, plugin.Codex, Options{})
	var h claudeHooks
	mustJSON(t, files, "hooks/hooks.json", &h)
	var events []string
	for ev := range h.Hooks {
		events = append(events, ev)
	}
	if _, ok := h.Hooks["PostToolUseFailure"]; ok {
		t.Error("Codex has no PostToolUseFailure event")
	}
	if len(h.Hooks) != 6 {
		t.Fatalf("events = %v", events)
	}
	names := map[string][]string{}
	for ev, groups := range h.Hooks {
		for _, g := range groups {
			tool := ev == "PreToolUse" || ev == "PostToolUse"
			if tool && g.Matcher != "*" || !tool && g.Matcher != "" {
				t.Errorf("%s matcher = %q", ev, g.Matcher)
			}
			for _, x := range g.Hooks {
				if x.Type != "command" || x.Timeout == 0 || !strings.HasPrefix(x.Command, `"${PLUGIN_ROOT}/bin/loops" hook `) || !strings.HasSuffix(x.Command, " --platform codex") {
					t.Errorf("%s handler = %+v", ev, x)
				}
				names[ev] = append(names[ev], strings.Fields(x.Command)[2])
			}
		}
	}
	if !reflect.DeepEqual(names["PreToolUse"], []string{"read-guard", "model-guard"}) || !reflect.DeepEqual(names["PostToolUse"], []string{"go-post-edit", "trace"}) ||
		!reflect.DeepEqual(names["Stop"], []string{"stop-gate"}) {
		t.Errorf("handlers = %v", names)
	}
	if !containsAll(warns, "PostToolUseFailure", "planner", "scout") {
		t.Errorf("warnings = %q", warns)
	}
}

func containsAll(warns []string, subs ...string) bool {
	all := strings.Join(warns, "\n")
	for _, s := range subs {
		if !strings.Contains(all, s) {
			return false
		}
	}
	return true
}

func TestCodexAgents(t *testing.T) {
	p, _ := loadFixture(t)
	files, _ := build(t, plugin.Codex, Options{Models: map[plugin.Tier]string{plugin.TierOpus: "gpt-x"}})
	for _, a := range p.Agents {
		name := "agents/" + a.Name + ".toml"
		got, err := parseTOML(mustFile(t, files, name))
		if err != nil {
			t.Fatalf("%s: %v\n%s", name, err, mustFile(t, files, name))
		}
		if got["name"] != a.Name || got["description"] != a.Description || !strings.HasSuffix(got["developer_instructions"], a.Body) {
			t.Errorf("%s = %#v", name, got)
		}
		_, hasModel := got["model"]
		if hasModel != (a.Tier == plugin.TierOpus) || hasModel && got["model"] != "gpt-x" {
			t.Errorf("%s model = %q (tier %s)", name, got["model"], a.Tier)
		}
		if ro := strings.HasPrefix(got["developer_instructions"], readOnlyNote+"\n\n"); ro != a.ReadOnly {
			t.Errorf("%s read-only note = %v, want %v", name, ro, a.ReadOnly)
		}
	}
}

func TestCodexReadme(t *testing.T) {
	files, _ := build(t, plugin.Codex, Options{Version: "v9.9.9"})
	r := mustFile(t, files, "README.md")
	for _, want := range []string{"codex plugin marketplace add arxdsilva/ai-loops", "loops install --target codex", "/hooks", "$work", "go install github.com/arxdsilva/ai-loops/cmd/loops@v9.9.9", "version 9.9.9"} {
		if !strings.Contains(r, want) {
			t.Errorf("README lacks %q", want)
		}
	}
	if !strings.Contains(mustFile(t, files, "bin/loops"), "V='9.9.9'") {
		t.Error("shim ignores Options.Version")
	}
}

func TestCursorManifest(t *testing.T) {
	files, _ := build(t, plugin.Cursor, Options{})
	var m map[string]any
	mustJSON(t, files, ".cursor-plugin/plugin.json", &m)
	for _, k := range []string{"name", "description", "version", "author", "homepage", "repository", "license", "keywords"} {
		if _, ok := m[k]; !ok {
			t.Errorf("manifest lacks %s", k)
		}
	}
	if m["name"] != "loops" || m["author"].(map[string]any)["name"] != "Ann" {
		t.Errorf("manifest = %v", m)
	}
}

func TestCursorAgents(t *testing.T) {
	p, _ := loadFixture(t)
	files, warns := build(t, plugin.Cursor, Options{Models: map[plugin.Tier]string{plugin.TierSonnet: "composer-2"}})
	bg := map[string]string{"planner": "false", "domain-engineer": "true", "scout": "false"}
	for _, a := range p.Agents {
		name := "agents/" + a.Name + ".md"
		fm, body, err := parseFrontmatter(name, []byte(mustFile(t, files, name)))
		if err != nil {
			t.Fatal(err)
		}
		wantModel := "inherit"
		if a.Tier == plugin.TierSonnet {
			wantModel = "composer-2"
		}
		// readonly: true would block the run-dir file every role writes
		// (research/*.md, plan.v*.md, qa/*.md), so read-only is an instruction.
		want := frontmatter{"name": a.Name, "description": a.Description, "model": wantModel,
			"readonly": "false", "is_background": bg[a.Name]}
		wantBody := a.Body
		if a.ReadOnly {
			wantBody = readOnlyNote + "\n\n" + a.Body
		}
		if !reflect.DeepEqual(fm, want) || body != wantBody {
			t.Errorf("%s = %#v body %q", name, fm, body)
		}
	}
	if !strings.Contains(readOnlyNote, "own file under the run dir") || !strings.Contains(readOnlyNote, "never edit repo code") {
		t.Errorf("readOnlyNote = %q", readOnlyNote)
	}
	if !containsAll(warns, "read-only is an instruction only for planner") || containsAll(warns, "for domain-engineer") {
		t.Errorf("read-only warning = %q", warns)
	}
	if !containsAll(warns, "scout") || !strings.Contains(strings.Join(warns, "\n"), "no phase for scout") {
		t.Errorf("warnings = %q", warns)
	}
}

func TestCursorBackgroundTable(t *testing.T) {
	// Lead decision: parallel-wave roles run in background.
	for role, want := range map[string]bool{
		"web-researcher": true, "practices-researcher": true, "code-researcher": true, "hardware-researcher": true,
		"domain-engineer": true, "application-engineer": true, "api-engineer": true,
		"persistence-engineer": true, "integration-engineer": true, "test-engineer": true,
		"docs-writer": true, "qa-verifier": true,
		"planner": false, "plan-reviewer": false, "architect": false, "product-reviewer": false, "reflector": false,
	} {
		if got, ok := cursorBackground[role]; !ok || got != want {
			t.Errorf("%s background = %v (known %v), want %v", role, got, ok, want)
		}
	}
	if len(cursorBackground) != 17 {
		t.Errorf("table has %d roles, want 17", len(cursorBackground))
	}
}

func TestCursorSkills(t *testing.T) {
	_, src := loadFixture(t)
	files, _ := build(t, plugin.Cursor, Options{})
	for skill, want := range map[string]string{"init": "true", "status": "true", "report": "true", "work": ""} {
		name := "skills/" + skill + "/SKILL.md"
		fm, body, err := parseFrontmatter(name, []byte(mustFile(t, files, name)))
		if err != nil {
			t.Fatal(err)
		}
		if fm["disable-model-invocation"] != want {
			t.Errorf("%s disable-model-invocation = %q, want %q", skill, fm["disable-model-invocation"], want)
		}
		_, srcBody, _ := parseFrontmatter(name, src[name].Data)
		if body != srcBody || fm["name"] != skill {
			t.Errorf("%s body/name changed: %q", skill, body)
		}
	}
	if !bytes.Equal(files["skills/testing/SKILL.md"].Data, src["skills/testing/SKILL.md"].Data) {
		t.Error("non-command skill changed")
	}
	if _, ok := files["skills/work/platforms/cursor.md"]; !ok {
		t.Error("cursor platform notes missing")
	}
	if _, ok := files["skills/work/platforms/codex.md"]; ok {
		t.Error("codex platform notes leaked into cursor")
	}
}

func TestCursorHooks(t *testing.T) {
	files, _ := build(t, plugin.Cursor, Options{})
	var h struct {
		Version int
		Hooks   map[string][]map[string]any
	}
	mustJSON(t, files, "hooks/hooks.json", &h)
	if h.Version != 1 {
		t.Fatalf("version = %d", h.Version)
	}
	want := map[string][]string{
		"sessionStart": {"session-start"}, "preToolUse": {"read-guard", "model-guard"}, "postToolUse": {"go-post-edit", "trace"},
		"postToolUseFailure": {"trace"}, "subagentStart": {"trace-agent"}, "subagentStop": {"trace-agent"}, "stop": {"stop-gate"},
	}
	got := map[string][]string{}
	for ev, entries := range h.Hooks {
		for _, e := range entries {
			cmd, _ := e["command"].(string)
			if !strings.HasPrefix(cmd, `"${CURSOR_PLUGIN_ROOT}/bin/loops" hook `) || !strings.HasSuffix(cmd, " --platform cursor") || e["timeout"] == nil {
				t.Errorf("%s entry = %v", ev, e)
			}
			if _, ok := e["matcher"]; ok {
				t.Errorf("%s has a matcher", ev)
			}
			_, hasLimit := e["loop_limit"]
			if hasLimit != (ev == "stop") || hasLimit && e["loop_limit"] != float64(5) {
				t.Errorf("%s loop_limit = %v", ev, e["loop_limit"])
			}
			got[ev] = append(got[ev], strings.Fields(cmd)[2])
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("hooks = %v", got)
	}
}

func TestCursorRuleAndReadme(t *testing.T) {
	files, warns := build(t, plugin.Cursor, Options{})
	fm, body, err := parseFrontmatter("rules/loops.mdc", []byte(mustFile(t, files, "rules/loops.mdc")))
	if err != nil {
		t.Fatal(err)
	}
	if fm["alwaysApply"] != "true" || fm["description"] == "" || body != "Always-on loops context.\n" {
		t.Errorf("rule = %v %q", fm, body)
	}
	if strings.Contains(strings.Join(warns, "\n"), "PLUGIN_CONTEXT") {
		t.Errorf("unexpected context warning: %q", warns)
	}
	r := mustFile(t, files, "README.md")
	for _, want := range []string{"~/.cursor/plugins/local/loops", ".cursor-plugin/marketplace.json", "/init"} {
		if !strings.Contains(r, want) {
			t.Errorf("README lacks %q", want)
		}
	}
}

func TestCursorEmptyContextWarns(t *testing.T) {
	p, src := loadFixture(t)
	delete(src, "templates/PLUGIN_CONTEXT.md")
	_, warns, err := Build(p, src, plugin.Cursor, Options{})
	if err != nil || !containsAll(warns, "PLUGIN_CONTEXT") {
		t.Fatalf("warns = %q, err = %v", warns, err)
	}
}

func TestCursorMissingSkillFile(t *testing.T) {
	p, src := loadFixture(t)
	delete(src, "skills/init/SKILL.md")
	if _, _, err := Build(p, src, plugin.Cursor, Options{}); err == nil {
		t.Fatal("Build succeeded without skills/init/SKILL.md")
	}
}

func TestSetFrontmatterKey(t *testing.T) {
	doc := []byte("---\nname: x\nflag: false\n---\nbody\n")
	on, err := setFrontmatterKey("d.md", doc, "flag", "true", true)
	if err != nil || string(on) != "---\nname: x\nflag: true\n---\nbody\n" {
		t.Fatalf("on = %q, %v", on, err)
	}
	off, err := setFrontmatterKey("d.md", doc, "flag", "true", false)
	if err != nil || string(off) != "---\nname: x\n---\nbody\n" {
		t.Fatalf("off = %q, %v", off, err)
	}
	if _, err := setFrontmatterKey("d.md", []byte("nope"), "k", "v", true); err == nil {
		t.Fatal("bad frontmatter accepted")
	}
}

func TestReadmeOnlyPackages(t *testing.T) {
	for target, wants := range map[plugin.Platform][]string{
		plugin.Claude:  {"/plugin marketplace add arxdsilva/ai-loops", "/plugin install loops@ai-loops"},
		plugin.Copilot: {"copilot plugin install arxdsilva/ai-loops", "Chat: Install Plugin from Source"},
	} {
		files, warns := build(t, target, Options{})
		if len(files) != 2 || len(warns) != 0 {
			t.Fatalf("%s: %d files, warns %q", target, len(files), warns)
		}
		mustFile(t, files, ".loops-generated")
		r := mustFile(t, files, "README.md")
		for _, w := range wants {
			if !strings.Contains(r, w) {
				t.Errorf("%s README lacks %q", target, w)
			}
		}
	}
}

func TestRootFiles(t *testing.T) {
	p, _ := loadFixture(t)
	files := map[string]plugin.File{}
	for _, f := range RootFiles(p) {
		files[f.Path] = f
	}
	if len(files) != 2 {
		t.Fatalf("root files = %v", files)
	}
	var codex struct {
		Name      string
		Interface struct{ DisplayName string }
		Plugins   []struct {
			Name     string
			Source   struct{ Source, Path string }
			Policy   struct{ Installation, Authentication string }
			Category string
		}
	}
	mustJSON(t, files, ".agents/plugins/marketplace.json", &codex)
	if codex.Name == "" || codex.Interface.DisplayName == "" || len(codex.Plugins) != 1 {
		t.Fatalf("codex marketplace = %+v", codex)
	}
	cp := codex.Plugins[0]
	if cp.Name != "loops" || cp.Source.Source != "local" || cp.Source.Path != "./platforms/codex" ||
		cp.Policy.Installation != "AVAILABLE" || cp.Policy.Authentication == "" || cp.Category == "" {
		t.Errorf("codex entry = %+v", cp)
	}
	var cursor struct {
		Name    string
		Owner   struct{ Name string }
		Plugins []struct{ Name, Source, Description string }
	}
	mustJSON(t, files, ".cursor-plugin/marketplace.json", &cursor)
	if cursor.Name == "" || cursor.Owner.Name != "Ann" || len(cursor.Plugins) != 1 ||
		cursor.Plugins[0].Name != "loops" || cursor.Plugins[0].Source != "platforms/cursor" {
		t.Errorf("cursor marketplace = %+v", cursor)
	}
	p.Author = ""
	for _, f := range RootFiles(p) {
		if f.Path == ".cursor-plugin/marketplace.json" && !strings.Contains(string(f.Data), `"name": "loops"`) {
			t.Errorf("owner fallback missing: %s", f.Data)
		}
	}
}

func TestBuildDeterministic(t *testing.T) {
	for _, target := range []plugin.Platform{plugin.Claude, plugin.Copilot, plugin.Codex, plugin.Cursor} {
		p, src := loadFixture(t)
		a, wa, err := Build(p, src, target, Options{})
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 10; i++ {
			b, wb, _ := Build(p, src, target, Options{})
			if !reflect.DeepEqual(a, b) || !reflect.DeepEqual(wa, wb) {
				t.Fatalf("%s: Build is not deterministic", target)
			}
		}
	}
	p, _ := loadFixture(t)
	r1, _ := json.Marshal(RootFiles(p))
	r2, _ := json.Marshal(RootFiles(p))
	if !bytes.Equal(r1, r2) {
		t.Fatal("RootFiles not deterministic")
	}
}

func TestBuildUnknownPlatform(t *testing.T) {
	p, src := loadFixture(t)
	if _, _, err := Build(p, src, "vim", Options{}); err == nil {
		t.Fatal("unknown platform built")
	}
}

func TestEmittersPropagateBaseErrors(t *testing.T) {
	p, src := loadFixture(t)
	for k := range src {
		if strings.HasPrefix(k, "skills/") {
			delete(src, k)
		}
	}
	for _, target := range []plugin.Platform{plugin.Codex, plugin.Cursor} {
		if _, _, err := Build(p, src, target, Options{}); err == nil {
			t.Errorf("%s built without skills/", target)
		}
	}
}
