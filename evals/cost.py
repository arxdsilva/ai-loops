#!/usr/bin/env python3
"""Dollar cost of loops runs from Claude Code transcripts (read-only).

usage: python3 -I evals/cost.py <repo> [--since YYYY-MM-DD]

The repo's Claude Code transcripts are read from
~/.claude/projects/<repo path with "/" and "." as "-"> and its runs from
<repo>/.loops/runs/*/events.jsonl (runs started on or after --since).

For each run (an events.jsonl), it prices every assistant message of the lead
session inside the run's time window and of every subagent the run labelled
with a role, at that message's model rates: uncached input, 5-minute and
1-hour cache writes, cache reads and output, each at its own rate. Prints the
cost per role and per run (with the triage tier). Prices are USD per million
tokens; check them against the published pricing page before quoting results.
"""
import collections
import datetime as dt
import glob
import json
import os
import sys

# (input, output, cache read, cache write 5m, cache write 1h) per MTok.
PRICES = {
    "claude-opus-5-5": (4.0, 20.0, 0.20, 5.0, 8.0),
    "claude-sonnet-5-5": (2.0, 10.0, 0.10, 2.5, 4.0),
    "claude-haiku-5-5": (0.10, 0.50, 0.01, 0.125, 0.20),
    "claude-fable-5-2": (8.0, 40.0, 0.20, 10.0, 16.0),
}


def ts(s):
    return dt.datetime.fromisoformat(s.replace("Z", "+00:00"))


def msg_cost(model, u):
    p = PRICES.get(model)
    if not p:
        return 0.0
    cc = u.get("cache_creation") or {}
    w1h = cc.get("ephemeral_1h_input_tokens", 0)
    w5m = cc.get("ephemeral_5m_input_tokens", u.get("cache_creation_input_tokens", 0) - w1h)
    return (u.get("input_tokens", 0) * p[0] + u.get("output_tokens", 0) * p[1]
            + u.get("cache_read_input_tokens", 0) * p[2] + w5m * p[3] + w1h * p[4]) / 1e6


def transcript(path):
    """Yields (time, model, usage) once per assistant message id."""
    seen = set()
    for line in open(path):
        try:
            m = json.loads(line)
        except ValueError:
            continue
        msg = m.get("message") or {}
        if m.get("type") != "assistant" or msg.get("id") in seen or not msg.get("usage"):
            continue
        seen.add(msg.get("id"))
        yield ts(m["timestamp"]), msg.get("model", ""), msg["usage"]


def load_run(path):
    evs = []
    for line in open(path):
        try:
            evs.append(json.loads(line))
        except ValueError:
            pass
    roles, tier = {}, "none"
    for e in evs:
        if e.get("agent") and e.get("role"):
            roles[e["agent"]] = e["role"]
        if e.get("kind") == "triage":
            tier = e.get("name") or tier
    outcome = next((e.get("status") for e in reversed(evs) if e.get("kind") == "end"), "open")
    return ts(evs[0]["t"]), ts(evs[-1]["t"]), roles, tier, outcome


def args():
    a = sys.argv[1:]
    since = ""
    if "--since" in a:
        i = a.index("--since")
        since = a[i + 1]
        del a[i:i + 2]
    if len(a) != 1:
        sys.exit("usage: python3 -I evals/cost.py <repo> [--since YYYY-MM-DD]")
    repo = os.path.abspath(a[0])
    proj = os.path.expanduser("~/.claude/projects/" + repo.replace("/", "-").replace(".", "-"))
    runs = sorted(glob.glob(os.path.join(repo, ".loops", "runs", "*", "events.jsonl")))
    runs = [r for r in runs if os.path.basename(os.path.dirname(r))[:8] >= since.replace("-", "")]
    return proj, runs


def main():
    proj, runs = args()
    subs = {os.path.basename(p)[6:-6]: p for p in glob.glob(os.path.join(proj, "*", "subagents", "agent-*.jsonl"))}
    leads = glob.glob(os.path.join(proj, "*.jsonl"))
    lead_msgs = [x for p in leads for x in transcript(p)]
    by_role = collections.defaultdict(list)
    rows = []
    for r in runs:
        start, end, roles, tier, outcome = load_run(r)
        lead = sum(msg_cost(mo, u) for t, mo, u in lead_msgs if start <= t <= end)
        agents = 0.0
        for aid, role in roles.items():
            if aid in subs:
                c = sum(msg_cost(mo, u) for _, mo, u in transcript(subs[aid]))
                by_role[role].append(c)
                agents += c
        rows.append((lead + agents, lead, tier, outcome, len(roles), os.path.basename(os.path.dirname(r))[:48]))
    print("per role: total $, agents, avg $")
    for role, cs in sorted(by_role.items(), key=lambda kv: -sum(kv[1])):
        print(f"  {sum(cs):9.2f} {len(cs):4d} {sum(cs) / len(cs):8.2f}  {role}")
    print("per run: total $, lead $, tier, outcome, agents, run")
    for row in sorted(rows, reverse=True):
        print(f"  {row[0]:8.2f} {row[1]:8.2f}  {row[2]:9s} {row[3]:9s} {row[4]:3d}  {row[5]}")
    by_tier = collections.defaultdict(list)
    for row in rows:
        by_tier[row[2]].append(row[0])
    print("per tier: runs, median $, mean $")
    for tier, cs in sorted(by_tier.items()):
        cs.sort()
        print(f"  {tier:9s} {len(cs):3d} {cs[len(cs) // 2]:8.2f} {sum(cs) / len(cs):8.2f}")
    tot = sum(r[0] for r in rows)
    print(f"total ${tot:.2f} over {len(rows)} runs; lead share {sum(r[1] for r in rows) / max(tot, 1e-9):.0%}")


if __name__ == "__main__":
    main()
