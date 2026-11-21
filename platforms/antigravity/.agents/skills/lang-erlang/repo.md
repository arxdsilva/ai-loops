# Erlang repo and project management

Digest of `specs/languages/erlang/RULES.md` REPO and TOOL. Universal repo practice: `specs/REPO_PRACTICES.md` when present.

## Layout (`ERL-REPO-01/02`)
```
rebar.config  rebar.lock  elvis.config  .tool-versions  .editorconfig
config/sys.config.src  config/vm.args.src
apps/<app>/{src/<app>.app.src, src/*.erl, include/, priv/, test/}
test/   cross-app Common Test suites          priv/   migrations, static files
```
- Create with `rebar3 new app|lib|release|umbrella`. `apps/` for several OTP apps, `lib/` only for libraries shipped together; one release per deployable.
- Split a repo only at a deployable boundary; no Bazel or Nx unless the org runs it (`ERL-REPO-03`).

## Dependencies (`ERL-REPO-04..07`)
- Commit `rebar.lock`; never hand-edit it. Bump with `rebar3 upgrade <dep>`, inspect with `rebar3 tree`.
- Hex deps use `"~> X.Y"`; git deps use `{tag, ...}` or `{ref, ...}`, never `{branch, ...}`. Set `{deps_error_on_conflict, true}.`.
- Remove unused deps with `rebar3_hank`. Renovate and Dependabot do not support rebar3: schedule a CI `rebar3 upgrade` job or a Renovate regex manager.
- Review `pre_hooks` and `provider_hooks` in dependencies: they run shell commands at compile time (`ERL-SEC-10`).

## Toolchain (`ERL-TOOL-01`, `ERL-REPO-09`)
- Target OTP 28 or 29 (OTP has no LTS; the last three majors get patches). Pin with `.tool-versions` (asdf or mise: `erlang`, `rebar`) and `{minimum_otp_vsn, "27"}.`. Use rebar3 3.27 or newer (CVE-2026-21619).
- Compile with `debug_info, warnings_as_errors, warn_export_vars, warn_unused_import, warn_missing_spec`; no blanket `nowarn_*` (`ERL-TOOL-03`).
- erlfmt: `{project_plugins, [erlfmt]}.`, `rebar3 fmt --check`; opt out with `%%% % @noformat` or `%% erlfmt:ignore <reason>`. The editor uses ELP and one formatter only.

## .gitignore and CI (`ERL-REPO-10/11`)
- Ignore `_build/`, `*.beam`, `erl_crash.dump`, `rebar3.crashdump`, `ct_logs/`, `*.coverdata`, `.rebar3/`, `*.plt`, `bom.xml`, `Mnesia.*/`, `cover/`, `*.tar.gz`. Commit `rebar.lock`, `elvis.config`.
- CI order: restore `_build` and `~/.cache/rebar3` keyed on `rebar.lock` plus the OTP version, then `compile`, `fmt --check`, `lint`, `xref`, `dialyzer`, `eunit`, `ct`, `proper`, `cover`, `sbom`. Matrix: OTP 28 and 29 (27 until dropped).

## Publishing and commits (`ERL-REPO-08/12`)
Libraries: `rebar3 hex publish`, SemVer, `CHANGELOG.md`, `{licenses,...}` and `{links,...}` in `.app.src`. Conventional commits.
