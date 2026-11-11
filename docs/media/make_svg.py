#!/usr/bin/env python3
"""Generate the loops workflow graphic (static + animated SVG).

Usage: make_svg.py <out_dir>
Writes loops-workflow.svg (animated, honours prefers-reduced-motion) and
loops-workflow-static.svg (no animation, for PNG export).
"""
import sys
from html import escape

W, H = 1290, 750
FONT = "-apple-system, BlinkMacSystemFont, 'Segoe UI', Inter, Helvetica, Arial, sans-serif"
INK, MUTED, LINE, CARD_BORDER = "#0f172a", "#475569", "#94a3b8", "#e2e8f0"

CW, CH = 262, 178            # card size
COLS = [222, 572, 922]       # card x positions
ROW1, ROW2 = 132, 452        # card y positions

PHASES = [
    # key, n, title, headline, plain text lines, chips, accent, (x, y), icon
    ("research", 1, "Research", "Look before leaping",
     ["Reads the web, best practices,", "your code and your machine", "before anything is decided."],
     ["web", "practices", "code", "hardware"], "#2563eb", (COLS[0], ROW1), "search"),
    ("plan", 2, "Plan", "Agree on the plan",
     ["A planner drafts it; a second", "reviewer pushes back until it", "is solid. You can approve it."],
     ["planner", "reviewer", "you"], "#7c3aed", (COLS[1], ROW1), "clipboard"),
    ("build", 3, "Build", "Specialists in parallel",
     ["An architect sets the contracts,", "then engineers build side by", "side, each in their own lane."],
     ["architect", "domain", "api", "tests"], "#0891b2", (COLS[2], ROW1), "layers"),
    ("verify", 4, "Verify", "Trust, but verify",
     ["Independent QA re-checks every", "claim and sends failures back", "with proof of what broke."],
     ["qa per engineer", "quality gates"], "#16a34a", (COLS[2], ROW2), "shield"),
    ("prove", 5, "Prove", "Show that it works",
     ["Tests every acceptance criteria", "by hand, then opens the pull", "request with the evidence."],
     ["product reviewer", "PR + proofs"], "#ea580c", (COLS[1], ROW2), "target"),
    ("reflect", 6, "Reflect", "Learn for next time",
     ["Saves lessons for future runs and", "reports where your project's", "rules overrode ours."],
     ["reflector", "ai-memory"], "#db2777", (COLS[0], ROW2), "bulb"),
]

CYCLE = 15.0          # seconds for one full animation loop
STEP = 2.0            # seconds each phase is highlighted


def icon(kind, cx, cy, color):
    s = f'stroke="{color}" stroke-width="2.2" fill="none" stroke-linecap="round" stroke-linejoin="round"'
    if kind == "search":
        return f'<circle cx="{cx-2}" cy="{cy-2}" r="6.5" {s}/><path d="M{cx+3} {cy+3} L{cx+8} {cy+8}" {s}/>'
    if kind == "clipboard":
        return (f'<rect x="{cx-7}" y="{cy-8}" width="14" height="17" rx="2.5" {s}/>'
                f'<path d="M{cx-3} {cy-1} h6 M{cx-3} {cy+3.5} h6" {s}/>')
    if kind == "layers":
        return (f'<path d="M{cx} {cy-8} L{cx+9} {cy-3.5} L{cx} {cy+1} L{cx-9} {cy-3.5} Z" {s}/>'
                f'<path d="M{cx-9} {cy+1.5} L{cx} {cy+6} L{cx+9} {cy+1.5}" {s}/>')
    if kind == "shield":
        return (f'<path d="M{cx} {cy-9} L{cx+8} {cy-6} V{cy} C{cx+8} {cy+5} {cx+4} {cy+8} {cx} {cy+10}'
                f' C{cx-4} {cy+8} {cx-8} {cy+5} {cx-8} {cy} V{cy-6} Z" {s}/>'
                f'<path d="M{cx-3.5} {cy} L{cx-0.5} {cy+3} L{cx+4} {cy-2.5}" {s}/>')
    if kind == "target":
        return (f'<circle cx="{cx}" cy="{cy}" r="8.5" {s}/><circle cx="{cx}" cy="{cy}" r="4" {s}/>'
                f'<circle cx="{cx}" cy="{cy}" r="0.8" fill="{color}"/>')
    if kind == "bulb":
        return (f'<path d="M{cx-5} {cy+4} C{cx-9} {cy+0} {cx-8} {cy-9} {cx} {cy-9}'
                f' C{cx+8} {cy-9} {cx+9} {cy+0} {cx+5} {cy+4} V{cy+6} H{cx-5} Z" {s}/>'
                f'<path d="M{cx-3} {cy+9.5} h6" {s}/>')
    return ""


def chip_row(x, y, chips, color, max_w):
    out, cx = [], x
    for c in chips:
        w = 13 + len(c) * 6.6
        if cx + w > x + max_w:
            break
        out.append(f'<rect x="{cx:.1f}" y="{y}" width="{w:.1f}" height="22" rx="11" fill="{color}" fill-opacity="0.10"/>'
                   f'<text x="{cx + w/2:.1f}" y="{y+15}" text-anchor="middle" font-size="11.5" font-weight="600" fill="{color}">{escape(c)}</text>')
        cx += w + 6
    return "".join(out)


def card(p, i, animated):
    key, n, title, head, lines, chips, color, (x, y), ic = p
    g = [f'<g id="card-{key}">']
    if animated:
        g.append(f'<rect class="glow glow-{i}" x="{x-5}" y="{y-5}" width="{CW+10}" height="{CH+10}" rx="20" fill="none" stroke="{color}" stroke-width="3"/>')
    g.append(f'<rect x="{x}" y="{y}" width="{CW}" height="{CH}" rx="16" fill="#ffffff" stroke="{CARD_BORDER}" filter="url(#shadow)"/>')
    g.append(f'<rect x="{x}" y="{y}" width="{CW}" height="5" rx="2.5" fill="{color}"/>')
    # badge + icon + title
    g.append(f'<circle cx="{x+28}" cy="{y+36}" r="15" fill="{color}" fill-opacity="0.12"/>')
    g.append(icon(ic, x + 28, y + 36, color))
    g.append(f'<text x="{x+52}" y="{y+32}" font-size="11.5" font-weight="700" fill="{color}" letter-spacing="1.2">{n} · {title.upper()}</text>')
    g.append(f'<text x="{x+52}" y="{y+50}" font-size="17" font-weight="700" fill="{INK}">{escape(head)}</text>')
    for k, line in enumerate(lines):
        g.append(f'<text x="{x+18}" y="{y+82+k*19}" font-size="13.5" fill="{MUTED}">{escape(line)}</text>')
    g.append(chip_row(x + 18, y + CH - 36, chips, color, CW - 30))
    g.append("</g>")
    return "".join(g)


def arrow(d, label=None, lx=0, ly=0, cls="flow", color=LINE, dashed=False, anchor="middle"):
    dash = ' stroke-dasharray="6 6"' if dashed else ""
    out = f'<path class="{cls}" d="{d}" fill="none" stroke="{color}" stroke-width="2.4"{dash} marker-end="url(#arrow-{color[1:]})"/>'
    if label:
        tw = len(label) * 6.3 + 14
        bx = lx - tw / 2 if anchor == "middle" else (lx if anchor == "start" else lx - tw)
        out += (f'<rect x="{bx:.1f}" y="{ly-13}" width="{tw:.1f}" height="19" rx="9.5" fill="#ffffff" stroke="{CARD_BORDER}"/>'
                f'<text x="{bx + tw/2:.1f}" y="{ly+1}" text-anchor="middle" font-size="11.5" font-weight="600" fill="{MUTED}">{escape(label)}</text>')
    return out


def pill(x, y, w, h, title, sub, color, ic):
    return (f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="{h/2}" fill="{color}"/>'
            f'<g>{icon(ic, x+26, y+h/2, "#ffffff")}</g>'
            f'<text x="{x+46}" y="{y+h/2-3}" font-size="14.5" font-weight="700" fill="#ffffff">{escape(title)}</text>'
            f'<text x="{x+46}" y="{y+h/2+14}" font-size="11.5" fill="#ffffff" fill-opacity="0.85">{escape(sub)}</text>')


def build(animated):
    markers = "".join(
        f'<marker id="arrow-{c[1:]}" viewBox="0 0 10 10" refX="8.5" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">'
        f'<path d="M0 0 L10 5 L0 10 z" fill="{c}"/></marker>'
        for c in {LINE, "#16a34a", "#ea580c", "#7c3aed", "#db2777", "#2563eb"})

    style = f"""
    text {{ font-family: {FONT}; }}
    """
    if animated:
        on = STEP / CYCLE * 100
        rise = on * 0.18
        style += f"""
    .glow {{ opacity: 0; animation: glow {CYCLE}s ease-in-out infinite; }}
    @keyframes glow {{ 0% {{ opacity: 0; }} {rise:.2f}% {{ opacity: 1; }} {on - rise:.2f}% {{ opacity: 1; }} {on:.2f}% {{ opacity: 0; }} 100% {{ opacity: 0; }} }}
    .flow {{ stroke-dasharray: 7 7; animation: flow 1.2s linear infinite; }}
    .loop {{ animation: flow 1.2s linear infinite; }}
    @keyframes flow {{ to {{ stroke-dashoffset: -14; }} }}
    @media (prefers-reduced-motion: reduce) {{ .glow, .flow, .loop {{ animation: none; }} .glow {{ opacity: 0; }} }}
    """ + "".join(f".glow-{i} {{ animation-delay: {i*STEP + 0.6:.1f}s; }}\n" for i in range(6))

    c = [f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" width="{W}" height="{H}" role="img" '
         f'aria-label="loops workflow: a ticket goes through research, plan, build, verify, prove and reflect, ending in a pull request with proofs.">',
         f"<title>How loops works</title>",
         f"<defs>{markers}"
         f'<filter id="shadow" x="-10%" y="-10%" width="120%" height="130%"><feDropShadow dx="0" dy="4" stdDeviation="7" flood-color="#0f172a" flood-opacity="0.08"/></filter>'
         f"</defs><style>{style}</style>",
         f'<rect width="{W}" height="{H}" fill="#ffffff"/>',
         # header
         f'<text x="40" y="56" font-size="28" font-weight="800" fill="{INK}">From ticket to proven pull request</text>',
         f'<text x="40" y="86" font-size="16" fill="{MUTED}">Six visible phases, each run by specialist AI agents that check each other\'s work.</text>',
         ]

    # flow arrows (main path, U shape)
    r1m, r2m = ROW1 + CH / 2, ROW2 + CH / 2
    c.append(arrow(f"M{40+152} {r1m} H{COLS[0]-6}"))                                   # ticket -> research
    c.append(arrow(f"M{COLS[0]+CW} {r1m} H{COLS[1]-6}"))                               # research -> plan
    c.append(arrow(f"M{COLS[1]+CW} {r1m} H{COLS[2]-6}"))                               # plan -> build
    bx = COLS[2] + CW / 2
    c.append(arrow(f"M{bx+40} {ROW1+CH} V{ROW2-6}", "claims", bx + 78, (ROW1 + CH + ROW2) / 2 + 4))  # build -> verify
    c.append(arrow(f"M{COLS[2]} {r2m} H{COLS[1]+CW+6}", "verified", (COLS[1] + CW + COLS[2]) / 2, r2m - 18))
    c.append(arrow(f"M{COLS[1]} {r2m} H{COLS[0]+CW+6}", "PR open", (COLS[0] + CW + COLS[1]) / 2, r2m - 18))
    c.append(arrow(f"M{COLS[0]} {r2m} H{40+152+6}"))                                   # reflect -> outcome

    # loop-backs
    c.append(arrow(f"M{bx-40} {ROW2} V{ROW1+CH+6}", "fix & retry", bx - 82, (ROW1 + CH + ROW2) / 2 + 4,
                   cls="loop", color="#16a34a", dashed=True))
    px = COLS[1] + CW / 2
    c.append(arrow(f"M{px+60} {ROW2} C{px+60} {ROW2-70} {COLS[2]-60} {ROW1+CH+50} {COLS[2]-6} {ROW1+CH-30}",
                   "gap found", px + 104, ROW2 - 52, cls="loop", color="#ea580c", dashed=True))
    pl = COLS[1] + CW / 2
    c.append(arrow(f"M{pl-50} {ROW1} C{pl-50} {ROW1-34} {pl+50} {ROW1-34} {pl+50} {ROW1-4}",
                   "review until solid", pl, ROW1 - 30, cls="loop", color="#7c3aed", dashed=True))

    # memory (between research and reflect)
    mx, my, mw, mh = COLS[0] + 26, ROW1 + CH + 52, CW - 52, 46
    c.append(f'<rect x="{mx}" y="{my}" width="{mw}" height="{mh}" rx="12" fill="#fdf2f8" stroke="#f9a8d4"/>'
             f'<text x="{mx+mw/2}" y="{my+20}" text-anchor="middle" font-size="13.5" font-weight="700" fill="#9d174d">ai-memory</text>'
             f'<text x="{mx+mw/2}" y="{my+36}" text-anchor="middle" font-size="11.5" fill="#9d174d" fill-opacity="0.8">lessons from earlier runs</text>')
    c.append(arrow(f"M{mx+mw/2-50} {my} V{ROW1+CH+6}", "recall", mx + mw / 2 - 86, my - 20, cls="loop", color="#db2777", dashed=True))
    c.append(arrow(f"M{mx+mw/2+50} {ROW2} V{my+mh+6}", "save", mx + mw / 2 + 80, ROW2 - 16, cls="loop", color="#db2777", dashed=True))

    # in / out pills
    c.append(pill(40, r1m - 34, 152, 68, "Ticket", "issue or idea", INK, "clipboard"))
    c.append(pill(40, r2m - 34, 152, 68, "Pull request", "with proofs", "#16a34a", "shield"))

    for i, p in enumerate(PHASES):
        c.append(card(p, i, animated))

    # guarantees band
    gy = H - 74
    items = [("Every step is visible", "status line · live pane · HTML report", "#2563eb", "search"),
             ("Sized to your machine", "up to 100 agents in parallel", "#0891b2", "layers"),
             ("Your rules always win", "overrides noted on the PR", "#16a34a", "shield")]
    gw = (W - 80 - 2 * 16) / 3
    for k, (t, s, col, ic) in enumerate(items):
        x = 40 + k * (gw + 16)
        c.append(f'<rect x="{x:.1f}" y="{gy}" width="{gw:.1f}" height="50" rx="14" fill="#f8fafc" stroke="{CARD_BORDER}"/>')
        c.append(f'<circle cx="{x+27:.1f}" cy="{gy+25}" r="14" fill="{col}" fill-opacity="0.12"/>{icon(ic, x+27, gy+25, col)}')
        c.append(f'<text x="{x+50:.1f}" y="{gy+21}" font-size="14" font-weight="700" fill="{INK}">{escape(t)}</text>')
        c.append(f'<text x="{x+50:.1f}" y="{gy+38}" font-size="12" fill="{MUTED}">{escape(s)}</text>')

    c.append("</svg>")
    return "\n".join(c)


if __name__ == "__main__":
    out = sys.argv[1]
    open(f"{out}/loops-workflow.svg", "w").write(build(True))
    open(f"{out}/loops-workflow-static.svg", "w").write(build(False))
    print("ok")
