---
name: documentation
description: Use when writing or updating package AGENTS.md files, interface summaries, README sections or, rarely, ADRs in a backend repo in any language pack, without duplicating canonical specs or recording reasoning transcripts.
---

# Agent documentation

Which document owns what: the specs are canonical; everything else cites them by section (`specs/CONTEXT_DISCIPLINE.md §13, §22`). Never copy long canonical sections into another file.

## Package `AGENTS.md`

Follow `templates/PACKAGE_AGENTS.md`: purpose, owned concepts, allowed and forbidden dependencies, contracts, invariants, hot paths, tests. Target 300-800 words, only what is local to the package. Update it in the same change that alters a contract, an invariant or a dependency. Omit empty sections.

## Interface summary

For a cross-agent dependency, write a compact signature and invariant summary in the plan's contract section, not whole packages.

## README sections

Only facts a user or operator needs: how to run, configure, migrate. Written by `docs-writer` from facts the lead supplies; never invent behaviour.

## ADR (rare)

Write one only for a decision that changes a contract: a port, the persisted or wire schema, the concurrency model, the command set, the consistency model. Use `templates/ADR.md`; save under `docs/adr/NNNN-<slug>.md`. Decisions, not tutorials. A system doc only for a subsystem no spec or package `AGENTS.md` already describes.

## Style

- Tables, short invariants, signatures and decision bullets over narrative.
- Strong headings and explicit paths for retrieval.
- Decisions and results only; never a reasoning transcript (`specs/CONTEXT_DISCIPLINE.md §18`).
- One home per fact; cite the rest.
