# loops for Gemini CLI

Tier B (subagents, no resume): loop-backs spawn a new agent of the same role
(specs/PLATFORMS.md section 4).

## Install

    gemini extensions install https://github.com/arxdsilva/ai-loops --ref <tag>

If your Gemini CLI cannot install from a subdirectory, clone the repository and run:

    gemini extensions install ./ai-loops/platforms/gemini

For development, symlink the package so edits apply without reinstalling:

    gemini extensions link ./platforms/gemini

## Use

- /loops:work <task>, /loops:init, /loops:status, /loops:report
- Role subagents live in agents/ (kind: local). Read-only roles get a tool
  allowlist without write_file or replace.
- Hooks (hooks/hooks.json) run bin/loops with --platform gemini. Timeouts are in
  milliseconds. Gemini has no subagent events.

## Models

Roles inherit the session model unless the package was built with
`loops port build --models <file.json>` mapping tiers to Gemini model ids.
