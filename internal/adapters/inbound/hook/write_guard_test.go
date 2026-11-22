package hook

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

type writeGuardRec struct {
	evs      []run.Event
	cur      string
	curErr   error
	dir      string
	dirErr   error
	state    run.State
	stateErr error
}

func (r *writeGuardRec) Record(_ context.Context, e run.Event) error {
	r.evs = append(r.evs, e)
	return nil
}
func (r *writeGuardRec) Current(context.Context) (string, error) { return r.cur, r.curErr }
func (r *writeGuardRec) State(context.Context, string) (run.State, error) {
	return r.state, r.stateErr
}
func (r *writeGuardRec) Events(context.Context, string) ([]run.Event, error) { return r.evs, nil }
func (r *writeGuardRec) Dir(context.Context, string) (string, error)         { return r.dir, r.dirErr }

func runWriteGuard(t *testing.T, rec *writeGuardRec, vars map[string]string, platform, in string) (int, string, string) {
	t.Helper()
	h := New(Deps{
		Rec:   rec,
		Files: fakeFiles{},
		Cmd:   &fakeCmd{},
		Env:   func(k string) string { return vars[k] },
	})
	var out, errw bytes.Buffer
	code := h.Handle(context.Background(), "write-guard", platform, strings.NewReader(in), &out, &errw)
	return code, out.String(), errw.String()
}

func TestWriteGuard_AntigravityToolsAndPlatforms(t *testing.T) {
	const (
		repo   = "/repo"
		runDir = "/repo/.loops/runs/r1"
	)
	cases := []struct {
		name     string
		platform string
		payload  string
		wantDeny bool
	}{
		{
			name:     "antigravity write_to_file outside runDir denied",
			platform: "antigravity",
			payload:  `{"hook_event_name":"PreToolUse","conversationId":"c1","workspacePaths":["/repo"],"toolCall":{"name":"write_to_file","args":{"TargetFile":"/repo/internal/x.go"}}}`,
			wantDeny: true,
		},
		{
			name:     "antigravity replace_file_content outside runDir denied",
			platform: "antigravity",
			payload:  `{"hook_event_name":"PreToolUse","conversationId":"c1","workspacePaths":["/repo"],"toolCall":{"name":"replace_file_content","args":{"TargetFile":"/repo/internal/x.go"}}}`,
			wantDeny: true,
		},
		{
			name:     "antigravity multi_replace_file_content outside runDir denied",
			platform: "antigravity",
			payload:  `{"hook_event_name":"PreToolUse","conversationId":"c1","workspacePaths":["/repo"],"toolCall":{"name":"multi_replace_file_content","args":{"TargetFile":"/repo/internal/x.go"}}}`,
			wantDeny: true,
		},
		{
			name:     "antigravity write_to_file inside runDir allowed",
			platform: "antigravity",
			payload:  `{"hook_event_name":"PreToolUse","conversationId":"c1","workspacePaths":["/repo"],"toolCall":{"name":"write_to_file","args":{"TargetFile":"/repo/.loops/runs/r1/plan.v1.md"}}}`,
		},
		{
			name:     "antigravity non-write view_file allowed",
			platform: "antigravity",
			payload:  `{"hook_event_name":"PreToolUse","conversationId":"c1","workspacePaths":["/repo"],"toolCall":{"name":"view_file","args":{"AbsolutePath":"/repo/internal/x.go"}}}`,
		},
		{
			name:     "claude Write outside runDir denied with exit 2",
			platform: "claude",
			payload:  `{"tool_name":"Write","tool_input":{"file_path":"/repo/internal/x.go"},"cwd":"/repo","agent_id":"c1"}`,
			wantDeny: true,
		},
		{
			name:     "claude Edit outside runDir denied with exit 2",
			platform: "claude",
			payload:  `{"tool_name":"Edit","tool_input":{"file_path":"/repo/internal/x.go"},"cwd":"/repo","agent_id":"c1"}`,
			wantDeny: true,
		},
		{
			name:     "claude MultiEdit outside runDir denied with exit 2",
			platform: "claude",
			payload:  `{"tool_name":"MultiEdit","tool_input":{"file_path":"/repo/internal/x.go"},"cwd":"/repo","agent_id":"c1"}`,
			wantDeny: true,
		},
		{
			name:     "claude Write inside runDir allowed",
			platform: "claude",
			payload:  `{"tool_name":"Write","tool_input":{"file_path":"/repo/.loops/runs/r1/research/code.md"},"cwd":"/repo","agent_id":"c1"}`,
		},
		{
			name:     "claude Read outside runDir ignored",
			platform: "claude",
			payload:  `{"tool_name":"Read","tool_input":{"file_path":"/repo/internal/x.go"},"cwd":"/repo","agent_id":"c1"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &writeGuardRec{
				cur: "r1",
				dir: runDir,
				state: run.State{
					Lanes: []run.Lane{{Agent: "c1", Role: "planner", Running: true}},
				},
			}
			code, out, errOut := runWriteGuard(t, rec, map[string]string{"CLAUDE_PROJECT_DIR": repo}, tc.platform, tc.payload)
			if !tc.wantDeny {
				if code != 0 || out != "" || errOut != "" || len(rec.evs) != 0 {
					t.Fatalf("want allow/silent, got code=%d out=%q err=%q evs=%+v", code, out, errOut, rec.evs)
				}
				return
			}
			if tc.platform == "antigravity" {
				assertAntigravityDeny(t, code, out, errOut, "write_guard:", rec.evs)
			} else {
				if code != 2 || out != "" || !strings.Contains(errOut, "write_guard:") {
					t.Fatalf("want exit 2 + stderr, got code=%d out=%q err=%q", code, out, errOut)
				}
				if len(rec.evs) != 1 || rec.evs[0].Kind != run.KindGuardDeny {
					t.Fatalf("want 1 KindGuardDeny event, got %+v", rec.evs)
				}
			}
			if rec.evs[0].Name != "write-guard" {
				t.Fatalf("want guard name write-guard, got %+v", rec.evs[0])
			}
		})
	}
}

func TestWriteGuard_RolesAndPaths(t *testing.T) {
	const (
		repo   = "/repo"
		runDir = "/repo/.loops/runs/r1"
	)
	cases := []struct {
		name     string
		platform string
		role     string
		payload  string
		wantDeny bool
	}{
		{
			name:     "reflector allowed inside .loops/memory",
			platform: "antigravity",
			role:     "reflector",
			payload:  `{"conversationId":"c1","workspacePaths":["/repo"],"toolCall":{"name":"write_to_file","args":{"TargetFile":"/repo/.loops/memory/decisions.md"}}}`,
		},
		{
			name:     "reflector allowed inside runDir",
			platform: "antigravity",
			role:     "reflector",
			payload:  `{"conversationId":"c1","workspacePaths":["/repo"],"toolCall":{"name":"write_to_file","args":{"TargetFile":"/repo/.loops/runs/r1/reflection.md"}}}`,
		},
		{
			name:     "reflector denied outside memory and runDir",
			platform: "antigravity",
			role:     "reflector",
			payload:  `{"conversationId":"c1","workspacePaths":["/repo"],"toolCall":{"name":"write_to_file","args":{"TargetFile":"/repo/README.md"}}}`,
			wantDeny: true,
		},
		{
			name:     "planner denied inside .loops/memory",
			platform: "antigravity",
			role:     "planner",
			payload:  `{"conversationId":"c1","workspacePaths":["/repo"],"toolCall":{"name":"write_to_file","args":{"TargetFile":"/repo/.loops/memory/decisions.md"}}}`,
			wantDeny: true,
		},
		{
			name:     "domain-engineer allowed outside runDir",
			platform: "antigravity",
			role:     "domain-engineer",
			payload:  `{"conversationId":"c1","workspacePaths":["/repo"],"toolCall":{"name":"write_to_file","args":{"TargetFile":"/repo/internal/domain/order.go"}}}`,
		},
		{
			name:     "docs-writer allowed outside runDir",
			platform: "antigravity",
			role:     "docs-writer",
			payload:  `{"conversationId":"c1","workspacePaths":["/repo"],"toolCall":{"name":"replace_file_content","args":{"TargetFile":"/repo/docs/GUIDE.md"}}}`,
		},
		{
			name:     "relative TargetFile outside runDir denied",
			platform: "antigravity",
			role:     "code-researcher",
			payload:  `{"conversationId":"c1","workspacePaths":["/repo"],"toolCall":{"name":"write_to_file","args":{"TargetFile":"internal/x.go"}}}`,
			wantDeny: true,
		},
		{
			name:     "relative TargetFile inside runDir allowed",
			platform: "antigravity",
			role:     "code-researcher",
			payload:  `{"conversationId":"c1","workspacePaths":["/repo"],"toolCall":{"name":"write_to_file","args":{"TargetFile":".loops/runs/r1/research/code.md"}}}`,
		},
		{
			name:     "codex apply_patch second file outside runDir denied",
			platform: "codex",
			role:     "planner",
			payload:  `{"hook_event_name":"PreToolUse","cwd":"/repo","agent_id":"c1","tool_name":"apply_patch","tool_input":{"command":"*** Add File: .loops/runs/r1/plan.v1.md\n+ok\n*** Update File: internal/x.go\n+bad\n"}}`,
			wantDeny: true,
		},
		{
			name:     "codex apply_patch without cwd uses repo env for relative files",
			platform: "codex",
			role:     "planner",
			payload:  `{"hook_event_name":"PreToolUse","agent_id":"c1","tool_name":"apply_patch","tool_input":{"command":"*** Add File: .loops/runs/r1/plan.v1.md\n+ok\n*** Update File: internal/x.go\n+bad\n"}}`,
			wantDeny: true,
		},
		{
			name:     "codex apply_patch all files inside runDir allowed",
			platform: "codex",
			role:     "planner",
			payload:  `{"hook_event_name":"PreToolUse","cwd":"/repo","agent_id":"c1","tool_name":"apply_patch","tool_input":{"command":"*** Add File: .loops/runs/r1/plan.v1.md\n+ok\n*** Update File: .loops/runs/r1/notes.md\n+ok\n"}}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &writeGuardRec{
				cur: "r1",
				dir: runDir,
				state: run.State{
					Lanes: []run.Lane{{Agent: "c1", Role: tc.role, Running: true}},
				},
			}
			code, out, errOut := runWriteGuard(t, rec, map[string]string{"CLAUDE_PROJECT_DIR": repo}, tc.platform, tc.payload)
			if !tc.wantDeny {
				if code != 0 || out != "" || errOut != "" || len(rec.evs) != 0 {
					t.Fatalf("want allow, got code=%d out=%q err=%q evs=%+v", code, out, errOut, rec.evs)
				}
				return
			}
			if tc.platform == "antigravity" {
				assertAntigravityDeny(t, code, out, errOut, tc.role, rec.evs)
			} else if code != 2 || !strings.Contains(errOut, tc.role) {
				t.Fatalf("want exit 2 for %s, got code=%d out=%q err=%q", tc.role, code, out, errOut)
			}
		})
	}
}

func TestWriteGuard_RoleResolution(t *testing.T) {
	const (
		repo   = "/repo"
		runDir = "/repo/.loops/runs/r1"
	)
	cases := []struct {
		name     string
		platform string
		lanes    []run.Lane
		stateErr error
		payload  string
		wantRole string // empty means fail-open (allowed)
	}{
		{
			name:     "from subagent_type prefixed",
			platform: "claude",
			payload:  `{"tool_name":"Write","tool_input":{"file_path":"/repo/x.go"},"cwd":"/repo","subagent_type":"loops:planner"}`,
			wantRole: "planner",
		},
		{
			name:     "from agent_type unprefixed",
			platform: "claude",
			payload:  `{"tool_name":"Write","tool_input":{"file_path":"/repo/x.go"},"cwd":"/repo","agent_type":"qa-verifier"}`,
			wantRole: "qa-verifier",
		},
		{
			name:     "from State.Lanes matching Agent via agent_id",
			platform: "claude",
			lanes:    []run.Lane{{Agent: "ag-42", Role: "loops:code-researcher"}},
			payload:  `{"tool_name":"Write","tool_input":{"file_path":"/repo/x.go"},"cwd":"/repo","agent_id":"ag-42"}`,
			wantRole: "code-researcher",
		},
		{
			name:     "from State.Lanes matching Label via agent_id",
			platform: "claude",
			lanes:    []run.Lane{{Agent: "ag-99", Label: "W0-plan", Role: "plan-reviewer"}},
			payload:  `{"tool_name":"Write","tool_input":{"file_path":"/repo/x.go"},"cwd":"/repo","agent_id":"W0-plan"}`,
			wantRole: "plan-reviewer",
		},
		{
			name:     "from State.Lanes matching Agent via conversationId",
			platform: "antigravity",
			lanes:    []run.Lane{{Agent: "conv-7", Role: "web-researcher"}},
			payload:  `{"conversationId":"conv-7","workspacePaths":["/repo"],"toolCall":{"name":"write_to_file","args":{"TargetFile":"/repo/x.go"}}}`,
			wantRole: "web-researcher",
		},
		{
			name:     "from State.Lanes matching Label via session_id",
			platform: "claude",
			lanes:    []run.Lane{{Agent: "ag-1", Label: "sess-1", Role: "security-reviewer"}},
			payload:  `{"session_id":"sess-1","tool_name":"Write","tool_input":{"file_path":"/repo/x.go"},"cwd":"/repo"}`,
			wantRole: "security-reviewer",
		},
		{
			name:     "from single running lane when agent_id empty",
			platform: "antigravity",
			lanes: []run.Lane{
				{Agent: "done-1", Role: "domain-engineer", Running: false},
				{Agent: "run-1", Role: "loops:practices-researcher", Running: true},
			},
			payload:  `{"conversationId":"unrelated","workspacePaths":["/repo"],"toolCall":{"name":"write_to_file","args":{"TargetFile":"/repo/x.go"}}}`,
			wantRole: "practices-researcher",
		},
		{
			name:     "from agent_id fallback when not in State.Lanes",
			platform: "claude",
			lanes:    []run.Lane{{Agent: "other", Role: "domain-engineer", Running: true}},
			payload:  `{"tool_name":"Write","tool_input":{"file_path":"/repo/x.go"},"cwd":"/repo","agent_id":"loops:product-reviewer"}`,
			wantRole: "product-reviewer",
		},
		{
			name:     "from agent_id fallback when State returns error",
			platform: "claude",
			stateErr: errors.New("corrupt state"),
			payload:  `{"tool_name":"Write","tool_input":{"file_path":"/repo/x.go"},"cwd":"/repo","agent_id":"planner"}`,
			wantRole: "planner",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &writeGuardRec{
				cur:      "r1",
				dir:      runDir,
				state:    run.State{Lanes: tc.lanes},
				stateErr: tc.stateErr,
			}
			code, out, errOut := runWriteGuard(t, rec, map[string]string{"CLAUDE_PROJECT_DIR": repo}, tc.platform, tc.payload)
			if tc.platform == "antigravity" {
				assertAntigravityDeny(t, code, out, errOut, tc.wantRole, rec.evs)
			} else if code != 2 || !strings.Contains(errOut, tc.wantRole) {
				t.Fatalf("want deny mentioning %q, got code=%d out=%q err=%q", tc.wantRole, code, out, errOut)
			}
		})
	}
}

func TestWriteGuard_SymlinkSafety(t *testing.T) {
	base := t.TempDir()
	projectDir := filepath.Join(base, "repo")
	outsideDir := filepath.Join(base, "outside")
	runDir := filepath.Join(projectDir, ".loops", "runs", "r1")
	memDir := filepath.Join(projectDir, ".loops", "memory")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outsideDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Symlink inside runDir pointing outside runDir.
	escapeLink := filepath.Join(runDir, "escape")
	if err := os.Symlink(outsideDir, escapeLink); err != nil {
		t.Fatal(err)
	}
	// Dangling symlink inside runDir pointing outside runDir.
	danglingLink := filepath.Join(runDir, "dangling")
	if err := os.Symlink(filepath.Join(outsideDir, "future-file.txt"), danglingLink); err != nil {
		t.Fatal(err)
	}
	// Relative dangling symlink inside runDir escaping out of runDir.
	relDanglingLink := filepath.Join(runDir, "rel-dangling")
	if err := os.Symlink(filepath.Join("..", "..", "..", "..", "outside", "rel-future.txt"), relDanglingLink); err != nil {
		t.Fatal(err)
	}
	// Symlink inside memory pointing outside memory (for reflector).
	memEscape := filepath.Join(memDir, "escape")
	if err := os.Symlink(outsideDir, memEscape); err != nil {
		t.Fatal(err)
	}
	// Symlink alias to runDir itself (legitimate writes through alias must still be allowed).
	runAlias := filepath.Join(base, "run-alias")
	if err := os.Symlink(runDir, runAlias); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name     string
		role     string
		runDir   string
		target   string
		wantDeny bool
	}{
		{
			name:     "symlink inside runDir pointing outside denied (non-existent leaf)",
			role:     "planner",
			runDir:   runDir,
			target:   filepath.Join(escapeLink, "sub", "secret.txt"),
			wantDeny: true,
		},
		{
			name:     "dangling symlink inside runDir pointing outside denied",
			role:     "planner",
			runDir:   runDir,
			target:   danglingLink,
			wantDeny: true,
		},
		{
			name:     "relative dangling symlink inside runDir pointing outside denied",
			role:     "planner",
			runDir:   runDir,
			target:   relDanglingLink,
			wantDeny: true,
		},
		{
			name:     "reflector symlink inside memory pointing outside denied",
			role:     "reflector",
			runDir:   runDir,
			target:   filepath.Join(memEscape, "leak.md"),
			wantDeny: true,
		},
		{
			name:   "legitimate write to new nested file inside runDir allowed",
			role:   "planner",
			runDir: runDir,
			target: filepath.Join(runDir, "research", "new.md"),
		},
		{
			name:   "legitimate write when runDir is a symlink alias allowed",
			role:   "planner",
			runDir: runAlias,
			target: filepath.Join(runDir, "plan.v1.md"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &writeGuardRec{
				cur:   "r1",
				dir:   tc.runDir,
				state: run.State{Lanes: []run.Lane{{Agent: "c1", Role: tc.role, Running: true}}},
			}
			in := mustJSON(map[string]any{
				"hook_event_name": "PreToolUse",
				"conversationId":  "c1",
				"workspacePaths":  []string{projectDir},
				"toolCall": map[string]any{
					"name": "write_to_file",
					"args": map[string]any{"TargetFile": tc.target},
				},
			})
			code, out, errOut := runWriteGuard(t, rec, map[string]string{"CLAUDE_PROJECT_DIR": projectDir}, "antigravity", in)
			if tc.wantDeny {
				assertAntigravityDeny(t, code, out, errOut, tc.role, rec.evs)
				return
			}
			if code != 0 || out != "" || errOut != "" || len(rec.evs) != 0 {
				t.Fatalf("want allow, got code=%d out=%q err=%q evs=%+v", code, out, errOut, rec.evs)
			}
		})
	}
}

func TestWriteGuard_FailOpen(t *testing.T) {
	cases := []struct {
		name     string
		platform string
		rec      *writeGuardRec
		payload  string
	}{
		{
			name:     "malformed JSON",
			platform: "antigravity",
			rec:      &writeGuardRec{cur: "r1", dir: "/repo/.loops/runs/r1"},
			payload:  "{bad json",
		},
		{
			name:     "no active run",
			platform: "antigravity",
			rec:      &writeGuardRec{cur: "", dir: ""},
			payload:  `{"conversationId":"c1","workspacePaths":["/repo"],"toolCall":{"name":"write_to_file","args":{"TargetFile":"/repo/x.go"}}}`,
		},
		{
			name:     "Current error",
			platform: "antigravity",
			rec:      &writeGuardRec{curErr: errors.New("io error")},
			payload:  `{"conversationId":"c1","workspacePaths":["/repo"],"toolCall":{"name":"write_to_file","args":{"TargetFile":"/repo/x.go"}}}`,
		},
		{
			name:     "Dir error",
			platform: "antigravity",
			rec:      &writeGuardRec{cur: "r1", dirErr: errors.New("missing dir")},
			payload:  `{"conversationId":"c1","workspacePaths":["/repo"],"toolCall":{"name":"write_to_file","args":{"TargetFile":"/repo/x.go"}}}`,
		},
		{
			name:     "empty runDir",
			platform: "antigravity",
			rec:      &writeGuardRec{cur: "r1", dir: ""},
			payload:  `{"conversationId":"c1","workspacePaths":["/repo"],"toolCall":{"name":"write_to_file","args":{"TargetFile":"/repo/x.go"}}}`,
		},
		{
			name:     "missing file_path",
			platform: "antigravity",
			rec: &writeGuardRec{
				cur:   "r1",
				dir:   "/repo/.loops/runs/r1",
				state: run.State{Lanes: []run.Lane{{Agent: "c1", Role: "planner", Running: true}}},
			},
			payload: `{"conversationId":"c1","workspacePaths":["/repo"],"toolCall":{"name":"write_to_file","args":{}}}`,
		},
		{
			name:     "unknown role from subagent_type",
			platform: "claude",
			rec:      &writeGuardRec{cur: "r1", dir: "/repo/.loops/runs/r1"},
			payload:  `{"tool_name":"Write","tool_input":{"file_path":"/repo/x.go"},"cwd":"/repo","subagent_type":"custom-role"}`,
		},
		{
			name:     "unknown role when no lane or agent matches",
			platform: "antigravity",
			rec: &writeGuardRec{
				cur:   "r1",
				dir:   "/repo/.loops/runs/r1",
				state: run.State{},
			},
			payload: `{"conversationId":"c1","workspacePaths":["/repo"],"toolCall":{"name":"write_to_file","args":{"TargetFile":"/repo/x.go"}}}`,
		},
		{
			name:     "multiple running lanes without matching id",
			platform: "antigravity",
			rec: &writeGuardRec{
				cur: "r1",
				dir: "/repo/.loops/runs/r1",
				state: run.State{Lanes: []run.Lane{
					{Agent: "a1", Role: "planner", Running: true},
					{Agent: "a2", Role: "code-researcher", Running: true},
				}},
			},
			payload: `{"conversationId":"unmatched","workspacePaths":["/repo"],"toolCall":{"name":"write_to_file","args":{"TargetFile":"/repo/x.go"}}}`,
		},
		{
			name:     "State error with no agent_id or subagent_type",
			platform: "antigravity",
			rec:      &writeGuardRec{cur: "r1", dir: "/repo/.loops/runs/r1", stateErr: errors.New("corrupt")},
			payload:  `{"conversationId":"c1","workspacePaths":["/repo"],"toolCall":{"name":"write_to_file","args":{"TargetFile":"/repo/x.go"}}}`,
		},
		{
			name:     "nil Recorder",
			platform: "antigravity",
			rec:      nil,
			payload:  `{"conversationId":"c1","workspacePaths":["/repo"],"toolCall":{"name":"write_to_file","args":{"TargetFile":"/repo/x.go"}}}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var rec Recorder
			if tc.rec != nil {
				rec = tc.rec
			}
			h := New(Deps{
				Rec:   rec,
				Files: fakeFiles{},
				Cmd:   &fakeCmd{},
				Env:   func(k string) string { return map[string]string{"CLAUDE_PROJECT_DIR": "/repo"}[k] },
			})
			var out, errw bytes.Buffer
			code := h.Handle(context.Background(), "write-guard", tc.platform, strings.NewReader(tc.payload), &out, &errw)
			if code != 0 || out.String() != "" || errw.String() != "" || (tc.rec != nil && len(tc.rec.evs) != 0) {
				t.Fatalf("want fail-open (0, empty, no events), got code=%d out=%q err=%q", code, out.String(), errw.String())
			}
		})
	}
	// Empty projectDir (no CLAUDE_PROJECT_DIR, no cwd) still checks runDir without error.
	rec := &writeGuardRec{
		cur:   "r1",
		dir:   "/repo/.loops/runs/r1",
		state: run.State{Lanes: []run.Lane{{Agent: "c1", Role: "reflector", Running: true}}},
	}
	code, out, errOut := runWriteGuard(t, rec, nil, "antigravity", `{"conversationId":"c1","toolCall":{"name":"write_to_file","args":{"TargetFile":"/repo/.loops/memory/p.md"}}}`)
	assertAntigravityDeny(t, code, out, errOut, "reflector", rec.evs)
}
