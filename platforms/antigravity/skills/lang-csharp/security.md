# Security (CS-SEC)

Digest of RULES.md SEC. The optional `vuln` check (`loops gate --vuln`) scans NuGet dependencies; SAST comes from the analyzers in the build.

- **Injection (CS-SEC-01):** parameterised queries only. EF Core: LINQ, `FromSql($"... {x}")` and `ExecuteSql` (interpolation is parameterised); never `FromSqlRaw`/`ExecuteSqlRaw` with concatenation (CA2100). Dapper with parameters. `Process.Start` with `ArgumentList`, never user-built strings.
- **Deserialisation (CS-SEC-02):** never `BinaryFormatter` (removed in .NET 9), `NetDataContractSerializer`, `SoapFormatter`, `LosFormatter`, or Newtonsoft `TypeNameHandling` other than `None` on untrusted data. Use `System.Text.Json` with typed models. XML: `DtdProcessing.Prohibit`, `XmlResolver = null`.
- **Web (CS-SEC-03):** SSRF allow-lists for outbound URLs; canonicalise paths (`Path.GetFullPath`, then a prefix check; never `Path.Combine` with user input); `LocalRedirect`; antiforgery on cookie-authenticated endpoints; CORS with named policies and exact origins.
- **Secrets (CS-SEC-04):** `dotnet user-secrets` in development, environment variables or Key Vault with managed identity in production; nothing committed in `appsettings.Production.json`.
- **Crypto (CS-SEC-05):** `AesGcm`, `RandomNumberGenerator` (never `System.Random` for tokens), `Rfc2898DeriveBytes.Pbkdf2` or Identity's `PasswordHasher`, Data Protection for cookies. No MD5/SHA1 for security, DES/3DES, ECB, custom crypto.
- **Input (CS-SEC-06):** validate at the adapter (`[ApiController]` auto-400, DataAnnotations, FluentValidation endpoint filter), map to domain types that enforce invariants, check ownership instead of trusting client ids.
- **Supply chain (CS-SEC-07, CS-REPO-08, CS-TOOL-06):** `--locked-mode`, source mapping, signed packages, no floating versions in applications, review new packages, treat package `build/` targets as code. NuGetAudit with NU1901-NU1904 as errors.
- **SAST (CS-SEC-08):** CA security rules via `AnalysisLevel`, `AnalysisMode`, SonarAnalyzer or Security Code Scan, CodeQL, Semgrep `p/csharp`.
- **AuthN/Z (CS-SEC-09):** `UseAuthentication` then `UseAuthorization`, a `FallbackPolicy` requiring an authenticated user, `[AllowAnonymous]` only where deliberate, HTTPS and HSTS in production.
