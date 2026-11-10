# C++ security (digest)

Cites RULES.md.

## Memory safety is the main risk (CPP-SEC-01)
Roughly two thirds of severe vulnerabilities in large C/C++ codebases are memory-safety bugs. Use RAII, `std::span`, `std::string`; bounds-checked access at trust boundaries; library hardening in production builds (`-D_GLIBCXX_ASSERTIONS`, or libc++ `_LIBCPP_HARDENING_MODE_FAST`/`EXTENSIVE`). Ban `strcpy`, `sprintf`, `gets`, `strcat` and `memcpy` with computed sizes.

## Hardened release builds (CPP-SEC-02)
OpenSSF flags for production binaries: `-O2 -D_FORTIFY_SOURCE=3 -D_GLIBCXX_ASSERTIONS -fstack-protector-strong -fstack-clash-protection -fcf-protection=full` (x86_64) or `-mbranch-protection=standard` (AArch64), `-fPIE -pie -Wl,-z,relro -Wl,-z,now -Wl,-z,noexecstack -ftrivial-auto-var-init=zero -fno-delete-null-pointer-checks -fno-strict-overflow -fstrict-flex-arrays=3`. Verify with `checksec` or `hardening-check`. Not in sanitizer builds (no `_FORTIFY_SOURCE`). The gate has no hardening check: put the flags in a `cmake/Hardening.cmake` applied to the release preset, and review it.

## Injection and processes (CPP-SEC-03)
Parameterised queries (libpqxx `exec_params`, SOCI `use()`, prepared statements); never build SQL with `std::format` or `+`. Run processes with argument vectors (`posix_spawn`, `boost::process::v2`), never `system()` or `popen()` with user data.

## Untrusted input (CPP-SEC-04, -05)
- Schema-checked formats (Protobuf, FlatBuffers with the verifier, simdjson or nlohmann with size limits). No `memcpy` or `reinterpret_cast` of network structs. Limit input size and nesting depth. libxml2: `XML_PARSE_NONET`, never `XML_PARSE_NOENT`. Fuzz every parser (CPP-TEST-08).
- Canonicalise paths (`std::filesystem::weakly_canonical`, then a prefix check). Allow-list outbound hosts (SSRF). Set cookie flags and CORS explicitly. Verify TLS: never `SSL_VERIFY_NONE` or `CURLOPT_SSL_VERIFYPEER=0`.

## Crypto and secrets (CPP-SEC-06, -07)
Vetted libraries only (OpenSSL 3.x EVP, libsodium, BoringSSL, Botan). Argon2id (`crypto_pwhash`) or scrypt/bcrypt for passwords. CSPRNG from the library (`RAND_bytes`, `randombytes_buf`), never `rand()` or `std::mt19937`. Constant-time compare (`CRYPTO_memcmp`, `sodium_memcmp`). Wipe with `OPENSSL_cleanse`/`sodium_memzero`, not `memset`. Secrets come from env or a vault at runtime, never source, images or logs; scan with gitleaks or trufflehog.

## Supply chain and analysis (CPP-SEC-08, -09, CPP-TOOL-09)
Pin the vcpkg baseline or Conan lock, verify source hashes, review new ports, no `curl | sh`, no `FetchContent` of mutable branches, a pinned build container, provenance. `osv-scanner scan source --lockfile conan.lock` runs with `loops gate --vuln` (Conan only; coverage is thin). SAST: CodeQL `cpp`, Clang Static Analyzer, `clang-tidy` `bugprone-*`/`cert-*`, cppcheck (the optional `cppcheck` check, off by default), Semgrep; sanitizers and fuzzers are the dynamic side.
