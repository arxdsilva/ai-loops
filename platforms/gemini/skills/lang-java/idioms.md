# Java: idioms

Digest of `specs/languages/java/RULES.md` IDIOM. Universal error and design rules: `specs/SOLID.md`, `specs/ARCHITECTURE.md`.

## Errors
- Unchecked exceptions for programming errors and unrecoverable domain failures; checked only for conditions the caller must handle. Wrap adapter `IOException`/`SQLException` in an unchecked adapter-level type at the boundary (`JAVA-IDIOM-01`).
- Never swallow (`JAVA-IDIOM-02`): no empty `catch`, no `catch (Exception | Throwable)` outside a boundary, no `printStackTrace()`. Wrap with the cause, log once at the boundary, restore the interrupt flag on `InterruptedException`.

## Null and Optional
- `Optional<T>` only as a return type for "maybe no result"; never a field, parameter or collection element; `orElseThrow`, `map`, `ifPresentOrElse` instead of `get()`; return `Optional.empty()` / `List.of()`, never `null` (`JAVA-IDIOM-03`).
- Null contract with JSpecify (`@NullMarked`, `@Nullable`) enforced by NullAway; `Objects.requireNonNull(x, "x")` in domain constructors (`JAVA-IDIOM-04`).

## Types
- Value objects and DTOs are `record`s with validation in the compact constructor and `List.copyOf` for collections; no Lombok `@Data` on domain types (`JAVA-IDIOM-05`).
- Closed sum types: `sealed interface ... permits ...` plus records, and `switch` pattern matching without `default` so the compiler proves exhaustiveness (`JAVA-IDIOM-06`).
- Override `equals` and `hashCode` together (records or `Objects.equals/hash`); `equals` for `String` and boxed types, `==` only for enums; no mutable fields in `hashCode` (`JAVA-IDIOM-08`). JPA entities: `data-api.md`.
- Composition over inheritance, `final` classes unless designed for extension, static factories and builders for many parameters, interface types in signatures, `var` only when the type is obvious (`JAVA-IDIOM-10`).

## Resources, time, money
- try-with-resources for every `AutoCloseable`: JDBC objects, streams, HTTP bodies, `Files.lines`/`Files.walk`; no `finalize()` (`JAVA-IDIOM-07`).
- `java.time` only; inject `Clock`; store UTC, ISO-8601 on the wire; no `Date`, `Calendar`, `SimpleDateFormat`. Money is `BigDecimal` from a `String`, compared with `compareTo`, with an explicit `RoundingMode` (`JAVA-IDIOM-09`).

## Style and avoids
- Google Java Style naming; Javadoc on every public API (`JAVA-IDIOM-12`).
- Avoid reflection, `Unsafe` and custom annotation processors in domain and application code; fine in frameworks and adapters. Annotations are metadata, not business logic (`JAVA-IDIOM-11`).
- Avoid the classic traps (`JAVA-IDIOM-13`): boxed `==` outside -128..127, `Arrays.asList(int[])`, `list.remove(int)` vs `remove(Object)`, `Collectors.toMap` without a merge function, mutation during iteration, `Stream.toList()` is unmodifiable, shared `SimpleDateFormat`/`DecimalFormat`.
