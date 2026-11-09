package port

import (
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
)

func TestOpenCode(t *testing.T) {
	m, warns := p2Build(t, buildOpenCode, Options{})
	if m[".agents/skills/work/SKILL.md"] != "work" || m["skills/work/SKILL.md"] != "" {
		t.Fatal("skills not relocated")
	}
	qa := m[".opencode/agents/loops/qa.md"]
	p2Has(t, qa, "mode: subagent")
	p2Has(t, qa, "action: edit")
	p2Has(t, qa, "effect: deny")
	if strings.Index(qa, `resource: ".loops/**"`) < strings.Index(qa, "effect: deny") {
		t.Fatal("allow .loops/** must follow the deny (last match wins)")
	}
	p2Has(t, qa, "write only your own file under the run dir")
	if m["agents/qa.md"] == "" || !strings.Contains(m["agents/dev.md"], "Dev body") {
		t.Fatal("role files missing at package root")
	}
	if strings.Contains(qa, "model:") {
		t.Fatal("unmapped model must be omitted")
	}
	if strings.Contains(m[".opencode/agents/loops/dev.md"], "permissions") {
		t.Fatal("writer role must not deny edit")
	}
	cmd := m[".opencode/commands/loops/work.md"]
	p2Has(t, cmd, "$ARGUMENTS")
	if _, ok := m[".opencode/commands/loops/go-style.md"]; ok {
		t.Fatal("non-command skill got a command")
	}
	ts := m[".opencode/plugins/loops.ts"]
	p2Has(t, ts, `const BEFORE: string[] = ["read-guard"]`)
	p2Has(t, ts, `const AFTER: string[] = ["trace"]`)
	p2Has(t, ts, "--platform")
	p2Has(t, ts, "opencode")
	p2Has(t, ts, "status === 2")
	p2Has(t, ts, "ASSUMED API")
	if len(warns) < 2 {
		t.Fatalf("warns %v", warns)
	}
}

func TestToAgentsSkills(t *testing.T) {
	in := []plugin.File{{Path: "skills/work/SKILL.md"}, {Path: "ROLES.md"}}
	out := toAgentsSkills(in, ".cline/skills/")
	got := map[string]bool{}
	for _, f := range out {
		got[f.Path] = true
	}
	for _, want := range []string{".agents/skills/work/SKILL.md", ".cline/skills/work/SKILL.md", "ROLES.md"} {
		if !got[want] {
			t.Fatalf("missing %s in %v", want, got)
		}
	}
	if got["skills/work/SKILL.md"] {
		t.Fatal("old path kept")
	}
}
