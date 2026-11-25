#!/usr/bin/env python3
"""Generate the loops run graph (left to right, static SVG).

Usage: make_run_graph.py <out_dir>
Writes loops-run-graph.svg: the ticket or backlog input, the run.Phase states
with the roles, commands, artifacts and third parties of each phase, the hooks
and events under them, backlog mode at the bottom, and a legend. Every label is
a real name from the specs (WORKFLOW, BACKLOG, ROLES, hooks/hooks.json).
No scripts, external fonts or foreignObject: GitHub shows it as an image.
"""
import math
import sys
from html import escape

COLW, GAP, X0 = 176, 14, 24
EXTRA = {5: 18}   # a wider gutter between implement and qa for the qa loop-back


def col(i):
    return X0 + i * (COLW + GAP) + sum(v for k, v in EXTRA.items() if i >= k)


W = col(8) + COLW + 24
SANS = "-apple-system, BlinkMacSystemFont, 'Segoe UI', Inter, Helvetica, Arial, sans-serif"
MONO = "ui-monospace, SFMono-Regular, Menlo, Consolas, 'Liberation Mono', monospace"
INK, MUTED, BG = "#0f172a", "#64748b", "#ffffff"


# Roles (agents and people) and the other node kinds.
ROLES = {
    "lead": ("#334155", "lead (master agent)"),
    "supervisor": ("#0f766e", "backlog supervisor"),
    "research": ("#2563eb", "research"),
    "plan": ("#7c3aed", "plan"),
    "implement": ("#0891b2", "implement"),
    "qa": ("#16a34a", "qa"),
    "product": ("#ea580c", "product"),
    "reflect": ("#db2777", "reflect"),
}
PHASE_KIND = {"intake": "lead", "research": "research", "plan": "plan", "implement": "implement",
              "qa": "qa", "product": "product", "reflect": "reflect"}

EDGE = {
    # name: (colour, width, dasharray, marker, legend)
    "flow": ("#334155", 1.4, "", "arrow", "flow"),
    "loop": ("#dc2626", 1.6, "6 4", "arrow", "loop-back (limit)"),
    "writes": ("#94a3b8", 1.3, "1.5 3", "arrow", "writes"),
    "third": ("#0d9488", 1.3, "8 3 2 3", "arrow", "third-party call"),
    "event": ("#d97706", 1.3, "3 3", "arrow", "event → events.jsonl"),
    "deny": ("#b91c1c", 1.6, "", "tee", "deny / block"),
}


def tint(hex_color, a):
    r, g, b = (int(hex_color[i:i + 2], 16) for i in (1, 3, 5))
    mix = lambda c: round(255 - (255 - c) * a)
    return f"#{mix(r):02x}{mix(g):02x}{mix(b):02x}"


class Node:
    def __init__(self, nid, kind, lines, x, y, w, h):
        self.id, self.kind, self.lines = nid, kind, lines
        self.x, self.y, self.w, self.h = x, y, w, h

    def anchor(self, side, f=0.5):
        if side == "l":
            return (self.x, self.y + self.h * f), (-1, 0)
        if side == "r":
            return (self.x + self.w, self.y + self.h * f), (1, 0)
        if side == "t":
            return (self.x + self.w * f, self.y), (0, -1)
        return (self.x + self.w * f, self.y + self.h), (0, 1)

    @property
    def bottom(self):
        return self.y + self.h


N = {}
ORDER = []
GROUPS = []   # (x, y, w, h, title)
EDGES = []    # dicts
TEXTS = []    # raw svg snippets drawn last


def node(nid, kind, lines, x, y, w=COLW, h=None):
    if isinstance(lines, str):
        lines = [lines]
    if h is None:
        h = {"art": 22}.get(kind, 26) + (len(lines) - 1) * 13
    n = Node(nid, kind, lines, x, y, w, h)
    N[nid] = n
    ORDER.append(n)
    return n


class Stack:
    """Stacks nodes down one column."""

    def __init__(self, i, y, x_off=0, w=COLW):
        self.x, self.y, self.w = col(i) + x_off, y, w

    def add(self, nid, kind, lines, indent=0, w=None, gap=7, h=None):
        n = node(nid, kind, lines, self.x + indent, self.y, w or (self.w - indent), h)
        self.y = n.bottom + gap
        return n

    def art(self, nid, lines, of=None, w=None, gap=10):
        """An artifact right under its writer, leaving the right edge free for the flow line."""
        n = self.add(nid, "art", lines, indent=8, w=w or (self.w - 34), gap=gap)
        self.y = n.bottom + gap
        if of:
            edge(of, nid, "writes", ("b", 0.3), ("t", 0.27))
        return n

    def space(self, px):
        self.y += px


def edge(a, b, kind, sa=("r", 0.5), sb=("l", 0.5), label=None, via=None, t=0.5, lpos=None, bend=None):
    EDGES.append(dict(a=a, b=b, kind=kind, sa=sa, sb=sb, label=label, via=via, t=t, lpos=lpos, bend=bend))


def seq(*ids, f=0.9):
    """Vertical flow down a column, along the right edge so artifacts stay clear."""
    for a, b in zip(ids, ids[1:]):
        edge(a, b, "flow", ("b", f), ("t", f))


# ---------------------------------------------------------------- layout
TOP = 152          # body starts
PILL_Y = 104
SIDE = COLW - 40   # width of a side node, leaving the right lane free for the flow line


def G(i, d=0):
    """x of the gutter left of column i (d nudges it to keep parallel lines apart)."""
    return col(i) - GAP / 2 + d


# column 0: input
s = Stack(0, TOP)
s.add("work", "lead", ["/loops:work <ref>"], gap=22)
s.add("gh_t", "third", ["GitHub  #123"], indent=24, w=COLW - 24)
s.add("gl_t", "third", ["GitLab  URL"], indent=24, w=COLW - 24)
s.add("jira_t", "third", ["Jira  ABC-123 (MCP)"], indent=24, w=COLW - 24)
s.add("lin_t", "third", ["Linear (MCP)"], indent=24, w=COLW - 24)
s.add("txt_t", "human", ["plain text"], indent=24, w=COLW - 24)

# column 1: intake
s = Stack(1, TOP)
s.add("upd", "cmd", "loops update --apply")
s.add("init", "cmd", "loops init")
s.art("cfg", ".loops/config.json", of="init")
s.add("start", "cmd", "loops run start")
s.add("fetch", "cmd", "loops ticket fetch")
s.art("ticket", "ticket.md", of="fetch")
s.add("triage", "cmd", ["loops run triage", "trivial · small · standard"])
s.add("memr", "lead", ["memory_briefing", "memory_query"])
s.art("memmd", "research/memory.md", of="memr")
s.add("ask", "cmd", "loops run ask")
s.add("clarify", "lead", "loops:clarify")
s.art("clar", "clarifications.md", of="clarify")
s.add("charter", "lead", "charter.md")
seq("upd", "init", "start", "fetch", "triage", "memr", "ask", "clarify", "charter")
edge("work", "upd", "flow")
for t in ("gh_t", "gl_t", "jira_t", "lin_t", "txt_t"):
    edge(t, "fetch", "third")

# column 2: research (parallel)
RX = 16
RW = COLW - RX - 12
s = Stack(2, TOP, x_off=RX, w=RW)
s.add("webr", "research", "web-researcher")
s.add("websearch", "third", "WebSearch · WebFetch", indent=8, w=RW - 22)
s.art("webmd", "research/web.md", of="webr", w=RW - 30)
s.space(2)
s.add("pracr", "research", "practices-researcher")
s.art("pracmd", "research/practices.md", of="pracr", w=RW - 22)
s.space(2)
s.add("coder", "research", "code-researcher")
s.add("detect", "cmd", "loops lang detect --write", indent=8, w=RW - 8)
s.art("codemd", ["research/code.md", "research/language.json"], of="detect", w=RW - 22)
s.space(2)
s.add("hwr", "research", "hardware-researcher")
s.add("hw", "cmd", "loops hw --write", indent=8, w=RW - 8)
s.art("hwjson", "research/hardware.json", of="hw", w=RW - 22)
edge("webr", "websearch", "third", ("b", 0.92), ("t", 0.92))
edge("coder", "detect", "flow", ("b", 0.9), ("t", 0.9))
edge("hwr", "hw", "flow", ("b", 0.9), ("t", 0.9))
fy, fh = N["webr"].y - 4, N["hwr"].bottom - N["webr"].y + 8
node("fork_r", "bar", [], col(2) + 2, fy, 5, fh)
jy, jh = N["webmd"].y - 2, N["hwjson"].bottom - N["webmd"].y + 4
node("join_r", "bar", [], col(2) + COLW - 6, jy, 5, jh)
for r in ("webr", "pracr", "coder", "hwr"):
    edge("fork_r", r, "flow", ("r", (N[r].y + 13 - fy) / fh), ("l", 0.5))
for a in ("webmd", "pracmd", "codemd", "hwjson"):
    edge(a, "join_r", "flow", ("r", 0.5), ("l", (N[a].y + N[a].h / 2 - jy) / jh))
edge("charter", "fork_r", "flow", ("r", 0.5), ("b", 0.5), via=[("x", col(2) + 4.5)])

# column 3: plan
PW = COLW - 36
s = Stack(3, TOP, w=PW)
s.add("planner", "plan", "planner")
s.art("planmd", "plan.v<N>.md", of="planner", w=PW - 30)
s.add("pcheck", "cmd", "loops plan check")
s.add("previewer", "plan", ["plan-reviewer", "APPROVE · CHANGES"])
s.art("prmd", "plan-review.r<N>.md", of="previewer", w=PW - 30)
s.add("askplan", "human", "--ask plan", indent=30, w=PW - 30)
seq("planner", "pcheck", "previewer", f=0.92)
edge("join_r", "planner", "flow", ("r", 0.02), ("l", 0.5), via=[("x", G(3))])
edge("previewer", "planner", "loop", ("r", 0.3), ("r", 0.5), via="right")
TEXTS.append(("vlbl", col(3) + PW + 14, (N["planner"].y + N["previewer"].bottom) / 2 + 6, "CHANGES · MaxPlanRounds 3", "#dc2626"))
edge("previewer", "askplan", "flow", ("b", 0.92), ("t", 0.92), label="APPROVE",
     lpos=("at", col(3) + PW * 0.92 + 6, N["askplan"].y - 8, "start"))

# column 4: implement
s = Stack(4, TOP + 14, x_off=8, w=COLW - 16)
s.add("arch", "implement", "architect")
s.add("langeng", "implement", "<id>-engineer ×15", gap=14)
GROUPS.append((col(4), TOP, COLW, N["langeng"].bottom - TOP + 8, "wave 0"))
s.space(2)
s.add("gate0", "cmd", "loops gate --only build,vet", gap=26)
w1_top = s.y - 14
for nid, name in (("dom", "domain-engineer"), ("app", "application-engineer"), ("api", "api-engineer"),
                  ("per", "persistence-engineer"), ("int", "integration-engineer"), ("tst", "test-engineer"),
                  ("doc", "docs-writer")):
    s.add(nid, "implement", name, gap=5)
GROUPS.append((col(4), w1_top, COLW, s.y - w1_top + 4, "W1…n"))
node("w1", "ghost", [], col(4), w1_top, COLW, s.y - w1_top + 4)
s.space(14)
s.art("reports", "reports/<agent-id>.md", of="w1", w=COLW - 54)
s.add("recheck", "cmd", "loops run triage --recheck")
edge("arch", "gate0", "flow", ("b", 0.08), ("t", 0.08), via=[("x", col(4) + 3)])
edge("langeng", "gate0", "flow", ("b", 0.5), ("t", 0.5))
edge("gate0", "w1", "flow", ("b", 0.5), ("t", 0.5))
edge("askplan", "arch", "flow", ("r", 0.5), ("l", 0.5), via=[("x", G(4, -2))])
edge("reports", "recheck", "flow", ("b", 0.9), ("t", 0.9))

# column 5: qa
s = Stack(5, TOP)
s.add("qav", "qa", ["qa-verifier ×lane", "VERIFIED · FAILED"])
s.art("qamd", "qa/<agent-id>.r<N>.md", of="qav", w=SIDE)
s.add("qaq", "qa", ["qa-verifier", "qa-quality"])
s.add("crap", "third", "crapper · dryer · mutator", indent=8, w=SIDE + 6)
s.art("qqmd", "qa/quality.r<N>.md", of="qaq", w=SIDE - 8)
s.add("qas", "qa", ["security-reviewer", "qa-security"])
s.add("gsec", "cmd", "loops gate --security", indent=8, w=SIDE + 6)
s.add("scan", "third", "govulncheck · gosec …", indent=8, w=SIDE + 6)
s.art("qsmd", "qa/security.r<N>.md", of="qas", w=SIDE - 8, gap=12)
s.add("gate", "gate", ["loops gate"], h=118)
seq("qav", "qaq", "qas", "gate", f=0.93)
edge("recheck", "qav", "flow", ("r", 0.5), ("l", 0.72), via=[("x", G(5, 8))])
edge("qav", "w1", "loop", ("l", 0.28), ("r", 0.1), via=[("x", G(5, -7))])
edge("gate", "w1", "loop", ("l", 0.5), ("r", 0.9), via=[("x", G(5, -7))])
edge("qaq", "crap", "third", ("b", 0.4), ("t", 0.4))
edge("qas", "gsec", "flow", ("b", 0.4), ("t", 0.4))
edge("gsec", "scan", "third", ("b", 0.4), ("t", 0.4))
TEXTS.append(("vlbl", G(5, -7), N["int"].y + 10, "FAILED · EscalateAfter 2 · MaxQARounds 3", "#dc2626"))

# column 6: product
s = Stack(6, TOP)
s.add("prv", "product", "product-reviewer")
s.add("curl", "third", "curl · grpcurl · CLI", indent=8, w=SIDE)
s.art("proofs", "proofs/AC-<n>.md", of="prv", w=SIDE - 8)
s.art("prmd2", "pr.md", of=None, w=SIDE - 40)
s.add("report", "cmd", "loops report")
s.art("rhtml", "report.html", of="report", w=SIDE - 20)
s.add("askpr", "human", "--ask pr", indent=30, w=COLW - 30)
s.add("git", "third", ["git commit · push", "loops/<ticket>-<slug>"])
s.add("ghpr", "third", ["gh pr create", "glab mr create"])
edge("prv", "curl", "third", ("b", 0.4), ("t", 0.4))
edge("prv", "prmd2", "writes", ("l", 0.7), ("l", 0.5), bend=-6)
seq("prv", "report", "askpr", "git", "ghpr", f=0.93)
edge("gate", "prv", "flow", ("r", 0.5), ("l", 0.5), via=[("x", G(6))], label="all ok",
     lpos=("at", G(6), N["report"].y + 12))

# column 7: reflect
s = Stack(7, TOP)
s.add("refl", "reflect", "reflector")
s.art("reflmd", ["reflection.md", "overrides.md"], of="refl", w=SIDE)
s.add("ghc", "third", "gh pr comment", indent=8, w=SIDE)
s.space(6)
s.art("pages", ["decisions/ · gotchas/", "procedures/"], of=None, w=SIDE + 10)
s.space(36)
s.add("clean", "cmd", "loops run clean --dry-run")
s.add("end", "cmd", ["loops run end", "--outcome done"])
edge("reflmd", "ghc", "third", ("b", 0.5), ("t", 0.5))
edge("refl", "pages", "writes", ("l", 0.7), ("l", 0.5), bend=-6)
seq("refl", "clean", "end", f=0.93)

# column 8: output
s = Stack(8, TOP)
s.add("aimem", "third", "ai-memory (MCP)")
s.art("filemem", ".loops/memory/", of=None, w=COLW - 40)
s.y = N["ghpr"].y + N["ghpr"].h / 2 - 22
s.add("pr", "third", ["PR / MR", "proofs · decisions · overrides"], h=44)
s.add("merge", "human", "review · merge", indent=30, w=COLW - 30)
edge("pages", "aimem", "third", ("r", 0.3), ("l", 0.5), via=[("x", G(8))])
edge("pages", "filemem", "writes", ("r", 0.7), ("l", 0.5), via=[("x", G(8, 4))])
edge("ghc", "pr", "third", ("r", 0.5), ("l", 0.2), via=[("x", G(8, -4))])
edge("pr", "merge", "flow", ("b", 0.9), ("t", 0.9))

body_bottom = max(n.bottom for n in ORDER)
edge("ghpr", "pr", "third", ("r", 0.5), ("l", 0.5))

# phase pills: the run.Phase state machine
for i, ph in enumerate(["intake", "research", "plan", "implement", "qa", "product", "reflect"], start=1):
    w = 112
    node("p_" + ph, "pill", [ph], col(i) + (COLW - w) / 2, PILL_Y, w, 26)
phs = ["intake", "research", "plan", "implement", "qa", "product", "reflect"]
for a, b in zip(phs, phs[1:]):
    edge("p_" + a, "p_" + b, "flow")
edge("p_product", "p_implement", "loop", ("t", 0.35), ("t", 0.65), via="arc:-46")
TEXTS.append(("lbl", (col(4) + col(6)) / 2 + COLW / 2, 72, "AC FAILED · MaxProductRounds 2", "#dc2626", "middle"))
edge("p_qa", "p_reflect", "flow", ("t", 0.8), ("t", 0.2), via="arc:-24")
TEXTS.append(("lbl", col(7) - 4, 87, "memory.store = files", "#334155", "middle"))
edge("p_reflect", "p_product", "flow", ("b", 0.2), ("b", 0.8), via="arc:16")
TEXTS.append(("raw", f'<text x="{col(0) + COLW - 4}" y="{PILL_Y + 17}" font-family="{MONO}" font-size="12" '
                     f'fill="{MUTED}" text-anchor="end" font-weight="600">run.Phase</text>'))
edge("aimem", "p_intake", "third", ("t", 0.5), ("t", 0.5), via="next-run", label="next run")

# ---------------------------------------------------------------- hooks and events band
HY = body_bottom + 62
h = Stack(0, HY + 4, x_off=24, w=COLW - 24)
h.add("hss", "hook", ["SessionStart", "session-start"], gap=18)
h.add("hstop", "hook", ["Stop", "stop-gate"])
h = Stack(1, HY + 12)
for nid, name in (("rg", "read-guard"), ("tg", "test-guard"), ("te", "toolchain-env"), ("mg", "model-guard")):
    h.add(nid, "hook", name, gap=5)
GROUPS.append((col(1) - 4, HY, COLW + 8, h.y - HY + 4, "PreToolUse"))
node("pre", "ghost", [], col(1) - 4, HY, COLW + 8, h.y - HY + 4)
h = Stack(2, HY + 36)
h.add("tools", "third", ["Bash · Read · Edit", "Write · Agent"], indent=18, w=COLW - 36)
h = Stack(3, HY + 12)
h.add("pe", "hook", ["PostToolUse", "post-edit · trace"], gap=18)
h.add("ta", "hook", ["SubagentStart/Stop", "trace-agent"])
h = Stack(4, HY + 4)
h.add("cmds", "cmd", ["loops run …", "loops gate"], indent=20, w=COLW - 40, gap=18)
h.add("ev", "art", [".loops/runs/<id>/", "events.jsonl"], h=40)
h = Stack(5, HY + 36)
h.add("fold", "cmd", ["run.Fold", "loops status --json"], indent=8, w=COLW - 16)
h.art("status", ".loops/status.json", of="fold", w=COLW - 40)
h = Stack(6, HY + 4, x_off=16, w=COLW - 32)
h.add("sl", "third", "status line", gap=5)
h.add("pane", "cmd", "mod pane", gap=5)
h.add("watch", "cmd", "loops watch", gap=5)
h.add("rep2", "cmd", "loops report", gap=5)
h = Stack(7, HY + 12)
h.add("dirty", "state", "source_dirty", indent=30, w=COLW - 60, gap=42)
h.add("clean_st", "state", "gate all ok", indent=30, w=COLW - 60)

edge("hss", "work", "flow", ("l", 0.5), ("l", 0.35), via=[("x", 14)], label="loops run brief",
     lpos=("at", col(0) + 26, N["hss"].bottom + 9, "start"))
edge("hstop", "work", "deny", ("l", 0.5), ("l", 0.75), via=[("x", 20)], label="while source_dirty",
     lpos=("at", col(0) + 26, N["hstop"].bottom + 10, "start"))
edge("pre", "tools", "deny", ("r", 0.5), ("l", 0.5))
edge("tools", "pe", "flow", ("r", 0.3), ("l", 0.5))
edge("tools", "ta", "flow", ("r", 0.7), ("l", 0.5))
edge("pe", "ev", "event", ("r", 0.5), ("l", 0.3))
edge("ta", "ev", "event", ("r", 0.5), ("l", 0.75))
edge("cmds", "ev", "event", ("b", 0.5), ("t", 0.5))
edge("pre", "ev", "event", ("b", 0.5), ("b", 0.5), via="under", label="guard_deny")
edge("dirty", "clean_st", "flow", ("l", 0.5), ("l", 0.5), bend=-22, label="loops gate", lpos=("at", col(7) + 4, HY + 64, "start"))
edge("clean_st", "dirty", "flow", ("r", 0.5), ("r", 0.5), bend=-22, label="edit", lpos=("at", col(7) + COLW - 4, HY + 64, "end"))
edge("ev", "fold", "flow", ("r", 0.5), ("l", 0.5))
edge("status", "sl", "flow", ("r", 0.5), ("l", 0.5))
edge("status", "pane", "flow", ("r", 0.5), ("l", 0.5))
edge("fold", "watch", "flow", ("r", 0.6), ("l", 0.5))
edge("fold", "rep2", "flow", ("r", 0.8), ("l", 0.5))
edge("dirty", "end", "deny", ("t", 0.5), ("b", 0.5), label="end gate", lpos=("at", col(7) + COLW / 2 + 4, N["end"].bottom + 30, "start"))

hooks_bottom = max(n.bottom for n in ORDER if n.y >= HY - 30) + 22

# ---------------------------------------------------------------- backlog band
BY = hooks_bottom + 64
b = Stack(0, BY, x_off=24, w=COLW - 24)
b.add("sup", "supervisor", ["/loops:backlog", "loops backlog run"], gap=12)
b.add("you_h", "human", "--human <login>")
b = Stack(1, BY)
b.add("sync", "cmd", "loops backlog sync")
b.add("ghil", "third", "gh issue list", indent=24, w=COLW - 24)
b = Stack(2, BY)
b.art("state", [".loops/backlog/", "state.json"], of=None, w=COLW - 44)
b = Stack(3, BY)
b.add("bwork", "cmd", ["loops backlog work", "--max-prs 5"])
b.add("wt", "third", ["git worktree add", "issue-<n>-<k>"], indent=24, w=COLW - 24)
b = Stack(4, BY)
b.add("lane", "lead", ["claude -p", "\"/loops:work <repo>#<n>\""], h=42)
# issue status FSM (backlog.Status)
SW, SG = 84, 57
fx, fy = col(5) + 16, BY + 4
states = {"queued": (0, 0), "running": (1, 0), "done": (2, 0), "skipped": (0, 1), "failed": (1, 1), "human": (2, 1)}
for st, (i, j) in states.items():
    node("s_" + st, "state", st, fx + i * (SW + SG), fy + j * 74, SW, 26)
GROUPS.append((col(5), BY - 12, 3 * SW + 2 * SG + 30, 74 + 26 + 50, "backlog.Status"))
b = Stack(7, BY, x_off=24, w=COLW - 24)
b.add("finding", "cmd", ["loops backlog work", "loops backlog file"])
b = Stack(8, BY)
b.add("ghic", "third", ["gh issue create", "loops-finding"])
b.space(36)
b.add("handoff", "third", ["comment · assign", "loops-needs-human"])

edge("sup", "sync", "flow")
edge("sync", "ghil", "third", ("b", 0.6), ("t", 0.55))
edge("sync", "state", "writes", ("r", 0.5), ("l", 0.3))
edge("state", "bwork", "flow", ("r", 0.3), ("l", 0.3), label="claim", t=0.5)
edge("bwork", "wt", "third", ("b", 0.6), ("t", 0.55))
edge("wt", "lane", "flow", ("r", 0.5), ("l", 0.7))
edge("lane", "work", "flow", ("t", 0.5), ("l", 0.15), via="lane-up", label="one run per lane",
     lpos=("at", col(2) + 40, BY - 44))
edge("s_queued", "s_running", "flow", label="claim")
edge("s_running", "s_done", "flow", label="pr")
edge("s_running", "s_failed", "flow", ("b", 0.5), ("t", 0.5))
edge("s_failed", "s_queued", "loop", ("l", 0.5), ("b", 0.8), label="--attempts 2",
     lpos=("at", fx + SW + SG / 2 + 4, fy + 52))
edge("s_failed", "s_human", "flow")
edge("s_running", "s_human", "flow", ("r", 0.8), ("t", 0.2), label="needs-human", t=0.62)
edge("s_queued", "s_skipped", "flow", ("b", 0.2), ("t", 0.2))
edge("s_human", "s_queued", "loop", ("b", 0.5), ("l", 0.5), via=[("y", fy + 74 + 26 + 18), ("x", col(5) + 6)],
     label="unassign", lpos="seg0")
edge("ev", "s_running", "event", ("b", 0.75), ("t", 0.5), via=[("y", BY - 36)], label="outcome",
     lpos=("at", N["s_running"].x + SW / 2 + 6, BY - 24, "start"))
edge("lane", "finding", "event", ("b", 0.5), ("b", 0.5), via=[("y", BY + 148)], label="note finding",
     lpos=("at", N["finding"].x + N["finding"].w / 2 + 6, N["finding"].bottom + 16, "start"))
edge("s_human", "handoff", "flow", ("r", 0.5), ("l", 0.5))
edge("finding", "ghic", "third", ("r", 0.5), ("l", 0.5))
edge("handoff", "you_h", "flow", ("b", 0.5), ("b", 0.5), via=[("y", BY + 162)], label="assignee",
     lpos=("at", col(2) + 40, BY + 162))
edge("ghic", "handoff", "flow", ("b", 0.5), ("t", 0.5))

backlog_bottom = BY + 172
backlog_band = (12, BY - 30, W - 24, backlog_bottom - BY + 34, "backlog mode")
hooks_band = (12, HY - 30, W - 24, hooks_bottom - HY + 34, "hooks · events")
LY = backlog_bottom + 40
H = LY + 50


# ---------------------------------------------------------------- rendering
def text_lines(n, color, weight="500", size=11.0, anchor="middle"):
    out = []
    lines = n.lines
    for j, line in enumerate(lines):
        sz = size if j == 0 else size - 1.2
        fit = (n.w - 14) / (0.61 * max(len(line), 1))
        sz = min(sz, fit)
        cy = n.y + n.h / 2 + (j - (len(lines) - 1) / 2) * 13 + sz * 0.36
        fill = color if j == 0 else MUTED
        wt = weight if j == 0 else "400"
        out.append(f'<text x="{n.x + n.w / 2:.1f}" y="{cy:.1f}" font-family="{MONO}" font-size="{sz:.1f}" '
                   f'font-weight="{wt}" fill="{fill}" text-anchor="{anchor}">{escape(line)}</text>')
    return "".join(out)


def draw_node(n):
    x, y, w, h = n.x, n.y, n.w, n.h
    k = n.kind
    if k in ROLES:
        c = ROLES[k][0]
        return (f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="6" fill="{tint(c, 0.1)}" stroke="{c}" stroke-width="1.3"/>'
                f'<path d="M{x + 6} {y} h-1 a5 5 0 0 0 -5 5 v{h - 10} a5 5 0 0 0 5 5 h1 z" fill="{c}"/>'
                f'<rect x="{x}" y="{y + 3}" width="4" height="{h - 6}" fill="{c}"/>' + text_lines(n, INK, "600"))
    if k == "human":
        r = h / 2
        px, py = x + 13, y + h / 2
        return (f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="{r}" fill="#ffffff" stroke="#111827" stroke-width="1.4"/>'
                f'<circle cx="{px}" cy="{py - 4}" r="3.2" fill="#111827"/>'
                f'<path d="M{px - 5.5} {py + 7} a5.5 5.5 0 0 1 11 0 z" fill="#111827"/>' + text_lines(n, INK, "600"))
    if k == "cmd":
        return (f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="3" fill="#f8fafc" stroke="#0f172a" stroke-width="1.2"/>'
                + text_lines(n, INK, "500"))
    if k == "hook":
        return (f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="3" fill="#fffbeb" stroke="#b45309" stroke-width="1.3"/>'
                f'<rect x="{x}" y="{y}" width="{w}" height="3" fill="#b45309"/>' + text_lines(n, "#92400e", "600"))
    if k == "art":
        f = 7
        return (f'<path d="M{x} {y} H{x + w - f} L{x + w} {y + f} V{y + h} H{x} Z" fill="#f1f5f9" stroke="#94a3b8" stroke-width="1.1"/>'
                f'<path d="M{x + w - f} {y} V{y + f} H{x + w}" fill="none" stroke="#94a3b8" stroke-width="1.1"/>'
                + text_lines(n, "#334155", "500", 10.5))
    if k == "third":
        r = min(h / 2, 13)
        return (f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="{r}" fill="#ffffff" stroke="#0d9488" '
                f'stroke-width="1.3" stroke-dasharray="5 3"/>' + text_lines(n, "#115e59", "600"))
    if k == "state":
        return (f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="{h / 2}" fill="#ffffff" stroke="#334155" stroke-width="1.8"/>'
                + text_lines(n, INK, "700", 11))
    if k == "pill":
        c = ROLES[PHASE_KIND[n.lines[0]]][0]
        return (f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="{h / 2}" fill="{c}"/>'
                f'<text x="{x + w / 2}" y="{y + h / 2 + 4.6}" font-family="{MONO}" font-size="13" font-weight="700" '
                f'fill="#ffffff" text-anchor="middle">{escape(n.lines[0])}</text>')
    if k == "bar":
        return f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="1.5" fill="#0f172a"/>'
    if k == "gate":
        out = [f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="3" fill="#f8fafc" stroke="#0f172a" stroke-width="1.6"/>',
               f'<text x="{x + w / 2}" y="{y + 17}" font-family="{MONO}" font-size="11.5" font-weight="700" fill="{INK}" '
               f'text-anchor="middle">loops gate</text>']
        chips = [["deps", "build", "format"], ["lint", "typecheck", "arch"], ["test (race)"], ["coverage ≥ 70%"]]
        cy = y + 28
        for row in chips:
            widths = [len(c) * 6.3 + 12 for c in row]
            total = sum(widths) + 5 * (len(row) - 1)
            cxp = x + (w - total) / 2
            for c, cw in zip(row, widths):
                hl = "#16a34a" if "race" in c or "coverage" in c else "#475569"
                out.append(f'<rect x="{cxp:.1f}" y="{cy}" width="{cw:.1f}" height="17" rx="8.5" fill="#ffffff" stroke="{hl}" stroke-width="1"/>'
                           f'<text x="{cxp + cw / 2:.1f}" y="{cy + 12}" font-family="{MONO}" font-size="10" fill="{INK}" '
                           f'text-anchor="middle">{escape(c)}</text>')
                cxp += cw + 5
            cy += 22
        return "".join(out)
    return ""


def bez(p0, n0, p1, n1, bend=None):
    d = max(18, min(110, math.dist(p0, p1) * 0.45))
    if bend is not None:
        d = abs(bend) + 10
    c1 = (p0[0] + n0[0] * d, p0[1] + n0[1] * d)
    c2 = (p1[0] + n1[0] * d, p1[1] + n1[1] * d)
    return c1, c2


def point_on(p0, c1, c2, p1, t):
    u = 1 - t
    return tuple(u ** 3 * p0[i] + 3 * u * u * t * c1[i] + 3 * u * t * t * c2[i] + t ** 3 * p1[i] for i in (0, 1))


def rounded(points, r=9):
    d = f"M{points[0][0]:.1f} {points[0][1]:.1f}"
    for i in range(1, len(points) - 1):
        (ax, ay), (bx, by), (cx_, cy_) = points[i - 1], points[i], points[i + 1]
        l1 = math.dist((ax, ay), (bx, by)) or 1
        l2 = math.dist((bx, by), (cx_, cy_)) or 1
        rr = min(r, l1 / 2, l2 / 2)
        p = (bx - (bx - ax) / l1 * rr, by - (by - ay) / l1 * rr)
        q = (bx + (cx_ - bx) / l2 * rr, by + (cy_ - by) / l2 * rr)
        d += f" L{p[0]:.1f} {p[1]:.1f} Q{bx:.1f} {by:.1f} {q[0]:.1f} {q[1]:.1f}"
    d += f" L{points[-1][0]:.1f} {points[-1][1]:.1f}"
    return d


def ortho(p0, p1, spec):
    """Polyline through ('x', X) / ('y', Y) channel specs."""
    pts = [p0]
    cur = p0
    for axis, v in spec:
        if axis == "x":
            nxt = (v, cur[1])
        else:
            nxt = (cur[0], v)
        pts.append(nxt)
        cur = nxt
    # finish: align to the target on the remaining axis
    if spec and spec[-1][0] == "x":
        pts.append((cur[0], p1[1]))
    else:
        pts.append((p1[0], cur[1]))
    pts.append(p1)
    return pts


def draw_edge(e):
    a, b = N[e["a"]], N[e["b"]]
    color, width, dash, marker, _ = EDGE[e["kind"]]
    p0, n0 = a.anchor(*e["sa"])
    p1, n1 = b.anchor(*e["sb"])
    via = e["via"]
    label_at = None
    if via is None:
        c1, c2 = bez(p0, n0, p1, n1, e["bend"])
        d = f"M{p0[0]:.1f} {p0[1]:.1f} C{c1[0]:.1f} {c1[1]:.1f} {c2[0]:.1f} {c2[1]:.1f} {p1[0]:.1f} {p1[1]:.1f}"
        label_at = point_on(p0, c1, c2, p1, e["t"])
    elif isinstance(via, str) and via.startswith("arc:"):
        h = float(via[4:])
        mx = (p0[0] + p1[0]) / 2
        c1, c2 = (p0[0], p0[1] + h), (p1[0], p1[1] + h)
        d = f"M{p0[0]:.1f} {p0[1]:.1f} C{c1[0]:.1f} {c1[1]:.1f} {c2[0]:.1f} {c2[1]:.1f} {p1[0]:.1f} {p1[1]:.1f}"
        label_at = (mx, p0[1] + h * 0.75 - 4)
    elif via == "right":
        xr = max(p0[0], p1[0]) + 14
        pts = [p0, (xr, p0[1]), (xr, p1[1]), p1]
        d = rounded(pts)
        label_at = (xr + 2, (p0[1] + p1[1]) / 2)
    elif via == "next-run":
        yt = 50
        pts = [p0, (p0[0], yt), (p1[0], yt), p1]
        d = rounded(pts, 14)
        label_at = ((p0[0] + p1[0]) / 2, yt - 6)
    elif via == "under":
        yb = max(p0[1], p1[1]) + 16
        pts = [p0, (p0[0], yb), (p1[0], yb), p1]
        d = rounded(pts)
        label_at = ((p0[0] + p1[0]) / 2, yb + 12)
    elif via == "under-fsm":
        yb = max(p0[1], p1[1]) + 22
        pts = [p0, (p0[0], yb), (p1[0], yb), p1]
        d = rounded(pts)
        label_at = ((p0[0] + p1[0]) / 2, yb + 12)
    elif via == "lane-up":
        ych = BY - 44
        xl = 8
        pts = [p0, (p0[0], ych), (xl, ych), (xl, p1[1]), p1]
        d = rounded(pts, 12)
        label_at = (xl + 4, (ych + p1[1]) / 2)
    else:
        pts = ortho(p0, p1, via)
        d = rounded(pts)
        seg = 0
        if isinstance(e["lpos"], str) and e["lpos"].startswith("seg"):
            seg = int(e["lpos"][3:])
        (sx, sy), (tx, ty) = pts[seg + 1], pts[seg + 2]
        label_at = ((sx + tx) / 2, (sy + ty) / 2)
    mk = f'marker-end="url(#{marker}-{e["kind"]})"'
    da = f' stroke-dasharray="{dash}"' if dash else ""
    path = f'<path d="{d}" fill="none" stroke="{color}" stroke-width="{width}"{da} {mk}/>'
    lpos = e["lpos"]
    if isinstance(lpos, tuple):
        label_at = lpos[1:3]
    label = edge_label(e["label"], label_at, color, lpos) if e["label"] and label_at else ""
    return path, label


def edge_label(txt, at, color, lpos):
    x, y = at
    tw = len(txt) * 5.9 + 8
    anchor = "middle"
    if isinstance(lpos, tuple) and len(lpos) > 3:
        anchor = lpos[3]
    elif lpos in ("right", "left"):
        anchor = "start"
    bx = {"middle": x - tw / 2, "start": x - 3, "end": x - tw + 3}[anchor]
    if lpos == "right":
        x, bx = x + 4, x + 1
    return (f'<rect x="{bx:.1f}" y="{y - 8:.1f}" width="{tw:.1f}" height="13" rx="3" fill="{BG}" opacity="0.92"/>'
            f'<text x="{x:.1f}" y="{y + 2:.1f}" font-family="{MONO}" font-size="9.8" fill="{color}" '
            f'text-anchor="{anchor}" font-weight="600">{escape(txt)}</text>')


def markers():
    out = []
    for k, (color, _, _, marker, _) in EDGE.items():
        if marker == "arrow":
            out.append(f'<marker id="arrow-{k}" viewBox="0 0 10 10" refX="8.6" refY="5" markerWidth="7" markerHeight="7" '
                       f'orient="auto-start-reverse"><path d="M0 0 L10 5 L0 10 z" fill="{color}"/></marker>')
        else:
            out.append(f'<marker id="tee-{k}" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="8" markerHeight="8" '
                       f'orient="auto"><path d="M9 0 V10" stroke="{color}" stroke-width="2.4"/>'
                       f'<path d="M0 5 H9" stroke="{color}" stroke-width="1.6"/></marker>')
    return "".join(out)


def band(x, y, w, h, title):
    return (f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="12" fill="#f8fafc" stroke="#e2e8f0" stroke-width="1"/>'
            f'<text x="{x + 14}" y="{y + 17}" font-family="{SANS}" font-size="12" font-weight="700" fill="{MUTED}" '
            f'letter-spacing="0.6">{escape(title)}</text>')


def group(x, y, w, h, title):
    return (f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="8" fill="none" stroke="#94a3b8" stroke-width="1" '
            f'stroke-dasharray="4 3"/>'
            f'<rect x="{x + 8}" y="{y - 7}" width="{len(title) * 6.2 + 10}" height="13" fill="{BG}"/>'
            f'<text x="{x + 13}" y="{y + 3}" font-family="{MONO}" font-size="10" font-weight="600" fill="{MUTED}">'
            f'{escape(title)}</text>')


def legend(y):
    out = [f'<line x1="24" y1="{y - 14}" x2="{W - 24}" y2="{y - 14}" stroke="#e2e8f0"/>']
    x = 24
    out.append(f'<text x="{x}" y="{y + 4}" font-family="{SANS}" font-size="11.5" font-weight="700" fill="{MUTED}">roles</text>')
    x += 46
    for k, (c, name) in ROLES.items():
        out.append(f'<rect x="{x}" y="{y - 7}" width="16" height="14" rx="3" fill="{tint(c, 0.12)}" stroke="{c}" stroke-width="1.3"/>'
                   f'<rect x="{x}" y="{y - 6}" width="4" height="12" fill="{c}"/>'
                   f'<text x="{x + 22}" y="{y + 4}" font-family="{SANS}" font-size="11.5" fill="{INK}">{escape(name)}</text>')
        x += 30 + len(name) * 6.3
    out.append(f'<rect x="{x}" y="{y - 7}" width="18" height="14" rx="7" fill="#fff" stroke="#111827" stroke-width="1.3"/>'
               f'<text x="{x + 24}" y="{y + 4}" font-family="{SANS}" font-size="11.5" fill="{INK}">human</text>')
    x += 80
    kinds = [("cmd", "loops command"), ("hook", "hook"), ("art", "artifact"), ("third", "third party"),
             ("state", "FSM state"), ("bar", "parallel fork / join")]
    for k, name in kinds:
        if k == "cmd":
            sw = f'<rect x="{x}" y="{y - 7}" width="18" height="14" rx="2" fill="#f8fafc" stroke="#0f172a" stroke-width="1.2"/>'
        elif k == "hook":
            sw = (f'<rect x="{x}" y="{y - 7}" width="18" height="14" rx="2" fill="#fffbeb" stroke="#b45309" stroke-width="1.2"/>'
                  f'<rect x="{x}" y="{y - 7}" width="18" height="3" fill="#b45309"/>')
        elif k == "art":
            sw = (f'<path d="M{x} {y - 7} H{x + 13} L{x + 18} {y - 2} V{y + 7} H{x} Z" fill="#f1f5f9" stroke="#94a3b8"/>')
        elif k == "third":
            sw = f'<rect x="{x}" y="{y - 7}" width="18" height="14" rx="7" fill="#fff" stroke="#0d9488" stroke-dasharray="4 2" stroke-width="1.2"/>'
        elif k == "state":
            sw = f'<rect x="{x}" y="{y - 7}" width="18" height="14" rx="7" fill="#fff" stroke="#334155" stroke-width="1.8"/>'
        else:
            sw = f'<rect x="{x + 7}" y="{y - 8}" width="4" height="16" fill="#0f172a"/>'
        out.append(sw + f'<text x="{x + 24}" y="{y + 4}" font-family="{SANS}" font-size="11.5" fill="{INK}">{escape(name)}</text>')
        x += 34 + len(name) * 6.3
    y2 = y + 30
    x = 24
    out.append(f'<text x="{x}" y="{y2 + 4}" font-family="{SANS}" font-size="11.5" font-weight="700" fill="{MUTED}">edges</text>')
    x += 46
    for k, (color, width, dash, marker, name) in EDGE.items():
        da = f' stroke-dasharray="{dash}"' if dash else ""
        out.append(f'<path d="M{x} {y2} H{x + 40}" stroke="{color}" stroke-width="{width}"{da} '
                   f'marker-end="url(#{marker}-{k})"/>'
                   f'<text x="{x + 50}" y="{y2 + 4}" font-family="{SANS}" font-size="11.5" fill="{INK}">{escape(name)}</text>')
        x += 70 + len(name) * 6.3
    return "".join(out)


def render():
    parts = [f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" width="{W}" height="{H}" role="img" '
             f'aria-label="loops run graph: a ticket or a backlog lane goes left to right through intake, research, plan, '
             f'implement, qa, product and reflect, with roles, loops commands, artifacts, third parties, loop-backs, '
             f'gates, hooks, events and the backlog issue states.">',
             '<title>loops run graph</title>',
             f'<defs>{markers()}</defs>',
             f'<rect width="{W}" height="{H}" fill="{BG}"/>',
             f'<text x="24" y="34" font-family="{SANS}" font-size="20" font-weight="800" fill="{INK}">loops run graph</text>',
             ]
    parts.append(band(*hooks_band))
    parts.append(band(*backlog_band))
    for g in GROUPS:
        parts.append(group(*g))
    drawn = [draw_edge(e) for e in EDGES]
    parts.extend(path for path, _ in drawn)
    for n in ORDER:
        parts.append(draw_node(n))
    parts.extend(label for _, label in drawn)
    for t in TEXTS:
        if t[0] == "raw":
            parts.append(t[1])
        elif t[0] == "vlbl":
            _, x, y, txt, color = t
            parts.append(f'<g transform="rotate(-90 {x:.1f} {y:.1f})">'
                         + edge_label(txt, (x, y + 1.5), color, ("at", x, y, "middle")) + '</g>')
        else:
            _, x, y, txt, color, *anchor = t
            parts.append(edge_label(txt, (x, y), color, ("at", x, y, anchor[0] if anchor else "start")))
    parts.append(legend(LY))
    parts.append("</svg>")
    return "\n".join(parts)


if __name__ == "__main__":
    out = sys.argv[1] if len(sys.argv) > 1 else "."
    with open(f"{out}/loops-run-graph.svg", "w") as f:
        f.write(render())
    print(f"{out}/loops-run-graph.svg  {W}x{H}")
