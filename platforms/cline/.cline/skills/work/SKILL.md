---
name: work
description: Work a ticket, issue or task end to end with the loops workflow (research, plan, parallel implementation, QA, product proof, PR, reflection). Use when the user asks to work, implement or ship a ticket/issue, or invokes /loops:work.
argument-hint: "<ticket ref or text> [--ask phases|all] [--approve] [--no-pr] [--models=cheap] [--max-agents N] [--budget phase=200k,...] [--agents phase=N,...] [--roles phase=role,...]"
---

# /loops:work - the lead's orchestrator

**Read `platforms/<your platform>.md` beside this file before starting** (Claude Code: claude.md; GitHub Copilot: copilot.md; Codex: codex.md; Cursor: cursor.md; Gemini CLI: gemini.md; OpenCode: opencode.md; Amp: amp.md; Windsurf/Devin: windsurf.md; Cline: cline.md). It defines the verbs used below: *spawn*, *resume or re-spawn*, *ask the user*, *stop the agent*, *label*. Tiers: `specs/PLATFORMS.md` §4.

You are the **lead**, the run's master agent: you run every phase yourself from intake to `loops run end`, hold the scope in `$RUN/charter.md`, take the decisions and are the only one who talks to the user (WORKFLOW §2.7). Spec: `specs/WORKFLOW.md` (cite `WORKFLOW §n`, do not restate). Roles: `ROLES.md`. Arguments: `$ARGUMENTS`.

**The `loops` binary.** First check `loops version`. If `loops` is not on PATH, use the absolute path the session context gives (`<plugin root>/bin/loops`), or, where the platform has no plugin root, install it once with `npx ai-loops --target <platform>` (prebuilt and checksum-verified, no Go needed; other ways in `docs/GUIDE.md#install`). Every role agent you spawn gets the same instruction, through the prompt header line `Loops: <abs path to loops>`.

## Discipline

- Route, relay, record. Do not read code dumps or research files in full; read verdict/summary lines and open a file only when a decision needs it. Do one-fact lookups and edits under ~30 lines yourself; everything else goes to a role (WORKFLOW §1).
- Do not enter a planning or read-only mode while agents run (it can pause them).
- **Re-anchor, never recall.** `loops run phase` prints the brief (phase path, checkpoints, open failures, decisions, next step): read it before acting. After a compaction or in a resumed session, run `loops run brief` first and continue from it; never restart a finished phase. The brief wins over your memory.
- **Decide, don't ask.** At each phase boundary run `loops run checkpoint <phase>`. `auto` (the default): decide yourself as WORKFLOW §2.7 says and record it (`loops run decide --topic … --note "<decision; reason>"`). `ask`: stop for the user as its table says and record the answer (`--by user`). The user may opt in or out mid-run ("ask me before the PR", "stop asking"): record a new `loops run ask <list>`.
- **Guard the scope.** Check each phase's output against the charter before moving on (WORKFLOW §2.7). Out-of-scope work goes to Open follow-ups, never into the run.
- User messages are short: one line per phase transition naming what is running (e.g. `research: 4 researchers running`). Outside a checkpoint, never a question.
- Every spawn names the role and (when limits are in force) first passes `loops budget check` (section 0a); never exceed the effective concurrency cap (section 3); spawn all agents of a wave together, in parallel and in the background where your platform allows. Pass a model only to escalate or under `--models=cheap`.
- Tier D (solo mode): where "spawn" appears, play the role yourself in phase order, write the same run-dir files, and re-read only that role's input files first (PLATFORMS §4).
- Never ask agents to commit. Only `product-reviewer` commits, after confirmation.

**Models.** Spawn each role at its tier. The planner and plan-reviewer default to opus. If a model is unavailable, fall back to the best one the user has and record it (WORKFLOW §3a).

## 0. Parse and intake (WORKFLOW §2.0)

1. Parse flags from `$ARGUMENTS`: `--ask <phases>`, `--approve` (= `--ask plan`), `--no-pr`, `--models=cheap`, and the opt-in limits `--max-agents N`, `--budget …`, `--agents …`, `--roles …` (WORKFLOW §4a). Also read the user's own words for checkpoints: "let me approve the plan" = `plan`, "ask me before the PR" = `pr`, "check with me at every phase" = `all`; they win over flags. Read `.loops/config.json` if present (`ask`, `memory`).
2. Start the run and capture the id and dir:
```
loops run start --title "<short title>" [--ticket <ref>] [--source <s>]
loops run dir; loops path specs; loops path templates
```
Note the three printed absolute paths: below, `$RUN`, `$SPECS` and `$TEMPLATES` mean those literal paths (shell variables do not persist between Bash calls).
3. Fetch the ticket per WORKFLOW §2.0: GitHub/GitLab `loops ticket fetch <ref>`; Jira/Linear via the connected MCP tool, then `... | loops ticket fetch <ref> --source jira|linear --body-file -`; free text `loops ticket fetch --source text --body-file -`. If a connector is unauthenticated and there is no ticket text, tell the user what is missing and wait: the one stop you cannot decide away.
4. **Languages.** `loops lang detect` prints the detected packs and their roots. Each implementation lane is a layer role plus one pack (ROLES.md). Nothing detected: continue (the gate will check nothing), put that under the charter's Constraints, and tell the user in one line that `languages` in `.loops/config.json` pins packs (`/loops:init` step 1a).
5. **ai-memory, once** (only if its tools exist and `memory_status` succeeds; else skip silently): call `memory_briefing`, then one `memory_query` on the ticket's key terms (pass `workspace`/`project` per WORKFLOW §7). Write hits to `$RUN/research/memory.md` (mark as unverified evidence) and run `loops run event --kind memory --name briefing`.
6. **Checkpoints.** `loops run ask [<list from step 1>]` records them (no list = the config) and prints them.
7. **Charter.** Write `$RUN/charter.md` from `$TEMPLATES/CHARTER.md`: goal, ACs from `ticket.md`, in and out of scope, constraints, assumptions (each also `loops run decide --topic assumption …`). Then `loops run checkpoint intake`; `ask`: show it and ask Continue / Edit scope / Stop.

### 0a. Opt-in limits (WORKFLOW §4a)

Only when the user passed a limit flag or `.loops/config.json` has `limits`; otherwise skip this whole section (nothing is limited, nothing asks).
1. At intake: `loops budget parse --budget <v> --agents <v> --roles <v> --max-agents <N>` prints the limits as JSON (omit flags not given). Record them: `loops run event --kind limits --note '<that json>'`.
2. Before every spawn: `loops budget check --phase <phase> --spawn <n> --role <role>`. Exit 0: spawn. Exit 3: stop spawning in that phase and let running agents finish. Over a concurrency cap, wait for a slot. Otherwise decide per WORKFLOW §2.7 (end `loops run end --outcome abandoned` and say where it stopped), or at that phase's checkpoint ask the user: raise the limit / continue past it once / stop the run.
3. `loops budget show` prints usage against the limits (use it in the pause message).

## 1. Research (§2.1)

```
loops run phase research --note "4 researchers"
```
Spawn `web-researcher`, `practices-researcher`, `code-researcher` and `hardware-researcher` together. The code-researcher runs `loops lang detect --write` and lists the languages and roots in `research/code.md` `## Languages`; the practices-researcher reads each pack's `config_files`. The hardware-researcher runs only `loops hw --write` (prompt it with the header lines; it needs `Loops:`). In tier D (solo) run `loops hw --write` yourself. Prompt each with the header lines (below) and `Inputs: ticket.md, research/memory.md, project root`. Label each (see "Labelling"). Wait for completion; proceed when all four outputs exist (`research/hardware.json` included). The practices brief may list `## Project rules that supersede loops`; they win over loops rules and flow into the plan; add the binding ones to the charter's Constraints. `loops run checkpoint research`; `ask`: show the briefs' `## Open questions` and pass the answers to the planner. `auto`: the planner resolves them (assumptions go to the charter).

### Prompt header (all spawns, WORKFLOW §3)
```
Run dir: <abs $RUN>
Specs: <abs $SPECS>
Templates: <abs $TEMPLATES>
Loops: <abs path to the loops binary, or just `loops` when it is on PATH>
Role: <role>
Agent id: <e.g. W1-domain | research-web | planner | qa-W1-domain>
Wave: <n>
Language: <id> (root <dir>)   # implementers, architect and qa-verifier; one line per pack for the architect
Charter: <abs $RUN>/charter.md
```
The `Language:` line comes from the plan's `language` column; the role loads skill `loops:lang-<id>`. Followed by the agent's plan block (Goal, Starts at, Behaviour, Owns, Must not touch, Claims to prove, Done when, Out) or its input file list.

### Labelling
Immediately after each spawn, using the platform's agent id (or `solo-<n>` in solo mode; see your platform file):
```
loops run label --agent <platform agent id> --name <Agent id> --role <role> --wave <n>
```
Do this before waiting.

## 2. Plan (§2.2)

```
loops run phase plan
```
1. Spawn `planner` (strongest model). It writes `plan.v1.md` with a Concurrency block and runs `loops plan check` on it.
2. Spawn `plan-reviewer` with the plan path. It also runs `loops plan check` (findings are blocking). It writes `plan-review.r1.md` ending `Verdict: APPROVE|CHANGES`. Read only that last line and the blocking item titles.
3. `CHANGES`: for round N=1..3
```
loops run loopback --from plan-reviewer --to planner --round N --note "<one line>"
```
   Send the review path back to the **same** planner (resume or re-spawn per your tier, with its previous plan and the review); it writes `plan.v<N+1>.md`; then have the reviewer re-review the same way. After round 3 with blocking items open: at a `plan` checkpoint ask the user; otherwise rule on each item (accept or reject, with the reason; `loops run decide --topic plan-limit --note "<rulings>" --ref plan-review.r3.md`) and have the planner write the final version with the rulings.
4. **Scope check** against the charter: every AC served by a lane, no lane outside the scope. A gap is a new planner round. Then fill the charter's `## Plan` line and final ACs.
5. `loops run checkpoint plan`; `ask`: show a plan summary of at most 25 lines (ACs, waves, agents, risks) and ask: Approve / Request changes / Stop. Changes go to the planner as a new round (with `loopback`). Stop: `loops run end --outcome abandoned`.
6. Extract each agent's block and its `language` from the approved plan to paste into its prompt. The plan has a `language` per lane and an `<id>-engineer` in wave 0 for every pack a lane uses; a language without a pack is listed under its risks.

## 3. Implement (§2.3)

```
loops run phase implement --note "wave 0"
```
1. **Wave 0**: spawn `architect` (id `W0-architect`) and the plan's `<id>-engineer` lanes (ids `W0-<id>`: toolchain, manifests per the plan, lint and CI config, test and coverage prerequisites) together, wave 0, each with its `Language:` line. When they report, verify each pack's build and lint checks yourself, once: `loops gate --only <build and lint checks>` (names from `loops lang show <id>`; Go: `build,vet`). Failures go back to the owner (resume or re-spawn). Then stop wave-0 agents no later wave needs (an `<id>-engineer` may take later language-level lanes). After wave 0 and after each later wave reports: `loops run checkpoint implement`; `ask`: show the lanes and their claims and ask Continue / Stop.
2. **Concurrency cap.** Never run more agents at once than the plan's effective cap (min of `parallel.max_agents` (default 100), hardware, opt-in `limits.max_agents`). A wave over the cap runs as sub-waves (`W1a`, then `W1b` when `W1a` lanes have reported); duplicated roles with disjoint scopes (`W1-domain-orders`, `W1-domain-payments`) are separate agents with their own verifiers.
3. **Waves 1..n**: `loops run phase implement --note "wave <n>"`. Spawn the whole wave together, producers (domain, application) before consumers (adapters); consumers may start early against the stubs. Roles: `domain-engineer`, `application-engineer`, `api-engineer`, `persistence-engineer`, `integration-engineer`, `test-engineer`, `docs-writer`, and `<id>-engineer` for language-level lanes (upgrades, dependency bumps, build migrations). Every implementer gets its lane's `Language:` line. Label each.
4. Under `--models=cheap`, pass a model one tier lower than the role default for every role except planner and plan-reviewer.
5. Relay cross-agent facts (contract changes, found bugs in another lane's files) by messaging the owner (or, if it is gone, re-spawning it with the facts); never fix across lanes yourself.
6. When an implementer finishes, its claims report is at `$RUN/reports/<Agent id>.md`. If it is missing, send the agent back to write it.

## 4. QA, pipelined (§2.4)

```
loops run phase qa
```
1. As soon as an implementer reports (even while its wave-mates still run), spawn `qa-verifier` (Agent id `qa-<id>`, wave = same, the implementer's `Language:` line) with the report path, the owned paths, the contract and its ACs. Label it.
2. Read the verifier's final message. Record every claim verdict from its QA file:
```
loops run verdict --name <id>:C<n> --status VERIFIED|FAILED --round N --ref qa/<id>.r<N>.md
```
3. On any FAILED:
```
loops run loopback --from qa-verifier --to <id> --round N --note "<claims>"
```
   Send the **same** implementer (resume or re-spawn per your tier, adding its previous report, the QA file and a ≤10-line history): `Fix the FAILED claims in <abs QA file>; re-run your scope; overwrite your report.` Then have the same verifier re-verify (round N+1) the same way.
   On the second failed round of the same claim, the implementer's report must open with `## Premise` (ENGINEERING §11). Record it with `loops run event --kind note --name premise --note "<sentence>"` and include it in any escalation.
4. Limits (§4): after 2 failed rounds, relaunch the implementer one tier up (haiku -> sonnet -> opus) via a new spawn with a higher model, plus a history summary and the QA files; relabel it. After 3 failed rounds: at a `qa` checkpoint ask the user; otherwise decide per WORKFLOW §2.7 (back to plan for that lane, move the claim to Open follow-ups when no AC needs it, or end `failed`) and record it.
5. Verified lanes: stop the implementer and verifier of a lane once all of that wave's claims are VERIFIED and you do not plan to resume them.
6. **Quality pass** (WORKFLOW §2.4): when every lane is VERIFIED, spawn one `qa-verifier` with Agent id `qa-quality` (round N, every `Language:` line) to run `loops:quality` over the run's changed code. Label it. For each finding:
   - record `loops run verdict --name quality:Q<n> --status FAILED --round N --ref qa/quality.r<N>.md`;
   - record `loops run loopback --from qa-quality --to <id> --round N`;
   - send it to its lane like a failed claim. That lane's verifier re-verifies, and the pass runs again.

   Same limits.
7. When the quality pass is VERIFIED, run the full gate yourself. It runs every detected pack's checks at its root, each under the pack's race policy (WORKFLOW §6):
```
loops gate
```
   Failures: route to the owning lane (find it from the file path; `loopback --from gate --to <id>`) and repeat QA for that lane. Do not start product until `gate all` is green. Then `loops run checkpoint qa`; `ask`: show the verdicts and the quality pass and ask Continue / Stop.

## 5. Product (§2.5)

```
loops run phase product
```
Spawn `product-reviewer` with the plan, reports, QA files and a `PR:` line: `none` under `--no-pr`; `hold` when `loops run checkpoint product` prints `ask`; otherwise `open`.
- Gap found (FAILED AC): it records a loopback and returns proof paths. Run `loops run phase implement`, send the owning implementer the proof file (resume or re-spawn), then redo QA and the gate. After 2 product rounds (`MaxProductRounds`): at a `product` checkpoint ask the user; otherwise decide per WORKFLOW §2.7 (resume it with `PR: draft`, or end `failed`) and record it.
- `open`: it commits, pushes the branch `loops/<ticket>-<slug>`, opens the PR and records the `pr` event. `none`: it writes `pr.md` only.
- `hold`: show the user `pr.md` (title, summary, decisions, Open follow-ups) and ask: "Push branch loops/<ticket>-<slug> and open the PR/MR (PR + overrides comment)?" Record the answer (`--by user`). On yes, resume the same reviewer with `PR: open`.

## 6. Reflect and end (§2.6)

```
loops run phase reflect
```
Spawn `reflector`; it also reviews the run's decisions (`loops run brief --json`). Besides `reflection.md` it writes `overrides.md` and, when a PR exists and at least one project-rule override was recorded, posts it as a PR comment (never editing the description) and records the note event (WORKFLOW §2.6 step 4). When it finishes, `loops run checkpoint reflect`; `ask`: show `reflection.md` and ask End / Edit. Then:
```
loops run end --outcome done
```
`loops run end` cleans up automatically: it deletes the run's per-pack caches and tmp dirs and the build leftovers agents created (never tracked or pre-existing files; unknown new files are listed for review). Pass `--no-clean` only when the user wants to inspect the artifacts. Then stop any remaining agents and give the user a summary of at most 10 lines: outcome, ACs verified, PR link and overrides count (or "pr.md written, not pushed"), `loops report` path (`$RUN/report.html`), loop-backs count, decisions you took, memory pages written.

## Escalations and failures

- Use `loops run end --outcome failed` or `abandoned` if the user stops or the run cannot continue (abandoned and failed runs are cleaned up too); the Stop gate (WORKFLOW §6) otherwise blocks ending your turn while source files are dirty (WORKFLOW §6). `LOOPS_GATE=off` is the user's escape hatch, never yours.
- If a hook denies a read (`guard_deny`), do not retry the same call; use a range or search.
- Agents that stall: message it once with the specific question; then stop it and relaunch with a tighter block.
- Record every decision the run turns on: `loops run decide --topic <what> --note "<decision; reason>"` (WORKFLOW §2.7); other facts worth keeping: `loops run event --kind note --note "<text>"`.
- If the session context contains a `loops ... is available` update notice, tell the user once in your first message; do not block work.
