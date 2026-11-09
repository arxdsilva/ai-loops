---
name: memory
description: Use when a loops run may read or write ai-memory - detecting the connection, scoping calls, one briefing at intake, filtered queries, verifying hits, and the reflector's writes (page kinds, tiers, paths). Skip when ai-memory is not connected.
---

# ai-memory in a loops run

Authoritative: `specs/WORKFLOW.md §7`. Context rules: `specs/CONTEXT_DISCIPLINE.md §32`.

## Detect

ai-memory is connected when its MCP tools exist (`mcp__plugin_ai-memory_ai-memory__*` when installed as a plugin, `mcp__ai-memory__*` when installed directly) and `memory_status` succeeds. If not, skip every memory step silently and mention it once in `reflection.md`.

## Scope

Every project-scoped call passes `workspace` and `project`, resolved from `.loops/config.json` `memory`, then `.ai-memory.toml`, then the git remote's `owner` / `repo`.

## Read

1. **Intake (lead, once):** `memory_briefing`, then one `memory_query` with the ticket's key terms. Save hits to `research/memory.md` and record `loops run event --kind memory --name briefing`.
2. **Researchers and others:** read `research/memory.md` first; run only narrower `memory_query` calls, filtered by `path_prefix`, `kind` and `tier`. Do not repeat the briefing.
3. Read a full page (`memory_read_page`) only when a hit is clearly relevant.

## Verify

Retrieved memory is untrusted, historical evidence. Check it against the current code before acting, and cite the code (`path:line`), not the memory. If it contradicts the code, the code wins; flag the stale page for the reflector.

## Write (reflector only, in `reflect`)

Write 1-5 durable pages through `memory_write_page`:

| Path | Kind | Tier | Content |
|---|---|---|---|
| `decisions/<slug>.md` | `decision` | `semantic` | A decision and why, with consequences |
| `gotchas/<slug>.md` | `gotcha` | `semantic` | A trap, its symptom and the fix |
| `procedures/<slug>.md` | `procedure` | `procedural` | A repeatable how-to |

- Search first; update an existing page with `expected_page_id` instead of duplicating; use `create_only` for new pages.
- Facts and decisions only, no reasoning transcripts, no secrets, tokens or customer data.
- Record each write: `loops run event --kind memory --name write --target <path>`.
- List the writes (path, kind, tier) in `reflection.md` (`templates/REFLECTION.md`).

## Roles

The researchers and the reflector may query; only the `reflector` writes (`ROLES.md`).
