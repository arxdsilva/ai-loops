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

	// Required files and platform filtering.
	for _, want := range []string{
		".agents/hooks.json",
		".agents/skills/work/SKILL.md",
		".agents/skills/work/platforms/antigravity.md",
		".loops-generated",
		"LICENSE",
		"README.md",
		"ROLES.md",
		"agents/domain-engineer.md",
		"agents/planner.md",
		"agents/scout.md",
		"bin/loops",
		"specs/WORKFLOW.md",
		"templates/CLAIMS_REPORT.md",
	} {
		if _, ok := m[want]; !ok {
			t.Errorf("missing %s", want)
		}
	}
	if _, ok := m["skills/work/SKILL.md"]; ok {
		t.Error("unrelocated skills/work/SKILL.md still present")
	}
	for _, other := range []string{"codex.md", "cursor.md", "gemini.md"} {
		if _, ok := m[".agents/skills/work/platforms/"+other]; ok {
			t.Errorf("other platform notes leaked: %s", other)
		}
	}
	if !m["bin/loops"].Exec {
		t.Error("bin/loops not executable")
	}

	// Determinism check.
	files2, warns2, err := Build(p, src, plugin.Antigravity, Options{})
	if err != nil {
		t.Fatalf("second Build: %v", err)
	}
	if !reflect.DeepEqual(files, files2) || !reflect.DeepEqual(warns, warns2) {
		t.Fatal("Build(Antigravity) is not deterministic")
	}

	// README content.
	readme := string(m["README.md"].Data)
	for _, sub := range []string{
		"npx ai-loops --target antigravity",
		"loops install --target antigravity",
		".agents/skills/",
		".agents/hooks.json",
		".loops/plugin/",
		"~/.gemini/config/skills/",
		"~/.gemini/config/hooks.json",
		"~/.local/share/loops/plugin/",
		"agy",
		"/skills reload",
	} {
		p2Has(t, readme, sub)
	}

	// Warnings for dropped events, subagent limitations, and PATH shim.
	w := strings.Join(warns, "\n")
	for _, sub := range []string{
		"PostToolUseFailure",
		"SubagentStart",
		"SubagentStop",
		"subagent",
		"loops install",
		"PATH",
	} {
		p2Has(t, w, sub)
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
		if f.Path == ".agents/hooks.json" {
			hooksData = f.Data
			break
		}
	}
	if len(hooksData) == 0 {
		t.Fatal("missing .agents/hooks.json")
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

	type hookGroup struct {
		Matcher string `json:"matcher,omitempty"`
		Hooks   []struct {
			Type    string `json:"type"`
			Command string `json:"command"`
			Timeout int    `json:"timeout"`
		} `json:"hooks"`
	}
	for _, ev := range []string{"SessionStart", "PreToolUse", "PostToolUse", "Stop"} {
		var groups []hookGroup
		if err := json.Unmarshal(doc.Loops[ev], &groups); err != nil {
			t.Fatalf("%s: %v", ev, err)
		}
		if len(groups) == 0 {
			t.Fatalf("%s has no groups", ev)
		}
		for _, g := range groups {
			if len(g.Hooks) != 1 {
				t.Fatalf("%s group hooks = %+v", ev, g)
			}
			h := g.Hooks[0]
			if h.Type != "command" || !strings.HasPrefix(h.Command, "loops hook ") || !strings.HasSuffix(h.Command, " --platform antigravity") || h.Timeout == 0 {
				t.Errorf("%s hook = %+v", ev, h)
			}
		}
	}

	// Verify dedupe & matchers on PreToolUse and PostToolUse.
	var pre []hookGroup
	_ = json.Unmarshal(doc.Loops["PreToolUse"], &pre)
	if len(pre) != 2 || pre[0].Matcher != "Read|Bash" || pre[1].Matcher != "Agent|Task" {
		t.Fatalf("PreToolUse = %+v", pre)
	}
	var post []hookGroup
	_ = json.Unmarshal(doc.Loops["PostToolUse"], &post)
	if len(post) != 2 || post[0].Matcher != "Edit|Write" || post[1].Matcher != "" {
		t.Fatalf("PostToolUse = %+v", post)
	}
	// trace had no matcher (normalized from "*"), so "matcher" key must be omitted.
	if strings.Contains(string(doc.Loops["PostToolUse"]), `"matcher": ""`) {
		t.Errorf("empty matcher serialized in PostToolUse: %s", doc.Loops["PostToolUse"])
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
		if f.Path == ".agents/hooks.json" {
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
