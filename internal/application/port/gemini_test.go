package port

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
)

func p2Plugin() plugin.Plugin {
	return plugin.Plugin{
		Name: "loops", Version: "0.2.0", Description: "d",
		Skills: []plugin.Skill{
			{Name: "work", Description: "Run \"work\": the loop", Command: true},
			{Name: "go-style", Description: "ref"},
		},
		Agents: []plugin.Agent{
			{Name: "qa", Description: "QA: checks", Tier: plugin.TierSonnet, ReadOnly: true, Body: "QA body\n"},
			{Name: "dev", Description: "Dev", Tier: plugin.TierOpus, Tools: []string{"Bash"}, Body: "Dev body\n"},
		},
		Hooks: []plugin.Hook{
			{Event: plugin.EventSessionStart, Name: "session-start", TimeoutSec: 120},
			{Event: plugin.EventPreToolUse, Matcher: "Read|Bash", Name: "read-guard", TimeoutSec: 10},
			{Event: plugin.EventPostToolUse, Name: "trace", TimeoutSec: 10},
			{Event: plugin.EventPostToolUseFailure, Name: "trace", TimeoutSec: 10},
			{Event: plugin.EventSubagentStart, Name: "trace-agent", TimeoutSec: 10},
			{Event: plugin.EventStop, Name: "stop-gate", TimeoutSec: 10},
		},
	}
}

func p2FS() fstest.MapFS {
	return fstest.MapFS{
		"skills/work/SKILL.md":            {Data: []byte("work")},
		"skills/work/platforms/amp.md":    {Data: []byte("amp")},
		"skills/work/platforms/gemini.md": {Data: []byte("gem")},
		"ROLES.md":                        {Data: []byte("roles")},
		"templates/PLUGIN_CONTEXT.md":     {Data: []byte("CTX")},
	}
}

func p2Build(t *testing.T, fn emitter, o Options) (map[string]string, []string) {
	t.Helper()
	files, warns, err := fn(p2Plugin(), p2FS(), o)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]string{}
	for _, f := range files {
		if _, dup := m[f.Path]; dup {
			t.Fatalf("duplicate path %s", f.Path)
		}
		m[f.Path] = string(f.Data)
	}
	// determinism
	files2, warns2, _ := fn(p2Plugin(), p2FS(), o)
	again, _ := json.Marshal(files2)
	first, _ := json.Marshal(files)
	if string(again) != string(first) || !reflect.DeepEqual(warns, warns2) {
		t.Fatal("non-deterministic output")
	}
	return m, warns
}

func p2Has(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Fatalf("missing %q in:\n%s", sub, s)
	}
}

func TestGemini(t *testing.T) {
	m, warns := p2Build(t, buildGemini, Options{Models: map[plugin.Tier]string{plugin.TierSonnet: "gemini-x"}})

	var man map[string]string
	if err := json.Unmarshal([]byte(m["gemini-extension.json"]), &man); err != nil || man["contextFileName"] != "GEMINI.md" || man["name"] != "loops" {
		t.Fatalf("manifest %v %v", man, err)
	}
	p2Has(t, m["GEMINI.md"], "skills/work")
	if m["skills/work/SKILL.md"] != "work" || m["bin/loops"] == "" {
		t.Fatal("baseFiles missing")
	}

	qa := m["agents/qa.md"]
	p2Has(t, qa, "kind: local")
	p2Has(t, qa, `model: "gemini-x"`)
	p2Has(t, qa, "  - read_file\n")
	p2Has(t, qa, "  - write_file\n")
	p2Has(t, qa, "write only your own file under the run dir")
	if strings.Contains(qa, "- replace") {
		t.Fatal("read-only role has replace")
	}
	p2Has(t, qa, "QA body")
	dev := m["agents/dev.md"]
	if strings.Contains(dev, "tools:") || strings.Contains(dev, "model:") {
		t.Fatalf("dev should have no tools/model:\n%s", dev)
	}

	cmd := m["commands/loops/work.toml"]
	p2Has(t, cmd, "{{args}}")
	p2Has(t, cmd, `description = "Run \"work\": the loop"`)
	if _, ok := m["commands/loops/go-style.toml"]; ok {
		t.Fatal("non-command skill got a command")
	}

	var hooks struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string
				Timeout int
			}
		}
	}
	if err := json.Unmarshal([]byte(m["hooks/hooks.json"]), &hooks); err != nil {
		t.Fatal(err)
	}
	for _, ev := range []string{"SessionStart", "BeforeTool", "AfterTool", "AfterAgent"} {
		if len(hooks.Hooks[ev]) == 0 {
			t.Fatalf("no %s", ev)
		}
	}
	if len(hooks.Hooks["AfterTool"]) != 1 {
		t.Fatalf("trace not deduped: %v", hooks.Hooks["AfterTool"])
	}
	h := hooks.Hooks["SessionStart"][0].Hooks[0]
	if h.Timeout != 120000 || !strings.Contains(h.Command, "${extensionPath}/bin/loops") || !strings.HasSuffix(h.Command, "--platform gemini") {
		t.Fatalf("hook %+v", h)
	}
	p2Has(t, m["README.md"], "gemini extensions link")
	p2Has(t, m["README.md"], "gemini extensions install")

	w := strings.Join(warns, "\n")
	p2Has(t, w, "write_file")
	p2Has(t, w, "no subagent events")
	p2Has(t, w, "role dev Claude tool allowlist")
}
