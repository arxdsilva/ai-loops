// Package hook translates Claude Code hook events (JSON on stdin) into
// guard decisions and run events. Every handler fails open: bad input,
// no active run, or an internal error exits 0 with no output.
//
// Names (hooks/hooks.json): session-start, read-guard, write-guard,
// test-guard, toolchain-env, model-guard, post-edit, trace, trace-agent,
// stop-gate. The pre-language-pack names race-guard, go-env and
// go-post-edit stay aliases.
package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/arxdsilva/ai-loops/internal/platform/loopsdir"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/application/guard"
	"github.com/arxdsilva/ai-loops/internal/application/updatecheck"
	"github.com/arxdsilva/ai-loops/internal/domain/run"
	"github.com/arxdsilva/ai-loops/internal/platform/projroot"
)

var projectEnv = map[string][]string{
	"cursor": {"CURSOR_PROJECT_DIR"}, "gemini": {"GEMINI_PROJECT_DIR"}, "windsurf": {"DEVIN_PROJECT_DIR"},
}

// maxBrief caps the run brief the session-start hook adds.
const maxBrief = 2400

var entryPoint = map[string]string{
	"claude": "/loops:work", "copilot": "/loops:work", "gemini": "/loops:work",
	"codex": "$work", "cursor": "/work", "opencode": "/loops/work", "windsurf": "the work skill",
	"antigravity": "the work skill (/work in Antigravity CLI)",
}

// maxIdleBlocks is how many stop-gate blocks with no tool call between them
// are tolerated before the gate releases the agent.
const maxIdleBlocks = 3

// handler dispatches hook names. It is unexported: New returns it for the
// composition root to wire to cli.Hooks (specs/languages/go/RULES.md §56).
type handler struct {
	d        Deps
	platform string
	langs    *detection // per Handle call; nil until first needed
	rec      Recorder   // per Handle call: the workspace recorder when Deps.RecFor is set
	// project is the project root for this call: projroot.Find of the payload
	// workspace. Hooks launched from the plugin root, or from a subdirectory
	// without a project-dir env var, must still find the project's manifest,
	// .loops/config.json and run store (Greptile P1).
	project string
}

// New returns the hook dispatcher; document behaviour here, not on handler.
func New(d Deps) *handler {
	if d.Env == nil {
		d.Env = func(string) string { return "" }
	}
	if d.Getwd == nil {
		d.Getwd = func() (string, error) { return "", nil }
	}
	return &handler{d: d}
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
	// SubagentStop: the stopped agent's own transcript (tokens.go).
	AgentTranscriptPath      string `json:"agent_transcript_path"`
	AgentTranscriptPathCamel string `json:"agentTranscriptPath"`

	// Other dialects' spellings (see dialect.go); folded by normalize.
	SessionIDCamel      string          `json:"sessionId"`
	ToolNameCamel       string          `json:"toolName"`
	ToolArgs            json.RawMessage `json:"toolArgs"`
	ToolResult          json.RawMessage `json:"toolResult"`
	StopHookActiveCamel bool            `json:"stopHookActive"`
	SubagentID          string          `json:"subagent_id"`
	SubagentType        string          `json:"subagent_type"`
	Duration            float64         `json:"duration"`
	LoopCount           int             `json:"loop_count"`
	IsInterrupt         bool            `json:"is_interrupt"`
	extraFiles          []string
	WorkspaceRoots      []string `json:"workspace_roots"`
	// Antigravity spellings (folded by normalize).
	ToolCall *struct {
		Name string          `json:"name"`
		Args json.RawMessage `json:"args"`
	} `json:"toolCall"`
	WorkspacePaths []string `json:"workspacePaths"`
	ConversationID string   `json:"conversationId"`
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

// Handle runs hook name for platform ("" autodetects) and returns the
// process exit code (2 = deny for PreToolUse; 0 otherwise). Hooks fail
// open: output is buffered, so a panic anywhere exits 0 with no output.
func (h *handler) Handle(ctx context.Context, name, platform string, in io.Reader, out, errw io.Writer) (code int) {
	var o, e bytes.Buffer
	defer func() {
		if recover() != nil {
			code = 0
			return
		}
		_, _ = out.Write(o.Bytes())
		_, _ = errw.Write(e.Bytes())
	}()
	return h.handle(ctx, name, platform, in, &o, &e)
}

// workspaceRecorder returns the recorder for the workspace the payload names,
// so hooks launched from the plugin root still find the active run
// (Greptile P1). It falls back to Deps.Rec when RecFor is nil, the workspace
// equals the process root, or nothing resolves.
func (h *handler) workspaceRecorder(_ payload, root string) Recorder {
	if h.d.RecFor == nil || h.d.Rec == nil || root == "" {
		return h.d.Rec
	}
	return h.d.RecFor(root)
}

// projectRoot is the project root for this Handle call, so pack detection,
// read/write base paths and file ownership all use one root.
func (h *handler) projectRoot(p payload) string {
	if h.project != "" {
		return h.project
	}
	return projroot.Find(h.repo(p))
}

// recorder is the recorder for this Handle call.
func (h *handler) recorder() Recorder {
	if h.rec != nil {
		return h.rec
	}
	return h.d.Rec
}

func (h *handler) handle(ctx context.Context, name, platform string, in io.Reader, out, errw io.Writer) int {
	raw, err := io.ReadAll(io.LimitReader(in, 8<<20))
	if err != nil {
		return 0
	}
	var p payload
	if json.Unmarshal(raw, &p) != nil {
		return 0
	}
	platform = detect(platform, h.d.Env)
	d, failed := normalize(&p, platform)
	h.platform = platform
	h.langs = nil
	h.project = projroot.Find(h.repo(p))
	h.rec = h.workspaceRecorder(p, h.project)
	h.absolutize(&p)
	switch name {
	case "session-start":
		h.sessionStart(ctx, d, p, out)
	case "read-guard":
		return h.readGuard(ctx, p, out, errw)
	case "write-guard":
		return h.writeGuard(ctx, p, out, errw)
	case "model-guard":
		return h.modelGuard(ctx, p, out, errw)
	case "test-guard", "race-guard":
		return h.testGuard(ctx, p, out, errw)
	case "toolchain-env", "go-env":
		return h.toolchainEnv(ctx, d, p, out, errw)
	case "post-edit", "go-post-edit":
		h.postEdit(ctx, d, p, out)
	case "trace":
		h.trace(ctx, p, failed)
	case "trace-agent":
		h.traceAgent(ctx, p)
	case "stop-gate":
		h.stopGate(ctx, d, p, out)
	}
	return 0
}

// absolutize makes a relative file_path absolute (payload cwd, then project
// dir) so guards see the real file whatever the platform sent.
func (h *handler) absolutize(p *payload) {
	switch p.ToolName {
	case "Read", "Edit", "Write", "MultiEdit":
	default:
		return
	}
	var in map[string]any
	if json.Unmarshal(p.ToolInput, &in) != nil {
		return
	}
	f, _ := in["file_path"].(string)
	if f == "" || filepath.IsAbs(f) {
		return
	}
	base := p.Cwd
	if base == "" {
		base = h.projectRoot(*p)
	}
	if base == "" {
		return
	}
	in["file_path"] = filepath.Join(base, f)
	if b, err := json.Marshal(in); err == nil {
		p.ToolInput = b
	}
}

func (h *handler) repo(p payload) string {
	for _, k := range projectEnv[h.platform] {
		if r := h.d.Env(k); r != "" {
			return r
		}
	}
	if r := h.d.Env("CLAUDE_PROJECT_DIR"); r != "" {
		return r
	}
	if p.Cwd != "" {
		return p.Cwd
	}
	wd, _ := h.d.Getwd()
	return wd
}

func roleName(agentType string) string { return strings.TrimPrefix(agentType, "loops:") }

func (h *handler) sessionStart(ctx context.Context, d dialect, p payload, out io.Writer) {
	root := h.d.Env("LOOPS_PLUGIN_ROOT")
	text := "loops plugin active."
	if root != "" {
		text = fmt.Sprintf("loops plugin active. Plugin root: %s. Work tickets with %s (WORKFLOW: %s/specs/WORKFLOW.md).", root, entryPoint[h.platform], strings.TrimRight(root, "/"))
	}
	brief := ""
	if id, err := h.recorder().Current(ctx); err == nil && id != "" {
		if st, err := h.recorder().State(ctx, id); err == nil {
			dir, _ := h.recorder().Dir(ctx, id)
			text += fmt.Sprintf(" Active run %s: phase %s, %d lanes running, run dir %s.", id, st.Phase, len(run.Running(st)), dir)
			// After a compaction or a resumed session the lead re-anchors
			// on the run instead of its memory (WORKFLOW §2.7).
			brief = run.Brief(st, dir)
			if len(brief) > maxBrief {
				brief = brief[:maxBrief]
			}
		}
	} else if root == "" {
		text = ""
	}
	if len(text) > 600 {
		text = text[:600]
	}
	if brief != "" {
		text += "\n" + brief + "Continue this run from that state: do not restart finished phases. `loops run brief` prints it again."
	}
	if root != "" {
		h.writePluginRoot(root, h.repo(p))
		text += fmt.Sprintf(" loops binary: %s/bin/loops — if `loops` is not on PATH, call it by that absolute path (or install it: `curl -fsSL https://raw.githubusercontent.com/arxdsilva/ai-loops/main/install.sh | sh`).", strings.TrimRight(root, "/"))
	}
	msg := ""
	if n := h.updateNotice(ctx); n != nil {
		cmd := updatecheck.Command(h.platform)
		if text != "" {
			text += " "
		}
		text += fmt.Sprintf("loops %s is available (you have %s). Tell the user once: update with: %s", vtag(n.Latest), vtag(n.Current), cmd)
		msg = fmt.Sprintf("loops %s available (you have %s) — update: %s", vtag(n.Latest), vtag(n.Current), cmd)
	}
	if text == "" {
		return
	}
	d.sessionContext(out, text, msg)
}

// writePluginRoot records the plugin root for shells that lack
// LOOPS_PLUGIN_ROOT. Best effort: it never fails the hook.
func (h *handler) writePluginRoot(root, repo string) {
	write := h.d.WriteFile
	if write == nil {
		write = writeFileAtomic
	}
	state := h.d.Env("XDG_STATE_HOME")
	if state == "" && h.d.Env("HOME") != "" {
		state = filepath.Join(h.d.Env("HOME"), ".local", "state")
	}
	if state != "" { // user-level fallback for fresh projects
		_ = write(filepath.Join(state, "loops", "plugin-root"), []byte(root))
	}
	if repo == "" {
		return
	}
	if fi, err := os.Stat(filepath.Join(repo, ".loops")); err != nil || !fi.IsDir() {
		return // only projects already using loops
	}
	_ = loopsdir.EnsureIgnore(repo) // projects from before .loops/.gitignore
	_ = write(filepath.Join(repo, ".loops", "plugin-root"), []byte(root))
}

func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".plugin-root-*")
	if err != nil {
		return err
	}
	_, err = tmp.Write(data)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
	}
	return err
}

func (h *handler) deny(ctx context.Context, p payload, guardName, reason string, out, errw io.Writer) int {
	_ = h.recorder().Record(ctx, run.Event{Kind: run.KindGuardDeny, Name: guardName, Agent: p.AgentID, Note: reason})
	if h.platform == "antigravity" {
		dialect{platform: h.platform}.emit(out, map[string]string{"decision": "deny", "reason": reason})
		return 0
	}
	fmt.Fprintln(errw, reason)
	return 2
}

func (h *handler) readGuard(ctx context.Context, p payload, out, errw io.Writer) int {
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
	repo := h.projectRoot(p)
	cwd := p.Cwd
	if cwd == "" {
		cwd = repo
	}
	reason := guard.New(h.d.Files, cfg).CheckRead(p.ToolName, p.ToolInput, cwd, repo)
	if reason == "" {
		return 0
	}
	return h.deny(ctx, p, "read-guard", reason, out, errw)
}

func (h *handler) writeGuard(ctx context.Context, p payload, out, errw io.Writer) int {
	if !isWriteTool(p.ToolName) {
		return 0
	}
	targets := h.writeTargets(p)
	if len(targets) == 0 {
		return 0
	}
	id, runDir, ok := h.activeRunDir(ctx)
	if !ok {
		return 0
	}
	role := h.writeRole(ctx, id, p)
	if role == "" {
		return 0
	}
	resolvedRunDir := resolvePath(runDir)
	resolvedProjectDir := resolvePath(h.projectRoot(p))
	for _, target := range targets {
		if reason := guard.CheckWrite(role, resolvePath(target), resolvedRunDir, resolvedProjectDir); reason != "" {
			return h.deny(ctx, p, "write-guard", reason, out, errw)
		}
	}
	return 0
}

func isWriteTool(name string) bool {
	return name == "Write" || name == "Edit" || name == "MultiEdit"
}

func (h *handler) activeRunDir(ctx context.Context) (id, dir string, ok bool) {
	if h.d.Rec == nil {
		return "", "", false
	}
	id, err := h.recorder().Current(ctx)
	if err != nil || id == "" {
		return "", "", false
	}
	dir, err = h.recorder().Dir(ctx, id)
	if err != nil || dir == "" {
		return "", "", false
	}
	return id, dir, true
}

func (h *handler) writeTargets(p payload) []string {
	var in toolInput
	_ = json.Unmarshal(p.ToolInput, &in)
	var targets []string
	// A write target is relative to the directory the tool ran in, not the
	// project root: resolving it against the root would mis-place a write made
	// from a subdirectory.
	base := p.Cwd
	if base == "" {
		base = h.projectRoot(p)
	}
	if in.FilePath != "" {
		fp := in.FilePath
		if !filepath.IsAbs(fp) && base != "" {
			fp = filepath.Join(base, fp)
		}
		targets = append(targets, fp)
	}
	for _, f := range p.extraFiles {
		if f == "" {
			continue
		}
		if !filepath.IsAbs(f) && base != "" {
			f = filepath.Join(base, f)
		}
		targets = append(targets, f)
	}
	return targets
}

func (h *handler) writeRole(ctx context.Context, runID string, p payload) string {
	if r := roleName(p.AgentType); r != "" {
		return r
	}
	st, _ := h.recorder().State(ctx, runID)
	if r := laneRole(st.Lanes, p.AgentID); r != "" {
		return r
	}
	if r := laneRole(st.Lanes, p.SessionID); r != "" {
		return r
	}
	if p.AgentID == "" {
		if r := soleRunningRole(st.Lanes); r != "" {
			return r
		}
	}
	return roleName(p.AgentID)
}

func laneRole(lanes []run.Lane, id string) string {
	if id == "" {
		return ""
	}
	for _, l := range lanes {
		if (l.Agent == id || l.Label == id) && l.Role != "" {
			if r := roleName(l.Role); r != "" {
				return r
			}
		}
	}
	return ""
}

func soleRunningRole(lanes []run.Lane) string {
	var role string
	for _, l := range lanes {
		if !l.Running || l.Role == "" {
			continue
		}
		r := roleName(l.Role)
		if r == "" {
			continue
		}
		if role != "" {
			return ""
		}
		role = r
	}
	return role
}

// resolvePath returns path with every symlink followed and ".." applied to the
// resolved tree, not lexically. Cleaning first would let <run>/link/../file
// pass a containment check while the filesystem resolves it outside the run
// (Greptile P1: writes escape the run directory).
func resolvePath(path string) string {
	if path == "" {
		return ""
	}
	// Split the raw path: filepath.Abs/Clean would collapse "link/.." lexically
	// before the symlink is followed, which is exactly the escape to catch.
	abs := path
	if !filepath.IsAbs(abs) {
		if wd, err := os.Getwd(); err == nil {
			abs = filepath.Join(wd, abs)
		}
	}
	vol := filepath.VolumeName(abs)
	rest := strings.TrimPrefix(abs[len(vol):], string(filepath.Separator))
	cur := vol + string(filepath.Separator)
	parts := strings.Split(rest, string(filepath.Separator))
	hops := 0
	for i := 0; i < len(parts); i++ {
		switch parts[i] {
		case "", ".":
			continue
		case "..":
			if parent := filepath.Dir(cur); parent != cur {
				cur = parent
			}
			continue
		}
		next := cur + parts[i]
		if cur != string(filepath.Separator) && !strings.HasSuffix(cur, string(filepath.Separator)) {
			next = cur + string(filepath.Separator) + parts[i]
		}
		if hops < 64 && isSymlink(next) {
			hops++
			if target, ok := readLink(next); ok {
				if filepath.IsAbs(target) {
					tvol := filepath.VolumeName(target)
					cur = tvol + string(filepath.Separator)
					parts = append(strings.Split(strings.TrimPrefix(target[len(tvol):], string(filepath.Separator)), string(filepath.Separator)), parts[i+1:]...)
				} else {
					parts = append(strings.Split(target, string(filepath.Separator)), parts[i+1:]...)
				}
				i = -1
				continue
			}
		}
		cur = next
	}
	return filepath.Clean(cur)
}

func isSymlink(path string) bool {
	fi, err := os.Lstat(path)
	return err == nil && fi.Mode()&os.ModeSymlink != 0
}

func readLink(path string) (string, bool) {
	target, err := os.Readlink(path)
	if err != nil {
		return "", false
	}
	return target, true
}

func (h *handler) modelGuard(ctx context.Context, p payload, out, errw io.Writer) int {
	if p.ToolName != "" && p.ToolName != "Agent" {
		return 0
	}
	switch h.platform {
	case "cursor", "opencode", "gemini", "antigravity":
		// No documented model field on these platforms' agent calls.
		return 0
	}
	if reason := guard.CheckModel(p.ToolInput); reason != "" {
		return h.deny(ctx, p, "model-guard", reason, out, errw)
	}
	return 0
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

func (h *handler) target(p payload, in toolInput, repo string) string {
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

func (h *handler) trace(ctx context.Context, p payload, failed bool) {
	if id, err := h.recorder().Current(ctx); err != nil || id == "" {
		return
	}
	var in toolInput
	_ = json.Unmarshal(p.ToolInput, &in)
	ok := !failed
	_ = h.recorder().Record(ctx, run.Event{
		Kind: run.KindStep, Agent: p.AgentID, Role: roleName(p.AgentType), Name: p.ToolName,
		Target: h.target(p, in, h.projectRoot(p)), OK: run.Bool(ok), DurationMS: int64(p.DurationMS),
	})
}

func (h *handler) traceAgent(ctx context.Context, p payload) {
	e := run.Event{Agent: p.AgentID, Role: roleName(p.AgentType)}
	switch p.Event {
	case "SubagentStart":
		if p.AgentID == "" {
			return
		}
		e.Kind = run.KindAgentStart
	case "SubagentStop":
		e.Kind, e.OK = run.KindAgentStop, run.Bool(true)
		e.Tokens = transcriptTokens(p.agentTranscript(), maxTranscriptBytes)
		if e.Agent == "" && !h.closestLane(ctx, &e) {
			return
		}
	default:
		return
	}
	_ = h.recorder().Record(ctx, e)
}

// closestLane fills e.Agent for a stop that carries no agent id: the most
// recently started, still-running lane with the same role.
func (h *handler) closestLane(ctx context.Context, e *run.Event) bool {
	if e.Role == "" {
		return false
	}
	st, err := h.recorder().State(ctx, "")
	if err != nil {
		return false
	}
	best := -1
	for i, l := range st.Lanes {
		if l.Running && strings.EqualFold(roleName(l.Role), e.Role) && (best < 0 || !l.Started.Before(st.Lanes[best].Started)) {
			best = i
		}
	}
	if best < 0 {
		return false
	}
	e.Agent = st.Lanes[best].Agent
	return true
}

// stalled reports whether the gate already blocked maxIdleBlocks times with
// no step event since the first of those blocks.
func (h *handler) stalled(ctx context.Context) bool {
	evs, err := h.recorder().Events(ctx, "")
	if err != nil {
		return false
	}
	n := 0
scan:
	for i := len(evs) - 1; i >= 0; i-- {
		switch e := evs[i]; {
		case e.Kind == run.KindStep:
			break scan
		case e.Kind == run.KindNote && e.Name == "stop-gate" && e.Note == "blocked":
			n++
		}
	}
	return n >= maxIdleBlocks
}

func (h *handler) stopGate(ctx context.Context, d dialect, p payload, out io.Writer) {
	if p.StopHookActive || strings.EqualFold(h.d.Env("LOOPS_GATE"), "off") {
		return
	}
	st, err := h.recorder().State(ctx, "")
	if err != nil {
		return
	}
	if reason := run.StopReason(st); reason != "" {
		if h.stalled(ctx) {
			_ = h.recorder().Record(ctx, run.Event{Kind: run.KindNote, Name: "stop-gate", Note: "released after 3 blocks without progress"})
			return
		}
		_ = h.recorder().Record(ctx, run.Event{Kind: run.KindNote, Name: "stop-gate", Note: "blocked"})
		d.blockStop(out, reason)
	}
}
