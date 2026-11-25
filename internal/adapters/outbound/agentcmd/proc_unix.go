//go:build unix

package agentcmd

import (
	"errors"
	"os/exec"
	"syscall"
)

// graceful runs the agent in its own process group. Cancelling sends SIGTERM
// to the whole group, so the agent and everything it started can stop;
// Run's WaitDelay kills the agent if it is still there after stopGrace. The
// returned reap kills whatever of the group is left once the agent has
// exited (WaitDelay's kill reaches only the agent itself), so no process
// the agent started outlives its lane.
func graceful(cmd *exec.Cmd) (reap func()) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
			return err
		}
		return nil
	}
	return func() {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}
}
