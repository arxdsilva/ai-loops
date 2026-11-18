package evalbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"
)

// DefaultAgent is the agent command template: Claude Code in print mode,
// loading the plugin from this checkout (not the installed copy), with
// permissions bypassed (the sandbox is a throwaway repo) and a spend cap.
// Placeholders: {prompt}, {plugin}, {budget}, {dir}.
const DefaultAgent = "claude -p {prompt} --plugin-dir {plugin} --permission-mode bypassPermissions --max-budget-usd {budget}"

// strippedEnv are variables of the calling session that would point the
// agent's hooks at the caller's project or plugin instead of the sandbox.
var strippedEnv = []string{
	"CLAUDE_PROJECT_DIR", "CURSOR_PROJECT_DIR", "GEMINI_PROJECT_DIR", "DEVIN_PROJECT_DIR",
	"CLAUDE_PLUGIN_ROOT", "LOOPS_PLUGIN_ROOT", "CLAUDECODE",
}

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
	r := strings.NewReplacer("{prompt}", prompt, "{plugin}", a.Plugin, "{budget}", a.Budget, "{dir}", dir)
	fields := strings.Fields(tmpl)
	argv := make([]string, len(fields))
	for i, f := range fields {
		argv[i] = r.Replace(f)
	}
	if !slices.Contains(fields, "{prompt}") {
		return nil, errors.New("agent template has no {prompt} argument")
	}
	return argv, nil
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
	cmd.Env = env(os.Environ())
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

// env drops strippedEnv from environ.
func env(environ []string) []string {
	out := make([]string, 0, len(environ))
	for _, kv := range environ {
		k, _, _ := strings.Cut(kv, "=")
		if !slices.Contains(strippedEnv, k) {
			out = append(out, kv)
		}
	}
	return out
}
