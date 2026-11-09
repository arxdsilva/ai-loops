package port

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
)

func pf(paths ...string) []plugin.File {
	var fs []plugin.File
	for _, p := range paths {
		fs = append(fs, plugin.File{Path: p, Data: []byte(p), Exec: p == "bin/loops"})
	}
	return fs
}

func dests(ps []plugin.Placement) []string {
	var d []string
	for _, p := range ps {
		d = append(d, p.Dest)
	}
	sort.Strings(d)
	return d
}

func TestInstallPlan(t *testing.T) {
	const proj, home = "/p", "/h"
	common := []string{"skills/work/SKILL.md", "specs/WORKFLOW.md", "templates/x.md", "ROLES.md", "bin/loops"}
	cases := []struct {
		name   string
		target plugin.Platform
		files  []plugin.File
		scope  string
		want   []string
	}{
		{"codex project", plugin.Codex, pf("agents/a.toml", "agents/b.toml", "plugin.json", "skills/work/SKILL.md"), "project",
			[]string{"/p/.codex/agents/a.toml", "/p/.codex/agents/b.toml"}},
		{"codex user", plugin.Codex, pf("agents/a.toml", "hooks/hooks.json"), "user", []string{"/h/.codex/agents/a.toml"}},
		{"opencode project", plugin.OpenCode, pf(append(common, ".opencode/agents/loops/x.md", ".opencode/plugins/loops.ts")...), "project",
			[]string{"/p/.agents/skills/work/SKILL.md", "/p/.loops/plugin/ROLES.md", "/p/.loops/plugin/bin/loops", "/p/.loops/plugin/skills/work/SKILL.md",
				"/p/.loops/plugin/specs/WORKFLOW.md", "/p/.loops/plugin/templates/x.md", "/p/.opencode/agents/loops/x.md", "/p/.opencode/plugins/loops.ts"}},
		{"opencode user", plugin.OpenCode, pf(append(common, ".opencode/agents/loops/x.md")...), "user",
			[]string{"/h/.agents/skills/work/SKILL.md", "/h/.config/opencode/agents/loops/x.md", "/h/.local/share/loops/plugin/ROLES.md",
				"/h/.local/share/loops/plugin/bin/loops", "/h/.local/share/loops/plugin/skills/work/SKILL.md", "/h/.local/share/loops/plugin/specs/WORKFLOW.md",
				"/h/.local/share/loops/plugin/templates/x.md"}},
		{"opencode native skills", plugin.OpenCode, pf(".agents/skills/work/SKILL.md", "skills/work/SKILL.md"), "project",
			[]string{"/p/.agents/skills/work/SKILL.md", "/p/.loops/plugin/skills/work/SKILL.md"}},
		{"amp project", plugin.Amp, pf(common...), "project",
			[]string{"/p/.agents/skills/work/SKILL.md", "/p/.loops/plugin/ROLES.md", "/p/.loops/plugin/bin/loops", "/p/.loops/plugin/skills/work/SKILL.md",
				"/p/.loops/plugin/specs/WORKFLOW.md", "/p/.loops/plugin/templates/x.md"}},
		{"windsurf project", plugin.Windsurf, pf(".devin/hooks.v1.json", "skills/work/SKILL.md"), "project",
			[]string{"/p/.agents/skills/work/SKILL.md", "/p/.devin/hooks.v1.json", "/p/.loops/plugin/skills/work/SKILL.md"}},
		{"windsurf user", plugin.Windsurf, pf(".devin/hooks.v1.json", "skills/work/SKILL.md"), "user",
			[]string{"/h/.agents/skills/work/SKILL.md", "/h/.config/devin/hooks.v1.json", "/h/.local/share/loops/plugin/skills/work/SKILL.md"}},
		{"cline project", plugin.Cline, pf(common...), "project",
			[]string{"/p/.cline/skills/work/SKILL.md", "/p/.loops/plugin/ROLES.md", "/p/.loops/plugin/bin/loops", "/p/.loops/plugin/skills/work/SKILL.md",
				"/p/.loops/plugin/specs/WORKFLOW.md", "/p/.loops/plugin/templates/x.md"}},
		{"cline native layout project", plugin.Cline, pf(".agents/skills/work/SKILL.md", ".cline/skills/work/SKILL.md", "specs/W.md", "bin/loops", ".loops-generated"), "project",
			[]string{"/p/.agents/skills/work/SKILL.md", "/p/.cline/skills/work/SKILL.md", "/p/.loops/plugin/bin/loops", "/p/.loops/plugin/specs/W.md"}},
		{"cline native layout user", plugin.Cline, pf(".agents/skills/work/SKILL.md", ".cline/skills/work/SKILL.md", "specs/W.md"), "user",
			[]string{"/h/.agents/skills/work/SKILL.md", "/h/.cline/skills/work/SKILL.md", "/h/.local/share/loops/plugin/specs/W.md"}},
		{"cline user", plugin.Cline, pf("skills/work/SKILL.md"), "user",
			[]string{"/h/.cline/skills/work/SKILL.md", "/h/.local/share/loops/plugin/skills/work/SKILL.md"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ps, notes, err := InstallPlan(c.target, c.files, c.scope, proj, home)
			if err != nil {
				t.Fatal(err)
			}
			got := dests(ps)
			if strings.Join(got, "\n") != strings.Join(c.want, "\n") {
				t.Errorf("dests:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(c.want, "\n"))
			}
			if len(notes) == 0 {
				t.Error("no notes")
			}
			for _, p := range ps {
				if !filepath.IsAbs(p.Dest) {
					t.Errorf("relative dest %s", p.Dest)
				}
				if p.File.Path == "bin/loops" && !p.File.Exec {
					t.Error("exec bit lost")
				}
			}
		})
	}
}

func TestInstallPlanWindsurfPinsShim(t *testing.T) {
	hooks := plugin.File{Path: ".devin/hooks.v1.json", Data: []byte(`{"hooks":{"Stop":[{"command":"loops hook stop-gate --platform windsurf"}]}}`)}
	ps, _, err := InstallPlan(plugin.Windsurf, []plugin.File{hooks, {Path: ".loops-generated"}, {Path: "bin/loops", Exec: true}}, "project", "/p", "/h")
	if err != nil {
		t.Fatal(err)
	}
	var got string
	for _, p := range ps {
		if strings.HasSuffix(p.Dest, "hooks.v1.json") {
			got = string(p.File.Data)
		}
		if strings.HasSuffix(p.Dest, ".loops-generated") {
			t.Error("marker installed")
		}
	}
	want := `{"hooks":{"Stop":[{"command":"\"$DEVIN_PROJECT_DIR/.loops/plugin/bin/loops\" hook stop-gate --platform windsurf"}]}}`
	if got != want {
		t.Errorf("got %s\nwant %s", got, want)
	}
	var v any
	if err := json.Unmarshal([]byte(got), &v); err != nil {
		t.Errorf("invalid JSON: %v", err)
	}
	if string(hooks.Data) != `{"hooks":{"Stop":[{"command":"loops hook stop-gate --platform windsurf"}]}}` {
		t.Error("input mutated")
	}
}

func TestInstallPlanNativeAndErrors(t *testing.T) {
	for _, tg := range []plugin.Platform{plugin.Claude, plugin.Copilot, plugin.Cursor, plugin.Gemini} {
		ps, notes, err := InstallPlan(tg, pf("README.md"), "project", "/p", "/h")
		if err != nil || len(ps) != 0 || len(notes) != 1 || !strings.HasPrefix(notes[0], string(tg)+":") {
			t.Errorf("%s: %v %v %v", tg, ps, notes, err)
		}
	}
	if _, _, err := InstallPlan("nope", nil, "project", "/p", "/h"); err == nil {
		t.Error("unknown platform")
	}
	if _, _, err := InstallPlan(plugin.Codex, nil, "weird", "/p", "/h"); err == nil {
		t.Error("unknown scope")
	}
	if _, _, err := InstallPlan(plugin.Amp, nil, "project", "", "/h"); err == nil {
		t.Error("missing project dir")
	}
	if _, _, err := InstallPlan(plugin.Amp, nil, "user", "/p", ""); err == nil {
		t.Error("missing home dir")
	}
	ps, _, _ := InstallPlan(plugin.Amp, pf("../evil", "/abs", "skills/x/SKILL.md"), "project", "/p", "/h")
	for _, d := range dests(ps) {
		if !strings.HasPrefix(d, "/p/") || strings.Contains(d, "evil") {
			t.Errorf("unsafe dest %s", d)
		}
	}
}

func TestInstallPlanWindsurfUserScopeAbsoluteShim(t *testing.T) {
	hooks := plugin.File{Path: ".devin/hooks.v1.json", Data: []byte(`{"c":"loops hook x"}`)}
	ps, _, _ := InstallPlan(plugin.Windsurf, []plugin.File{hooks}, "user", "/p", "/h")
	if len(ps) != 1 || !strings.Contains(string(ps[0].File.Data), `\"/h/.local/share/loops/plugin/bin/loops\" hook x`) {
		t.Errorf("%v", ps)
	}
}

func TestInstallPlanCopiesRootAgentsToPluginHome(t *testing.T) {
	ps, _, _ := InstallPlan(plugin.Amp, pf("agents/qa-verifier.md", ".agents/skills/work/SKILL.md"), "project", "/p", "/h")
	if got := strings.Join(dests(ps), " "); !strings.Contains(got, "/p/.loops/plugin/agents/qa-verifier.md") {
		t.Errorf("%s", got)
	}
}

func TestGeminiNoteAndPluginHome(t *testing.T) {
	_, notes, _ := InstallPlan(plugin.Gemini, nil, "project", "/p", "/h")
	if !strings.Contains(notes[0], "gemini extensions install ./ai-loops/platforms/gemini") {
		t.Errorf("%v", notes)
	}
	if PluginHome("user", "/h") != "/h/.local/share/loops/plugin" || PluginHome("project", "/p") != "/p/.loops/plugin" {
		t.Error("PluginHome")
	}
}
