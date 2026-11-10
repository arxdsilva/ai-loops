# The 15 biggest backend languages: ranking

As of **2026-11-10**. This picks the 15 languages that will each get a loops language pack. It is a proposal awaiting approval (Checkpoint 1). The method is explicit, so the list can be recomputed when new survey editions come out.

## Sources

| # | Source | Edition | Signal used | Read |
|---|---|---|---|---|
| S1 | [Stack Overflow Developer Survey](https://survey.stackoverflow.co/2026/technology) ([data](https://survey.stackoverflow.co/2026/technology/data/languages), [announcement](https://stackoverflow.blog/2026/10/06/the-results-of-the-2026-developer-survey-are-here/)) | 2026, published 2026-10-06 | % of all respondents (n = 14,099) who used the language extensively in the past year. The 2026 edition has no "professional developers" tab. | 2026-11-10 |
| S2 | [TIOBE Index](https://www.tiobe.com/tiobe-index/) | November 2026 | Rating % | 2026-11-10 |
| S3 | [RedMonk Programming Language Rankings](https://redmonk.com/sogrady/2026/04/14/language-rankings-1-26/) | January 2026, published 2026-04-14 (the latest edition) | Rank, which combines GitHub and Stack Overflow signals | 2026-11-10 |
| S4 | [GitHub Octoverse](https://github.blog/news-insights/octoverse/octoverse-commits-pushes-reach-new-highs-in-2026-as-developers-build-with-agents/) | 2026, published 2026-10-28, covering Oct 2025 to Sep 2026 | Qualitative cross-check only. Order: TypeScript, Python, JavaScript; Rust is the fastest-growing at +162%; Java grew 30%. | 2026-11-10 |
| S5 | [JetBrains State of Developer Ecosystem](https://devecosystem-2025.jetbrains.com/tools-and-trends) | 2025 (the 2026 language data isn't published) | Qualitative only: "PHP, Ruby, and Objective-C are in long-term decline while Rust, Go, and Kotlin continue their slow, steady ascent." | 2026-11-10 |
| S6 | [IEEE Spectrum Top Programming Languages](https://spectrum.ieee.org/top-programming-languages-2025) | 2025 (the 2026 page returns 404) | Not used. Its ranking table is interactive and couldn't be read. | 2026-11-10 |

The planned weights assumed numbers from JetBrains and IEEE. Neither published usable figures, so the weights are renormalised over the three numeric sources.

## Method

1. **Candidates:** every general-purpose language that appears in S1. Languages that aren't general-purpose are excluded up front: SQL, HTML/CSS, shells, PowerShell, assembly, R, MATLAB, Fortran, Scratch, the Visual Basic variants and Delphi. SQL practices become part of the generic persistence guidance, and shell becomes part of the tooling guidance.
2. **Merges:** TypeScript and JavaScript count as one pack, since they share a runtime (Node, Deno or Bun); each source takes the higher of the two values. C counts toward C++ only as a tooling note, and the C++ values are its own.
3. **Score:** `0.4 × S1/max(S1) + 0.3 × S2/max(S2) + 0.3 × RedMonk`, where the RedMonk term is `(21 − rank)/20` for its top 20 and 0 otherwise. Languages in TIOBE's 51–100 band have no published rating and get 0.1%.
4. **Backend filter:** a language is in only if building server-side services is one of its main uses, backed by a mainstream backend ecosystem. Languages whose main use is elsewhere are dropped, whatever their score:
   - Dart is mainly used for Flutter clients;
   - Lua is mainly embedded scripting;
   - Groovy is mainly Gradle and Jenkins scripting;
   - Zig is systems programming with no mainstream backend frameworks.

## Scores

| Rank | Language | S1 % | S2 % | S3 rank | Score | Backend filter |
|---|---|---|---|---|---|---|
| 1 | Python | 58.0 | 20.86 | 2 | 0.959 | in |
| 2 | TypeScript / JavaScript | 62.0 | 2.42 | 1 | 0.735 | in |
| 3 | Java | 27.6 | 8.89 | 3 | 0.576 | in |
| 4 | C++ | 24.3 | 8.35 | 7 | 0.487 | in |
| 5 | C# | 27.1 | 3.39 | 4 | 0.479 | in |
| 6 | PHP | 17.7 | 1.58 | 4 | 0.392 | in |
| 7 | Go | 16.5 | 1.69 | 12 | 0.266 | in |
| 8 | Ruby | 5.8 | 0.54 | 9 | 0.225 | in |
| 9 | Swift | 5.9 | 0.89 | 11 | 0.201 | in (server-side Swift: Vapor, Hummingbird) |
| 10 | Kotlin | 10.8 | 0.44 | 14 | 0.181 | in |
| 11 | Rust | 18.0 | 1.63 | 20 | 0.155 | in |
| 12 | Scala | 2.2 | 0.34 | 14 | 0.124 | in |
| – | Dart | 4.9 | 0.74 | 18 | 0.087 | **out** (Flutter clients) |
| – | Lua | 9.4 | 0.49 | – | 0.068 | **out** (embedded scripting) |
| 13 | Perl | 3.8 | 1.14 | – | 0.041 | in (legacy web and back-office) |
| – | Groovy | 3.9 | 0.1 | – | 0.027 | **out** (build and CI scripting) |
| – | Zig | 2.8 | 0.1 | – | 0.020 | **out** (systems) |
| 14 | Haskell | 2.3 | 0.28 | – | 0.019 | in |
| 15 | Elixir | 2.6 | 0.1 | – | 0.018 | in |
| alt 1 | OCaml | 1.2 | 0.43 | – | 0.014 | in |
| alt 2 | Clojure | 1.3 | 0.23 | – | 0.012 | in |
| alt 3 | Erlang | 1.5 | 0.1 | – | 0.011 | in |
| alt 4 | F# | 1.2 | 0.1 | – | 0.009 | in |
| alt 5 | Gleam | 1.2 | 0.0 | – | 0.008 | in |

## Proposed list

**Positions 1–12 are clear from the data.** Each is separated from the next by at least 0.03:

1. Python
2. TypeScript / JavaScript
3. Java
4. C++
5. C#
6. PHP
7. Go
8. Ruby
9. Swift
10. Kotlin
11. Rust
12. Scala

**Positions 13–15 fall in a noise band.** Perl (0.041) sits clearly above the rest. Haskell, Elixir, OCaml, Clojure, Erlang, F# and Gleam all score between 0.008 and 0.019, too close for the data to separate them. By score, the list continues:

13. Perl
14. Haskell
15. Elixir

The alternates are OCaml, Clojure, Erlang, F# and Gleam.

**Judgement calls for you to confirm:**
- **Perl vs. a BEAM or JVM language.** Perl ranks on volume, but S5 and its trend put it in long-term decline. Swapping Perl for **Clojure** or **Erlang** would give the packs more forward-looking backend value.
- **Haskell vs. Erlang.** Elixir and Erlang share the BEAM and OTP. If Erlang replaces Haskell, the BEAM tooling research can be shared.
- **Swift** is in on score, but server-side Swift is a small niche. Its pack would focus on Vapor and Hummingbird.

## Decision (Checkpoint 1, 2026-11-10)

The maintainer chose these for slots 13–15:
- **Elixir**, **Erlang** and **Clojure** replace Perl and Haskell. They are server-first, and Elixir and Erlang can share their BEAM/OTP research.
- **Swift** stays in.

The final 15, with their pack ids:

| # | Language | Pack id |
|---|---|---|
| 1 | Python | `python` |
| 2 | TypeScript / JavaScript | `typescript` |
| 3 | Java | `java` |
| 4 | C++ | `cpp` |
| 5 | C# | `csharp` |
| 6 | PHP | `php` |
| 7 | Go | `go` |
| 8 | Ruby | `ruby` |
| 9 | Swift | `swift` |
| 10 | Kotlin | `kotlin` |
| 11 | Rust | `rust` |
| 12 | Scala | `scala` |
| 13 | Elixir | `elixir` |
| 14 | Erlang | `erlang` |
| 15 | Clojure | `clojure` |

The alternates are Perl, Haskell, OCaml, F# and Gleam.

## Refreshing

Re-run with the newest editions of S1–S3. Record the edition dates, recompute the score with the formula above, and keep the same backend filter unless a language's main use changes.
