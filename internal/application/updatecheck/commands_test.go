package updatecheck

import (
	"strings"
	"testing"
)

func TestCommand(t *testing.T) {
	tests := []struct{ platform, want string }{
		{"claude", "claude plugin update loops@ai-loops"},
		{"copilot", "copilot plugin update loops"},
		{"codex", "install.sh | sh -s -- --target codex --force"},
		{"cursor", "Refresh"},
		{"gemini", "gemini extensions update loops"},
		{"opencode", "install.sh | sh -s -- --target opencode --force"},
		{"amp", "install.sh | sh -s -- --target amp --force"},
		{"windsurf", "install.sh | sh -s -- --target windsurf --force"},
		{"cline", "install.sh | sh -s -- --target cline --force"},
		{"Claude ", "claude plugin update"},
		{"zed", "curl -fsSL https://raw.githubusercontent.com/arxdsilva/ai-loops/main/install.sh | sh -s -- --target zed --force"},
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
