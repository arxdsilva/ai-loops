package hook

// Verified platform facts (official docs read 2026-11-09). The lead copies
// this block back into specs/PLATFORMS.md §5.
//
// CURSOR (https://cursor.com/docs/hooks)
//   - Events (camelCase): sessionStart, sessionEnd, preToolUse, postToolUse,
//     postToolUseFailure, subagentStart, subagentStop, stop, beforeShellExecution,
//     afterFileEdit, ... (21 total).
//   - Common input: conversation_id, generation_id, model, hook_event_name,
//     cursor_version, workspace_roots, transcript_path. cwd is present on the
//     tool events only.
//   - sessionStart output: {"env":{...},"additional_context":"..."} (VERIFIED).
//   - preToolUse input: tool_name, tool_input, tool_use_id, cwd. Documented
//     tool_name values: Shell, Read, Write, Grep, Delete, Task, MCP:<tool>.
//     "Terminal" and "StrReplace" are NOT documented. Only Shell has documented
//     tool_input fields (command, working_directory). Output: permission
//     allow|deny, user_message, agent_message, updated_input.
//   - postToolUse input adds tool_output (JSON string), duration (ms); output:
//     additional_context (VERIFIED), updated_mcp_tool_output (MCP only).
//   - postToolUseFailure input: error_message, failure_type
//     (error|timeout|permission_denied), duration, is_interrupt; output
//     additional_context.
//   - subagentStart input: subagent_id, subagent_type, task, parent_conversation_id.
//     subagentStop input: subagent_type, status, task, summary, duration_ms,
//     loop_count, modified_files. subagentStop has NO subagent id.
//   - stop input: status (completed|aborted|error), loop_count (starts 0);
//     output {"followup_message":R} submitted as next user message (VERIFIED);
//     default loop_limit 5. There is no stop_hook_active; loop_count > 0 is the
//     equivalent.
//   - Exit 2 = deny (VERIFIED). Other non-zero = fail-open.
//   - Env: CURSOR_PROJECT_DIR, CURSOR_PLUGIN_ROOT (plugin hooks), and
//     CLAUDE_PROJECT_DIR as an alias of CURSOR_PROJECT_DIR.
//
// GEMINI CLI (https://geminicli.com/docs/hooks/reference/ and /docs/hooks/)
//   - Events: SessionStart, BeforeTool, AfterTool, AfterAgent (and others).
//   - Common input: session_id, transcript_path, cwd, hook_event_name, timestamp.
//   - BeforeTool: tool_name, tool_input, mcp_context. AfterTool adds
//     tool_response {llmContent, returnDisplay, error?}; the shape of error is
//     not documented (we treat any non-empty/non-null value as failure).
//     AfterAgent: prompt, prompt_response, stop_hook_active (VERIFIED).
//     SessionStart: source (startup|resume|clear).
//   - Outputs: decision "deny" (alias "block") + reason. BeforeTool deny blocks.
//     AfterAgent deny forces a retry with reason as the new prompt (VERIFIED).
//     AfterTool deny HIDES the real tool output and replaces it with reason, so
//     the post-edit failure uses hookSpecificOutput.additionalContext instead
//     (appended; VERIFIED). SessionStart: hookSpecificOutput.additionalContext
//     (VERIFIED; decision/continue ignored).
//   - Exit 0 = JSON parsed; 2 = block with stderr reason; other = warning.
//   - Env (https://geminicli.com/docs/hooks/): GEMINI_PROJECT_DIR, GEMINI_CWD,
//     GEMINI_SESSION_ID, GEMINI_PLANS_DIR; CLAUDE_PROJECT_DIR is an alias.
//   - Built-in tool names/tool_input field names are NOT on the docs pages
//     (only write_file, replace, read_file, run_shell_command appear);
//     the spec §5 mapping is kept as unverified and file_path is also read
//     from absolute_path/path.
//
// CODEX (https://learn.chatgpt.com/docs/hooks)
//   - Events: PreToolUse, PostToolUse, SubagentStart, SubagentStop, Stop,
//     SessionStart, ... (PascalCase).
//   - Common input: session_id, transcript_path, cwd, hook_event_name, model,
//     turn_id.
//   - Shell tool_name is "Bash" (tool_input.command). apply_patch tool_name is
//     "apply_patch" with tool_input.command holding the PATCH TEXT (no
//     file_path); matchers accept Edit/Write as aliases. So the paths are
//     parsed from "*** Add/Update/Delete File:" lines.
//   - SubagentStart: agent_id, agent_type. SubagentStop: agent_id, agent_type,
//     stop_hook_active, last_assistant_message.
//   - Stop: stop_hook_active, last_assistant_message; block with
//     {"decision":"block","reason":R} (VERIFIED) or exit 2.
//   - SessionStart: {"hookSpecificOutput":{"hookEventName":"SessionStart",
//     "additionalContext":C}} (VERIFIED). PreToolUse deny: exit 2 + stderr, or
//     hookSpecificOutput.permissionDecision "deny", or {"decision":"block"}.
//   - Env: PLUGIN_ROOT, PLUGIN_DATA, CLAUDE_PLUGIN_ROOT, CLAUDE_PLUGIN_DATA.
//     No project-dir variable is documented; use payload cwd.
//
// COPILOT (https://docs.github.com/en/copilot/reference/hooks-configuration)
//   - Two dialects. camelCase events (preToolUse, postToolUse, agentStop,
//     subagentStop, sessionStart) use camelCase payloads WITHOUT
//     hook_event_name: sessionId, timestamp(number), cwd, toolName, toolArgs,
//     toolResult{text_result_for_llm,result_type}, agentStop: transcriptPath,
//     stopReason, stopHookActive. PascalCase events (PreToolUse, Stop,
//     SessionStart) use the VS Code/Claude snake_case payload with Claude tool
//     names (bash -> "Bash").
//   - camelCase outputs: preToolUse deny = exit 2 (or permissionDecision
//     "deny"); agentStop/subagentStop block = {"decision":"block","reason":R};
//     sessionStart/postToolUse context = top-level {"additionalContext":C}.
//     PascalCase uses the Claude shapes. Non-2 non-zero on preToolUse fails
//     CLOSED. After 8 consecutive stop blocks the turn ends anyway.
//   - Copilot is therefore NOT identical to Claude for camelCase events.
//
// DEVIN / WINDSURF (https://docs.devin.ai/cli/extensibility/hooks.md)
//   - Events: PreToolUse, PostToolUse, PermissionRequest, UserPromptSubmit,
//     Stop, PostCompaction, SessionStart, SessionEnd. There are NO subagent
//     events.
//   - Payload: hook_event_name, tool_name (documented: "exec"), tool_input
//     ({"command":...} for exec), session_id, prompt_id. No cwd and no
//     stop_hook_active are documented.
//   - Output: top-level decision "approve"|"block" + reason;
//     hookSpecificOutput{hookEventName,additionalContext} on UserPromptSubmit,
//     SessionStart, PostToolUse. Exit 2 = block.
//   - Env: DEVIN_PROJECT_DIR (only variable documented).
//   - Windsurf itself is not covered by that page.

import (
	"encoding/json"
	"fmt"
	"io"
)

// dialect captures how a platform wants decisions encoded.
type dialect struct {
	platform string
	camel    bool // Copilot camelCase event family
}

func (d dialect) emit(out io.Writer, v any) {
	b, err := json.Marshal(v)
	if err == nil {
		fmt.Fprintln(out, string(b))
	}
}

func (d dialect) sessionContext(out io.Writer, text string) {
	switch {
	case d.platform == "cursor":
		d.emit(out, map[string]string{"additional_context": text})
	case d.camel:
		d.emit(out, map[string]string{"additionalContext": text})
	default:
		d.emit(out, map[string]any{"hookSpecificOutput": map[string]string{"hookEventName": "SessionStart", "additionalContext": text}})
	}
}

func (d dialect) postEditFailure(out io.Writer, reason string) {
	switch {
	case d.platform == "cursor":
		d.emit(out, map[string]string{"additional_context": reason})
	case d.platform == "gemini":
		d.emit(out, map[string]any{"hookSpecificOutput": map[string]string{"hookEventName": "AfterTool", "additionalContext": reason}})
	case d.camel:
		d.emit(out, map[string]string{"additionalContext": reason})
	default:
		d.emit(out, map[string]string{"decision": "block", "reason": reason})
	}
}

func (d dialect) blockStop(out io.Writer, reason string) {
	switch d.platform {
	case "cursor":
		d.emit(out, map[string]string{"followup_message": reason})
	case "gemini":
		d.emit(out, map[string]string{"decision": "deny", "reason": reason})
	default:
		d.emit(out, map[string]string{"decision": "block", "reason": reason})
	}
}
