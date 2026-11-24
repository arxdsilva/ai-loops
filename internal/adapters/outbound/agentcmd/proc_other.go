//go:build !unix

package agentcmd

import "os/exec"

// ownGroup leaves the default: cancelling kills the agent process only.
func ownGroup(*exec.Cmd) {}
