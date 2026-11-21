# C++ frameworks (digest)

Cites RULES.md `CPP-FW-01..04`. Project rules win: use the framework the project already has and its conventions inside the inbound adapter; keep the application layer framework-free. Usage evidence is vendor and community self-reporting, so treat the ranking as qualitative.

| Framework | Fits | Conventions | Footguns |
|---|---|---|---|
| Boost.Asio / Beast | low-level control, the most widely deployed foundation | executors, strands, completion tokens; handlers run on the I/O executor | handlers capturing raw `this`; `io_context.run()` from many threads without strands; blocking in a handler |
| gRPC C++ | RPC | generated `Service` classes, CMake `protobuf_generate`, `ServerBuilder::RegisterService`; callback API | unbounded streams without flow control; calls without deadlines |
| userver | coroutine microservices | component list plus YAML static config, own coroutine runtime, `testsuite` (pytest) for functional tests | `std::mutex` or `std::this_thread::sleep_for` instead of `engine::` primitives |
| Drogon | async web, built-in ORM and websockets | controllers registered by macros (`METHOD_LIST_BEGIN`), config JSON, `drogon_ctl` models, `app().registerController` | blocking DB calls on the I/O loop |
| Crow / Oat++ | minor, small services | embedded HTTP routers | same Asio-style lifetime rules |

## Hexagonal mapping (CPP-FW-04)
- Inbound adapter: controller or handler classes that call an application use case (`PlaceOrderUseCase::execute`) and translate DTO to domain; map errors to problem details or gRPC status.
- Outbound adapters: implement application ports with framework clients.
- `main.cpp` builds the object graph and starts the server (userver component list and static config; Drogon `app().registerController`; gRPC `ServerBuilder::RegisterService`).

## Shared footguns (CPP-FW-03)
Asio handlers capturing raw `this` (use `shared_from_this` or coroutines); blocking on I/O threads; unbounded gRPC streams; mixing exceptions across framework callback boundaries (catch at the boundary, convert to an error). Run the TSan and ASan builds against the framework's own test server, not only unit tests.
