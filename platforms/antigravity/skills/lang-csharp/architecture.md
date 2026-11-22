# Architecture in .NET (CS-ARCH)

Layering, ports and SOLID are in `specs/ARCHITECTURE.md` and `specs/SOLID.md`; this file only maps them onto assemblies.

## Hexagon as projects (CS-ARCH-01/02/03)
- Domain: no `PackageReference`, no `ProjectReference`. Application -> Domain. Infrastructure -> Application. Host -> Application and Infrastructure. MSBuild rejects cycles, so a wrong reference cannot hide.
- Never reference `Microsoft.AspNetCore.*` or `Microsoft.EntityFrameworkCore` from Domain or Application.
- Types are `internal` by default; `public` only for the project's API. `InternalsVisibleTo` goes to the test project only. Namespace equals `<Assembly>.<Folder>`.
- `loops arch` resolves layers from `<ProjectReference>` items (resolver `project-reference`) and scans `using` lines. It forbids in domain: `System.IO`, `System.Net.Http`, `System.Data`, `Microsoft.EntityFrameworkCore`, `Microsoft.AspNetCore`, `Microsoft.Extensions.Logging`, `System.Diagnostics.Process`. Impure calls flagged: `DateTime.Now/UtcNow`, `DateTimeOffset.Now/UtcNow`, `Guid.NewGuid`, `Random.Shared`, `new Random(`, `Environment.GetEnvironmentVariable`, `Task.Run`, `Thread.Sleep` (CS-ARCH-07).

## Architecture tests (CS-ARCH-04)
Cycles inside one assembly and `using` leaks are not compile errors. Add `tests/<Name>.Architecture.Tests` with NetArchTest.Rules or ArchUnitNET (`Types.InAssembly(...).That().ResideInNamespace("Shop.Domain").ShouldNot().HaveDependencyOn("Shop.Infrastructure")`). It runs in `dotnet test`. Optional, but the native tool of record when the repo has it.

## Composition and DI (CS-ARCH-05/06/08)
- Compose only in the host's `Program.cs`, using `AddShopApplication(this IServiceCollection)` extension methods per project. Enable `ValidateScopes` and `ValidateOnBuild`.
- Constructor injection only: no `IServiceProvider.GetService` outside the root or factories, no static singletons.
- Ports are small consumer-owned interfaces in Application (`IOrderRepository`, `IPaymentGateway`); implementations are `internal sealed` in Infrastructure. No generic `IRepository<T>` over `DbContext`.

## Purity (CS-ARCH-07)
Domain code takes `TimeProvider` (`FakeTimeProvider` in tests), an id generator and `Random` as ports. No logging, no `[JsonProperty]` in the domain.

## Domain modelling (CS-ARCH-10/11/09)
Aggregates: private setters, factories, invariants in the constructor, domain events raised by the aggregate and dispatched after commit. Value objects as `readonly record struct` or `sealed record`. Modular monolith: separate projects and `DbContext` per module, a `Contracts` project for cross-module types, rule tested with NetArchTest. Size heuristics: classes under ~300 lines, methods ~40, at most 4-5 constructor parameters, `sealed` by default.
