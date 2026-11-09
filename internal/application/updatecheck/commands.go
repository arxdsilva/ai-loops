package updatecheck

import "strings"

const installScript = "curl -fsSL https://raw.githubusercontent.com/arxdsilva/ai-loops/main/install.sh | sh -s --"

// installer is the update route for platforms installed by `loops install`.
func installer(platform string) string {
	return installScript + " --target " + platform + " --force"
}

var commands = map[string]string{
	"claude":   "claude plugin marketplace update ai-loops && claude plugin update loops@ai-loops",
	"copilot":  "copilot plugin marketplace update ai-loops && copilot plugin update loops",
	"codex":    "codex plugin marketplace upgrade && " + installer("codex"),
	"cursor":   "Cursor: Dashboard > Plugins & MCPs > open the marketplace > Refresh, then start a new agent",
	"gemini":   "git -C ai-loops pull && gemini extensions update loops",
	"opencode": installer("opencode"),
	"amp":      installer("amp"),
	"windsurf": installer("windsurf"),
	"cline":    installer("cline"),
}

// Command returns the one-line update command for a platform. Unknown or
// empty platforms get the generic install.sh line.
func Command(platform string) string {
	p := strings.ToLower(strings.TrimSpace(platform))
	if c, ok := commands[p]; ok {
		return c
	}
	if p == "" {
		p = "<platform>"
	}
	return installer(p)
}

// Commands returns a copy of the per-platform table (for `loops update`).
func Commands() map[string]string {
	out := make(map[string]string, len(commands))
	for k, v := range commands {
		out[k] = v
	}
	return out
}
