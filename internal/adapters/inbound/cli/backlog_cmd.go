package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	appbacklog "github.com/arxdsilva/ai-loops/internal/application/backlog"
	"github.com/arxdsilva/ai-loops/internal/domain/backlog"
	"github.com/arxdsilva/ai-loops/internal/domain/hw"
)

// Backlogger is backlog mode's use cases (specs/BACKLOG.md).
type Backlogger interface {
	Init(ctx context.Context, in appbacklog.InitInput) (backlog.Settings, error)
	Sync(ctx context.Context) (backlog.SyncResult, error)
	Status(ctx context.Context) (appbacklog.Report, error)
	Work(ctx context.Context, n int) (appbacklog.LaneResult, error)
	Run(ctx context.Context, in appbacklog.RunInput) (appbacklog.RunSummary, error)
	Finish(ctx context.Context, n int, status backlog.Status, note string, force bool) (backlog.Item, error)
	File(ctx context.Context, title, body string, from int) (appbacklog.Filed, error)
	Clean(ctx context.Context, all, dryRun bool) ([]string, error)
}

// BacklogEnv is the backlog service of the main checkout, built on demand
// (finding the main checkout runs git, which other commands never need).
type BacklogEnv struct {
	Svc       Backlogger
	Worktrees string // the default worktrees directory
}

const backlogUsage = "usage: loops backlog init|sync|status|work|run|finish|file|clean … (specs/BACKLOG.md §8)"

// cmdBacklog implements `loops backlog <sub>`.
func (a *App) cmdBacklog(ctx context.Context, args []string) int {
	if len(args) == 0 || a.Backlog == nil {
		return a.usageErr(backlogUsage)
	}
	env := a.Backlog(ctx, func(format string, v ...any) {
		fmt.Fprintf(a.Err, "loops backlog: warning: "+format+"\n", v...)
	})
	sub, rest := args[0], args[1:]
	if sub == "work" || sub == "run" {
		// A stopped lane (Ctrl-C, a harness stopping the task, a closed
		// terminal) stops its agent and records the attempt instead of
		// orphaning the agent, which runs in its own process group. SIGHUP
		// stays ignored under nohup.
		sigs := []os.Signal{os.Interrupt, syscall.SIGTERM}
		if !signal.Ignored(syscall.SIGHUP) {
			sigs = append(sigs, syscall.SIGHUP)
		}
		var stop context.CancelFunc
		ctx, stop = signal.NotifyContext(ctx, sigs...)
		defer stop()
	}
	switch sub {
	case "init":
		return a.backlogInit(ctx, env, rest)
	case "sync":
		return a.backlogSync(ctx, env.Svc, rest)
	case "status":
		return a.backlogStatus(ctx, env.Svc, rest)
	case "work":
		return a.backlogWork(ctx, env.Svc, rest)
	case "run":
		return a.backlogRun(ctx, env.Svc, rest)
	case "finish":
		return a.backlogFinish(ctx, env.Svc, rest)
	case "file":
		return a.backlogFile(ctx, env.Svc, rest)
	case "clean":
		return a.backlogClean(ctx, env.Svc, rest)
	}
	return a.usageErr(backlogUsage)
}

func (a *App) printJSON(v any) int {
	enc := json.NewEncoder(a.Out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return a.fail("backlog", err)
	}
	return 0
}

func (a *App) backlogInit(ctx context.Context, env BacklogEnv, args []string) int {
	fs := a.newFlagSet("backlog init")
	var s backlog.Settings
	fs.StringVar(&s.Human, "human", "", "forge login findings and hand-offs are assigned to (required on first init)")
	fs.StringVar(&s.Repo, "repo", "", "owner/name (default: this checkout's repo)")
	fs.IntVar(&s.Lanes, "lanes", 0, "lanes running at once (default 2)")
	fs.StringVar(&s.Base, "base", "", "branch lanes start from (default: the repo's default branch)")
	labels := fs.String("label", "", "only issues with one of these labels (comma-separated)")
	exclude := fs.String("exclude-label", "", "never issues with these labels (default wontfix,duplicate,invalid,question)")
	authors := fs.String("authors", "", "only issues by these authors, or any (default: the forge user)")
	fs.StringVar(&s.Agent, "agent", "", "lane command template with {prompt} (default: claude -p {prompt} --permission-mode auto)")
	fs.StringVar(&s.Budget, "budget", "", "USD cap per lane, or none to clear it (default: none)")
	fs.IntVar(&s.MaxAgents, "max-agents", 0, "agents per lane (default: loops hw's cap / lanes)")
	fs.IntVar(&s.Attempts, "attempts", 0, "lane attempts per issue before the hand-off (default 2)")
	fs.StringVar(&s.Timeout, "timeout", "", "per-lane time limit (default 4h)")
	fs.StringVar(&s.Worktrees, "worktrees", "", "where lane worktrees go (default: beside the checkout)")
	maxPRs := fs.String("max-prs", "", "lane PRs open at once before lanes pause, or none (default 5)")
	const usage = "usage: loops backlog init --human <login> [--repo o/n] [--lanes N] [--base b] [--label l,…] [--exclude-label l,…] [--authors a,…|any] [--agent tmpl] [--budget usd] [--max-agents N] [--attempts N] [--timeout d] [--max-prs N|none] [--worktrees dir]"
	if pos, err := parseFlagsAnywhere(fs, args); err != nil || len(pos) > 0 {
		return a.usageErr(usage)
	}
	switch n, err := strconv.Atoi(*maxPRs); {
	case *maxPRs == "":
	case *maxPRs == "none":
		s.MaxPRs = -1
	case err != nil || n <= 0:
		return a.usageErr("loops backlog init: --max-prs takes a positive number or none")
	default:
		s.MaxPRs = n
	}
	if *labels != "" {
		s.Labels = splitList(*labels)
	}
	if *exclude != "" {
		s.Exclude = splitList(*exclude)
	}
	in := appbacklog.InitInput{Settings: s, Worktrees: env.Worktrees, MachineCap: hw.DefaultMaxAgents}
	if *authors == "any" {
		in.AnyAuthor = true
	} else if *authors != "" {
		in.Settings.Authors = splitList(*authors)
	}
	if a.HW != nil {
		if facts, err := a.HW.Probe(ctx, a.Root); err == nil {
			in.MachineCap = hw.Recommend(facts, a.Parallel).MaxParallelAgents
		}
	}
	set, err := env.Svc.Init(ctx, in)
	if err != nil {
		return a.fail("backlog init", err)
	}
	printSettings(a.Out, set)
	fmt.Fprintln(a.Out, "next: loops backlog sync, then loops backlog status")
	return 0
}

func printSettings(w io.Writer, s backlog.Settings) {
	authors := strings.Join(s.Authors, ",")
	if authors == "" {
		authors = "any"
	}
	agent := s.Agent
	if agent == "" {
		agent = "default (claude -p … --permission-mode auto)"
	}
	fmt.Fprintf(w, "repo %s, base %s, human @%s\n", s.Repo, s.Base, s.Human)
	prs := "none"
	if c := s.PRCap(); c > 0 {
		prs = strconv.Itoa(c)
	}
	fmt.Fprintf(w, "lanes %d × %d agents, %d attempts, timeout %s, budget %s, open PRs cap %s\n", s.Lanes, s.MaxAgents, s.Attempts, s.Timeout, orDash(s.Budget), prs)
	fmt.Fprintf(w, "issues: labels %s, excluded %s, authors %s\n", orDash(strings.Join(s.Labels, ",")), strings.Join(s.Exclude, ","), authors)
	fmt.Fprintf(w, "agent: %s\nworktrees: %s\n", agent, s.Worktrees)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func (a *App) backlogSync(ctx context.Context, svc Backlogger, args []string) int {
	fs := a.newFlagSet("backlog sync")
	asJSON := fs.Bool("json", false, "print JSON")
	if pos, err := parseFlagsAnywhere(fs, args); err != nil || len(pos) > 0 {
		return a.usageErr("usage: loops backlog sync [--json]")
	}
	res, err := svc.Sync(ctx)
	if err != nil {
		return a.fail("backlog sync", err)
	}
	if *asJSON {
		return a.printJSON(res)
	}
	fmt.Fprintf(a.Out, "sync: %d open issues, %d added, %d dropped, %d queued\n", res.Open, res.Added, res.Dropped, res.Queued)
	if res.Filed > 0 || res.Handed > 0 {
		fmt.Fprintf(a.Out, "retried: %d pending findings filed, %d hand-offs confirmed\n", res.Filed, res.Handed)
	}
	if g := res.Gate; g.Cap > 0 {
		state := "lanes may start"
		if g.Closed() {
			state = "lanes paused until some are merged or closed"
		}
		fmt.Fprintf(a.Out, "open PRs: %d of %d (lane PRs and running lanes): %s\n", g.Open, g.Cap, state)
	}
	return 0
}

func (a *App) backlogStatus(ctx context.Context, svc Backlogger, args []string) int {
	fs := a.newFlagSet("backlog status")
	asJSON := fs.Bool("json", false, "print JSON")
	if pos, err := parseFlagsAnywhere(fs, args); err != nil || len(pos) > 0 {
		return a.usageErr("usage: loops backlog status [--json]")
	}
	r, err := svc.Status(ctx)
	if err != nil {
		return a.fail("backlog status", err)
	}
	if *asJSON {
		return a.printJSON(r)
	}
	printSettings(a.Out, r.Settings)
	fmt.Fprintf(a.Out, "synced: %s · findings filed: %d\n", orDash(r.Synced), r.Findings)
	var counts []string
	for _, st := range []backlog.Status{backlog.Queued, backlog.Running, backlog.Done, backlog.Failed, backlog.Human, backlog.Skipped} {
		counts = append(counts, fmt.Sprintf("%s %d", st, r.Counts[st]))
	}
	fmt.Fprintln(a.Out, strings.Join(counts, " · "))
	for _, l := range r.Running {
		alive := "live"
		if !l.Alive {
			alive = "process gone (next sync recovers it)"
		}
		fmt.Fprintf(a.Out, "running #%d (attempt %d, %s, phase %s): %s\n  log %s\n", l.Item.Number, l.Item.Attempts, alive, orDash(l.Phase), l.Item.Title, l.Log)
	}
	for _, it := range r.Next {
		fmt.Fprintf(a.Out, "next #%d (P%s): %s\n", it.Number, prio(it.Priority), it.Title)
	}
	return 0
}

func prio(p int) string {
	if p >= backlog.NoPriority {
		return "-"
	}
	return strconv.Itoa(p)
}

func (a *App) backlogWork(ctx context.Context, svc Backlogger, args []string) int {
	fs := a.newFlagSet("backlog work")
	asJSON := fs.Bool("json", false, "print JSON")
	pos, err := parseFlagsAnywhere(fs, args)
	if err != nil || len(pos) > 1 {
		return a.usageErr("usage: loops backlog work [<issue>] [--json]")
	}
	n := 0
	if len(pos) == 1 {
		if n, err = strconv.Atoi(strings.TrimPrefix(pos[0], "#")); err != nil || n <= 0 {
			return a.usageErr("loops backlog work: issue must be a number")
		}
	}
	res, err := svc.Work(ctx, n)
	if err != nil {
		return a.fail("backlog work", err)
	}
	if *asJSON {
		if code := a.printJSON(res); code != 0 {
			return code
		}
	} else {
		printLane(a.Out, res)
	}
	switch {
	case res.Empty:
		return 3
	case res.Gated:
		return 4
	case res.Status == backlog.Done:
		return 0
	}
	return 1
}

func printLane(w io.Writer, r appbacklog.LaneResult) {
	if r.Empty {
		fmt.Fprintln(w, "nothing queued (loops backlog sync)")
		return
	}
	if r.Gated {
		fmt.Fprintf(w, "paused: %d of %d lane PRs open (--max-prs); no lane started\n", r.Gate.Open, r.Gate.Cap)
		return
	}
	switch r.Status {
	case backlog.Done:
		fmt.Fprintf(w, "#%d done: %s\n", r.Item.Number, r.Item.PR)
	case backlog.Queued:
		fmt.Fprintf(w, "#%d failed (attempt %d), queued again: %s\n", r.Item.Number, r.Item.Attempts, r.Reason)
	default:
		fmt.Fprintf(w, "#%d handed to the human: %s\n", r.Item.Number, r.Reason)
	}
	for _, f := range r.Findings {
		state := "filed"
		if f.Duplicate {
			state = "already tracked"
		}
		fmt.Fprintf(w, "finding %s: %s %s\n", state, f.Title, f.URL)
	}
}

func (a *App) backlogRun(ctx context.Context, svc Backlogger, args []string) int {
	fs := a.newFlagSet("backlog run")
	maxLanes := fs.Int("max", 0, "lanes to start in all (0 = until the queue is empty)")
	wait := fs.Duration("wait", 0, "keep watching: at the open-PR cap or with nothing queued, sync again after this long instead of stopping")
	if pos, err := parseFlagsAnywhere(fs, args); err != nil || len(pos) > 0 || *maxLanes < 0 || *wait < 0 {
		return a.usageErr("usage: loops backlog run [--max N] [--wait dur]")
	}
	sum, err := svc.Run(ctx, appbacklog.RunInput{Max: *maxLanes, Out: a.Out, Wait: *wait})
	fmt.Fprintf(a.Out, "run: %d lanes, %d PRs, %d retried, %d handed off, %d findings, %d errors\n",
		sum.Started, sum.Done, sum.Retried, sum.Human, sum.Findings, sum.Errors)
	if err != nil {
		return a.fail("backlog run", err)
	}
	return 0
}

func (a *App) backlogFinish(ctx context.Context, svc Backlogger, args []string) int {
	fs := a.newFlagSet("backlog finish")
	status := fs.String("status", "", "queued|human|done|skipped")
	note := fs.String("note", "", "why (goes into the hand-off comment)")
	force := fs.Bool("force", false, "queue an issue whose attempts are spent")
	pos, err := parseFlagsAnywhere(fs, args)
	usage := "usage: loops backlog finish <issue> --status queued|human|done|skipped [--note text] [--force]"
	if err != nil || len(pos) != 1 {
		return a.usageErr(usage)
	}
	n, err := strconv.Atoi(strings.TrimPrefix(pos[0], "#"))
	if err != nil || n <= 0 {
		return a.usageErr(usage)
	}
	st, err := backlog.ParseStatus(*status)
	if err != nil {
		return a.usageErr("loops backlog finish: " + err.Error())
	}
	it, err := svc.Finish(ctx, n, st, *note, *force)
	if err != nil {
		return a.fail("backlog finish", err)
	}
	fmt.Fprintf(a.Out, "#%d %s\n", it.Number, it.Status)
	return 0
}

func (a *App) backlogFile(ctx context.Context, svc Backlogger, args []string) int {
	fs := a.newFlagSet("backlog file")
	title := fs.String("title", "", "the finding, one line")
	bodyFile := fs.String("body-file", "", "details, or - for stdin")
	from := fs.Int("from", 0, "the issue whose lane found it")
	if pos, err := parseFlagsAnywhere(fs, args); err != nil || len(pos) > 0 || *title == "" {
		return a.usageErr("usage: loops backlog file --title T [--body-file f|-] [--from n]")
	}
	body := ""
	switch *bodyFile {
	case "":
	case "-":
		b, err := io.ReadAll(a.In)
		if err != nil {
			return a.fail("backlog file", err)
		}
		body = string(b)
	default:
		b, err := os.ReadFile(*bodyFile)
		if err != nil {
			return a.fail("backlog file", err)
		}
		body = string(b)
	}
	f, err := svc.File(ctx, *title, body, *from)
	if err != nil {
		return a.fail("backlog file", err)
	}
	if f.Duplicate {
		fmt.Fprintf(a.Out, "duplicate %s\n", f.URL)
	} else {
		fmt.Fprintln(a.Out, f.URL)
	}
	return 0
}

func (a *App) backlogClean(ctx context.Context, svc Backlogger, args []string) int {
	fs := a.newFlagSet("backlog clean")
	all := fs.Bool("all", false, "also failed and handed-off lanes")
	dry := fs.Bool("dry-run", false, "list only")
	if pos, err := parseFlagsAnywhere(fs, args); err != nil || len(pos) > 0 {
		return a.usageErr("usage: loops backlog clean [--all] [--dry-run]")
	}
	removed, err := svc.Clean(ctx, *all, *dry)
	if err != nil {
		return a.fail("backlog clean", err)
	}
	verb := "removed"
	if *dry {
		verb = "would remove"
	}
	for _, d := range removed {
		fmt.Fprintf(a.Out, "%s %s\n", verb, d)
	}
	if len(removed) == 0 {
		fmt.Fprintln(a.Out, "no finished worktrees")
	}
	return 0
}
