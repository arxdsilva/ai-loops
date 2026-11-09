package port

import (
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
)

func TestLoadMetadata(t *testing.T) {
	p, _ := loadFixture(t)
	if p.Name != "loops" || p.Version != "0.2.0" || p.Author != "Ann" || p.Homepage != "https://h" ||
		p.Repository != "https://r" || p.License != "MIT" || !reflect.DeepEqual(p.Keywords, []string{"go", "qa"}) {
		t.Fatalf("metadata = %+v", p)
	}
}

func TestLoadSkills(t *testing.T) {
	p, _ := loadFixture(t)
	var names []string
	for _, s := range p.Skills {
		names = append(names, s.Name)
	}
	if want := []string{"init", "report", "status", "testing", "work"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("skills = %v, want %v", names, want)
	}
	work, _ := skillByName(p, "work")
	if !work.Command || work.Dir != "skills/work" || work.ArgumentHint != "<ticket> [--approve]" || work.Description != "Work a ticket." {
		t.Fatalf("work = %+v", work)
	}
	if want := []string{"skills/work/SKILL.md", "skills/work/platforms/codex.md", "skills/work/platforms/cursor.md", "skills/work/platforms/gemini.md"}; !reflect.DeepEqual(work.Files, want) {
		t.Fatalf("work files = %v", work.Files)
	}
	rep, _ := skillByName(p, "report")
	if rep.Description != "Report's skill" || !rep.Command {
		t.Fatalf("report = %+v", rep)
	}
	testing_, _ := skillByName(p, "testing")
	if testing_.Command || !reflect.DeepEqual(testing_.Files, []string{"skills/testing/SKILL.md", "skills/testing/ref/extra.md"}) {
		t.Fatalf("testing = %+v", testing_)
	}
}

func TestLoadAgents(t *testing.T) {
	p, _ := loadFixture(t)
	if len(p.Agents) != 3 {
		t.Fatalf("agents = %d, want 3 (README.txt skipped)", len(p.Agents))
	}
	de, pl, sc := p.Agents[0], p.Agents[1], p.Agents[2]
	if de.Name != "domain-engineer" || de.Tier != plugin.TierSonnet || de.ReadOnly || de.Color != "green" || de.Body != "You implement.\n" {
		t.Fatalf("domain-engineer = %+v", de)
	}
	if pl.Name != "planner" || pl.Tier != plugin.TierOpus || !pl.ReadOnly || pl.Description != `Plan phase: "quoted" plan.` ||
		!reflect.DeepEqual(pl.DisallowedTools, []string{"Edit", "NotebookEdit"}) || !strings.HasPrefix(pl.Body, "You plan.") {
		t.Fatalf("planner = %+v", pl)
	}
	if sc.Tier != plugin.TierInherit || !reflect.DeepEqual(sc.Tools, []string{"Read", "Grep"}) || sc.ReadOnly {
		t.Fatalf("scout = %+v", sc)
	}
}

func TestLoadHooks(t *testing.T) {
	p, _ := loadFixture(t)
	want := []plugin.Hook{
		{Event: plugin.EventSessionStart, Name: "session-start", TimeoutSec: 120},
		{Event: plugin.EventPreToolUse, Matcher: "Read|Bash", Name: "read-guard", TimeoutSec: 10},
		{Event: plugin.EventPreToolUse, Matcher: "Agent|Task", Name: "model-guard", TimeoutSec: 10},
		{Event: plugin.EventPostToolUse, Matcher: "Edit|Write", Name: "go-post-edit", TimeoutSec: 60},
		{Event: plugin.EventPostToolUse, Name: "trace", TimeoutSec: 10},
		{Event: plugin.EventPostToolUseFailure, Name: "trace", TimeoutSec: 10},
		{Event: plugin.EventSubagentStart, Name: "trace-agent", TimeoutSec: 10},
		{Event: plugin.EventSubagentStop, Name: "trace-agent", TimeoutSec: 10},
		{Event: plugin.EventStop, Name: "stop-gate", TimeoutSec: 10},
	}
	if !reflect.DeepEqual(p.Hooks, want) {
		t.Fatalf("hooks =\n%+v\nwant\n%+v", p.Hooks, want)
	}
}

func TestLoadDeterministic(t *testing.T) {
	a, _ := loadFixture(t)
	for i := 0; i < 20; i++ {
		b, _ := loadFixture(t)
		if !reflect.DeepEqual(a, b) {
			t.Fatal("Load is not deterministic")
		}
	}
}

func TestLoadErrors(t *testing.T) {
	cases := []struct {
		name, file, data, want string
		remove                 bool
	}{
		{name: "no manifest", file: ".claude-plugin/plugin.json", remove: true, want: "plugin.json"},
		{name: "bad manifest", file: ".claude-plugin/plugin.json", data: "{", want: ".claude-plugin/plugin.json"},
		{name: "manifest without version", file: ".claude-plugin/plugin.json", data: `{"name":"x"}`, want: "version"},
		{name: "bad author", file: ".claude-plugin/plugin.json", data: `{"name":"x","version":"1","author":3}`, want: "author"},
		{name: "skill name mismatch", file: "skills/work/SKILL.md", data: "---\nname: other\ndescription: d\n---\n", want: "skills/work/SKILL.md"},
		{name: "skill without description", file: "skills/work/SKILL.md", data: "---\nname: work\n---\n", want: "description"},
		{name: "skill bad frontmatter", file: "skills/work/SKILL.md", data: "no frontmatter", want: "skills/work/SKILL.md"},
		{name: "skill without SKILL.md", file: "skills/empty/notes.md", data: "x", want: "skills/empty/SKILL.md"},
		{name: "agent unknown tier", file: "agents/scout.md", data: "---\nname: scout\ndescription: d\nmodel: gpt\n---\n", want: "agents/scout.md: unknown model tier"},
		{name: "agent name mismatch", file: "agents/scout.md", data: "---\nname: spy\ndescription: d\n---\n", want: "agents/scout.md"},
		{name: "agent without name", file: "agents/scout.md", data: "---\ndescription: d\n---\n", want: "required"},
		{name: "agent bad tools", file: "agents/scout.md", data: "---\nname: scout\ndescription: d\ntools: Read,,Grep\n---\n", want: "tools"},
		{name: "agent bad disallowed", file: "agents/scout.md", data: "---\nname: scout\ndescription: d\ndisallowedTools: [Edit\n---\n", want: "disallowedTools"},
		{name: "agent bad yaml", file: "agents/scout.md", data: "---\nname: \"scout\n---\n", want: "agents/scout.md:2"},
		{name: "hooks bad json", file: "hooks/hooks.json", data: "{", want: "hooks/hooks.json"},
		{name: "hooks unknown event", file: "hooks/hooks.json", data: `{"hooks":{"Nope":[]}}`, want: `unknown event "Nope"`},
		{name: "hooks not loops", file: "hooks/hooks.json", data: `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"echo hi"}]}]}}`, want: "loops hook"},
		{name: "hooks prompt type", file: "hooks/hooks.json", data: `{"hooks":{"Stop":[{"hooks":[{"type":"prompt","command":"x"}]}]}}`, want: "unsupported hook type"},
		{name: "no hooks", file: "hooks/hooks.json", remove: true, want: "hooks/hooks.json"},
		{name: "no agents", file: "agents", remove: true, want: "agents"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := fixture()
			if tc.remove {
				for k := range src {
					if k == tc.file || strings.HasPrefix(k, tc.file+"/") {
						delete(src, k)
					}
				}
			} else {
				src[tc.file] = &fstest.MapFile{Data: []byte(tc.data)}
			}
			_, err := Load(src)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestLoadNoSkillsDir(t *testing.T) {
	src := fixture()
	for k := range src {
		if strings.HasPrefix(k, "skills/") {
			delete(src, k)
		}
	}
	if _, err := Load(src); err == nil {
		t.Fatal("Load without skills/ succeeded")
	}
}

func TestLoadStringFields(t *testing.T) {
	src := fixture()
	src[".claude-plugin/plugin.json"] = &fstest.MapFile{Data: []byte(`{"name":"x","version":"1","author":"Bo","repository":{"type":"git","url":"https://g"}}`)}
	p, err := Load(src)
	if err != nil {
		t.Fatal(err)
	}
	if p.Author != "Bo" || p.Repository != "https://g" {
		t.Fatalf("author %q repository %q", p.Author, p.Repository)
	}
}

func TestHookName(t *testing.T) {
	for cmd, want := range map[string]string{
		`"${X}/bin/loops" hook trace`:                      "trace",
		`"${X}/bin/loops" hook stop-gate --platform codex`: "stop-gate",
	} {
		got, err := hookName(cmd)
		if err != nil || got != want {
			t.Errorf("hookName(%q) = %q, %v", cmd, got, err)
		}
	}
	if _, err := hookName("loops hook --platform x"); err == nil {
		t.Error("flag taken as a hook name")
	}
}
