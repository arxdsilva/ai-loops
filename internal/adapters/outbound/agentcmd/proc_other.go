//go:build !unix

package agentcmd

import "os/exec"

// graceful leaves the default: cancelling kills the agent process.
func graceful(*exec.Cmd) {}
