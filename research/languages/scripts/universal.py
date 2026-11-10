#!/usr/bin/env python3
"""Find practices stated by >=12 of 15 languages. Each candidate = (key, tags, regex).
A language 'states' a practice when one of its rule rows carries one of the
taxonomy tags and the rule text matches the regex (case-insensitive)."""
import json, re, sys
from collections import Counter
d = json.load(open(sys.argv[1]))
C = [
 ("commit-lockfile", "REPO-04 SEC-06 REPO-01 REPO-03", r"(commit|committed).{0,60}(lock|\.sum|resolved)|lock.{0,40}commit"),
 ("frozen-install", "REPO-04 REPO-08 SEC-06 REPO-06", r"frozen|--locked|locked-mode|check-locked|npm ci|force-resolved|--write-locks|lockfile.{0,30}(fail|check|verify)|composer install|deps\.get|verify|-P\b"),
 ("pin-toolchain", "REPO-06 REPO-08 TOOL-01", r"pin|tool-versions|toolchain|wrapper|\.\w+-version|global\.json|engines"),
 ("dep-update-bot", "REPO-04 REPO-08 TOOL-05 SEC-06", r"renovate|dependabot|steward|antq"),
 ("semver-tags", "REPO-05 REPO-09", r"semver|semantic version"),
 ("gitignore-outputs", "REPO-07 OPS-06", r"gitignore|git-ignore|ignore "),
 ("ci-stage-order", "REPO-08", r"ci|stage"),
 ("conventional-commits", "REPO-09 REPO-05", r"conventional commit"),
 ("standard-layout", "REPO-01", r"layout|src/|lib/|sources/|structure|standard"),
 ("single-repo-default", "REPO-02 REPO-01", r"(prefer|default|keep|start).{0,40}(one|single|monolith|monorepo)|one (repo|solution|package|mix|application)|split.{0,40}(only|when)"),
 ("formatter-check", "TOOL-02 REPO-08", r"check|verify|dry-run|lint|--diff|ci"),
 ("lint-strict", "TOOL-03", r"warning|strict|error|-d warnings|max-warnings|level|fail"),
 ("typecheck-strict", "TOOL-04", r"strict|warning|error|nullable|dialyzer|mypy|tsc|phpstan|sorbet|-werror|type"),
 ("vuln-scan-ci", "TOOL-05 SEC-06 REPO-08", r"audit|osv|vuln|deny|dependency-check|nvd|advisor"),
 ("sbom", "TOOL-05 SEC-06 OPS-02 REPO-05", r"sbom|cyclonedx|spdx"),
 ("domain-pure", "ARCH-06", r"."),
 ("domain-no-clock-rng", "ARCH-06 IDIOM-06 TEST-09", r"clock|time|now|random|rand|uuid"),
 ("consumer-small-ports", "ARCH-07", r"."),
 ("ctor-di-root", "ARCH-05", r"constructor|composition root|main|wiring|manual|explicit"),
 ("arch-tool", "ARCH-04", r"."),
 ("size-limits", "ARCH-08", r"\d"),
 ("errors-not-swallowed", "IDIOM-01 IDIOM-09", r"swallow|empty catch|bare except|rescue|ignore|discard|catch.{0,40}(all|exception|throwable)|let it crash|wrap|context"),
 ("expected-outcomes-as-values", "IDIOM-01 IDIOM-04", r"result|either|expected|outcome|tagged tuple|\{:?ok|value"),
 ("immutability", "IDIOM-03", r"."),
 ("resources-scoped", "IDIOM-05", r"."),
 ("utc-time", "IDIOM-06", r"utc|time zone|timezone|instant|offset"),
 ("money-decimal", "IDIOM-06 IDIOM-04 DATA-01 DATA-02 IDIOM-03 IDIOM-09", r"money|decimal|currency|float"),
 ("metaprog-sparingly", "IDIOM-08", r"."),
 ("no-blocking-async", "CONC-05 CONC-01 CONC-02", r"block"),
 ("cancel-timeouts", "CONC-05 CONC-06 API-04 API-06", r"cancel|timeout|deadline"),
 ("bounded-concurrency", "CONC-05 CONC-06 CONC-02 API-05", r"bound|backpressure|limit|pool"),
 ("graceful-worker-shutdown", "CONC-06 OPS-04", r"shutdown|drain|graceful|sigterm|stop"),
 ("immutable-or-message-passing", "CONC-02", r"immutab|message|actor|owner|confine|channel|share nothing|process|atom"),
 ("race-policy", "CONC-04 CONC-03", r"."),
 ("test-globs", "TEST-02", r"."),
 ("parametrised", "TEST-03", r"param|table|data-driven|theory|each|property|cases"),
 ("fakes-over-mocks", "TEST-04", r"fake|in-memory|stub|mock"),
 ("testcontainers", "TEST-05", r"testcontainers|container|docker|real"),
 ("coverage-tool", "TEST-06", r"."),
 ("new-code-coverage", "TEST-07", r"."),
 ("property-fuzz", "TEST-08", r"property|fuzz|proptest|hypothesis|streamdata|proper|jqwik|fast-check|test\.check|quickcheck|libfuzzer"),
 ("random-order-isolation", "TEST-09 CONC-04", r"random|shuffle|order|seed|isolat|parallel"),
 ("controlled-clock-in-tests", "TEST-09 TEST-04 ARCH-06", r"clock|time|sleep|virtual"),
 ("parameterised-queries", "SEC-01", r"parameter|bind|prepared|placeholder|\?|query builder|orm"),
 ("safe-deserialization", "SEC-02", r"."),
 ("secrets-env-not-code-logs", "SEC-04 OPS-01", r"secret"),
 ("password-hash", "SEC-04", r"argon2|bcrypt|scrypt|pbkdf2|password"),
 ("csprng", "SEC-04", r"securerandom|crypto|csprng|strong_rand|secrets|random"),
 ("validate-at-boundary", "SEC-05 API-03", r"boundary|edge|adapter|validat"),
 ("sast", "SEC-07", r"."),
 ("versioned-migrations", "DATA-02", r"."),
 ("expand-contract", "DATA-02", r"expand|contract|zero-downtime|backward|two-step|additive|online"),
 ("tx-in-application", "DATA-03", r"application|use case|service|unit of work|boundary|transaction"),
 ("n-plus-one", "DATA-04", r"n\+1|n \+ 1|preload|eager|batch"),
 ("pool-timeouts", "DATA-05", r"pool"),
 ("problem-details", "API-03", r"9457|7807|problem"),
 ("timeouts-retries-idempotency", "API-04", r"timeout|retr|idempot|backoff|circuit"),
 ("outbox", "API-05", r"outbox"),
 ("client-reuse-deadline", "API-06", r"reuse|shared|single|pool|timeout|deadline|one client"),
 ("contract-first", "API-02", r"openapi|contract|grpc|protobuf|schema"),
 ("structured-logs", "OBS-01", r"structured|json|key|correlation|request.?id|trace"),
 ("no-pii-logs", "OBS-01 SEC-04", r"pii|personal|secret|sensitive|redact|filter"),
 ("otel", "OBS-03 OBS-02", r"opentelemetry|otel"),
 ("liveness-readiness", "OBS-04", r"liveness|readiness|live|ready"),
 ("config-env-validated", "OPS-01", r"."),
 ("container-multistage", "OPS-02", r"multi-stage|distroless|slim|non-root|chiselled|scratch|minimal"),
 ("graceful-shutdown-sigterm", "OPS-04 CONC-06", r"sigterm|graceful|drain|shutdown"),
 ("run-scoped-caches", "OPS-05", r"."),
 ("artifacts-list", "OPS-06", r"."),
 ("api-doc-comments", "DOC-01", r"."),
 ("readme-adr-changelog", "DOC-02", r"adr|readme|changelog"),
 ("review-focus", "DOC-03", r"."),
 ("framework-conventions", "FW-02 FW-03", r"."),
]
langs = list(d)
rows = []
for key, tags, rx in C:
    tg = tags.split(); r = re.compile(rx, re.I)
    hit = {}
    for lang in langs:
        for rid, ru in d[lang]["rules"].items():
            ts = set(x.strip() for t in re.findall(r"\[([A-Z]+-\d\d[^\]]*)\]", ru["rule"]) for x in t.split(","))
            if ts & set(tg) and r.search(re.sub(r"\[[^\]]*\]$", "", ru["rule"])):
                hit.setdefault(lang, []).append((rid, ru["status"]))
    st = Counter(s for v in hit.values() for _, s in v)
    rows.append((key, len(hit), hit, st))
for key, n, hit, st in sorted(rows, key=lambda x: -x[1]):
    miss = [l for l in langs if l not in hit]
    print("%-30s %2d  st=%s  missing=%s" % (key, n, dict(st), ",".join(miss)))
    if len(sys.argv) > 2 and key in sys.argv[2].split(","):
        for l in langs:
            if l in hit: print("     ", l, " ".join(i for i, _ in hit[l]))

if len(sys.argv) > 2 and sys.argv[2] == "--md":
    sev_rank = {"MUST": 0, "SHOULD": 1, "AVOID": 2}
    print("\n=== MD")
    for key, n, hit, st in rows:
        ids = []
        for l in langs:
            if l in hit:
                best = sorted(hit[l], key=lambda x: sev_rank.get(d[l]["rules"][x[0]]["sev"].split()[0], 3))[0][0]
                ids.append(best)
        tot = sum(st.values())
        maj = max(st, key=st.get)
        print("%s|%d|%s|cov %d/go %d/mis %d|%s" % (key, n, ", ".join(ids), st.get("covered",0), st.get("go-only",0), st.get("missing",0), maj))
