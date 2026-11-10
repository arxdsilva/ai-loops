# Ruby architecture

Digest of RUBY-ARCH and RUBY-FW. The neutral rules are `specs/ARCHITECTURE.md` and `specs/SOLID.md`; this file only maps them to Ruby.

## Choose the style (RUBY-ARCH-01)
- CRUD-heavy: the Rails way is fine. Rich domain: hexagon with plain Ruby objects. Say which in an ADR; project rules win.
- Alternatives closer to the hexagon: Hanami 2, Roda, Sinatra with your own structure (RUBY-FW-03).

## Mapping
| Loops layer | Ruby |
|---|---|
| domain | `app/domain/**`: plain classes, `Data.define` value objects, errors; no Rails, no I/O |
| application | `app/application/**`: use cases with one public `call`, ports as small modules (RUBY-ARCH-03, 10) |
| inbound adapters | controllers and jobs: parse, authorize, one `call`, render (RUBY-ARCH-02) |
| outbound adapters | ActiveRecord repositories, Faraday clients (RUBY-ARCH-04) |
| composition | initializer or `Dry::Container` (RUBY-ARCH-08) |

Each `app/*` folder is an autoload root, so `app/domain/billing/invoice.rb` defines `Billing::Invoice` (RUBY-ARCH-06).

## Boundaries
- `private` is per class and bypassable; use `private_constant` and Packwerk (RUBY-ARCH-05).
- Packwerk: `bundle exec packwerk check` and `validate`; each `package.yml` sets `enforce_dependencies: true`; privacy needs `packwerk-extensions`; commit `package_todo.yml` and only shrink it (RUBY-ARCH-07).
- Packwerk sees constants, not method calls. It is the pack's `native_check`; `loops arch` does not scan Ruby imports.

## Purity (RUBY-ARCH-09)
The domain avoids `ActiveRecord::`, `ActionController::`, `Rails.`, `ENV`, `Net::HTTP`, `File`, `Time.now`, `Date.today`, `Time.current`, `rand`, `SecureRandom`, `Thread`, `sleep`, `puts`. Inject a clock and an id generator. These lists are in the pack for review; the gate does not scan them.

## Dependency injection (RUBY-ARCH-08)
Keyword arguments with defaults: `def initialize(clock: Clock.new, repo: InvoiceRepo.new)`. No service locators, no global singletons.

## Active Record
- It merges entity, repository and persistence. For a rich domain map rows to domain objects in a repository; for a simple one use models directly if rules stay in plain objects (RUBY-ARCH-04).
- Avoid callbacks that call services or send mail; use a use case or a domain event after commit (RUBY-ARCH-12).

## Size limits (RUBY-ARCH-11)
Methods about 10 lines, classes about 100, at most 4 params; enforced by RuboCop `Metrics/*`. Follow the project's configured limits first.

## Rails overrides (RUBY-FW-02)
Zeitwerk roots, `app/models`, `db/migrate`, committed schema, `spec/` or `test/`. Where the hexagon conflicts, follow Rails and log the override.
