# Plan review template

Written by the `plan-reviewer` as `plan-review.r<N>.md`. Blocking findings (B) must be fixed before approval; non-blocking (N) never block. The last line is the verdict.

```markdown
# Plan review r<N> of plan.v<N>.md

## Checklist
- [ ] Every AC is observable and testable; derived ACs are marked
- [ ] Task packet complete, including consistency and compatibility impact
- [ ] Contract (wave 0) lists ports, commands/DTOs, stubs, deps, shared config
- [ ] Layering respects specs/ARCHITECTURE.md (inward deps, consumer-owned ports, wiring-only composition root)
- [ ] Waves table has every column, including each lane's language (one pack per lane); each agent has Goal/Starts at/Behaviour/Owns/Must not touch/Claims/Done when/Out
- [ ] File ownership is disjoint across agents; manifests, lockfiles and shared config belong to the architect and the <id>-engineer only
- [ ] Models are the cheapest competent tier, each with a reason (ROLES.md)
- [ ] Every claim is checkable with named evidence; every AC maps to claims
- [ ] QA pairing covers every implementer
- [ ] Proof plan covers every AC and its edge cases
- [ ] Consistency: ordering, idempotency, clock/RNG, migrations considered
- [ ] Compatibility: API, persisted schema, config, migrations considered
- [ ] Performance evidence planned for any hot path
- [ ] Risks and open questions are real and owned

## Blocking
B1. what: <problem, with plan section>
    why: <consequence if unfixed>
    suggested change: <concrete edit>

## Non-blocking
N1. what / why / suggested change

Verdict: APPROVE|CHANGES
```
