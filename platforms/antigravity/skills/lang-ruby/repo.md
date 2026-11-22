# Ruby repo and project management

Digest of `specs/languages/ruby/RULES.md` REPO and TOOL. Universal repo rules: `specs/REPO_PRACTICES.md` (when present).

## Layout
- Rails service: generate with `rails new <app> --api -d postgresql` and keep the generated tree (RUBY-REPO-01).
- Non-Rails: Bundler gem layout, `lib/<name>.rb` plus `lib/<name>/`, `spec/` or `test/`, `exe/` (RUBY-REPO-02).
- One app with internal boundaries (Packwerk packs, engines) beats many repos; one root `Gemfile.lock` (RUBY-REPO-03).

## Pins and lockfile
- `.ruby-version` is the single Ruby pin; the Gemfile reads it with `ruby file: ".ruby-version"` (RUBY-REPO-08). Do not start on 3.3 or older (RUBY-TOOL-01).
- Commit `Gemfile.lock` for applications; gems test against a lockless matrix too (RUBY-REPO-04).
- CI installs frozen (`BUNDLE_FROZEN=true bundle install`); the loops gate runs the verify form `bundle check` (RUBY-REPO-05, D11).
- Pessimistic constraints (`~> 8.1.0`); never `git:` without `ref:` (RUBY-REPO-06). Dependabot for `bundler` and `github-actions` (RUBY-REPO-07).

## Files the engineer owns in wave 0
- `Gemfile`, `Gemfile.lock`, `.ruby-version`, `.rubocop.yml` or `.standard.yml`, `.rspec`, `sorbet/config` or `Steepfile`, `packwerk.yml`, `.editorconfig`, `.github/dependabot.yml`.
- `.gitignore` covers `/log/*`, `/tmp/*`, `/coverage`, `/.bundle`, `/vendor/bundle`, `config/*.key`, `.env*` (RUBY-REPO-11).
- The SimpleCov helper (D13, `testing.md`).
- Add a gem only with a reason recorded in the claims report; install through `bundle add`, then commit the lockfile.

## Toolchain
- One of RuboCop or Standard, never both; check-only in the gate, autocorrect with `-a` locally and review `-A` (RUBY-TOOL-02/03).
- Department gems match the stack: `rubocop-rails`, `rubocop-rspec` or `rubocop-minitest`, `rubocop-performance` (RUBY-TOOL-04).
- Typing: Sorbet or RBS+Steep, never both; the checker runs in CI and strictness only rises (RUBY-TOOL-05).
- `bundle exec` or binstubs for every tool (RUBY-TOOL-08). Ruby LSP for editors (RUBY-TOOL-07).
- `.editorconfig`: UTF-8, LF, 2 spaces; `# frozen_string_literal: true` first line (RUBY-REPO-12).

## CI order (RUBY-REPO-13)
frozen install -> rubocop -> brakeman -> bundle-audit -> typecheck -> packwerk -> `db:test:prepare` -> tests with SimpleCov -> upload `coverage/`. Rails 8.1 ships `config/ci.rb` and `bin/ci`.

## Releases
- Gems: `vX.Y.Z` tag, trusted publishing, `rubygems_mfa_required` (RUBY-REPO-09); SemVer and `CHANGELOG.md` (RUBY-REPO-10).
- CI matrix: newest plus oldest supported Ruby for gems; the pinned one for apps (RUBY-REPO-14).
