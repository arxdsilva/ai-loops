# Tooling synthesis: the Stage C input

As of **2026-11-10**. This file merges the 15 LOOPS blocks and what their verifiers found in spot runs. Every argv here appears in a language file; spot-run facts cite the file's Verification section.

The blocks were parsed with `scripts/packs.py`. All 15 parse as JSON and carry every draft key (TAXONOMY §15).

## 1. Merged tooling table

The argv are short forms; the language file has the full array. `—` means the pack has no check of that kind.

| Lang | Detect | Source globs | Test globs | Format | Lint | Typecheck | Build | Test | Vuln (SAST) | Race | Coverage: tool · format · path | cache_env (scoped; shared) | Artifacts (main) | Arch: resolver; native tool |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| python | `pyproject.toml`, `uv.lock`, `requirements.txt` | `**/*.py`, `**/*.pyi` | `**/test_*.py`, `**/*_test.py`, `**/conftest.py`, `tests/**` | `ruff format --check .` | `ruff check .` | `mypy --strict src` | `uv sync --locked` (when `uv.lock`) | `pytest -p randomly -n 2 -W error --cov --cov-branch --cov-report=xml:{coverage}` with `PYTHONDEVMODE=1` | `pip-audit --locked .` (`bandit -r src -ll -q`) | `none`: the test argv itself (randomly, xdist, dev mode); optional free-threaded `--parallel-threads` | pytest-cov · cobertura (lcov alt) · `{coverage}` | scoped: `UV_CACHE_DIR`, `PIP_CACHE_DIR`, `PYTHONPYCACHEPREFIX`, `MYPY_CACHE_DIR`, `RUFF_CACHE_DIR`, `COVERAGE_FILE` | `__pycache__`, `.pytest_cache`, `.mypy_cache`, `.ruff_cache`, `.coverage*`, `coverage.xml`, `htmlcov` | `dotted-module`; import-linter `lint-imports` |
| typescript | `package.json`, `tsconfig.json`, `deno.json` | `**/*.{ts,mts,cts,js,mjs,cjs}` | `**/*.test.ts`, `**/*.spec.ts`, `**/__tests__/**`, `test/**` (+ js) | `prettier --check .` | `eslint --max-warnings=0 .` | `tsc --noEmit` | `npm ci --ignore-scripts`; `npm run build --if-present` | `vitest run --coverage --coverage.reporter=lcov --coverage.reportsDirectory={coverage_dir} --sequence.shuffle.files --sequence.shuffle.tests --sequence.concurrent` (via `npx --no-install`) | `npm audit --audit-level=high --omit=dev`; `osv-scanner scan source -r .` | `none`: shuffle + concurrent, printed seed, `--repeats=3` on changed files, no `--retry.count` | v8 · lcov · `{coverage_dir}/lcov.info` | scoped: `npm_config_cache`, `YARN_CACHE_FOLDER`, `npm_config_store_dir`, `BUN_INSTALL_CACHE_DIR`, `DENO_DIR`, `TMPDIR` | `dist/`, `build/`, `coverage/`, `*.tsbuildinfo`, `node_modules/.cache/`, `.eslintcache` | `relative-path`; dependency-cruiser |
| java | `pom.xml`, `build.gradle.kts`, `build.gradle` | `**/src/main/java/**/*.java` | `**/src/test/java/**`, `**/*Test.java`, `**/*Tests.java`, `**/*IT.java` | `./mvnw spotless:check` | `./mvnw checkstyle:check`, `spotbugs:check`, `pmd:check` | — | `./mvnw -B -ntp -DskipTests verify` | `./mvnw -B -ntp verify -Djunit.jupiter.execution.parallel.enabled=true …mode.default=concurrent …testmethod.order.default=…MethodOrderer$Random` | `osv-scanner scan source -r .`; `dependency-check-maven:check -DfailBuildOnCVSS=7` | `none`: JUnit parallel + random order, twice with two seeds | JaCoCo · jacoco-xml · `**/target/site/jacoco/jacoco.xml` (glob) | scoped: `JAVA_TOOL_OPTIONS=-Djava.io.tmpdir=<run>/tmp`, `MAVEN_ARGS=-Djacoco.destFile=… -Djacoco.dataFile=…`; shared: `~/.m2`, `GRADLE_USER_HOME` | `target/`, `build/`, `.gradle/`, `*.class`, `*.hprof`, `*.jfr`, `*.exec` | `package-dir`; ArchUnit inside the test run |
| cpp | `CMakeLists.txt`, `CMakePresets.json`, `vcpkg.json`, `conanfile.txt/.py`, `MODULE.bazel`, `meson.build` | `**/*.{cpp,cc,cxx,hpp,hh,h,ipp,tpp}` | `**/*_test.cpp`, `**/*_test.cc`, `**/test_*.cpp`, `tests/**`, `test/**` | `clang-format --dry-run -Werror` over `git ls-files` | `run-clang-tidy -p build/dev -quiet` | — (`-Werror` build) | `cmake --preset dev`; `cmake --build --preset dev` | three chains `cmake --preset X && cmake --build --preset X && ctest --preset X --output-on-failure --no-tests=error` for `tsan`, `asan-ubsan`, `coverage` (+ `gcovr --lcov {coverage} --filter src/`) | `osv-scanner scan source --lockfile conan.lock` | `build`: TSan preset `-fsanitize=thread -fno-omit-frame-pointer -g -O1`, `TSAN_OPTIONS=halt_on_error=1 second_deadlock_stack=1` | gcovr (or llvm-cov) · lcov · `{coverage}` | scoped: `CCACHE_TEMPDIR`; shared: `CCACHE_DIR`, `VCPKG_DEFAULT_BINARY_CACHE`, `VCPKG_DOWNLOADS`, `CONAN_HOME` | `build/`, `cmake-build-*/`, `CMakeFiles/`, `*.gcda`, `*.gcno`, `*.profraw`, `core*` | `include-path`; CMake target graph (`cmake --graphviz`) |
| csharp | `global.json`, `*.slnx`, `*.sln`, `*.csproj`, `Directory.Build.props` | `**/*.cs` | `**/*Tests.cs`, `**/*Test.cs`, `tests/**/*.cs`, `**/*.Tests/**/*.cs` | `dotnet format --verify-no-changes --no-restore` | analyzers in the `-warnaserror` build | nullable in the build | `dotnet restore --locked-mode` (when lock); `dotnet build --no-restore -warnaserror -c Release` | VSTest: `dotnet test --no-build -c Release "--collect:XPlat Code Coverage" --results-directory {results}`; MTP variant: `… --minimum-expected-tests 1 -- --coverage --coverage-output-format cobertura` | `dotnet package list --vulnerable --include-transitive --format json` (exit 0: parse the output) | `none`: xUnit parallel collections, repeated `Category=Concurrency` tests, concurrency analyzers as errors | coverlet / MS CodeCoverage · cobertura · `{results}/**/*cobertura.xml` | scoped: `NUGET_PACKAGES`, `NUGET_HTTP_CACHE_PATH`, `NUGET_PLUGINS_CACHE_PATH`, `DOTNET_CLI_HOME`, `TMPDIR`; `check_env`: `MSBUILDDISABLENODEREUSE=1`, `DOTNET_NOLOGO=1`, `DOTNET_CLI_TELEMETRY_OPTOUT=1` | `bin/`, `obj/`, `TestResults/`, `*.trx`, `*.binlog` | `project-reference`; NetArchTest / ArchUnitNET in tests |
| php | `composer.json` | `**/*.php` | `tests/**/*Test.php`, `**/*Test.php`, `tests/**/*.php` | `php-cs-fixer check --diff --using-cache=no` (when config; exit 8 = needs fixing) | `phpstan analyse --no-progress --memory-limit=1G` (when `phpstan.neon*`) | PHPStan | `composer validate --strict`; `composer install --no-interaction --prefer-dist` | `phpunit --order-by=random --fail-on-warning --fail-on-risky --coverage-clover {coverage}` with `XDEBUG_MODE=coverage` | `composer audit --locked --format=plain` | `none`: random order with two seeds, serial and `paratest --processes=N` | PHPUnit · clover (cobertura also spot-run 0) · `{coverage}` | scoped: `COMPOSER_CACHE_DIR`, `TMPDIR`; keep `COMPOSER_HOME` | `.phpunit.cache/`, `.phpunit.result.cache`, `.php-cs-fixer.cache`, `coverage/`, `var/cache/` | `namespace-psr4`; deptrac |
| go | `go.mod` | `**/*.go` | `**/*_test.go` | `gofmt -l .` (fails on output) | `go vet ./...`; `staticcheck ./...` | compiler | `go build ./...` | `go test -race -count=1 -covermode=atomic -coverprofile={coverage} ./...` | `govulncheck ./...` (with `--vuln`) | `flag`: `-race` / `GOFLAGS=-race` (needs cgo) | go · gocover · `{coverage}` | scoped: `GOCACHE`, `GOTMPDIR`; shared: `GOMODCACHE`, `GOPATH` | `*.test`, `*.out`, `*.prof`, `*.pprof`, `*.cov`, `*.exe`, `__debug_bin*` | `module-path`; loops `goparse` |
| ruby | `Gemfile`, `.ruby-version` | `**/*.rb`, `**/*.rake`, `**/*.ru`, `**/Gemfile`, `**/*.gemspec` | `spec/**/*_spec.rb`, `test/**/*_test.rb`, `test/**/test_*.rb` | `rubocop --only Layout` | `rubocop` | `srb tc` or `steep check` | `bundle check` | `bundle exec rspec --order random`, or `bin/rails test` | `bundle-audit check --update` (`brakeman -q --no-pager`) | `none`: random order + printed seed, parallel_tests, re-run of changed specs | SimpleCov · simplecov-json · `coverage/coverage.json` (lcov alt `coverage/lcov.info`) | scoped: `BOOTSNAP_CACHE_DIR`, `TMPDIR`; shared: `BUNDLE_USER_CACHE`, `BUNDLE_PATH` | `coverage/`, `tmp/`, `log/*.log`, `.rspec_status` | `native`; packwerk |
| swift | `Package.swift` | `Sources/**/*.swift` | `Tests/**/*.swift`, `**/*Tests.swift` | `swift format lint --strict --recursive Sources Tests` (exit code) | `swiftlint lint --strict` | Swift 6 language mode | `swift build --force-resolved-versions --explicit-target-dependency-import-check error` | `swift test --sanitize=thread --enable-code-coverage` | — (only `swift package generate-sbom`) | `flag`: `--sanitize=thread` + Swift 6 mode | llvm-cov · lcov · `{coverage}` via a 2-step export | no env vars: `--scratch-path` per run; `--cache-path`, `--config-path`, `--security-path` shared | `.build/`, `.swiftpm/`, `*.profraw`, `*.profdata` | `target-name`; `--explicit-target-dependency-import-check=error` |
| kotlin | `settings.gradle.kts`, `build.gradle.kts`, `gradlew` | `**/src/main/**/*.kt`, `*.kts` | `**/src/test/**/*.kt`, `**/*Test.kt`, `**/*Tests.kt`, `**/*Spec.kt` | `./gradlew ktlintCheck` | `./gradlew detekt` | compiler warnings | `./gradlew --console=plain assemble` | `./gradlew --console=plain test --rerun koverXmlReport` | `osv-scanner scan source -L gradle.lockfile` | `none`: JUnit parallel + random via `junit-platform.properties`, `@RepeatedTest`, Lincheck | Kover · jacoco-xml · `build/reports/kover/report.xml` | shared: `GRADLE_USER_HOME` (or seed from `GRADLE_RO_DEP_CACHE`); `cache_argv`: `--project-cache-dir {run}/.gradle` | `build/`, `.gradle/`, `.kotlin/`, `*.class`, `*.hprof` | `package-dir`; Konsist |
| rust | `Cargo.toml` | `**/*.rs` | `**/tests/**/*.rs`, `**/benches/**`, `**/tests.rs`, `**/*_test(s).rs` | `cargo fmt --all --check` | `cargo clippy --workspace --all-targets --all-features --locked -- -D warnings` | compiler | `cargo build --workspace --all-targets --locked` | `cargo llvm-cov --workspace --all-features --locked --lcov --output-path {coverage}`; `cargo test --doc --workspace --locked` | `cargo deny check`; `cargo audit` | `none`: `unsafe_code=forbid`, clippy; `race.build` tier (miri, loom, nightly TSan) for `unsafe`/atomics | cargo-llvm-cov · lcov · `{coverage}` (its parent dir must exist) | scoped: `CARGO_TARGET_DIR`, `TMPDIR`, `CARGO_INCREMENTAL=0`; shared: `CARGO_HOME`, `RUSTUP_HOME`, `SCCACHE_DIR` | `target/`, `lcov.info`, `*.profraw`, `mutants.out/` | `crate-path`; `cargo metadata` crate graph |
| scala | `build.sbt`, `build.sc`, `build.mill`, `project.scala` | `**/src/main/scala*/**/*.scala`, `**/*.sc` | `**/src/test/scala/**`, `**/src/it/scala/**`, `**/*{Spec,Suite,Test}.scala` | `sbt -batch scalafmtCheckAll scalafmtSbtCheck` | `sbt -batch "scalafixAll --check"` | `-Werror` compile | `sbt -batch Test/compile` | `sbt -batch clean coverage test coverageReport` | `sbt -batch dependencyCheck`; `osv-scanner scan source -r .` | `none`: loops runs the test argv 3× with parallel suites (`testForkedParallel` with fork) | sbt-scoverage · cobertura · `target/scala-*/coverage-report/cobertura.xml` | scoped: `JAVA_TOOL_OPTIONS=-Djava.io.tmpdir=<dir>/tmp`; shared: `COURSIER_CACHE`, sbt boot | `target/`, `project/target/`, `.bsp/`, `.bloop/`, `.metals/`, `*.class` | `package-dir`; sbt `dependsOn` (compiler-enforced), ArchUnit optional |
| elixir | `mix.exs` | `lib/**/*.ex`, `apps/*/lib/**/*.ex` | `test/**/*_test.exs`, `apps/*/test/**/*_test.exs` | `mix format --check-formatted` | `mix credo --strict` (`mix sobelow --exit medium`) | `mix dialyzer` | `mix deps.get --check-locked`; `MIX_ENV=test mix compile --warnings-as-errors` | `mix coveralls.lcov --warnings-as-errors` | `mix deps.audit`; `mix hex.audit` | `none`: `mix test --repeat-until-failure 5 --warnings-as-errors`, async modules, printed seed | excoveralls · lcov · `cover/lcov.info` | scoped: `MIX_BUILD_ROOT`; shared: `deps/`, `MIX_HOME`, `HEX_HOME` | `_build`, `cover`, `doc`, `erl_crash.dump`, `*.beam` | `native`; boundary compiler + `mix xref graph --format cycles --fail-above 0` |
| erlang | `rebar.config`, `rebar.lock`, `erlang.mk` | `**/src/*.erl`, `**/include/*.hrl`, `**/src/*.app.src` | `**/test/*_SUITE.erl`, `**/*_tests.erl`, `**/test/prop_*.erl` | `rebar3 fmt --check` | `rebar3 xref`; `rebar3 lint` (when `elvis.config`) | `rebar3 dialyzer` | `rebar3 compile` | `rebar3 eunit --cover`; `rebar3 ct --cover`; then `rebar3 covertool generate` | `rebar3 audit` | `none`: eunit + ct once per `race.variants` entry (`ERL_FLAGS=+S 1:1`, `+S 8:8`), `ct --repeat`, proper_statem | cover + covertool · cobertura · `_build/test/covertool/*.covertool.xml` | scoped: `ERL_CRASH_DUMP`; shared: `REBAR_CACHE_DIR`, `REBAR_GLOBAL_CONFIG_DIR` | `_build`, `*.beam`, `ct_logs`, `erl_crash.dump`, `*.coverdata` | `module-name`; `rebar3 xref` |
| clojure | `deps.edn`, `project.clj`, `workspace.edn` | `**/*.clj`, `**/*.cljc` | `test/**/*_test.clj(c)`, `**/test/**/*_test.clj(c)` | `clojure -Tcljfmt check` | `clj-kondo --lint src test --parallel` (exit 2/3) | — | `clojure -P -M:test` | `clojure -M:cov -o target/coverage`; `clojure -M:test --randomize --seed {seed}` with `repeat: 3` | `clojure -Tnvd nvd.task/check :classpath "{classpath}" …` (needs `NVD_API_TOKEN`) | `none`: 3 seeded Kaocha runs (`test-repeat`) | cloverage · lcov · `target/coverage/lcov.info` | scoped: `JAVA_TOOL_OPTIONS=-Djava.io.tmpdir={tmp}`; shared: `~/.m2`, `~/.gitlibs` | `target/`, `.cpcache/`, `.clj-kondo/.cache/`, `.lsp/.cache/`, `classes/` | `dotted-module-underscore`; `poly check` |

## 2. The final proposed pack schema

### 2.1 Principles

1. **Express the Go pack exactly.** The gate today is `internal/application/gate/gate.go`, and the Go pack below reproduces it. That covers:
   - the check order and the required set (build, vet, test, coverage);
   - skipping optional tools that aren't on PATH;
   - `--vuln`;
   - the gofmt output filter;
   - the stale-staticcheck skip;
   - coverage args sent only when a profile is requested;
   - the test-output signal lines.
2. **Data, not code.** Every language difference the researchers found is a field. The gate loop, Skip/Only/FailFast and event recording stay as they are.
3. **Stdlib-only parsing.** The pack is JSON (`encoding/json`). Globs use the existing `matchGlob`, where `**` already matches zero directories (`internal/domain/coverage/evaluate.go`).
4. **Additive events.** Check names become pack data. The `gate` event keeps its fields; per-module or per-seed notes are new fields (AGENTS.md non-negotiable).

### 2.2 Annotated example: the Go pack, exactly as the gate behaves today

```jsonc
{
  "schema": 1,                                   // pack schema version
  "id": "go", "name": "Go",
  "versions": {"stable": "1.27", "lts": null, "as_of": "2026-11"},
  "detect": [{"file": "go.mod"}],                // init picks the first pack (and variant) whose detect matches
  "variants": [],                                // none for Go (see 2.3)
  "scope": {"pkgs": ["./..."], "pkg_dirs": ["."]},    // defaults for {pkgs} and {pkg_dirs}; --pkgs replaces them
  "source_globs": ["**/*.go"],
  "test_globs": ["**/*_test.go"],
  "generated": {"globs": ["**/*.pb.go", "**/*_gen.go", "**/zz_generated*"],
                "header_regex": "^// Code generated .* DO NOT EDIT\\.$"},
  "env": {},                                     // flag-valued env for every check (was csharp check_env)
  "checks": [                                    // order = run order = CheckNames today
    {"name": "build", "kind": "build", "argv": ["go", "build", "{pkgs}"], "required": true, "tool": "go"},
    {"name": "gofmt", "kind": "format", "argv": ["gofmt", "-l", "{pkg_dirs}"], "required": false, "tool": "gofmt",
     "fails_on": "output", "output_exclude": ["vendor/", "testdata/"]},          // = filterFmt
    {"name": "vet", "kind": "lint", "argv": ["go", "vet", "{pkgs}"], "required": true, "tool": "go"},
    {"name": "staticcheck", "kind": "lint", "argv": ["staticcheck", "{pkgs}"], "required": false, "tool": "staticcheck",
     "install": "go install honnef.co/go/tools/cmd/staticcheck@latest",
     "skip_on": {"output_regex": "unsupported version|internal error in importing",
                 "note": "staticcheck too old for this Go toolchain — upgrade: go install honnef.co/go/tools/cmd/staticcheck@latest"}},
    {"name": "arch", "kind": "arch", "internal": "arch", "required": false},    // loops arch; no argv
    {"name": "test", "kind": "test", "required": true, "tool": "go",
     "argv": ["go", "test", "-race", "-count=1", "{coverage_args}", "{pkgs}"],
     "coverage_args": ["-covermode=atomic", "-coverprofile={coverage}"],         // dropped when no profile is requested
     "signal_regex": "^FAIL|--- FAIL|panic|DATA RACE|_test\\.go:"},              // = isTestSignal (trim keeps these first)
    {"name": "coverage", "kind": "coverage", "internal": "coverage", "required": true, "skippable": false},
    {"name": "govulncheck", "kind": "vuln", "argv": ["govulncheck", "{pkgs}"], "required": false,
     "tool": "govulncheck", "default": "off",                                    // runs only with --vuln or gate.vuln
     "install": "go install golang.org/x/vuln/cmd/govulncheck@latest"}
  ],
  "test_commands": [{"argv_prefix": ["go", "test"]}, {"argv_prefix": ["gotestsum"]}, {"argv_prefix": ["richgo", "test"]}],
  "race": {"mode": "flag", "flags": ["-race"], "env": {"GOFLAGS": "-race"},
           "checks": ["test"],                   // checks that carry the policy in the full gate
           "variants": [], "reason": "Go's built-in race detector; needs CGO_ENABLED=1 and a C compiler",
           "substitute": null},
  "coverage": {"format": "gocover", "path": "{coverage}", "produced_by": ["test"], "produce": [],
               "granularity": "statement",
               "exclude": ["cmd/**", "internal/compose/**", "**/testdata/**"]},  // + generated.globs = DefaultExclude
  "post_edit": {"format_file": ["gofmt", "-w", "{file}"], "lint_dir": ["go", "vet", "./{dir}"]},
  "cache_env": {"GOCACHE": "cache", "GOTMPDIR": "tmp"},  // "cache"/"tmp" = <run>/<id>/cache|tmp = workspace.GoCache/GoTmp
  "shared_env": ["GOMODCACHE", "GOPATH"],        // documented; loops never relocates these
  "cache_argv": {},
  "env_hints": {"GOFLAGS": "-p=2"},
  "artifacts": ["*.test", "*.test.exe", "*.out", "*.prof", "*.pprof", "*.cov", "*.exe"],  // = artifactSuffixes today
  "arch": {"parser": "go", "import_regex": null, "resolver": "module-path", "native_check": null,
           "forbidden_in_domain": ["os", "net/http"], "third_party_forbidden_in": ["domain", "application", "pkg"],
           "impure_calls": ["time.Now", "math/rand", "math/rand/v2", "go"],
           "size_warnings": {"type_methods": 40, "struct_fields": 25, "interface_methods": 8,
                             "module_lines": 12000, "module_files": 60}},
  "modules": null,                               // Go proposal: {"discover": ["go","work","edit","-json"], "marker": "go.mod"}
  "test_jobs": "-p=2",
  "placeholders": {},                            // pack-declared extra placeholders (2.4)
  "config_files": [".golangci.yml", ".golangci.yaml", ".golangci.toml", ".golangci.json", ".editorconfig"],
  "sources": {"checks": "https://pkg.go.dev/cmd/go", "race": "https://go.dev/doc/articles/race_detector"}
}
```

Two values differ from go.md's research block:
- **`artifacts`** keeps today's seven suffixes. go.md adds `coverage*.txt`, `cover*.out` and `__debug_bin*`, which is a change.
- **`arch.forbidden_in_domain`** keeps `os` and `net/http`. Adding `database/sql` is go.md's proposal, GO-ARCH-02.

### 2.3 Feature snippets from real packs

```jsonc
// Build-tool variants (java Maven/Gradle; csharp VSTest/MTP; replaces "variants" and "test_variants")
"variants": [
  {"name": "gradle", "detect": [{"file": "build.gradle.kts"}, {"file": "build.gradle"}],
   "checks": [{"name": "test", "argv": ["./gradlew", "--no-daemon", "test", "--rerun", "jacocoTestReport"]}],  // merged by name
   "coverage": {"path": "build/reports/jacoco/test/jacocoTestReport.xml"},
   "cache_argv": {"./gradlew": ["--project-cache-dir", "{run}/.gradle"]}},
  {"name": "mtp", "detect": [{"file": "global.json", "contains": "Microsoft.Testing.Platform"}],
   "checks": [{"name": "test", "argv": ["dotnet", "test", "--no-build", "-c", "Release", "--results-directory", "{coverage_dir}",
               "--minimum-expected-tests", "1", "--", "--coverage", "--coverage-output-format", "cobertura"]}]}
]

// Conditional checks, exit-code maps, per-check env (php; erlang elvis; python uv)
{"name": "php-cs-fixer", "kind": "format", "argv": ["vendor/bin/php-cs-fixer", "check", "--diff", "--using-cache=no", "--no-interaction"],
 "when_file": [".php-cs-fixer.dist.php", ".php-cs-fixer.php"], "exit_codes": {"0": "pass", "8": "fail"}},
{"name": "test", "kind": "test", "argv": ["vendor/bin/phpunit", "--order-by=random", "--coverage-cobertura", "{coverage}"],
 "env": {"XDEBUG_MODE": "coverage"}},
{"name": "sync", "kind": "deps", "argv": ["uv", "sync", "--locked"], "when_file": ["uv.lock"],
 "fallback_argv": ["python", "-m", "pip", "install", "-e", ".", "--group", "dev"]}

// Output-based failure across lines (csharp: the vuln list exits 0)
{"name": "vuln", "kind": "vuln", "argv": ["dotnet", "package", "list", "--vulnerable", "--include-transitive", "--format", "json"],
 "fail_regex": "\"vulnerabilities\"\\s*:\\s*\\[\\s*\\{"}          // matched against the whole output, not line by line

// Repeats with seeds, required env, pack placeholders (clojure)
{"name": "test-repeat", "kind": "test", "argv": ["clojure", "-M:test", "--randomize", "--seed", "{seed}"], "repeat": 3},
{"name": "nvd", "kind": "vuln", "argv": ["clojure", "-Tnvd", "nvd.task/check", ":classpath", "\"{classpath}\""],
 "env_required": ["NVD_API_TOKEN"]},
"placeholders": {"classpath": {"argv": ["clojure", "-Spath", "-M:test"]}},
"race": {"mode": "none", "checks": ["test-repeat"], "substitute": "Kaocha randomised order, 3 runs with different seeds"}

// Race variants (erlang): every race check runs once per env map
"race": {"mode": "none", "checks": ["eunit", "ct"], "variants": [{"ERL_FLAGS": "+S 1:1"}, {"ERL_FLAGS": "+S 8:8"}]},
"coverage": {"format": "cobertura", "path": "_build/test/covertool/*.covertool.xml",           // glob, merged
             "produced_by": ["eunit", "ct"], "produce": [{"argv": ["rebar3", "covertool", "generate"]}]}

// Multi-step coverage export and argv-based cache scoping (swift)
"placeholders": {"codecov_dir": {"argv": ["swift", "test", "--show-codecov-path"], "dirname": true},
                 "bin_path": {"argv": ["swift", "build", "--show-bin-path"]},
                 "test_objects": {"glob": ["{bin_path}/*-test-runner", "{bin_path}/*.xctest/Contents/MacOS/*"]}},
"coverage": {"format": "lcov", "path": "{coverage}", "produced_by": ["test"],
             "produce": [{"argv": ["llvm-cov", "export", "-format=lcov", "-instr-profile={codecov_dir}/default.profdata",
                                    "-ignore-filename-regex=(\\.build|Tests)/", "{test_objects}"], "stdout_to": "{coverage}"}]},
"cache_env": null, "cache_argv": {"swift": ["--scratch-path", "{run}/swift/build"]},

// Values read from a manifest (rust post-edit)
"placeholders": {"edition": {"file": "{manifest_dir}/Cargo.toml", "regex": "edition\\s*=\\s*\"(\\d+)\""}},
"manifest": "Cargo.toml",                         // {manifest_dir} = nearest ancestor of {file} holding it
"post_edit": {"format_file": ["rustfmt", "--edition", "{edition}", "{file}"],
              "lint_dir": ["cargo", "clippy", "--manifest-path", "{manifest_dir}/Cargo.toml", "--all-targets", "--", "-D", "warnings"]}
```

### 2.4 Field table

| Field | Type | Meaning | Origin | Go value |
|---|---|---|---|---|
| `schema` | int | pack schema version | new | 1 |
| `id`, `name`, `versions` | | as in the draft | draft | |
| `detect[]` | `{file}` or `{glob}`, optional `contains` | init and variant selection; `contains` is a substring match on the file | draft + csharp MTP | `go.mod` |
| `variants[]` | `{name, detect, ...partial pack}` | first matching variant overlays the base; `checks` merge by `name` | java `variants`, csharp `test_variants` | `[]` |
| `scope` | `{pkgs[], pkg_dirs[]}` | defaults for `{pkgs}`/`{pkg_dirs}`; `--pkgs` overrides | new (fmtDirs, `./...`) | `./...`, `.` |
| `source_globs`, `test_globs`, `generated` | | as in the draft; coverage candidates and ownership use them | draft | |
| `manifest` | string | file that defines `{manifest_dir}` | rust `{crate_dir}` | absent |
| `env` | map | flag-valued env for every check | csharp `check_env` | `{}` |
| `checks[].name`, `kind`, `argv`, `required`, `tool`, `install` | | `kind` ∈ `deps`, `build`, `format`, `lint`, `typecheck`, `arch`, `test`, `coverage`, `vuln`, `sast`, `sbom` | draft + python, ruby, ts, go, erlang, swift | |
| `checks[].internal` | `"arch"` or `"coverage"` | loops-internal check, no argv | go | arch, coverage |
| `checks[].default` | `"on"` or `"off"` | `off` runs only when enabled (`--vuln`, `gate.vuln`, or config `checks.enable`) | go `proposed_checks`/`proposal`, `Options.Vuln` | govulncheck `off` |
| `checks[].skippable` | bool | `false` ignores CLI `--skip`; config skip still fails the full gate if `required` | go (`selected`) | coverage `false` |
| `checks[].when_file` | string list | run only if one of these paths (file or dir) exists; otherwise skip, or run `fallback_argv` | python, php, erlang; ruby rspec/rails | absent |
| `checks[].fallback_argv` | argv | used when `when_file` doesn't match; the override is logged | python | absent |
| `checks[].env` | map | env for this check only | php, elixir, python, rust | absent |
| `checks[].env_required` | string list | a missing var skips an optional check, or fails a required one, with a note | clojure | absent |
| `checks[].fails_on` | `"exit"` (default) or `"output"` | `output`: any output left after `output_exclude` fails the check | draft, cpp, rust, swift | gofmt `output` |
| `checks[].output_exclude` | substrings | lines dropped before the `fails_on: output` test | go (`filterFmt`) | `vendor/`, `testdata/` |
| `checks[].fail_regex` | regex | fail if it matches the whole output (multi-line), even on exit 0 | csharp `output_regex_multiline` | absent |
| `checks[].exit_codes` | map code → `pass`/`fail`/`skip` | non-standard exit codes | php, clojure (kondo 2/3) | absent |
| `checks[].skip_on` | `{output_regex, note}` | turn a known tool failure into a skip with a note | go (stale staticcheck) | staticcheck |
| `checks[].coverage_args` | argv | spliced in at `{coverage_args}` only when coverage is collected | go (`CoverProfile != ""`) | test |
| `checks[].signal_regex` | regex | failure lines kept first when output is trimmed | go (`isTestSignal`) | test |
| `checks[].repeat` | int | run N times; `{seed}` gets a new value each time and is recorded | clojure `repeat`/`{seed}`, scala 3× | absent |
| `security` | `{checks[]}` or `{none}` | the pack's security scanners for `loops gate --security`: names of its `vuln`/`sast` checks (pack order), or `none` with the reason there is no scanner; validated by `Parse`, required by `RequireSecurity` | new | `gosec`, `govulncheck` (W1-gate) |
| `test_commands[]` | `{argv_prefix}` | what race-guard and the test guard treat as a test run | draft | 3 prefixes |
| `race.mode` | `flag`/`build`/`none` | CONC-04 | draft | `flag` |
| `race.flags`, `race.env` | | `flag` mode: what race-guard requires on test commands | draft | `-race`, `GOFLAGS` |
| `race.checks` | names | the checks that carry the policy in the full gate; all required | new (replaces wiring the substitute in prose) | `["test"]` |
| `race.variants` | env-map list | each race check runs once per map | erlang | `[]` |
| `race.reason`, `race.substitute` | text | for docs and the gate note | draft | |
| `coverage.format` | `gocover`/`lcov`/`cobertura`/`jacoco`/`simplecov-json` | parser selection (§3) | draft + ruby | `gocover` |
| `coverage.path` | `{coverage}`, `{coverage_dir}/…`, or repo-relative glob | multiple matches are merged; stale matches are deleted before the producing checks run | ts, java, csharp, scala, erlang | `{coverage}` |
| `coverage.produced_by` | names | checks that must have run, as `testRan` does today | go (`testRan`) | `["test"]` |
| `coverage.produce[]` | `{argv, stdout_to?}` | post-test export steps | swift `produce`, erlang `cover`/`covertool` checks | `[]` |
| `coverage.exclude` | globs | replaces `DefaultExclude` (plus `generated.globs`); config only adds | go `default_exclude` | 3 globs |
| `coverage.source_roots` | dirs | JaCoCo/Cobertura path resolution (§3) | new | absent |
| `coverage.granularity` | | informational | draft | statement |
| `post_edit` | `{format_file, lint_dir}` | per-file hook; `null` when no single-file formatter exists | draft | |
| `cache_env` | map var → `cache`/`tmp`/template | per-run; templates may use `{run}`, `{cache}`, `{tmp}` | draft; java, scala, clojure templates | GOCACHE, GOTMPDIR |
| `shared_env` | names | never relocated; documented | rust `shared_not_scoped`, ruby/java/kotlin/scala `"shared"` values | GOMODCACHE, GOPATH |
| `cache_argv` | map tool → argv | inserted after the tool name for cache flags | kotlin `cache_argv`, swift `cache_flags` | `{}` |
| `env_hints` | map | advisory env the roles print | go | `GOFLAGS=-p=2` |
| `artifacts` | globs; a trailing `/` marks a directory | cleanup candidates (§5) | draft | 7 suffixes |
| `arch.parser` | `go`, `regex`, or `none` | the built-in parser to use | new | `go` |
| `arch.import_regex`, `resolver` | | for `regex` parsers | draft | |
| `arch.native_check` | check name or null | an `arch`-kind check that enforces direction natively | python/ruby/elixir/php `native_tool` | null |
| `arch.forbidden_in_domain`, `impure_calls`, `third_party_forbidden_in`, `size_warnings` | | archrule data | draft + go | |
| `modules` | `{discover argv, marker}` or null | per-module gating (§7, D19) | go `multi_module` | null |
| `placeholders` | map name → `{argv, dirname?}`, `{file, regex}` or `{glob[]}` | pack-declared values resolved before a check | clojure `{classpath}`, swift `{codecov_dir}`/`{test_binary}`, rust `{edition}` | `{}` |
| `test_jobs`, `config_files`, `sources` | | as in the draft | draft | |

**Built-in placeholders:**
- `{file}`, `{dir}`, `{manifest_dir}`: post-edit;
- `{pkgs}`, `{pkg_dirs}`: scope, list-valued;
- `{coverage}` (a file) and `{coverage_dir}` (a dir): both under the run directory;
- `{coverage_args}`: list-valued;
- `{run}`, `{cache}`, `{tmp}`: the runspace;
- `{seed}`: one per repeat.

A list-valued placeholder must be a whole argv element, and it expands in place, possibly to nothing.

### 2.5 Extensions dropped or merged

| Extension (file) | Decision | Why |
|---|---|---|
| `test_variants` (csharp) | merged into `variants` | the same mechanism: an overlay chosen at detect time |
| `check_env` (csharp) | renamed `env` (top level) | per-check `env` already exists; top-level is the pack-wide form |
| `fails_on: "output_regex_multiline:<re>"` (csharp) | split into `fail_regex` | a regex inside a string enum can't be validated; matching is always whole-output |
| `exit_codes` (php) | kept, values normalised to `pass`/`fail`/`skip` | php's `"clean"`/`"files need fixing"` are labels, not outcomes |
| `{results}` (csharp), `{coverage_dir}` (ts), clojure's `-o` dir | merged into `{coverage_dir}` | the same concept: a per-run output directory |
| `{crate_dir}` (rust) | generalised to `{manifest_dir}` + `manifest` | Elixir umbrella apps and Maven modules need the same lookup |
| `{edition}`, `{classpath}`, `{codecov_dir}`, `{test_binary}` | became pack-declared `placeholders` | keeps loops free of per-language code |
| `{crate}`, `{host_triple}` (rust `race.build`), `{binary}` (go proposal) | dropped for v1 | they need per-crate iteration or a built binary; the tiers they serve are optional (D3) |
| `{base}` and `coverage.diff_tool` (python) | dropped | loops computes new-code coverage itself (stdlib only, D23) |
| `coverage.alternate`, `alt`, `default_path`, `data_file`, `notes`, `mtp_default_path`, `vstest_default_path`, `exec_relocation` | dropped from the schema | documentation; `exec_relocation` lives in java's `cache_env.MAVEN_ARGS` |
| `coverage.produce` (swift), erlang `kind: coverage` checks `cover`/`covertool` | merged into `coverage.produce` | export steps, not gate checks |
| kotlin `kover-verify` (`kind: coverage`) | dropped | a whole-project threshold that duplicates the loops new-code check |
| `kind: install` (ts), `kind: deps` (go proposal) | merged into `deps` | frozen install or lock verify, run first (D11) |
| `kind: supply` (java) | merged into `sbom` | same purpose as erlang and swift `sbom` |
| `race.build` (rust) | became optional checks with `default: off`, not in `race.checks` | an opt-in tier for `unsafe`/atomics crates (D3) |
| `race.substitute` as the only machine field | kept as text; `race.checks` is the machine part | prose can't be enforced |
| `repeat` (clojure) | kept | needed by clojure, scala, ts and elixir substitutes |
| `cache_flags` (swift), `cache_argv` (kotlin) | merged into `cache_argv` keyed by tool | same purpose: some build tools take cache dirs only as flags |
| `shared_not_scoped` (rust), `"shared"`-style values in `cache_env` (ruby, java, kotlin, scala) | merged into `shared_env` | `cache_env` values must be dirs loops creates; "shared" isn't one |
| `cache_env` values with `<run>`/`<dir>` (java, scala) | rewritten to `{tmp}`/`{run}` templates | one placeholder syntax |
| `proposed_checks`, `proposal: true` (go) | dropped from packs; a real check uses `default: off` | packs describe what runs |
| `multi_module` (go) | renamed `modules` | |
| `env_hints` (go) | kept | |
| `note`/`notes` on checks | kept as free text, ignored by loops | |

## 3. Coverage formats

| Format | Languages (primary) | Also possible | Parser quirks found in spot runs |
|---|---|---|---|
| `gocover` | go | — | file names carry the module path; strip it via `go.mod` (`Coverage.Module` today) |
| `lcov` | typescript, cpp, swift, rust, elixir, clojure | python (`coverage lcov`), ruby (simplecov-lcov), csharp (coverlet `Format=lcov`, B2 spot run) | **Elixir:** excoveralls writes absolute `SF:/tmp/hello/lib/hello.ex`. **Ruby:** simplecov-lcov writes `SF:./lib/hello.rb`. **Clojure:** cloverage writes relative `SF:src/…`. **Rust:** `cargo llvm-cov` exits 1 if the `--output-path` parent is missing. **Swift:** a two-step export; the test objects differ by OS (Linux `<T>.so`/`<T>-test-runner` under `.build/out/Products/Debug-linux-<arch>`, macOS `<T>.xctest/Contents/MacOS/<T>`, `xcrun` prefix). **TS:** untested files are absent unless `coverage.include` is set. **C++:** gcovr needs `--object-directory`; Clang needs `llvm-profdata merge` + `llvm-cov export` |
| `cobertura` | python, csharp, scala, erlang | php (`--coverage-cobertura`, exit 0 in the B2 spot run), rust (`--cobertura`), cpp (gcovr `--cobertura`) | **C#:** VSTest writes `{dir}/<guid>/coverage.cobertura.xml` and MTP writes `{dir}/<guid>.cobertura.xml`, so `**` must match zero dirs (the existing `matchGlob` does). Projects must not share one output path. **Erlang:** one file per OTP app plus the umbrella root (`_build/test/covertool/{a1,a2,um}.covertool.xml`), to be merged. **Scala:** the path is `target/scala-*/coverage-report/cobertura.xml`, not `scoverage-report/` (B2 round 1 finding 3); sbt 2 moves `target/`. Resolve `filename` against `<sources>` |
| `jacoco` | java, kotlin (Kover writes the JaCoCo schema) | — | **Java:** the Maven report path can't be redirected by a user property, so it is a glob (`**/target/site/jacoco/jacoco.xml`), one per module. Relocating the exec needs both `jacoco.destFile` and `jacoco.dataFile`, or the report reads stale data (B2 spot run). **Kotlin:** Gradle needs `test --rerun` or the report is stale (`:test UP-TO-DATE`). JaCoCo records `<package name>` + `<sourcefile name>`, not a repo path, so resolve them against the source roots (`src/main/java`, `src/main/kotlin`) |
| `clover` | php | — | `<file name>` paths are absolute and must be made relative to the repo root |
| `simplecov-json` | ruby | — | `coverage/coverage.json` (coverage-v1.3 schema) is written by SimpleCov 1.x with no extra gem. `add_filter` is deprecated in favour of `skip` |

**Recommended minimal parser set: 4 formats.** `gocover` exists today; `lcov`, `cobertura` and `jacoco` are new.
- These cover 14 of 15 languages as primaries.
- PHP switches to `--coverage-cobertura`, which the spot run verified, so Clover is dropped.
- Ruby uses simplecov-lcov: one dev gem, and its `./` prefix gets normalised. If adding a project gem isn't acceptable, `simplecov-json` becomes a small fifth parser (D6).

Every parser must:
- normalise paths to repo-relative form (strip the absolute root, `./`, or the module prefix; resolve Cobertura `<sources>` and JaCoCo package + file names);
- merge several files;
- produce line ranges for the existing `judge`.

## 4. Race policy

### 4.1 Breakdown

| Mode | Language | What loops enforces | Substitute (from the pack) |
|---|---|---|---|
| `flag` | go | `-race` on every `go test` (today), plus the gate's `test` | — |
| `flag` | swift | `--sanitize=thread` on `swift test`; Swift 6 language mode | — |
| `build` | cpp | the `test-tsan` preset check in the full gate | TSan can't combine with ASan, so `asan-ubsan` is separate. TSan + coverage combine on GCC and Clang (B2 spot run) |
| `none` | python | the test check with `-p randomly -n 2 -W error`, `PYTHONDEVMODE=1` | optional free-threaded `PYTHON_GIL=0 pytest --parallel-threads=4 --iterations=10` |
| `none` | typescript | vitest shuffle + concurrent, printed seed; `--repeats=3` on changed tests; no `--retry.count` > 0 | node:test `--test-randomize` (Node 26.1+), jest `--randomize` |
| `none` | java | JUnit parallel + random order, twice with different seeds | jcstress/Lincheck if present |
| `none` | csharp | xUnit parallel collections on; repeat `Category=Concurrency` tests; CA2007/CA2012/CA2016/VSTHRD as errors | optional Microsoft Coyote |
| `none` | php | `--order-by=random` under two seeds, serial and `paratest --processes=N`; `--fail-on-warning --fail-on-risky` | a worker-mode smoke suite run twice in one process |
| `none` | ruby | `rspec --order random` (printed seed) or Minitest; parallel_tests; repeat the changed specs | — |
| `none` | kotlin | JUnit parallel + random via `junit-platform.properties` (Gradle `-D` doesn't reach the test JVM); `@RepeatedTest` | Lincheck; `runTest` virtual time |
| `none` | rust | `unsafe_code=forbid`, clippy `-D warnings` (`await_holding_lock`) | `race.build` tier: miri (`-Zmiri-many-seeds`), loom (`--cfg loom`), TSan (nightly, `-Zbuild-std`) |
| `none` | scala | the test argv 3× with parallel suites (`Test / testForkedParallel := true` under fork) | jcstress/Lincheck; `nonFlaky` fiber tests |
| `none` | elixir | `mix test --repeat-until-failure 5 --warnings-as-errors`, async modules, printed seed | — (Concuerror is stale: last release 2020) |
| `none` | erlang | eunit + ct under `ERL_FLAGS=+S 1:1` and `+S 8:8` (B2 confirmed `ERL_FLAGS` reaches the BEAM), `ct --repeat`, proper_statem for gen_statem/ETS modules | Concuerror advisory only |
| `none` | clojure | `test-repeat`: 3 Kaocha runs with `--randomize --seed {seed}` | concurrency-tagged tests ×20; clj-kondo, reflection warnings |

### 4.2 Enforcement proposal

race-guard denies today with exit 2 (`internal/adapters/inbound/hook/race.go`). Note: AGENTS.md says only read-guard and model-guard deny; that line predates or overlooks race-guard.

| Mode | race-guard (PreToolUse, per command) | gate (full run) |
|---|---|---|
| `flag` | As today, but data-driven: a command matching `test_commands` must carry `race.flags`, or `race.env` must be set in the command's environment (`GOFLAGS=-race`). The existing shell parsing (`embedded`, `checkRaceLine`) stays. | `race.checks` are required; their argv contains the flags. |
| `build` | Fail open. A `ctest` against a non-TSan preset is legitimate (the ASan run). The guard may add a one-line reminder but never denies. | `race.checks` (`test-tsan`) are required. A full gate that skipped them is not green, as with `RequiredChecks` today. |
| `none` | Deny only the anti-patterns that defeat the substitute, matched against `test_commands`: a retry flag (TS `--retry.count` > 0), a fixed order (`--no-randomize`, `--order defined`). Everything else is allowed: the policy is checked in the gate, not per command (GENERIC §5.2). | Every check in `race.checks` runs (× `repeat`, × `race.variants`) and must pass. The gate event records the seeds and variants as new additive fields. A skipped race check fails the full gate. |

**Preflight (all modes).** If the policy can't run on the host (Go without cgo, TSan unsupported on the platform, Swift TSan on macOS libc++), the gate fails with "race policy unavailable: …". It never skips silently (D24).

## 5. Run-scoped caches

**The distilled rule.** Keep download and package caches **shared**: concurrent runs are safe because the tools file-lock them (Cargo and Gradle say so explicitly), and scoping them re-downloads the world. Scope **build outputs and tmp** per run. Clean what can't be redirected.

| Lang | Scoped per run | Shared (never relocated) | Cleaned instead | Why / spot-run note |
|---|---|---|---|---|
| go | `GOCACHE`, `GOTMPDIR` | `GOMODCACHE`, `GOPATH` | `*.test`, `*.prof`, … | today's behaviour |
| python | `PYTHONPYCACHEPREFIX`, `COVERAGE_FILE`, `MYPY_CACHE_DIR`, `RUFF_CACHE_DIR` | **should be** `UV_CACHE_DIR`, `PIP_CACHE_DIR` (the pack scopes them; PY-OPS-06 warns of a cold re-download) | `.pytest_cache`, `__pycache__` | the pack disagrees with the rule (see "Inconsistencies") |
| typescript | `TMPDIR` | **should be** `npm_config_cache`, the pnpm store, `YARN_CACHE_FOLDER`, `BUN_INSTALL_CACHE_DIR`, `DENO_DIR` (the pack scopes them) | `node_modules/.cache/`, `.eslintcache`, `coverage/` | `node_modules/.cache` lives in the tree |
| java | `java.io.tmpdir` (`JAVA_TOOL_OPTIONS`), the JaCoCo exec, Gradle `--project-cache-dir` | `~/.m2/repository`, `GRADLE_USER_HOME` | `target/`, `build/` | `-Djacoco.destFile` alone reads stale data |
| cpp | `CCACHE_TEMPDIR`; the build dir is a preset `binaryDir` | `CCACHE_DIR`, `VCPKG_DEFAULT_BINARY_CACHE`, `VCPKG_DOWNLOADS`, `CONAN_HOME` | `build/`, `CMakeFiles/`, `*.gcda` | the vcpkg binary cache dir must exist first |
| csharp | `DOTNET_CLI_HOME`, `TMPDIR`; build servers off (`MSBUILDDISABLENODEREUSE=1`) | **should be** `NUGET_PACKAGES`, `NUGET_HTTP_CACHE_PATH` (the pack scopes them; CS-OPS-05 says "holds downloads, so seed or copy it") | `bin/`, `obj/`, `TestResults/` | MSBuild/Roslyn servers outlive the run otherwise |
| php | `TMPDIR`; PHPUnit/PHPStan/CS-Fixer cache via flags; `APP_CACHE_DIR` (Symfony) | **should be** `COMPOSER_CACHE_DIR` (the pack scopes it); `COMPOSER_HOME` holds auth | `.phpunit.cache/`, `var/cache/`, Laravel `bootstrap/cache` (no env) | |
| ruby | `BOOTSNAP_CACHE_DIR`, `TMPDIR` | `BUNDLE_PATH`, `BUNDLE_USER_CACHE` | `coverage/`, `tmp/` | |
| swift | `--scratch-path` (argv; there are no env vars) | `--cache-path`, `--config-path`, `--security-path` | `.build/`, `.swiftpm/` | a flag on every swift call |
| kotlin | `--project-cache-dir {run}/.gradle` (argv; `ORG_GRADLE_PROJECT_*` has no effect, spot run) | `GRADLE_USER_HOME`, or a per-run home seeded from `GRADLE_RO_DEP_CACHE` | `build/`, `.kotlin/` | a fresh home re-downloads and starts a new daemon |
| rust | `CARGO_TARGET_DIR`, `TMPDIR`, `CARGO_INCREMENTAL=0` | `CARGO_HOME`, `RUSTUP_HOME`, `SCCACHE_DIR` | `target/llvm-cov-target` if it isn't redirected (RUST Open question 8) | a cold target dir recompiles everything (D15) |
| scala | `java.io.tmpdir` | `COURSIER_CACHE`, sbt boot | `target/`, `.bsp/`, `.bloop/` (sbt 1 can't redirect `target/` cheaply) | |
| elixir | `MIX_BUILD_ROOT` (preferred over `MIX_BUILD_PATH`) | `deps/`, `MIX_HOME`, `HEX_HOME` | `_build`, `cover` | a per-run build root recompiles the deps |
| erlang | `ERL_CRASH_DUMP` (a *file* var: use `{tmp}/erl_crash.dump`) | `REBAR_CACHE_DIR` (also holds the PLT and project plugins, spot run), `REBAR_GLOBAL_CONFIG_DIR` | `_build`, `ct_logs` | |
| clojure | `java.io.tmpdir` | `~/.m2`, `~/.gitlibs` | `./.cpcache` (`CLJ_CACHE` has no effect with a project `deps.edn`, spot run) | |

**Cleanup today deletes only regular files** that are new since the baseline and match an artifact name or an executable header. It never deletes directories (`internal/domain/workspace.Decide`), and it skips `vendor/` and `node_modules/` trees. Most pack artifacts are directories (`target/`, `_build/`, `obj/`, `.build/`), so cleanup needs a rule: delete an artifact directory only when the directory itself is new since the baseline (D25).

## 6. Arch

### 6.1 Built-in scanner or native tool

| Lang | A stdlib-Go regex scanner works? | Resolver | Native tool to delegate to | Recommendation |
|---|---|---|---|---|
| go | yes (AST today) | `module-path` | — | keep `goparse` |
| java | yes: one `import` per line, package = dir | `package-dir` | ArchUnit (runs in the test check) | scanner + ArchUnit when present |
| kotlin | yes: read the `package` header, not the dir (the dir isn't enforced) | `package-dir` | Konsist; Gradle module deps | scanner + Gradle module graph |
| scala | partly: chained `package a; package b` clauses and relative imports | `package-dir` | sbt `dependsOn` (compiler-enforced), ArchUnit | native first (subprojects), scanner for purity |
| csharp | yes: `using` lines + the `<ProjectReference>` graph (encoding/xml) | `project-reference` | NetArchTest / ArchUnitNET (in tests) | scanner on the project graph |
| php | yes: `use` + PSR-4 map from `composer.json` | `namespace-psr4` | deptrac | scanner; deptrac when configured |
| typescript | mostly: `import`/`export`/`import()`/`require`; needs `tsconfig` `paths` aliases | `relative-path` | dependency-cruiser | scanner + dependency-cruiser |
| cpp | yes: `#include` with include roots | `include-path` | CMake target graph (`--graphviz`); STATIC-lib cycles are allowed by CMake | scanner + target-graph cycle check |
| rust | yes: `use`/`extern crate` + `cargo metadata` JSON for crate edges | `crate-path` | `cargo metadata`, clippy `disallowed-*`, cargo-deny bans | crate graph first, scanner inside crates |
| swift | yes: `import` forms (SE-0409 access levels); targets from `swift package dump-package`, because `Package.swift` is code | `target-name` | `--explicit-target-dependency-import-check=error` | native (build) + scanner for purity |
| clojure | yes: `ns` `:require`/`:import` forms, dash → underscore | `dotted-module-underscore` | `poly check`, clj-kondo `:discouraged-namespace` | scanner; Polylith when present |
| python | no: relative imports and multi-line parenthesised imports need `ast` (PY-ARCH-03) | `dotted-module` | import-linter (`lint-imports`) | **delegate** |
| ruby | no: Zeitwerk constant references, no imports (RUBY-ARCH-06) | `native` | packwerk | **delegate** |
| elixir | no: `alias`/`import`/`use` are compile-time; boundaries are a compiler concern | `native` | boundary (compiler), `mix xref graph --format cycles` | **delegate** |
| erlang | no: `mod:fun(` regex false positives on atoms and maps (ERL Open question 7) | `module-name` | `rebar3 xref` + `.app.src` `applications` | **delegate** (or `erl_scan` later) |

### 6.2 Forbidden-in-domain and impure calls, per pack

This is the pack data verbatim. Go's entries are today's checker (`internal/domain/archrule`).

| Language | Resolver | Forbidden in domain | Impure calls |
|---|---|---|---|
| python | `dotted-module` | `os`, `requests`, `httpx`, `sqlalchemy`, `django`, `fastapi`, `flask`, `boto3`, `pydantic`, `subprocess`, `socket` | `datetime.now`, `datetime.utcnow`, `time.time`, `random.`, `uuid.uuid4`, `os.environ`, `threading.Thread`, `asyncio.create_task` |
| typescript | `relative-path` | `node:fs`, `node:net`, `node:http`, `node:https`, `node:child_process`, `node:crypto`, `fs`, `http`, `child_process`, `@nestjs/*`, `express`, `fastify`, `hono`, `pg`, `@prisma/client`, `typeorm` | `Date.now`, `new Date(`, `Math.random`, `crypto.randomUUID`, `process.env`, `fetch(`, `setTimeout(`, `setInterval(` |
| java | `package-dir` | `org.springframework`, `jakarta.persistence`, `javax.persistence`, `jakarta.validation`, `com.fasterxml.jackson`, `java.sql`, `java.net.http`, `java.io`, `java.nio.file` | `System.currentTimeMillis`, `Instant.now`, `LocalDate.now`, `LocalDateTime.now`, `ZonedDateTime.now`, `UUID.randomUUID`, `new Random`, `Math.random`, `new Thread`, `Executors.` |
| cpp | `include-path` | `fstream`, `filesystem`, `thread`, `mutex`, `future`, `iostream`, `sys/socket.h`, `unistd.h`, `boost/asio`, `boost/beast`, `grpcpp`, `pqxx`, `drogon`, `curl/curl.h` | `std::chrono::system_clock::now`, `std::chrono::steady_clock::now`, `std::random_device`, `std::rand(`, `getenv(`, `std::cout`, `std::cerr`, `std::thread`, `std::async` |
| csharp | `project-reference` | `System.IO`, `System.Net.Http`, `System.Data`, `Microsoft.EntityFrameworkCore`, `Microsoft.AspNetCore`, `Microsoft.Extensions.Logging`, `System.Diagnostics.Process` | `DateTime.Now`, `DateTime.UtcNow`, `DateTimeOffset.Now`, `DateTimeOffset.UtcNow`, `Guid.NewGuid`, `Random.Shared`, `new Random(`, `Environment.GetEnvironmentVariable`, `Task.Run`, `Thread.Sleep` |
| php | `namespace-psr4` | `Illuminate\`, `Symfony\Component\HttpFoundation\`, `Symfony\Component\HttpKernel\`, `Doctrine\ORM\`, `Doctrine\DBAL\`, `PDO`, `Psr\Http\Message\`, `GuzzleHttp\` | `new DateTime(`, `new DateTimeImmutable(`, `date(`, `time(`, `microtime(`, `rand(`, `mt_rand(`, `random_int(`, `uniqid(`, `getenv(`, `$_ENV`, `$_SERVER`, `file_get_contents(`, `curl_init(` |
| go | `module-path` | `os`, `net/http` | `time.Now`, `math/rand (global funcs)`, `math/rand/v2 (global funcs)`, `go statement` |
| ruby | `native` | `ActiveRecord`, `ActionController`, `Rails`, `Net::HTTP`, `Faraday`, `ENV`, `File`, `Sidekiq` | `Time.now`, `Time.current`, `Date.today`, `DateTime.now`, `rand`, `SecureRandom`, `Random.new`, `Thread.new`, `sleep`, `puts` |
| swift | `target-name` | `NIO`, `NIOCore`, `NIOHTTP1`, `Vapor`, `Fluent`, `Hummingbird`, `PostgresNIO`, `SQLKit`, `AsyncHTTPClient`, `FoundationNetworking` | `Date()`, `Date.now`, `UUID()`, `.random(`, `Task {`, `Task.detached`, `URLSession`, `FileManager`, `ProcessInfo` |
| kotlin | `package-dir` | `java.sql`, `java.net`, `java.io`, `javax.persistence`, `jakarta.persistence`, `org.springframework`, `io.ktor`, `com.fasterxml.jackson` | `Instant.now`, `LocalDateTime.now`, `LocalDate.now`, `System.currentTimeMillis`, `UUID.randomUUID`, `Random.`, `GlobalScope`, `Thread.` |
| rust | `crate-path` | `std::fs`, `std::net`, `std::env`, `std::process`, `std::thread`, `tokio`, `sqlx`, `diesel`, `axum`, `actix_web`, `reqwest`, `hyper`, `tracing` | `SystemTime::now`, `Instant::now`, `Utc::now`, `OffsetDateTime::now_utc`, `Uuid::new_v4`, `rand::`, `thread_rng`, `tokio::spawn`, `thread::spawn` |
| scala | `package-dir` | `java.io`, `java.net`, `java.sql`, `java.nio.file`, `scala.concurrent`, `cats.effect`, `zio`, `fs2`, `org.http4s`, `doobie`, `skunk`, `slick`, `play`, `org.apache.pekko`, `akka` | `Instant.now`, `LocalDateTime.now`, `System.currentTimeMillis`, `System.nanoTime`, `scala.util.Random`, `java.util.UUID.randomUUID`, `Thread.sleep`, `Await.result`, `sys.env`, `null`, `var ` |
| elixir | `native` | `Ecto.Repo`, `Ecto.Query`, `Phoenix`, `Plug`, `Logger`, `File`, `System`, `Oban` | `DateTime.utc_now`, `DateTime.now`, `NaiveDateTime.utc_now`, `Date.utc_today`, `System.os_time`, `System.system_time`, `Enum.random`, `:rand.`, `spawn`, `Task.async`, `Task.start`, `send(`, `:ets.` |
| erlang | `module-name` | `gen_server`, `gen_statem`, `gen_event`, `supervisor`, `ets`, `dets`, `mnesia`, `file`, `gen_tcp`, `gen_udp`, `httpc`, `ssl`, `os`, `application`, `persistent_term`, `logger` | `os:timestamp`, `erlang:system_time`, `erlang:monotonic_time`, `erlang:now`, `rand:`, `crypto:strong_rand_bytes`, `erlang:send`, `spawn`, `self()`, `!` |
| clojure | `dotted-module-underscore` | `clojure.java.io`, `clojure.java.shell`, `clojure.java.jdbc`, `next.jdbc`, `honey.sql`, `ring.`, `reitit.`, `org.httpkit`, `clj-http`, `hato`, `clojure.core.async` | `System/currentTimeMillis`, `Instant/now`, `LocalDate/now`, `(rand`, `random-uuid`, `UUID/randomUUID`, `(slurp`, `(spit`, `(future`, `(swap!`, `(reset!` |

### 6.3 Inconsistencies in the arch data

- Swift spells the import check two ways: the build check uses `--explicit-target-dependency-import-check error`, and `native_tool` uses `=error`. Both forms appear in the file.
- C#'s `resolver: project-reference` and Clojure's `dotted-module-underscore` are new resolver values. TAXONOMY lists neither, but both files explain them.
- Erlang's `!` and `self()` and Scala's `null` and `var ` in `impure_calls` are syntax, not calls. A substring scanner will report false positives on them.

## 7. Decisions for the maintainer before Stage C

The duplicates across the 15 Open questions sections are merged; each item names the files it comes from. "Rec." is the synthesizer's recommendation.

| # | Decision | Options | Rec. | Languages (source) |
|---|---|---|---|---|
| D1 | Does `race.mode = none` plus a substitute satisfy the hard "race or equivalent" rule? | (a) yes, substitute checks are required gate checks; (b) yes, advisory only; (c) no, require language-specific concurrency tests (Lincheck, jcstress, proper_statem) | **(a)**, with the wording in GENERIC §5.2. Language-specific stress tests stay review items | python 1, typescript, java, csharp, php 1, ruby, kotlin, rust 1, scala, elixir 4, erlang 4, clojure |
| D2 | Substitute cost: how often and on what | (a) N× the full suite; (b) one full randomised run + N repeats of the changed test files; (c) one run | **(b)** with N = 3 where the runner selects files (vitest, pytest, rspec, mix test, kaocha `--focus`, phpunit); else (a) with N = 2. Erlang: both scheduler variants on the changed apps | php 1, elixir 4, erlang 4, ruby, typescript, scala, clojure, java |
| D3 | Shape of `build` mode and optional tiers | C++: one merged `tsan+coverage` preset or separate builds. Rust: `race.build` tiers or `mode=build` per crate. Swift: TSan suppressions | merge TSan+coverage (verified on GCC and Clang) and keep `asan-ubsan` separate and required. Rust tiers are optional `default: off` checks. Allow `TSAN_OPTIONS=suppressions=` only through project config | cpp 1, rust 1, swift |
| D4 | Build-tool variants | (a) one pack with `variants` picked at detect; (b) separate packs (`java-maven`, `java-gradle`) | **(a)**. Kotlin and Java share the Gradle variant data | java 1, csharp 1, typescript (runner), clojure (lein), scala (mill, scala-cli), python (uv/pip), kotlin |
| D5 | Coverage path model | (a) `{coverage}` only; (b) `{coverage}`, `{coverage_dir}` and globs, merged, with stale matches deleted first | **(b)**; drop `{results}` | java 3, csharp 2, typescript, elixir 2, erlang 2, scala, clojure |
| D6 | Parser set and tool choice per format | Ruby simplecov-json parser vs. simplecov-lcov gem; PHP Clover vs. Cobertura; Swift lcov vs. SwiftPM JSON; C++ gcovr vs. llvm-cov | **4 parsers** (gocover, lcov, cobertura, jacoco). PHP uses Cobertura; Ruby uses simplecov-lcov; Swift lcov via llvm-cov; C++ picks gcovr or llvm-cov by compiler. Add simplecov-json only if project gems are refused | ruby, php, swift 2, cpp 4 |
| D7 | Does "a changed unit has no tests" apply outside Go? | (a) per-pack unit mapping; (b) Go only, the per-file rule elsewhere | **(b)** for v1 | all (synthesis) |
| D8 | Rust inline `#[cfg(test)]` tests inflate new-code coverage | accept the bias; `--ignore-filename-regex`; nightly `coverage(off)` | accept, and exclude `tests/` and `benches/` | rust 3 |
| D9 | Go `-coverpkg=./...` | keep the default; add it | keep the default and document the trade-off (GO_BEST §30) | go 1 |
| D10 | Status semantics: the Go file counts GO_BEST rules as `covered` | re-mark them `go-only`; keep them | keep: after Stage C, GO_BEST is the Go pack. MATRIX notes it | go 9 |
| D11 | Check kinds, and whether install belongs in the gate | the kind list; `install` mutates the tree | the kinds in §2.4; `deps` checks allowed only in frozen or verify form and run first | typescript, python 8, ruby, erlang 11, go |
| D12 | A tool or plugin is missing | fail; skip; fall back | optional checks skip with the `install` hint in the note. A required check uses `fallback_argv` if declared (logged), else fails. Missing coverage always fails | python 9, php 2 (coverage driver), clojure (NVD token), swift (SwiftLint on Linux), kotlin |
| D13 | Project-side prerequisites | Kotlin/Gradle `junit-platform.properties`; Clojure `:cov` alias + `tests.edn`; Elixir excoveralls; Erlang covertool + `cover_export_enabled`; Swift `.swift-format`; PHP coverage driver; Ruby SimpleCov start. Who adds them: loops `init`, the architect in wave 0, or the gate as a failure? | the architect adds them in wave 0 (an override, logged). The gate preflight fails with the exact snippet | kotlin, java 4, clojure, elixir, erlang, swift, php 2, ruby |
| D14 | Are format checks required? | the packs disagree: Go `gofmt` optional; Python, Rust and Scala required; others optional | optional by default; required when the formatter's config file is committed (`when_file`) | python, rust, scala vs. the rest |
| D15 | Per-run cache policy | scope download caches; share them | share downloads, scope outputs and tmp (§5). Fix the python, typescript, csharp and php packs. Rust: keep `CARGO_TARGET_DIR` per run (it is build output) and recommend a shared sccache to cut the cold start | rust 7, kotlin, python, typescript, csharp, php |
| D16 | Vulnerability tools, and whether vuln is required | Java OSV vs. dependency-check; Clojure nvd vs. clj-watson; Swift none; Erlang `rebar3 audit` vs. OSV; C++ OSV with `conan.lock` only; Go `govulncheck` silent | OSV-Scanner where it reads the lockfile, else the ecosystem tool. `vuln` is opt-in locally and required in a CI profile. A configured vuln check whose tool is missing fails instead of skipping | java 8, clojure, swift 3, erlang 3, cpp 5, go (GO-TOOL-06) |
| D17 | Competing tools in one slot | pick one; detect from config | detect from config files: mypy/pyright, Sorbet/Steep, detekt 1/2, staticcheck/golangci-lint, Biome/eslint, PER-CS/PSR-12, the Spring/Quarkus/Micronaut heuristic | python 4, ruby, kotlin, go 2, typescript, php 7, java 7 |
| D18 | Post-edit scope | format + lint; format only; nothing | format the one file only, with a single-file formatter, never the crate or module (rust R2-1). `null` when none exists (Java Spotless, Kotlin, sbt) | elixir 7, java 6, kotlin, scala, rust |
| D19 | Multi-module and umbrella gating | per module; tool aggregates | per module for Go only (`modules`); everyone else uses the tool's workspace mode. Event fields stay additive | go 5, elixir 8, erlang, clojure (Polylith ownership globs) |
| D20 | Version lines to assume | sbt 1 vs. 2; Maven 3 vs. 4; Node vs. Bun/Deno; Swift Build vs. native on Linux; Ruby 4.0 vs. 3.4 | target the current GA lines and read the project's pins; re-verify sbt 2 and Maven 4 paths when the pack is built | scala, java 2, typescript, swift 1, ruby |
| D21 | Concuerror | advisory; drop | drop it from the packs (stale: OTP 20.3–23.3) | erlang 1, elixir 4 |
| D22 | Commit `Cargo.lock` for libraries | yes; per project | yes | rust 5 |
| D23 | New-code coverage: compute in loops or shell out to diff-cover | compute; diff-cover | compute in loops (stdlib only); drop `diff_tool` | python, typescript, rust 4 |
| D24 | The race policy can't run on the host | skip with a warning; fail | fail with a clear preflight message | go 6, swift, cpp 7 |
| D25 | Cleanup of artifact directories (synthesizer-added; the code deletes files only) | files only; also new directories | delete an artifact directory only if it didn't exist at the baseline | all (from `workspace.Decide`) |

## 8. Inconsistencies found in the inputs

- **The race count.** The brief says 11 of 15 packs use `race.mode = none`. The blocks say **12**: Python, TypeScript, Java, C#, PHP, Ruby, Kotlin, Rust, Scala, Elixir, Erlang and Clojure. Go and Swift are `flag`; C++ is `build`.
- **Download caches.** The Python, TypeScript, C# and PHP `cache_env` scope download caches per run (`UV_CACHE_DIR`, `npm_config_cache`, `NUGET_PACKAGES`, `COMPOSER_CACHE_DIR`). The other ten packs share them (§5, D15).
- **Erlang `ERL_CRASH_DUMP: "tmp"`** maps a file variable to a directory token. The verifier noted it as non-blocking.
- **Ruby** has two required test checks (`rspec`, `rails-test`) and no `when_file`, so a gate would run both. Each only fits a project that has `spec/` or `test/`.
- **Elixir vs. Clojure.** Elixir marks its `test-repeat` substitute `required: false`; Clojure marks its own `required: true`. D1 settles it.
- **Prefix-only `test_commands`.** Java (`./mvnw`, `mvn`, `./gradlew`, `gradle`), Scala (`sbt`, `mill`) and Erlang (`rebar3 do`) match every build command, so race-guard can't use them as written.
- **Kotlin** marks KT-CONC-04 `missing`; every other file marks the race rule `go-only`.
- **GO_BEST §30 "Other languages".** Its table disagrees with the packs: Python `--cov-fail-under=70` is whole-project, and Java/Kotlin cite `jcstress` as the race tool (GENERIC §4).
- **Python and Clojure `source_globs`** (`**/*.py`, `**/*.clj`) include test files. The coverage candidates must subtract `test_globs`, which the generalised gate does (GENERIC §5.1).
- **Go readiness.** It isn't comparable with the other languages (MATRIX §1.2).
