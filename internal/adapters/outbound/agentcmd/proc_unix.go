//go:build unix

package agentcmd

import (
	"os/exec"
	"syscall"
)

// ownGroup puts the agent in its own process group and makes cancelling
// signal the whole group, so the agent's own children stop with it.
func ownGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }
}
