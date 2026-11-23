# C++ repository and toolchain (digest)

Cites `specs/languages/cpp/RULES.md`. Pack: `specs/languages/cpp/pack.json`.

## Layout
- Pitchfork-style tree: `src/<layer>/` (one CMake target per directory), `tests/{unit,integration,fuzz,bench}/`, `cmake/` (modules only), `proto/`, `docs/adr/`, `external/` only for vendored code (CPP-REPO-01).
- Modern CMake only: `add_library`, `target_link_libraries(... PRIVATE|PUBLIC|INTERFACE)`, `target_include_directories`, `target_compile_features(cxx_std_20)`. No global `include_directories`, `link_libraries` or `CMAKE_CXX_FLAGS` in subdirectories (CPP-REPO-02).
- One repo, many targets (modular monolith). Polyrepo only for independently versioned libraries (CPP-REPO-06). CMake is the default; Bazel (bzlmod) for hermetic monorepos, Meson for small projects; never two sources of truth (CPP-REPO-05).

## Presets
- Commit `CMakePresets.json` (configure, build, test, workflow presets); git-ignore `CMakeUserPresets.json` (CPP-REPO-03). Use the lowest schema `version` your CMake floor supports.
- The pack assumes presets named `dev`, `tsan`, `asan-ubsan`, each with a distinct `binaryDir` under `build/` (CPP-REPO-04). Out-of-source builds only (`CMAKE_DISABLE_IN_SOURCE_BUILD`).
- `tsan` carries coverage flags too (the merged build, D3). Example cache variables for GCC: `CMAKE_CXX_FLAGS="-fsanitize=thread -fno-omit-frame-pointer -g -O1 --coverage -fprofile-update=atomic"`, `CMAKE_EXE_LINKER_FLAGS="-fsanitize=thread --coverage"`. Its test preset `environment`: `TSAN_OPTIONS=halt_on_error=1 second_deadlock_stack=1`.
- `asan-ubsan`: `-fsanitize=address,undefined -fno-sanitize-recover=all -fno-omit-frame-pointer`; no `_FORTIFY_SOURCE` in sanitizer builds (CPP-CONC-06, CPP-SEC-02).
- `dev`: `CMAKE_EXPORT_COMPILE_COMMANDS=ON`, `CMAKE_COMPILE_WARNING_AS_ERROR=ON`, `-DCMAKE_CXX_COMPILER_LAUNCHER=ccache`.

## Standards and warnings
- C++20 floor (`CMAKE_CXX_STANDARD_REQUIRED ON`, `CMAKE_CXX_EXTENSIONS OFF`); C++23 where every CI compiler supports it; no C++26 in service code. Modules are opt-in (CPP-TOOL-01, -02).
- Strict warnings on every target (`cmake/Warnings.cmake`): `-Wall -Wextra -Wpedantic -Wshadow -Wconversion -Wsign-conversion -Wnon-virtual-dtor -Wold-style-cast -Woverloaded-virtual -Wnull-dereference -Wdouble-promotion -Wformat=2 -Wimplicit-fallthrough` (MSVC `/W4 /permissive-`). `-Werror` in dev and CI presets, not in downstream source builds (CPP-TOOL-04).
- `clang-format` is the only formatter; `clang-tidy` from `.clang-tidy` with `WarningsAsErrors` (CPP-TOOL-03, -05). IWYU and cppcheck are advisory (CPP-TOOL-06, -07).

## Dependencies
- Manifest, never copied sources or `FetchContent` of a moving branch: vcpkg `vcpkg.json` with `builtin-baseline` (+ `overrides`), or Conan 2 with a committed `conan.lock` (CPP-REPO-08, -09). Regenerate the baseline or lock deliberately in its own change (`vcpkg x-update-baseline`, `conan lock create .`), and pass `--lockfile=conan.lock` in CI.
- vcpkg manifest mode installs during `cmake --preset dev` (the configure check). Conan projects run `conan install` with the lock before configure; add that as the wave-0 step in CI, not as an ad hoc gate edit.
- Update bots: Dependabot has a `vcpkg` ecosystem (baseline updates, no security updates); Renovate has `conan` and `bazel-module` managers, no vcpkg (CPP-REPO-10).
- OSV-Scanner reads `conan.lock` only, with thin advisory coverage; for vcpkg check the baseline against upstream advisories (CPP-TOOL-09).

## Toolchain pin and hygiene
- Pin compiler and CMake versions in the CI image tag and a toolchain preset or `CMAKE_TOOLCHAIN_FILE`; pin the vcpkg triplet and checkout (CPP-REPO-12).
- `.gitignore`: `build/`, `cmake-build-*/`, `CMakeUserPresets.json`, `vcpkg_installed/`, `compile_commands.json`, `*.gcda`, `*.gcno`, `*.profraw`, `.cache/`. `.gitattributes` `* text=auto`. Pre-commit runs `clang-format` (CPP-REPO-13).
- Cleanup deletes artifact directories only when new since the baseline; the `core` / `core.[0-9]*` globs never match `core.cpp` (CPP-OPS-06).

## CI order
Restore caches (ccache, vcpkg binary cache) → format check → configure → build (warnings as errors) → clang-tidy → TSan+coverage run → ASan+UBSan run → OSV scan → package; matrix over GCC and Clang (CPP-REPO-14). Versioning: SemVer in `project(<name> VERSION x.y.z)`, tag `vX.Y.Z`, the OCI image tag follows (CPP-REPO-11). Conventional commits (CPP-REPO-15).
