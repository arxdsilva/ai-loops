package loops

import (
	"testing"
	"testing/fstest"

	ailoops "github.com/arxdsilva/ai-loops"
)

func TestAgentTiers(t *testing.T) {
	src := fstest.MapFS{
		"agents/a.md":     {Data: []byte("---\nname: a\nmodel: sonnet\n---\nbody model: opus\n")},
		"agents/b.md":     {Data: []byte("---\r\nmodel: \"Opus\"\r\n---\r\n")},
		"agents/nofm.md":  {Data: []byte("model: opus\n")},
		"agents/nomod.md": {Data: []byte("---\nname: x\n---\nmodel: opus\n")},
	}
	of := agentTiers(src)
	for role, want := range map[string]string{
		"a": "sonnet", "b": "opus", "nofm": "", "nomod": "", "missing": "", "": "", "../x": "", "a/b": "",
	} {
		if got := of(role); got != want {
			t.Errorf("tier(%q) = %q, want %q", role, got, want)
		}
	}
}

// The embedded plugin carries a tier for every role the guard can see.
func TestAgentTiersEmbedded(t *testing.T) {
	of := agentTiers(ailoops.Source)
	for role, want := range map[string]string{"planner": "opus", "domain-engineer": "sonnet", "code-researcher": "haiku"} {
		if got := of(role); got != want {
			t.Errorf("tier(%q) = %q, want %q", role, got, want)
		}
	}
}
