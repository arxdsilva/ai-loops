# Erlang idioms

Digest of `specs/languages/erlang/RULES.md` IDIOM.

## Errors (`ERL-IDIOM-01..03`)
- Return `{ok, V}` or `{error, Reason}` for expected outcomes; crash (match failure, `error/1`) on programmer errors and corrupt state. No defensive code for unspecified input.
- Avoid `catch Expr` and `try ... catch _:_ -> ok`: catch a specific `Class:Reason:Stacktrace` and re-raise with `erlang:raise/3`.
- Cleanup with `try ... after`, monitors or links; every socket, port or file has an owner process.

## Data and types (`ERL-IDIOM-04..08`)
- Absence is explicit (`undefined`, `error`, `{ok, V}`); `maps:find/2` or `maps:get/3` with a default.
- Immutable records or maps with `-type`; one `.hrl` per app, never shared across apps.
- `-spec` on every exported function, `-type` for exported shapes, `-opaque` for value objects.
- Pattern matching in heads and guards; strict generators (`<:-`, OTP 28) so a non-matching element crashes. Old guard tests go in OTP 30.
- Strings are binaries (`<<"x"/utf8>>`) in services; use `string` and `uri_string`; build output as iolists.

## Time, naming, macros (`ERL-IDIOM-09..11`)
- `erlang:system_time/1`, `monotonic_time/1`, `calendar:universal_time()`; inject a clock; no `now/0` in the domain.
- `snake_case` modules, functions and atoms; `CamelCase` variables; module prefix per app; 100 columns; `-moduledoc` and `-doc` on public modules.
- Macros only for constants, logging and `?assert*`; no parse transforms, no `-compile(export_all)`.

## Footguns (`ERL-IDIOM-12/13`)
- Never create atoms from input: the table holds 1,048,576 entries and is never collected.
- `++` with a growing left side is O(n): cons and `lists:reverse/1`; `maps` for lookup, not `proplists` or `lists:keyfind` on large lists.
