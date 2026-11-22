# Clojure repo and project management

Digest of CLJ-REPO and CLJ-OPS. Universal repo rules: `specs/REPO_PRACTICES.md` (when present). Rules: `specs/languages/clojure/RULES.md`.

## Layout and namespaces (CLJ-REPO-01/02)
- `deps.edn` `:paths ["src" "resources"]`; one namespace prefix per service; tests under `test/` mirror `src/` and files end in `_test.clj`.
- Dashes in namespaces, underscores in files: `my-service.http.handler` is `src/my_service/http/handler.clj`. The ns name must equal the path.
- Dev-only code (`dev/user.clj`) is on the path only through the `:dev` alias.

## Build tool (CLJ-REPO-03/04/09)
- New services: `deps.edn` plus tools.build (`build.clj`: `clean`, `copy-dir`, `compile-clj`, `uber`; run `clojure -T:build uber`). Leiningen only for existing projects that need its plugins; never both as the dependency source of truth.
- Day-one files: `deps.edn`, `build.clj`, `tests.edn`, one passing test, `.clj-kondo/`, `.cljfmt.edn`, CI workflow, README, `.gitignore`.

## Pinning (CLJ-REPO-05/06/07/08/11)
- There is no lockfile. Pin Maven coordinates to exact versions (never `RELEASE`, `LATEST`, ranges); git deps by full `:git/sha` (plus `:git/tag`); tools in `-T` installs.
- Snapshot the resolution when reproducibility matters: `clojure -Stree`, or `clojure -X:deps mvn-pom` for scanners.
- Bumps by Renovate (`deps.edn` manager) or `clojure -Tantq outdated`; remove unused deps with kondo `:unused-namespace`.
- Toolchain pins in `.tool-versions` (java temurin 21, clojure, babashka, clj-kondo).

## Ignore files and CI (CLJ-REPO-12/13)
- `.gitignore`: `.cpcache/`, `target/`, `.clj-kondo/.cache`, `.lsp/.cache`, `.nrepl-port`, `.lein-*`, `classes/`, `*.class`, `hs_err_pid*.log`. Commit `.clj-kondo/config.edn` and imported lib configs.
- CI order: JDK + Clojure, `clojure -P -M:test`, cljfmt, clj-kondo, tests with coverage, `test-repeat`, vuln scan, `clojure -T:build uber`, container. Cache `~/.m2`, `~/.gitlibs`, `.cpcache`.

## Monorepos (CLJ-REPO-15/16/17)
- Polylith (`poly` CLI): `components/` (interface + implementation), `bases/`, `projects/` (own `deps.edn`), `development/`. Run `poly check` and `poly test`. Without it, use `:local/root` deps; never copy namespaces.
- Gate caveat: pass `--pkgs components bases projects` so kondo lints the workspace; coverage globs already exclude `development/`.

## Releases and packaging (CLJ-REPO-10/14, CLJ-OPS-02/05/06)
- Libraries: SemVer from a tag or `b/git-count-revs`, `deps-deploy` with signing. Services: the container tag is the release; multi-stage image, uberjar, non-root.
- Conventional commits drive the changelog and release level.
- Run caches: `~/.m2` and `~/.gitlibs` shared; `.cpcache/`, `target/`, `*.hprof`, `hs_err_pid*.log` are cleaned artifacts.
