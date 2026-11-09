package port

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
)

// fixture is a small canonical plugin tree in the repository layout.
func fixture() fstest.MapFS {
	f := func(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s)} }
	return fstest.MapFS{
		".claude-plugin/plugin.json": f(`{"name":"loops","version":"0.2.0","description":"Desc <x> & y","author":{"name":"Ann"},
			"homepage":"https://h","repository":"https://r","license":"MIT","keywords":["go","qa"]}`),
		"skills/work/SKILL.md":            f("---\nname: work\ndescription: Work a ticket.\nargument-hint: \"<ticket> [--approve]\"\n---\n\n# Work\n"),
		"skills/work/platforms/codex.md":  f("codex notes\n"),
		"skills/work/platforms/cursor.md": f("cursor notes\n"),
		"skills/work/platforms/gemini.md": f("gemini notes\n"),
		"skills/init/SKILL.md":            f("---\nname: init\ndescription: Set up loops.\ndisable-model-invocation: true\n---\nInit body\n"),
		"skills/status/SKILL.md":          f("---\nname: status\ndescription: Show status.\n---\nStatus body\n"),
		"skills/report/SKILL.md":          f("---\nname: report\ndescription: 'Report''s skill'\nargument-hint: \"[run-id]\"\ndisable-model-invocation: true\n---\nReport body\n"),
		"skills/testing/SKILL.md":         f("---\nname: testing\ndescription: Use when testing.\n---\nTesting body\n"),
		"skills/testing/ref/extra.md":     f("extra\n"),
		"agents/planner.md":               f("---\nname: planner\ndescription: \"Plan phase: \\\"quoted\\\" plan.\"\nmodel: opus\ncolor: purple\ndisallowedTools: Edit, NotebookEdit\n---\n\nYou plan. Path C:\\\\x and \"\"\"triple\"\"\" quotes.\n"),
		"agents/domain-engineer.md":       f("---\nname: domain-engineer\ndescription: Implement domain.\nmodel: sonnet\ncolor: green\n---\nYou implement.\n"),
		"agents/scout.md":                 f("---\nname: scout\ndescription: Scout.\ntools: Read, Grep\n---\nScout body\n"),
		"agents/README.txt":               f("not an agent"),
		"hooks/hooks.json": f(`{"hooks":{
			"Stop":[{"hooks":[{"type":"command","command":"\"${CLAUDE_PLUGIN_ROOT}/bin/loops\" hook stop-gate","timeout":10}]}],
			"PreToolUse":[{"matcher":"Read|Bash","hooks":[{"type":"command","command":"\"${CLAUDE_PLUGIN_ROOT}/bin/loops\" hook read-guard","timeout":10}]},
			              {"matcher":"Agent|Task","hooks":[{"type":"command","command":"\"${CLAUDE_PLUGIN_ROOT}/bin/loops\" hook model-guard","timeout":10}]}],
			"PostToolUse":[{"matcher":"Edit|Write","hooks":[{"type":"command","command":"\"${CLAUDE_PLUGIN_ROOT}/bin/loops\" hook go-post-edit","timeout":60}]},
			               {"matcher":"*","hooks":[{"type":"command","command":"\"${CLAUDE_PLUGIN_ROOT}/bin/loops\" hook trace","timeout":10}]}],
			"PostToolUseFailure":[{"matcher":"*","hooks":[{"type":"command","command":"\"${CLAUDE_PLUGIN_ROOT}/bin/loops\" hook trace","timeout":10}]}],
			"SessionStart":[{"hooks":[{"type":"command","command":"\"${CLAUDE_PLUGIN_ROOT}/bin/loops\" hook session-start","timeout":120}]}],
			"SubagentStart":[{"hooks":[{"type":"command","command":"\"${CLAUDE_PLUGIN_ROOT}/bin/loops\" hook trace-agent","timeout":10}]}],
			"SubagentStop":[{"hooks":[{"type":"command","command":"\"${CLAUDE_PLUGIN_ROOT}/bin/loops\" hook trace-agent","timeout":10}]}]
		},"modules":["../mod/register.js"]}`),
		"specs/WORKFLOW.md":           f("workflow\n"),
		"templates/PLUGIN_CONTEXT.md": f("Always-on loops context.\n"),
		"templates/CLAIMS_REPORT.md":  f("claims\n"),
		"ROLES.md":                    f("roles\n"),
	}
}

func loadFixture(t *testing.T) (plugin.Plugin, fstest.MapFS) {
	t.Helper()
	src := fixture()
	p, err := Load(src)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return p, src
}

func build(t *testing.T, target plugin.Platform, o Options) (map[string]plugin.File, []string) {
	t.Helper()
	p, src := loadFixture(t)
	files, warns, err := Build(p, src, target, o)
	if err != nil {
		t.Fatalf("Build(%s): %v", target, err)
	}
	m := map[string]plugin.File{}
	for i, f := range files {
		if i > 0 && files[i-1].Path >= f.Path {
			t.Fatalf("files not sorted/unique at %q after %q", f.Path, files[i-1].Path)
		}
		m[f.Path] = f
	}
	return m, warns
}

func mustFile(t *testing.T, files map[string]plugin.File, name string) string {
	t.Helper()
	f, ok := files[name]
	if !ok {
		t.Fatalf("missing %s", name)
	}
	return string(f.Data)
}

func mustJSON(t *testing.T, files map[string]plugin.File, name string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(mustFile(t, files, name)), v); err != nil {
		t.Fatalf("%s: invalid JSON: %v", name, err)
	}
}

// parseTOML is a tiny decoder for the subset the emitters write:
// comments, `key = "basic"` and `key = """multi-line basic"""`.
func parseTOML(s string) (map[string]string, error) {
	out := map[string]string{}
	for len(s) > 0 {
		line, rest, _ := strings.Cut(s, "\n")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			s = rest
			continue
		}
		key, val, ok := strings.Cut(line, " = ")
		if !ok {
			return nil, fmt.Errorf("bad line %q", line)
		}
		if _, dup := out[key]; dup {
			return nil, fmt.Errorf("duplicate key %q", key)
		}
		if strings.HasPrefix(val, `"""`) {
			body := s[len(key)+len(" = ")+3:]
			body = strings.TrimPrefix(body, "\n")
			v, n, err := decodeBasic(body, true)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", key, err)
			}
			out[key] = v
			s = body[n:]
			if !strings.HasPrefix(s, "\n") && s != "" {
				return nil, fmt.Errorf("%s: trailing data after string", key)
			}
			continue
		}
		if !strings.HasPrefix(val, `"`) {
			return nil, fmt.Errorf("%s: unsupported value %q", key, val)
		}
		v, n, err := decodeBasic(val[1:], false)
		if err != nil || n != len(val)-1 {
			return nil, fmt.Errorf("%s: bad basic string %q: %v", key, val, err)
		}
		out[key] = v
		s = rest
	}
	return out, nil
}

// decodeBasic decodes up to the closing delimiter (`"` or `"""`) and
// returns the value and the bytes consumed including the delimiter.
func decodeBasic(s string, multi bool) (string, int, error) {
	var b strings.Builder
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\\':
			if i+1 >= len(s) {
				return "", 0, fmt.Errorf("dangling escape")
			}
			switch s[i+1] {
			case '\\', '"':
				b.WriteByte(s[i+1])
				i += 2
			case 'n':
				b.WriteByte('\n')
				i += 2
			case 't':
				b.WriteByte('\t')
				i += 2
			case 'u':
				r, err := strconv.ParseUint(s[i+2:i+6], 16, 32)
				if err != nil {
					return "", 0, err
				}
				b.WriteRune(rune(r))
				i += 6
			default:
				return "", 0, fmt.Errorf("bad escape \\%c", s[i+1])
			}
		case c == '"' && !multi:
			return b.String(), i + 1, nil
		case c == '"' && strings.HasPrefix(s[i:], `"""`):
			// Up to two extra quotes before the delimiter belong to the value.
			j := i
			for j < len(s) && s[j] == '"' {
				j++
			}
			if j-i > 5 {
				return "", 0, fmt.Errorf("too many quotes")
			}
			b.WriteString(s[i : j-3])
			return b.String(), j, nil
		case c == '\n' && !multi:
			return "", 0, fmt.Errorf("newline in basic string")
		case c < 0x20 && c != '\t' && c != '\n':
			return "", 0, fmt.Errorf("control character %#x", c)
		default:
			b.WriteByte(c)
			i++
		}
	}
	return "", 0, fmt.Errorf("unterminated string")
}
