# Erlang frameworks

Digest of `specs/languages/erlang/RULES.md` FW and API. The project's framework and layout win (`ERL-FW-02`).

## Choices (`ERL-FW-01`)
- Cowboy 2.21 is the de-facto base and the default for hexagonal services. Elli is small and HTTP/1.1 only. Nova (0.18) is MVC-like on Cowboy. Phoenix lives on the Elixir side (see `lang-elixir`).

## Cowboy in a hexagon (`ERL-FW-02`)
- Routes and handlers live in the inbound adapter app; a handler decodes, calls a use case, encodes. The release app starts the Ranch listener.
- Nova imposes `src/controllers` and views: keep use cases outside and call them from controllers. Follow the Nova layout, log the override.

## Build tool (`ERL-FW-03`)
`erlang.mk` is a Makefile alternative to rebar3 (targets for EUnit, CT, PropEr, Xref, Dialyzer, cover, relx). Use whichever the repo has; a mixed repo needs both lock and Makefile checks. The pack's argv are rebar3: on an `erlang.mk` project, run the Makefile targets and log an override.

## Footguns (`ERL-FW-04`)
- A crashing handler only kills the request process, but a crash in long-lived `init/2` state leaks.
- `cowboy_rest` defaults surprise (for example `content_types_accepted`): read the callback table.
- Ranch `max_connections` defaults to 1024; WebSocket handlers need ping and idle timeouts.
- Set listener limits (`ERL-API-01`) and stop listeners before workers on shutdown (`ERL-OPS-04`).
