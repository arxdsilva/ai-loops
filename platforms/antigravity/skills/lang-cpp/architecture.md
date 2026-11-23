# C++ architecture (digest)

Cites RULES.md. Rules: `specs/ARCHITECTURE.md`, `specs/SOLID.md`; this file only maps them to C++.

## Layers as CMake targets (CPP-ARCH-02)
```
src/domain/            add_library(<p>_domain STATIC ...)          links nothing but std
src/application/       add_library(<p>_application STATIC ...)     PUBLIC <p>_domain; owns ports
src/adapters/inbound_http/   <p>_http   PRIVATE <p>_application Boost::beast
src/adapters/outbound_pg/    <p>_pg     PRIVATE <p>_application libpqxx::pqxx
src/app/main.cpp       add_executable(<p>_service)                 PRIVATE all of the above (composition root)
tests/                 one executable per layer, linking that target + GTest::gtest_main
```
- Expose headers only through `target_include_directories`; internal headers live in `src/`, not `include/` (CPP-ARCH-04). A header from an unlinked target fails to compile: that is the cheapest dependency rule.
- One namespace per layer mirroring directories; no `using namespace` and no anonymous-namespace definitions in headers (CPP-ARCH-03).
- Include project headers as `#include "orders/domain/order.hpp"` (root-relative, quotes), system and third-party with `<...>`; use guards or `#pragma once` consistently (CPP-ARCH-05). The arch resolver depends on this.

## Ports
- Define ports in the consumer (application): a small abstract class with a virtual destructor for runtime swap and test doubles, or a C++20 `concept` when the implementation is fixed at compile time. `std::function` only for single-method callbacks (CPP-ARCH-09).
- Wire by hand in `main.cpp` with constructor injection (`std::unique_ptr<Port>`, references or templates). No DI container, no singletons or service locators (CPP-ARCH-07). `main.cpp` stays wiring only, `ARCHITECTURE §23`.
- Repositories return `std::expected` or `std::optional`; value objects are immutable structs with defaulted `operator==` (CPP-ARCH-10).

## Domain purity: what `loops arch` flags
`arch.forbidden_in_domain` (an `#include` of any of these in `domain`): `fstream`, `filesystem`, `thread`, `mutex`, `future`, `iostream`, `sys/socket.h`, `unistd.h`, `boost/asio`, `boost/beast`, `grpcpp`, `pqxx`, `drogon`, `curl/curl.h`.
`arch.impure_calls` (in `domain` and `application`): `std::chrono::system_clock::now`, `std::chrono::steady_clock::now`, `std::random_device`, `std::rand(`, `getenv(`, `std::cout`, `std::cerr`, `std::thread`, `std::async`. Take a `Clock&` and an `Rng&` port instead (CPP-ARCH-08, CPP-IDIOM-10). `std::chrono` types themselves are fine; only `now()` is impure.

## What the scanner cannot see
- Target-link cycles: CMake allows cycles among STATIC libraries. Review `cmake --graphviz=deps.dot` for forbidden edges and cycles (CPP-ARCH-04, -06); a CI step may parse it, but it is not a pack check.
- Templates and macros that hide includes; forward declarations that bypass layers; the `-Wshadow`-level hygiene (compiler flags).
- Bazel projects: use `visibility` and `deps` with `--check_visibility` instead.

## Size limits (SOLID, CPP-ARCH-11)
Units: class = type, `struct` with fields = stateful type, abstract class = port. Keep ports small (split by interface segregation), files under ~400 lines, functions under ~50 lines, at most 4 to 5 parameters (CG F.2, F.3, I.23). Rule of zero; non-virtual interface idiom for Liskov.
