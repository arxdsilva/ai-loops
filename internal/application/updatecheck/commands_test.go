package updatecheck

import (
	"strings"
	"testing"
)

func TestCommand(t *testing.T) {
	tests := []struct{ platform, want string }{
		{"claude", "claude plugin update loops@ai-loops"},
		{"copilot", "copilot plugin update loops"},
		{"codex", "loops install --target codex --force"},
		{"cursor", "Refresh"},
		{"gemini", "gemini extensions update loops"},
		{"opencode", "loops install --target opencode --force"},
		{"amp", "loops install --target amp --force"},
		{"windsurf", "loops install --target windsurf --force"},
		{"cline", "loops install --target cline --force"},
		{"Claude ", "claude plugin update"},
		{"zed", "go install github.com/arxdsilva/ai-loops/cmd/loops@latest && loops install --target zed --force"},
	}
	for _, tt := range tests {
		if got := Command(tt.platform); !strings.Contains(got, tt.want) {
			t.Errorf("Command(%q) = %q, want substring %q", tt.platform, got, tt.want)
		}
	}
}

func TestCommandsCopy(t *testing.T) {
	m := Commands()
	if len(m) != 9 {
		t.Fatalf("len = %d, want 9", len(m))
	}
	m["claude"] = "x"
	if Command("claude") == "x" {
		t.Fatal("Commands must return a copy")
	}
}
