#!/usr/bin/env python3
"""Count loops-gap statuses per language x taxonomy section.

Usage: python3 -I matrix.py <research/languages dir> [--json out.json]

Rule rows: lines matching  | `<LANG>-<SEC>-NN` |  before "## Top 10 footguns".
Cells are split on unescaped '|'. Status = leading word of the "loops today"
cell (6th cell). Gaps rows: table rows between "## Gaps vs loops" and the
next "## " heading; rule IDs are expanded from forms like `X-SEC-02/03`,
`X-SEC-01..05`, `X-SEC-01`..`-05`, `X-SEC-06` / `-07`.
"""
import json
import re
import sys
from collections import Counter, defaultdict

LANGS = ["python", "typescript", "java", "cpp", "csharp", "php", "go", "ruby",
         "swift", "kotlin", "rust", "scala", "elixir", "erlang", "clojure"]
SECS = ["REPO", "TOOL", "ARCH", "IDIOM", "CONC", "TEST", "PERF", "SEC", "DATA",
        "API", "OBS", "OPS", "DOC", "FW"]
STATUSES = ["covered", "go-only", "missing", "n/a"]
ROW = re.compile(r"^\| `([A-Z]+)-([A-Z]+)-(\d+)` \|")


def cells(line):
    parts = re.split(r"(?<!\\)\|", line.strip())
    return [p.strip() for p in parts[1:-1]]


def status_of(cell):
    m = re.match(r"[`*]*\s*(covered|go-only|missing|n/a|partial)", cell, re.I)
    return m.group(1).lower() if m else "UNPARSED:" + cell[:30]


def expand_ids(text, prefix, sec_counts=None):
    """Expand compressed rule-ID references inside a Gaps 'Rule ID' cell.

    Handles `X-SEC-02/03`, `X-SEC-01..05`, `X-SEC-01`..`X-SEC-05`,
    `X-SEC-02`..`-08`, `X-SEC-06` / `-07`, `X-SEC-01..04,07..09`, bare
    `SEC-NN` after a prefixed ID, and section wildcards `X-SEC-*`.
    """
    ids = set()
    t = text.replace("`", "").replace("\u2013", "..").replace("\u2014", "..")
    head = re.compile(r"(?:%s-)?(%s)-(\*|\d+)" % (prefix, "|".join(SECS)))

    pos = 0
    while True:
        m = head.search(t, pos)
        if not m:
            break
        sec, n = m.group(1), m.group(2)
        pos = m.end()
        if n == "*":
            for k in range(1, (sec_counts or {}).get(sec, 0) + 1):
                ids.add((sec, k))
            continue
        prev = int(n)
        ids.add((sec, prev))
        tail = re.compile(r"\s*(/|,|\.\.)\s*(?:(?:%s-)?%s-|-)?(\d+)\b(?!-)" % (prefix, sec))
        while True:
            tm = tail.match(t, pos)
            if not tm:
                break
            sep, k = tm.group(1), int(tm.group(2))
            if sep == "..":
                for j in range(prev, k + 1):
                    ids.add((sec, j))
            else:
                ids.add((sec, k))
            prev = k
            pos = tm.end()
    return ids


def parse(path):
    rules = {}
    bad = []
    gaps_rows = []
    in_rules = True
    in_gaps = False
    prefix = None
    with open(path, encoding="utf-8") as fh:
        for line in fh:
            if line.startswith("## Top 10 footguns"):
                in_rules = False
            if line.startswith("## "):
                in_gaps = line.startswith("## Gaps vs loops")
                continue
            if in_rules:
                m = ROW.match(line)
                if m:
                    prefix = m.group(1)
                    c = cells(line)
                    if len(c) != 7:
                        bad.append((m.group(0), len(c)))
                        continue
                    sec = m.group(2)
                    rules[(sec, int(m.group(3)))] = {
                        "id": "%s-%s-%s" % m.groups(), "sec": sec,
                        "rule": c[1], "sev": c[3].strip("`"),
                        "status": status_of(c[5]), "enforce": c[6]}
            elif in_gaps and line.startswith("|") and not line.startswith("|---") \
                    and not line.startswith("| Rule ID"):
                c = cells(line)
                if len(c) >= 4:
                    gaps_rows.append(c)
    return prefix, rules, bad, gaps_rows


def main():
    root = sys.argv[1]
    out = {}
    grid_lines = []
    hdr = "| Language | " + " | ".join(SECS) + " | Rules | cov | go-only | missing | n/a | Readiness |"
    grid_lines.append(hdr)
    grid_lines.append("|" + "---|" * (len(SECS) + 7))
    tot_all = Counter()
    sanity = []
    for lang in LANGS:
        prefix, rules, bad, gaps_rows = parse("%s/%s.md" % (root, lang))
        per = defaultdict(Counter)
        tot = Counter()
        for (sec, _), r in rules.items():
            per[sec][r["status"]] += 1
            tot[r["status"]] += 1
        tot_all.update(tot)
        denom = tot["covered"] + tot["go-only"] + tot["missing"]
        ready = tot["covered"] / denom if denom else 0.0
        row = ["`%s`" % lang]
        for s in SECS:
            c = per[s]
            row.append("%d/%d/%d/%d" % (c["covered"], c["go-only"], c["missing"], c["n/a"]))
        row += [str(len(rules)), str(tot["covered"]), str(tot["go-only"]),
                str(tot["missing"]), str(tot["n/a"]), "%.0f%%" % (100 * ready)]
        grid_lines.append("| " + " | ".join(row) + " |")
        # gaps sanity
        secmax = Counter()
        for (s, n) in rules:
            secmax[s] = max(secmax[s], n)
        cited = set()
        prio = {}
        for c in gaps_rows:
            ids = expand_ids(c[0], prefix, secmax)
            p = re.search(r"P[0-2]", c[3])
            for i in ids:
                cited.add(i)
                if p:
                    prio.setdefault(i, p.group(0))
        noncov = {k for k, r in rules.items() if r["status"] in ("go-only", "missing")}
        uncited = sorted(noncov - cited)
        p_count = Counter(prio[k] for k in noncov if k in prio)
        sanity.append((lang, len(gaps_rows), len(noncov), len(noncov & cited),
                       uncited, p_count, bad,
                       sorted(k for k, r in rules.items() if r["status"].startswith(("UNPARSED", "partial")))))
        out[lang] = {
            "prefix": prefix,
            "rules": {r["id"]: dict(r, priority=prio.get(k)) for k, r in rules.items()},
            "totals": dict(tot), "readiness": ready,
        }
    print("Cells are covered/go-only/missing/n-a. Readiness = covered / (covered + go-only + missing).\n")
    print("\n".join(grid_lines))
    d = tot_all["covered"] + tot_all["go-only"] + tot_all["missing"]
    print("\nAll 15: %d rules; covered %d, go-only %d, missing %d, n/a %d; readiness %.0f%%" % (
        sum(tot_all.values()), tot_all["covered"], tot_all["go-only"], tot_all["missing"],
        tot_all["n/a"], 100 * tot_all["covered"] / d))
    print("\nGaps-table sanity (non-covered = go-only + missing):\n")
    print("| Language | Gaps rows | Non-covered rules | Cited in Gaps | Uncited | P0/P1/P2 of cited | Malformed rows | Odd status |")
    print("|---|---|---|---|---|---|---|---|")
    for lang, rows, nc, cit, unc, pc, bad, odd in sanity:
        pref = out[lang]["prefix"]
        print("| `%s` | %d | %d | %d | %d | %d/%d/%d | %s | %s |" % (
            lang, rows, nc, cit, len(unc), pc["P0"], pc["P1"], pc["P2"],
            ", ".join("%s(%d cells)" % b for b in bad) or "-",
            ", ".join("%s-%s-%02d" % (pref, s, n) for s, n in odd) or "-"))
    print()
    for lang, rows, nc, cit, unc, pc, bad, odd in sanity:
        if unc:
            pref = out[lang]["prefix"]
            print("- `%s` uncited: %s" % (lang, ", ".join("%s-%s-%02d" % (pref, s, n) for s, n in unc)))
    if len(sys.argv) > 3 and sys.argv[2] == "--json":
        with open(sys.argv[3], "w") as fh:
            json.dump(out, fh, indent=1)


if __name__ == "__main__":
    main()
