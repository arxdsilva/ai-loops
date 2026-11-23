# Repository practices

The universal rules for the repository around the code: layout, manifests and lockfiles, toolchain pins, hygiene files, CI, releases, dependency updates, vulnerability scans and SBOMs. Each rule is stated once here. A pack's `specs/languages/<id>/RULES.md §REPO` (and `§TOOL`, `§SEC`) gives the mechanics; the **Pack rules** line under each section lists the rule IDs, so `PY-REPO-04` is in `specs/languages/python/RULES.md` (Go's `RULES.md` is numbered; its appendix maps the `GO-*` IDs to sections). Cite as `REPO_PRACTICES §3`.

The project's own setup wins over these defaults (WORKFLOW §1). The architect and the `<id>-engineer` own these files (`ROLES.md`); everyone else reports what they need.

## 1. Layout

Use the ecosystem's canonical source, test and resource layout; don't invent one. The hexagonal layers sit inside it (`specs/ARCHITECTURE.md §4`), and the tree per language is in `skills/lang-<id>/architecture.md`.

Pack rules: PY-REPO-01, TS-REPO-01, JAVA-REPO-01, CPP-REPO-01, CS-REPO-01, PHP-REPO-01, GO-REPO-02, RUBY-REPO-01, SWIFT-REPO-01, KT-REPO-01, RUST-REPO-01, SCALA-REPO-01, ELIXIR-REPO-01, ERL-REPO-01, CLJ-REPO-01.

## 2. One repo, one build, until a deployable splits

Default to one module, solution, workspace or project. Split only at a boundary that is released or deployed separately. A split adds a build, a lockfile and a release line; it is never a way to organise code (`specs/ARCHITECTURE.md §15`).

Pack rules: PY-REPO-10, JAVA-REPO-11, CPP-REPO-06, CS-REPO-02, PHP-REPO-01, GO-REPO-03, RUBY-REPO-03, SWIFT-REPO-03, KT-REPO-02, RUST-REPO-04, SCALA-REPO-03, ELIXIR-REPO-01, ERL-REPO-02, CLJ-REPO-16.

## 3. Manifest, lockfile and frozen install

- One manifest declares the direct dependencies, each with a version constraint.
- Applications commit the lockfile. Libraries follow the ecosystem's norm (Rust commits `Cargo.lock` for libraries too). Where the ecosystem has no lockfile, pin exact versions (Clojure).
- CI and the gate install from the lockfile and fail if it is stale (frozen or locked mode). A pack's `deps` check runs only in that form, and first.
- Supply-chain hygiene: lockfile integrity hashes on, registries scoped and named, install scripts off unless a dependency needs them and the project says so.

Pack rules: PY-REPO-04/05, TS-REPO-05/06, JAVA-REPO-06, CPP-REPO-09, CPP-SEC-08, CS-REPO-07, PHP-REPO-04, GO-REPO-07/08, RUBY-REPO-04/05, SWIFT-REPO-05, KT-REPO-06, RUST-REPO-06, SCALA-REPO-06, ELIXIR-REPO-04/05, ERL-REPO-04, CLJ-REPO-05/06, CLJ-SEC-09; supply chain: `<LANG>-SEC` rows in each pack (PY-SEC-08 … CLJ-SEC-09).

## 4. Toolchain pinning

Pin the language and build-tool versions in a committed file, read by both developers and CI: `.tool-versions`, `.python-version`, `rust-toolchain.toml`, `global.json`, the Maven or Gradle wrapper, the `go`/`toolchain` lines in `go.mod`, `.ruby-version`, `Package.swift`'s tools version. The pack's `versions` names the current stable line; the project's pin wins.

Pack rules: PY-REPO-09, TS-REPO-10, JAVA-REPO-03, CPP-REPO-12, CS-REPO-11, PHP-REPO-10, GO-REPO-11, RUBY-REPO-08, SWIFT-REPO-09, KT-REPO-07, RUST-REPO-11, SCALA-REPO-11, ELIXIR-REPO-08, ERL-REPO-09, CLJ-REPO-11.

## 5. Dependency policy

Search in this order before adding code: this repository, the standard library, a well-known and maintained library, then our own code. A new dependency needs a reason in the PR body (what it replaces, its maintenance and licence), and goes only where the layer allows it: never in the domain (`specs/ARCHITECTURE.md §3`). Remove a dependency in the change that makes it unused. Go's version of this rule: `specs/languages/go/RULES.md §48`.

## 6. Dependency updates

- An update bot (Renovate or Dependabot, or the ecosystem's own: Scala Steward, antq) opens grouped update PRs on a schedule.
- Majors are never auto-merged. A cooldown of a few days before adopting a new release lowers supply-chain risk.
- Every update PR runs the full gate; a lockfile-only change is still a change.
- Where no bot supports the ecosystem (rebar3), the `<id>-engineer` runs updates as a language lane.

Pack rules: PY-REPO-11, TS-REPO-07, JAVA-REPO-07, CPP-REPO-10, CS-REPO-09, PHP-REPO-08, GO-REPO-09, RUBY-REPO-07, SWIFT-REPO-07, KT-REPO-11, RUST-REPO-08, SCALA-REPO-07, ELIXIR-REPO-06, ERL-REPO-07, CLJ-REPO-07.

## 7. Versioning, tags, release, provenance

SemVer; releases are `v<x.y.z>` tags; packages go to the ecosystem's registry from CI, with provenance or signatures where the registry supports them. The version lives in one place. A release is reproducible from its tag.

Pack rules: PY-REPO-12, TS-REPO-09, JAVA-REPO-08, CPP-REPO-11, CS-REPO-10, PHP-REPO-09, GO-REPO-10, RUBY-REPO-10, SWIFT-REPO-08, KT-REPO-08, RUST-REPO-09, SCALA-REPO-10, ELIXIR-REPO-07, ERL-REPO-08, CLJ-REPO-10.

## 8. Hygiene files

- `.gitignore` lists the ecosystem's build outputs and caches: the same list as the pack's `artifacts`, which loops cleans up after a run (WORKFLOW §2.6).
- `.editorconfig`, the formatter's and linter's committed configs (the pack's `config_files`), `CODEOWNERS`, a licence, a `README.md`, and an `AGENTS.md` (`templates/ROOT_AGENTS.md`).
- Generated code is marked as generated and never edited by hand.

Pack rules: PY-REPO-13, TS-REPO-11, JAVA-REPO-09, CPP-REPO-13, CS-REPO-04, PHP-REPO-11, GO-REPO-13, RUBY-REPO-11, SWIFT-REPO-10, KT-REPO-09, RUST-REPO-13, SCALA-REPO-12, ELIXIR-REPO-09, ERL-REPO-10, CLJ-REPO-12.

## 9. CI stage order and caching

Every PR runs, in this order: restore (frozen) → format check → lint → typecheck → build → test with coverage and the race policy → vulnerability scan → package. It is the same pipeline as `loops gate` (WORKFLOW §6), so a green gate predicts a green CI.

- One formatter with a committed config; CI and the gate run its check mode.
- The strictest practical compile and type checks: warnings as errors, or the type checker at a stated level.
- Cache the dependency downloads keyed on the lockfile; never cache build outputs across branches.

Pack rules: PY-REPO-15, TS-REPO-12, JAVA-REPO-10, CPP-REPO-14, CS-REPO-14, PHP-REPO-12, GO-REPO-14, RUBY-REPO-13, SWIFT-REPO-11, KT-REPO-10, RUST-REPO-14, SCALA-REPO-14, ELIXIR-REPO-10, ERL-REPO-11, CLJ-REPO-13; formatter and strict checks: `<LANG>-TOOL` rows (PY-TOOL-03/07 … CLJ-TOOL-02/06).

## 10. Conventional commits

Commit subjects follow Conventional Commits (`feat:`, `fix:`, `feat!:` …). They drive the changelog and the release level.

Pack rules: PY-REPO-16, TS-REPO-13, JAVA-REPO-12, CPP-REPO-15, CS-REPO-15, PHP-REPO-14, GO-REPO-15, SWIFT-REPO-12, KT-REPO-12, RUST-REPO-15, SCALA-REPO-15, ELIXIR-REPO-11, ERL-REPO-12, CLJ-REPO-14.

## 11. Vulnerability scan and SBOM

- Scan the lockfile with OSV-Scanner where it reads the ecosystem's lockfile, else with the ecosystem's tool. The pack's `vuln` check is off by default locally (`loops gate --vuln`, `--enable <check>` or `gate.vuln`) and required in CI. A configured vuln check whose tool is missing fails instead of skipping.
- Run the ecosystem's static security analyser (`sast` check) in CI.
- Generate a CycloneDX or SPDX SBOM for every deployable at release.

Pack rules: PY-TOOL-10, TS-TOOL-08, JAVA-TOOL-05/06, CPP-TOOL-09, CS-TOOL-06/07, PHP-TOOL-08/09, GO-TOOL-06/07, RUBY-TOOL-06, SWIFT-TOOL-05, KT-TOOL-05, RUST-TOOL-07, SCALA-TOOL-08, ELIXIR-TOOL-08/09, ERL-TOOL-08, CLJ-TOOL-07/08; SAST: PY-SEC-09 … CLJ-SEC-10.
