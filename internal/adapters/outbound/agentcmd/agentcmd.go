// Package agentcmd runs a coding agent's CLI from a command template, in a
// directory of its own: `loops eval` runs scenarios with it and `loops
// backlog` runs lanes (specs/BACKLOG.md §3).
package agentcmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/arxdsilva/ai-loops/internal/domain/backlog"
)

// DefaultLane is the backlog lane's command: Claude Code in print mode
// with the auto permission mode (a classifier gates risky actions; issue
// text is untrusted input, BACKLOG §7).
const DefaultLane = "claude -p {prompt} --permission-mode auto"

// stopGrace is how long a stopped lane gets to exit before it is killed.
const stopGrace = 30 * time.Second

// StrippedEnv are variables of the calling session that would point the
// agent's hooks and `loops` calls at the caller's project or plugin instead
// of the agent's own directory.
var StrippedEnv = []string{
	"CLAUDE_PROJECT_DIR", "CURSOR_PROJECT_DIR", "GEMINI_PROJECT_DIR", "DEVIN_PROJECT_DIR",
	"CLAUDE_PLUGIN_ROOT", "LOOPS_PLUGIN_ROOT", "CLAUDECODE",
}

// Env returns environ without StrippedEnv.
func Env(environ []string) []string {
	out := make([]string, 0, len(environ))
	for _, kv := range environ {
		k, _, _ := strings.Cut(kv, "=")
		if !slices.Contains(StrippedEnv, k) {
			out = append(out, kv)
		}
	}
	return out
}

// Expand splits tmpl on spaces, then replaces each {name} placeholder with
// vars[name] inside each field, so {prompt} stays one argument however many
// spaces the prompt has. The template must hold a {prompt} field.
func Expand(tmpl string, vars map[string]string) ([]string, error) {
	fields := strings.Fields(tmpl)
	if !slices.Contains(fields, "{prompt}") {
		return nil, errors.New("agent template has no {prompt} argument")
	}
	pairs := make([]string, 0, 2*len(vars))
	for k, v := range vars {
		pairs = append(pairs, "{"+k+"}", v)
	}
	r := strings.NewReplacer(pairs...)
	argv := make([]string, len(fields))
	for i, f := range fields {
		argv[i] = r.Replace(f)
	}
	return argv, nil
}

// Lane runs backlog lanes; it implements the backlog Agent port.
type Lane struct{}

// LaneArgv is the lane command for settings s.
func LaneArgv(s backlog.Settings, dir, prompt string) ([]string, error) {
	tmpl := s.Agent
	if strings.TrimSpace(tmpl) == "" {
		tmpl = DefaultLane
		if s.Budget != "" {
			tmpl += " --max-budget-usd {budget}"
		}
	}
	return Expand(tmpl, map[string]string{"prompt": prompt, "budget": s.Budget, "dir": dir})
}

// Run runs the lane's agent in dir until it exits or ctx ends, appending
// its output to logPath. Ending ctx asks the agent and its children to stop
// (SIGTERM), then kills the agent after stopGrace; on Unix whatever it
// started is killed when it exits.
func (Lane) Run(ctx context.Context, dir, prompt, logPath string, s backlog.Settings) error {
	argv, err := LaneArgv(s, dir, prompt)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer log.Close()
	fmt.Fprintf(log, "\n== lane started %s in %s\n", time.Now().UTC().Format(time.RFC3339), dir)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = Env(os.Environ())
	cmd.Stdout, cmd.Stderr = log, log
	reap := graceful(cmd)
	cmd.WaitDelay = stopGrace
	err = cmd.Run()
	reap()
	fmt.Fprintf(log, "== lane ended %s: %v\n", time.Now().UTC().Format(time.RFC3339), errOrOK(err))
	return err
}

func errOrOK(err error) any {
	if err == nil {
		return "ok"
	}
	return err
}
