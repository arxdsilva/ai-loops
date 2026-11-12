# Ruby frameworks

Digest of RUBY-FW and the framework-facing rules. Project conventions win; log overrides with `loops run override`.

## Versions (RUBY-FW-01)
Rails 8.1 is current (bug fixes about 2026-10, security about 2027-10); 8.0 security about 2026-11; 7.2 is EOL (about 2026-08). Jobs: Sidekiq or Solid Queue (Rails 8 default). Servers: Puma (default), Falcon, Unicorn. Dates derive from the Rails maintenance policy.

## Rails
- Keep the generated layout and Zeitwerk roots; `bin/rails zeitwerk:check` in CI (RUBY-FW-02, RUBY-FW-04).
- Thin controllers, use cases for orchestration (RUBY-ARCH-02/03). Callbacks without side effects (RUBY-ARCH-12).
- Footguns (RUBY-FW-04): `default_scope`, callbacks with side effects, fat controllers and models, `update_attribute`/`update_column` skipping validations (`Rails/SkipsModelValidations`), unchecked `find_by` nil, `Time.now` instead of `Time.current`, `rescue_from` swallowing errors.
- Tests: RSpec with `rspec-rails`, or Minitest with `bin/rails test` (the `minitest` variant). Both need the SimpleCov helper (`testing.md`).
- Rails 8.1 ships `config/ci.rb` and `bin/ci`; `rails new` generates a Dockerfile and Kamal config.

## Hanami, Roda, Sinatra (RUBY-FW-03)
Hanami 2 gives slices, containers, repositories (ROM), operations and `Deps[]` injection: close to the hexagon. Roda and Sinatra impose nothing, so add the layers yourself (`architecture.md`).

## Puma and Sidekiq (RUBY-FW-05)
- Puma: workers = cores, `threads 3,3` (5 is typical), `preload_app!` with DB reconnect in `on_worker_boot`.
- Sidekiq: `concurrency` <= DB pool, weighted queues, idempotent retries (RUBY-CONC-04/08).

## Typing and linting add-ons
Sorbet with Tapioca or RBS with Steep, never both (RUBY-TOOL-05); `rubocop-rails` and friends (RUBY-TOOL-04); Standard as a zero-config alternative (RUBY-TOOL-02).
