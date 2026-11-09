package port

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRoleFilesAtRoot(t *testing.T) {
	for name, fn := range map[string]emitter{"amp": buildAmp, "cline": buildCline, "windsurf": buildWindsurf} {
		m, _ := p2Build(t, fn, Options{})
		qa := m["agents/qa.md"]
		p2Has(t, qa, "name: qa")
		p2Has(t, qa, "QA body")
		if strings.Contains(qa, "run dir") {
			t.Fatalf("%s: body must be intact", name)
		}
	}
}

func TestAmp(t *testing.T) {
	m, warns := p2Build(t, buildAmp, Options{})
	p2Has(t, m["README.md"], "Tier C")
	if m[".agents/skills/work/platforms/amp.md"] != "amp" || m[".agents/skills/work/platforms/gemini.md"] != "" {
		t.Fatal("platform notes not filtered")
	}
	p2Has(t, m["README.md"], ".agents/skills/")
	if !strings.Contains(strings.Join(warns, "\n"), "amp: role") {
		t.Fatalf("warns %v", warns)
	}
}

func TestCline(t *testing.T) {
	m, warns := p2Build(t, buildCline, Options{})
	if m[".cline/skills/work/SKILL.md"] != "work" || m[".agents/skills/work/SKILL.md"] != "work" {
		t.Fatal("skills not in both roots")
	}
	p2Has(t, m["README.md"], "solo mode")
	p2Has(t, m["README.md"], ".cline/skills/")
	if !strings.Contains(strings.Join(warns, "\n"), "solo mode") {
		t.Fatalf("warns %v", warns)
	}
}

func TestWindsurf(t *testing.T) {
	m, warns := p2Build(t, buildWindsurf, Options{})
	var h map[string][]struct {
		Hooks []struct {
			Type, Command string
			Timeout       int
		}
	}
	if err := json.Unmarshal([]byte(m[".devin/hooks.v1.json"]), &h); err != nil {
		t.Fatal(err)
	}
	if len(h) != 4 {
		t.Fatalf("events %v", h)
	}
	for _, ev := range []string{"SessionStart", "PreToolUse", "PostToolUse", "Stop"} {
		if len(h[ev]) != 1 {
			t.Fatalf("%s: %v", ev, h[ev])
		}
	}
	c := h["PreToolUse"][0].Hooks[0]
	if c.Command != "loops hook read-guard --platform windsurf" || c.Timeout != 10 || c.Type != "command" {
		t.Fatalf("%+v", c)
	}
	w := strings.Join(warns, "\n")
	p2Has(t, w, "rewritten to the plugin-home shim by `loops install`")
	p2Has(t, w, "PostToolUseFailure")
	p2Has(t, w, "SubagentStart")
	p2Has(t, m["README.md"], "solo mode")
}
