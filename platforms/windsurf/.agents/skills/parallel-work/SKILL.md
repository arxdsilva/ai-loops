---
name: parallel-work
description: Use when the lead plans or runs a loops wave of role agents, or when asked to work in parallel - contract first, disjoint ownership, spawn and label lanes, relay, gates, lifecycle and model tiers.
---

# Parallel work (lead checklist)

Rules: `specs/PARALLEL_WORK.md`. Run phases: `specs/WORKFLOW.md §2-4`. Roles and tiers: `ROLES.md`. This skill is the checklist; a one-file fix is still a wave with one agent.

1. **Plan has its waves.** The plan (`templates/PLAN.md`) carries the contract, a waves table per wave and a block per agent (`specs/PARALLEL_WORK.md §1`). `plan-reviewer` rejects a plan without them.
2. **Map areas to roles.** One role agent per layer or package area (`ROLES.md`). Only as many agents as there are disjoint areas. Do not exceed what the machine can run: stagger heavy starts and give every agent `GOFLAGS=-p=2`.
3. **Pick each model** (`ROLES.md` tiers): haiku for mechanical work, sonnet for implementation against a contract, opus for contracts, consistency, concurrency and migrations. State the reason in the table. Escalate one tier after two failed QA rounds.
4. **Contract first (architect alone).** Ports, typed commands, DTOs, stubs, `go get`, shared config. Wave 0 is done when `go build ./...` and `go vet ./...` pass.
5. **Launch a wave in one message**, in the background: producers (domain, application) first; consumers (adapters) may start against stubs. Use `subagent_type: loops:<role>`, with the agent's plan block and these prompt lines (`specs/WORKFLOW.md §3`):

```text
Run dir: <abs>   Specs: <abs: loops path specs>   Role: <role>   Agent id: <W1-domain>   Wave: <n>
```

6. **Label each lane right after spawn:**

```bash
loops run label --agent <agentId from the Agent result> --name W1-domain --role domain-engineer --wave 1
```

7. **Pipeline QA.** Launch a `qa-verifier` as soon as its implementer reports. Record verdicts and loop-backs with `loops run verdict` and `loops run loopback`.
8. **Relay** cross-agent facts through the lead with `SendMessage`; broadcast additive contract changes to every affected agent. Do not enter plan mode while agents run.
9. **Integrate and verify.** After each wave run `loops gate` (`specs/WORKFLOW.md §6`); route failures to the owning agent, resumed with `SendMessage`.
10. **Lifecycle** (`specs/PARALLEL_WORK.md §7`). Reuse the owning agent for fixes and the next wave. Do one-fact lookups and edits under ~30 lines yourself. Commit with `git commit -m ... -- <paths>` (only `product-reviewer` commits, after you confirm). `TaskStop` agents you will not resume. Split a worker before about 200 API calls.

## Agent prompt skeleton

```text
Model: <haiku|sonnet|opus> - <one-line reason>
You are agent <id> (<role>) on <repo>. Read first: <plan file §>, AGENTS.md, nearest package AGENTS.md, <spec sections>.
Contract: <the plan's contract section>
Your job: <the agent's block from the plan>
Rules: owns ONLY <globs>; must NOT touch <globs>, go.mod/go.sum, shared config; no git writes; strict TDD;
  targeted checks only (GOFLAGS=-p=2 go test -race -count=1 -cover ./<owned>/...).
Final report: templates/CLAIMS_REPORT.md (<=400 words).
```
