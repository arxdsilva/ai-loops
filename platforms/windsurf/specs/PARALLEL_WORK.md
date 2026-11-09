# Parallel work

How a loops plan fans out into role agents without collisions or waste. Extracted from rimworld-go and rewritten for loops roles (`ROLES.md`). The run phases are in `specs/WORKFLOW.md §2`; the lead's checklist is `skills/parallel-work`.

A single-file or single-area change under ~150 lines is still a wave, with one agent; the plan says so in one line.

## 1. The plan carries its waves

A plan (`templates/PLAN.md`) is incomplete without:

- **Contract (wave 0):** the ports and consumer-owned interfaces, typed commands and DTOs, stubs, dependencies and shared config the `architect` writes before anyone else starts.
- **Waves in dependency order**, one table per wave:

  `| id | role | model - reason | job | owns | must not touch | tests | claims |`

- **A block per agent** under each table: Goal (one sentence), Starts at (`file:line`), Behaviour (numbered rules including edge cases), Owns, Must not touch, Claims to prove (C1...), Done when (the tests that prove it), Out (what looks in scope and is not, and who owns it).
- **QA pairing** (which `qa-verifier` checks which implementer) and the **proof plan** per AC.

Detail level: enough that a stranger follows the plan and an agent works from its block alone. Point at `file:line` instead of pasting code, never restate a spec, say each thing once. `plan-reviewer` rejects a plan missing any of the above (`templates/PLAN_REVIEW.md`).

## 2. The contract goes first

Wave 0 is a single `architect` agent (opus). Nobody else starts until `go build ./...` and `go vet ./...` pass.

| Do | Don't |
| --- | --- |
| Write ports, typed commands, DTOs, read models; additive changes only | Let implementers invent interfaces other agents depend on |
| Stub every new hook so every agent compiles from the first minute | Make a consumer wait for a producer to compile |
| Run `go get` / `go mod tidy`; edit Makefile, CI, `cmd/**`, `internal/compose/**` | Let anyone else touch `go.mod`, `go.sum` or shared config |

A contract change after wave 0 is additive, made by the architect, and relayed to every affected agent by the lead.

## 3. Disjoint ownership

| Do | Don't |
| --- | --- |
| Give every agent explicit owned globs, forbidden globs, the spec sections it follows, targeted verification commands and a word-capped report | Launch an agent with "fix whatever you find" |
| Split along the hexagon: one agent per layer or package area (`ROLES.md` default scopes, narrowed in the plan) | Give two agents the same file |
| When ownership cannot be disjoint, split the file, give it to one agent, or sequence the work | Hope concurrent edits to one file work out |

## 4. Inside an agent

The shared implementer rules are in `ROLES.md` (TDD, targeted checks, ownership, no git writes, absolute paths, claims report). Also:

- Tests run with `GOFLAGS=-p=2` and `-race`: `go test -race -count=1 -cover ./<owned>/...`.
- Never run `go test ./...`, `make all` or rebuild shared binaries; the lead and QA run the full gates.
- In zsh, split a variable with `${=files}` or `xargs`.
- Report failures in other agents' files; never fix them.

## 5. Phase order

contract (architect) -> producers in parallel (domain, application) -> consumers (inbound and outbound adapters, which may start early against stubs) -> tests/docs agents -> QA, pipelined per implementer -> gates -> product review.

Machine limits: do not launch more agents than the machine can run. Compiling many Go test binaries at once crawls; stagger heavy starts, keep `GOFLAGS=-p=2`, and re-check if an agent reports slow tests or the machine swaps.

## 6. Coordination and gates

| Do | Don't |
| --- | --- |
| Relay cross-agent facts through the lead (`SendMessage`) | Let agents coordinate implicitly through files |
| Route fixes to the owning agent | Patch an active agent's files behind its back |
| Declare done from QA verdicts plus green gates | Declare done from implementer reports alone |

Gates after each wave are `loops gate` (`specs/WORKFLOW.md §6`); gate failures go back to the owning lane (`specs/WORKFLOW.md §2.4`). Do not enter plan mode while agents run: it pauses them. If you must, resume each with `SendMessage` afterwards.

## 7. Agent lifecycle: reuse, then clean up

Every spawned agent stays in the agent list for the session; keep the count low at the source.

| Do | Don't |
| --- | --- |
| Send fixes, QA rejections and the same area's next wave to the agent that owns it (`SendMessage`, which resumes it with context; WORKFLOW "Send back with context") | Spawn a fresh agent for a fix in an area an existing agent owns |
| Do one-fact lookups, small edits and single test runs yourself | Spawn an agent for under ~30 lines or one search |
| While agents share the checkout, commit with `git commit -m ... -- <paths>` (only `product-reviewer` commits, after the lead confirms) | `git add ... && git commit`: it also commits what other agents staged |
| `TaskStop` each agent once its work is verified and committed and you will not resume it | Leave finished agents open "just in case" |
| Split a worker before about 200 API calls: stop it and hand the rest to a fresh agent with a short summary (every call re-reads the whole context) | Keep one worker going for a whole programme |

Stopped agents stay listed but are not running; messaging one resumes it. When the list grows past about 50 and the work is committed, tell the user that a new session starts with a clean list.

## 8. Pick the cheapest competent model

Tiers and defaults are in `ROLES.md` (haiku, sonnet, opus). Rules:

- Start at the lowest tier that can do the job; never go higher "to be safe".
- State the model and a one-line reason for every agent in the plan table.
- Escalate one tier after two failed QA rounds (`specs/WORKFLOW.md §4`), relaunching with a summary of the history.
- Untyped `general-purpose` spawns without a `model` are denied by the model guard (`specs/CONTEXT_DISCIPLINE.md §11`).
