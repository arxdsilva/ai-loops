# C++ testing (digest)

Cites RULES.md. Tests recognised by the pack: `**/*_test.cpp`, `**/*_test.cc`, `**/test_*.cpp`, `tests/**`, `test/**` (CPP-TEST-02). Testing architecture: `ARCHITECTURE §24`.

## Tooling
- GoogleTest + GoogleMock by default (`GTest::gtest_main`, `gtest_discover_tests()` so each `TEST` is a CTest test); Catch2 v3 or doctest are fine; one per repo (CPP-TEST-01).
- Layout: `tests/unit/<layer>/*_test.cpp`, `tests/integration/`, `tests/fuzz/`, `tests/bench/`; one executable per layer target, named `*_test` (the Clang coverage export finds them by that name).
- Style: `TEST(Suite, DoesXWhenY)`, `TEST_F` only for shared setup, `INSTANTIATE_TEST_SUITE_P` as the table-driven form, Arrange-Act-Assert, `EXPECT_*` over `ASSERT_*` unless the test cannot continue (CPP-TEST-03).

## Doubles and integration
- Hand-written fakes of ports first; GoogleMock only for interaction checks at the application/port boundary; do not mock value types or the standard library; avoid `NiceMock` hiding unexpected calls (CPP-TEST-04).
- Integration tests use real dependencies: containers (testcontainers-native is community-maintained, verify it) or docker-compose started by a CTest fixture (`FIXTURES_REQUIRED`), a throw-away schema per test, ports from the OS (bind to 0) (CPP-TEST-05).

## Running
- Always `ctest --output-on-failure --no-tests=error`; never drop `--no-tests=error` (an empty selection passes without it, CPP-TEST-10). Iterate with `ctest --preset dev -R <regex>` or `cmake --build --preset dev --target <t>_test && ctest ... -R`.
- Targeted runs are still sanitized runs for the final check: the gate's `test-tsan` and `test-asan-ubsan` run the whole suite. For a race you are chasing: `ctest --preset tsan --repeat until-fail:20 -R <regex>`.
- Randomise order (`--gtest_shuffle --gtest_random_seed`), inject clock and RNG, set `TIMEOUT` per test, never `sleep` for synchronisation (CPP-TEST-09).

## Coverage (CPP-TEST-06, -07)
Measured from the `tsan` build, never a separate one: GCC with `gcovr ... --lcov {coverage}`, Clang with `llvm-profdata merge` + `llvm-cov export -format=lcov`. The gate requires at least 70% of new lines (WORKFLOW §6). `src/app/**`, `main.cpp`, generated code, `build/` and third-party code are excluded. Header-only code counts through the translation units that include it, so cover it from a test that instantiates it. The gcovr lcov has absolute `SF:` paths and a checksum as third `DA:` field; loops normalises both.

## Fuzz, property, mutation (CPP-TEST-08, -11)
Fuzz parsers and codecs with libFuzzer (`LLVMFuzzerTestOneInput`, `-fsanitize=fuzzer,address,undefined`, `-max_total_time=60` in CI), keep a seed corpus in the repo; RapidCheck or Catch2 generators for properties. Mutation testing (Mull) is optional, domain layer only, nightly.
