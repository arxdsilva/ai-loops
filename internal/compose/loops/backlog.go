package loops

import (
	"context"

	"github.com/arxdsilva/ai-loops/internal/adapters/inbound/cli"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/agentcmd"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/backlogstore"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/ghforge"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/gitworktree"
	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/lanefs"
	appbacklog "github.com/arxdsilva/ai-loops/internal/application/backlog"
)

// backlogEnv wires `loops backlog` on the main checkout of the repository
// holding root, so a call from inside a lane's worktree reaches the same
// state as the supervisor (specs/BACKLOG.md §1).
func backlogEnv(root string) func(context.Context, func(string, ...any)) cli.BacklogEnv {
	return func(ctx context.Context, warn func(string, ...any)) cli.BacklogEnv {
		main := gitworktree.MainRoot(ctx, root)
		gh := ghforge.New(main)
		svc := appbacklog.New(backlogstore.New(main), gh, gh, gitworktree.New(main),
			agentcmd.Lane{}, lanefs.Lanes{}, systemClock{})
		svc.Warn = warn
		return cli.BacklogEnv{Svc: svc, Worktrees: gitworktree.DefaultWorktrees(main)}
	}
}
