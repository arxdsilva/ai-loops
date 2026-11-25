//go:build unix

package agentcmd

import (
	"os/exec"
	"syscall"
)

// graceful makes cancelling send SIGTERM, so the agent can stop its own
// children; Run's WaitDelay kills it if it is still there after stopGrace.
// The agent stays in the caller's process group, so whatever stops the
// caller's group (a harness stopping a background task) stops it too.
func graceful(cmd *exec.Cmd) {
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
}
