# Gap matrix: 15 languages × 14 sections

As of **2026-11-10**, against loops v0.4.1 ([LOOPS_BASELINE.md](LOOPS_BASELINE.md)). This file is the B3 synthesis of the 15 verified language files. Read [README.md](README.md) for the method.

## 1. Counts

A script computed every count here; none were made by hand.

- **Script:** [scripts/matrix.py](scripts/matrix.py).
- **Command:** `python3 -I matrix.py research/languages --json rules.json`.
- **Rule rows** are the lines that match `` | `<LANG>-<SEC>-NN` | `` before `## Top 10 footguns`. Cells are split on unescaped `|`. The status is the leading word of the "loops today" cell.
- **Gaps rows** are the table rows under `## Gaps vs loops`. The script expands the compressed IDs found there:
  - slash lists: `X-SEC-02/03`;
  - ranges: `X-SEC-01..05`, or `` `X-SEC-02`..`-08` ``;
  - comma lists of ranges: `X-SEC-01..04,07..09`;
  - section wildcards: `X-SEC-*`;
  - bare `SEC-NN` IDs that follow a prefixed ID.
- **Priority** of a rule is the first `P0`/`P1`/`P2` in the first Gaps row that cites it.

### 1.1 Script output (verbatim)

Cells are covered/go-only/missing/n-a. Readiness = covered / (covered + go-only + missing).

| Language | REPO | TOOL | ARCH | IDIOM | CONC | TEST | PERF | SEC | DATA | API | OBS | OPS | DOC | FW | Rules | cov | go-only | missing | n/a | Readiness |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `python` | 1/1/14/0 | 0/3/9/0 | 6/3/3/0 | 2/1/13/0 | 2/5/5/0 | 2/11/3/0 | 0/2/3/0 | 0/3/7/0 | 4/1/3/0 | 3/1/4/0 | 0/2/3/0 | 1/3/3/0 | 1/2/1/0 | 0/0/6/0 | 137 | 22 | 38 | 77 | 0 | 16% |
| `typescript` | 1/0/12/0 | 0/0/10/0 | 2/4/5/0 | 6/1/7/0 | 4/2/4/0 | 1/8/3/0 | 0/2/3/0 | 1/2/7/0 | 3/0/3/0 | 3/1/3/0 | 0/1/4/0 | 1/2/5/0 | 1/2/1/0 | 0/0/6/0 | 121 | 23 | 25 | 73 | 0 | 19% |
| `java` | 1/1/10/0 | 0/4/4/0 | 4/4/2/0 | 3/2/8/0 | 1/3/4/0 | 0/7/5/0 | 0/2/3/0 | 0/2/6/0 | 3/0/4/0 | 4/1/1/0 | 0/1/4/0 | 1/3/3/0 | 1/2/1/0 | 0/0/7/0 | 112 | 18 | 32 | 62 | 0 | 16% |
| `cpp` | 0/0/15/0 | 0/3/7/0 | 5/2/4/0 | 1/1/12/0 | 0/6/3/0 | 1/9/1/0 | 0/3/2/0 | 0/3/6/0 | 3/0/2/0 | 3/1/3/0 | 0/1/4/0 | 0/4/3/0 | 1/2/1/0 | 2/0/2/0 | 116 | 16 | 35 | 65 | 0 | 14% |
| `csharp` | 2/0/13/0 | 0/3/5/0 | 3/5/3/0 | 2/1/8/0 | 2/4/3/0 | 1/8/3/0 | 0/3/2/0 | 1/4/4/0 | 4/0/2/0 | 3/1/2/0 | 0/1/4/0 | 1/3/3/0 | 1/2/1/0 | 1/0/3/0 | 112 | 21 | 35 | 56 | 0 | 19% |
| `php` | 1/0/13/0 | 0/2/8/0 | 3/3/4/0 | 0/0/12/0 | 1/2/4/0 | 1/7/3/0 | 0/3/2/0 | 0/2/8/0 | 1/2/3/0 | 3/1/2/0 | 0/1/3/0 | 0/4/2/0 | 1/3/0/0 | 1/0/3/0 | 109 | 12 | 30 | 67 | 0 | 11% |
| `go` | 6/1/8/0 | 6/0/3/0 | 11/0/1/0 | 17/0/2/0 | 6/0/4/0 | 10/0/5/0 | 4/0/3/0 | 8/0/3/0 | 5/0/4/0 | 7/0/3/0 | 4/0/3/0 | 3/0/7/1 | 3/0/2/0 | 3/0/3/0 | 146 | 93 | 1 | 51 | 1 | 64% |
| `ruby` | 0/1/13/0 | 0/1/7/0 | 1/7/4/0 | 0/5/8/0 | 1/3/6/0 | 0/8/4/0 | 0/2/3/0 | 0/3/5/0 | 1/2/3/0 | 1/3/2/0 | 0/1/3/0 | 0/3/2/1 | 1/2/1/0 | 1/0/4/0 | 113 | 6 | 41 | 65 | 1 | 5% |
| `swift` | 0/1/11/0 | 0/0/7/0 | 6/1/4/0 | 1/2/8/0 | 1/2/7/0 | 0/7/3/0 | 0/2/2/0 | 1/2/4/0 | 3/0/2/0 | 3/1/2/0 | 0/1/3/0 | 1/2/2/1 | 2/1/1/0 | 1/0/3/0 | 101 | 19 | 22 | 59 | 1 | 19% |
| `kotlin` | 1/1/10/0 | 0/4/2/0 | 2/5/2/0 | 1/5/5/0 | 1/5/2/0 | 0/9/2/0 | 0/3/1/0 | 0/3/4/0 | 2/0/3/0 | 3/1/2/0 | 0/1/3/0 | 0/4/2/0 | 1/2/1/0 | 0/0/4/0 | 97 | 11 | 43 | 43 | 0 | 11% |
| `rust` | 0/3/12/0 | 0/4/4/0 | 2/8/2/0 | 1/4/10/0 | 2/3/6/0 | 1/9/3/0 | 0/3/2/0 | 0/4/5/0 | 2/1/4/0 | 3/2/3/0 | 0/1/5/0 | 1/3/3/1 | 1/2/2/0 | 0/1/4/0 | 127 | 13 | 48 | 65 | 1 | 10% |
| `scala` | 1/0/14/0 | 0/2/7/0 | 4/4/4/0 | 1/1/10/0 | 1/4/2/0 | 2/7/2/0 | 0/3/1/0 | 0/5/2/0 | 2/0/3/0 | 3/1/2/0 | 0/1/3/0 | 1/3/2/0 | 1/2/1/0 | 0/0/4/0 | 106 | 16 | 33 | 57 | 0 | 15% |
| `elixir` | 0/0/11/0 | 0/1/9/0 | 3/3/5/0 | 1/2/9/0 | 1/1/7/0 | 2/7/2/0 | 0/2/2/0 | 1/1/7/0 | 4/0/3/0 | 3/0/3/0 | 0/1/3/0 | 0/4/1/1 | 1/2/1/0 | 1/0/4/0 | 109 | 17 | 24 | 67 | 1 | 16% |
| `erlang` | 0/0/12/0 | 0/4/5/0 | 0/7/4/0 | 1/4/8/0 | 0/3/6/0 | 1/9/2/0 | 0/3/2/0 | 0/5/6/0 | 2/0/3/0 | 2/3/1/0 | 0/2/3/0 | 0/4/1/1 | 2/1/1/0 | 0/1/3/0 | 112 | 8 | 46 | 57 | 1 | 7% |
| `clojure` | 0/0/17/0 | 0/3/6/0 | 4/4/2/0 | 0/5/9/0 | 1/1/6/0 | 1/7/3/0 | 0/2/3/0 | 0/3/7/0 | 2/0/3/0 | 3/1/2/0 | 0/1/3/0 | 0/4/2/0 | 1/2/1/0 | 2/0/2/0 | 113 | 14 | 33 | 66 | 0 | 12% |

All 15: 1731 rules; covered 309, go-only 486, missing 930, n/a 6; readiness 18%

Gaps-table sanity (non-covered = go-only + missing):

| Language | Gaps rows | Non-covered rules | Cited in Gaps | Uncited | P0/P1/P2 of cited | Malformed rows | Odd status |
|---|---|---|---|---|---|---|---|
| `python` | 15 | 115 | 103 | 12 | 16/56/31 | - | - |
| `typescript` | 15 | 98 | 74 | 24 | 11/21/42 | - | - |
| `java` | 19 | 94 | 87 | 7 | 18/48/21 | - | - |
| `cpp` | 28 | 100 | 100 | 0 | 11/56/33 | - | - |
| `csharp` | 19 | 91 | 87 | 4 | 10/51/26 | - | - |
| `php` | 20 | 97 | 91 | 6 | 20/48/23 | - | - |
| `go` | 29 | 52 | 46 | 6 | 5/27/14 | - | - |
| `ruby` | 13 | 106 | 85 | 21 | 15/36/34 | - | - |
| `swift` | 29 | 81 | 81 | 0 | 6/22/53 | - | - |
| `kotlin` | 19 | 86 | 64 | 22 | 8/31/25 | - | - |
| `rust` | 22 | 113 | 113 | 0 | 23/58/32 | - | - |
| `scala` | 16 | 90 | 66 | 24 | 9/42/15 | - | - |
| `elixir` | 19 | 91 | 81 | 10 | 10/39/32 | - | - |
| `erlang` | 18 | 103 | 80 | 23 | 16/38/26 | - | - |
| `clojure` | 15 | 99 | 85 | 14 | 18/44/23 | - | - |

- `python` uncited: PY-ARCH-04, PY-ARCH-12, PY-SEC-01, PY-SEC-04, PY-SEC-05, PY-SEC-06, PY-TEST-09, PY-TOOL-01, PY-TOOL-02, PY-TOOL-06, PY-TOOL-11, PY-TOOL-12
- `typescript` uncited: TS-ARCH-08, TS-ARCH-09, TS-ARCH-11, TS-CONC-01, TS-CONC-03, TS-CONC-05, TS-IDIOM-03, TS-IDIOM-04, TS-IDIOM-07, TS-IDIOM-09, TS-IDIOM-11, TS-IDIOM-12, TS-IDIOM-14, TS-SEC-07, TS-SEC-08, TS-SEC-09, TS-SEC-10, TS-TEST-07, TS-TOOL-01, TS-TOOL-02, TS-TOOL-04, TS-TOOL-05, TS-TOOL-09, TS-TOOL-10
- `java` uncited: JAVA-ARCH-01, JAVA-DATA-07, JAVA-SEC-03, JAVA-SEC-04, JAVA-SEC-08, JAVA-TOOL-01, JAVA-TOOL-07
- `csharp` uncited: CS-IDIOM-09, CS-OBS-01, CS-OPS-01, CS-REPO-14
- `php` uncited: PHP-ARCH-08, PHP-ARCH-09, PHP-CONC-05, PHP-FW-01, PHP-FW-04, PHP-TEST-08
- `go` uncited: GO-DOC-04, GO-DOC-05, GO-OPS-04, GO-REPO-06, GO-REPO-13, GO-TOOL-01
- `ruby` uncited: RUBY-ARCH-04, RUBY-CONC-06, RUBY-CONC-07, RUBY-DATA-01, RUBY-OPS-01, RUBY-OPS-02, RUBY-OPS-04, RUBY-SEC-06, RUBY-SEC-07, RUBY-TEST-01, RUBY-TEST-03, RUBY-TEST-04, RUBY-TEST-05, RUBY-TEST-08, RUBY-TEST-10, RUBY-TEST-11, RUBY-TEST-12, RUBY-TOOL-01, RUBY-TOOL-04, RUBY-TOOL-05, RUBY-TOOL-07
- `kotlin` uncited: KT-ARCH-05, KT-ARCH-08, KT-ARCH-09, KT-CONC-01, KT-CONC-02, KT-CONC-03, KT-CONC-05, KT-CONC-06, KT-CONC-07, KT-IDIOM-05, KT-IDIOM-07, KT-IDIOM-08, KT-IDIOM-09, KT-IDIOM-10, KT-IDIOM-11, KT-SEC-01, KT-SEC-02, KT-SEC-03, KT-SEC-04, KT-SEC-05, KT-TOOL-01, KT-TOOL-06
- `scala` uncited: SCALA-ARCH-07, SCALA-ARCH-12, SCALA-CONC-01, SCALA-CONC-02, SCALA-CONC-03, SCALA-CONC-05, SCALA-CONC-06, SCALA-DATA-01, SCALA-DATA-04, SCALA-DATA-05, SCALA-FW-01, SCALA-FW-02, SCALA-FW-04, SCALA-OPS-02, SCALA-OPS-03, SCALA-TEST-01, SCALA-TEST-02, SCALA-TEST-03, SCALA-TEST-08, SCALA-TEST-10, SCALA-TOOL-02, SCALA-TOOL-06, SCALA-TOOL-07, SCALA-TOOL-09
- `elixir` uncited: ELIXIR-ARCH-02, ELIXIR-ARCH-03, ELIXIR-ARCH-11, ELIXIR-CONC-06, ELIXIR-OPS-01, ELIXIR-REPO-01, ELIXIR-REPO-02, ELIXIR-REPO-03, ELIXIR-REPO-10, ELIXIR-TOOL-09
- `erlang` uncited: ERL-ARCH-11, ERL-CONC-01, ERL-CONC-02, ERL-CONC-03, ERL-CONC-09, ERL-REPO-12, ERL-SEC-01, ERL-SEC-02, ERL-SEC-06, ERL-SEC-07, ERL-SEC-08, ERL-SEC-09, ERL-SEC-10, ERL-SEC-11, ERL-TEST-01, ERL-TEST-03, ERL-TEST-04, ERL-TEST-05, ERL-TEST-10, ERL-TEST-11, ERL-TEST-12, ERL-TOOL-01, ERL-TOOL-09
- `clojure` uncited: CLJ-ARCH-01, CLJ-TEST-01, CLJ-TEST-03, CLJ-TEST-04, CLJ-TEST-05, CLJ-TEST-08, CLJ-TEST-09, CLJ-TEST-11, CLJ-TOOL-01, CLJ-TOOL-04, CLJ-TOOL-05, CLJ-TOOL-06, CLJ-TOOL-08, CLJ-TOOL-09

### 1.2 Reading the grid

- Every cell is `covered/go-only/missing/n-a`.
- **Readiness** is covered ÷ (covered + go-only + missing). `n/a` rows don't count.
- **Go's 64% is not comparable** with the other languages. The Go file marks a rule `covered` when GO_BEST, HEXAGONAL or SOLID states it in Go terms (go.md Open question 9, accepted by its verifier). Every other file marks those same specs `go-only`, as LOOPS_BASELINE says. On the baseline's scale, Go's readiness would also be low.
- Among the other 14 languages, readiness runs from 5% (Ruby) to 19% (TypeScript, C#, Swift). Their median is 14%, and pooled they are 14% (216 covered, 485 go-only, 879 missing).
- **The pattern repeats across languages:**
  - REPO is almost entirely `missing`: there are no repo-management rules.
  - TEST is almost entirely `go-only`: the hard gate is Go-only.
  - DATA and API hold most of the `covered` rows: the consistency and jobs skills, and HEXAGONAL's error taxonomy, are neutral.


## 2. P0 gap clusters

Each language's P0 rows from its Gaps table, deduplicated across languages. "Hits" lists the languages whose files raise the cluster at P0. Languages in brackets raise it only at P1. File paths are relative to the repo root.

| # | Cluster | Hits | Evidence (rule IDs) | loops files to change |
|---|---|---|---|---|
| P0-1 | **Gate is a Go `switch`.** `runCheck` hard-codes build, gofmt, vet, staticcheck, test and govulncheck, so no other language's format, lint, typecheck, build or test check can run. The other P0 failure modes come with it: a gate that passes without running tests (a Gradle `UP-TO-DATE` test task, a `ctest` that finds zero tests), tools that fail only by exit code, and conditional checks | 12 [Python, C# at P1] | TS-TOOL-06/07/03, JAVA-TOOL-02..08, CPP-TEST-10, CPP-CONC-03/06, CPP-TOOL-04, PHP-TOOL-02/04/05, RUBY-TOOL-02/03/06, SWIFT-TOOL-02, SWIFT-REPO-05, KT-TEST-11, RUST-TOOL-02/03/06, SCALA-TOOL-03/04/05, ELIXIR-TOOL-03/04/05/08, ERL-TOOL-02..08, CLJ-TOOL-02/03; P1: PY-TOOL-03..09, CS-TOOL-02/03/05, KT-TOOL-02/03/04 | `internal/application/gate/gate.go` (`CheckNames`, `RequiredChecks`, `runCheck`, `available`, `trim`/`isTestSignal`); a new pack loader (domain, pure); `specs/WORKFLOW.md` §6; `agents/qa-verifier.md` |
| P0-2 | **No non-Go coverage parser**, so the ≥70% new-code gate can't run. Parsing reads only Go coverprofiles. Candidate selection knows only `.go` and `_test.go`, and `DefaultExclude` is Go-only | 14 (all but Go) | PY-TEST-08/10, TS-TEST-06/08, JAVA-TEST-06/07, CPP-TEST-06/07, CS-TEST-06/07/09, PHP-TEST-06/07, RUBY-TEST-06/07, SWIFT-TEST-06/07, KT-TEST-06/07, RUST-TEST-06/07/13, SCALA-TEST-06/07, ELIXIR-TEST-06/07, ERL-TEST-06/07, CLJ-TEST-06/07 | `internal/domain/coverage/profile.go` (format-neutral `Profile`), `evaluate.go` (`Candidates`, `DefaultExclude`, the `module` prefix), `internal/application/gate/coverage.go` (`Coverage` port: `Profile`, `Module`, `HasTests`), new parser adapters beside `internal/adapters/outbound/gocover` |
| P0-3 | **No `race.mode = none` (or `build`) policy.** race-guard and the hard rule assume `-race` | 14: 12 `none`, C++ `build`, and Swift (`flag` with a different flag) | PY-CONC-05/06, TS-CONC-04, JAVA-CONC-06, CPP-CONC-04, CS-CONC-04, PHP-CONC-04, PHP-TEST-09, PHP-CONC-07, RUBY-CONC-05, SWIFT-CONC-03, KT-CONC-04, RUST-CONC-03/04, SCALA-CONC-04, SCALA-TEST-11, ELIXIR-CONC-04, ERL-CONC-04, CLJ-CONC-04/08 | `internal/application/guard/race.go` (`CheckRace`, `raceMsg`), `internal/adapters/inbound/hook/race.go`, gate `CheckTest`, `specs/WORKFLOW.md` §6 and §8 item 3, `ROLES.md` (shared implementer rules) |
| P0-4 | **Run-scoped cache and cleanup are Go-only.** The run env is just `GOCACHE`/`GOTMPDIR`. Artifacts are matched only by Go suffixes. Cleanup deletes regular files only, so it never removes `target/`, `_build/` or `obj/` | 8 [TS, PHP, Ruby, Swift, Scala, Clojure at P1] | PY-REPO-13, PY-OPS-06/07, JAVA-OPS-05, CPP-OPS-05/06, CS-OPS-05/06, KT-OPS-05/06, RUST-OPS-05/06, ELIXIR-OPS-05/06, ERL-OPS-05/06; P1: TS-OPS-05/06, PHP-OPS-05/06, RUBY-OPS-05/06, SWIFT-OPS-05/06, SCALA-OPS-05/06, CLJ-OPS-05/06 | `internal/domain/workspace/workspace.go` (`GoCache`, `GoTmp`, `Env`, `Exports`, `artifactSuffixes`, `Decide`), `internal/application/guard/goenv.go` and the hook `go-env`, `loops go`, `loops run env`, `internal/adapters/outbound/runspace` |
| P0-5 | **No language detection.** `loops init` needs `go.mod`, and toolchain and wrapper pins aren't read | 5 [Swift, Elixir at P1] | TS-REPO-05/06/10, JAVA-REPO-03/04, CS-REPO-11, CS-TOOL-01, the Kotlin "Language detection" row, the Clojure "Language detection" row; P1: SWIFT-REPO-09, ELIXIR-TOOL-01 | `internal/adapters/inbound/cli` (`init`), `internal/domain/hw` (`go_version` hint), a pack `detect` matcher |
| P0-6 | **Test-file recognition and the dirty flag are Go-only.** `GoDirty` fires only on `.go` edits. Ownership globs are `*_test.go`. The test guard recognises only `go test` | 5 [Python, C#, Ruby, PHP at P1] | JAVA-TEST-02, ELIXIR-TEST-02, ERL-TEST-02, CLJ-TEST-02, SCALA-TEST-11; P1: PY-TEST-01..07, CS-TEST-01..05, RUBY-TEST-02/09, PHP-TEST-01..05 | `internal/domain/run/fold.go` (`GoDirty`), the hook `stop-gate`, `internal/application/plancheck`, race-guard's command recogniser, ownership globs in `agents/*.md` |
| P0-7 | **No vulnerability step for non-Go ecosystems.** For Go, `govulncheck` is optional and silent when it isn't installed | 12 [Kotlin, C++ at P1; Swift has no tool] | PY-TOOL-10, TS-SEC-06, TS-TOOL-08, JAVA-SEC-06, CS-TOOL-06, CS-SEC-07, PHP-TOOL-08, PHP-SEC-09, RUBY-SEC-08 row, SWIFT-TOOL-05, RUST-TOOL-07, SCALA-TOOL-08, SCALA-SEC-06, ELIXIR-TOOL-08, CLJ-TOOL-07, CLJ-SEC-09, GO-TOOL-06 | gate `CheckVuln` and `available` (`o.Vuln && Has`), `Options.Vuln`, `gate.vuln` config |
| P0-8 | **No SAST step and no security rules** (no OWASP mapping, no unsafe-deserialisation rules) | 9 [TS, C#, Kotlin, Scala, Elixir at P1] | GO-SEC-10, GO-SEC-05, GO-SEC-03, PY-SEC-02/03/08/09/10, JAVA-SEC-01/02/07, CPP-SEC-01/02, PHP-SEC-01..10, RUBY-SEC-01..05, RUBY-SEC-08, RUST-SEC-06/07, ERL-SEC-03/04/05, CLJ-SEC-01/03/09 | a new neutral `specs/SECURITY.md`; gate check kind `sast`; `skills/review-integration` checklist; `GO_BEST` §29 and §38 (`os.Root`) |
| P0-9 | **Lockfile, frozen-install and toolchain pins are not enforced** (repo management) | 10 [all others at P1] | PY-REPO-04/05/09, TS-REPO-05/06/10, JAVA-REPO-03/04, CS-REPO-11, PHP-REPO-07, RUBY-REPO-05, RUBY-TOOL-08, SWIFT-REPO-05, KT-REPO-06, KT-SEC-06, RUST-REPO-06/11/12, GO-REPO-11/12 | a new `specs/REPO_PRACTICES.md`; a gate check kind `deps` (frozen install or lock verify); `agents/practices-researcher.md` (`config_files`) |
| P0-10 | **Go multi-module repos aren't gated.** `./...` stops at a nested `go.mod` | Go | GO-REPO-04 | gate `Run`: discover modules (`go work edit -json`) and record one result per module, additive in the event schema |

## 3. P1 gap clusters

| # | Cluster | Hits | Evidence (rule IDs) | loops files to change |
|---|---|---|---|---|
| P1-1 | **No arch parser for language X.** `archcheck.Parser` has only the Go adapter, and there is no delegation to a native tool | 14 | PY-ARCH-02/03/05, TS-ARCH-04/05, JAVA-ARCH-02/03/05, CPP-ARCH-02..06, CS-ARCH-02/04, PHP-ARCH-03..05, RUBY-ARCH-05/06/07, SWIFT-ARCH-05/06/07, KT-ARCH-02..04, RUST-ARCH-02/03/04, SCALA-ARCH-08/09, ELIXIR-ARCH-04/05, ERL-ARCH-01..07, CLJ-ARCH-02/03 | `internal/application/archcheck` (Parser port, plus a native-tool runner), new adapters beside `internal/adapters/outbound/goparse`, `internal/domain/archrule/imports.go` |
| P1-2 | **Purity and impure-call lists are Go-only** (`os`, `net/http`, `time.Now`, `math/rand`, `go`) | 9 | PY-ARCH-07, TS-ARCH-07, PHP-ARCH-07, KT-ARCH-06, RUST-ARCH-07, ELIXIR-ARCH-07, CS-ARCH-07, the RUBY-ARCH-09 row; SWIFT-ARCH-09 is `covered` but has a P1 row | `internal/domain/archrule` (data from the pack's `arch` block) |
| P1-3 | **SOLID size limits use Go units** | 14 (the ARCH-08 rows are `go-only`); P1 rows in 6 | PY-ARCH-09, CPP-ARCH-07..11, SWIFT-ARCH-11, ELIXIR-ARCH-09/10, RUST-ARCH-12 (in the RUST-ARCH-08..12 row), CLJ-ARCH-04..10 | `specs/SOLID_GO.md` §8, `internal/domain/archrule` |
| P1-4 | **The hexagonal layout and examples are Go** (HX §4, §14, §15, §22–§25) | 12 | ERL-ARCH-01..07, CLJ-ARCH-04..10, RUST-ARCH-01/05/06/08..12, CS-ARCH-01/03/11, JAVA-ARCH-04/06/07/10, SCALA-ARCH-04/05/06, TS-ARCH-02/03/06/10, RUBY-ARCH-02/03/08/10..12, PHP-ARCH-10, KT-ARCH-02..04, ELIXIR-ARCH-07/09/10, CPP-ARCH-07..11 | `specs/HEXAGONAL_ARCHITECTURE.md` (a neutral core plus per-language layout appendices), `agents/architect.md` |
| P1-5 | **The post-edit hook is Go-only** (`.go`, gofmt, vet) | 7 | PY-TOOL-03..09, PHP-TOOL-02/04/05 (a P0 row), RUST-TOOL-02/03/06 (a P0 row), the Scala "post-edit hook" row, ERL-IDIOM-01..13, CPP-TOOL-03/05, the Kotlin "Post-edit hook" row (P2) | the hook `go-post-edit` (pack `post_edit`), `internal/adapters/inbound/hook` |
| P1-6 | **No repository-management rules** (layout, workspaces, bootstrap, bots, release, hygiene, CI shape) | 15 (P2 for TS, Ruby, Swift) | CPP-REPO-02..14, CS-REPO-01..10, PHP-REPO-01..13, KT-REPO-01..11, RUST-REPO-01..10/13/14, SCALA-REPO-*, ELIXIR-REPO-04/05/08/09, ERL-REPO-01..11, CLJ-REPO-01..17, GO-REPO-03/05/09/10/14, PY-REPO-01..15 | new `specs/REPO_PRACTICES.md` and a `repo-management` skill; `templates/` CI shapes; `agents/architect.md` |
| P1-7 | **No per-language idiom rules** (errors, null, immutability, resources, metaprogramming) | 15 (P2 for Elixir, Go) | PY-IDIOM-02..16, PHP-IDIOM-01..12, CS-IDIOM-02..07, JAVA-IDIOM-03..13, CPP-IDIOM-01..14, RUST-IDIOM-05..15, KT-IDIOM-01..04, SCALA-IDIOM-*, RUBY-IDIOM-*, ERL-IDIOM-01..13, CLJ-IDIOM-01..14, SWIFT-IDIOM-02/11 | `specs/languages/<lang>/RULES.md` (TAXONOMY intro), engineer roles |
| P1-8 | **Concurrency and async guidance is Go-worded or absent** | 13 | PY-CONC-01..12, TS-CONC-06/07/08, JAVA-CONC-01..04, CPP-CONC-01/02/05/07/08/09, CS-CONC-01..09, PHP-CONC-01..03, RUBY-CONC-01..10, SWIFT-CONC-02/04..07, RUST-CONC-01/02/05..08/11, ELIXIR-CONC-01..09, ERL-CONC-05..08, CLJ-CONC-01..07, GO-CONC-01/06/09/10 | a neutral `specs/CONCURRENCY.md` that lifts GO_BEST §10–§12; pack RULES |
| P1-9 | **The testing skill is Go** (httptest, table tests, `*_test.go`) | 13 (P2 for TS) | PY-TEST-01..07, TS-TEST-01..05, JAVA-TEST-01/03/04/05/08..12, CPP-TEST-01..05/08/09/11, CS-TEST-01..05, PHP-TEST-01..05, RUST-TEST-01..05/08..12, KT-TEST-01..05/08..10, ELIXIR-TEST-01/03/04/05/08..11, ERL-TEST-08, SWIFT-TEST-01..10, GO-TEST-10/12 | `skills/testing` split into a neutral core plus a per-language reference; `specs/TESTING.md` |
| P1-10 | **practices-researcher reads only Go linter configs** (`.golangci.*`, `.editorconfig`) | 6 named (all 15 need it) | PHP-ARCH-10, SWIFT-TOOL-03, RUST-REPO-06/11/12, GO-TOOL-04, PY-REPO-01..15, RUBY-REPO-05 | `agents/practices-researcher.md` (read the pack's `config_files`) |
| P1-11 | **Persistence and ORM traps** (N+1, migrations, pools) | 7 (8 more at P2) | JAVA-DATA-04/05/06, RUBY-DATA-02/04/05/06, PHP-DATA-01/02/04/05, CS-DATA-04/05, ELIXIR-DATA-02/03/05/06, RUST-DATA-01..07, GO-DATA-01..05/08 | `skills/consistency` plus a neutral `specs/PERSISTENCE.md` |
| P1-12 | **Security guidance (OWASP) beyond P0** | 8 | TS-SEC-01..05, CS-SEC-01..05/08/09, KT-SEC-07, SCALA-SEC-*, ELIXIR-SEC-01..09, CPP-SEC-03..07/09, CLJ-SEC-* (except 01/03/09), GO-SEC-02/04/07/09 | `specs/SECURITY.md`, pack RULES |

The P2 clusters: performance tooling, observability and OpenTelemetry, packaging and AOT, doc tools, and framework conventions. All 15 files rank them P2. They belong in skill reference files, not in the gate.

## 4. The 5 most important gaps per language

Taken from each file's Gaps table (most important first) and tied to the clusters above.

| Language | 1 | 2 | 3 | 4 | 5 |
|---|---|---|---|---|---|
| Python | No Cobertura parser for the hard gate (PY-TEST-08/10; P0-2) | Race `none`: substitute flags `-p randomly -n 2` and dev mode aren't checked (PY-CONC-05/06; P0-3) | No pip-audit or bandit step (PY-TOOL-10, PY-SEC-09; P0-7/8) | No `uv sync --locked`, `.python-version` or run-scoped cache (PY-REPO-05/09, PY-OPS-06/07; P0-9/4) | No ast parser or import-linter bridge (PY-ARCH-02/03/05; P1-1) |
| TypeScript | No lcov parser (TS-TEST-06/08; P0-2) | Race `none`: shuffle, concurrent and repeats, and a ban on `--retry` (TS-CONC-04; P0-3) | No tsc, eslint, prettier or vitest checks (TS-TOOL-06/07/03; P0-1) | No `npm audit`, and no install-script policy (TS-SEC-06, TS-TOOL-08; P0-7) | No lockfile, `engines` or `.nvmrc` rules, and no `package.json` detection (TS-REPO-05/06/10; P0-5) |
| Java | No JaCoCo XML parser (JAVA-TEST-06/07; P0-2) | Race `none`: parallel plus random order with two seeds (JAVA-CONC-06; P0-3) | No pom/gradle detection and no `./mvnw`/`./gradlew` preference (JAVA-REPO-03/04; P0-5) | Test globs `src/test/java`, `*Test.java` (JAVA-TEST-02; P0-6) | Share `~/.m2`, scope only outputs and tmp (JAVA-OPS-05; P0-4) |
| C++ | Race is a separate TSan build (`build` mode) (CPP-CONC-04; P0-3) | No lcov parser and no C++ candidate globs (CPP-TEST-06/07; P0-2) | `ctest` exits 0 with zero tests unless `--no-tests=error` (CPP-TEST-10; P0-1) | Several sanitizer builds mean several test checks (CPP-CONC-03/06; P0-1) | No hardening or memory-safety policy (CPP-SEC-01/02; P0-8) |
| C# | Race `none` (CS-CONC-04; P0-3) | No Cobertura parser; the `{results}` glob; multi-project collisions (CS-TEST-06/07/09; P0-2) | No detection and no SDK pin check (CS-REPO-11, CS-TOOL-01; P0-5) | The NuGet vuln list exits 0, so its output must be parsed (CS-TOOL-06, CS-SEC-07; P0-7) | NuGet and MSBuild cache env, and `bin/` `obj/` cleanup (CS-OPS-05/06; P0-4) |
| PHP | No PHPUnit run and no Clover parser (PHP-TEST-06/07; P0-2) | Race `none`: random order, paratest, fail-on-risky (PHP-CONC-04, PHP-TEST-09; P0-3) | No security rules (`unserialize`, injection, php.ini) (PHP-SEC-01..10; P0-8) | No `composer audit --locked` or `validate --strict` (PHP-TOOL-08, PHP-SEC-09; P0-7) | No PHP-CS-Fixer/PHPStan checks, and the post-edit hook ignores `.php` (PHP-TOOL-02/04/05; P0-1) |
| Go | Nested modules are never gated (GO-REPO-04; P0-10) | No SAST (`gosec`) (GO-SEC-10; P0-8) | HTTP server timeouts and `os.Root` aren't required (GO-SEC-05, GO-SEC-03; P0-8) | `govulncheck` is optional and silent (GO-TOOL-06; P0-7) | No toolchain pin or `tool` directive; tools come from PATH `@latest` (GO-REPO-11/12; P0-9) |
| Ruby | Race `none` (RUBY-CONC-05; P0-3) | No SimpleCov JSON or lcov parser (RUBY-TEST-06/07; P0-2) | No bundle, RuboCop, Brakeman or bundler-audit checks (RUBY-TOOL-02/03/06, RUBY-SEC-08; P0-1/8) | No frozen-lockfile install or `bundle exec` discipline (RUBY-REPO-05, RUBY-TOOL-08; P0-9) | No Ruby security rules (Marshal/YAML, mass assignment) (RUBY-SEC-01..05; P0-8) |
| Swift | lcov is a two-step export, and the test binaries must be discovered per OS (SWIFT-TEST-06/07; P0-2) | race-guard knows only `go test -race`, not `--sanitize=thread` (SWIFT-CONC-03; P0-3) | No lockfile-freshness check `--force-resolved-versions` (SWIFT-REPO-05; P0-9) | No vulnerability scanner exists (SWIFT-TOOL-05; P0-7) | `swift format lint --strict` fails by exit code and needs a committed config (SWIFT-TOOL-02; P0-1) |
| Kotlin | No Kover/JaCoCo parser (KT-TEST-06; P0-2) | Race `none`; the substitute needs `junit-platform.properties` (KT-CONC-04; P0-3) | No `.kt` new-code candidates or test globs (KT-TEST-07; P0-2/6) | Gradle `UP-TO-DATE` makes `test` a no-op without `--rerun` (KT-TEST-11; P0-1) | `GRADLE_USER_HOME` must be shared, `.kotlin/` and `build/` cleaned (KT-OPS-05/06; P0-4) |
| Rust | No `cargo llvm-cov` lcov run or parse (RUST-TEST-06/07/13; P0-2) | Race `none` plus an optional miri/loom/TSan tier (RUST-CONC-03/04; P0-3) | No fmt or clippy `-D warnings` checks (RUST-TOOL-02/03/06; P0-1) | No `cargo deny`/`cargo audit` (RUST-TOOL-07, RUST-SEC-06/07; P0-7) | `CARGO_TARGET_DIR` per run, `CARGO_HOME` shared (RUST-OPS-05/06; P0-4) |
| Scala | No Cobertura parser (SCALA-TEST-06/07; P0-2) | Race `none` (3× parallel repeat); the guard must accept the sbt, mill and scala-cli prefixes (SCALA-CONC-04, SCALA-TEST-11; P0-3) | No scalafmt, scalafix or `-Werror` checks (SCALA-TOOL-03/04/05; P0-1) | No dependency-check or OSV step (SCALA-TOOL-08, SCALA-SEC-06; P0-7) | No Scala import parser (SCALA-ARCH-08/09; P1-1) |
| Elixir | No lcov parser; excoveralls is needed (ELIXIR-TEST-06/07; P0-2) | Race `none`: `--repeat-until-failure` (ELIXIR-CONC-04; P0-3) | No `mix format`, compile `--warnings-as-errors`, Credo or audit checks (ELIXIR-TOOL-03/04/05/08; P0-1) | Test recognition is `*_test.go` (ELIXIR-TEST-02; P0-6) | `MIX_BUILD_ROOT`, and cleanup of `_build`/`cover` (ELIXIR-OPS-05/06; P0-4) |
| Erlang | Race `none`: scheduler variants `+S 1:1`/`+S 8:8` (ERL-CONC-04; P0-3) | covertool writes Cobertura, one file per app (ERL-TEST-06/07; P0-2) | Test globs `*_SUITE.erl`, `*_tests.erl` (ERL-TEST-02; P0-6) | No fmt, xref, Elvis, Dialyzer or audit checks (ERL-TOOL-02..08; P0-1) | No `binary_to_term [safe]`/atom-exhaustion rules (ERL-SEC-03/04/05; P0-8) |
| Clojure | Race `none`: repeated seeded runs (CLJ-CONC-04/08; P0-3) | No lcov parser (CLJ-TEST-06/07; P0-2) | No cljfmt or clj-kondo checks; kondo exits 2/3 (CLJ-TOOL-02/03; P0-1) | The vuln step needs an NVD key, and OSV lacks Clojure (CLJ-TOOL-07, CLJ-SEC-09; P0-7) | Test globs, and detection of `deps.edn`/`project.clj` (CLJ-TEST-02, detection row; P0-5/6) |

## 5. Sanity check: are the Gaps tables complete?

The script compares each file's non-covered rules (`go-only` + `missing`) with the rule IDs cited in its Gaps table (§1.1, second table). Rows that cite no rule ID are counted as rows but can't be matched: Kotlin "Language detection" and "Post-edit hook", Scala "post-edit hook", Clojure "Language detection". This is a note only; the files are not changed.

| Finding | Files |
|---|---|
| Complete: every non-covered rule is cited | `cpp`, `swift`, `rust` |
| Nearly complete (≤7 uncited) | `java` 7, `go` 6, `php` 6, `csharp` 4 |
| Notable omissions (10–24 uncited) | `typescript` 24 (TOOL-01/02/04/05/09/10, IDIOM-03..14, SEC-07..10), `scala` 24 (CONC-01..06, TEST-01..03/08/10, FW-01/02/04), `erlang` 23 (SEC-01/02/06..11, TEST-01/03..05/10..12, CONC-01..03/09), `kotlin` 22 (CONC-01..07, SEC-01..05, IDIOM-05/07..11), `ruby` 21 (TEST-01/03..05/08/10..12, TOOL-01/04/05/07), `clojure` 14 (TEST-01/03..05/08/09/11, TOOL-01/04..06/08/09), `python` 12 (SEC-01/04/05/06, TOOL-01/02/06/11/12), `elixir` 10 (REPO-01..03/10, ARCH-02/03/11) |
| P0 rows citing `covered` rules (allowed, but they skew the counts) | `go`: GO-SEC-03, GO-TOOL-06 (P0), GO-TEST-06, GO-ARCH-02 (P1); `swift`: SWIFT-ARCH-09 (P1) |
| Mixed priority in one row; the script takes the first | `clojure`: `CLJ-SEC-01..10` "P0 (SEC-01,03,09) P1 rest" |
| Uncited rules carry no priority, so the P0/P1/P2 totals undercount | the 173 uncited rules (§1.1) |

The uncited rules are mostly P1/P2 guidance (idioms, concurrency, security detail). None of them changes a P0 cluster: each one belongs to a cluster above (P1-7, P1-8, P1-12, P1-9).
