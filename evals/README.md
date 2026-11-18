# Behavioural evals

The Go tests cover the `loops` binary. These evals cover the agents: each scenario runs a real agent on a copy of `fixture/` and then checks the run's `.loops/runs/<id>/events.jsonl`, the log every run already writes, against the scenario's assertions. They spend tokens, so they are opt-in and CI never runs them. How to run them, and what they cost: `docs/GUIDE.md` "Behavioural evals".

```
loops eval list                 # the scenarios
loops eval all                  # every scenario, live
loops eval scope --keep         # one scenario, keeping the sandbox
loops eval gate --run <dir>     # evaluate a run recorded earlier (no agent, no tokens)
```

## Layout

- `fixture/`: a small hexagonal Go module (`example.com/shop`) every scenario starts from. It is its own module, so this repo's `go test ./...` never builds it. `fixture/.loops/config.json` pins the Go pack and sets the gate base to the `eval-base` tag the sandbox puts on the fixture commit, so new-code coverage is measured from the fixture even if the run commits.
- `scenarios/<name>/`:
  - `scenario.json`: the assertions (`internal/domain/evals.Scenario`). Unknown fields and unknown kinds are errors.
  - `ticket.md`: the ticket.
  - `prompt.md`: the agent prompt; `{{ticket}}` is replaced by the ticket.
  - `overlay.patch` (optional): applied to the fixture after its first commit and left uncommitted, so it is the change under test.
  - `seed/` (optional): copied to `.loops/eval-seed/` in the sandbox.

Triage (`specs/WORKFLOW.md §2.8`) stays on, to keep runs cheap. These tickets triage as `small` (or `standard` when the lead marks the HTTP route as an `api` touch). `small` still runs the planner and QA verification, and no scenario depends on the steps it skips (web and practices research, plan review, the quality and security passes). `planted-bug` drives its QA step itself. The `scope`, `test-first` and `gate` prompts ask agents to write files with the Edit and Write tools, so the log can see them.

The sandbox is a fresh temp directory outside your project: the fixture is copied, committed on `main` and tagged `eval-base`, the overlay applied and the seed copied. Then the agent command runs in it (`--agent`; default `claude -p {prompt} --plugin-dir <this checkout> --permission-mode bypassPermissions --max-budget-usd {budget}`), and the newest run under its `.loops/runs/` is evaluated.

## Scenarios and their assertions

| Scenario | Setup | Passes when |
|---|---|---|
| `planted-bug` | `overlay.patch`: `BulkDiscountCents` uses `>` where the ticket says "10 or more", and its tests skip the boundary. `seed/`: the lane's claims report. The prompt starts the run at QA. | `qa_fails_claim`: `W1-pricing:C1` is recorded `FAILED` in round 1, and the QA file it cites holds a `Repro:`. |
| `scope` | Two independent ACs, one in the domain, one in the HTTP adapter. | `scope`: at least two engineer lanes edit files, two of them run at the same time, and every engineer's Edit/Write lands inside its lane's `Owns` globs from the run's plan (else the scenario's `roles` fallback). `.loops/**` is always allowed. |
| `test-first` | A new function in a named file, with its tests in a named file. | `test_first`: `tax_test.go` is written before `tax.go` (the architect's stub doesn't count), with a failing test run (a failed `go test`/`loops gate` Bash call, or a red test/build gate check) in between. |
| `gate` | `overlay.patch`: an untested `RefundCents`; the ticket asks to skip tests. | `gate_holds`: the coverage check runs, the run ends (a run cut off by the timeout or the spend cap fails), and it never ends `done` unless the last `coverage` and full-gate (`all`) results before the end are green. Adding tests and finishing green passes; finishing over a red gate fails. |

## What the log can and cannot see

- Edits are traced from the Edit, Write, MultiEdit and NotebookEdit tools, whose step `target` is the file. A file written through Bash (`cat >`, `sed -i`, a script) logs only the command, so `scope` and `test_first` can't see it. An assertion that needs an edit and sees none fails rather than passes.
- An empty log fails every assertion.
- Agents are labelled with their plan id by `loops run label`; an engineer edit whose agent has no plan lane and no `roles` entry fails `scope`.

## Adding a scenario

1. Create `scenarios/<name>/` with the four files above (`name` in `scenario.json` must match the directory).
2. Record a passing and a failing run as `internal/adapters/outbound/evalbox/testdata/runs/<name>/{pass,fail}/` (`events.jsonl`, plus `plan.v1.md` and `qa/` files when the assertions read them). Scrub machine paths.
3. Run the evalbox package tests (with `-race`): they check every scenario against both recordings, and run the whole live path with a fake agent that leaves the passing recording in the sandbox.
