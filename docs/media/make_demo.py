#!/usr/bin/env python3
"""Generate the readme's animated terminal demo (loops-demo.svg).

Usage: make_demo.py <out_dir>
A sample `/loops:work #42` session: one ticket in, every phase, a QA
failure sent back and fixed, the gate, and a PR with proofs. Pure SVG with
CSS animation (GitHub renders it through <img>); prefers-reduced-motion
shows the final frame. Record it like the workflow graphic (record.mjs).
"""
import sys
from html import escape

W, H = 1204, 620
MONO = "ui-monospace, SFMono-Regular, Menlo, Consolas, 'Liberation Mono', monospace"
BG, BAR, FG, DIM = "#0d1117", "#161b22", "#e6edf3", "#7d8590"
GREEN, RED, BLUE, YELLOW, PURPLE, CYAN, ORANGE, PINK = (
    "#3fb950", "#f85149", "#58a6ff", "#d29922", "#bc8cff", "#39c5cf", "#ffa657", "#ff7b72")
CYCLE = 16.0  # seconds per loop

# (appear_at_seconds, [(text, color), ...])
LINES = [
    (0.3, [("❯ ", GREEN), ("/loops:work #42", FG), ("   # “Add partial refunds to orders”", DIM)]),
    (1.6, [("● intake    ", BLUE), ("ticket → charter.md (goal, 4 ACs, scope)   checkpoints: ", FG), ("none", YELLOW)]),
    (2.6, [("● research  ", BLUE), ("web + practices + code + hardware", FG), ("          4 agents · 3m", DIM)]),
    (3.8, [("● plan      ", PURPLE), ("planner ⇄ reviewer  ", FG), ("r1 CHANGES", RED), (" → ", DIM), ("r2 APPROVE", GREEN), ("     2 rounds · 7m", DIM)]),
    (5.0, [("● build     ", CYAN), ("architect → domain + persistence + api + tests", FG), ("   wave 1 · 9m", DIM)]),
    (6.3, [("● verify    ", GREEN), ("W1-api:C2 ", FG), ("FAILED", RED), (" second refund → 500, repro sent back", DIM)]),
    (7.4, [("            ", FG), ("W1-api:C2 ", FG), ("VERIFIED", GREEN), (" (round 2)   10/10 claims verified", DIM)]),
    (8.5, [("● gate      ", GREEN), ("build ✓ fmt ✓ lint ✓ arch ✓ test -race ✓ ", FG), ("new-code coverage 84% ✓", GREEN)]),
    (9.6, [("● prove     ", ORANGE), ("AC-1 ✓ AC-2 ✓ AC-3 ✓ AC-4 ✓", FG), ("   tried by hand, transcripts attached", DIM)]),
    (10.7, [("● reflect   ", PINK), ("lesson saved: refunds need idempotency keys", FG)]),
    (11.8, [("✔ PR #43 opened with proofs", GREEN), ("  ·  31 min  ·  ", DIM), ("0 questions asked", YELLOW), ("  ·  1 decision recorded", DIM)]),
]

# status line states: (from_seconds, text)
STATUS = [
    (0.0, "loops · idle"),
    (1.6, "▶ #42 refunds · Intake"),
    (2.6, "▶ #42 refunds · Research · 4/4 lanes · 2m"),
    (3.8, "▶ #42 refunds · Plan · r2 · 9m"),
    (5.0, "▶ #42 refunds · Implement W1 · 4/4 lanes · 15m"),
    (6.3, "▶ #42 refunds · QA 9✓ 1↺ · 21m"),
    (8.5, "▶ #42 refunds · QA 10✓ · gates build✓ test✓ cov 84% · 25m"),
    (9.6, "▶ #42 refunds · Product · AC 4/4 · 29m"),
    (11.8, "✔ #42 refunds · done · PR #43 · 31m"),
]

X0, Y0, LH = 36, 110, 40


def pct(t):
    return f"{t / CYCLE * 100:.2f}%"


def keyframes(name, start, end=CYCLE - 0.6):
    return (f"@keyframes {name}{{0%,{pct(start)}{{opacity:0}}"
            f"{pct(start + 0.25)},{pct(end)}{{opacity:1}}{pct(end + 0.4)},100%{{opacity:0}}}}")


def main(out):
    css, body = [], []
    css.append(f"text{{font-family:{MONO};font-size:19px;white-space:pre}}")
    css.append(".anim{animation-duration:%ss;animation-iteration-count:infinite;animation-timing-function:linear}" % CYCLE)
    css.append("@media (prefers-reduced-motion: reduce){.anim{animation:none!important;opacity:1!important}.st{opacity:0!important}.st.last{opacity:1!important}}")
    for i, (t, spans) in enumerate(LINES):
        name = f"l{i}"
        css.append(keyframes(name, t))
        css.append(f".{name}{{animation-name:{name};opacity:0}}")
        tspans = "".join(f'<tspan fill="{c}">{escape(s)}</tspan>' for s, c in spans)
        body.append(f'<text class="anim {name}" x="{X0}" y="{Y0 + i * LH}">{tspans}</text>')
    # status line: each state visible until the next one starts
    for i, (t, s) in enumerate(STATUS):
        end = STATUS[i + 1][0] if i + 1 < len(STATUS) else CYCLE - 0.6
        name = f"s{i}"
        if i == 0:
            css.append(f"@keyframes {name}{{0%,{pct(end)}{{opacity:1}}{pct(end + 0.01)},100%{{opacity:0}}}}")
        else:
            css.append(f"@keyframes {name}{{0%,{pct(t - 0.01)}{{opacity:0}}{pct(t)},{pct(end)}{{opacity:1}}"
                       f"{pct(end + 0.01)},100%{{opacity:0}}}}")
        css.append(f".{name}{{animation-name:{name};opacity:0}}")
        last = " last" if i == len(STATUS) - 1 else ""
        color = GREEN if s.startswith("✔") else FG
        body.append(f'<text class="anim st {name}{last}" x="{X0}" y="{H - 26}" fill="{color}">{escape(s)}</text>')
    # caret blink on the prompt line
    css.append("@keyframes blink{0%,49%{opacity:1}50%,100%{opacity:0}}")
    svg = f'''<svg xmlns="http://www.w3.org/2000/svg" width="{W}" height="{H}" viewBox="0 0 {W} {H}" role="img" aria-labelledby="t d">
<title id="t">loops demo: one ticket in, a verified pull request out</title>
<desc id="d">A sample terminal session. /loops:work #42 runs intake, research, plan with a reviewer round, parallel build, QA that catches a failure and sends it back until it is verified, the gate with race detector and 84% new-code coverage, every acceptance criteria proven, and a pull request opened with proofs after 31 minutes and zero questions.</desc>
<style>{"".join(css)}</style>
<rect width="{W}" height="{H}" rx="14" fill="{BG}"/>
<rect width="{W}" height="52" rx="14" fill="{BAR}"/><rect y="38" width="{W}" height="14" fill="{BAR}"/>
<circle cx="30" cy="26" r="7" fill="#ff5f57"/><circle cx="54" cy="26" r="7" fill="#febc2e"/><circle cx="78" cy="26" r="7" fill="#28c840"/>
<text x="{W / 2}" y="32" text-anchor="middle" fill="{DIM}" style="font-size:15px">claude · ~/acme/orders</text>
{chr(10).join(body)}
<line x1="0" x2="{W}" y1="{H - 56}" y2="{H - 56}" stroke="#30363d"/>
</svg>
'''
    with open(f"{out}/loops-demo.svg", "w") as fh:
        fh.write(svg)
    print(f"{out}/loops-demo.svg")


if __name__ == "__main__":
    main(sys.argv[1] if len(sys.argv) > 1 else ".")
