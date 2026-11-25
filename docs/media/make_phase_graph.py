#!/usr/bin/env python3
"""Generate the simple loops phase graph (left to right, static SVG).

Usage: make_phase_graph.py <out_dir>
Writes loops-phases.svg: one card per run.Phase with who runs it, what it does
in a line, and its gate (what must be true to move on), plus the loop-backs and
what is always on. The full graph is make_run_graph.py.
"""
import sys
from html import escape

SANS = "-apple-system, BlinkMacSystemFont, 'Segoe UI', Inter, Helvetica, Arial, sans-serif"
MONO = "ui-monospace, SFMono-Regular, Menlo, Consolas, 'Liberation Mono', monospace"
INK, MUTED, BG, RED, GATE = "#0f172a", "#475569", "#ffffff", "#dc2626", "#0f172a"

M, IO_W, IO_GAP, CW, GAP = 16, 84, 18, 148, 20
CY, HEAD_H, CH = 118, 40, 284          # card top, header height, card height
GATE_H = 90                            # gate box height, at the card's bottom

# phase, colour, who, what it does, gate lines (the exit criterion, WORKFLOW §2)
PHASES = [
    ("intake", "#334155", "lead", ["Reads the ticket,", "fills the gaps,", "sets the scope"],
     ["charter.md:", "goal, ACs, scope"]),
    ("research", "#2563eb", "4 researchers", ["Web, practices,", "your code, your", "machine's limits"],
     ["4 briefs written"]),
    ("plan", "#7c3aed", "planner · reviewer", ["Waves of work", "and a proof", "for every AC"],
     ["reviewer says", "APPROVE"]),
    ("implement", "#0891b2", "architect · engineers", ["Contracts first,", "then parallel", "lanes, tests first"],
     ["each lane reports", "claims + evidence"]),
    ("qa", "#16a34a", "a QA agent per lane", ["Re-runs every", "claim, scans for", "quality, security"],
     ["claims VERIFIED", "loops gate green", "race · ≥70% coverage"]),
    ("product", "#ea580c", "product reviewer", ["Tries every AC", "by hand, then", "opens the PR"],
     ["every AC proven"]),
    ("reflect", "#db2777", "reflector", ["Saves lessons", "for the next run"],
     ["run ends only", "on a green gate"]),
]


def cx(i):
    return M + IO_W + IO_GAP + i * (CW + GAP)


W = cx(len(PHASES) - 1) + CW + IO_GAP + IO_W + M


def tint(c, a):
    r, g, b = (int(c[i:i + 2], 16) for i in (1, 3, 5))
    m = lambda v: round(255 - (255 - v) * a)
    return f"#{m(r):02x}{m(g):02x}{m(b):02x}"


def text(x, y, s, size, fill=INK, weight="400", anchor="middle", family=SANS, fit=None):
    if fit:
        size = min(size, fit / (0.52 * max(len(s), 1)))
    return (f'<text x="{x:.1f}" y="{y:.1f}" font-family="{family}" font-size="{size:.1f}" font-weight="{weight}" '
            f'fill="{fill}" text-anchor="{anchor}">{escape(s)}</text>')


def shield(x, y, color):
    s = f'stroke="{color}" stroke-width="1.8" fill="none" stroke-linecap="round" stroke-linejoin="round"'
    return (f'<path d="M{x} {y - 8} L{x + 7} {y - 5.5} V{y} C{x + 7} {y + 4.5} {x + 3.5} {y + 7} {x} {y + 8.5}'
            f' C{x - 3.5} {y + 7} {x - 7} {y + 4.5} {x - 7} {y} V{y - 5.5} Z" {s}/>'
            f'<path d="M{x - 3} {y} L{x - 0.5} {y + 2.5} L{x + 3.5} {y - 2}" {s}/>')


def card(i, phase, color, who, does, gate):
    x, y = cx(i), CY
    out = [f'<rect x="{x}" y="{y}" width="{CW}" height="{CH}" rx="12" fill="{tint(color, 0.07)}" stroke="{color}" stroke-width="1.6"/>',
           f'<path d="M{x} {y + 12} a12 12 0 0 1 12 -12 h{CW - 24} a12 12 0 0 1 12 12 v{HEAD_H - 12} h-{CW} z" fill="{color}"/>',
           text(x + CW / 2, y + 27, phase, 21, "#ffffff", "700", family=MONO),
           f'<rect x="{x + 10}" y="{y + HEAD_H + 10}" width="{CW - 20}" height="22" rx="11" fill="#ffffff" stroke="{color}" stroke-width="1"/>',
           text(x + CW / 2, y + HEAD_H + 25.5, who, 12.5, color, "700", fit=CW - 30)]
    ty = y + HEAD_H + 56
    for line in does:
        out.append(text(x + CW / 2, ty, line, 15, INK, "500", fit=CW - 14))
        ty += 20
    gy = y + CH - GATE_H - 10
    out.append(f'<rect x="{x + 8}" y="{gy}" width="{CW - 16}" height="{GATE_H}" rx="8" fill="#ffffff" stroke="{GATE}" stroke-width="1.8"/>')
    out.append(shield(x + 22, gy + 15, GATE))
    out.append(text(x + 34, gy + 19.5, "gate", 12, GATE, "800", anchor="start", family=MONO))
    ly = gy + 64 - (len(gate) - 1) * 8.5
    for j, line in enumerate(gate):
        out.append(text(x + CW / 2, ly + j * 17, line, 14 if j < 2 else 12.5, INK if j < 2 else MUTED,
                        "700" if j < 2 else "500", fit=CW - 26))
    return "".join(out)


def arrow(x1, x2, y):
    return f'<path d="M{x1} {y} H{x2 - 2}" stroke="#334155" stroke-width="2" marker-end="url(#a-flow)"/>'


def loop(i_from, i_to, f_from, f_to, peak, label, lx=None):
    x1, x2 = cx(i_from) + CW * f_from, cx(i_to) + CW * f_to
    yt = CY
    d = f"M{x1} {yt} C{x1} {peak} {x2} {peak} {x2} {yt - 3}"
    mid = (x1 + x2) / 2 if lx is None else lx
    ly = yt + (peak - yt) * 0.75
    tw = len(label) * 7.4 + 12
    return (f'<path d="{d}" fill="none" stroke="{RED}" stroke-width="2" stroke-dasharray="7 5" marker-end="url(#a-loop)"/>'
            f'<rect x="{mid - tw / 2}" y="{ly - 11}" width="{tw}" height="19" rx="4" fill="{BG}"/>'
            + text(mid, ly + 3, label, 13, RED, "700"))


def io(x, label, sub):
    y = CY + CH / 2 - 36
    return (f'<rect x="{x}" y="{y}" width="{IO_W}" height="72" rx="12" fill="#ffffff" stroke="#334155" stroke-width="1.6"/>'
            + text(x + IO_W / 2, y + 30, label, 15, INK, "800")
            + text(x + IO_W / 2, y + 50, sub, 12, MUTED, "500", fit=IO_W - 10))


def render():
    strip_y = CY + CH + 30
    leg_y = strip_y + 74
    H = leg_y + 30
    p = [f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" width="{W}" height="{H}" role="img" '
         f'aria-label="loops in a nutshell: a ticket goes through seven phases, each with a gate that must pass before the next. '
         f'intake ends with charter.md, research with four briefs, plan with the reviewer saying APPROVE, implement with every lane '
         f'reporting claims and evidence, qa with every claim VERIFIED and loops gate green, product with every acceptance criteria '
         f'proven and the PR opened, reflect with lessons saved. Failed plans, claims and ACs go back, a limited number of times.">',
         '<title>loops in a nutshell</title>',
         '<defs>'
         '<marker id="a-flow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto">'
         '<path d="M0 0 L10 5 L0 10 z" fill="#334155"/></marker>'
         f'<marker id="a-loop" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto">'
         f'<path d="M0 0 L10 5 L0 10 z" fill="{RED}"/></marker></defs>',
         f'<rect width="{W}" height="{H}" fill="{BG}"/>']
    # loop-backs above the cards
    p.append(loop(2, 2, 0.78, 0.22, CY - 46, "changes · max 3"))
    p.append(loop(4, 3, 0.3, 0.72, CY - 52, "fails · max 3"))
    p.append(loop(5, 3, 0.4, 0.38, CY - 106, "an AC fails · max 2", lx=cx(4) + CW / 2))
    # input, cards, output, arrows at gate level
    p.append(io(M, "ticket", "or issue"))
    gate_mid = CY + CH - 10 - GATE_H / 2
    p.append(arrow(M + IO_W, cx(0), CY + CH / 2))
    for i, ph in enumerate(PHASES):
        p.append(card(i, *ph))
        if i < len(PHASES) - 1:
            p.append(arrow(cx(i) + CW, cx(i + 1), gate_mid))
    p.append(arrow(cx(6) + CW, cx(6) + CW + IO_GAP, CY + CH / 2))
    p.append(io(cx(6) + CW + IO_GAP, "PR", "with proofs"))
    # always on
    p.append(f'<rect x="{M}" y="{strip_y}" width="{W - 2 * M}" height="48" rx="12" fill="#f8fafc" stroke="#e2e8f0"/>')
    p.append(text(M + 18, strip_y + 30, "always on", 14, MUTED, "800", anchor="start"))
    chips = ["hooks check every tool call", "every step is logged", "can't stop on a red gate", "asks you only where you opt in"]
    x = M + 118
    for c in chips:
        w = len(c) * 7.6 + 26
        p.append(f'<rect x="{x}" y="{strip_y + 11}" width="{w}" height="26" rx="13" fill="#ffffff" stroke="#94a3b8"/>'
                 + text(x + w / 2, strip_y + 29, c, 13.5, INK, "600"))
        x += w + 12
    # legend
    x = M
    p.append(f'<rect x="{x}" y="{leg_y - 9}" width="22" height="18" rx="4" fill="#ffffff" stroke="{GATE}" stroke-width="1.8"/>'
             + shield(x + 11, leg_y, GATE) + text(x + 30, leg_y + 5, "gate: must pass to move on", 13.5, INK, anchor="start"))
    x += 240
    p.append(f'<path d="M{x} {leg_y} H{x + 34}" stroke="#334155" stroke-width="2" marker-end="url(#a-flow)"/>'
             + text(x + 44, leg_y + 5, "next phase", 13.5, INK, anchor="start"))
    x += 140
    p.append(f'<path d="M{x} {leg_y} H{x + 34}" stroke="{RED}" stroke-width="2" stroke-dasharray="7 5" marker-end="url(#a-loop)"/>'
             + text(x + 44, leg_y + 5, "sent back (max rounds, then the lead decides)", 13.5, INK, anchor="start"))
    p.append("</svg>")
    return "\n".join(p)


if __name__ == "__main__":
    out = sys.argv[1] if len(sys.argv) > 1 else "."
    with open(f"{out}/loops-phases.svg", "w") as f:
        f.write(render())
    print(f"{out}/loops-phases.svg  {W}")
