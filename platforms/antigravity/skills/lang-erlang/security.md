# Erlang security

Digest of `specs/languages/erlang/RULES.md` SEC. Hardening guide: https://security.erlef.org/.

- **Injection (`ERL-SEC-01/02`):** parameterised queries only (`epgsql:equery/3` with `$1`, `pgo:query/2`). No `os:cmd/1` or `open_port({spawn, ...})` with user input; use `{spawn_executable, Path}` with an `args` list.
- **Deserialization and atoms (`ERL-SEC-03/04`, `ERL-IDIOM-12`):** `binary_to_term(Bin, [safe])` only, and `safe` does not stop funs: prefer JSON, Protobuf or CBOR on the wire. No `file:consult/1` on untrusted input. JSON keys stay binaries; validate with a schema.
- **Distribution (`ERL-SEC-05`):** the cookie is access control, not encryption. TLS distribution with `{verify, verify_peer}` and mutual certificates; bind EPMD and `inet_dist_use_interface` internally; never expose 4369 or the distribution ports; no `.erlang.cookie` in git or logs.
- **Secrets (`ERL-SEC-06`):** from the environment or a vault via `sys.config.src` and `vm.args.src`; wrap them in `fun() -> ... end` so crash dumps and `sys:get_state` do not print them. `ERL_CRASH_DUMP` output holds memory: treat it as sensitive.
- **Crypto (`ERL-SEC-07`):** `crypto:strong_rand_bytes/1`, never `rand`, for secrets. Argon2id or bcrypt through a maintained NIF, not bare SHA-2.
- **TLS and HTTP (`ERL-SEC-08`):** `{verify, verify_peer}`, `{cacerts, public_key:cacerts_get()}`, `customize_hostname_check`; validate redirects against SSRF; explicit CORS and CSRF in Cowboy.
- **Input (`ERL-SEC-09`):** validate at the adapter (`jesse` JSON Schema or pattern-matching decoders) before a use case runs.
- **Supply chain (`ERL-SEC-10/11`, `ERL-TOOL-08`):** `rebar.lock` holds checksums; pin plugins; `rebar3 audit` (GitHub Advisory Database, needs network, single maintainer) and `rebar3 sbom` are off by default. No gosec-class SAST exists: rely on Dialyzer, Elvis and review.
