#!/usr/bin/env python3
"""Structure lint for research/languages/<lang>.md (TAXONOMY.md rules)."""
import json, re, sys, subprocess, tempfile, os

SECTIONS = ["REPO","TOOL","ARCH","IDIOM","CONC","TEST","PERF","SEC","DATA","API","OBS","OPS","DOC","FW"]
MIN = {"REPO":8,"ARCH":8,"IDIOM":8,"TEST":8,"TOOL":5,"CONC":5,"SEC":5,"DATA":5,"API":5,"OPS":5,"PERF":4,"OBS":4,"DOC":4,"FW":4}
STATUS = {"covered","go-only","missing","n/a"}
SEV = {"MUST","SHOULD","AVOID"}
ENF = {"gate","hook","arch","review"}
REQ_KEYS = ["id","detect","source_globs","test_globs","checks","race","coverage","cache_env","artifacts","arch"]

def lint(path):
    text = open(path, encoding="utf-8").read()
    errs, warn = [], []
    # sections
    for s in SECTIONS:
        if not re.search(rf"^## \d+\. {s}\b", text, re.M):
            errs.append(f"missing section {s}")
    for h in ["Top 10 footguns","15. LOOPS","Gaps vs loops","Open questions","Verification"]:
        if not re.search(rf"^## .*{re.escape(h)}", text, re.M):
            errs.append(f"missing heading '{h}'")
    # rule rows
    counts = {s:0 for s in SECTIONS}
    bad = 0
    body = re.split(r"^## (?:Top 10 footguns|Gaps vs loops)", text, flags=re.M)[0]
    for line in body.splitlines():
        m = re.match(r"^\|\s*`?([A-Z]+)-([A-Z]+)-(\d+)`?\s*\|", line)
        if not m or m.group(2) not in counts:
            continue
        cells = [c.strip() for c in re.split(r"(?<!\\)\|", line.strip().strip("|"))]
        if len(cells) < 7:
            bad += 1; continue
        counts[m.group(2)] += 1
        sev, src, st, enf = cells[3].strip("`* "), cells[4], cells[5].strip("`* ").lower(), cells[6].strip("`* ").lower()
        problems = []
        if not any(sev.upper().startswith(x) for x in SEV): problems.append(f"sev={sev!r}")
        if "http" not in src: problems.append("no url")
        if not any(st.startswith(x) for x in STATUS): problems.append(f"status={st!r}")
        if not any(e in enf for e in ENF): problems.append(f"enforce={enf!r}")
        if problems:
            bad += 1
            if bad <= 8: warn.append(f"{cells[0]}: " + ", ".join(problems))
    for s,n in counts.items():
        if n < MIN[s]: errs.append(f"{s}: {n} rules < {MIN[s]}")
    if bad: errs.append(f"{bad} malformed rule rows")
    # LOOPS json
    sec = re.split(r"^## 15\. LOOPS.*$", text, flags=re.M)
    block = None
    if len(sec) > 1:
        m = re.search(r"```json\s*\n(.*?)\n```", sec[1], re.S)
        if m: block = m.group(1)
    if block is None:
        errs.append("no LOOPS json block")
    else:
        try:
            d = json.loads(block)
            for k in REQ_KEYS:
                if k not in d: errs.append(f"LOOPS missing key {k}")
            r = d.get("race") or {}
            if r.get("mode") not in ("flag","build","none"): errs.append(f"race.mode={r.get('mode')!r}")
            if (d.get("coverage") or {}).get("format") is None: errs.append("coverage.format null")
        except Exception as e:
            errs.append(f"LOOPS json invalid: {e}")
    if "As of" not in text: errs.append("no 'As of'")
    lines = text.count("\n")
    return lines, counts, errs, warn

ok = True
for p in sys.argv[1:]:
    lines, counts, errs, warn = lint(p)
    total = sum(counts.values())
    print(f"{os.path.basename(p)}: {lines} lines, {total} rules " + " ".join(f"{k}{v}" for k,v in counts.items()))
    for e in errs: print("  ERR", e)
    for w in warn: print("  warn", w)
    if errs: ok = False
sys.exit(0 if ok else 1)
