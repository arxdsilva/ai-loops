package evalbox

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/agentcmd"
)

// DefaultAgent is the agent command template: Claude Code in print mode,
// loading the plugin from this checkout (not the installed copy), with
// permissions bypassed (the sandbox is a throwaway repo) and a spend cap.
// Placeholders: {prompt}, {plugin}, {budget}, {dir}.
const DefaultAgent = "claude -p {prompt} --plugin-dir {plugin} --permission-mode bypassPermissions --max-budget-usd {budget}"

// Agent runs the agent command in the sandbox.
type Agent struct {
	// Template is the command; fields are split on spaces, then each
	// placeholder is replaced (so {prompt} stays one argument).
	Template string
	Plugin   string // the plugin root to load
	Budget   string // spend cap, in USD
	// Out receives the agent's stdout and stderr (nil = discard).
	Out io.Writer
}

// Argv expands the template for dir and prompt.
func (a Agent) Argv(dir, prompt string) ([]string, error) {
	tmpl := a.Template
	if strings.TrimSpace(tmpl) == "" {
		tmpl = DefaultAgent
	}
	return agentcmd.Expand(tmpl, map[string]string{"prompt": prompt, "plugin": a.Plugin, "budget": a.Budget, "dir": dir})
}

// Run runs the agent in dir until it exits or timeout passes.
func (a Agent) Run(ctx context.Context, dir, prompt string, timeout time.Duration) error {
	argv, err := a.Argv(dir, prompt)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = agentcmd.Env(os.Environ())
	out := a.Out
	if out == nil {
		out = io.Discard
	}
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("agent stopped after %s: %w", timeout, ctx.Err())
		}
		return fmt.Errorf("agent: %w", err)
	}
	return nil
}
