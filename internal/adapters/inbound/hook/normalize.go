package hook

import (
	"encoding/json"
	"regexp"
	"strings"
)

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
	// Antigravity
	"view_file": "Read", "run_command": "Bash", "replace_file_content": "Edit",
	"multi_replace_file_content": "MultiEdit", "write_to_file": "Write",
	"find_by_name": "Glob", "invoke_subagent": "Agent",
	// Copilot camelCase
	"powershell": "Bash", "view": "Read", "create": "Write", "task": "Agent",
	// lowercase canonical
	"read": "Read", "bash": "Bash", "edit": "Edit", "write": "Write", "multiedit": "MultiEdit",
	"agent": "Agent", "grep": "Grep", "webfetch": "WebFetch", "websearch": "WebSearch",
}

var patchFile = regexp.MustCompile(`(?m)^\*\*\* (?:Add|Update|Delete) File: (.+?)\s*$`)

var platforms = map[string]string{
	"claude": "claude", "copilot": "copilot", "codex": "codex", "cursor": "cursor",
	"gemini": "gemini", "windsurf": "windsurf", "devin": "windsurf", "opencode": "opencode",
	"antigravity": "antigravity", "agy": "antigravity", "antigravity-cli": "antigravity",
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

func canonTool(n string) string {
	if c, ok := toolNames[n]; ok {
		return c
	}
	if rest, ok := strings.CutPrefix(n, "MCP:"); ok {
		return "mcp__" + rest
	}
	return n
}

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

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstElem(ss []string) string {
	if len(ss) > 0 {
		return ss[0]
	}
	return ""
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, _ := m[k].(string); s != "" {
			return s
		}
	}
	return ""
}

// normalize rewrites p in place into the canonical (Claude) vocabulary and
// returns the output dialect and whether the tool call failed.
func normalize(p *payload, platform string) (dialect, bool) {
	d := dialect{platform: platform}
	if platform == "copilot" && p.Event == "" {
		d.camel = true
	}
	normalizeHeader(p, platform)
	normalizeToolInput(p)
	return d, normalizeFailed(p, platform)
}

func normalizeHeader(p *payload, platform string) {
	if e, ok := eventNames[p.Event]; ok {
		p.Event = e
	}
	p.SessionID = firstNonEmpty(p.SessionID, p.SessionIDCamel, p.ConversationID)
	var tcName string
	if p.ToolCall != nil {
		tcName = p.ToolCall.Name
	}
	p.ToolName = canonTool(firstNonEmpty(p.ToolName, p.ToolNameCamel, tcName))
	p.AgentID = firstNonEmpty(p.AgentID, p.SubagentID)
	p.AgentType = firstNonEmpty(p.AgentType, p.SubagentType)
	if p.DurationMS == 0 {
		p.DurationMS = p.Duration
	}
	if p.StopHookActiveCamel || (platform == "cursor" && p.Event == "Stop" && p.LoopCount > 0) {
		p.StopHookActive = true
	}
	p.Cwd = firstNonEmpty(p.Cwd, firstElem(p.WorkspaceRoots), firstElem(p.WorkspacePaths))
}

func decodeToolInput(p *payload) map[string]any {
	raw := p.ToolInput
	if !nonEmpty(raw) {
		raw = p.ToolArgs
	}
	if !nonEmpty(raw) && p.ToolCall != nil {
		raw = p.ToolCall.Args
	}
	in := map[string]any{}
	if !nonEmpty(raw) {
		return in
	}
	if json.Unmarshal(raw, &in) != nil {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			_ = json.Unmarshal([]byte(s), &in)
		}
	}
	return in
}

func normalizeToolInput(p *payload) {
	in := decodeToolInput(p)
	switch p.ToolName {
	case "Read", "Edit", "Write", "MultiEdit":
		normalizeFileInput(p, in)
	case "Bash":
		if cmd := firstString(in, "command", "CommandLine", "command_line", "cmd"); cmd != "" {
			in["command"] = cmd
		}
	}
	if len(in) > 0 {
		if b, err := json.Marshal(in); err == nil {
			p.ToolInput = b
		}
	}
}

func normalizeFileInput(p *payload, in map[string]any) {
	if fp := firstString(in, "file_path", "absolute_path", "AbsolutePath", "path", "Path", "file", "filePath", "target_file", "TargetFile"); fp != "" {
		in["file_path"] = fp
		return
	}
	if p.ToolName != "Edit" {
		return
	}
	cmd, _ := in["command"].(string)
	if f := patchTarget(cmd); f != "" {
		in["file_path"] = f
		p.extraFiles = patchFiles(cmd)
		delete(in, "command")
	}
}

func normalizeFailed(p *payload, platform string) bool {
	if p.Event == "PostToolUseFailure" || p.IsInterrupt {
		return true
	}
	if p.ToolName == "" && p.Event != "PostToolUse" {
		return false
	}
	return responseFailed(p.ToolResponse, platform) || responseFailed(p.ToolResult, platform)
}

func responseFailed(r json.RawMessage, platform string) bool {
	var resp struct {
		Error       json.RawMessage `json:"error"`
		Interrupted bool            `json:"interrupted"`
		ResultType  string          `json:"result_type"`
	}
	if json.Unmarshal(r, &resp) != nil {
		return false
	}
	return (platform == "gemini" && nonEmpty(resp.Error)) ||
		resp.Interrupted ||
		resp.ResultType == "failure" ||
		resp.ResultType == "error"
}
