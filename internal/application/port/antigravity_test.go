package port

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
)

func TestAntigravity(t *testing.T) {
	p, src := loadFixture(t)
	src["skills/work/platforms/antigravity.md"] = &fstest.MapFile{Data: []byte("antigravity notes\n")}
	src["LICENSE"] = &fstest.MapFile{Data: []byte("MIT\n")}

	files, warns, err := Build(p, src, plugin.Antigravity, Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	m := map[string]plugin.File{}
	for i, f := range files {
		if i > 0 && files[i-1].Path >= f.Path {
			t.Fatalf("files not sorted/unique at %q after %q", f.Path, files[i-1].Path)
		}
		m[f.Path] = f
	}

	// Required files and platform filtering (native Antigravity plugin layout).
	for _, want := range []string{
		".loops-generated",
		"LICENSE",
		"README.md",
		"ROLES.md",
		"agents/domain-engineer.md",
		"agents/planner.md",
		"agents/scout.md",
		"bin/loops",
		"hooks.json",
		"plugin.json",
		"rules/AGENTS.md",
		"skills/work/SKILL.md",
		"skills/work/platforms/antigravity.md",
		"specs/WORKFLOW.md",
		"templates/CLAIMS_REPORT.md",
	} {
		if _, ok := m[want]; !ok {
			t.Errorf("missing %s", want)
		}
	}
	for _, absent := range []string{".agents/hooks.json", ".agents/skills/work/SKILL.md"} {
		if _, ok := m[absent]; ok {
			t.Errorf("unexpected old placement path %s", absent)
		}
	}
	for _, other := range []string{"codex.md", "cursor.md", "gemini.md"} {
		if _, ok := m["skills/work/platforms/"+other]; ok {
			t.Errorf("other platform notes leaked: %s", other)
		}
	}
	if !m["bin/loops"].Exec {
		t.Error("bin/loops not executable")
	}

	// plugin.json matches https://antigravity.google/schemas/v1/plugin.json without repository.
	var man map[string]any
	if err := json.Unmarshal(m["plugin.json"].Data, &man); err != nil {
		t.Fatalf("invalid plugin.json: %v", err)
	}
	if man["$schema"] != "https://antigravity.google/schemas/v1/plugin.json" {
		t.Errorf("$schema = %v", man["$schema"])
	}
	if man["name"] != "loops" || man["displayName"] != "loops" || man["version"] != "0.2.0" || man["description"] != "Desc <x> & y" {
		t.Errorf("core manifest fields = %v", man)
	}
	if auth, ok := man["author"].(map[string]any); !ok || auth["name"] != "Ann" {
		t.Errorf("author = %v", man["author"])
	}
	if man["homepage"] != "https://h" || man["license"] != "MIT" {
		t.Errorf("homepage/license = %v / %v", man["homepage"], man["license"])
	}
	if kws, ok := man["keywords"].([]any); !ok || len(kws) != 2 || kws[0] != "go" || kws[1] != "qa" {
		t.Errorf("keywords = %v", man["keywords"])
	}
	if _, ok := man["repository"]; ok {
		t.Errorf("repository must be omitted from plugin.json: %v", man["repository"])
	}

	// rules/AGENTS.md carries templates/PLUGIN_CONTEXT.md.
	if got := string(m["rules/AGENTS.md"].Data); got != "Always-on loops context.\n" {
		t.Errorf("rules/AGENTS.md = %q", got)
	}

	// Determinism check.
	files2, warns2, err := Build(p, src, plugin.Antigravity, Options{})
	if err != nil {
		t.Fatalf("second Build: %v", err)
	}
	if !reflect.DeepEqual(files, files2) || !reflect.DeepEqual(warns, warns2) {
		t.Fatal("Build(Antigravity) is not deterministic")
	}

	// README content for Tier A.
	readme := string(m["README.md"].Data)
	for _, sub := range []string{
		"Tier A",
		"agy plugin validate",
		"agy plugin install ./platforms/antigravity",
		"npx ai-loops --target antigravity",
		"loops install --target antigravity",
		"send_message",
		"invoke_subagent",
		"rules/AGENTS.md",
		"hooks.json",
		".agents/",
		"~/.gemini/config/",
		"`tools:` allowlists",
		"write-guard",
		"PreToolUse",
	} {
		p2Has(t, readme, sub)
	}
	if strings.Contains(readme, "tools` omitted") {
		t.Errorf("README.md still mentions omitted tools:\n%s", readme)
	}

	// Warnings for dropped events, read-only enforcement, and hook shim.
	w := strings.Join(warns, "\n")
	for _, sub := range []string{
		"PostToolUseFailure",
		"SubagentStart",
		"SubagentStop",
		"read-only",
		"replace_file_content",
		"write_to_file",
		"write-guard",
		"run_command",
		"instruction",
		"loops install",
	} {
		p2Has(t, w, sub)
	}
	for _, stale := range []string{"tool lists omitted", "tools omitted"} {
		if strings.Contains(w, stale) {
			t.Errorf("unexpected stale warning %q:\n%s", stale, w)
		}
	}
}

func TestAntigravityOptionalManifestAndEmptyContextWarning(t *testing.T) {
	p := p2Plugin()
	src := p2FS()
	src["templates/PLUGIN_CONTEXT.md"] = &fstest.MapFile{Data: []byte("   \n")}

	files, warns, err := Build(p, src, plugin.Antigravity, Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	var manData []byte
	for _, f := range files {
		if f.Path == "plugin.json" {
			manData = f.Data
			break
		}
	}
	var man map[string]any
	if err := json.Unmarshal(manData, &man); err != nil {
		t.Fatalf("invalid plugin.json: %v", err)
	}
	for _, omitted := range []string{"author", "homepage", "repository", "license", "keywords"} {
		if _, ok := man[omitted]; ok {
			t.Errorf("empty optional field %q present in plugin.json", omitted)
		}
	}
	p2Has(t, strings.Join(warns, "\n"), "PLUGIN_CONTEXT.md")
}

func TestAntigravityAgents(t *testing.T) {
	p, src := loadFixture(t)
	p.Agents = append(p.Agents,
		plugin.Agent{
			Name:        "hardware-researcher",
			Description: "Probe hardware.",
			Tier:        plugin.TierHaiku,
			ReadOnly:    true,
			Body:        "Hardware body.\n",
		},
		plugin.Agent{
			Name:        "web-researcher",
			Description: "Research web.",
			Tier:        plugin.TierSonnet,
			ReadOnly:    true,
			Body:        "Web body.\n",
		},
	)

	// Default options -> model: "inherit".
	files, _, err := Build(p, src, plugin.Antigravity, Options{})
	if err != nil {
		t.Fatalf("Build default: %v", err)
	}
	byPath := map[string]string{}
	for _, f := range files {
		byPath[f.Path] = string(f.Data)
	}

	wantTools := map[string][]string{
		"agents/hardware-researcher.md": {"run_command", "send_message"},
		"agents/web-researcher.md":      {"view_file", "write_to_file", "run_command", "grep_search", "find_by_name", "list_dir", "send_message", "search_web"},
		"agents/planner.md":             {"view_file", "write_to_file", "edit_file", "run_command", "grep_search", "find_by_name", "list_dir", "send_message"},
		"agents/domain-engineer.md":     {"view_file", "write_to_file", "replace_file_content", "edit_file", "run_command", "grep_search", "find_by_name", "list_dir", "send_message"},
		"agents/scout.md":               {"view_file", "write_to_file", "replace_file_content", "edit_file", "run_command", "grep_search", "find_by_name", "list_dir", "send_message"},
	}
	for file, want := range wantTools {
		fm, _, err := parseFrontmatter(file, []byte(byPath[file]))
		if err != nil {
			t.Fatalf("%s frontmatter: %v", file, err)
		}
		got, err := list(fm["tools"])
		if err != nil {
			t.Fatalf("%s tools list(%q): %v", file, fm["tools"], err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s tools = %v, want %v", file, got, want)
		}
		wantLine := "tools: [" + strings.Join(want, ", ") + "]\n"
		if !strings.Contains(byPath[file], wantLine) {
			t.Errorf("%s missing inline YAML sequence %q", file, wantLine)
		}
	}

	// Read-only role (planner) gets readOnlyNote and role-tailored tools.
	fm, body, err := parseFrontmatter("agents/planner.md", []byte(byPath["agents/planner.md"]))
	if err != nil {
		t.Fatalf("planner frontmatter: %v", err)
	}
	if fm["name"] != "planner" || fm["subagent"] != "true" || fm["mainAgent"] != "false" || fm["model"] != "inherit" || fm["commandExecutionPolicy"] != "auto" {
		t.Fatalf("planner frontmatter = %+v", fm)
	}
	if !strings.HasPrefix(body, readOnlyNote+"\n\n") {
		t.Fatalf("planner missing readOnlyNote prefix: %q", body)
	}
	p2Has(t, body, "You plan.")

	// Writer role (domain-engineer) has no readOnlyNote and keeps body intact.
	fm, body, err = parseFrontmatter("agents/domain-engineer.md", []byte(byPath["agents/domain-engineer.md"]))
	if err != nil {
		t.Fatalf("domain-engineer frontmatter: %v", err)
	}
	if fm["subagent"] != "true" || fm["mainAgent"] != "false" || fm["model"] != "inherit" || fm["commandExecutionPolicy"] != "auto" {
		t.Fatalf("domain-engineer frontmatter = %+v", fm)
	}
	if strings.Contains(body, readOnlyNote) {
		t.Fatalf("writer role should not have readOnlyNote: %q", body)
	}
	if body != "You implement.\n" {
		t.Fatalf("domain-engineer body = %q", body)
	}

	// Role with canonical tools allowlist (scout) gets implementer tools in Antigravity frontmatter.
	fm, body, err = parseFrontmatter("agents/scout.md", []byte(byPath["agents/scout.md"]))
	if err != nil {
		t.Fatalf("scout frontmatter: %v", err)
	}
	if fm["model"] != "inherit" || strings.Contains(body, readOnlyNote) {
		t.Fatalf("scout fm=%+v body=%q", fm, body)
	}

	// Mapped Options.Models sets model per tier; unmapped tiers fall back to inherit.
	mappedFiles, _, err := Build(p, src, plugin.Antigravity, Options{
		Models: map[plugin.Tier]string{
			plugin.TierOpus:   "pro",
			plugin.TierSonnet: "flash",
		},
	})
	if err != nil {
		t.Fatalf("Build mapped: %v", err)
	}
	mapped := map[string]string{}
	for _, f := range mappedFiles {
		mapped[f.Path] = string(f.Data)
	}
	for _, tc := range []struct {
		file string
		want string
	}{
		{"agents/planner.md", "pro"},
		{"agents/domain-engineer.md", "flash"},
		{"agents/scout.md", "inherit"},
	} {
		fm, _, err := parseFrontmatter(tc.file, []byte(mapped[tc.file]))
		if err != nil {
			t.Fatalf("%s: %v", tc.file, err)
		}
		if fm["model"] != tc.want {
			t.Errorf("%s model = %q, want %q", tc.file, fm["model"], tc.want)
		}
	}
}

func TestAntigravityHooksJSON(t *testing.T) {
	p, src := loadFixture(t)
	// Add a duplicate (Event, Name) hook to confirm deduplication.
	p.Hooks = append(p.Hooks, plugin.Hook{
		Event: plugin.EventPreToolUse, Matcher: "Read|Bash", Name: "read-guard", TimeoutSec: 10,
	})
	files, _, err := Build(p, src, plugin.Antigravity, Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	var hooksData []byte
	for _, f := range files {
		if f.Path == "hooks.json" {
			hooksData = f.Data
			break
		}
	}
	if len(hooksData) == 0 {
		t.Fatal("missing root hooks.json")
	}

	var doc struct {
		Loops map[string]json.RawMessage `json:"loops"`
	}
	if err := json.Unmarshal(hooksData, &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, hooksData)
	}
	var enabled bool
	if err := json.Unmarshal(doc.Loops["enabled"], &enabled); err != nil || !enabled {
		t.Fatalf("loops.enabled = %s, err = %v", doc.Loops["enabled"], err)
	}
	for _, bad := range []string{"PostToolUseFailure", "SubagentStart", "SubagentStop"} {
		if _, ok := doc.Loops[bad]; ok {
			t.Errorf("unsupported event %s present in hooks.json", bad)
		}
	}
	if len(doc.Loops) != 5 { // enabled + 4 events
		t.Fatalf("unexpected keys in loops: %v", doc.Loops)
	}

	type hookHandler struct {
		Type    string `json:"type"`
		Command string `json:"command"`
		Timeout int    `json:"timeout"`
	}
	type hookGroup struct {
		Matcher string        `json:"matcher,omitempty"`
		Hooks   []hookHandler `json:"hooks"`
	}

	// SessionStart and Stop are flat handler arrays.
	for _, ev := range []string{"SessionStart", "Stop"} {
		var handlers []hookHandler
		if err := json.Unmarshal(doc.Loops[ev], &handlers); err != nil {
			t.Fatalf("%s flat handlers: %v\n%s", ev, err, doc.Loops[ev])
		}
		if len(handlers) != 1 {
			t.Fatalf("%s handlers = %+v", ev, handlers)
		}
		h := handlers[0]
		if h.Type != "command" || !strings.HasPrefix(h.Command, "./bin/loops hook ") || !strings.HasSuffix(h.Command, " --platform antigravity") || h.Timeout == 0 {
			t.Errorf("%s handler = %+v", ev, h)
		}
	}

	// PreToolUse and PostToolUse are matcher-wrapped arrays with Antigravity tool regexes.
	for _, ev := range []string{"PreToolUse", "PostToolUse"} {
		var groups []hookGroup
		if err := json.Unmarshal(doc.Loops[ev], &groups); err != nil {
			t.Fatalf("%s matcher groups: %v\n%s", ev, err, doc.Loops[ev])
		}
		if len(groups) == 0 {
			t.Fatalf("%s has no groups", ev)
		}
		for _, g := range groups {
			if len(g.Hooks) != 1 {
				t.Fatalf("%s group hooks = %+v", ev, g)
			}
			h := g.Hooks[0]
			if h.Type != "command" || !strings.HasPrefix(h.Command, "./bin/loops hook ") || !strings.HasSuffix(h.Command, " --platform antigravity") || h.Timeout == 0 {
				t.Errorf("%s hook = %+v", ev, h)
			}
		}
	}

	var pre []hookGroup
	_ = json.Unmarshal(doc.Loops["PreToolUse"], &pre)
	if len(pre) != 3 || pre[0].Matcher != "view_file|run_command" || pre[1].Matcher != "invoke_subagent" {
		t.Fatalf("PreToolUse = %+v", pre)
	}
	wg := pre[2]
	if wg.Matcher != "replace_file_content|edit_file|write_to_file|multi_replace_file_content" ||
		len(wg.Hooks) != 1 ||
		wg.Hooks[0].Command != "./bin/loops hook write-guard --platform antigravity" ||
		wg.Hooks[0].Timeout != 10 {
		t.Fatalf("write-guard PreToolUse group = %+v", wg)
	}
	var post []hookGroup
	_ = json.Unmarshal(doc.Loops["PostToolUse"], &post)
	if len(post) != 2 || post[0].Matcher != "replace_file_content|edit_file|write_to_file" || post[1].Matcher != "" {
		t.Fatalf("PostToolUse = %+v", post)
	}
	// trace had no matcher (normalized from "*"), so "matcher" key must be omitted.
	if strings.Count(string(doc.Loops["PostToolUse"]), `"matcher"`) != 1 {
		t.Errorf("expected exactly 1 matcher in PostToolUse: %s", doc.Loops["PostToolUse"])
	}

	// Existing write-guard in PreToolUse is not duplicated.
	p.Hooks = append(p.Hooks, plugin.Hook{
		Event: plugin.EventPreToolUse, Matcher: "Edit|Write|MultiEdit", Name: "write-guard", TimeoutSec: 10,
	})
	files2, _, err := Build(p, src, plugin.Antigravity, Options{})
	if err != nil {
		t.Fatalf("Build with explicit write-guard: %v", err)
	}
	for _, f := range files2 {
		if f.Path == "hooks.json" {
			var doc2 struct {
				Loops struct {
					PreToolUse []hookGroup `json:"PreToolUse"`
				} `json:"loops"`
			}
			if err := json.Unmarshal(f.Data, &doc2); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if len(doc2.Loops.PreToolUse) != 3 {
				t.Fatalf("expected 3 PreToolUse groups without duplicate write-guard, got %+v", doc2.Loops.PreToolUse)
			}
			break
		}
	}
}

func TestAntigravityMatcher(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"*", ""},
		{"Read|*", ""},
		{"Read|Bash", "view_file|run_command"},
		{"Edit|Write|MultiEdit", "replace_file_content|edit_file|write_to_file|multi_replace_file_content"},
		{"Agent|Task", "invoke_subagent"},
		{"Read|Edit|Read", "view_file|replace_file_content|edit_file"},
		{"Bash|CustomTool|Bash", "run_command|CustomTool"},
	}
	for _, tc := range cases {
		p, src := loadFixture(t)
		p.Hooks = []plugin.Hook{
			{Event: plugin.EventPreToolUse, Matcher: tc.in, Name: "guard", TimeoutSec: 10},
		}
		files, _, err := Build(p, src, plugin.Antigravity, Options{})
		if err != nil {
			t.Fatalf("Build(%q): %v", tc.in, err)
		}
		var hooksData []byte
		for _, f := range files {
			if f.Path == "hooks.json" {
				hooksData = f.Data
				break
			}
		}
		if len(hooksData) == 0 {
			t.Fatalf("%q: missing hooks.json", tc.in)
		}
		var doc struct {
			Loops struct {
				PreToolUse []struct {
					Matcher string `json:"matcher"`
				} `json:"PreToolUse"`
			} `json:"loops"`
		}
		if err := json.Unmarshal(hooksData, &doc); err != nil {
			t.Fatalf("%q: invalid JSON: %v", tc.in, err)
		}
		if len(doc.Loops.PreToolUse) != 2 || doc.Loops.PreToolUse[0].Matcher != tc.want {
			t.Errorf("matcher(%q) = %+v, want %q", tc.in, doc.Loops.PreToolUse, tc.want)
		}
	}
}

func TestAntigravityPropagatesBaseErrors(t *testing.T) {
	p, src := loadFixture(t)
	for k := range src {
		if strings.HasPrefix(k, "skills/") {
			delete(src, k)
		}
	}
	if _, _, err := Build(p, src, plugin.Antigravity, Options{}); err == nil {
		t.Fatal("antigravity built without skills/")
	}
}

func TestAntigravityHooksSubsetOmitsEmptyEvents(t *testing.T) {
	p, src := loadFixture(t)
	p.Hooks = []plugin.Hook{
		{Event: plugin.EventSessionStart, Name: "session-start", TimeoutSec: 5},
		{Event: plugin.EventSubagentStart, Name: "subagent-start", TimeoutSec: 5},
	}
	files, warns, err := Build(p, src, plugin.Antigravity, Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	var hooksData []byte
	for _, f := range files {
		if f.Path == "hooks.json" {
			hooksData = f.Data
			break
		}
	}
	var doc struct {
		Loops map[string]json.RawMessage `json:"loops"`
	}
	if err := json.Unmarshal(hooksData, &doc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(doc.Loops) != 2 { // enabled + SessionStart only
		t.Fatalf("expected only enabled and SessionStart, got keys: %v", doc.Loops)
	}
	var start []struct {
		Type    string `json:"type"`
		Command string `json:"command"`
		Timeout int    `json:"timeout"`
	}
	if err := json.Unmarshal(doc.Loops["SessionStart"], &start); err != nil || len(start) != 1 || start[0].Command != "./bin/loops hook session-start --platform antigravity" {
		t.Fatalf("SessionStart = %s, err = %v", doc.Loops["SessionStart"], err)
	}
	for _, omitted := range []string{"PreToolUse", "PostToolUse", "Stop", "SubagentStart"} {
		if _, ok := doc.Loops[omitted]; ok {
			t.Errorf("omitted/unsupported event %s present in hooks.json", omitted)
		}
	}
	w := strings.Join(warns, "\n")
	p2Has(t, w, "SubagentStart")
	for _, absent := range []string{"PostToolUseFailure", "SubagentStop"} {
		if strings.Contains(w, absent) {
			t.Errorf("unexpected warning for absent event %s:\n%s", absent, w)
		}
	}
}
