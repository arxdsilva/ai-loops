# C++ concurrency and the TSan build mode (digest)

Cites RULES.md. The rule: `WORKFLOW §6`; the pack: `race.mode = build`, `race.checks = ["test-tsan"]`.

## The policy (CPP-CONC-03, -04, -06)
- Required check `test-tsan`: configure, build and run the whole CTest suite from the `tsan` preset (`-fsanitize=thread -fno-omit-frame-pointer -g -O1` plus coverage flags, own `binaryDir`), `TSAN_OPTIONS="halt_on_error=1 second_deadlock_stack=1"`.
- Required check `test-asan-ubsan`: separate build (`-fsanitize=address,undefined -fno-sanitize-recover=all`). TSan cannot be combined with ASan, LSan or MSan; ASan and UBSan combine; skip MSan for services.
- TSan with coverage works on both compilers: GCC `--coverage -fprofile-update=atomic`, Clang `-fprofile-instr-generate -fcoverage-mapping`.
- Instrument everything: tests, libraries, and dependencies built by vcpkg or Conan (a `tsan` triplet or profile). Expect 5x to 15x slowdown and 5x to 10x memory. Linux x86_64/aarch64 and macOS only, no 32-bit. On macOS the Apple libc++ interception differs: do not assume Linux behaviour.
- Suppressions (`TSAN_OPTIONS=suppressions=...`) only for third-party false positives and only through the project's committed file, with an override logged. Annotate custom fibers and coroutines.
- Host cannot run TSan: the gate fails, it does not skip (D24).

## Models and sharing (CPP-CONC-01, -02)
- Models: OS threads (`std::thread`, `std::jthread`), pools (Asio `thread_pool`, userver task processors), C++20 coroutines (Asio, Boost.Cobalt, userver), event loops with few I/O threads. No GIL: every shared mutable object has a documented owner.
- Avoid sharing; guard with RAII locks (`std::scoped_lock`, `std::unique_lock`, never bare `lock()`/`unlock()`), tiny critical sections, never call unknown code under a lock. `std::atomic` for single variables with the default `seq_cst`; relaxed orderings are expert-only. Prefer queues and strands to shared state.
- Static help: `clang-tidy concurrency-*`, Clang Thread Safety Analysis (`-Wthread-safety`, `GUARDED_BY`, `REQUIRES` over a capability-annotated mutex) (CPP-CONC-09).

## Async (CPP-CONC-07)
Never block an I/O thread or coroutine (`sleep`, sync DB, `future.get()`). A deadline on every operation. Cancellation through `std::stop_token` or Asio cancellation slots. Keep in-flight callback state alive (`shared_from_this` or coroutines owning state). Bound queues. See `ARCHITECTURE §14`.

## Background work and shutdown (CPP-CONC-08)
`std::jthread` (joins on destruction, has a stop token), never `std::thread` + `detach`. Pools have an explicit shutdown: stop accepting, drain with a deadline, join. Signals set an atomic or use `asio::signal_set`; no logic in raw handlers (`CPP-OPS-04`, `ARCHITECTURE §22`).

## Sanitizer builds in CMake (CPP-CONC-05)
A `cmake/Sanitizers.cmake` module reads `SANITIZE=thread|address,undefined|none` and applies `target_compile_options` and `target_link_options` through an INTERFACE target that every target links. Test-time options go in the test preset `environment`.
