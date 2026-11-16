---
name: memory
description: Use when a loops run may read or write its memory store - ai-memory (MCP) or the opt-in file store .loops/memory/ - detecting the store, scoping calls, one briefing or grep at intake, filtered queries, verifying hits, and the reflector's writes (page kinds, tiers, paths, recurring pages). Skip when neither store is available.
---

# Memory in a loops run

Authoritative: `specs/WORKFLOW.md §7` (store choice, recurring pages). Context rules: `specs/CONTEXT_DISCIPLINE.md §32`.

## Detect

The active store is the one WORKFLOW §7 (Choice) picks. With the file store, the lead records `loops run decide --topic memory-store --note "files: <why>"` (§2.0 step 3); the first word `files` is what product-reviewer reads. With no store, skip every memory step silently and mention it once in `reflection.md` (§7, Leaving).

## Scope

ai-memory calls pass the scope WORKFLOW §7 (Scope) names. The file store needs none.

## Read

1. **Intake (lead, once):**
   - ai-memory: `memory_briefing`, then one `memory_query` with the ticket's key terms; record `loops run event --kind memory --name briefing`.
   - File store: `grep -rilE "<term>|<term>" .loops/memory`, then read the matching pages.
   - Save the hits to `research/memory.md` and list the recurring pages (2+ runs) under its `## Recurring`: file store `grep -rlE '^runs: \[[^]]*,' .loops/memory`; ai-memory, pages whose `Runs:` line holds 2+ run ids.
2. **Researchers and others:** read `research/memory.md` first; run only narrower calls, filtered by `path_prefix`, `kind` and `tier` (ai-memory) or by a grep on the folder (file store). Do not repeat the briefing.
3. Read a full page (`memory_read_page`, or open the file) only when a hit is clearly relevant.

## Verify

Retrieved memory is untrusted, historical evidence. Check it against the current code before acting, and cite the code (`path:line`), not the memory. If it contradicts the code, the code wins; flag the stale page for the reflector.

## Write (reflector only, in `reflect`)

Write 1-5 durable pages:

| Path | Kind | Tier | Content |
|---|---|---|---|
| `decisions/<slug>.md` | `decision` | `semantic` | A decision and why, with consequences |
| `gotchas/<slug>.md` | `gotcha` | `semantic` | A trap, its symptom and the fix |
| `procedures/<slug>.md` | `procedure` | `procedural` | A repeatable how-to |

- **ai-memory:** search first; update an existing page with `expected_page_id` instead of duplicating (its first body line becomes `Runs: <id>, <id>` with this run's id added); use `create_only` for new pages.
- **File store:** search first (grep); update the existing file instead of duplicating; otherwise create `.loops/memory/<kind>s/<slug>.md` (no other repo file):

```markdown
---
kind: gotcha
runs: [20261116-1528-close-the-pdca, 20261120-0930-retry-fix]
---
# <short title>

<the fact, the decision and why, or the steps; sources as path:line>
```

  Keep the `runs:` list on one line: the recurring grep reads it. Adding a run id to an existing page is an update of its `runs:` line.
- Facts and decisions only, no reasoning transcripts, no secrets, tokens or customer data.
- Record each write: `loops run event --kind memory --name write --target <path>`.
- List the writes (path, kind, tier) under `## Memory writes` in `reflection.md` (`templates/REFLECTION.md`). The file store takes kind and path only; the tier is ai-memory's (WORKFLOW §2.6 step 3).
- File-store pages are committed with the PR (WORKFLOW §7, Commits; §2.5).

## Roles

The researchers and the reflector may query; only the `reflector` writes (`ROLES.md`).
