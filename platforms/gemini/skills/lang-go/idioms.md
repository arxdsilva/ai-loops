# Go idioms digest

Rules: `specs/languages/go/RULES.md §3-§27, §41-§44`.

## Errors (GO-IDIOM-01..03; RULES §8, §9)
- `error` is the last return; wrap with `fmt.Errorf("verb noun %q: %w", x, err)`; handle once (return or log); never `_` an error without a comment.
- Match with `errors.Is`, `errors.As`, or `errors.AsType[*MyErr](err)` (1.26). Combine with `errors.Join`.
- Expected business outcomes are result values, not errors. Sentinels (`io.EOF`-style) only for conditions callers branch on.
- Panic only for programmer invariants; recover only at an edge (HTTP middleware, goroutine entry). `log.Fatal` and `os.Exit` only in `main`.

## Context (GO-IDIOM-19; RULES §10)
First parameter named `ctx`; never in a struct, never nil; every outbound call has a deadline. `WithCancelCause`, `WithoutCancel`, `AfterFunc` where they fit; `context.Value` only for request-scoped data. Do not thread it through pure domain functions.

## Types and values (GO-IDIOM-04, 05, 07; RULES §7, §15, §18)
- Useful zero value; constructors only to establish invariants. `*T` for optionality at a boundary; `new(expr)` (1.26) sets a pointer field inline.
- A nil `*MyErr` stored in an `error` is non-nil: return `nil` explicitly.
- Small immutable values by value; never expose a mutable slice or map (`slices.Clone`, `maps.Clone`).
- Enums: typed `iota` constants with a meaningful zero (`Unknown`); close sets with an unexported interface method and a `default` that errors; the `exhaustive` linter checks switches.
- Distinct ID types where mixing is a real bug (`type OrderID string`).

## Generics and iterators (GO-IDIOM-06, 08, 17; RULES §17)
Generics for container and algorithm duplication, never frameworks. Type aliases (1.24), self-referential constraints (1.26) and generic methods (1.27, not on interfaces) exist; need is the test. Iterators: `iter.Seq` with `for range`, `slices.Collect`, `slices.Sorted(maps.Keys(m))`; `iter.Pull` needs `defer stop()`. Use `min`, `max`, `clear`, `slices.*`, `maps.*`, `cmp.Compare`, `sync.OnceValue`, `strings.Cut`/`CutLast` and delete hand-written equivalents (`go fix` suggests them).

## Resources (GO-IDIOM-09; RULES §22)
`defer` right after acquiring: `rows.Close()`, `resp.Body.Close()` (drain to reuse the connection), `cancel()`, `tx.Rollback()`. Check `Close` on writers. Hot tiny loop bodies: benchmark before dropping `defer`.

## Strings, time, maps, slices (GO-IDIOM-10, 11, 16; RULES §14, §20)
- `strings.Builder` not `+=` in loops. Time in UTC internally, `time.RFC3339`, compare with `Equal`, never `==` on `time.Time`.
- Never rely on map order; writing to a nil map panics; maps are not safe for concurrent writes.
- `append` may alias; use `s[:n:n]` or `slices.Clone`; re-slicing a huge array retains it; preallocate `make([]T, 0, n)`.

## Naming and API surface (GO-IDIOM-12, 13; RULES §3, §4, §26, §41)
MixedCaps, short consistent receivers, initialisms uppercase (`ID`, `URL`), no `Get` prefix, no stutter, `// Package x ...`. Minimal exports; every exported name has a doc comment starting with its name; `// Deprecated:` paragraphs for retirement.

## Avoid (GO-IDIOM-14, 15, 18)
`any` and `map[string]any` in domain/application APIs; reflection outside serialisation adapters; `unsafe`, cgo and `//go:linkname` without an ADR; `init()` except trivial driver registration in `cmd`; package-level mutable state; copying a struct with a `sync.Mutex` or `WaitGroup` (`copylocks`); unkeyed struct literals.

## Consistency (RULES §13, §20, §21, §34)
Inject `Clock`, `*rand.Rand` (`rand.New(rand.NewPCG(s1, s2))`) and ID sources; sort keys with a tie-break; `crypto/rand` and `rand.Text()` for tokens; `uuid` package on 1.27+. Procedure: `skills/consistency`.

## Performance (RULES §22, §31, §39, §53)
Measure first. `for b.Loop()` benchmarks with `b.ReportAllocs()`, `pprof`, `benchstat`. Hot-path mistakes: `fmt.Sprintf` in loops, `[]byte`<->`string` copies, boxing, `regexp.MustCompile` inside functions, `time.After` in a loop (use `time.NewTimer` and `Reset`). Escape analysis: `go build -gcflags='-m=2'`. Procedure: `skills/performance`.
