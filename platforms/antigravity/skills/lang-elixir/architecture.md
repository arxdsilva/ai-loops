# Architecture (Elixir)

Digest of RULES.md ARCH. Universal rules live in `specs/ARCHITECTURE.md` and `specs/SOLID.md`; this file is the Elixir mapping.

## Style by size (`ELIXIR-ARCH-01`)
Phoenix contexts for CRUD-heavy apps; hexagonal (ports as behaviours, Ecto only in adapters) for rich domains or several I/O boundaries; Boundary-checked modular monolith before microservices; CQRS/ES (Commanded) only with a proven need. One process per aggregate is native but not a default. Project conventions win (`ELIXIR-FW-02`).

## Layer map
| loops layer | Elixir |
|---|---|
| domain | structs and pure functions in `lib/<app>/<ctx>/` (no Repo, File, System, Logger, clock, RNG, processes, `:ets`) |
| ports | `@behaviour` with `@callback`s owned by the consumer context |
| application | the context module (public API, scope first) and one plain module per use case; processes only for runtime state |
| outbound adapters | `@impl true` modules: Ecto store and schema, `Req` clients, Oban workers |
| inbound adapters | `lib/<app>_web/`: controllers, LiveViews, channels, plugs; call the context API only |
| composition root | `Application.start/2` child specs plus `config/runtime.exs` adapter selection |

## Rules to apply
- `ELIXIR-ARCH-02`: `lib/<app>/` never mentions `Phoenix.*`, `Plug.*`, `<App>Web.*`; web never touches `Repo` or schemas.
- `ELIXIR-ARCH-03`: one module per file, path mirrors the name; grouped `alias A.{B, C}` and qualified calls are dependencies too.
- `ELIXIR-ARCH-06`: wire in the supervision tree and `runtime.exs`; read config once, pass it in; no `Application.get_env` in the domain.
- `ELIXIR-ARCH-07`: inject time, ids and randomness through a `Clock` or `IdGenerator` behaviour. Pure code returns tagged tuples.
- `ELIXIR-ARCH-08`: few callbacks per behaviour; Mox doubles need the behaviour.
- `ELIXIR-ARCH-09`: no process for code organisation; one API module per process.
- `ELIXIR-ARCH-10`: at most 5 parameters, structs under 32 fields, no complex `with ... else`.
- `ELIXIR-ARCH-11`: Phoenix 1.8 scope first argument.

## Enforcement (parser `none`)
`loops arch` has no Elixir scanner. Layering is enforced natively:
- Boundary 0.12: `compilers: [:boundary] ++ Mix.compilers()`, `use Boundary, deps: [...], exports: [...]` per context root. Violations are compile warnings, so the gate's `compile` check (`--warnings-as-errors`) fails them (`ELIXIR-ARCH-04`). If the project does not use Boundary, propose it in the plan and add it as an override in wave 0.
- `xref-cycles` (`mix xref graph --format cycles --fail-above 0`): off by default because compile-time cycles are common; enable it when the project is clean (`ELIXIR-ARCH-05`). `--source`/`--sink` explain a path.
- The pack's `forbidden_in_domain` (`Ecto.Repo`, `Ecto.Query`, `Phoenix`, `Plug`, `Logger`, `File`, `System`, `Oban`) and `impure_calls` (`DateTime.utc_now`, `Enum.random`, `:rand.`, `spawn`, `Task.async`, `send(`, `:ets.`) are review checklists for the domain layer, not scanned.
