---
name: parallel-work
description: Use when the lead plans or runs a loops wave of role agents, or when asked to work in parallel - contract first, disjoint ownership, spawn and label lanes, relay, gates, lifecycle and model tiers.
---

# Parallel work (lead checklist)

Rules: `specs/PARALLEL_WORK.md`. Run phases: `specs/WORKFLOW.md §2-4`. Roles and tiers: `ROLES.md`. This skill is the checklist; a one-file fix is still a wave with one agent.

1. **Plan has its waves.** The plan (`templates/PLAN.md`) carries the contract, a waves table per wave and a block per agent (`specs/PARALLEL_WORK.md §1`). `plan-reviewer` rejects a plan without them.
2. **Map areas to roles.** One role agent per layer or package area (`ROLES.md`). One role may run as several agents when their globs are disjoint (`W1-domain-orders`, `W1-domain-payments`), each with its own qa-verifier. Size waves to the effective cap = min(`parallel.max_agents`, default 100; `loops hw` recommendation; opt-in `limits.max_agents`); split larger waves into sub-waves (`W1a`, `W1b`) (`specs/WORKFLOW.md §2.2`). Every lane carries a `language` (a detected pack); test parallelism per agent comes from the pack (`env_hints`, `test_jobs`; Go: `GOFLAGS=-p=2`). Check the plan with `loops plan check`.
3. **Pick each model** (`ROLES.md` tiers): haiku for mechanical work, sonnet for implementation against a contract, opus for contracts, consistency, concurrency and migrations. State the reason in the table. Escalate one tier after two failed QA rounds.
4. **Contract first (wave 0).** The architect writes ports, typed commands, DTOs, stubs and shared config; one `<id>-engineer` per pack a lane uses sets up the toolchain and test prerequisites (`loops:lang-<id>`). Manifests and lockfiles have one owner. Wave 0 is done when each pack's build and lint checks pass (`loops gate --only <build,lint checks>`; Go: `build,vet`).
5. **Launch a wave in one message**, in the background: producers (domain, application) first; consumers (adapters) may start against stubs. Use `subagent_type: loops:<role>`, with the agent's plan block and these prompt lines (`specs/WORKFLOW.md §3`):

```text
Run dir: <abs>   Specs: <abs: loops path specs>   Role: <role>   Agent id: <W1-domain>   Wave: <n>
Language: <id> (root <dir>)
```

6. **Label each lane right after spawn:**

```bash
loops run label --agent <agentId from the Agent result> --name W1-domain --role domain-engineer --wave 1
```

7. **Pipeline QA.** Launch a `qa-verifier` as soon as its implementer reports. Record verdicts and loop-backs with `loops run verdict` and `loops run loopback`.
8. **Relay** cross-agent facts through the lead with `SendMessage`; broadcast additive contract changes to every affected agent. Do not enter plan mode while agents run.
9. **Integrate and verify.** After each wave run `loops gate`, which checks every detected pack (`specs/WORKFLOW.md §6`); route failures to the owning agent, resumed with `SendMessage`.
10. **Lifecycle** (`specs/PARALLEL_WORK.md §7`). Reuse the owning agent for fixes and the next wave. Do one-fact lookups and edits under ~30 lines yourself. Commit with `git commit -m ... -- <paths>` (only `product-reviewer` commits, after you confirm). `TaskStop` agents you will not resume. Split a worker before about 200 API calls.

## Agent prompt skeleton

```text
Model: <haiku|sonnet|opus> - <one-line reason>
You are agent <id> (<role>, language <id>) on <repo>. Load loops:lang-<id>. Read first: <plan file §>, AGENTS.md, nearest package AGENTS.md, <spec sections>.
Contract: <the plan's contract section>
Your job: <the agent's block from the plan>
Rules: owns ONLY <globs>; must NOT touch <globs>, manifests, lockfiles, shared config; no git writes; strict TDD;
  targeted checks only (loops gate --only <check>,... --pkgs <owned>; the pack's race policy).
Final report: templates/CLAIMS_REPORT.md (<=400 words).
```
