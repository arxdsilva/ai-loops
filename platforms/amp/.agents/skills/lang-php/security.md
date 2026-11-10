# PHP security

Digest of RULES SEC. Review these in every diff (`PHP-DOC-03`).

- **SQL (`PHP-SEC-01`)**: PDO `prepare` + bound parameters, `ATTR_ERRMODE => ERRMODE_EXCEPTION`, `ATTR_EMULATE_PREPARES => false`. Identifiers and sort directions come from an allow-list. ORMs: no `whereRaw`/`DB::raw`/DQL string building with input.
- **Deserialization (`-02`)**: never `unserialize()` untrusted data; use `json_decode(..., JSON_THROW_ON_ERROR)`. If unavoidable: HMAC with `hash_equals`, then `['allowed_classes' => false]`. XML: `LIBXML_NONET`, never `LIBXML_NOENT`.
- **Code and commands (`-03`)**: no `eval`, string `assert`, or `system/exec/shell_exec/passthru/proc_open/popen` with input; Symfony Process with an argv array. The backtick operator is deprecated in 8.5.
- **Passwords (`-04`)**: `password_hash($p, PASSWORD_ARGON2ID)` (OWASP minimum m=19456 KiB, t=2, p=1) or default bcrypt (cost 12 in 8.4; 72-byte limit); `password_verify`, `password_needs_rehash`; no `salt` option; column at least 255 bytes.
- **Randomness and crypto (`-05`)**: `random_bytes`/`random_int` for security, never `rand`/`mt_rand`/`uniqid`; compare secrets with `hash_equals`; libsodium or `defuse/php-encryption`; no `md5`/`sha1` for passwords or tokens.
- **Web (`-06`)**: context escaping (Twig/Blade autoescape on; avoid `{!! !!}` and Twig `raw`); CSRF on (`PreventRequestForgery`, `csrf_protection`); redirect allow-lists; `realpath` + base-dir check for paths; SSRF host allow-list; explicit CORS.
- **php.ini production (`-07`)**: `expose_php=Off`, `display_errors=Off`, `log_errors=On`, `allow_url_include=Off`, secure/httponly/samesite session cookies, `session.use_strict_mode=1`, `disable_functions` for the exec family.
- **Validation (`-08`)**: at the adapter (Symfony Validator on DTOs, Laravel `FormRequest`); domain re-checks invariants. Eloquent `$fillable`, never `$guarded = []` with request input.
- **Supply chain (`-09`, `PHP-REPO-07`, `PHP-TOOL-08`)**: commit the lock; `composer audit --locked` (gate: `--vuln`); explicit `allow-plugins`; `--no-scripts` in untrusted CI; `--abandoned=fail`; watch for typosquats.
- **SAST (`-10`)**: PHPStan `phpstan-strict-rules`, Psalm `--taint-analysis`, or Semgrep `p/php`.
- Never log passwords, tokens or PII (`PHP-OBS-01`); `APP_DEBUG=false`, no stack traces to clients (`PHP-API-03`).
