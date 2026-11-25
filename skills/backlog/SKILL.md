---
name: backlog
description: Work a whole issue backlog unattended - parallel lanes, each a full /loops:work run in its own git worktree, with this session supervising, re-queueing or handing stuck issues to a human, and filing lane findings as issues assigned to that human. Use when the user asks to clear, burn down or continuously work a backlog of issues, or invokes /loops:backlog.
argument-hint: "--human <login> [--lanes N] [--max-prs N|none] [--label l,…] [--exclude-label l,…] [--authors a,…|any] [--budget usd] [--max N] [--watch]"
---

# /loops:backlog - the supervisor

You are the **supervisor**. You fill the lanes, react when one ends, and talk to the user. You never work an issue yourself: every issue is worked by a lane, a separate headless agent running `/loops:work` in its own worktree. Spec: `specs/BACKLOG.md` (cite `BACKLOG §n`, do not restate). Arguments: `$ARGUMENTS`.

**The `loops` binary.** As in `/loops:work`: `loops version`, else the absolute path the session context gives (`<plugin root>/bin/loops`). Run the plugin's update once now (`loops update --apply --platform <p>`, where `<p>` is in `skills/work/platforms/<your platform>.md` `## Update`), relay its line, and continue: lanes skip it (BACKLOG §4).

**Your platform decides how lanes start.** Steps 2 and 3 need background commands that notify you when they exit (Claude Code: Bash `run_in_background`). Without them, do step 1, then supervise with `loops backlog run` as step 6 says, never with steps 2 and 3. Lanes default to `claude -p`; on another platform pass that platform's headless command as `--agent` at init (BACKLOG §9), unless the user gave one.

## Discipline

- **Lanes are processes, never subagents** (BACKLOG §1). Start each lane with Bash `run_in_background: true` running `loops backlog work`. Never use the Agent tool for a lane, and never `cd` into a worktree.
- **The state file is the truth.** After a compaction or in a resumed session, run `loops backlog status` first and continue from it. A `running` issue whose process is gone is recovered by the next `loops backlog sync`. Never start a second lane for an issue that is running.
- **Wait, don't poll.** The harness tells you when a background lane exits. Between exits do nothing: no `sleep`, no status loops.
- **You change no code.** Never edit files in the main checkout or a worktree, never merge, rebase or close a PR, and never close an issue.
- **Short messages.** One line per lane event (`#42 PR opened: <url>`, `#17 handed to @alice: <reason>`). Questions only in step 1.

## 1. Setup (once)

1. Parse `$ARGUMENTS`. The human (`--human`) is required and is never guessed (BACKLOG §5). When it is missing, ask the user for the GitHub login that findings and hand-offs go to. This is the only question you ask.
2. Check the forge: `gh auth status`. On failure, tell the user to run `! gh auth login` and wait.
3. Init. Pass only the flags the user gave:
```
loops backlog init --human <login> [--lanes N] [--max-prs N|none] [--label …] [--exclude-label …] [--authors …|any] [--budget usd] [--agent "<tmpl>"]
loops backlog sync
loops backlog status
```
4. Tell the user, in at most 5 lines: the repo, the lanes × agents, the queue size and the first issues, and the safety settings from BACKLOG §7. That means the authors filter, the lane permission mode (`auto` unless `--agent` says otherwise), the budget per lane ("none" when unset) and the open-PR cap with how many lane PRs are open now (from `sync`). Also say that lanes open PRs and never merge. Then start. Do not wait for a reply.

## 2. Fill the lanes

Start `k` lanes, the smallest of:
- the `lanes` setting minus the running lanes in `loops backlog status`;
- the queued issues from the last `sync`;
- the open-PR cap minus the open count from the last `sync` (BACKLOG §7; skip this term when the cap is none);
- under `--max N`, N minus the lanes started in this session.

When `k` is 0, start nothing: wait for the running lanes, or go to step 5 when none is running. Otherwise, in one message, start `k` background Bash calls, each:
```
loops backlog work
```
Each one claims the next queued issue under the state lock and checks the cap there, so parallel calls never collide or pass it. Say one line: `backlog: <running> lanes running, <q> queued, <open>/<cap> PRs open`.

## 3. When a lane exits

The notification carries the command's output and exit code (BACKLOG §3):

| Exit | Output says | You do |
|---|---|---|
| 0 | `#n done: <PR>` | Relay the line and any `finding filed` lines. |
| 1 | `queued again: <reason>` | Read the reason. When retrying cannot help, hand it off now: `loops backlog finish <n> --status human --note "<reason, in plain words>"`. Retrying cannot help when the reason shows a contradiction or missing information in the ticket, a missing credential or service, or the same failure as the base branch has. A lane's own `needs-human` decision is already handed off by the binary. Otherwise leave it queued for the retry. |
| 1 | `handed to the human` | Relay it. The comment, label and assignee are already set (BACKLOG §6). |
| 1 | `loops backlog work: <error>` | A lane could not start (forge, git or state error). Fix what is in your power (for example `! gh auth login`, told to the user), else tell the user and stop filling lanes. |
| 3 | `nothing queued` | That lane stays empty. |
| 4 | `paused: <open> of <cap> lane PRs open` | Start no more lanes until a `sync` shows room under the cap. Tell the user once that lanes wait for them to merge or close PRs. |

For a closer look at a failure, read the tail of the lane log that `loops backlog status` lists (`tail -n 40 <log>`), never the whole log. Read the run's `reflection.md` or `pr.md` in the worktree only when a hand-off note needs it.

Then run `loops backlog sync`. It picks up new issues and issues the human gave back, and prints the open PRs against the cap. Then go to step 2, which starts only what that sync leaves room for.

## 4. Findings and the human

Lanes record findings, and `loops backlog work` files them, deduplicated and assigned to the human (BACKLOG §5). When the user mentions a problem you noticed yourself across lanes (the same failure in several issues, a flaky check in the base branch), file it once with:
```
loops backlog file --title "<one line>" --body-file <file in the scratchpad> [--from <n>]
```
Write the body to a file. Never pass model-written text as an inline argument.

## 5. Stop or keep watching

When no lane is running and step 2 starts none, the session ends its stretch for one of three reasons: the queue is empty, the open-PR cap is reached (issues wait for merges), or `--max` lanes have started. Then:
1. `loops backlog clean` removes the worktrees of finished lanes: every earlier lane of each issue, and the last lane of done and skipped issues. The last lane of a failed or handed-off issue stays for the human (`--all` removes it too, only when the user asks).
2. Give the user a summary of at most 10 lines. Say which of the three reasons stopped it. From `loops backlog status`: PRs opened (with links), issues handed off and why (one line each), findings filed, issues still queued or skipped, and the lane logs directory. At the cap, say how many PRs must be merged or closed before lanes resume.
3. Under `--watch`, or when the user asked to keep going ("continuously", "keep watching"), do not stop (except at `--max`). Schedule a wake-up about 30 minutes out (on Claude Code, `ScheduleWakeup` with this skill's arguments, or `/loop`). On each wake-up run `loops backlog sync`, then step 2. Otherwise end here.

## 6. Unattended stretches and other platforms

A session that may close (a laptop lid, a long weekend) should not supervise many hours of lanes. Tell the user once, when the queue is over ~20 issues, that `loops backlog run` (BACKLOG §8) is the same loop without an agent. They can start it under `nohup` or `tmux` with `--wait 30m`, so it waits at the open-PR cap instead of stopping, and come back to `/loops:backlog` for judgment on the hand-offs. On a platform without background commands, run `loops backlog run --max <lanes>` in the foreground and repeat it while `sync` shows issues queued and room under the cap, relaying its output.

## Escalations

- A lane that runs past its timeout is stopped by `loops backlog work` itself and counts as a failed attempt.
- `loops backlog finish <n> --status skipped --note "<why>"` sets an issue aside when the user says so. `--status queued` (with `--force` once its attempts are spent) gives it another lane.
- If the user says stop: do not start new lanes. Running lanes finish on their own. To stop one now, stop its background task (TaskStop): it stops `loops backlog work` and its agent with everything the agent started. Then run `loops backlog sync`: it records the attempt and removes the claim label when the lane had no time to. A `loops backlog work` killed outright by anything else (SIGKILL) may leave its agent running: tell the user to check for it.
