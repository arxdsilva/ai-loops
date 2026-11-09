// Package hook translates Claude Code hook events (JSON on stdin) into
// guard decisions and run events. Every handler fails open: bad input,
// no active run, or an internal error exits 0 with no output.
//
// Names (hooks/hooks.json): session-start, read-guard, model-guard,
// go-post-edit, trace, trace-agent, stop-gate.
package hook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/insurgencygames/ai-loops/internal/application/guard"
	"github.com/insurgencygames/ai-loops/internal/domain/run"
)

// Handler dispatches hook names.
type Handler struct{ d Deps }

// New returns a Handler.
func New(d Deps) *Handler {
	if d.Env == nil {
		d.Env = func(string) string { return "" }
	}
	if d.Getwd == nil {
		d.Getwd = func() (string, error) { return "", nil }
	}
	return &Handler{d: d}
}

// payload is the subset of the hook JSON we read.
type payload struct {
	SessionID      string          `json:"session_id"`
	Cwd            string          `json:"cwd"`
	Event          string          `json:"hook_event_name"`
	ToolName       string          `json:"tool_name"`
	ToolInput      json.RawMessage `json:"tool_input"`
	ToolResponse   json.RawMessage `json:"tool_response"`
	DurationMS     float64         `json:"duration_ms"`
	AgentID        string          `json:"agent_id"`
	AgentType      string          `json:"agent_type"`
	StopHookActive bool            `json:"stop_hook_active"`
}

type toolInput struct {
	FilePath     string `json:"file_path"`
	NotebookPath string `json:"notebook_path"`
	Command      string `json:"command"`
	Pattern      string `json:"pattern"`
	SubagentType string `json:"subagent_type"`
	Description  string `json:"description"`
	URL          string `json:"url"`
	Query        string `json:"query"`
}

func (in toolInput) path() string {
	if in.FilePath != "" {
		return in.FilePath
	}
	return in.NotebookPath
}

// Handle runs hook name and returns the process exit code (2 = deny for
// PreToolUse; 0 otherwise).
func (h *Handler) Handle(ctx context.Context, name string, in io.Reader, out, errw io.Writer) int {
	raw, err := io.ReadAll(io.LimitReader(in, 8<<20))
	if err != nil {
		return 0
	}
	var p payload
	if json.Unmarshal(raw, &p) != nil {
		return 0
	}
	switch name {
	case "session-start":
		h.sessionStart(ctx, out)
	case "read-guard":
		return h.readGuard(ctx, p, errw)
	case "model-guard":
		return h.modelGuard(ctx, p, errw)
	case "go-post-edit":
		h.goPostEdit(ctx, p, out)
	case "trace":
		h.trace(ctx, p)
	case "trace-agent":
		h.traceAgent(ctx, p)
	case "stop-gate":
		h.stopGate(ctx, p, out)
	}
	return 0
}

func (h *Handler) repo(p payload) string {
	if r := h.d.Env("CLAUDE_PROJECT_DIR"); r != "" {
		return r
	}
	if p.Cwd != "" {
		return p.Cwd
	}
	wd, _ := h.d.Getwd()
	return wd
}

func emit(out io.Writer, v any) {
	b, err := json.Marshal(v)
	if err == nil {
		fmt.Fprintln(out, string(b))
	}
}

func roleName(agentType string) string { return strings.TrimPrefix(agentType, "loops:") }

func (h *Handler) sessionStart(ctx context.Context, out io.Writer) {
	root := h.d.Env("LOOPS_PLUGIN_ROOT")
	text := "loops plugin active."
	if root != "" {
		text = fmt.Sprintf("loops plugin active. Plugin root: %s. Work tickets with /loops:work (WORKFLOW: %s/specs/WORKFLOW.md).", root, strings.TrimRight(root, "/"))
	}
	if id, err := h.d.Rec.Current(ctx); err == nil && id != "" {
		if st, err := h.d.Rec.State(ctx, id); err == nil {
			n := 0
			for _, l := range st.Lanes {
				if l.Running {
					n++
				}
			}
			dir, _ := h.d.Rec.Dir(ctx, id)
			text += fmt.Sprintf(" Active run %s: phase %s, %d lanes running, run dir %s.", id, st.Phase, n, dir)
		}
	} else if root == "" {
		return
	}
	if len(text) > 600 {
		text = text[:600]
	}
	emit(out, map[string]any{"hookSpecificOutput": map[string]string{"hookEventName": "SessionStart", "additionalContext": text}})
}

func (h *Handler) deny(ctx context.Context, p payload, guardName, reason string, errw io.Writer) int {
	fmt.Fprintln(errw, reason)
	_ = h.d.Rec.Record(ctx, run.Event{Kind: run.KindGuardDeny, Name: guardName, Agent: p.AgentID, Note: reason})
	return 2
}

func (h *Handler) readGuard(ctx context.Context, p payload, errw io.Writer) int {
	cfg := guard.DefaultConfig()
	cfg.Off = strings.EqualFold(h.d.Env("READ_GUARD"), "off")
	for _, kv := range []struct {
		env string
		dst *int
	}{{"READ_GUARD_MIN_LINES", &cfg.MinLines}, {"READ_GUARD_MAX_VIEW_LINES", &cfg.MaxViewLines}} {
		if v := h.d.Env(kv.env); v != "" {
			n, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				return 0
			}
			*kv.dst = n
		}
	}
	repo := h.repo(p)
	cwd := p.Cwd
	if cwd == "" {
		cwd = repo
	}
	reason := guard.New(h.d.Files, cfg).CheckRead(p.ToolName, p.ToolInput, cwd, repo)
	if reason == "" {
		return 0
	}
	return h.deny(ctx, p, "read-guard", reason, errw)
}

func (h *Handler) modelGuard(ctx context.Context, p payload, errw io.Writer) int {
	if reason := guard.CheckModel(p.ToolInput); reason != "" {
		return h.deny(ctx, p, "model-guard", reason, errw)
	}
	return 0
}

func (h *Handler) goPostEdit(ctx context.Context, p payload, out io.Writer) {
	var in toolInput
	_ = json.Unmarshal(p.ToolInput, &in)
	file := in.path()
	if !strings.HasSuffix(file, ".go") {
		return
	}
	repo := h.repo(p)
	if !filepath.IsAbs(file) {
		file = filepath.Join(repo, file)
	}
	if !h.d.Files.Exists(file) {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	tool, args := "gofmt", []string{"-w", file}
	if h.d.Cmd.Has("goimports") {
		tool = "goimports"
	}
	output, err := h.d.Cmd.Run(ctx, repo, tool, args...)
	if err == nil {
		if rel, rerr := filepath.Rel(repo, filepath.Dir(file)); rerr == nil && !strings.HasPrefix(rel, "..") {
			tool = "go vet"
			output, err = h.d.Cmd.Run(ctx, repo, "go", "vet", "./"+filepath.ToSlash(rel))
		}
	}
	_ = h.d.Rec.Record(ctx, run.Event{Kind: run.KindStep, Name: "go-post-edit", Agent: p.AgentID, Role: roleName(p.AgentType), Target: file, OK: run.Bool(err == nil)})
	if err != nil {
		lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
		if len(lines) > 30 {
			lines = lines[:30]
		}
		emit(out, map[string]string{"decision": "block", "reason": fmt.Sprintf("go-post-edit: %s failed for %s:\n%s", tool, file, strings.Join(lines, "\n"))})
	}
}

func firstLine(s string, max int) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > max {
		s = s[:max]
	}
	return s
}

func (h *Handler) target(p payload, in toolInput, repo string) string {
	switch p.ToolName {
	case "Read", "Edit", "Write", "MultiEdit", "NotebookEdit":
		f := in.path()
		if filepath.IsAbs(f) && repo != "" {
			if rel, err := filepath.Rel(repo, f); err == nil && !strings.HasPrefix(rel, "..") {
				return rel
			}
		}
		return f
	case "Bash":
		return firstLine(in.Command, 100)
	case "Grep", "Glob":
		return in.Pattern
	case "Agent", "Task":
		return in.SubagentType + ": " + in.Description
	case "WebFetch":
		return in.URL
	case "WebSearch":
		return in.Query
	}
	if strings.HasPrefix(p.ToolName, "mcp__") {
		return p.ToolName
	}
	return ""
}

func (h *Handler) trace(ctx context.Context, p payload) {
	if id, err := h.d.Rec.Current(ctx); err != nil || id == "" {
		return
	}
	var in toolInput
	_ = json.Unmarshal(p.ToolInput, &in)
	ok := p.Event != "PostToolUseFailure"
	if ok && p.ToolName == "Bash" {
		var resp struct {
			Interrupted bool `json:"interrupted"`
		}
		if json.Unmarshal(p.ToolResponse, &resp) == nil && resp.Interrupted {
			ok = false
		}
	}
	_ = h.d.Rec.Record(ctx, run.Event{
		Kind: run.KindStep, Agent: p.AgentID, Role: roleName(p.AgentType), Name: p.ToolName,
		Target: h.target(p, in, h.repo(p)), OK: run.Bool(ok), DurationMS: int64(p.DurationMS),
	})
}

func (h *Handler) traceAgent(ctx context.Context, p payload) {
	if p.AgentID == "" {
		return
	}
	e := run.Event{Agent: p.AgentID, Role: roleName(p.AgentType)}
	switch p.Event {
	case "SubagentStart":
		e.Kind = run.KindAgentStart
	case "SubagentStop":
		e.Kind, e.OK = run.KindAgentStop, run.Bool(true)
	default:
		return
	}
	_ = h.d.Rec.Record(ctx, e)
}

func (h *Handler) stopGate(ctx context.Context, p payload, out io.Writer) {
	if p.StopHookActive || strings.EqualFold(h.d.Env("LOOPS_GATE"), "off") {
		return
	}
	st, err := h.d.Rec.State(ctx, "")
	if err != nil {
		return
	}
	if reason := run.StopReason(st); reason != "" {
		emit(out, map[string]string{"decision": "block", "reason": reason})
	}
}
