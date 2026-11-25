//go:build unix

package agentcmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/arxdsilva/ai-loops/internal/domain/backlog"
)

// childPID waits for the agent to write its background child's pid.
func childPID(t *testing.T, path string) int {
	t.Helper()
	for range 200 {
		if b, err := os.ReadFile(path); err == nil && strings.HasSuffix(string(b), "\n") {
			pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
			if err != nil {
				t.Fatal(err)
			}
			return pid
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("the agent never started its child")
	return 0
}

// gone waits for pid to be reaped (a killed orphan is reaped by init).
func gone(pid int) bool {
	for range 200 {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return true
		}
		time.Sleep(25 * time.Millisecond)
	}
	return false
}

func TestLaneRunStopsDescendants(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(t.TempDir(), "lane.log")
	s := backlog.Settings{Agent: "sh -c {prompt}"}
	pidFile := filepath.Join(dir, "child.pid")

	// A stopped lane stops the agent's children, not only the agent.
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- (Lane{}).Run(ctx, dir, "sleep 300 & echo $! > child.pid; wait", log, s) }()
	pid := childPID(t, pidFile)
	cancel()
	if err := <-done; err == nil {
		t.Fatal("a stopped lane reported success")
	}
	if !gone(pid) {
		syscall.Kill(pid, syscall.SIGKILL)
		t.Fatalf("child %d outlived its stopped lane", pid)
	}

	// An agent that exits leaving a background child behind: the child goes too.
	os.Remove(pidFile)
	if err := (Lane{}).Run(context.Background(), dir, "sleep 300 & echo $! > child.pid", log, s); err != nil {
		t.Fatal(err)
	}
	pid = childPID(t, pidFile)
	if !gone(pid) {
		syscall.Kill(pid, syscall.SIGKILL)
		t.Fatalf("child %d outlived its lane", pid)
	}
}
