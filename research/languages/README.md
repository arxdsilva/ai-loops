# Backend-language research for loops

As of **2026-11-10**, against loops **v0.4.1**. This research asks what it would take for loops to drive backend work in 15 languages instead of Go alone, and where loops falls short today.

## Headline numbers

| Measure | Value |
|---|---|
| Languages | 15, chosen in Stage A ([RANKING.md](RANKING.md)) |
| Rule rows | 1,731: 309 `covered`, 486 `go-only`, 930 `missing`, 6 `n/a` |
| Non-covered rules (`go-only` + `missing`) | 1,416; 1,243 of them are cited in a Gaps table |
| Gaps-table rows | 296 |
| Cited rules by priority | P0 196 · P1 617 · P2 430 |
| Gap clusters | 10 P0, 12 P1 ([MATRIX.md](MATRIX.md) §2–§3) |
| Maintainer decisions before Stage C | 25 ([TOOLING.md](TOOLING.md) §7) |
| Readiness, all 15 languages | 18% |
| Readiness, excluding Go | 14% (Go counts GO_BEST rules as `covered`, MATRIX §1.2) |

**Readiness** is covered ÷ (covered + go-only + missing):

| go | typescript | swift | csharp | java | python | elixir | scala | cpp | clojure | kotlin | php | rust | erlang | ruby |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 64% | 19% | 19% | 19% | 16% | 16% | 16% | 15% | 14% | 12% | 11% | 11% | 10% | 7% | 5% |

**The five biggest gap clusters**, by how many languages raise them at P0 (MATRIX §2):
1. There is no non-Go coverage parser, so the ≥70% new-code gate can't run (14 languages).
2. There is no `race.mode = none` or `build` policy (14).
3. The gate is a Go `switch` (12, plus 2 at P1).
4. There is no vulnerability step outside Go, and `govulncheck` is silent when it isn't installed (12).
5. The run cache and cleanup are Go-only (8, plus 6 at P1).

Language detection (`init` requires `go.mod`) is raised at P0 by only 5 languages. It still has to come first: no pack can load without it.

## Files

| File | What it is |
|---|---|
| [TAXONOMY.md](TAXONOMY.md) | The 14 sections and their items, the rule-row format, statuses, priorities, minimum depth, and the draft LOOPS schema (§15) |
| [LOOPS_BASELINE.md](LOOPS_BASELINE.md) | What loops v0.4.1 covers today and where, and whether each area is neutral or Go-only. It is the reference for the "loops today" column |
| [RANKING.md](RANKING.md) | Stage A: sources, the scoring formula, and the final 15 with their pack ids |
| [_TEMPLATE.md](_TEMPLATE.md) | The skeleton every language file follows |
| `python.md` … `clojure.md` (15 files) | One per language: rules by section, Top 10 footguns, the LOOPS json block, Gaps vs loops, Open questions, Verification |
| [MATRIX.md](MATRIX.md) | B3: the script-computed status grid, readiness, the P0 and P1 gap clusters with rule IDs and loops files, the top 5 gaps per language, and a Gaps-table completeness check |
| [GENERIC.md](GENERIC.md) | B3: practices that at least 12 languages agree on, the proposed neutral Stage C specs and their outlines, an audit of the Go specs (universal vs. Go-only), and the generalised hard test gate with a proposed race-rule wording |
| [TOOLING.md](TOOLING.md) | B3: the merged per-language tooling table, the final pack schema (an annotated Go pack plus snippets, a field table, and what was dropped), coverage formats and their quirks, the race-policy breakdown and enforcement, run-scoped caches, arch scanning, and the 25 decisions |

Language files: [python](python.md) · [typescript](typescript.md) · [java](java.md) · [cpp](cpp.md) · [csharp](csharp.md) · [php](php.md) · [go](go.md) · [ruby](ruby.md) · [swift](swift.md) · [kotlin](kotlin.md) · [rust](rust.md) · [scala](scala.md) · [elixir](elixir.md) · [erlang](erlang.md) · [clojure](clojure.md)

## Method

1. **Stage A: ranking** ([RANKING.md](RANKING.md)).
   - Score = `0.4 × Stack Overflow 2026 + 0.3 × TIOBE Nov 2026 + 0.3 × RedMonk Jan 2026`, then a backend filter.
   - Positions 1–12 are clear from the data.
   - At Checkpoint 1 the maintainer swapped Perl and Haskell for Elixir, Erlang and Clojure, and kept Swift.
2. **B1: research.** One researcher per language filled in the template against TAXONOMY. Each rule cites a primary source with a version or date and is judged against LOOPS_BASELINE. The researcher also wrote:
   - a LOOPS json block in the draft schema;
   - a Gaps table;
   - Open questions.
3. **B2: verification.** Each verifier recorded its work in the file's Verification section, in up to 2 rounds (WORKFLOW §4 caps them):
   - a structure lint (`lint.py`) and a link check (`links.sh`);
   - at least 8 sources opened;
   - every argv checked against its docs;
   - an optional container **spot run** of the core commands, with exit codes.
   - **Outcome:** 7 files passed in round 2 (cpp, csharp, go, swift, scala, elixir, clojure). 8 were finished by **lead-applied fixes** after round 2 (python, typescript, java, php, ruby, kotlin, rust, erlang); each says so in "Lead fix after round 2".
4. **B3: synthesis** (this README, MATRIX, GENERIC, TOOLING).
   - Counts come from scripts that parse the rule rows and the LOOPS blocks. The scripts live in [scripts/](scripts/): `matrix.py`, `universal.py`, `packs.py`, plus `lint.py` and `links.sh` from B2.
   - Clusters, schema and decisions are reconciled across all 15 files.
   - Nothing was invented: every fact cites a rule ID, a spot run or a loops file.

## How to read a language file

1. **Header and summary.** Versions, runtimes and frameworks, and the three biggest gaps.
2. **§1–§14, the rule tables.** Each row is `ID | Rule | Why | Sev | Source | loops today | Enforce`.
   - The bracket at the end of a Rule (`[TEST-06]`) is the taxonomy item it answers.
   - **Sev:** `MUST` is a defect when broken, `SHOULD` is the default, `AVOID` is an anti-pattern and names the alternative.
   - **loops today:** `covered`, `go-only`, `missing` or `n/a`. Go is the exception: its file counts rules stated by its own Go specs as `covered`.
   - **Enforce:** the cheapest mechanical home, one of `gate`, `hook`, `arch` or `review`.
3. **Fixed lines.** §5 opens with the **race policy line** (mode and argv or substitute), and §6 with the **coverage recipe**. These two lines feed the hard gate.
4. **Top 10 footguns.** Each gives the bad pattern, the fix and a source.
5. **§15 LOOPS json.** The machine-readable pack draft. The notes above the block explain any schema extensions; TOOLING §2 reconciles them.
6. **Gaps vs loops.** Every non-covered rule, most important first, with a priority (P0 = gate correctness, security or the hard test gate; P1 = architecture or quality; P2 = guidance) and the loops file to change.
   - IDs are often compressed (`X-TEST-06/07`, `X-REPO-01..14`, `X-IDIOM-*`).
   - MATRIX §5 lists the rules a file's Gaps table leaves out.
7. **Open questions.** Unsettled facts and decisions; TOOLING §7 merges them.
8. **Verification.** The verdict per round, sources opened, tooling truth, the spot run (image, commands, exit codes) and findings. **Spot-run facts outrank the rule text** where they differ.

## Refreshing the research

1. **Ranking.** Re-run RANKING with the newest editions of its sources, recording edition dates and keeping the formula and the backend filter. Changes to the list need maintainer approval.
2. **A language file.** Re-check `versions`, every argv in the LOOPS block (against the docs, plus a spot run in the official image), and the rows marked as changing over time (they carry a version or date). Keep rule IDs stable: renumbering breaks the citations here and in future packs. Then re-run B2 on that file.
3. **Baseline.** When loops changes, update LOOPS_BASELINE first, then re-judge the "loops today" column of the affected rows.
4. **Synthesis.** Re-run `matrix.py` (`python3 -I matrix.py research/languages --json rules.json`), `universal.py rules.json --md` and `packs.py research/languages`. Then update the grid in MATRIX §1.1 verbatim, the counts in this README, the clusters that moved, and TOOLING §1 and §7.
5. **Date it.** Update every "As of" line.
