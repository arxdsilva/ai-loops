#!/usr/bin/env python3
"""Generate the loops workflow graph (static SVG).

Usage: make_workflow_graph.py <out_dir>
Writes loops-workflow-graph.svg: the middle level between loops-phases.svg
(one card per phase) and loops-run-graph.svg (every command, hook and state).
Phase containers hold their agents; gates are hexagons on each phase's exit;
inputs, MCP servers, CLI tools and data stores have their own shapes.
"""
import math
import sys
from html import escape

SANS = "-apple-system, BlinkMacSystemFont, 'Segoe UI', Inter, Helvetica, Arial, sans-serif"
MONO = "ui-monospace, SFMono-Regular, Menlo, Consolas, 'Liberation Mono', monospace"
INK, MUTED, BG = "#0f172a", "#475569", "#ffffff"
W = 1500
PH = {"intake": "#334155", "research": "#2563eb", "plan": "#7c3aed", "implement": "#0891b2",
      "qa": "#16a34a", "product": "#ea580c", "reflect": "#db2777", "backlog": "#0f766e"}
TOOL, MCP, STORE, HOOK, RED = "#0d9488", "#0369a1", "#64748b", "#b45309", "#dc2626"
EDGE = {  # colour, width, dash, legend
    "flow": ("#334155", 1.8, "", "flow"),
    "loop": (RED, 1.8, "7 5", "sent back (max rounds)"),
    "link": (TOOL, 1.5, "8 3 2 3", "uses a tool / service"),
    "data": ("#94a3b8", 1.5, "2 3", "reads / writes"),
}


def tint(c, a):
    r, g, b = (int(c[i:i + 2], 16) for i in (1, 3, 5))
    m = lambda v: round(255 - (255 - v) * a)
    return f"#{m(r):02x}{m(g):02x}{m(b):02x}"


def txt(x, y, s, size, fill=INK, weight="400", anchor="middle", family=SANS, fit=None):
    if fit:
        k = 0.6 if family == MONO else 0.53
        size = min(size, fit / (k * max(len(s), 1)))
    return (f'<text x="{x:.1f}" y="{y:.1f}" font-family="{family}" font-size="{size:.1f}" font-weight="{weight}" '
            f'fill="{fill}" text-anchor="{anchor}">{escape(s)}</text>')


class Node:
    def __init__(self, nid, kind, x, y, w, h, title, sub=None, color=None):
        self.id, self.kind, self.x, self.y, self.w, self.h = nid, kind, x, y, w, h
        self.title, self.sub, self.color = title, sub, color

    def anchor(self, side, f=0.5):
        return {"l": ((self.x, self.y + self.h * f), (-1, 0)), "r": ((self.x + self.w, self.y + self.h * f), (1, 0)),
                "t": ((self.x + self.w * f, self.y), (0, -1)), "b": ((self.x + self.w * f, self.y + self.h), (0, 1))}[side]


N, ORDER, EDGES, GROUPS, LABELS = {}, [], [], [], []


def node(nid, kind, x, y, w, h, title, sub=None, color=None):
    n = Node(nid, kind, x, y, w, h, title, sub, color)
    N[nid] = n
    ORDER.append(n)
    return n


def edge(a, b, kind="flow", sa=("r", 0.5), sb=("l", 0.5), via=None, label=None, lpos=None, bend=None):
    EDGES.append(dict(a=a, b=b, kind=kind, sa=sa, sb=sb, via=via, label=label, lpos=lpos, bend=bend))


def group(phase, x, y, w, h):
    GROUPS.append((phase, x, y, w, h))


# ------------------------------------------------------------------ layout
R1, R2, GH = 96, 500, 336          # row tops and group height

# input
group("input", 16, R1, 196, GH)
node("gh", "tool", 30, R1 + 40, 168, 40, "GitHub issue", "gh")
node("gl", "tool", 30, R1 + 88, 168, 40, "GitLab issue", "glab")
node("jira", "mcp", 30, R1 + 136, 168, 40, "Jira", "connector")
node("lin", "mcp", 30, R1 + 184, 168, 40, "Linear", "connector")
node("text", "human", 30, R1 + 232, 168, 32, "plain text")
node("backlog", "agent", 30, R1 + 276, 168, 48, "/loops:backlog", "a lane per issue", PH["backlog"])

# intake
group("intake", 238, R1, 222, GH)
node("lead", "agent", 252, R1 + 40, 194, 44, "lead", "master agent: your session", PH["intake"])
node("fetch", "loops", 252, R1 + 100, 194, 30, "loops ticket fetch")
node("triage", "loops", 252, R1 + 140, 194, 30, "loops run triage")
node("clarify", "loops", 252, R1 + 180, 194, 30, "loops:clarify")
node("charter", "art", 266, R1 + 224, 166, 30, "charter.md")
node("g1", "gate", 264, R1 + 274, 170, 44, "scope written")
for s in ("gh", "gl", "jira", "lin", "text"):
    edge(s, "fetch", "flow" if s == "text" else "link", ("r", 0.5), ("l", 0.5))
edge("backlog", "fetch", "flow", ("r", 0.5), ("l", 0.7))
edge("lead", "fetch", "flow", ("b", 0.5), ("t", 0.5))
edge("fetch", "triage", "flow", ("b", 0.5), ("t", 0.5))
edge("triage", "clarify", "flow", ("b", 0.5), ("t", 0.5))
edge("clarify", "charter", "data", ("b", 0.5), ("t", 0.5))
edge("charter", "g1", "flow", ("b", 0.5), ("t", 0.5))

# research
group("research", 486, R1, 310, GH)
RY = [R1 + 40, R1 + 98, R1 + 156, R1 + 214]
for nid, name, (k, chip) in zip(("web", "prac", "code", "hw"),
                                ("web-researcher", "practices-researcher", "code-researcher", "hardware-researcher"),
                                (("tool", "WebSearch · WebFetch"), ("art", "AGENTS.md · specs"),
                                 ("loops", "loops lang detect"), ("loops", "loops hw"))):
    node(nid, "agent", 514, RY.pop(0), 252, 50, name, None, PH["research"])
    N[nid].chip = (k, chip)
node("fork", "bar", 498, R1 + 36, 6, 222, "")
node("join", "bar", 776, R1 + 36, 6, 222, "")
node("briefs", "art", 514, R1 + 286, 116, 30, "research/*.md")
node("g2", "gate", 642, R1 + 280, 140, 42, "4 briefs")
for r in ("web", "prac", "code", "hw"):
    f = (N[r].y + 25 - (R1 + 36)) / 222
    edge("fork", r, "flow", ("r", f), ("l", 0.5))
    edge(r, "join", "flow", ("r", 0.5), ("l", f))
edge("join", "g2", "flow", ("b", 0.5), ("t", 0.8))
edge("join", "briefs", "data", ("b", 0.2), ("r", 0.5), via=[("y", R1 + 268)])
edge("g1", "fork", "flow", ("r", 0.5), ("l", 0.5), via=[("x", 474)])

# plan
group("plan", 822, R1, 290, GH)
node("planner", "agent", 840, R1 + 44, 160, 50, "planner", None, PH["plan"])
node("reviewer", "agent", 840, R1 + 168, 160, 50, "plan-reviewer", None, PH["plan"])
node("planmd", "art", 1012, R1 + 52, 88, 30, "plan.md")
node("pcheck", "loops", 1012, R1 + 112, 88, 46, "loops plan", "check")
node("askplan", "human", 1012, R1 + 178, 88, 32, "--ask plan")
node("g3", "gate", 858, R1 + 278, 150, 42, "APPROVE")
edge("planner", "reviewer", "flow", ("b", 0.3), ("t", 0.3))
edge("reviewer", "planner", "loop", ("t", 0.72), ("b", 0.72), label="CHANGES · max 3",
     lpos=(920 + 20, R1 + 131))
edge("planner", "planmd", "data", ("r", 0.5), ("l", 0.5))
edge("planner", "pcheck", "flow", ("r", 0.85), ("l", 0.5))
edge("reviewer", "askplan", "flow", ("r", 0.5), ("l", 0.5), label=None)
edge("reviewer", "g3", "flow", ("b", 0.5), ("t", 0.5))
edge("g2", "planner", "flow", ("r", 0.5), ("l", 0.5), via=[("x", 808)])

# stores (right column, row 1)
group("stores", 1296, R1, 188, GH)
node("aimem", "store", 1318, R1 + 60, 144, 86, "ai-memory", "MCP server")
node("filemem", "store", 1318, R1 + 196, 144, 76, ".loops/memory/", "opt-in, in git")

# implement
group("implement", 16, R2, 300, GH)
node("arch", "agent", 30, R2 + 40, 136, 52, "architect", "contracts, wave 0", PH["implement"])
node("langeng", "agent", 176, R2 + 40, 126, 52, "<id>-engineer", "toolchain", PH["implement"])
node("eng", "stack", 30, R2 + 124, 272, 84, "layer engineers ×n", "domain · application · api ·|persistence · integration · test · docs",
     PH["implement"])
node("claims", "art", 44, R2 + 228, 150, 30, "claims reports")
node("g4", "gate", 62, R2 + 276, 210, 42, "every lane reported")
edge("arch", "eng", "flow", ("b", 0.5), ("t", 0.25))
edge("langeng", "eng", "flow", ("b", 0.5), ("t", 0.79))
edge("eng", "claims", "data", ("b", 0.33), ("t", 0.4))
edge("eng", "g4", "flow", ("b", 0.75), ("t", 0.83))
edge("g3", "arch", "flow", ("r", 0.5), ("t", 0.9), via=[("x", 1150), ("y", R2 - 30), ("x", 152)])

# qa
group("qa", 342, R2, 384, GH)
node("qav", "stack", 358, R2 + 40, 176, 56, "qa-verifier ×lane", "re-runs every claim", PH["qa"])
node("qamd", "art", 556, R2 + 52, 150, 30, "VERIFIED · FAILED")
node("qaq", "agent", 358, R2 + 110, 176, 44, "qa-quality", None, PH["qa"])
node("crap", "tool", 552, R2 + 112, 160, 40, "crapper · dryer", "mutator")
node("qas", "agent", 358, R2 + 166, 176, 44, "security-reviewer", None, PH["qa"])
node("scan", "tool", 552, R2 + 168, 160, 40, "govulncheck · gosec", "each pack's scanners")
node("gate", "checks", 358, R2 + 222, 354, 50, "loops gate")
node("g5", "gate", 418, R2 + 284, 236, 42, "VERIFIED + gate green")
edge("g4", "qav", "flow", ("r", 0.5), ("l", 0.5), via=[("x", 330)])
edge("qav", "qamd", "data")
edge("qav", "qaq", "flow", ("b", 0.5), ("t", 0.5))
edge("qaq", "crap", "link")
edge("qaq", "qas", "flow", ("b", 0.5), ("t", 0.5))
edge("qas", "scan", "link")
edge("qas", "gate", "flow", ("b", 0.5), ("t", 0.25))
edge("gate", "g5", "flow", ("b", 0.5), ("t", 0.5))
edge("qav", "eng", "loop", ("l", 0.25), ("r", 0.3), via=[("x", 326)])
LABELS.append((326, R2 + 30, "FAILED · max 3", RED, "middle"))

# product
group("product", 752, R2, 300, GH)
node("prv", "agent", 768, R2 + 40, 170, 48, "product-reviewer", None, PH["product"])
node("svc", "tool", 950, R2 + 40, 88, 48, "your app", "curl · CLI")
node("proofs", "art", 782, R2 + 104, 142, 30, "proofs/AC-n.md")
node("report", "loops", 940, R2 + 100, 98, 38, "loops report")
node("g6", "gate", 768, R2 + 152, 176, 42, "every AC proven")
node("askpr", "human", 952, R2 + 158, 88, 32, "--ask pr")
node("git", "tool", 768, R2 + 214, 130, 44, "git push", "loops/<ticket>")
node("ghpr", "tool", 910, R2 + 214, 128, 44, "gh pr create", "glab mr create")
edge("g5", "prv", "flow", ("r", 0.5), ("l", 0.5), via=[("x", 740)])
edge("prv", "svc", "link")
edge("prv", "proofs", "data", ("b", 0.4), ("t", 0.4))
edge("prv", "report", "flow", ("r", 0.9), ("t", 0.5), via=[("x", 989)])
edge("proofs", "g6", "flow", ("b", 0.5), ("t", 0.5))
edge("g6", "askpr", "flow")
edge("g6", "git", "flow", ("b", 0.35), ("t", 0.5))
edge("git", "ghpr", "link")
edge("g6", "eng", "loop", ("l", 0.5), ("b", 0.5), via=[("x", 760), ("y", R2 + GH + 22)])
LABELS.append((520, R2 + GH + 22, "an AC fails · max 2", RED, "middle"))

# output
group("output", 1078, R2, 198, GH)
node("pr", "tool", 1096, R2 + 92, 162, 58, "PR / MR", "proofs · decisions")
node("merge", "human", 1096, R2 + 190, 162, 34, "you review, merge")
edge("ghpr", "pr", "link", ("r", 0.5), ("l", 0.5), via=[("x", 1070)])
edge("pr", "merge", "flow", ("b", 0.5), ("t", 0.5))

# reflect
group("reflect", 1296, R2, 188, GH)
node("refl", "agent", 1310, R2 + 40, 160, 48, "reflector", None, PH["reflect"])
node("reflmd", "art", 1324, R2 + 106, 132, 30, "reflection.md")
node("ghc", "tool", 1310, R2 + 152, 160, 44, "gh pr comment", "rule overrides")
node("end", "loops", 1310, R2 + 214, 160, 30, "loops run end")
node("g7", "gate", 1314, R2 + 276, 152, 42, "gate green")
edge("pr", "refl", "flow", ("r", 0.3), ("l", 0.5), via=[("x", 1286)])
edge("refl", "reflmd", "data", ("b", 0.5), ("t", 0.5))
edge("reflmd", "ghc", "link", ("b", 0.5), ("t", 0.5))
edge("ghc", "end", "flow", ("b", 0.5), ("t", 0.5))
edge("end", "g7", "flow", ("b", 0.5), ("t", 0.5))

# memory: read at intake, written at reflect
edge("aimem", "lead", "data", ("t", 0.5), ("t", 0.5), via=[("y", 66), ("x", 349)], label="memory_briefing", lpos=(820, 66))
edge("refl", "filemem", "data", ("t", 0.78), ("b", 0.78))
edge("refl", "aimem", "data", ("t", 0.94), ("r", 0.5), via=[("y", R2 - 20), ("x", 1476)])
LABELS.append((1424, R1 + GH + 22, "lessons", "#64748b", "middle"))

# always on
AY = R2 + GH + 58
node("hooks", "hook", 16, AY, 560, 52, "hooks on every tool call",
     "read-guard · test-guard · toolchain-env · model-guard · post-edit · stop-gate")
node("ev2", "store", 610, AY - 4, 150, 60, "events.jsonl")
node("watch", "loops", 794, AY + 8, 330, 36, "status line · loops watch · loops report")
node("stop", "gate", 1158, AY + 5, 326, 42, "can't stop while code is untested")
edge("hooks", "ev2", "data")
edge("ev2", "watch", "flow")
edge("watch", "stop", "flow")
LY = AY + 92
H = LY + 70


# ------------------------------------------------------------------ rendering
def lines(n, x, y, title, sub, title_fill, size=15, sub_size=12.5, family=SANS, weight="700"):
    subs = sub.split("|") if sub else []
    total = 1 + len(subs)
    cy = y - (total - 1) * 8
    out = [txt(x, cy + 5, title, size, title_fill, weight, family=family, fit=n.w - 16)]
    for j, s in enumerate(subs):
        out.append(txt(x, cy + 21 + j * 15, s, sub_size, MUTED, "500", fit=n.w - 14))
    return "".join(out)


def draw(n):
    x, y, w, h, k = n.x, n.y, n.w, n.h, n.kind
    cxm, cym = x + w / 2, y + h / 2
    if k in ("agent", "stack"):
        c = n.color
        out = []
        if k == "stack":
            for d in (8, 4):
                out.append(f'<rect x="{x + d}" y="{y - d}" width="{w}" height="{h}" rx="8" fill="{tint(c, 0.18)}" stroke="{c}" stroke-width="1.2"/>')
        out.append(f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="8" fill="{tint(c, 0.1)}" stroke="{c}" stroke-width="1.6"/>'
                   f'<rect x="{x}" y="{y + 4}" width="5" height="{h - 8}" rx="2" fill="{c}"/>')
        chip = getattr(n, "chip", None)
        if chip:
            out.append(txt(cxm, y + 20, n.title, 15, INK, "700", fit=w - 16))
            out.append(chipshape(chip[0], x + 22, y + 27, w - 44, 18, chip[1]))
        else:
            out.append(lines(n, cxm, cym, n.title, n.sub, INK))
        return "".join(out)
    if k == "loops":
        return (f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="3" fill="#f8fafc" stroke="#0f172a" stroke-width="1.4"/>'
                + lines(n, cxm, cym, n.title, n.sub, INK, 13.5, 12.5, MONO, "600"))
    if k == "art":
        f = 8
        return (f'<path d="M{x} {y} H{x + w - f} L{x + w} {y + f} V{y + h} H{x} Z" fill="#f1f5f9" stroke="#94a3b8" stroke-width="1.3"/>'
                f'<path d="M{x + w - f} {y} V{y + f} H{x + w}" fill="none" stroke="#94a3b8" stroke-width="1.3"/>'
                + txt(cxm, cym + 4.5, n.title, 13, "#334155", "600", family=MONO, fit=w - 14))
    if k == "tool":
        return (f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="{min(h / 2, 14)}" fill="#ffffff" stroke="{TOOL}" '
                f'stroke-width="1.6" stroke-dasharray="6 3"/>' + lines(n, cxm, cym, n.title, n.sub, "#115e59", 14.5))
    if k == "mcp":
        return (f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="10" fill="{tint(MCP, 0.06)}" stroke="{MCP}" '
                f'stroke-width="1.6" stroke-dasharray="6 3"/>' + badge(x + w - 40, y - 8)
                + lines(n, cxm - 10, cym, n.title, n.sub, "#075985", 14.5))
    if k == "human":
        r = h / 2
        px, py = x + 16, cym
        return (f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="{r}" fill="#ffffff" stroke="#111827" stroke-width="1.6"/>'
                f'<circle cx="{px}" cy="{py - 4}" r="3.6" fill="#111827"/><path d="M{px - 6} {py + 8} a6 6 0 0 1 12 0 z" fill="#111827"/>'
                + txt(cxm + 8, cym + 5, n.title, 14, INK, "700", fit=w - 34))
    if k == "gate":
        i = 14
        pts = f"{x + i},{y} {x + w - i},{y} {x + w},{cym} {x + w - i},{y + h} {x + i},{y + h} {x},{cym}"
        return (f'<polygon points="{pts}" fill="#ffffff" stroke="#0f172a" stroke-width="2"/>' + shield(x + i + 12, cym, "#0f172a")
                + txt(cxm + 12, cym + 5, n.title, 14, INK, "800", fit=w - 64))
    if k == "store":
        ry = 9
        return (f'<path d="M{x} {y + ry} V{y + h - ry} A{w / 2} {ry} 0 0 0 {x + w} {y + h - ry} V{y + ry}" fill="#f1f5f9" stroke="{STORE}" stroke-width="1.5"/>'
                f'<ellipse cx="{cxm}" cy="{y + ry}" rx="{w / 2}" ry="{ry}" fill="#e2e8f0" stroke="{STORE}" stroke-width="1.5"/>'
                + (badge(x + w - 30, y + 2 * ry - 4) if n.sub == "MCP server" else "")
                + lines(n, cxm, cym + 6, n.title, n.sub, INK, 14.5))
    if k == "hook":
        return (f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="4" fill="#fffbeb" stroke="{HOOK}" stroke-width="1.6"/>'
                f'<rect x="{x}" y="{y}" width="{w}" height="4" fill="{HOOK}"/>' + lines(n, cxm, cym, n.title, n.sub, "#92400e", 14.5))
    if k == "checks":
        out = [f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="3" fill="#f8fafc" stroke="#0f172a" stroke-width="1.8"/>',
               txt(x + 12, y + 30, n.title, 15, INK, "700", "start", MONO)]
        for row, chips in enumerate((("build", "lint", "arch", "deps"), ("test · race", "coverage ≥ 70%"))):
            cx_ = x + 112
            cy_ = y + 5 + row * 21
            for c in chips:
                cw = len(c) * 6.6 + 16
                hl = "#16a34a" if c[0] in "tc" else "#475569"
                out.append(f'<rect x="{cx_:.1f}" y="{cy_}" width="{cw:.1f}" height="18" rx="9" fill="#fff" stroke="{hl}" stroke-width="1.2"/>'
                           + txt(cx_ + cw / 2, cy_ + 13, c, 11.5, INK, "600"))
                cx_ += cw + 5
        return "".join(out)
    if k == "bar":
        return f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="2" fill="#0f172a"/>'
    return ""


def badge(x, y):
    return (f'<rect x="{x}" y="{y}" width="34" height="16" rx="4" fill="{MCP}"/>'
            + txt(x + 17, y + 12, "MCP", 10.5, "#ffffff", "800", family=MONO))


def chipshape(kind, x, y, w, h, label):
    if kind == "tool":
        box = f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="9" fill="#fff" stroke="{TOOL}" stroke-dasharray="5 3" stroke-width="1.2"/>'
        fill = "#115e59"
    elif kind == "art":
        box = f'<path d="M{x} {y} H{x + w - 6} L{x + w} {y + 6} V{y + h} H{x} Z" fill="#f1f5f9" stroke="#94a3b8" stroke-width="1.1"/>'
        fill = "#334155"
    else:
        box = f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="2" fill="#f8fafc" stroke="#0f172a" stroke-width="1.1"/>'
        fill = INK
    return box + txt(x + w / 2, y + 13.5, label, 12, fill, "600", family=MONO if kind == "loops" else SANS, fit=w - 10)


def shield(x, y, color):
    s = f'stroke="{color}" stroke-width="1.8" fill="none" stroke-linecap="round" stroke-linejoin="round"'
    return (f'<path d="M{x} {y - 8} L{x + 7} {y - 5.5} V{y} C{x + 7} {y + 4.5} {x + 3.5} {y + 7} {x} {y + 8.5}'
            f' C{x - 3.5} {y + 7} {x - 7} {y + 4.5} {x - 7} {y} V{y - 5.5} Z" {s}/>'
            f'<path d="M{x - 3} {y} L{x - 0.5} {y + 2.5} L{x + 3.5} {y - 2}" {s}/>')


def draw_group(phase, x, y, w, h):
    c = PH.get(phase, "#94a3b8")
    neutral = phase in ("input", "stores", "output")
    fill = "#f8fafc" if neutral else tint(c, 0.05)
    stroke = "#cbd5e1" if neutral else c
    tw = len(phase) * 10.5 + 22
    head = (f'<rect x="{x + 12}" y="{y - 13}" width="{tw}" height="26" rx="13" fill="{"#475569" if neutral else c}"/>'
            + txt(x + 12 + tw / 2, y + 5, phase, 15, "#ffffff", "800", family=MONO))
    return (f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="14" fill="{fill}" stroke="{stroke}" stroke-width="1.4"'
            f'{"" if not neutral else " stroke-dasharray=\"5 4\""}/>' + head)


def bez(p0, n0, p1, n1, bend=None):
    d = max(20, min(110, math.dist(p0, p1) * 0.45)) if bend is None else bend
    return (p0[0] + n0[0] * d, p0[1] + n0[1] * d), (p1[0] + n1[0] * d, p1[1] + n1[1] * d)


def rounded(pts, r=10):
    d = f"M{pts[0][0]:.1f} {pts[0][1]:.1f}"
    for i in range(1, len(pts) - 1):
        a, b, c = pts[i - 1], pts[i], pts[i + 1]
        l1, l2 = math.dist(a, b) or 1, math.dist(b, c) or 1
        rr = min(r, l1 / 2, l2 / 2)
        p = (b[0] - (b[0] - a[0]) / l1 * rr, b[1] - (b[1] - a[1]) / l1 * rr)
        q = (b[0] + (c[0] - b[0]) / l2 * rr, b[1] + (c[1] - b[1]) / l2 * rr)
        d += f" L{p[0]:.1f} {p[1]:.1f} Q{b[0]:.1f} {b[1]:.1f} {q[0]:.1f} {q[1]:.1f}"
    return d + f" L{pts[-1][0]:.1f} {pts[-1][1]:.1f}"


def ortho(p0, p1, spec):
    pts, cur = [p0], p0
    for axis, v in spec:
        cur = (v, cur[1]) if axis == "x" else (cur[0], v)
        pts.append(cur)
    pts.append((cur[0], p1[1]) if spec[-1][0] == "x" else (p1[0], cur[1]))
    pts.append(p1)
    return pts


def edge_svg(e):
    a, b = N[e["a"]], N[e["b"]]
    color, width, dash, _ = EDGE[e["kind"]]
    p0, n0 = a.anchor(*e["sa"])
    p1, n1 = b.anchor(*e["sb"])
    if e["via"]:
        d = rounded(ortho(p0, p1, e["via"]))
    else:
        c1, c2 = bez(p0, n0, p1, n1, e["bend"])
        d = f"M{p0[0]:.1f} {p0[1]:.1f} C{c1[0]:.1f} {c1[1]:.1f} {c2[0]:.1f} {c2[1]:.1f} {p1[0]:.1f} {p1[1]:.1f}"
    da = f' stroke-dasharray="{dash}"' if dash else ""
    path = f'<path d="{d}" fill="none" stroke="{color}" stroke-width="{width}"{da} marker-end="url(#m-{e["kind"]})"/>'
    lab = label_svg(e["lpos"][0], e["lpos"][1], e["label"], color) if e["label"] else ""
    return path, lab


def label_svg(x, y, s, color, anchor="middle"):
    tw = len(s) * 7.3 + 12
    bx = {"middle": x - tw / 2, "start": x - 4, "end": x - tw + 4}[anchor]
    return (f'<rect x="{bx:.1f}" y="{y - 10}" width="{tw:.1f}" height="19" rx="4" fill="{BG}" opacity="0.95"/>'
            + txt(x, y + 4.5, s, 13, color, "700", anchor))


def legend(y):
    out = [f'<line x1="16" y1="{y - 24}" x2="{W - 16}" y2="{y - 24}" stroke="#e2e8f0"/>']
    x = 16
    items = [("agent", "agent"), ("loops", "loops command"), ("gate", "gate: must pass"), ("art", "artifact"),
             ("store", "data store"), ("tool", "CLI / tool"), ("mcp", "MCP server"), ("human", "you"), ("hook", "hook")]
    for k, name in items:
        sw = {"agent": f'<rect x="{x}" y="{y - 9}" width="26" height="18" rx="4" fill="{tint(PH["qa"], 0.1)}" stroke="{PH["qa"]}" stroke-width="1.5"/>'
                       f'<rect x="{x}" y="{y - 7}" width="5" height="14" fill="{PH["qa"]}"/>',
              "loops": f'<rect x="{x}" y="{y - 9}" width="26" height="18" rx="2" fill="#f8fafc" stroke="#0f172a" stroke-width="1.4"/>',
              "gate": f'<polygon points="{x + 6},{y - 9} {x + 20},{y - 9} {x + 26},{y} {x + 20},{y + 9} {x + 6},{y + 9} {x},{y}" fill="#fff" stroke="#0f172a" stroke-width="1.8"/>',
              "art": f'<path d="M{x} {y - 9} H{x + 20} L{x + 26} {y - 3} V{y + 9} H{x} Z" fill="#f1f5f9" stroke="#94a3b8" stroke-width="1.3"/>',
              "store": f'<path d="M{x} {y - 5} V{y + 6} A13 4 0 0 0 {x + 26} {y + 6} V{y - 5}" fill="#f1f5f9" stroke="{STORE}" stroke-width="1.4"/>'
                       f'<ellipse cx="{x + 13}" cy="{y - 5}" rx="13" ry="4" fill="#e2e8f0" stroke="{STORE}" stroke-width="1.4"/>',
              "tool": f'<rect x="{x}" y="{y - 9}" width="26" height="18" rx="9" fill="#fff" stroke="{TOOL}" stroke-dasharray="4 2" stroke-width="1.5"/>',
              "mcp": f'<rect x="{x}" y="{y - 9}" width="26" height="18" rx="5" fill="{tint(MCP, 0.06)}" stroke="{MCP}" stroke-dasharray="4 2" stroke-width="1.5"/>',
              "human": f'<rect x="{x}" y="{y - 9}" width="26" height="18" rx="9" fill="#fff" stroke="#111827" stroke-width="1.6"/>',
              "hook": f'<rect x="{x}" y="{y - 9}" width="26" height="18" rx="2" fill="#fffbeb" stroke="{HOOK}" stroke-width="1.5"/>'
                      f'<rect x="{x}" y="{y - 9}" width="26" height="4" fill="{HOOK}"/>'}[k]
        out.append(sw + txt(x + 34, y + 5, name, 14, INK, "500", "start"))
        x += 34 + len(name) * 7.6 + 22
    y += 32
    x = 16
    for k, (color, width, dash, name) in EDGE.items():
        da = f' stroke-dasharray="{dash}"' if dash else ""
        out.append(f'<path d="M{x} {y} H{x + 40}" stroke="{color}" stroke-width="{width}"{da} marker-end="url(#m-{k})"/>'
                   + txt(x + 50, y + 5, name, 14, INK, "500", "start"))
        x += 50 + len(name) * 7.6 + 30
    return "".join(out)


def render():
    p = [f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" width="{W}" height="{H}" role="img" '
         f'aria-label="loops workflow graph: a ticket from GitHub, GitLab, Jira or Linear (MCP), plain text, or a backlog lane goes '
         f'through intake, research, plan, implement, qa, product and reflect. Each phase holds its agents and the tools and '
         f'services they use, and ends in a gate. Failed plans, claims and ACs are sent back. ai-memory is read at intake and '
         f'written at reflect; hooks log every step to events.jsonl.">',
         '<title>loops workflow graph</title>', '<defs>']
    for k, (color, *_r) in EDGE.items():
        p.append(f'<marker id="m-{k}" viewBox="0 0 10 10" refX="8.5" refY="5" markerWidth="7" markerHeight="7" '
                 f'orient="auto-start-reverse"><path d="M0 0 L10 5 L0 10 z" fill="{color}"/></marker>')
    p.append(f'</defs><rect width="{W}" height="{H}" fill="{BG}"/>')
    p.append(txt(16, 40, "loops workflow", 26, INK, "800", "start"))
    for g in GROUPS:
        p.append(draw_group(*g))
    drawn = [edge_svg(e) for e in EDGES]
    p.extend(d for d, _ in drawn)
    p.extend(draw(n) for n in ORDER)
    p.extend(l for _, l in drawn)
    for x, y, s, c, a in LABELS:
        p.append(label_svg(x, y, s, c, a))
    p.append(txt(16, R2 + GH + 40, "always on", 14, MUTED, "800", "start"))
    p.append(legend(LY))
    p.append("</svg>")
    return "\n".join(p)


if __name__ == "__main__":
    out = sys.argv[1] if len(sys.argv) > 1 else "."
    with open(f"{out}/loops-workflow-graph.svg", "w") as f:
        f.write(render())
    print(f"{out}/loops-workflow-graph.svg  {W}x{H}")
