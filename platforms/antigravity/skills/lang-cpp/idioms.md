# C++ idioms (digest)

Cites RULES.md.

## Errors (CPP-IDIOM-01, -02)
- State one strategy per layer and write it down (ADR). Default: `std::expected<T,E>` for expected business failures in domain and application code; exceptions for rare, exceptional faults. The project's own policy wins (Google style bans exceptions; the Core Guidelines prefer them): log an override when you follow it.
- Map both to a typed error at the adapter edge (`ARCHITECTURE §13`). No error codes with out-parameters; no `catch (...) {}` without rethrow or log.
- With exceptions: throw by value, catch by `const&`, derive from `std::exception`, `noexcept` on destructors and moves, and catch at every thread, task and callback boundary (an escape calls `std::terminate`).

## Ownership and RAII (CPP-IDIOM-05, -06)
- No `new`, `delete`, `malloc` in application code: `std::make_unique` first, `std::make_shared` only for truly shared lifetime, never a `shared_ptr` cycle (`weak_ptr`).
- Raw `T*` and `T&` mean non-owning; transfer ownership with `unique_ptr<T>` by value.
- Rule of zero; rule of five when a class owns a raw resource; `explicit` single-argument constructors; virtual (or protected non-virtual) base destructors; polymorphic classes non-copyable.

## Undefined behaviour (CPP-IDIOM-03, -11, -14)
- No unchecked `operator[]` at trust boundaries (`.at()`, `std::span`, hardened library), no signed overflow, no uninitialised reads, no dangling references or views, no strict-aliasing violations. UBSan and ASan run in `test-asan-ubsan`.
- Initialise everything: `T x{}`, in-class member initialisers, `= delete` for unwanted operations; `-ftrivial-auto-var-init=zero` is a backstop, not a substitute.
- No C-style casts; `static_cast` or `std::bit_cast`; `reinterpret_cast` and `const_cast` only with a comment; `std::byte` for raw memory.

## Values and parameters (CPP-IDIOM-04, -07, -08, -09)
- `std::optional` for "maybe a value", references for "must exist", `gsl::not_null` or a reference for non-null pointers. `const` and `constexpr` by default, `enum class`, defaulted `operator<=>`/`==`, designated initialisers, no mutable globals.
- Parameters: small trivially copyable by value, others by `const&`, sinks by value then `std::move`, outputs by return (a struct for several). `std::span` and `std::string_view` for ranges and text, never stored beyond the owner's lifetime.
- `std::vector` by default, `std::array` for fixed size, `std::string` not `char*`, ranges and algorithms over raw loops, `std::format` over `printf`.

## Style and metaprogramming (CPP-IDIOM-12, -13)
- One published naming style (Google or Core Guidelines/`std`-style); no Hungarian prefixes; no `_Capital` or `__` identifiers.
- `constexpr` and concepts over SFINAE and macros; macros only for include guards, `__FILE__` logging, platform detection. No RTTI or `dynamic_cast` in domain or hot code (variants and visitors).

## Time (CPP-IDIOM-10)
`std::chrono` types only; UTC internally; inject a `Clock` port; never a bare `int` or `double` duration. See `ARCHITECTURE §20`.
