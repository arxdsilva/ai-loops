package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/arxdsilva/ai-loops/internal/application/updatecheck"
	"github.com/arxdsilva/ai-loops/internal/domain/update"
)

// Updater is the release check (updatecheck.Service).
type Updater interface {
	Fetch(ctx context.Context) (update.State, error)
}

// UpdateStore reads the cached update state (no network).
type UpdateStore interface {
	Load() (update.State, error)
}

func (a *App) cmdVersion(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	check := fs.Bool("check", false, "")
	platform := fs.String("platform", "", "")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(a.Err, "usage: loops version [--check [--platform p]]")
		return 2
	}
	fmt.Fprintln(a.Out, "loops", Version)
	if !*check {
		return 0
	}
	if a.Update == nil {
		fmt.Fprintln(a.Out, "latest: unknown (update check unavailable)")
		return 0
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	st, err := a.Update.Fetch(ctx)
	if err != nil {
		fmt.Fprintf(a.Out, "latest: unknown (%v)\n", err)
		return 0
	}
	fmt.Fprintln(a.Out, "latest:", st.Latest)
	if update.Newer(st.Latest, Version) {
		p := *platform
		if p == "" {
			p = detectPlatform(os.Getenv)
		}
		fmt.Fprintln(a.Out, "update available:", updatecheck.Command(p))
	} else {
		fmt.Fprintln(a.Out, "up to date")
	}
	return 0
}

// detectPlatform guesses the agent platform from its environment ("" = unknown).
func detectPlatform(env func(string) string) string {
	switch {
	case env("CURSOR_PLUGIN_ROOT") != "" || env("CURSOR_PROJECT_DIR") != "":
		return "cursor"
	case env("GEMINI_PROJECT_DIR") != "" || env("GEMINI_CWD") != "":
		return "gemini"
	case env("DEVIN_PROJECT_DIR") != "":
		return "windsurf"
	case env("CLAUDE_PROJECT_DIR") != "" || env("CLAUDE_PLUGIN_ROOT") != "":
		return "claude"
	case env("PLUGIN_ROOT") != "":
		return "codex"
	}
	return ""
}

// updateMarker is " · ⬆ <latest>" when the cached state (never the
// network) says a newer release exists, else "".
func updateMarker(a *App) string {
	if a.UpdateState == nil || a.UpdateOff || os.Getenv("LOOPS_NO_UPDATE_CHECK") != "" {
		return ""
	}
	st, err := a.UpdateState.Load()
	if err != nil || !update.Newer(st.Latest, Version) {
		return ""
	}
	return " · ⬆ " + strings.TrimSpace(st.Latest)
}
