package hook

import (
	"encoding/json"
	"regexp"
	"strings"
)

var platforms = map[string]string{
	"claude": "claude", "copilot": "copilot", "codex": "codex", "cursor": "cursor",
	"gemini": "gemini", "windsurf": "windsurf", "devin": "windsurf", "opencode": "opencode",
}

// detect picks the platform from the flag, else the environment (PLATFORMS.md §5).
func detect(flag string, env func(string) string) string {
	if p, ok := platforms[strings.ToLower(flag)]; ok {
		return p
	}
	switch {
	case env("CURSOR_PLUGIN_ROOT") != "" || env("CURSOR_PROJECT_DIR") != "":
		return "cursor"
	case env("GEMINI_PROJECT_DIR") != "" || env("GEMINI_CWD") != "":
		return "gemini"
	case env("DEVIN_PROJECT_DIR") != "":
		return "windsurf"
	case env("PLUGIN_ROOT") != "" && env("CLAUDE_PROJECT_DIR") == "":
		return "codex"
	}
	return "claude"
}

var eventNames = map[string]string{
	"sessionStart": "SessionStart", "preToolUse": "PreToolUse", "postToolUse": "PostToolUse",
	"postToolUseFailure": "PostToolUseFailure", "subagentStart": "SubagentStart",
	"subagentStop": "SubagentStop", "stop": "Stop", "agentStop": "Stop",
	"BeforeTool": "PreToolUse", "AfterTool": "PostToolUse", "AfterAgent": "Stop",
}

var toolNames = map[string]string{
	// Gemini (unverified by docs)
	"read_file": "Read", "read_many_files": "Read", "run_shell_command": "Bash", "replace": "Edit",
	"write_file": "Write", "search_file_content": "Grep", "grep_search": "Grep", "glob": "Glob",
	"web_fetch": "WebFetch", "google_web_search": "WebSearch",
	// Cursor
	"Shell": "Bash", "Terminal": "Bash", "StrReplace": "Edit", "Task": "Agent",
	// Codex
	"shell": "Bash", "exec_command": "Bash", "local_shell": "Bash", "apply_patch": "Edit",
	// Devin
	"exec": "Bash",
	// Copilot camelCase
	"powershell": "Bash", "view": "Read", "create": "Write", "task": "Agent",
	// lowercase canonical
	"read": "Read", "bash": "Bash", "edit": "Edit", "write": "Write", "multiedit": "MultiEdit",
	"agent": "Agent", "grep": "Grep", "webfetch": "WebFetch", "websearch": "WebSearch",
}

func canonTool(n string) string {
	if c, ok := toolNames[n]; ok {
		return c
	}
	if rest, ok := strings.CutPrefix(n, "MCP:"); ok {
		return "mcp__" + rest
	}
	return n
}

var patchFile = regexp.MustCompile(`(?m)^\*\*\* (?:Add|Update|Delete) File: (.+?)\s*$`)

// patchFiles lists the files an apply_patch touches, in order.
func patchFiles(patch string) []string {
	var fs []string
	for _, m := range patchFile.FindAllStringSubmatch(patch, -1) {
		fs = append(fs, m[1])
	}
	return fs
}

// patchTarget picks the file an apply_patch touches, preferring .go files.
func patchTarget(patch string) string {
	fs := patchFiles(patch)
	for _, f := range fs {
		if strings.HasSuffix(f, ".go") {
			return f
		}
	}
	if len(fs) > 0 {
		return fs[0]
	}
	return ""
}

func nonEmpty(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return s != "" && s != "null" && s != `""` && s != "false" && s != "{}"
}

// normalize rewrites p in place into the canonical (Claude) vocabulary and
// returns the output dialect and whether the tool call failed.
func normalize(p *payload, platform string) (dialect, bool) {
	d := dialect{platform: platform}
	if platform == "copilot" && p.Event == "" {
		d.camel = true
	}
	if e, ok := eventNames[p.Event]; ok {
		p.Event = e
	}
	if p.SessionID == "" {
		p.SessionID = p.SessionIDCamel
	}
	if p.ToolName == "" {
		p.ToolName = p.ToolNameCamel
	}
	p.ToolName = canonTool(p.ToolName)
	if p.AgentID == "" {
		p.AgentID = p.SubagentID
	}
	if p.AgentType == "" {
		p.AgentType = p.SubagentType
	}
	if p.DurationMS == 0 {
		p.DurationMS = p.Duration
	}
	if p.StopHookActiveCamel {
		p.StopHookActive = true
	}
	if platform == "cursor" && p.Event == "Stop" && p.LoopCount > 0 {
		p.StopHookActive = true
	}
	if p.Cwd == "" && len(p.WorkspaceRoots) > 0 {
		p.Cwd = p.WorkspaceRoots[0]
	}

	in := map[string]any{}
	raw := p.ToolInput
	if !nonEmpty(raw) {
		raw = p.ToolArgs
	}
	if nonEmpty(raw) {
		if json.Unmarshal(raw, &in) != nil {
			var s string
			if json.Unmarshal(raw, &s) == nil {
				_ = json.Unmarshal([]byte(s), &in)
			}
		}
	}
	str := func(k string) string { s, _ := in[k].(string); return s }
	switch p.ToolName {
	case "Read", "Edit", "Write", "MultiEdit":
		if str("file_path") == "" {
			for _, k := range []string{"absolute_path", "path", "file", "filePath", "target_file"} {
				if v := str(k); v != "" {
					in["file_path"] = v
					break
				}
			}
		}
		if p.ToolName == "Edit" && str("file_path") == "" {
			if f := patchTarget(str("command")); f != "" {
				in["file_path"] = f
				p.extraFiles = patchFiles(str("command"))
				delete(in, "command")
			}
		}
	}
	if len(in) > 0 {
		if b, err := json.Marshal(in); err == nil {
			p.ToolInput = b
		}
	}

	failed := p.Event == "PostToolUseFailure" || p.IsInterrupt
	if p.ToolName != "" || p.Event == "PostToolUse" || p.Event == "PostToolUseFailure" {
		var resp struct {
			Error       json.RawMessage `json:"error"`
			Interrupted bool            `json:"interrupted"`
			ResultType  string          `json:"result_type"`
		}
		for _, r := range []json.RawMessage{p.ToolResponse, p.ToolResult} {
			if json.Unmarshal(r, &resp) == nil {
				if (platform == "gemini" && nonEmpty(resp.Error)) || resp.Interrupted ||
					resp.ResultType == "failure" || resp.ResultType == "error" {
					failed = true
				}
			}
		}
	}
	return d, failed
}
