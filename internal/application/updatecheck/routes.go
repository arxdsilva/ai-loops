package updatecheck

import "strings"

// routes are the automatic update steps per platform, as argv lists (never
// shell strings, so `&&` or `| sh` cannot be executed by construction).
var routes = map[string][][]string{
	"claude": {
		{"claude", "plugin", "marketplace", "update", "ai-loops"},
		{"claude", "plugin", "update", "loops@ai-loops"},
	},
	"copilot": {
		{"copilot", "plugin", "marketplace", "update", "ai-loops"},
		{"copilot", "plugin", "update", "loops"},
	},
}

func normPlatform(p string) string { return strings.ToLower(strings.TrimSpace(p)) }

// Route returns a copy of the platform's automatic update steps as argv
// lists (never shell strings), and true only for platforms that have one
// (claude, copilot).
func Route(platform string) ([][]string, bool) {
	r, ok := routes[normPlatform(platform)]
	if !ok {
		return nil, false
	}
	out := make([][]string, len(r))
	for i, step := range r {
		out[i] = append([]string(nil), step...)
	}
	return out, true
}

// joinRoute renders a route as the one-line display command.
func joinRoute(r [][]string) string {
	steps := make([]string, len(r))
	for i, step := range r {
		steps[i] = strings.Join(step, " ")
	}
	return strings.Join(steps, " && ")
}

// Reload returns how the user loads the updated plugin on platform.
func Reload(platform string) string {
	switch normPlatform(platform) {
	case "claude":
		return "/reload-plugins or a new session"
	case "copilot":
		return "/restart or a new session"
	}
	return "a new session"
}
