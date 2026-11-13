# Roles

Each role is a plugin agent in `agents/<role>.md`, spawned as `loops:<role>` on Claude Code and through each platform's own mechanism elsewhere (`skills/work/platforms/<platform>.md`, `specs/PLATFORMS.md §4`). Its frontmatter `model` is the default tier. The lead passes `model` only to escalate a role (WORKFLOW §4) or under `--models=cheap`. A guard test keeps this table in sync with the agents' frontmatter.

The "Owns (default)" column is the default scope, written for the layer homes of `specs/ARCHITECTURE.md §4` (Go paths shown; each language's tree is in `skills/lang-<id>/architecture.md`). The plan narrows it per agent with explicit globs, and every implementer's file ownership is disjoint from every other implementer's.

| Role | Phase | Model | Edits code | Owns (default) | Can | Cannot |
|---|---|---|---|---|---|---|
| web-researcher | research | sonnet | no | `research/web.md` | WebSearch, WebFetch, ai-memory query | Edit the repo |
| practices-researcher | research | sonnet | no | `research/practices.md` | Read plugin specs, skills and repo docs, the detected packs' `config_files`; ai-memory query; official language docs | Edit the repo |
| code-researcher | research | haiku | no | `research/code.md`, `research/language.json` | `loops lang detect --write`; grep, find, ranged Read, LSP; ai-memory query | Edit the repo, judge correctness |
| hardware-researcher | research | haiku | no | `research/hardware.{json,md}` | Run `loops hw --write`, then reply with the one-line recommendation | Anything else (3 turns max) |
| planner | plan | opus | no | `plan.v<N>.md` | Read the briefs and repo; write the plan | Edit code |
| plan-reviewer | plan | opus | no | `plan-review.r<N>.md` | Read; write the review | Edit the plan or code |
| architect | implement (wave 0) | opus | yes | Interfaces and ports, typed commands and DTOs, stubs, the composition root (`cmd/**`, `internal/compose/**`), Makefile; manifests, lockfiles and CI when no `<id>-engineer` is in wave 0 | Add dependencies (Go: `go get`, `go mod tidy`); shared config | Implement behaviour beyond stubs |
| domain-engineer | implement | sonnet | yes | `internal/domain/**` | Pure rules, entities, value objects, domain services and their tests | Import adapters, application, the pack's `forbidden_in_domain`, third-party code; the pack's `impure_calls` (wall clock, global RNG, spawning work) |
| application-engineer | implement | sonnet | yes | `internal/application/**` | Use cases, orchestration, consumer-owned ports, and their tests with fakes | Import adapters; put business rules here |
| api-engineer | implement | sonnet | yes | `internal/adapters/inbound/**` | HTTP/gRPC/CLI handlers, request validation, DTO↔command mapping, error→status mapping | Mutate domain state outside commands; leak transport types inward |
| persistence-engineer | implement | sonnet | yes | `internal/adapters/outbound/{db,store,repo,cache,queue}*/**`, `migrations/**` | Repositories, migrations, caches, queue producers and consumers, and their integration tests | Leak driver or DB types past a port; write a migration without a version |
| integration-engineer | implement | sonnet | yes | `internal/adapters/outbound/<client>/**` | Clients for external APIs and brokers, retries and timeouts, contract tests with fakes | Leak library types past a port |
| test-engineer | implement | sonnet | yes | The pack's `test_globs` (Go: `**/*_test.go`) and fixtures within its scope, `test/**`, benchmarks | Integration, e2e, fuzz and benchmark tests; fixtures | Weaken or delete a test; edit non-test code |
| python-engineer | implement (wave 0, language lanes) | sonnet | yes | `pyproject.toml`, `uv.lock`/`requirements*.txt`, `.python-version`, ruff, mypy and pytest config; CI for its pack; test and coverage prerequisites | Frozen installs and lock updates, toolchain pins and upgrades, dependency bumps, build migrations; idiom review when the lead asks (`loops:lang-python`) | Implement layer behaviour; edit another pack's files |
| typescript-engineer | implement (wave 0, language lanes) | sonnet | yes | `package.json`, the lockfile, `tsconfig*.json`, `.nvmrc`/`.node-version`, eslint, prettier and vitest config; CI for its pack; test and coverage prerequisites | Frozen installs and lock updates, toolchain pins and upgrades, dependency bumps, build migrations; idiom review when the lead asks (`loops:lang-typescript`) | Implement layer behaviour; edit another pack's files |
| java-engineer | implement (wave 0, language lanes) | sonnet | yes | `pom.xml` or `build.gradle*`, the wrapper, `.tool-versions`, Spotless, Checkstyle, SpotBugs, JaCoCo config; CI for its pack; test and coverage prerequisites | Frozen installs and lock updates, toolchain pins and upgrades, dependency bumps, build migrations; idiom review when the lead asks (`loops:lang-java`) | Implement layer behaviour; edit another pack's files |
| cpp-engineer | implement (wave 0, language lanes) | sonnet | yes | `CMakeLists.txt`, `CMakePresets.json`, `vcpkg.json`/`conanfile.*` and lock, `.clang-format`, `.clang-tidy`; CI for its pack; test and coverage prerequisites | Frozen installs and lock updates, toolchain pins and upgrades, dependency bumps, build migrations; idiom review when the lead asks (`loops:lang-cpp`) | Implement layer behaviour; edit another pack's files |
| csharp-engineer | implement (wave 0, language lanes) | sonnet | yes | `*.sln[x]`, `*.csproj`, `Directory.Build.props`, `Directory.Packages.props`, `packages.lock.json`, `global.json`, `.editorconfig`; CI for its pack; test and coverage prerequisites | Frozen installs and lock updates, toolchain pins and upgrades, dependency bumps, build migrations; idiom review when the lead asks (`loops:lang-csharp`) | Implement layer behaviour; edit another pack's files |
| php-engineer | implement (wave 0, language lanes) | sonnet | yes | `composer.json`, `composer.lock`, `phpunit.xml*`, `phpstan.neon*`, `.php-cs-fixer*.php`, deptrac config; CI for its pack; test and coverage prerequisites | Frozen installs and lock updates, toolchain pins and upgrades, dependency bumps, build migrations; idiom review when the lead asks (`loops:lang-php`) | Implement layer behaviour; edit another pack's files |
| go-engineer | implement (wave 0, language lanes) | sonnet | yes | `go.mod`, `go.sum`, `go.work`, `.golangci.*`, `tools` directives; CI for its pack; test and coverage prerequisites | Frozen installs and lock updates, toolchain pins and upgrades, dependency bumps, build migrations; idiom review when the lead asks (`loops:lang-go`) | Implement layer behaviour; edit another pack's files |
| ruby-engineer | implement (wave 0, language lanes) | sonnet | yes | `Gemfile`, `Gemfile.lock`, `.ruby-version`, `.rubocop.yml`, `.rspec`, SimpleCov setup, packwerk config; CI for its pack; test and coverage prerequisites | Frozen installs and lock updates, toolchain pins and upgrades, dependency bumps, build migrations; idiom review when the lead asks (`loops:lang-ruby`) | Implement layer behaviour; edit another pack's files |
| swift-engineer | implement (wave 0, language lanes) | sonnet | yes | `Package.swift`, `Package.resolved`, `.swift-format`, `.swiftlint.yml`; CI for its pack; test and coverage prerequisites | Frozen installs and lock updates, toolchain pins and upgrades, dependency bumps, build migrations; idiom review when the lead asks (`loops:lang-swift`) | Implement layer behaviour; edit another pack's files |
| kotlin-engineer | implement (wave 0, language lanes) | sonnet | yes | `settings.gradle.kts`, `build.gradle.kts`, `gradle.lockfile`, version catalog, the wrapper, `junit-platform.properties`, detekt and ktlint config; CI for its pack; test and coverage prerequisites | Frozen installs and lock updates, toolchain pins and upgrades, dependency bumps, build migrations; idiom review when the lead asks (`loops:lang-kotlin`) | Implement layer behaviour; edit another pack's files |
| rust-engineer | implement (wave 0, language lanes) | sonnet | yes | `Cargo.toml`, `Cargo.lock`, `rust-toolchain.toml`, `clippy.toml`, `rustfmt.toml`, `deny.toml`; CI for its pack; test and coverage prerequisites | Frozen installs and lock updates, toolchain pins and upgrades, dependency bumps, build migrations; idiom review when the lead asks (`loops:lang-rust`) | Implement layer behaviour; edit another pack's files |
| scala-engineer | implement (wave 0, language lanes) | sonnet | yes | `build.sbt`, `project/**`, `.scalafmt.conf`, `.scalafix.conf`; CI for its pack; test and coverage prerequisites | Frozen installs and lock updates, toolchain pins and upgrades, dependency bumps, build migrations; idiom review when the lead asks (`loops:lang-scala`) | Implement layer behaviour; edit another pack's files |
| elixir-engineer | implement (wave 0, language lanes) | sonnet | yes | `mix.exs`, `mix.lock`, `.tool-versions`, `.formatter.exs`, `.credo.exs`, `coveralls.json`; CI for its pack; test and coverage prerequisites | Frozen installs and lock updates, toolchain pins and upgrades, dependency bumps, build migrations; idiom review when the lead asks (`loops:lang-elixir`) | Implement layer behaviour; edit another pack's files |
| erlang-engineer | implement (wave 0, language lanes) | sonnet | yes | `rebar.config`, `rebar.lock`, `.tool-versions`, `elvis.config`, covertool settings; CI for its pack; test and coverage prerequisites | Frozen installs and lock updates, toolchain pins and upgrades, dependency bumps, build migrations; idiom review when the lead asks (`loops:lang-erlang`) | Implement layer behaviour; edit another pack's files |
| clojure-engineer | implement (wave 0, language lanes) | sonnet | yes | `deps.edn` or `project.clj`, `tests.edn`, `:cov`/`:test` aliases, `.cljfmt.edn`, `.clj-kondo/config.edn`; CI for its pack; test and coverage prerequisites | Frozen installs and lock updates, toolchain pins and upgrades, dependency bumps, build migrations; idiom review when the lead asks (`loops:lang-clojure`) | Implement layer behaviour; edit another pack's files |
| docs-writer | implement | haiku | docs only | Package `AGENTS.md`, `README.md` sections, `docs/adr/**` | Write docs from facts the lead supplies | Edit code; invent facts |
| qa-verifier | qa | sonnet | no | `qa/<agent-id>.r<N>.md`; as `qa-quality`, `qa/quality.r<N>.md` | Read; run tests, `loops gate --only <test checks>,coverage --pkgs <owned>`, `loops arch`; check test honesty (`specs/ENGINEERING.md §12`); as `qa-quality`, run crapper, dryer and mutator over the run's changed code (`loops:quality`); put counterexample test snippets in its QA file (never in the repo). Runs tools hook-scoped or through `loops exec` (Go: `loops go`), never sets cache variables, leaves no binaries | Edit repo code (it reports; the owner fixes). It FAILS a claim set when new-code coverage is below 70% or new code lacks tests, and puts the uncovered `file:line` list in the repro |
| security-reviewer | qa | sonnet | no | `qa/security.r<N>.md` | Read; run `loops gate --security`, `loops lang show <id>`; scan the run base with `git archive` to tell new findings from old; map findings to lanes | Edit repo code; install tools; block on a missing tool |
| product-reviewer | product | opus | no | `proofs/**`, `pr.md`, `report.html` | Run the service; curl, grpcurl, CLI; `loops report`; branch, commit, push; `gh` or `glab` PR when the lead's `PR:` line is `open` or `draft` | Edit code (unmet ACs go back); push to the main line, force-push, merge |
| reflector | reflect | sonnet | no | `reflection.md` | Read the run dir; ai-memory query and write | Edit code; store secrets in memory |

## Language × layer

There are two kinds of implementer. **Layer roles** (architect, domain, application, api, persistence, integration, test, docs) own a hexagonal layer in any language. **Language roles** (`<id>-engineer`, one per pack in `specs/languages/`) own their pack's toolchain and repo files (`specs/REPO_PRACTICES.md`): with the architect in wave 0, and as language lanes later (version upgrades, dependency bumps, build migrations).

Every implementer lane is a layer role plus a language pack. The lead adds `Language: <id>` to the spawn prompt (WORKFLOW §3), from `research/language.json`, and the agent loads `loops:lang-<id>` first for the toolchain, race policy, coverage recipe, layout and idioms. In a repo with several packs, each lane has one language; its owned globs stay inside that pack's root.

## The lead and the charter (every role)

The lead is the main session, not a plugin agent: the run's master agent, which owns every phase, the scope and every decision (WORKFLOW §2.7). Every role:
- reads `charter.md` (the `Charter:` header line) first: the goal, the ACs, what is in and out of scope;
- puts anything it finds outside the scope in its report's `Out` section, and never does it;
- never asks the user. When it needs a decision, it takes the safest option inside the scope and names it in its report with the alternatives; the lead confirms or reverses it (`loops run decide`).

## Project rules first (every role)

The target project's own rules always take precedence over loops rules (WORKFLOW §1). When you follow one instead of a loops rule, record it right away with `loops run override --rule … --project … --note …`. The reflector reports every override to the user as a PR comment.

## Rules shared by every implementer (architect and the engineers)

These come from rimworld-go PARALLEL_WORK §3.

- **Strict TDD, tests for all new code.** Write the failing test, make it pass, then refactor. Never weaken or delete a test to get green. New code ships with unit tests and at least 70% coverage of new code (`specs/TESTING.md §8.1`); every test run uses the race detector or the pack's race substitute (WORKFLOW §6). Architect stubs are exempt; the implementers' tests cover them later.
- **Run-scoped environment.** Run your pack's tools normally; the `toolchain-env` hook scopes them to the run's caches (WORKFLOW §6). Where the hook denies with a hint, use `loops exec -- <cmd>` (Go: `loops go …`). Never set cache variables yourself. Don't leave binaries or test artifacts around: build into the run's tmp dir, never into the repo.
- **Targeted checks only:** the pack's test command narrowed to your owned scope, with its `env_hints` (Go: `GOFLAGS=-p=2 go test -race -count=1 -cover ./<owned>/...` and `go vet ./<owned>/...`). Never the whole suite (`go test ./...`, `make all`); the lead and QA run the full gates.
- **Ownership.**
  - Never edit outside your owned globs.
  - Report failures you see in other agents' files instead of fixing them.
  - Never touch manifests, lockfiles (Go: `go.mod`/`go.sum`) or shared config. Only the architect and the `<id>-engineer` do; everyone else reports missing deps.
  - No `git add`/`commit`/`push`. Only product-reviewer commits, when the lead sends the go.
- **Producers not done?** Develop against the contract and stubs, then report what is still stubbed.
- **Paths.** Use absolute paths; cwd may change between calls.
- **Report.** Finish with a claims report (`templates/CLAIMS_REPORT.md`, at most 400 words). Every claim names its evidence.

## Model tiers

These come from rimworld-go PARALLEL_WORK §6.

| Tier | Model | Use for |
|---|---|---|
| 1 | haiku | Mechanical, fully specified work: locating code, inventories, docs from supplied facts |
| 2 | sonnet | Implementation against a written contract, tests, adapters, QA of ordinary code |
| 3 | opus | Planning and plan review, contracts, consistency and concurrency, migrations, product judgement |

Escalate one tier after two failed QA rounds (WORKFLOW §4). Never start higher "to be safe".

**When a model isn't available.** The tier is a preference, not a hard requirement (WORKFLOW §3a).
- If the platform rejects a role's model (not available on the user's plan, not enabled, or over capacity), re-spawn the same role with the best model the user does have.
- **Reasoning roles** (planner, plan-reviewer, architect, product-reviewer, and any escalated role) take the strongest reasoning model available: opus, then sonnet, then the session model, or the platform's equivalent.
- **Other roles** step down one tier at a time.
- Never stop the run because a preferred model is missing.
