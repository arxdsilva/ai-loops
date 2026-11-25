//go:build unix

package agentcmd

import (
	"context"
	"errors"
	"os"
	"os/exec"
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

// gone waits for pid to end. A killed orphan is reaped by init, but in a
// container whose init reaps nothing it stays a zombie: ended all the same.
func gone(pid int) bool {
	for range 200 {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) || zombie(pid) {
			return true
		}
		time.Sleep(25 * time.Millisecond)
	}
	return false
}

// zombie says whether pid has ended but was not reaped: state Z in
// /proc/<pid>/stat where there is one (Linux, any ps), else in ps.
func zombie(pid int) bool {
	if b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat"); err == nil {
		// The state follows the command name, which may hold spaces or ")".
		if i := strings.LastIndexByte(string(b), ')'); i >= 0 {
			return strings.HasPrefix(strings.TrimSpace(string(b)[i+1:]), "Z")
		}
	}
	out, err := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	return err == nil && strings.HasPrefix(strings.TrimSpace(string(out)), "Z")
}

// fileLease is a lane lock held on f, as backlogstore's lease is.
type fileLease struct{ f *os.File }

func (l fileLease) Release()       { syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN); l.f.Close() }
func (l fileLease) File() *os.File { return l.f }

// locked says whether another open of p cannot take its lock.
func locked(t *testing.T, p string) bool {
	t.Helper()
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		return true
	}
	syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return false
}

// TestLeaseHolder is the `loops backlog work` side of
// TestLaneRunAgentHoldsTheLease, run in a child process: it takes the lane
// lock and runs a lane whose agent waits for a go file.
func TestLeaseHolder(t *testing.T) {
	p, dir := os.Getenv("LOOPS_TEST_LEASE"), os.Getenv("LOOPS_TEST_LANE_DIR")
	if p == "" {
		t.Skip("helper process for TestLaneRunAgentHoldsTheLease")
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	s := backlog.Settings{Agent: "sh -c {prompt}"}
	(Lane{}).Run(context.Background(), dir, "echo $$ > ready.pid; while [ ! -f go ]; do sleep 0.05; done", filepath.Join(dir, "lane.log"), s, fileLease{f})
}

// The agent inherits the lane lock: when `loops backlog work` is killed
// outright, the lane still looks alive until the agent ends, so a sync
// never recovers a lane whose agent is still writing its run.
func TestLaneRunAgentHoldsTheLease(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(t.TempDir(), "issue-1.lock")
	gate := filepath.Join(dir, "go")
	holder := exec.Command(os.Args[0], "-test.run=^TestLeaseHolder$")
	holder.Env = append(os.Environ(), "LOOPS_TEST_LEASE="+p, "LOOPS_TEST_LANE_DIR="+dir)
	if err := holder.Start(); err != nil {
		t.Fatal(err)
	}
	agent := childPID(t, filepath.Join(dir, "ready.pid"))
	t.Cleanup(func() { os.WriteFile(gate, nil, 0o644); syscall.Kill(agent, syscall.SIGKILL) })
	holder.Process.Kill()
	holder.Wait()
	if !locked(t, p) {
		t.Fatal("the lane lock died with a killed loops backlog work while its agent runs")
	}
	os.WriteFile(gate, nil, 0o644)
	if !gone(agent) {
		t.Fatalf("agent %d did not end", agent)
	}
	for range 200 {
		if !locked(t, p) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("the lane lock outlived the agent")
}
