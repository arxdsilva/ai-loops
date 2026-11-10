# <Language>: backend practices and loops gap analysis

| | |
|---|---|
| Pack id | `<id>` |
| Current stable / LTS | `<x.y>` / `<x.y or none>` (support window: …) |
| Dominant runtimes / frameworks | … |
| As of | 2026-11 |
| Researcher | `<agent label>` |
| Verifier | (filled in by B2) |

> **Summary:** five lines at most. How a strong backend team in this language works, and the three biggest gaps between that and loops today.

Rules follow [TAXONOMY.md](TAXONOMY.md), using the row format `ID | Rule | Why | Sev | Source | loops today | Enforce`. Gaps are judged against [LOOPS_BASELINE.md](LOOPS_BASELINE.md).

## 1. REPO: repository and project management

Give a canonical tree for a service, as a code block, before the rules.

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|
| `<LANG>-REPO-01` | … [REPO-01] | … | MUST | [..](..) | missing | review |

## 2. TOOL: toolchain

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|

## 3. ARCH: architecture

Include the ARCH-02 directory tree mapping the loops layers onto this language and its main framework.

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|

## 4. IDIOM: what to do and what to avoid

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|

## 5. CONC: concurrency and the `-race` equivalent

State the **loops race policy** in one line before the rules: `mode = flag|build|none`, plus the exact argv or the substitute check.

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|

## 6. TEST: testing

State the **coverage recipe** in one line before the rules: the tool, the format, the exact argv, and how coverage of new code is obtained.

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|

## 7. PERF: performance and profiling

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|

## 8. SEC: security

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|

## 9. DATA: persistence

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|

## 10. API: APIs and integration

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|

## 11. OBS: observability

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|

## 12. OPS: configuration, packaging and deployment

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|

## 13. DOC: documentation and review

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|

## 14. FW: frameworks

| ID | Rule | Why | Sev | Source | loops today | Enforce |
|---|---|---|---|---|---|---|

## Top 10 footguns

1. **<name>.** Bad: `…`. Fix: `…`. ([source](..))

## 15. LOOPS: tooling data

```json
{ "id": "<id>", "...": "the schema from TAXONOMY.md §15" }
```

## Gaps vs loops

Every rule that isn't `covered`, most important first.

| Rule ID | Gap | loops today | Priority | Where loops should fix it (spec / skill / role / gate / hook / arch) |
|---|---|---|---|---|

## Open questions

Things the sources didn't settle, or decisions for the loops maintainers.

## Verification

Filled in by the B2 verifier.

- Verdict: `pass` or `fail`, round N.
- Completeness: sections present, rule counts per section, whether the LOOPS block parses.
- Sources opened (≥8): `URL - supports the claim? yes/no - note`.
- Tooling truth: `argv - doc URL - confirmed?`
- Spot run (optional): image, commands, exit codes.
- Findings: numbered.
