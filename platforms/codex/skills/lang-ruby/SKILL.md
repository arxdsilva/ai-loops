---
name: lang-ruby
description: Use when writing, reviewing, testing or setting up Ruby backend code in a loops run (the ruby language pack) - toolchain, layout, race policy, coverage, idioms and footguns.
---

# Ruby backend (pack `ruby`)

Rules: `specs/languages/ruby/RULES.md` (IDs `RUBY-<SEC>-NN`). Pack data: `specs/languages/ruby/pack.json` (`loops lang show ruby`). Ruby 4.0 (3.4 acceptable), Rails 8.1, as of 2026-11.

## When it applies

`loops lang detect` reports `ruby` (a `Gemfile`, `.ruby-version` or `*.gemspec` at the root). Rails, Sinatra, Roda, Hanami and plain gems all use this pack. Read the project's `.ruby-version` instead of assuming a Ruby version (RUBY-TOOL-01, RUBY-REPO-08).

## Project rules first

The repo's own config beats the pack defaults: `.rubocop.yml` or `.standard.yml`, `.rspec`, `sorbet/config` or `Steepfile`, `packwerk.yml`, `.editorconfig`, Rails conventions (RUBY-FW-02). The pack's `config_files` list is what the practices-researcher reads. Log every override: `loops run override --rule "<loops rule>" --project "<project rule, file:line>" --note "<what you did>"` (ROLES.md "Project rules first").

## Toolchain the gate runs

Agents run `loops gate` (WORKFLOW §6), or one check while iterating. Every command goes through `bundle exec` or a binstub, never a global gem (RUBY-TOOL-08). Checks are gated on committed config files (D17); a missing config skips the check.

| Check | Command | Runs when |
|---|---|---|
| `bundle` (deps) | `bundle check` | `Gemfile` exists; verify form, never installs |
| `rubocop` (lint, includes Layout) | `bundle exec rubocop {pkgs}` | `.rubocop.yml` |
| `standard` (lint) | `bundle exec standardrb {pkgs}` | `.standard.yml` (never with rubocop, RUBY-TOOL-02) |
| `sorbet` / `steep` (typecheck) | `bundle exec srb tc` / `bundle exec steep check` | `sorbet/config` / `Steepfile` |
| `packwerk` (arch) | `bundle exec packwerk check` | `packwerk.yml` or `package.yml` |
| `test` | `bundle exec rspec --order random --seed {seed} {pkgs}` (twice) | `.rspec` or `spec/` |
| `coverage` | loops reads `coverage/lcov.info` | after `test` |
| `brakeman` (sast) | `bundle exec brakeman -q --no-pager` | `config/application.rb`; optional |
| `bundler-audit` (vuln) | `bundle exec bundle-audit check --update` | `Gemfile.lock`; off by default (`--vuln`) |

Variant `minitest` (detected by `test/` + `bin/rails`): `test` becomes `bin/rails test --seed {seed} {pkgs}`. Variants are picked from the repo (D17); pin one in `.loops/config.json` `languages` if both `spec/` and `test/` exist. Test commands the guard recognises: `bundle exec rspec`, `rspec`, `bin/rspec`, `bin/rails test`, `rails test`, `bundle exec rake test`, `bundle exec parallel_rspec`.

Post-edit hook: `bundle exec rubocop -a --only Layout <file>` formats one file; `bundle exec rubocop <dir>` lints its directory. Missing tools fail open.

Install hints for optional tools: `bundle add <gem> --group development` (rubocop, standard, sorbet, steep, packwerk, brakeman, bundler-audit). Add one only in wave 0 and record it.

## Race policy

CRuby has no race detector, so `race.mode = none` (WORKFLOW §6, RUBY-CONC-05). The required substitute is the `test` check: random order, the seed printed by the runner, and a second run with a new seed (`repeat: 2`). To satisfy it:

- Never pin a seed, never disable random order (`config.order = :defined`), never add retries that hide a flake.
- A failure that passes on rerun is a bug: report the seed (`Randomized with seed N`) and fix the shared state.
- Stress tests (N threads against one shared object) and parallel runs (`parallel_rspec -n N`, Rails `parallelize`) are review items for code with shared state (RUBY-CONC-02/03).
- The GVL does not make `@x ||= v` or `count += 1` safe (RUBY-CONC-02).

## Coverage recipe

loops parses **lcov** at `coverage/lcov.info` (`./`-prefixed `SF:` paths are normalised). SimpleCov has no native lcov, so the project needs the `simplecov-lcov` gem and a helper that starts SimpleCov first. This is a D13 prerequisite: the architect or `ruby-engineer` adds it in wave 0 (an override, logged); otherwise the gate preflight fails with this snippet.

```ruby
# top of spec/spec_helper.rb (or test/test_helper.rb), before any app code loads
require "simplecov"
require "simplecov-lcov"
SimpleCov::Formatter::LcovFormatter.config do |c|
  c.report_with_single_file = true
  c.single_report_path = "coverage/lcov.info"
end
SimpleCov.formatter = SimpleCov::Formatter::MultiFormatter.new(
  [SimpleCov::Formatter::HTMLFormatter, SimpleCov::Formatter::LcovFormatter]
)
SimpleCov.start do
  enable_coverage :branch
  track_files "{lib,app}/**/*.rb" # without it a never-loaded new file is absent from lcov and the gate cannot see it
  skip "/spec/"; skip "/test/"; skip "/config/"; skip "/db/"; skip "/vendor/"
end
```

New code needs at least 70% coverage (WORKFLOW §6). `skip` replaces the deprecated `add_filter` in SimpleCov 1.x (RUBY-TEST-06). Parallel runs: merge result sets with `SimpleCov.collate` first (RUBY-TEST-07). Details: `testing.md`.

## Layers in Ruby

```text
app/domain/            pure Ruby: entities, Data.define value objects, services (no Rails, no Time.now)
app/application/       use cases with one public `call`, ports as small modules
app/adapters/inbound/  controllers, jobs: thin
app/adapters/outbound/ ActiveRecord repositories, Faraday clients
config/initializers/container.rb   composition root
packs/<name>/package.yml           Packwerk boundaries
```

Constants are autoloaded by Zeitwerk from each `app/*` root; dependencies are constant references, not imports, so `loops arch` delegates to Packwerk (`arch.parser = none`, `native_check = packwerk`). The pack still lists `forbidden_in_domain` (`ActiveRecord`, `ActionController`, `Rails`, `Net::HTTP`, `Faraday`, `ENV`, `File`, `Sidekiq`) and `impure_calls` (`Time.now`, `Time.current`, `Date.today`, `rand`, `SecureRandom`, `Thread.new`, `sleep`, `puts`) for review (RUBY-ARCH-09). Rails conventions win over the hexagon when they conflict; log the override (RUBY-FW-02).

## Cache and cleanup

Per run: `BOOTSNAP_CACHE_DIR` and `TMPDIR`. Shared, never relocated: `BUNDLE_PATH` and `BUNDLE_USER_CACHE` (D15), because a per-run gem dir forces re-downloads (RUBY-OPS-05). Cleanup candidates: `coverage/`, `tmp/`, `log/*.log`, `.rspec_status`, `*.gem`, `.yardoc/`.

## Footguns

1. No race detector: do not assume the GVL protects compound operations.
2. `Marshal.load`/`YAML.load` on untrusted data: use `JSON.parse` or `YAML.safe_load`.
3. SQL interpolation: `where(name: x)`, never `"... #{x}"`.
4. Mass assignment: strong params, never `permit!`.
5. N+1 queries: `includes`, `strict_loading`.
6. Shared mutable state under Puma: class variables, class-level memoisation, mutable constants.
7. `rescue Exception` or an empty `rescue`.
8. Jobs that are not idempotent, or enqueued inside a transaction.
9. Callback side effects and fat models: use explicit use cases.
10. `Hash.new([])` and unfrozen literals: block form, `# frozen_string_literal: true`.

## Load on demand

- `repo.md`: layout, Gemfile and lockfile policy, version pins, CI order.
- `architecture.md`: hexagon in Rails, Zeitwerk, Packwerk, ports and DI.
- `idioms.md`: errors, nil, immutability, pattern matching, style.
- `testing.md`: RSpec or Minitest, SimpleCov and lcov setup, random order, doubles.
- `concurrency.md`: GVL, Puma threads, pools, jobs, Ractors, the race substitute.
- `security.md`: injection, deserialization, strong params, secrets, supply chain.
- `data-api.md`: Active Record, migrations, N+1, HTTP edges, logging, config, deployment.
- `frameworks.md`: Rails, Hanami, Sinatra, Roda, Puma, Sidekiq.
