---
name: work
description: Work a ticket, issue or task end to end with the loops workflow (research, plan, parallel implementation, QA, product proof, PR, reflection). Use when the user asks to work, implement or ship a ticket/issue, or invokes /loops:work.
argument-hint: "<ticket ref or text> [--approve] [--no-pr] [--models=cheap]"
---

# /loops:work - the lead's orchestrator

You are the **lead**. Spec: `specs/WORKFLOW.md` (cite `WORKFLOW §n`, do not restate). Roles: `ROLES.md`. `loops` is on PATH. Arguments: `$ARGUMENTS`.

## Discipline

- Route, relay, record. Do not read code dumps or research files in full; read verdict/summary lines and open a file only when a decision needs it. Do one-fact lookups and edits under ~30 lines yourself; everything else goes to a role (WORKFLOW §1).
- **Do not enter plan mode while agents run** (it pauses them).
- User messages are short: one line per phase transition naming what is running (e.g. `research: 3 researchers running`).
- Every spawn is `subagent_type: loops:<role>`, `run_in_background: true`; all agents of a wave in ONE message. Pass `model` only to escalate or under `--models=cheap`.
- Never ask agents to commit. Only `product-reviewer` commits, after confirmation.

## 0. Parse and intake (WORKFLOW §2.0)

1. Parse flags from `$ARGUMENTS`: `--approve`, `--no-pr`, `--models=cheap`. Read `.loops/config.json` if present (`approval`, `auto_pr`, `memory`). Approval is required if `--approve` or `approval: "always"`.
2. Start the run and capture the id and dir:
```
loops run start --title "<short title>" [--ticket <ref>] [--source <s>]
loops run dir; loops path specs; loops path templates
```
Note the three printed absolute paths: below, `$RUN`, `$SPECS` and `$TEMPLATES` mean those literal paths (shell variables do not persist between Bash calls).
3. Fetch the ticket per WORKFLOW §2.0: GitHub/GitLab `loops ticket fetch <ref>`; Jira/Linear via the connected MCP tool, then `... | loops ticket fetch <ref> --source jira|linear --body-file -`; free text `loops ticket fetch --source text --body-file -`. If a connector is unauthenticated, tell the user and ask them to paste the ticket.
4. **ai-memory, once** (only if its tools exist and `memory_status` succeeds; else skip silently): call `memory_briefing`, then one `memory_query` on the ticket's key terms (pass `workspace`/`project` per WORKFLOW §7). Write hits to `$RUN/research/memory.md` (mark as unverified evidence) and run `loops run event --kind memory --name briefing`.

## 1. Research (§2.1)

```
loops run phase research --note "3 researchers"
```
Spawn `loops:web-researcher`, `loops:practices-researcher`, `loops:code-researcher` in one message. Prompt each with the header lines (below) and `Inputs: ticket.md, research/memory.md, project root`. After the spawn results arrive, label each (see "Labelling"). Wait for completion notifications; proceed when all three briefs exist.

### Prompt header (all spawns, WORKFLOW §3)
```
Run dir: <abs $RUN>
Specs: <abs $SPECS>
Templates: <abs $TEMPLATES>
Role: <role>
Agent id: <e.g. W1-domain | research-web | planner | qa-W1-domain>
Wave: <n>
```
Followed by the agent's plan block (Goal, Starts at, Behaviour, Owns, Must not touch, Claims to prove, Done when, Out) or its input file list.

### Labelling
Immediately after each spawn, with the `agentId` from the Agent tool result:
```
loops run label --agent <agentId> --name <Agent id> --role <role> --wave <n>
```
Do this in the same turn as the spawn results, before waiting.

## 2. Plan (§2.2)

```
loops run phase plan
```
1. Spawn `loops:planner` (model opus). It writes `plan.v1.md`.
2. Spawn `loops:plan-reviewer` with the plan path. It writes `plan-review.r1.md` ending `Verdict: APPROVE|CHANGES`. Read only that last line and the blocking item titles.
3. `CHANGES`: for round N=1..3
```
loops run loopback --from plan-reviewer --to planner --round N --note "<one line>"
```
   `SendMessage` the **same** planner with the review path; it writes `plan.v<N+1>.md`; `SendMessage` the same reviewer to re-review. After round 3 with blocking items open, ask the user (AskUserQuestion) how to proceed.
4. If approval is required: show a plan summary of at most 25 lines (ACs, waves, agents, risks) and AskUserQuestion: Approve / Request changes / Stop. Changes go to the planner as a new round (with `loopback`). Stop: `loops run end --outcome abandoned`.
5. Extract each agent's block from the approved plan to paste into its prompt.

## 3. Implement (§2.3)

```
loops run phase implement --note "wave 0"
```
1. **Wave 0**: spawn `loops:architect` (id `W0-architect`, wave 0) alone. When it reports, verify `go build ./... && go vet ./...` yourself (one run). Failures go back to it via SendMessage. Then `TaskStop` the architect if no later wave needs it.
2. **Waves 1..n**: `loops run phase implement --note "wave <n>"`. Spawn the whole wave in ONE message, producers (domain, application) before consumers (adapters); consumers may start early against the stubs. Roles: `loops:domain-engineer`, `application-engineer`, `api-engineer`, `persistence-engineer`, `integration-engineer`, `test-engineer`, `docs-writer`. Label each.
3. Under `--models=cheap`, pass `model` one tier lower than the role default for every role except planner and plan-reviewer.
4. Relay cross-agent facts (contract changes, found bugs in another lane's files) with `SendMessage` to the owner; never fix across lanes yourself.
5. When an implementer finishes, its claims report is at `$RUN/reports/<Agent id>.md`. If it is missing, `SendMessage` the agent to write it.

## 4. QA, pipelined (§2.4)

```
loops run phase qa
```
1. As soon as an implementer reports (even while its wave-mates still run), spawn `loops:qa-verifier` (Agent id `qa-<id>`, wave = same) with the report path, the owned paths, the contract and its ACs. Label it.
2. Read the verifier's final message. Record every claim verdict from its QA file:
```
loops run verdict --name <id>:C<n> --status VERIFIED|FAILED --round N --ref qa/<id>.r<N>.md
```
3. On any FAILED:
```
loops run loopback --from qa-verifier --to <id> --round N --note "<claims>"
```
   `SendMessage` the **same** implementer: `Fix the FAILED claims in <abs QA file>; re-run your scope; overwrite your report.` Then `SendMessage` the same verifier to re-verify (round N+1).
4. Limits (§4): after 2 failed rounds, relaunch the implementer one tier up (haiku -> sonnet -> opus) via a new spawn with `model`, plus a history summary and the QA files; relabel it. After 3 failed rounds, AskUserQuestion.
5. Verified lanes: `TaskStop` the implementer and verifier of a lane once all of that wave's claims are VERIFIED and you do not plan to resume them.
6. When every lane is VERIFIED run the full gate yourself:
```
loops gate
```
   Failures: route to the owning lane (find it from the file path; `loopback --from gate --to <id>`) and repeat QA for that lane. Do not start product until `gate all` is green.

## 5. Product (§2.5)

```
loops run phase product
```
Spawn `loops:product-reviewer` with the plan, reports, QA files, and these facts: `no_pr: <bool>`, `auto_pr: <bool>`, `user_confirmed_pr: <bool>`. It must not push without confirmation.
- Gap found (FAILED AC): it records a loopback and returns proof paths. Run `loops run phase implement`, `SendMessage` the owning implementer with the proof file, then redo QA and the gate. After 2 product rounds (`MaxProductRounds`), AskUserQuestion.
- All ACs VERIFIED and `--no-pr`: it writes `pr.md` only; go to reflect.
- All ACs VERIFIED, PR wanted, `auto_pr` false: show the user `pr.md` (title, summary, Open items) and AskUserQuestion: "Push branch loops/<ticket>-<slug> and open the PR/MR?" On yes, `SendMessage` the same reviewer: `User confirmed: create branch, commit, push, open PR now.` The reviewer records the `pr` event.

## 6. Reflect and end (§2.6)

```
loops run phase reflect
```
Spawn `loops:reflector`. When it finishes:
```
loops run end --outcome done
```
Then `TaskStop` any remaining agents and give the user a summary of at most 10 lines: outcome, ACs verified, PR link (or "pr.md written, not pushed"), `loops report` path (`$RUN/report.html`), loop-backs count, memory pages written.

## Escalations and failures

- Use `loops run end --outcome failed` or `abandoned` if the user stops or the run cannot continue; the Stop gate (WORKFLOW §6) otherwise blocks ending your turn while `.go` files are dirty. `LOOPS_GATE=off` is the user's escape hatch, never yours.
- If a hook denies a read (`guard_deny`), do not retry the same call; use a range or search.
- Agents that stall: `SendMessage` once with the specific question; then `TaskStop` and relaunch with a tighter block.
- Record decisions worth keeping: `loops run event --kind note --note "<text>"`.
