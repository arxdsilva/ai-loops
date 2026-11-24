package agentcmd

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/arxdsilva/ai-loops/internal/domain/backlog"
)

func TestExpand(t *testing.T) {
	argv, err := Expand("run {dir} -p {prompt} --x={budget}", map[string]string{"prompt": "fix it now", "dir": "/d", "budget": "3"})
	if err != nil || !slices.Equal(argv, []string{"run", "/d", "-p", "fix it now", "--x=3"}) {
		t.Fatalf("argv = %q, %v", argv, err)
	}
	if _, err := Expand("claude --print", nil); err == nil {
		t.Fatal("template without {prompt} accepted")
	}
}

func TestEnv(t *testing.T) {
	got := Env([]string{"CLAUDE_PROJECT_DIR=/caller", "PATH=/bin", "CLAUDECODE=1", "KEEP=x=y"})
	if !slices.Equal(got, []string{"PATH=/bin", "KEEP=x=y"}) {
		t.Fatalf("env = %q", got)
	}
}

func TestLaneArgv(t *testing.T) {
	argv, _ := LaneArgv(backlog.Settings{}, "/wt", "do #1")
	if !slices.Equal(argv, []string{"claude", "-p", "do #1", "--permission-mode", "auto"}) {
		t.Fatalf("default = %q", argv)
	}
	argv, _ = LaneArgv(backlog.Settings{Budget: "20"}, "/wt", "p")
	if strings.Join(argv[len(argv)-2:], " ") != "--max-budget-usd 20" {
		t.Fatalf("budget = %q", argv)
	}
	argv, _ = LaneArgv(backlog.Settings{Agent: "codex exec {prompt}", Budget: "20"}, "/wt", "p")
	if !slices.Equal(argv, []string{"codex", "exec", "p"}) {
		t.Fatalf("custom = %q", argv)
	}
}

func TestLaneRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	dir := t.TempDir()
	log := filepath.Join(t.TempDir(), "logs", "issue-1.log")
	t.Setenv("CLAUDE_PROJECT_DIR", "/caller")
	s := backlog.Settings{Agent: "sh -c {prompt}"}
	err := Lane{}.Run(context.Background(), dir, `pwd; echo "proj=[$CLAUDE_PROJECT_DIR]"`, log, s)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(log)
	real, _ := filepath.EvalSymlinks(dir)
	if !strings.Contains(string(b), real) || !strings.Contains(string(b), "proj=[]") || !strings.Contains(string(b), "lane ended") {
		t.Fatalf("log:\n%s", b)
	}
	// Ending ctx stops the lane.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := (Lane{}).Run(ctx, dir, "sleep 30", log, s); err == nil {
		t.Fatal("a stopped lane reported success")
	}
	if time.Since(start) > 10*time.Second {
		t.Fatal("lane not stopped promptly")
	}
	if err := (Lane{}).Run(context.Background(), dir, "x", log, backlog.Settings{Agent: "no-prompt"}); err == nil {
		t.Fatal("bad template accepted")
	}
}
