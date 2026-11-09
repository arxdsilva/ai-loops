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
//
// PRETOOLUSE INPUT REWRITING (go-env hook; docs read 2026-11-09)
//   - CLAUDE (https://code.claude.com/docs/en/hooks.md, "PreToolUse decision
//     control"): hookSpecificOutput.updatedInput "Replaces the entire input
//     object, so include unchanged fields"; permission rules are evaluated
//     against the returned input. permissionDecision "allow" skips the
//     permission prompt (deny/ask rules still apply); with no decision the
//     normal flow runs. Matching hooks run in parallel; precedence
//     deny > defer > ask > allow, so race-guard's deny wins. A plugin
//     command hook may change Bash input without the auto-mode re-review
//     denial. Allow-rule matching only strips known-safe env assignments
//     (permissions.md), so GOCACHE=… prefixes would miss Bash(go test *)
//     rules: go-env adds "allow" only for a lone go build|test|vet|list.
//     -> REWRITE (VERIFIED).
//   - CODEX (https://learn.chatgpt.com/docs/hooks): "return
//     permissionDecision: "allow" with updatedInput"; for Bash it "must
//     include a string command field"; "Return updatedInput only with
//     permissionDecision: "allow"; other updatedInput shapes are reported as
//     errors." -> REWRITE+ALLOW for a lone gate command, else DENY (VERIFIED).
//   - CURSOR (https://cursor.com/docs/hooks): preToolUse output
//     {"permission":"allow","updated_input":{...}} ("Modified tool input to
//     use instead"); permission is not marked optional; replace-vs-merge is
//     not documented (we send the whole input). beforeShellExecution has no
//     rewrite field. -> REWRITE+ALLOW for a lone gate command, else DENY.
//   - GEMINI (https://geminicli.com/docs/hooks/reference/): BeforeTool
//     hookSpecificOutput.tool_input "merges with and overrides the model's
//     arguments"; run_shell_command params are command, description,
//     dir_path, is_background (https://geminicli.com/docs/tools/shell/).
//     -> REWRITE, no decision (VERIFIED).
//   - DEVIN/WINDSURF (https://docs.devin.ai/cli/extensibility/hooks.md):
//     hookSpecificOutput.updatedInput "merged into the tool's arguments";
//     exec tool_input.command; the documented example has no decision.
//     -> REWRITE (VERIFIED).
//   - COPILOT (https://docs.github.com/en/copilot/reference/hooks-configuration):
//     preToolUse documents modifiedArgs ("Substitute tool arguments"), but
//     the bash/powershell toolArgs field names are not documented and the
//     PascalCase variant says nothing about it. -> DENY (unverified).
//   - OPENCODE: its plugin does not call go-env. -> DENY if invoked.

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

// sessionContext emits the session-start context; msg, when non-empty, is
// also shown to the user on platforms with a user-visible message field
// (Claude, Codex, Copilot: top-level systemMessage).
func (d dialect) sessionContext(out io.Writer, text, msg string) {
	var v map[string]any
	switch {
	case d.platform == "cursor":
		v = map[string]any{"additional_context": text}
	case d.camel:
		v = map[string]any{"additionalContext": text}
	default:
		v = map[string]any{"hookSpecificOutput": map[string]string{"hookEventName": "SessionStart", "additionalContext": text}}
	}
	switch d.platform {
	case "claude", "codex", "copilot":
		if msg != "" {
			v["systemMessage"] = msg
		}
	}
	d.emit(out, v)
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

// rewriteTool replaces a PreToolUse call's input (the whole object, with
// unchanged fields kept, since Claude and possibly Cursor replace rather
// than merge); allow also approves the rewritten call.
func (d dialect) rewriteTool(out io.Writer, input map[string]any, allow bool) {
	switch d.platform {
	case "cursor":
		d.emit(out, map[string]any{"permission": "allow", "updated_input": input})
	case "gemini":
		d.emit(out, map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": "BeforeTool", "tool_input": input}})
	default:
		hso := map[string]any{"hookEventName": "PreToolUse", "updatedInput": input}
		if allow {
			hso["permissionDecision"] = "allow"
			hso["permissionDecisionReason"] = "go-env: go command scoped to the run's Go cache"
		}
		d.emit(out, map[string]any{"hookSpecificOutput": hso})
	}
}
