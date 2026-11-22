# Idioms (Elixir)

Digest of RULES.md IDIOM.

## Errors (`ELIXIR-IDIOM-01`, `-02`, `-03`)
- Expected failure: `{:ok, v}` / `{:error, reason}`; offer `foo/1` and `foo!/1`. Raise only for programmer errors.
- Let it crash: handle only errors you can act on; never `rescue _` without re-raise or logging.
- Normalise error shapes in a small private function; `{:error, %MyApp.Error{}}` or atoms such as `:not_found`, not strings. No complex `with ... else`.

## Access and data (`ELIXIR-IDIOM-04`, `-05`, `-06`)
- Required key `map.key` or `Map.fetch!/2`; optional `map[:key]`; no truthiness on non-booleans.
- Structs with `@enforce_keys` for domain values; do not mix maps and structs for one concept.
- Multi-clause functions and guards; no catch-all `_` that hides new cases.

## Types and docs (`ELIXIR-IDIOM-07`, `ELIXIR-TOOL-06`)
`@spec` on public functions, `@type`/`@typep` for data, `@behaviour` + `@impl true`. The compiler's type system ignores specs; Dialyzer reads them. Fix compiler type warnings instead of working around them.

## Resources (`ELIXIR-IDIOM-08`)
`File.open/2` with a function; supervisors for long-lived processes; `Task.Supervisor` for fire-and-forget; link or monitor what you spawn.

## Text, time, money (`ELIXIR-IDIOM-09`)
`String.*` for text; iodata in hot paths; `DateTime` in UTC with a tz database, never `NaiveDateTime` for instants; `Decimal` for money.

## Style (`ELIXIR-IDIOM-10`, `-12`)
`snake_case` functions, `CamelCase` modules, `?` booleans, `is_` guards, `!` raising. Directive order: `@moduledoc`, `@behaviour`, `use`, `import`, `require`, `alias`, attributes, `defstruct`, `@type`, `@callback`. Pipe chains of two or more calls from a bare value; never a single pipe. `mix format` does not reorder directives; Credo checks the rest.

## Metaprogramming (`ELIXIR-IDIOM-11`)
Macros only when a function cannot do it; a small explicit `__using__`; never create atoms from external input.

## Performance traps (`ELIXIR-PERF-04`)
`++` on large lists in loops, `Enum.at`/`length` in loops, N+1 queries, `Process.sleep` in requests, large messages. Benchmark with Benchee before tuning (`ELIXIR-PERF-01`); profile with `mix profile.tprof` (`ELIXIR-PERF-02`).
