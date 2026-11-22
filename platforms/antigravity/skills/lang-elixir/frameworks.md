# Frameworks (Elixir)

Digest of RULES.md FW, plus framework facts the roles need.

## Detection (`ELIXIR-FW-01`)
Phoenix: `{:phoenix, ...}` in `mix.exs` and `lib/*_web/`. Also look for `ecto`, `oban`, `broadway`, `absinthe`, `commanded`, `nerves`. Phoenix 1.8 (Bandit default), Ecto 3.1x, LiveView, Plug.

## Phoenix conventions win (`ELIXIR-FW-02`)
Generators (`phx.gen.context`, `phx.gen.live`, `phx.gen.auth`) produce `lib/<app>/` contexts, `lib/<app>_web/`, `test/support/{data_case,conn_case}.ex`. Keep them unless the project says otherwise. The generated contexts call `Repo` and return schemas; the hexagonal layout is an opt-in refinement (schemas to adapters, behaviour ports). Log the choice with `loops run override` if the project's convention differs from the plan.

## Boundaries inside Phoenix (`ELIXIR-FW-03`, `-04`, `ELIXIR-ARCH-11`)
- No business logic in controllers, LiveView callbacks or schemas; LiveViews hold UI state and call contexts; do not `assign` large structs.
- Changesets cast and validate; cross-field business invariants live in domain functions.
- Phoenix 1.8 scopes: a `scope` struct first in every context function.

## Footguns (`ELIXIR-FW-05`)
- Plug order in the endpoint matters.
- `runtime.exs` vs `config.exs`: secrets and URLs only in `runtime.exs`.
- Releases need `mix assets.deploy`, and `PHX_SERVER`/`bin/server` to start the web server; verify `check_origin`.

## Ecto, Oban, Broadway
- Ecto: sandbox tests (`testing.md`), safe migrations (`data-api.md`).
- Oban: jobs enqueued inside the data transaction, `Oban.Testing` helpers (`assert_enqueued`, `perform_job`).
- Broadway: backpressure and batching; graceful drain on shutdown.

## Umbrella notes (`ELIXIR-REPO-03`)
Shared `mix.lock` and build; per-app `coveralls.json` (`skip_files` does not work at the umbrella root); coverage merges with `mix test.coverage` at the root. These are unverified in the pack: report to the lead before relying on `cover/lcov.info` for an umbrella.
