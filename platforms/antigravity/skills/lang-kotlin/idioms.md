# Kotlin idioms

Digest of RULES IDIOM.

## Errors (`KT-IDIOM-01/02`)
- Expected business failures: sealed result or error types (or Arrow `Either`/`Raise`). Exceptions for programmer errors and infrastructure only; Kotlin has no checked exceptions.
- Never `catch (e: Exception)` or `runCatching` around suspend code without rethrowing `CancellationException`.
- Wrap with a cause. `kotlin.Result` is local plumbing, not a public API type.

## Null-safety (`KT-IDIOM-03/04`)
- Absence is `T?`, not a sentinel. No `!!` outside tests (detekt `UnsafeCallOnNullableType`); use `?.`, `?:`, `requireNotNull(x) { msg }`. Avoid `lateinit` for domain fields.
- At every Java boundary declare the Kotlin type explicitly; public API never exposes `String!`. Build with `-Xjsr305=strict` (and `-Xjspecify-annotations=strict`).

## Values (`KT-IDIOM-05/06/08`)
- `val` over `var`; read-only collection types in APIs; `data class` for value holders; `@JvmInline value class` for ids and amounts.
- Sealed interfaces with exhaustive `when` as an expression and no `else`; `data object` for stateless cases.
- Classes with `equals` override `hashCode`; no mutable `data class` in hash sets. `java.time` or `kotlin.time`, never `Date`/`Calendar`; store instants in UTC.

## Resources (`KT-IDIOM-07`)
`.use { }` on `Closeable`; structured scopes for coroutines; never `GlobalScope.launch`.

## Style (`KT-IDIOM-09/10/11`)
- `UpperCamelCase` types, `lowerCamelCase` functions, `SCREAMING_SNAKE_CASE` constants, no underscores in package names; KDoc on public members without `@param`/`@return` boilerplate.
- Reflection and annotation processing only at framework seams; KSP over kapt; configure `kotlinx.serialization` or Jackson Kotlin module once.
- Avoid `object` singletons with mutable state, top-level `var`, companion service locators, scope functions nested more than two deep, and operator overloading beyond domain algebra.

## Review first (`KT-DOC-03`)
Platform types and `!!`, `GlobalScope`, swallowed cancellation, blocking on `Default`, `data class` JPA entities, exposed mutable collections, wrong annotation use-site targets, `runBlocking` outside `main` and tests, framework types in the domain.
