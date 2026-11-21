# Swift: frameworks digest

Rules in RULES.md FW. Project rules win: follow the framework layout the project already uses.

- Dominant: Vapor 4.122.x (5.0 is beta), Hummingbird 2.28.x (needs Swift 6.2+), raw SwiftNIO and swift-openapi for gateways. Prefer SSWG-graduated packages: SwiftNIO, swift-log, swift-metrics, swift-distributed-tracing, swift-service-lifecycle, AsyncHTTPClient, swift-crypto (SWIFT-FW-01).
- Hummingbird: `buildApplication(...)` returns an `ApplicationProtocol`; generic `Router` and `RequestContext`; test with `app.test(.router)`. Dependencies pass explicitly, which fits hexagonal well (SWIFT-FW-02, SWIFT-ARCH-08).
- Vapor: `Sources/App`, `configure.swift`, `routes.swift`, Fluent `Model`s, `Content` DTOs, the `Application` singleton (a locator). Put use cases in a separate target and keep controllers thin so Vapor stays an adapter (SWIFT-FW-02).
- Vapor 4 predates strict concurrency: enable Swift 6 mode on your own targets and map Fluent models to DTOs at the `Sendable` boundary. Hummingbird 2 is `Sendable`-first (SWIFT-FW-03).
- Footguns: blocking calls on a NIO event loop; skipping `app.shutdown()` / `asyncShutdown()` in tests (Vapor asserts); mutating `app.storage` after boot; reading unloaded Fluent fields; mixing `EventLoopFuture` and `async` APIs (SWIFT-FW-04, SWIFT-CONC-07).
- Layer mapping: the framework lives only in adapter targets and the App target; Domain and Application never import it (SWIFT-ARCH-07).
