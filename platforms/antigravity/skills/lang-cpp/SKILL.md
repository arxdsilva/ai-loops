---
name: lang-cpp
description: Use when writing, reviewing, testing or setting up C++ backend code in a loops run (the cpp language pack) - toolchain, layout, race policy, coverage, idioms and footguns.
---
# lang-cpp

The C++ language pack: `specs/languages/cpp/pack.json` (what the gate runs) and `specs/languages/cpp/RULES.md` (the rule book, IDs `CPP-<SEC>-NN`). Universal rules are not repeated here: layering is `specs/ARCHITECTURE.md`, SOLID is `specs/SOLID.md`, gates and the race rule are `WORKFLOW §6`.

## When it applies
`loops lang detect` reports `cpp` (a `CMakeLists.txt` or `CMakePresets.json` at the root). `loops lang show cpp` prints the resolved pack: the `clang` variant is picked when `CMakePresets.json` contains `fprofile-instr-generate` (Clang source-based coverage); otherwise the GCC/gcovr path applies. Bazel and Meson projects are not auto-detected: their test commands are recognised by the guard, but the checks assume CMake presets.

## Project rules first
Follow the project's own config over this pack: `CMakePresets.json` (its real preset names), `.clang-format`, `.clang-tidy`, `.clangd`, `vcpkg.json`, `conanfile.*`, `.editorconfig`, its style guide (Google, LLVM, Core Guidelines) and its exception policy, and the framework already in use (Boost.Asio/Beast, gRPC, userver, Drogon). Read them first (`config_files` in the pack). Every time you follow a project rule over a pack rule, run `loops run override --rule "<pack rule>" --project "<project rule, file:line>" --note "<what you did>"`. If the project's presets are not named `dev`, `tsan` and `asan-ubsan`, override the check argv in `.loops/config.json` and log it.

## Toolchain the gate runs
Agents run `loops gate`, or single checks while iterating (`loops gate --only build,test-tsan`). The checks, in order:

| Check | Command | Required |
|---|---|---|
| configure | `cmake --preset dev` | yes |
| build | `cmake --build --preset dev` (warnings as errors, RULES TOOL-04) | yes |
| clang-format | `git ls-files … \| xargs clang-format --dry-run -Werror` | yes when `.clang-format` is committed, else skipped |
| clang-tidy | `run-clang-tidy -p build/dev -quiet` | no (runs when `.clang-tidy` exists) |
| arch | `loops arch` (`#include` scan, `include-path` resolver) | no |
| test-tsan | `cmake --preset tsan && cmake --build --preset tsan && ctest --preset tsan --output-on-failure --no-tests=error` | yes (the race policy and the coverage run) |
| test-asan-ubsan | the same chain with the `asan-ubsan` preset | yes |
| coverage | new-code coverage from the lcov file, at least 70% | yes |
| cppcheck, osv-scanner | see pack | off by default (`--vuln` enables OSV when `conan.lock` exists) |

Cap parallelism: `CMAKE_BUILD_PARALLEL_LEVEL=2 CTEST_PARALLEL_LEVEL=2`. After each edit the post-edit hook runs `clang-format -i <file>` on that one file only; never run `clang-format -i` over the tree (it rewrites files other agents own, `specs/PARALLEL_WORK.md`). Never run `cmake` in the source tree: every build goes in a preset `binaryDir` under `build/` (CPP-REPO-04).

Caches: loops scopes only `CCACHE_TEMPDIR` per run. `CCACHE_DIR`, `VCPKG_DEFAULT_BINARY_CACHE` (the directory must exist), `VCPKG_DOWNLOADS` and `CONAN_HOME` are shared (CPP-OPS-05); never set them yourself. Enable ccache with `-DCMAKE_CXX_COMPILER_LAUNCHER=ccache` in the preset.

Required tools: `cmake`, `ninja` (or the preset's generator), a compiler, `ctest`, `clang-format`, and `gcovr` (`pipx install gcovr`) for GCC or `llvm-profdata` and `llvm-cov` for Clang. A missing coverage tool fails the gate. A missing required tool fails; only `clang-tidy`, `cppcheck` and `osv-scanner` skip with an install hint.

## Race policy
C++ uses `race.mode = build` (WORKFLOW §6, CPP-CONC-04): ThreadSanitizer needs its own instrumented build and cannot share a binary with ASan. The full gate runs the required `test-tsan` check; a gate that skipped it is not green. To satisfy it:
- every test target runs under the `tsan` preset, so tests, libraries and dependencies are all instrumented (a `tsan` vcpkg triplet or Conan profile for dependencies, CPP-CONC-03);
- the `tsan` test preset sets `TSAN_OPTIONS="halt_on_error=1 second_deadlock_stack=1"`; suppressions go through the project's own file, never inline (D3);
- `test-asan-ubsan` is a separate required build (`-fsanitize=address,undefined -fno-sanitize-recover=all`); do not combine the two (CPP-CONC-06);
- never ship a flaky test as green: no `sleep` for sync, no retries; repeat with `ctest --repeat until-fail:N` while debugging (CPP-TEST-09).
If TSan cannot run on the host (32-bit, unsupported libc, a sandbox without ptrace/ASLR control) the gate fails with "race policy unavailable"; fix the host or run in the CI image, never skip. The race-guard hook does not deny `ctest` runs. Details in [concurrency.md](concurrency.md).

## Coverage recipe
Coverage is measured from the `tsan` build (TSan and coverage combine, verified on GCC 16.2 and Clang 22), so there is no third build. Write lcov to `{coverage}`:
- **GCC (default):** the `tsan` preset adds `--coverage -fprofile-update=atomic`; after the tests the pack runs `gcovr --root . --object-directory build/tsan --lcov {coverage} --filter src/`.
- **Clang (`clang` variant):** the `tsan` preset adds `-fprofile-instr-generate -fcoverage-mapping`, and its test preset sets `LLVM_PROFILE_FILE=${sourceDir}/build/tsan/profraw/%p-%m.profraw`; the pack merges the raw profiles with `llvm-profdata merge -sparse` and exports lcov with `llvm-cov export`, over the executables named `*_test` under `build/tsan`.
`build/`, `third_party/`, `external/`, `vcpkg_installed/`, the composition root (`src/app/**`, `main.cpp`) and generated protobuf code are not judged. If coverage looks short at `-O1`, build the `tsan` preset at `-O0`.
Prerequisites the architect or `cpp-engineer` adds in wave 0 (an override when the project lacks them, D13):
- `CMakePresets.json` with configure, build and test presets `dev`, `tsan`, `asan-ubsan` (distinct `binaryDir`s under `build/`), `CMAKE_EXPORT_COMPILE_COMMANDS=ON` in `dev`, and the sanitizer and coverage flags above;
- `.clang-format` and `.clang-tidy` (`WarningsAsErrors` set), a warnings module (`cmake/Warnings.cmake`) and `CMAKE_COMPILE_WARNING_AS_ERROR ON` in the dev preset;
- test executables named `*_test` and registered with `add_test` or `gtest_discover_tests` (so `ctest --no-tests=error` finds them);
- `vcpkg.json` with `builtin-baseline`, or `conanfile.txt` plus a committed `conan.lock`;
- `.gitignore` with `build/`, `CMakeUserPresets.json`, `vcpkg_installed/`, `*.gcda`, `*.gcno`, `*.profraw`.

## Layers on C++
```
src/
  domain/              CMake target <p>_domain (STATIC): std + value types; no I/O, clocks, threads
  application/         target <p>_application: ports (abstract classes or concepts) + use cases; links domain
  adapters/
    inbound_http/      target <p>_http: Beast/Drogon/gRPC handlers, DTOs, error mapping; links application
    outbound_pg/       target <p>_pg: libpqxx repositories implementing ports; links application
  app/main.cpp         composition root: config, wiring, signal_set, shutdown (links everything)
tests/{unit,integration}/   one executable per layer target, named *_test
```
Namespace per layer (`orders::domain`, `orders::app`). The target link graph is the first line of defence; `loops arch` scans `#include` lines (`forbidden_in_domain`, `impure_calls`) for the rest, and target cycles (legal in CMake for STATIC libraries) are a review item (CPP-ARCH-04). See [architecture.md](architecture.md).

## Top 10 footguns
1. Dangling `string_view`, `span` or reference to a temporary: own the data, return by value, run ASan.
2. Async callbacks capturing raw `this`: `shared_from_this()` or coroutines that own their state.
3. Mixed or uninstrumented sanitizer builds: one `tsan` preset, a separate `asan-ubsan` preset.
4. An unsynchronised `bool` flag across threads (UB): `std::atomic`, `std::stop_token`, `std::scoped_lock`.
5. An exception escaping a thread, `noexcept` function or destructor: catch at the task entry.
6. Raw `new`/`delete`: `std::make_unique`, rule of zero.
7. Unchecked `operator[]` and signed overflow: hardened libstdc++/libc++, `std::span`, `.at()` at boundaries, UBSan.
8. Uninitialised locals and members: brace-init, in-class initialisers.
9. `using namespace` in headers, non-self-contained headers: qualified names, include what you use.
10. Blocking an I/O thread (sync DB call, `future.get()`) or calls without deadlines: async drivers, deadlines on every call.

## Load on demand
- [repo.md](repo.md): layout, CMake presets, dependency manifests and locks, toolchain pin, CI shape, releases.
- [architecture.md](architecture.md): targets as layers, ports (virtual vs concepts), domain purity list, `loops arch` limits.
- [idioms.md](idioms.md): error strategy, RAII and ownership, UB, rule of zero, views, casts, naming, time.
- [testing.md](testing.md): GoogleTest and CTest, doubles, integration, coverage, fuzzing, flakiness.
- [concurrency.md](concurrency.md): the TSan build mode, sanitizer rules, locks, Asio, shutdown.
- [security.md](security.md): memory safety, hardening flags, injection, parsing, crypto, supply chain.
- [data-api.md](data-api.md): SQL layers, migrations, gRPC and HTTP, resilience, logging, metrics, tracing, config, images.
- [frameworks.md](frameworks.md): Asio/Beast, gRPC, userver, Drogon conventions and framework footguns.
