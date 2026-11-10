# Idioms (CS-IDIOM)

## Errors and null
- **Exceptions (CS-IDIOM-01):** exceptions for exceptional failures, `Result<T>`/`TryXxx` for expected ones. Throw the most specific type; derive from `Exception`. Rethrow with `throw;`. No blanket `catch (Exception)` without rethrow or log; no exceptions for control flow.
- **Arguments (CS-IDIOM-02):** `ArgumentNullException.ThrowIfNull(x)` and the other throw helpers at public entry. Never return `null` for a collection.
- **Nullable (CS-IDIOM-03, CS-TOOL-05):** `<Nullable>enable</Nullable>`, honest `string?`, `required` members, `[NotNullWhen]` on `Try` methods. `!` only with a proven invariant in a comment. `= null!` only for EF-filled navigation properties.

## Types
- **Records (CS-IDIOM-04):** `record`/`readonly record struct` for values and DTOs, `init` and `required` for DTO fields, `IReadOnlyList<T>`/`ImmutableArray<T>` on APIs. Not for EF entities (reference identity).
- **Pattern matching (CS-IDIOM-05):** switch expressions kept exhaustive (CS8509 as error); closed hierarchies as `sealed` records until C# union types ship (preview in C# 15).
- **Equality and strings (CS-IDIOM-07):** override `Equals` and `GetHashCode` together (`HashCode.Combine`), `IEquatable<T>` on structs; explicit `StringComparison` (`Ordinal`, `OrdinalIgnoreCase`); `CultureInfo.InvariantCulture` for machine data; `StringBuilder` in loops.
- **Disposal (CS-IDIOM-06):** `using var`, `await using`; `IAsyncDisposable` for async resources; `SafeHandle` instead of finalizers.
- **Time (CS-IDIOM-08):** `DateTimeOffset` UTC, `DateOnly`/`TimeOnly`; "now" only from `TimeProvider.GetUtcNow()`; IANA zone ids.
- **LINQ (CS-IDIOM-11):** no double enumeration (CA1851), `Any()` over `Count()`, `FrozenDictionary` for read-mostly lookups.
- **Reflection (CS-IDIOM-10):** prefer source generators (`System.Text.Json`, `LoggerMessage`, `GeneratedRegex`); no `dynamic` or `Activator.CreateInstance` in business code.
- **Naming (CS-IDIOM-09):** PascalCase types and members, `_camelCase` private fields, `I` prefix, `Async` suffix; file-scoped namespaces; XML docs on public API.
