# Security (Elixir)

Digest of RULES.md SEC. See also the loops review and consistency skills for universal checks.

## Injection and execution (`ELIXIR-SEC-01`, `-02`)
- SQL: Ecto `^` interpolation, `fragment("... ?", ^x)`, `Repo.query/3` with params; never interpolate into `fragment/1`.
- Shell: `System.cmd(cmd, args_list)`, not `:os.cmd/1`.
- No `binary_to_term/1` on untrusted data (at least `[:safe]`; prefer JSON or Protobuf), no `Code.eval_string`/`eval_quoted` on input.

## Atoms (`ELIXIR-SEC-03`, `ELIXIR-IDIOM-11`)
`String.to_existing_atom/1` or an explicit map; never `String.to_atom/1` on external input (atoms are not garbage-collected).

## Web (`ELIXIR-SEC-04`)
Keep CSRF protection and `put_secure_browser_headers` in `:browser`; HEEx escapes by default, never `raw/1` on user input; `Path.safe_relative/2` before file access; validate redirects; explicit CORS; SSRF-check outbound URLs; verify `check_origin`.

## Secrets and crypto (`ELIXIR-SEC-05`)
Secrets from the environment in `config/runtime.exs`; `mix phx.gen.secret`; hash with `argon2_elixir`, `bcrypt_elixir` or `pbkdf2_elixir`; `Plug.Crypto.secure_compare/2`; tokens from `:crypto.strong_rand_bytes/1`, never `:rand`.

## Input validation (`ELIXIR-SEC-06`)
Changesets (`cast(attrs, [:field])`, also schemaless) or `NimbleOptions` at the adapter boundary; never pass raw `params` into the domain.

## Supply chain (`ELIXIR-SEC-07`, `ELIXIR-TOOL-08`, `-09`)
Commit `mix.lock`; review `mix deps.update` diffs; `git:` deps need a pinned `ref:`; scan with `mix deps.audit` and `mix hex.audit` (gate checks `deps-audit`, `hex-audit`, run with `--vuln`); SBOM via `mix sbom.cyclonedx`.

## SAST (`ELIXIR-SEC-08`)
`mix sobelow --exit medium` (gate check `sobelow`, off by default; Phoenix projects). Suppress with `# sobelow_skip` and a reason; config in `.sobelow-conf`.

## Deployment (`ELIXIR-SEC-09`)
No exposed EPMD or distribution ports; secret `RELEASE_COOKIE`; TLS `verify_peer` with `:public_key.cacerts_get/0`; no crash dumps in production.
