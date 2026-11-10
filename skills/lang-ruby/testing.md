# Ruby testing and coverage

Digest of RUBY-TEST. Universal testing rules: `specs/TESTING.md` (when present). Skill `loops:testing` has the loops flow; this file has the Ruby specifics.

## Framework and layout (RUBY-TEST-01/02)
- RSpec (`spec/**/*_spec.rb`, helpers `spec_helper.rb`/`rails_helper.rb`) or Minitest (`test/**/*_test.rb`, `test_helper.rb`). One per project, never mixed.
- The pack's base is RSpec (`.rspec` or `spec/`); the `minitest` variant is `test/` + `bin/rails`.
- Quick loops: `bundle exec rspec path/to/file_spec.rb:LINE`, `bin/rails test test/models/user_test.rb:12`, `--only-failures`, `--fail-fast` (RUBY-TEST-10). Lanes run the files they own; the lead and QA run the full gate (`ROLES.md`).

## Random order and the race substitute (RUBY-TEST-09, RUBY-CONC-05)
- The gate runs `--order random --seed {seed}` twice (`repeat: 2`). Minitest and Rails randomise by default and accept `--seed`.
- RSpec: `config.order = :random`, `config.warnings = true`; reuse a failing seed with `--seed N`.
- Freeze time with `travel_to` or `Timecop`, inject clocks, seed `Random.srand`.
- Parallelism: `parallelize(workers: :number_of_processors)` (Rails) or `parallel_tests` with one DB per process via `TEST_ENV_NUMBER`.

## Coverage prerequisite (D13, RUBY-TEST-06)
Add the helper snippet from `SKILL.md` (Coverage recipe) to the top of the spec or test helper, plus `gem "simplecov", require: false` and `gem "simplecov-lcov", require: false` in the `:test` group. The helper must load SimpleCov before any app code. The gate reads `coverage/lcov.info`; a missing or empty file fails the coverage check with this snippet.
- SimpleCov 1.x DSL: `cover`, `skip`, `group`; `add_filter` is deprecated. Use `cover` for files the suite never loads.
- `# :nocov:` only with a justification comment (RUBY-TEST-12). Exclude generated code (`db/schema.rb`, migrations, vendor) with `skip`.
- Parallel workers write separate result sets; `SimpleCov.collate Dir["coverage/.resultset*.json"]` merges them (RUBY-TEST-07).
- Both gate runs write the same `coverage/lcov.info`; SimpleCov merges them, which is fine for a union.
- New code needs at least 70% coverage; `undercover --compare <base>` is a project-side check that reads `coverage/coverage.json`.

## Doubles and boundaries (RUBY-TEST-04/05)
- Fakes and in-memory adapters first. `instance_double` with `verify_partial_doubles` so doubles fail when the real method changes. WebMock or VCR at the HTTP boundary. Never mock the object under test or Active Record chains.
- Real database for integration tests (transactional tests by default); Postgres in CI, not SQLite. DatabaseCleaner only for non-transactional system tests.

## Style (RUBY-TEST-03/11)
`describe`/`context`/`it` with behaviour names, one concept per example, lazy `let`, sparing `subject`, shared examples for port contracts. Factories (`factory_bot`) or fixtures, consistently; no deep association graphs by default; `FactoryBot.lint` in CI.

## Property and mutation tests (RUBY-TEST-08)
`rantly` or `propcheck` rarely; `mutant` on pure domain code only; `benchmark-ips` for performance assertions (RUBY-PERF-01).
