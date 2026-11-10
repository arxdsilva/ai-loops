# Ruby security

Digest of RUBY-SEC. Universal rules: `specs/SECURITY.md` (when present). Rails defaults protect only if they are not disabled.

## Injection (RUBY-SEC-01)
- SQL: `where(name: x)`, `where("name = ?", x)`, named binds, `sanitize_sql_like`, an allowlist for `order`/`pluck`/`group` columns. Avoid raw strings in `find_by_sql`, `exists?`, `calculate`.
- Commands: `system("cmd", arg1, arg2)`; never backticks, `system("..#{x}")` or `open(...)` with user data.

## Deserialization (RUBY-SEC-02)
Never `Marshal.load`, `YAML.load`, `Psych.unsafe_load`, `Oj.load` in object mode or `ERB.new(user)` on untrusted data. Use `JSON.parse`, `YAML.safe_load(permitted_classes: [...])`, `config.active_record.use_yaml_unsafe_load = false`.

## Input and authorization (RUBY-SEC-03/06)
Strong params (`params.require(:x).permit(...)`); never `permit!` or `update(params[:x])`; do not permit `admin`, `role` or `*_id` ownership fields; scope lookups to the user (`current_user.orders.find`). Validate shape at the boundary (dry-schema or form objects); model validations are not authorization.

## Web (RUBY-SEC-04)
Keep CSRF protection, escape output (avoid `raw`, `html_safe`, loose `sanitize`), `config.force_ssl`, `config.hosts`, a CSP, SSRF-safe outbound HTTP (allowlist hosts, check resolved IPs, no redirects to private ranges), `redirect_to` only with `only_path` or allowlisted URLs.

## Secrets and crypto (RUBY-SEC-05)
Encrypted credentials (`config/credentials/*.yml.enc`, key never committed) or env vars; `config.filter_parameters`. `SecureRandom`, `bcrypt`/`has_secure_password`/argon2, Active Record Encryption. No `rand`, MD5 or SHA1 for passwords; `ActiveSupport::SecurityUtils.secure_compare` for secrets.

## Supply chain and scanners (RUBY-SEC-07/08, RUBY-TOOL-06)
- Commit `Gemfile.lock`, use `https://rubygems.org` only, no forked `git:` gems, MFA on own gems, review native-extension gems.
- `brakeman -q --no-pager` (the `brakeman` check, optional) with a reviewed `config/brakeman.ignore`; `bundle-audit check --update` (the `bundler-audit` check, off by default; enable with `--vuln`).
