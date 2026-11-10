# Repo and toolchain (Elixir)

Digest of RULES.md REPO and TOOL. The `elixir-engineer` owns these files in wave 0.

## Layout (`ELIXIR-REPO-01`, `-02`, `-03`)
```
mix.exs  mix.lock  .tool-versions  .formatter.exs  .credo.exs  coveralls.json
config/{config,dev,test,prod,runtime}.exs
lib/<app>/  lib/<app>_web/   priv/repo/migrations/   test/{<app>,<app>_web}/  test/support/
```
- One Mix project; `mix phx.new` or `mix new NAME --sup` for new services (a supervised app is the deployable).
- Umbrella only for independent deploys or dependency sets; prefer `in_umbrella: true`. Umbrella coverage config is per app (unverified; tell the lead).
- Test helpers compile only in `:test`: `elixirc_paths(:test)` adds `test/support`.

## Lock, pins, drift (`ELIXIR-REPO-04`, `-05`, `-08`)
- Commit `mix.lock`; never hand-edit; update with `mix deps.update`.
- Version ranges use `~>`. Drift checks: `mix deps.get --check-locked` (the gate's `deps` check), `mix deps.unlock --check-unused`.
- `.tool-versions` pins `elixir 1.20.5-otp-28` and `erlang 28.x`; mirror in CI and the Dockerfile; `elixir: "~> 1.19"` in `mix.exs`. Never the distro package (`ELIXIR-TOOL-02`).
- Targets: Elixir 1.19/1.20 on OTP 27-29; 1.19 on OTP 28 needs 28.1+ (`ELIXIR-TOOL-01`).

## Ignore and commit (`ELIXIR-REPO-09`)
Ignore `_build/ deps/ cover/ doc/ erl_crash.dump *.ez priv/plts/*.plt* tmp/ .elixir_ls/ .lexical/ *.beam`. Commit `.formatter.exs .credo.exs coveralls.json .sobelow-conf`.

## CI order (`ELIXIR-REPO-10`)
deps (cached on `mix.lock` + `.tool-versions`), format, compile `--warnings-as-errors`, credo `--strict`, dialyzer, tests with coverage, `deps.audit` + `hex.audit`, `sobelow`, `mix release`. Matrix over Elixir/OTP only for libraries.

## Releases and updates (`ELIXIR-REPO-06`, `-07`, `-11`)
Dependabot `package-ecosystem: "mix"` (version updates only; advisories come from the audit tasks). Conventional commits; `@version` in `mix.exs` is the single source; services are tagged `v<version>` and shipped as releases. A Hex package needs `package/0` and a clean `mix hex.build`.

## Tool configuration (`ELIXIR-TOOL-03`..`-10`)
- `mix format`: `.formatter.exs` with `inputs`, `import_deps: [:ecto, :phoenix]`, `plugins` (LiveView HEEx formatter). Do not run a competing formatter in the editor.
- Credo: `mix credo gen.config`; enable `Readability.ModuleDoc` and `Design.TagTODO` (`ELIXIR-DOC-04`).
- Dialyxir: `plt_file: {:no_warn, "priv/plts/project.plt"}`, `plt_local_path: "priv/plts"`; slow first run, so optional.
- Vulnerabilities: `{:mix_audit, "~> 2.1", only: [:dev, :test], runtime: false}`; `mix hex.audit` ships with Hex. SBOM: `mix sbom.cyclonedx`.
- When a dev-only dep is missing, the check's `install` hint in the pack shows the line to add; add it as an override.
