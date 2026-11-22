---
name: report
description: Generate the HTML report for a loops run and show where it is. Use when the user runs /loops:report or asks for the run report.
argument-hint: "[run-id]"
disable-model-invocation: true
---

# /loops:report

1. Run `loops report` (or `loops report --run $ARGUMENTS` when a run id is given). It prints the path of `report.html`.
2. Print the path in your reply and offer to open it (`open <path>` on macOS, `xdg-open <path>` on Linux); open it only if the user says yes.
3. Mention publishing the report as a private Artifact only if the user asks for sharing; do not publish on your own.
