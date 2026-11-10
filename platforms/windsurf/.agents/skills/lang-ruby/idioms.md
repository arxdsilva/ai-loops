# Ruby idioms

Digest of RUBY-IDIOM. Follow the project's RuboCop or Standard config first.

## Errors (RUBY-IDIOM-01/02)
- Inherit from `StandardError`, keep an app error hierarchy, rescue the narrowest class. Never `rescue Exception` or an empty `rescue`; re-raise with `cause`.
- No exceptions for control flow: return a result object (own `Result` struct or `Dry::Monads::Result`) for expected outcomes (RUBY-ARCH-03). `retry` only with a cap.

## nil and arguments (RUBY-IDIOM-03/12)
- Absence is `nil`: `&.`, `Hash#fetch`, `dig`; never `false` for "not found".
- Keyword arguments for more than 2 params and for booleans; no opts hashes; Ruby 3 has no implicit hash-to-kwargs.

## Immutability and values (RUBY-IDIOM-04/10/11)
- First line `# frozen_string_literal: true`; freeze constants; value objects are `Data.define(:a, :b)`; `Struct` with `keyword_init: true` only when mutation is needed.
- `Hash.new([])` and `Array.new(3, [])` share one object: use the block form. Do not mutate default args.
- Money is `Integer`, `BigDecimal` or `Rational`, never `Float`.

## Control flow and resources (RUBY-IDIOM-05/06/13)
- `case/in` pattern matching for destructuring; add an `else` or accept `NoMatchingPatternError` on purpose.
- Block forms release resources (`File.open(path) { }`, `connection_pool.with { }`) or `ensure`.
- `each`/`map`/`select`/`sum`/`each_with_object`; `find_each` and `in_batches` for large tables; `lazy` for streams.

## Equality, strings, time (RUBY-IDIOM-07)
Override `==`, `eql?` and `hash` together (or use `Data`); build strings with `<<` or `join`; UTF-8 everywhere; `Time.current`/`Time.zone` in Rails, `Time.now.utc` elsewhere; store UTC. Inject the clock (RUBY-ARCH-09).

## Style and metaprogramming (RUBY-IDIOM-08/09)
- snake_case methods, CamelCase classes, predicates end in `?`, `!` only with a non-bang twin. YARD tags on public API (RUBY-DOC-01).
- No monkey-patching core classes, no `method_missing` magic without `respond_to_missing?`, never `eval` user input. Active Support extensions are the accepted exception in Rails.
