package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/insurgencygames/ai-loops/internal/application/ticket"
	"github.com/insurgencygames/ai-loops/internal/domain/run"
)

// cmdTicket implements `loops ticket fetch [REF] [--source s] [--body-file f|-]`.
// The source is --source or ticket.Detect(ref, "") (no git remote is consulted,
// so pass --source gitlab for bare GitLab refs). The ticket is printed on
// stdout and, when a run is active, also written to <run dir>/ticket.md.
func (a *App) cmdTicket(ctx context.Context, args []string) int {
	if len(args) == 0 || args[0] != "fetch" {
		return a.usageErr("usage: loops ticket fetch [REF] [--source s] [--body-file f|-]")
	}
	fs := a.newFlagSet("ticket fetch")
	source := fs.String("source", "", "github|gitlab|jira|linear|text (default: detected from REF)")
	bodyFile := fs.String("body-file", "", "file with the ticket text, or - for stdin")
	pos, err := parseFlagsAnywhere(fs, args[1:])
	if err != nil {
		return 2
	}
	if len(pos) > 1 {
		return a.usageErr("usage: loops ticket fetch [REF] [--source s] [--body-file f|-]")
	}
	ref := ""
	if len(pos) == 1 {
		ref = pos[0]
	}
	src := *source
	if src == "" {
		src = ticket.Detect(ref, "")
	}
	body := ""
	switch *bodyFile {
	case "":
	case "-":
		b, err := io.ReadAll(a.In)
		if err != nil {
			fmt.Fprintf(a.Err, "loops ticket: read stdin: %v\n", err)
			return 1
		}
		body = string(b)
	default:
		b, err := os.ReadFile(*bodyFile)
		if err != nil {
			fmt.Fprintf(a.Err, "loops ticket: %v\n", err)
			return 1
		}
		body = string(b)
	}
	t, err := a.Tickets.Fetch(ctx, src, ref, body)
	if err != nil {
		fmt.Fprintf(a.Err, "loops ticket: %v\n", err)
		return 1
	}
	md := ticket.Markdown(t)
	if dir, err := a.Tracker.Dir(ctx, ""); err == nil {
		if err := os.MkdirAll(dir, 0o755); err == nil {
			if err := os.WriteFile(filepath.Join(dir, "ticket.md"), []byte(md), 0o644); err != nil {
				fmt.Fprintf(a.Err, "loops ticket: write ticket.md: %v\n", err)
				return 1
			}
		}
		_ = a.Tracker.Record(ctx, run.Event{Kind: run.KindNote, Note: "ticket fetched", Target: t.Ref, Ref: "ticket.md"})
	}
	fmt.Fprint(a.Out, md)
	return 0
}
