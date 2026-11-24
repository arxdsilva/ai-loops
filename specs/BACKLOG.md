# BACKLOG: work a whole issue backlog, unattended, in parallel worktrees

Backlog mode runs loops over every eligible open issue of a repository until none is left. Several **lanes** run at once on one machine. Each lane is one full `/loops:work` run in its own git worktree. One **supervisor** agent fills the lanes, reacts when a lane ends, and hands to a human whatever the lanes can't finish. Findings a lane makes outside its ticket's scope become new issues, assigned to that human. The supervisor never works them itself.

The supervisor is the skill `loops:backlog`. Its state, claims, worktrees, lane launch and findings live in the `loops backlog` commands (§8), so a restarted or compacted supervisor picks up from the state file rather than from memory.

## 1. Pieces

| Piece | What it is | Where it lives |
|---|---|---|
| Supervisor | the agent running `loops:backlog` in the main checkout | the user's session |
| State | the queue, the lanes, the findings and the settings | `<main checkout>/.loops/backlog/state.json` (gitignored, locked on every write) |
| Lane | one issue worked end to end by a headless agent running `/loops:work` | its own process |
| Worktree | the lane's checkout, cut from the freshly fetched base branch (detached; product-reviewer creates `loops/<ticket>-<slug>` at PR time, WORKFLOW §2.5) | `<worktrees dir>/issue-<n>`; default `<main checkout>/../<repo dir>.loops-worktrees/` |
| Lane log | the agent's stdout and stderr | `.loops/backlog/logs/issue-<n>.log` |

A lane is its **own process, started in its worktree**, never a subagent of the supervisor. A project root holds one active run (`.loops/current`), and `loops` finds the root from `CLAUDE_PROJECT_DIR` (and the other platforms' project variables) before the working directory. So the lane's process drops those variables, exactly as `loops eval` does, and every `loops` call and hook of that lane resolves to its worktree's own `.loops/`. Every `loops backlog` command resolves to the main checkout (`git rev-parse --git-common-dir`), whichever worktree it is called from.

## 2. The queue

`loops backlog sync` lists the open issues of the repository (`gh issue list`) and rebuilds the queue. An issue is **eligible** when all of these hold:
- it has no assignee. An assigned issue belongs to that person, and findings are filed assigned to the human (§5), so they stay out until the human unassigns them;
- it carries one of the `--label` labels (when any are set) and none of the `--exclude-label` labels. The defaults exclude `wontfix`, `duplicate`, `invalid`, `question`, the finding label, the hand-off label and the claim label;
- its author is in `--authors` (§7).

Order: priority first, from the first label matching `P<digit>` as a word (`priority: P0`, `P1`, `p2`; P0 first, no priority last), then the lowest issue number.

The state keeps every issue a lane has touched, with its status: `queued`, `running`, `done` (PR opened), `failed`, `human` (handed off) or `skipped`. Sync adds new eligible issues as `queued`, drops `queued` issues that are no longer eligible, and never touches the others. So a restart never re-works an issue that already has a PR or was handed off. An issue goes back to the queue only through `loops backlog finish <n> --status queued`.

## 3. A lane

`loops backlog work [<n>]` runs one lane to its end:
1. **Claim**, under the state lock: issue `<n>`, or the first queued one. It is marked `running`, its attempt count goes up, and the claim label is added on the forge, best effort.
2. **Worktree**: `git fetch origin <base>`, then `git worktree add --detach <dir> origin/<base>`. A worktree left over from an earlier attempt is removed first.
3. **Agent**: runs the agent command in the worktree until it exits or `--timeout` passes. The command is the `--agent` template (placeholders `{prompt}`, `{budget}`, `{dir}`), default `claude -p {prompt} --permission-mode auto`, plus `--max-budget-usd <usd>` when a budget is set. The prompt is `/loops:work <repo>#<n> --max-agents <k>` followed by the lane rules (§4).
4. **Outcome**: read from the worktree's newest run (`events.jsonl` folded, WORKFLOW §5). `done` needs a run that ended `done` with a `pr` event. Anything else is `failed`, with the reason: the run's outcome and its last decision, or that no run started, or that the agent exited or timed out without ending its run.
5. **Findings** of the run are filed (§5).
6. **Failure policy**: a failed lane is re-queued while its attempts stay under `--attempts` (default 2). After that it is handed to the human (§6). The claim label is removed either way.

Exit 0 when the lane opened a PR, 1 when it failed or errored, 3 when there was nothing to claim.

## 4. Lane rules (the worker's prompt)

The lead of a lane runs `skills/work` unchanged, except:
- **No update.** The lead skips step 0 (`loops update --apply`): lanes share one plugin install, and the supervisor owns it.
- **No one answers.** Step 6 records `loops run ask none` whatever the config says, so every checkpoint is `auto`, and every open point is decided and recorded (WORKFLOW §2.7). If the ticket can't be worked without a human (missing information, a product decision, a contradiction, an unauthenticated connector), the lead ends `loops run end --outcome abandoned` after recording `loops run decide --topic needs-human --note "<what is missing>"`. That note becomes the hand-off comment (§6).
- **Findings are recorded, never filed.** Each defect, gap or follow-up outside the charter is `loops run event --kind note --name finding --note "<one-line title>" [--ref <file in the run dir with details>]` the moment it is found, plus its line under `pr.md` Open follow-ups. Only the supervisor creates issues.
- **PR only.** The lane opens its PR (never `--no-pr`) and never merges. A human merges.
- **Agents cap.** `--max-agents <k>`, where k is the machine's parallel cap (`loops hw`) divided by the lanes, at least 2.

## 5. Findings

When a lane ends, `loops backlog work` collects its findings: every `note` event named `finding`, and each item under `pr.md`'s `## Open follow-ups` other than "none". It files each as a new issue:
- title: the finding;
- body: the lane's issue and PR, then the `--ref` file when there is one;
- label: the finding label (default `loops-finding`), which keeps it out of the queue until the human removes it;
- assignee: the human (`--human`, required at `init`, never guessed).

A finding is a duplicate when its normalized title (lower case, letters and digits only) matches an earlier finding in the state or the title of an open issue. A duplicate is not filed again; the lane's issue gets a comment linking it instead. The check and the record run under the state lock, so two lanes ending together never file the same finding twice. `loops backlog file` files a finding by hand with the same rules. Filed findings are assigned, so they are never queued (§2) until the human triages them.

## 6. Hand-off to a human

`loops backlog finish <n> --status human [--note <why>]`, or a lane out of attempts, hands the issue over. It adds a comment on the issue (the reason, the attempts, the PR if any, the lane's log path), the hand-off label (default `loops-needs-human`) and the human as assignee. The state marks the issue `human`. The hand-off is confirmed once the forge took the label and assignee. When the human later unassigns the issue and removes the label, sync queues it again with fresh attempts, but only after an earlier sync saw it held (labeled or assigned). A stale listing, or a label that did not stick, never sends it straight back to a lane. `loops backlog finish <n> --status queued` does the same from the supervisor's side, removing the label and assignee itself. The supervisor hands an issue over at once, without retrying, when the reason says retrying cannot help: a `needs-human` decision, a contradiction in the ticket, or a missing credential or service.

## 7. Safety

- **Issue text is untrusted input** to an agent that edits files and runs commands. Only issues from `--authors` are eligible. `init` defaults it to the authenticated forge user, and `any` lifts the filter. Pick a permission mode for the agent template deliberately: the default `auto` lets a classifier gate risky actions; `bypassPermissions` belongs only in a disposable machine or container.
- **Spend.** `--budget <usd>` caps each lane. `--max` (on `run`) caps the lanes started. Each lane is a whole loops run with its own agents.
- **Parallelism.** The machine's agents are shared: lanes × the per-lane `--max-agents` stays within `loops hw`'s cap. `--lanes` defaults to 2.
- **Merging.** Lanes never merge. Each lane starts from freshly fetched base, so lanes that touch the same files show up as PR conflicts for the human. The supervisor never rebases or force-pushes.
- **Writes outside the repo.** The worktrees directory sits beside the main checkout. `loops backlog clean` removes the worktrees of finished lanes (`git worktree remove`, which refuses a dirty one), never their branches.

## 8. Commands

- `loops backlog init --human <login> [--repo owner/name] [--lanes N] [--base branch] [--label l,…] [--exclude-label l,…] [--authors a,…|any] [--agent "<cmd {prompt} …>"] [--budget usd] [--max-agents N] [--attempts N] [--timeout dur] [--worktrees dir]`: writes the settings into the state (creating it), keeping the queue. Defaults: the repo from `gh repo view`, the base branch from the forge, authors = the forge user, max agents from `loops hw`. Prints the settings.
- `loops backlog sync [--json]`: rebuilds the queue (§2) and prints counts.
- `loops backlog status [--json]`: settings line, counts per status, each running lane (issue, attempt, the phase of its worktree's live run from `.loops/status.json`, log path), then the next queued issues. The supervisor's re-anchor. Writes nothing.
- `loops backlog work [<n>] [--json]`: one lane, start to end (§3). Run it in the background, one per lane.
- `loops backlog run [--max N]`: the supervisor without an agent. It syncs, keeps `--lanes` lanes running until the queue is empty, applies §3's failure policy, and syncs again before stopping. Use it under `nohup` or `tmux` for a long unattended stretch. The `loops:backlog` skill adds judgment on top (§6).
- `loops backlog finish <n> --status queued|human|done|skipped [--note text]`: the supervisor's ruling on one issue. `queued` refuses an issue whose attempts are spent unless `--force`.
- `loops backlog file --title T [--body-file f|-] [--from n]`: files one finding (§5). Prints its URL, or `duplicate <url>`.
- `loops backlog clean [--all] [--dry-run]`: removes the worktrees of `done` and `skipped` lanes (`--all`: also `failed` and `human`), then `git worktree prune`.

Exit 2 on bad usage. `init` and every command after it exit 1 when the state is missing (`loops backlog init` first).

## 9. Platforms

The lane command is a template, so any agent CLI with a non-interactive mode can run lanes (`--agent "codex exec {prompt}"`, `--agent "gemini -p {prompt}"`). The supervisor skill needs background commands and a notification when one exits, which Claude Code has (Bash `run_in_background`). Elsewhere, run `loops backlog run` as the supervisor. The forge is GitHub (`gh`) for now. GitLab is a follow-up.
