# Go security digest

Rules: `specs/languages/go/RULES.md §29, §38, §44, §46`. Research IDs `GO-SEC-01..11`, `GO-TOOL-06`, `GO-TOOL-07`.

## Input (GO-SEC-01, 02, 08)
- SQL: parameterised only (`db.QueryContext(ctx, "... WHERE id = $1", id)`); identifiers from an allow-list; sqlc and pgx give typed params.
- Commands: `exec.CommandContext(ctx, name, args...)`, no shell, no user-controlled name. Templates: `html/template` (contextual autoescape), never `text/template` for HTML.
- Parsing: cap sizes with `http.MaxBytesReader` and `io.LimitReader`; `json.Decoder.DisallowUnknownFields`; typed structs, not `map[string]any`. `encoding/gob` and YAML of untrusted data are risky. `encoding/xml` has no DTD resolution but still cap depth and size. `encoding/json` is backed by v2 on 1.27 (stricter UTF-8 and duplicate-key handling): read the diffs before adopting `encoding/json/v2` directly.
- Validate once at the inbound adapter with an explicit function or `go-playground/validator`; produce typed commands; the domain trusts invariants.

## Paths (GO-SEC-03)
Open user-named files through `os.Root`: `root, err := os.OpenRoot(dir)`, `root.Open(name)`, `root.OpenFile`, or `os.OpenInRoot(dir, name)` (1.24). It cannot be escaped by `..` or symlinks. Since 1.25 `Root` also offers `Symlink`, `Rename`, `Link`, `Readlink`, `Chmod`, `Chown`, `Chtimes`, `MkdirAll`, `RemoveAll`, `ReadFile` and `WriteFile`. `filepath.IsLocal` validates a relative name without touching disk. Do not use `filepath.Join` plus a prefix check.

## Servers and outbound (GO-SEC-04, 05, 11)
- Every server sets `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`, `MaxHeaderBytes`; never bare `http.ListenAndServe` (gosec G112).
- SSRF: allow-list URLs; resolve and block private and link-local ranges in a `net.Dialer.Control` hook; set client timeouts; `net.JoinHostPort`, not `Sprintf("%s:%d")` (vet `hostport`). Allow-list redirect targets.
- CSRF: SameSite cookies plus a token for browser clients; CORS with explicit origins.
- Authn and authz at the inbound adapter, principal passed inward as data. Do not trust `X-Forwarded-For` without a trusted-proxy list.

## Secrets and crypto (GO-SEC-06, 07)
- Secrets come from the environment or a secret manager in `cmd`/`platform`, held in typed config whose `String()` redacts; never logged or committed.
- `crypto/rand` and `rand.Text()` (1.24) for tokens; `math/rand/v2` only for non-security randomness; argon2id (`golang.org/x/crypto/argon2`) or bcrypt for passwords; `crypto/subtle.ConstantTimeCompare`; no MD5, SHA-1, DES, RC4; TLS 1.2+ with std defaults. FIPS 140-3 module where compliance requires it.

## Supply chain and scanning (GO-SEC-09, 10, GO-TOOL-06, 07)
- `go mod verify`, `GOFLAGS=-mod=readonly` (default), checksum DB (`GOPRIVATE`/`GONOSUMDB` only for private modules), GitHub Actions pinned by SHA.
- `govulncheck ./...` (reachability-aware; exits non-zero on called vulnerabilities) and `govulncheck -mode=binary ./app` on the release artifact. With `-format sarif` or `openvex` it exits 0: gate on the findings yourself. Enable in the gate with `loops gate --vuln`.
- SAST: `gosec -exclude-generated ./...` (exit 1 on findings; `-fmt sarif -out gosec.sarif` for code scanning), or the `gosec` linter in golangci-lint. CI makes both required, not "when installed".
- SBOM (`cyclonedx-gomod`, `syft`), image scan (`trivy`, `grype`), signing (`cosign`).

## Review prompts
Unbounded body or goroutine per request; `http.DefaultClient`; `Sprintf` into SQL; `filepath.Join` with user input; secrets in logs; missing deadline on an outbound call; error text leaked to clients (return RFC 9457 problem details, RULES §44).
