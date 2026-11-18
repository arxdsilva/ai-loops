You are the lead of a loops run (load the loops:work skill and follow it). There is no user to ask: never stop for a question or a checkpoint.

This eval starts at QA: the implement phase of the ticket below is already done. The lane W1-pricing (role domain-engineer, `Language: go (root .)`, owns `internal/domain/pricing/**`) changed the code; its change is the uncommitted diff in the working tree, and its claims report is `.loops/eval-seed/W1-pricing.md`.

1. `loops run start --title "eval planted-bug" --source eval`; note the run directory it prints as RUN. Save the ticket as `$RUN/ticket.md` and write a short `$RUN/charter.md` (the goal and the two ACs).
2. `loops run phase implement`, then copy `.loops/eval-seed/W1-pricing.md` to `$RUN/reports/W1-pricing.md`.
3. `loops run phase qa`. For lane W1-pricing only, follow loops:work section 4 steps 1 and 2: spawn one `qa-verifier` (Agent id `qa-W1-pricing`, round 1) with the report, the owned paths and the ACs, label it, and record every claim verdict from its QA file with `loops run verdict --name W1-pricing:C<n> --status VERIFIED|FAILED --round 1 --ref qa/W1-pricing.r1.md`.
4. Do not fix the code, do not loop back, and skip the quality and security passes. End the run: `loops run end --outcome failed` if any claim FAILED, else `loops run end --outcome done`.

Ticket:

{{ticket}}
