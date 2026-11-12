package updatecheck

import (
	"reflect"
	"testing"
)

func TestRoutes(t *testing.T) {
	r, ok := Route(" Claude ")
	want := [][]string{
		{"claude", "plugin", "marketplace", "update", "ai-loops"},
		{"claude", "plugin", "update", "loops@ai-loops"},
	}
	if !ok || !reflect.DeepEqual(r, want) {
		t.Fatalf("claude route = %v %v", r, ok)
	}
	r[0][0] = "x" // a copy
	if r2, _ := Route("claude"); r2[0][0] != "claude" {
		t.Fatal("Route leaked internal state")
	}
	if c, ok := Route("copilot"); !ok || c[1][3] != "loops" {
		t.Fatalf("copilot route = %v", c)
	}
	for _, p := range []string{"cursor", "gemini", "codex", "opencode", ""} {
		if r, ok := Route(p); ok || r != nil {
			t.Errorf("Route(%q) = %v %v", p, r, ok)
		}
	}
}

func TestReload(t *testing.T) {
	cases := map[string]string{
		"claude": "/reload-plugins or a new session", "COPILOT": "/restart or a new session",
		"cursor": "a new session", "": "a new session",
	}
	for p, want := range cases {
		if got := Reload(p); got != want {
			t.Errorf("Reload(%q) = %q, want %q", p, got, want)
		}
	}
}

func TestDisplayLinesDeriveFromRoutes(t *testing.T) {
	for _, p := range []string{"claude", "copilot"} {
		r, _ := Route(p)
		if got := Command(p); got != joinRoute(r) {
			t.Errorf("Command(%s) = %q, want %q", p, got, joinRoute(r))
		}
	}
	if Command("claude") != "claude plugin marketplace update ai-loops && claude plugin update loops@ai-loops" {
		t.Error("claude display line changed")
	}
	if Command("copilot") != "copilot plugin marketplace update ai-loops && copilot plugin update loops" {
		t.Error("copilot display line changed")
	}
}
