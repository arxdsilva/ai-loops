# Go repository and toolchain digest

Rules: `specs/languages/go/RULES.md §2, §41, §48, §49`; research IDs in the RULES appendix.

## Layout (GO-REPO-01, 02)
- A service is `cmd/<bin>/main.go` plus everything else under `internal/` (compiler-enforced privacy). Export a package only when another module must import it, and then give it its own module.
- No `pkg/`, `utils/`, `common/`, `models/` or top-level `src/`: golang-standards "project-layout" is not an official standard. Where a project already has `pkg/`, keep it stdlib-only (RULES §48).
- `api/` holds contract sources (OpenAPI, `.proto`), `db/migrations` and `db/queries` the SQL, `deploy/` Dockerfile and manifests, `testdata/` per package (ignored by `./...`).

## Modules (GO-REPO-03..05)
- Default: one `go.mod` at the root. A second module only for a separately versioned library, a tool with a different dependency set, or a generated client; tag it `path/vX.Y.Z`.
- `./...` stops at a nested `go.mod`. In a multi-module repo run build, vet and test in every module (`find . -name go.mod`, or `go work edit -json`), and run `loops gate` per module root. The gate itself only sees the root it is given.
- `go.work` is for local development: gitignore `go.work` and `go.work.sum` unless every module is developed only against each other. CI uses `GOWORK=off`.

## Toolchain pin (GO-REPO-06, 11, 12, GO-TOOL-01)
- `go 1.N.0` in `go.mod` is the minimum language version (a hard floor); `toolchain go1.N.P` is the suggested exact toolchain. CI: `actions/setup-go` with `go-version-file: go.mod`. Air-gapped: `GOTOOLCHAIN=local`.
- Support the two newest majors (1.27 and 1.26 as of 2026-11); releases come each February and August. `go get go@1.N.0` lowers the `go` line (do not rely on `go mod init` writing N-1: it wrote the full local version on 1.26.4 and 1.27.3).
- Dev tools live in `go.mod` with the `tool` directive: `go get -tool honnef.co/go/tools/cmd/staticcheck@<version>`, run `go tool staticcheck`. Never `tools.go` blank imports or `@latest` in CI. Latest staticcheck as of 2026-11 is v0.8.2 (2026.2.2); check before pinning.
- Only the architect or `go-engineer` edits `go.mod`, `go.sum`, `go`/`toolchain`/`tool` lines and `go.work`.

## Dependencies (GO-REPO-07..09, GO-TOOL-07)
- Commit `go.mod` and `go.sum` always; there is no separate lockfile. No `vendor/` unless hermetic builds need it, then verify with `go mod vendor && git diff --exit-code`.
- CI: `go mod tidy -diff` (non-zero when changes are needed) and `go mod verify`.
- Dependabot (`package-ecosystem: gomod`) or Renovate `gomod`, grouped by minor and patch; `govulncheck` and `-race` tests are required checks; never auto-merge a major bump. Multi-module repos upgrade per module.
- SBOM with `cyclonedx-gomod` or `syft`; licenses with `go-licenses` or `trivy`.

## Version control and releases (GO-REPO-10, 13..15)
- SemVer through git tags `vMAJOR.MINOR.PATCH`; a v2+ module path ends in `/v2`. Check library API diffs with `gorelease` before tagging.
- `.gitignore`: `*.test`, `*.out`, `*.prof`, `*.pprof`, `*.cov`, `coverage*.txt`, `/bin/`, `/dist/`, `go.work`, `go.work.sum`, `__debug_bin*`, `.env`. `.gitattributes`: `*.go text eol=lf`; mark generated files `linguist-generated`.
- Conventional commits drive the next tag; sign release artifacts and publish checksums or provenance (cosign, SLSA).

## CI shape per PR (GO-REPO-14)
setup-go with module cache -> `go mod tidy -diff` -> format check -> `go vet` -> `golangci-lint run` (when configured) -> `go build ./...` -> `go test -race -count=1 -shuffle=on -covermode=atomic -coverprofile` -> `gosec` -> `govulncheck ./...` -> (main only) build image and sign. Matrix over the oldest supported and newest Go. `loops gate` covers the local subset; CI adds the rest.

## Linters (GO-TOOL-02..04, 08)
- `staticcheck`, or golangci-lint **v2**: `version: "2"`, `linters.default: standard`, explicit `enable:`, a `formatters:` section, `exclusions.generated: lax`. Do not mix v1 and v2 (`golangci-lint migrate`). Minimum set: `errcheck govet staticcheck unused ineffassign errorlint revive gosec sloglint depguard`.
- `gopls` formats on save; do not add a second formatter. `go fix -diff ./...` shows modernizer rewrites (1.26+; 1.27 adds `atomictypes`, `slicesbackward`).
- `gofmt` alignment changed in 1.27: reformat once when upgrading, in its own commit.

## Version upgrades (language lane checklist)
1. Read the release notes for the target; check `go.mod`, CI image, Dockerfile builder and linter versions agree.
2. `go get go@1.N.0 toolchain@go1.N.P`, `go mod tidy`, `go fix -diff ./...` (review), `gofmt -w` once.
3. `go vet ./...`, `go test -race -count=1 -shuffle=on ./...`, `loops arch`, `govulncheck ./...`.
4. Update the staticcheck/golangci-lint pin if it reports "unsupported version".
